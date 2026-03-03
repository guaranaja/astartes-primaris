"""Librarium writer — batched inserts into TimescaleDB.

Accumulates bar and tick data in memory buffers, then flushes to the
market_bars and market_ticks hypertables in bulk using COPY for throughput.
Deduplication is handled by the UNIQUE index on market_bars (ON CONFLICT DO NOTHING).
"""

from __future__ import annotations

import asyncio
import logging
import time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import TYPE_CHECKING

import psycopg
from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool

if TYPE_CHECKING:
    from .config import Config

logger = logging.getLogger("auspex.writer")


@dataclass
class BarRow:
    """A single row for the market_bars hypertable."""

    time: datetime
    symbol: str
    timeframe: str
    source: str
    open: float
    high: float
    low: float
    close: float
    volume: int
    vwap: float | None = None
    trade_count: int | None = None


@dataclass
class TickRow:
    """A single row for the market_ticks hypertable."""

    time: datetime
    symbol: str
    source: str
    price: float
    size: int
    side: str  # 'bid', 'ask', 'trade'


class LibrariumWriter:
    """Async batched writer for TimescaleDB market data tables."""

    def __init__(self, config: Config):
        self.cfg = config.librarium
        self.source = config.data.source
        self._pool: AsyncConnectionPool | None = None

        # Buffers
        self._bar_buffer: list[BarRow] = []
        self._tick_buffer: list[TickRow] = []
        self._lock = asyncio.Lock()

        # Flush control
        self._batch_size = self.cfg.batch_size
        self._flush_interval = self.cfg.flush_interval
        self._flush_task: asyncio.Task | None = None
        self._running = False

        # Stats
        self.bars_written = 0
        self.ticks_written = 0
        self.flush_count = 0
        self.last_flush: float = 0

    async def connect(self) -> None:
        """Open connection pool to TimescaleDB."""
        logger.info("Connecting to Librarium at %s", self.cfg.dsn.replace(self.cfg.password, "***"))

        self._pool = AsyncConnectionPool(
            conninfo=self.cfg.dsn,
            min_size=self.cfg.min_pool,
            max_size=self.cfg.max_pool,
            open=False,
        )
        await self._pool.open()
        await self._pool.wait()

        # Verify connection
        async with self._pool.connection() as conn:
            row = await conn.execute("SELECT 1 AS ok")
            logger.info("Librarium connection verified")

    async def start_flush_loop(self) -> None:
        """Start the periodic flush loop."""
        self._running = True
        self._flush_task = asyncio.create_task(self._flush_loop())
        logger.info(
            "Flush loop started (batch_size=%d, interval=%.1fs)",
            self._batch_size,
            self._flush_interval,
        )

    async def stop(self) -> None:
        """Flush remaining data and close connections."""
        self._running = False
        if self._flush_task:
            self._flush_task.cancel()
            try:
                await self._flush_task
            except asyncio.CancelledError:
                pass

        # Final flush
        await self._flush()

        if self._pool:
            await self._pool.close()
            logger.info("Librarium connection pool closed")

        logger.info(
            "Writer stopped. Total: %d bars, %d ticks written in %d flushes",
            self.bars_written,
            self.ticks_written,
            self.flush_count,
        )

    def add_bar(self, row: BarRow) -> None:
        """Add a bar to the write buffer (non-blocking)."""
        self._bar_buffer.append(row)

    def add_tick(self, row: TickRow) -> None:
        """Add a tick to the write buffer (non-blocking)."""
        self._tick_buffer.append(row)

    async def _flush_loop(self) -> None:
        """Periodically flush buffers to DB."""
        while self._running:
            await asyncio.sleep(self._flush_interval)
            await self._flush()

    async def _flush(self) -> None:
        """Flush both bar and tick buffers to TimescaleDB."""
        async with self._lock:
            bars = self._bar_buffer[:]
            ticks = self._tick_buffer[:]
            self._bar_buffer.clear()
            self._tick_buffer.clear()

        if not bars and not ticks:
            return

        try:
            if bars:
                await self._write_bars(bars)
            if ticks:
                await self._write_ticks(ticks)

            self.flush_count += 1
            self.last_flush = time.time()
        except Exception:
            logger.exception("Error flushing to Librarium")
            # Put data back so we don't lose it
            async with self._lock:
                self._bar_buffer = bars + self._bar_buffer
                self._tick_buffer = ticks + self._tick_buffer

    async def _write_bars(self, bars: list[BarRow]) -> None:
        """Bulk insert bars using executemany with ON CONFLICT DO NOTHING."""
        sql = """
            INSERT INTO market_bars (time, symbol, timeframe, source, open, high, low, close, volume, vwap, trade_count)
            VALUES (%(time)s, %(symbol)s, %(timeframe)s, %(source)s, %(open)s, %(high)s, %(low)s, %(close)s, %(volume)s, %(vwap)s, %(trade_count)s)
            ON CONFLICT (symbol, timeframe, time, source) DO NOTHING
        """

        params = [
            {
                "time": b.time,
                "symbol": b.symbol,
                "timeframe": b.timeframe,
                "source": b.source,
                "open": b.open,
                "high": b.high,
                "low": b.low,
                "close": b.close,
                "volume": b.volume,
                "vwap": b.vwap,
                "trade_count": b.trade_count,
            }
            for b in bars
        ]

        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.executemany(sql, params)
            await conn.commit()

        self.bars_written += len(bars)
        logger.debug("Flushed %d bars to market_bars", len(bars))

    async def _write_ticks(self, ticks: list[TickRow]) -> None:
        """Bulk insert ticks."""
        sql = """
            INSERT INTO market_ticks (time, symbol, source, price, size, side)
            VALUES (%(time)s, %(symbol)s, %(source)s, %(price)s, %(size)s, %(side)s)
        """

        params = [
            {
                "time": t.time,
                "symbol": t.symbol,
                "source": t.source,
                "price": t.price,
                "size": t.size,
                "side": t.side,
            }
            for t in ticks
        ]

        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.executemany(sql, params)
            await conn.commit()

        self.ticks_written += len(ticks)
        logger.debug("Flushed %d ticks to market_ticks", len(ticks))

    @property
    def buffer_size(self) -> int:
        return len(self._bar_buffer) + len(self._tick_buffer)

"""Auspex — Market Data Collection for Astartes Primaris.

Main entry point. Orchestrates:
1. Connect to IBKR (TWS / IB Gateway)
2. Connect to Librarium (TimescaleDB)
3. Connect to Vox (NATS JetStream)
4. Resolve contracts for configured symbols
5. Backfill historical bars
6. Stream real-time bars and ticks
7. Write everything to Librarium + publish to Vox

Usage:
    python -m auspex
"""

from __future__ import annotations

import asyncio
import logging
import signal
import sys
from datetime import datetime, timezone

from ib_insync import util as ib_util

from .config import Config
from .health import register as register_health, start_server as start_health
from .ibkr import IBKRConnector, TIMEFRAME_NORMALIZE
from .writer import LibrariumWriter, BarRow, TickRow
from .vox import VoxPublisher

# ── Logging ──────────────────────────────────────────────────

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(name)s] %(levelname)s: %(message)s",
    datefmt="%H:%M:%S",
    stream=sys.stderr,
)
logger = logging.getLogger("auspex")


# ── Data Routing Callbacks ───────────────────────────────────

def make_bar_handler(writer: LibrariumWriter, vox: VoxPublisher, source: str):
    """Create a callback for historical/backfill bars."""

    def on_bar(symbol: str, timeframe: str, bar) -> None:
        # Convert ib_insync BarData to our BarRow
        bar_time = bar.date
        if isinstance(bar_time, str):
            bar_time = datetime.fromisoformat(bar_time)
        if bar_time.tzinfo is None:
            bar_time = bar_time.replace(tzinfo=timezone.utc)

        row = BarRow(
            time=bar_time,
            symbol=symbol,
            timeframe=timeframe,
            source=source,
            open=float(bar.open),
            high=float(bar.high),
            low=float(bar.low),
            close=float(bar.close),
            volume=int(bar.volume),
            vwap=float(bar.average) if hasattr(bar, "average") and bar.average else None,
            trade_count=int(bar.barCount) if hasattr(bar, "barCount") and bar.barCount else None,
        )
        writer.add_bar(row)

    return on_bar


def make_realtime_bar_handler(writer: LibrariumWriter, vox: VoxPublisher, source: str):
    """Create a callback for real-time 5-second bars."""

    async def _publish(symbol, bar_time, bar):
        await vox.publish_bar(
            symbol=symbol,
            timeframe="5s",
            time_=bar_time,
            open_=float(bar.open),
            high=float(bar.high),
            low=float(bar.low),
            close=float(bar.close),
            volume=int(bar.volume),
        )

    def on_realtime_bar(symbol: str, bar) -> None:
        bar_time = bar.time
        if isinstance(bar_time, str):
            bar_time = datetime.fromisoformat(bar_time)
        if hasattr(bar_time, "tzinfo") and bar_time.tzinfo is None:
            bar_time = bar_time.replace(tzinfo=timezone.utc)

        row = BarRow(
            time=bar_time,
            symbol=symbol,
            timeframe="5s",
            source=source,
            open=float(bar.open),
            high=float(bar.high),
            low=float(bar.low),
            close=float(bar.close),
            volume=int(bar.volume),
        )
        writer.add_bar(row)

        # Fire-and-forget Vox publish
        try:
            loop = asyncio.get_running_loop()
            loop.create_task(_publish(symbol, bar_time, bar))
        except RuntimeError:
            pass  # No loop running, skip Vox

    return on_realtime_bar


def make_tick_handler(writer: LibrariumWriter, vox: VoxPublisher, source: str):
    """Create a callback for real-time tick data."""

    async def _publish(symbol, price, size, side, tick_time):
        await vox.publish_tick(
            symbol=symbol,
            price=price,
            size=size,
            side=side,
            time_=tick_time,
        )

    def on_tick(symbol: str, ticker) -> None:
        tick_time = datetime.now(timezone.utc)

        # Emit bid, ask, and last as separate ticks
        entries = []
        if ticker.bid and ticker.bid > 0:
            entries.append(("bid", ticker.bid, int(ticker.bidSize or 0)))
        if ticker.ask and ticker.ask > 0:
            entries.append(("ask", ticker.ask, int(ticker.askSize or 0)))
        if ticker.last and ticker.last > 0:
            entries.append(("trade", ticker.last, int(ticker.lastSize or 0)))

        for side, price, size in entries:
            row = TickRow(
                time=tick_time,
                symbol=symbol,
                source=source,
                price=price,
                size=size,
                side=side,
            )
            writer.add_tick(row)

            try:
                loop = asyncio.get_running_loop()
                loop.create_task(_publish(symbol, price, size, side, tick_time))
            except RuntimeError:
                pass

    return on_tick


# ── Main ─────────────────────────────────────────────────────

async def main() -> None:
    config = Config()

    log_level = getattr(logging, config.log_level.upper(), logging.INFO)
    logging.getLogger().setLevel(log_level)

    logger.info("=" * 60)
    logger.info("  AUSPEX — Market Data Collection")
    logger.info("  Astartes Primaris Imperium")
    logger.info("=" * 60)
    logger.info("Symbols: %s", ", ".join(config.data.symbols))
    logger.info("Timeframes: %s", ", ".join(config.data.bar_timeframes))
    logger.info("Backfill: %d days", config.data.backfill_days)
    logger.info("Ticks: %s | Bars: %s", config.data.stream_ticks, config.data.stream_bars)

    # ── Components ──
    ibkr = IBKRConnector(config)
    writer = LibrariumWriter(config)
    vox = VoxPublisher(config)

    # ── Health server ──
    register_health(ibkr, writer, vox)
    start_health(config.health.port)

    # ── Connect all ──
    logger.info("Connecting to Librarium...")
    await writer.connect()
    await writer.start_flush_loop()

    logger.info("Connecting to Vox...")
    try:
        await vox.connect()
    except Exception:
        logger.warning("Vox connection failed — continuing without event publishing")

    logger.info("Connecting to IBKR...")
    await ibkr.connect()

    # ── Resolve contracts ──
    await ibkr.resolve_contracts()

    if ibkr.contract_count == 0:
        logger.error("No contracts resolved. Check AUSPEX_SYMBOLS configuration.")
        await shutdown(ibkr, writer, vox)
        return

    # ── Wire up data routing ──
    source = config.data.source
    ibkr.on_bar = make_bar_handler(writer, vox, source)
    ibkr.on_realtime_bar = make_realtime_bar_handler(writer, vox, source)
    ibkr.on_tick = make_tick_handler(writer, vox, source)

    # ── Backfill ──
    logger.info("Starting historical backfill...")
    total_bars = await ibkr.backfill_bars()
    logger.info("Backfill complete: %d bars queued for writing", total_bars)

    # Flush the backfill data before starting streaming
    logger.info("Flushing backfill data...")
    await asyncio.sleep(config.librarium.flush_interval + 1)

    # ── Stream ──
    logger.info("Starting real-time streams...")
    await ibkr.start_streaming()

    logger.info("=" * 60)
    logger.info("  AUSPEX ONLINE — Watching the void")
    logger.info("=" * 60)

    # ── Graceful shutdown ──
    stop_event = asyncio.Event()

    def _signal_handler():
        logger.info("Shutdown signal received")
        stop_event.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, _signal_handler)

    # Run until stopped
    await stop_event.wait()
    await shutdown(ibkr, writer, vox)


async def shutdown(ibkr: IBKRConnector, writer: LibrariumWriter, vox: VoxPublisher) -> None:
    """Gracefully shut down all components."""
    logger.info("Shutting down Auspex...")
    await ibkr.disconnect()
    await writer.stop()
    await vox.disconnect()
    logger.info("Auspex offline. The Emperor protects.")


if __name__ == "__main__":
    ib_util.patchAsyncio()  # Required for ib_insync + asyncio compatibility
    asyncio.run(main())

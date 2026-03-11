"""Alpaca Markets connector — market data via alpaca-py SDK.

Handles:
- Historical bar backfills via REST API
- Real-time bar streaming via websocket
- Real-time trade streaming via websocket
- Both stock and crypto data feeds
"""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timedelta, timezone
from typing import TYPE_CHECKING, Callable

from alpaca.data.historical import StockHistoricalDataClient, CryptoHistoricalDataClient
from alpaca.data.live import StockDataStream, CryptoDataStream
from alpaca.data.requests import StockBarsRequest, CryptoBarsRequest
from alpaca.data.timeframe import TimeFrame, TimeFrameUnit

if TYPE_CHECKING:
    from .config import Config

logger = logging.getLogger("auspex.alpaca")

# Map our timeframe strings to Alpaca TimeFrame objects
TIMEFRAME_MAP: dict[str, TimeFrame] = {
    "1 min": TimeFrame(1, TimeFrameUnit.Minute),
    "5 mins": TimeFrame(5, TimeFrameUnit.Minute),
    "15 mins": TimeFrame(15, TimeFrameUnit.Minute),
    "1 hour": TimeFrame(1, TimeFrameUnit.Hour),
    "1 day": TimeFrame(1, TimeFrameUnit.Day),
}

# Normalize to our storage labels
TIMEFRAME_NORMALIZE = {
    "1 min": "1m",
    "5 mins": "5m",
    "15 mins": "15m",
    "1 hour": "1h",
    "1 day": "1d",
}

# Symbols that should use the crypto feed
_CRYPTO_PREFIXES = {"BTC", "ETH", "LTC", "BCH", "DOGE", "SHIB", "AVAX", "UNI", "LINK", "SOL"}


def _is_crypto(symbol: str) -> bool:
    """Heuristic: treat symbols ending with USD or containing / as crypto."""
    upper = symbol.upper()
    if "/" in upper:
        return True
    # Common crypto symbols
    base = upper.replace("USD", "").replace("/", "")
    return base in _CRYPTO_PREFIXES


class AlpacaConnector:
    """Manages Alpaca market data connections and data collection."""

    def __init__(self, config: Config):
        self.cfg = config.alpaca
        self.data_cfg = config.data
        self._running = False

        api_key = self.cfg.api_key
        secret_key = self.cfg.secret_key

        # REST clients for historical data
        self._stock_client = StockHistoricalDataClient(api_key, secret_key)
        self._crypto_client = CryptoHistoricalDataClient(api_key, secret_key)

        # Websocket streams for real-time data
        self._stock_stream = StockDataStream(api_key, secret_key, feed=self.cfg.feed)
        self._crypto_stream = CryptoDataStream(api_key, secret_key)

        # Classify symbols
        self._stock_symbols: list[str] = []
        self._crypto_symbols: list[str] = []
        for sym in self.data_cfg.symbols:
            if _is_crypto(sym):
                self._crypto_symbols.append(sym)
            else:
                self._stock_symbols.append(sym)

        # Callbacks — set by the main orchestrator
        self.on_bar: Callable | None = None  # (symbol, timeframe, bar_data)
        self.on_tick: Callable | None = None  # (symbol, tick_data)
        self.on_realtime_bar: Callable | None = None  # (symbol, bar_data)

    async def connect(self) -> None:
        """Validate credentials by making a small request."""
        logger.info(
            "Alpaca connector initialized (feed=%s, stocks=%d, crypto=%d)",
            self.cfg.feed,
            len(self._stock_symbols),
            len(self._crypto_symbols),
        )

    async def disconnect(self) -> None:
        """Stop streams and clean up."""
        self._running = False
        try:
            await self._stock_stream.close()
        except Exception:
            pass
        try:
            await self._crypto_stream.close()
        except Exception:
            pass
        logger.info("Disconnected from Alpaca")

    async def backfill_bars(self) -> int:
        """Fetch historical bars for all symbols and timeframes.

        Returns total number of bars fetched.
        """
        total = 0
        days = self.data_cfg.backfill_days
        end = datetime.now(timezone.utc)
        start = end - timedelta(days=days)

        for tf in self.data_cfg.bar_timeframes:
            if tf not in TIMEFRAME_MAP:
                logger.warning("Unknown timeframe '%s', skipping", tf)
                continue

            timeframe = TIMEFRAME_MAP[tf]
            normalized_tf = TIMEFRAME_NORMALIZE.get(tf, tf)

            # Stock symbols
            if self._stock_symbols:
                total += await self._backfill_stock_bars(
                    self._stock_symbols, timeframe, normalized_tf, start, end, tf,
                )

            # Crypto symbols
            if self._crypto_symbols:
                total += await self._backfill_crypto_bars(
                    self._crypto_symbols, timeframe, normalized_tf, start, end, tf,
                )

        logger.info("Backfill complete: %d total bars", total)
        return total

    async def _backfill_stock_bars(
        self,
        symbols: list[str],
        timeframe: TimeFrame,
        normalized_tf: str,
        start: datetime,
        end: datetime,
        tf_label: str,
    ) -> int:
        """Backfill stock bars via REST API."""
        total = 0
        logger.info("Backfilling stocks %s %s (%s -> %s)", symbols, tf_label, start, end)

        try:
            request = StockBarsRequest(
                symbol_or_symbols=symbols,
                timeframe=timeframe,
                start=start,
                end=end,
            )
            barset = self._stock_client.get_stock_bars(request)

            if self.on_bar:
                for symbol in symbols:
                    bars = barset.data.get(symbol, [])
                    for bar in bars:
                        self.on_bar(symbol, normalized_tf, bar)
                    total += len(bars)
                    logger.info("  -> %d bars for %s %s", len(bars), symbol, tf_label)

        except Exception:
            logger.exception("Error backfilling stocks %s %s", symbols, tf_label)

        return total

    async def _backfill_crypto_bars(
        self,
        symbols: list[str],
        timeframe: TimeFrame,
        normalized_tf: str,
        start: datetime,
        end: datetime,
        tf_label: str,
    ) -> int:
        """Backfill crypto bars via REST API."""
        total = 0
        logger.info("Backfilling crypto %s %s (%s -> %s)", symbols, tf_label, start, end)

        try:
            request = CryptoBarsRequest(
                symbol_or_symbols=symbols,
                timeframe=timeframe,
                start=start,
                end=end,
            )
            barset = self._crypto_client.get_crypto_bars(request)

            if self.on_bar:
                for symbol in symbols:
                    bars = barset.data.get(symbol, [])
                    for bar in bars:
                        self.on_bar(symbol, normalized_tf, bar)
                    total += len(bars)
                    logger.info("  -> %d bars for %s %s", len(bars), symbol, tf_label)

        except Exception:
            logger.exception("Error backfilling crypto %s %s", symbols, tf_label)

        return total

    async def start_streaming(self) -> None:
        """Start real-time websocket streams for all symbols."""
        self._running = True

        # Subscribe stock streams
        if self._stock_symbols:
            if self.data_cfg.stream_bars:
                self._stock_stream.subscribe_bars(self._on_stock_bar, *self._stock_symbols)
            if self.data_cfg.stream_ticks:
                self._stock_stream.subscribe_trades(self._on_stock_trade, *self._stock_symbols)

            asyncio.create_task(self._run_stock_stream())

        # Subscribe crypto streams
        if self._crypto_symbols:
            if self.data_cfg.stream_bars:
                self._crypto_stream.subscribe_bars(self._on_crypto_bar, *self._crypto_symbols)
            if self.data_cfg.stream_ticks:
                self._crypto_stream.subscribe_trades(self._on_crypto_trade, *self._crypto_symbols)

            asyncio.create_task(self._run_crypto_stream())

        logger.info("Real-time streams started")

    async def _run_stock_stream(self) -> None:
        """Run the stock websocket stream (reconnects on failure)."""
        while self._running:
            try:
                logger.info("Connecting stock data stream...")
                await self._stock_stream._run_forever()
            except Exception:
                if not self._running:
                    break
                logger.exception("Stock stream disconnected, reconnecting in 5s...")
                await asyncio.sleep(5)

    async def _run_crypto_stream(self) -> None:
        """Run the crypto websocket stream (reconnects on failure)."""
        while self._running:
            try:
                logger.info("Connecting crypto data stream...")
                await self._crypto_stream._run_forever()
            except Exception:
                if not self._running:
                    break
                logger.exception("Crypto stream disconnected, reconnecting in 5s...")
                await asyncio.sleep(5)

    async def _on_stock_bar(self, bar) -> None:
        """Handle incoming real-time stock bar."""
        if self.on_realtime_bar:
            self.on_realtime_bar(bar.symbol, bar)

    async def _on_crypto_bar(self, bar) -> None:
        """Handle incoming real-time crypto bar."""
        if self.on_realtime_bar:
            self.on_realtime_bar(bar.symbol, bar)

    async def _on_stock_trade(self, trade) -> None:
        """Handle incoming real-time stock trade."""
        if self.on_tick:
            self.on_tick(trade.symbol, trade)

    async def _on_crypto_trade(self, trade) -> None:
        """Handle incoming real-time crypto trade."""
        if self.on_tick:
            self.on_tick(trade.symbol, trade)

    @property
    def connected(self) -> bool:
        return self._running

    @property
    def contract_count(self) -> int:
        return len(self._stock_symbols) + len(self._crypto_symbols)

    async def run_until_stopped(self) -> None:
        """Block until stopped."""
        while self._running:
            await asyncio.sleep(0.5)

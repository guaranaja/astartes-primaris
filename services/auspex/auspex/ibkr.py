"""IBKR connector — connects to TWS/Gateway via ib_insync.

Handles:
- Connection lifecycle and reconnection
- Contract resolution from symbol strings
- Historical bar backfills
- Real-time 5-second bar streaming
- Real-time tick streaming (bid/ask/last)
"""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timedelta, timezone
from typing import TYPE_CHECKING, Callable

from ib_insync import IB, BarData, Contract, Future, Stock, Ticker, util

if TYPE_CHECKING:
    from .config import Config

logger = logging.getLogger("auspex.ibkr")

# Map our timeframe strings to IBKR barSizeSetting values
TIMEFRAME_MAP = {
    "1 min": "1 min",
    "5 mins": "5 mins",
    "15 mins": "15 mins",
    "1 hour": "1 hour",
    "1 day": "1 day",
}

# Map IBKR barSizeSetting to our normalized timeframe labels for storage
TIMEFRAME_NORMALIZE = {
    "1 min": "1m",
    "5 mins": "5m",
    "15 mins": "15m",
    "1 hour": "1h",
    "1 day": "1d",
    "5 secs": "5s",
}

# Duration string for IBKR historical requests per timeframe
BACKFILL_DURATION = {
    "1 min": "{days} D",
    "5 mins": "{days} D",
    "15 mins": "{days} D",
    "1 hour": "{days} D",
    "1 day": "1 Y",
}

# IBKR rate limit: max 60 historical requests per 10 minutes
_HIST_THROTTLE_SECS = 11  # ~5.5 requests per minute, safe margin


def parse_contract(symbol_str: str) -> Contract:
    """Parse 'SYMBOL:EXCHANGE:SECTYPE:CURRENCY' into an IB Contract.

    Examples:
        'ES:CME:FUT:USD'   -> Future('ES', 'CME', 'USD')
        'AAPL:SMART:STK:USD' -> Stock('AAPL', 'SMART', 'USD')
    """
    parts = symbol_str.strip().split(":")
    if len(parts) != 4:
        raise ValueError(
            f"Invalid symbol format '{symbol_str}'. Expected SYMBOL:EXCHANGE:SECTYPE:CURRENCY"
        )

    symbol, exchange, sectype, currency = parts
    sectype = sectype.upper()

    if sectype == "FUT":
        return Future(symbol=symbol, exchange=exchange, currency=currency)
    elif sectype == "STK":
        return Stock(symbol=symbol, exchange=exchange, currency=currency)
    else:
        # Generic contract for other types
        c = Contract()
        c.symbol = symbol
        c.exchange = exchange
        c.secType = sectype
        c.currency = currency
        return c


class IBKRConnector:
    """Manages the IBKR connection and data collection."""

    def __init__(self, config: Config):
        self.cfg = config.ibkr
        self.data_cfg = config.data
        self.ib = IB()
        self._contracts: dict[str, Contract] = {}  # symbol_str -> qualified contract
        self._running = False

        # Callbacks — set by the main orchestrator
        self.on_bar: Callable | None = None  # (symbol, timeframe, bar_data)
        self.on_tick: Callable | None = None  # (symbol, tick_data)
        self.on_realtime_bar: Callable | None = None  # (symbol, bar_data)

    async def connect(self) -> None:
        """Connect to TWS/Gateway."""
        logger.info(
            "Connecting to IBKR at %s:%d (client_id=%d, readonly=%s)",
            self.cfg.host,
            self.cfg.port,
            self.cfg.client_id,
            self.cfg.readonly,
        )

        self.ib.connectedEvent += self._on_connected
        self.ib.disconnectedEvent += self._on_disconnected
        self.ib.errorEvent += self._on_error

        await self.ib.connectAsync(
            host=self.cfg.host,
            port=self.cfg.port,
            clientId=self.cfg.client_id,
            readonly=self.cfg.readonly,
            account=self.cfg.account or "",
            timeout=self.cfg.timeout,
        )

        logger.info("Connected to IBKR. Server version: %s", self.ib.client.serverVersion())

    async def disconnect(self) -> None:
        """Disconnect from TWS/Gateway."""
        self._running = False
        self.ib.disconnect()
        logger.info("Disconnected from IBKR")

    async def resolve_contracts(self) -> None:
        """Qualify all configured symbol contracts with IBKR."""
        for symbol_str in self.data_cfg.symbols:
            try:
                contract = parse_contract(symbol_str)
                qualified = await self.ib.qualifyContractsAsync(contract)

                if qualified:
                    self._contracts[symbol_str] = qualified[0]
                    c = qualified[0]
                    logger.info(
                        "Resolved %s -> %s %s %s (conId=%d)",
                        symbol_str,
                        c.symbol,
                        c.lastTradeDateOrContractMonth,
                        c.exchange,
                        c.conId,
                    )
                else:
                    logger.warning("Could not resolve contract for %s", symbol_str)
            except Exception:
                logger.exception("Error resolving contract %s", symbol_str)

        logger.info(
            "Resolved %d / %d contracts", len(self._contracts), len(self.data_cfg.symbols)
        )

    async def backfill_bars(self) -> int:
        """Fetch historical bars for all contracts and timeframes.

        Returns total number of bars fetched.
        """
        total = 0
        days = self.data_cfg.backfill_days

        for symbol_str, contract in self._contracts.items():
            for tf in self.data_cfg.bar_timeframes:
                if tf not in TIMEFRAME_MAP:
                    logger.warning("Unknown timeframe '%s', skipping", tf)
                    continue

                bar_size = TIMEFRAME_MAP[tf]
                duration_tmpl = BACKFILL_DURATION.get(tf, "{days} D")
                duration = duration_tmpl.format(days=days)
                normalized_tf = TIMEFRAME_NORMALIZE.get(tf, tf)

                logger.info(
                    "Backfilling %s %s — duration=%s barSize=%s",
                    symbol_str,
                    tf,
                    duration,
                    bar_size,
                )

                try:
                    bars = await self.ib.reqHistoricalDataAsync(
                        contract,
                        endDateTime="",
                        durationStr=duration,
                        barSizeSetting=bar_size,
                        whatToShow="TRADES",
                        useRTH=False,
                        formatDate=2,  # UTC
                    )

                    if bars and self.on_bar:
                        symbol = contract.symbol
                        for bar in bars:
                            self.on_bar(symbol, normalized_tf, bar)
                        total += len(bars)
                        logger.info("  -> %d bars for %s %s", len(bars), symbol_str, tf)
                    else:
                        logger.info("  -> 0 bars for %s %s", symbol_str, tf)

                except Exception:
                    logger.exception("Error backfilling %s %s", symbol_str, tf)

                # Throttle to respect IBKR rate limits
                await asyncio.sleep(_HIST_THROTTLE_SECS)

        logger.info("Backfill complete: %d total bars", total)
        return total

    async def start_streaming(self) -> None:
        """Start real-time data streams for all resolved contracts."""
        self._running = True

        for symbol_str, contract in self._contracts.items():
            symbol = contract.symbol

            # Real-time 5-second bars
            if self.data_cfg.stream_bars:
                logger.info("Starting 5s bar stream for %s", symbol_str)
                self.ib.reqRealTimeBars(
                    contract,
                    barSize=5,
                    whatToShow="TRADES",
                    useRTH=False,
                )

            # Real-time ticks
            if self.data_cfg.stream_ticks:
                logger.info("Starting tick stream for %s", symbol_str)
                self.ib.reqMktData(contract, genericTickList="", snapshot=False)

        # Wire up event handlers
        self.ib.pendingTickersEvent += self._on_pending_tickers
        self.ib.barUpdateEvent += self._on_bar_update

        logger.info("Real-time streams started")

    def _on_bar_update(self, bars, has_new_bar: bool) -> None:
        """Handle incoming 5-second real-time bars."""
        if not has_new_bar or not self.on_realtime_bar:
            return

        contract = bars.contract
        bar = bars[-1]
        self.on_realtime_bar(contract.symbol, bar)

    def _on_pending_tickers(self, tickers: list[Ticker]) -> None:
        """Handle incoming tick updates."""
        if not self.on_tick:
            return

        for ticker in tickers:
            if ticker.contract:
                self.on_tick(ticker.contract.symbol, ticker)

    def _on_connected(self) -> None:
        logger.info("IBKR connection established")

    def _on_disconnected(self) -> None:
        logger.warning("IBKR connection lost")
        if self._running:
            logger.info("Will attempt reconnection...")

    def _on_error(self, req_id: int, error_code: int, error_string: str, contract) -> None:
        # Filter out non-critical "market data farm" messages
        if error_code in (2104, 2106, 2158, 2119):
            logger.debug("IBKR info [%d]: %s", error_code, error_string)
        else:
            logger.warning("IBKR error [%d] req=%d: %s", error_code, req_id, error_string)

    @property
    def connected(self) -> bool:
        return self.ib.isConnected()

    @property
    def contract_count(self) -> int:
        return len(self._contracts)

    @property
    def contracts(self) -> dict[str, Contract]:
        return dict(self._contracts)

    async def run_until_stopped(self) -> None:
        """Block until stopped — keeps the ib_insync event loop alive."""
        while self._running:
            self.ib.sleep(0.1)
            await asyncio.sleep(0.01)

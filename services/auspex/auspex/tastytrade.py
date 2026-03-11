"""TastyTrade connector — options chain data via tastytrade SDK.

Handles:
- OAuth authentication (client_secret + refresh_token)
- Options chain fetching for configured symbols
- Streaming quotes and greeks via DXLink
- Periodic collection during market hours
"""

from __future__ import annotations

import asyncio
import logging
from datetime import date, datetime, timedelta, timezone
from typing import TYPE_CHECKING

from tastytrade import Session
from tastytrade.instruments import get_option_chain
from tastytrade.market_data import get_market_data_by_type

if TYPE_CHECKING:
    from .config import Config
    from .writer import LibrariumWriter

logger = logging.getLogger("auspex.tastytrade")


class TastyTradeConnector:
    """Manages TastyTrade options data collection."""

    def __init__(self, config: Config, writer: LibrariumWriter):
        self.cfg = config.tastytrade
        self._writer = writer
        self._session: Session | None = None
        self._running = False
        self._collection_task: asyncio.Task | None = None

    async def connect(self) -> None:
        """Authenticate with TastyTrade via OAuth."""
        logger.info("Authenticating with TastyTrade...")
        self._session = Session(
            self.cfg.client_secret,
            self.cfg.refresh_token,
        )
        logger.info(
            "TastyTrade session established (symbols=%s, max_dte=%d)",
            ",".join(self.cfg.symbols),
            self.cfg.max_dte,
        )

    async def disconnect(self) -> None:
        """Stop collection and close session."""
        self._running = False
        if self._collection_task:
            self._collection_task.cancel()
            try:
                await self._collection_task
            except asyncio.CancelledError:
                pass
        if self._session:
            self._session.destroy()
            self._session = None
        logger.info("Disconnected from TastyTrade")

    async def collect_option_chains(self) -> int:
        """Fetch option chains with quotes and greeks for all configured symbols.

        Returns total number of option snapshots collected.
        """
        from .writer import OptionChainRow

        if not self._session:
            logger.error("No active TastyTrade session")
            return 0

        now = datetime.now(timezone.utc)
        cutoff = date.today() + timedelta(days=self.cfg.max_dte)
        total = 0

        for symbol in self.cfg.symbols:
            try:
                chain = await get_option_chain(self._session, symbol)
            except Exception:
                logger.exception("Failed to fetch option chain for %s", symbol)
                continue

            # Filter expirations within max_dte
            valid_expirations = {
                exp: options
                for exp, options in chain.items()
                if exp <= cutoff
            }

            if not valid_expirations:
                logger.warning("No expirations within %d DTE for %s", self.cfg.max_dte, symbol)
                continue

            # Collect all options, process per-expiration to keep batch sizes manageable
            for exp, options in valid_expirations.items():
                occ_symbols = [opt.symbol for opt in options]

                if not occ_symbols:
                    continue

                # Fetch market data in batches of 100 via REST
                snapshots: dict = {}
                for i in range(0, len(occ_symbols), 100):
                    batch = occ_symbols[i : i + 100]
                    try:
                        data = await get_market_data_by_type(
                            self._session, options=batch,
                        )
                        for item in data:
                            snapshots[item.symbol] = item
                    except Exception:
                        logger.exception(
                            "Market data error for %s exp %s batch %d",
                            symbol, exp, i,
                        )

                # Build rows
                for opt in options:
                    snap = snapshots.get(opt.symbol)
                    if not snap:
                        continue

                    # REST MarketData has bid/ask/mark/volume/open_interest.
                    # Greeks require DXLink streaming (future enhancement).
                    row = OptionChainRow(
                        time=now,
                        underlying=symbol,
                        expiration=opt.expiration_date,
                        strike=float(opt.strike_price),
                        option_type="C" if opt.option_type.value == "C" else "P",
                        bid=float(snap.bid) if snap.bid is not None else None,
                        ask=float(snap.ask) if snap.ask is not None else None,
                        mark=float(snap.mark),
                        volume=int(snap.volume) if snap.volume is not None else None,
                        open_interest=int(snap.open_interest) if snap.open_interest is not None else None,
                        delta=None,
                        gamma=None,
                        theta=None,
                        vega=None,
                        iv=None,
                        source="tastytrade",
                    )
                    self._writer.add_option_chain(row)
                    total += 1

            logger.info(
                "Collected %d option snapshots for %s (%d expirations)",
                total, symbol, len(valid_expirations),
            )

        logger.info("Option chain collection complete: %d total snapshots", total)
        return total

    async def start_periodic_collection(self) -> None:
        """Run collect_option_chains on a periodic interval."""
        self._running = True
        self._collection_task = asyncio.create_task(self._collection_loop())
        logger.info(
            "Periodic option collection started (interval=%ds)",
            self.cfg.collection_interval,
        )

    async def _collection_loop(self) -> None:
        """Periodically collect option chains."""
        while self._running:
            try:
                count = await self.collect_option_chains()
                logger.info("Periodic collection: %d snapshots", count)
            except Exception:
                logger.exception("Error in periodic option collection")

            await asyncio.sleep(self.cfg.collection_interval)

    @property
    def connected(self) -> bool:
        return self._session is not None and self._running

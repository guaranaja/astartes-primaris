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
from tastytrade.dxfeed import Quote, Greeks
from tastytrade import DXLinkStreamer

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

            # Collect all streamer symbols for quotes and greeks
            all_options = []
            for exp, options in valid_expirations.items():
                all_options.extend(options)

            streamer_symbols = [opt.streamer_symbol for opt in all_options]

            if not streamer_symbols:
                continue

            # Fetch quotes and greeks via DXLink streamer
            quotes: dict[str, Quote] = {}
            greeks: dict[str, Greeks] = {}

            try:
                async with DXLinkStreamer(self._session) as streamer:
                    await streamer.subscribe(Quote, streamer_symbols)
                    await streamer.subscribe(Greeks, streamer_symbols)

                    # Collect events with a timeout to avoid hanging
                    deadline = asyncio.get_event_loop().time() + 30
                    while len(quotes) < len(streamer_symbols) or len(greeks) < len(streamer_symbols):
                        remaining = deadline - asyncio.get_event_loop().time()
                        if remaining <= 0:
                            break

                        try:
                            quote = await asyncio.wait_for(
                                streamer.get_event(Quote), timeout=min(remaining, 2.0),
                            )
                            quotes[quote.eventSymbol] = quote
                        except asyncio.TimeoutError:
                            pass

                        try:
                            greek = await asyncio.wait_for(
                                streamer.get_event(Greeks), timeout=min(remaining, 2.0),
                            )
                            greeks[greek.eventSymbol] = greek
                        except asyncio.TimeoutError:
                            pass

            except Exception:
                logger.exception("Streamer error for %s", symbol)
                continue

            # Build rows from collected data
            for opt in all_options:
                ss = opt.streamer_symbol
                q = quotes.get(ss)
                g = greeks.get(ss)

                if not q and not g:
                    continue

                row = OptionChainRow(
                    time=now,
                    underlying=symbol,
                    expiration=opt.expiration_date,
                    strike=float(opt.strike_price),
                    option_type="C" if opt.option_type.value == "C" else "P",
                    bid=float(q.bidPrice) if q and q.bidPrice else None,
                    ask=float(q.askPrice) if q and q.askPrice else None,
                    mark=float(g.price) if g and g.price else None,
                    volume=int(q.dayVolume) if q and q.dayVolume else None,
                    open_interest=int(opt.open_interest) if hasattr(opt, "open_interest") and opt.open_interest else None,
                    delta=float(g.delta) if g and g.delta else None,
                    gamma=float(g.gamma) if g and g.gamma else None,
                    theta=float(g.theta) if g and g.theta else None,
                    vega=float(g.vega) if g and g.vega else None,
                    iv=float(g.volatility) if g and g.volatility else None,
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

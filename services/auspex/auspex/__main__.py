"""Auspex — Market Data Collection for Astartes Primaris.

Main entry point. Orchestrates:
1. Connect to data provider (IBKR or Alpaca)
2. Connect to Librarium (TimescaleDB)
3. Connect to Vox (NATS JetStream)
4. Resolve contracts / validate symbols
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

from .config import Config
from .health import register as register_health, start_server as start_health
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

def make_bar_handler(writer: LibrariumWriter, vox: VoxPublisher, source: str, provider: str):
    """Create a callback for historical/backfill bars."""

    def on_bar(symbol: str, timeframe: str, bar) -> None:
        if provider == "ibkr":
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
        else:
            # Alpaca bar object
            bar_time = bar.timestamp
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
                vwap=float(bar.vwap) if bar.vwap else None,
                trade_count=int(bar.trade_count) if bar.trade_count else None,
            )
        writer.add_bar(row)

    return on_bar


def make_realtime_bar_handler(writer: LibrariumWriter, vox: VoxPublisher, source: str, provider: str):
    """Create a callback for real-time bars."""

    async def _publish(symbol, bar_time, bar):
        await vox.publish_bar(
            symbol=symbol,
            timeframe="1m",
            time_=bar_time,
            open_=float(bar.open),
            high=float(bar.high),
            low=float(bar.low),
            close=float(bar.close),
            volume=int(bar.volume),
        )

    def on_realtime_bar(symbol: str, bar) -> None:
        if provider == "ibkr":
            bar_time = bar.time
            if isinstance(bar_time, str):
                bar_time = datetime.fromisoformat(bar_time)
            if hasattr(bar_time, "tzinfo") and bar_time.tzinfo is None:
                bar_time = bar_time.replace(tzinfo=timezone.utc)
            tf = "5s"
            vol = int(bar.volume)
        else:
            # Alpaca real-time bar
            bar_time = bar.timestamp
            if bar_time.tzinfo is None:
                bar_time = bar_time.replace(tzinfo=timezone.utc)
            tf = "1m"
            vol = int(bar.volume)

        row = BarRow(
            time=bar_time,
            symbol=symbol,
            timeframe=tf,
            source=source,
            open=float(bar.open),
            high=float(bar.high),
            low=float(bar.low),
            close=float(bar.close),
            volume=vol,
        )
        writer.add_bar(row)

        # Fire-and-forget Vox publish
        try:
            loop = asyncio.get_running_loop()
            loop.create_task(_publish(symbol, bar_time, bar))
        except RuntimeError:
            pass  # No loop running, skip Vox

    return on_realtime_bar


def make_tick_handler(writer: LibrariumWriter, vox: VoxPublisher, source: str, provider: str):
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

        if provider == "ibkr":
            # Emit bid, ask, and last as separate ticks
            entries = []
            if ticker.bid and ticker.bid > 0:
                entries.append(("bid", ticker.bid, int(ticker.bidSize or 0)))
            if ticker.ask and ticker.ask > 0:
                entries.append(("ask", ticker.ask, int(ticker.askSize or 0)))
            if ticker.last and ticker.last > 0:
                entries.append(("trade", ticker.last, int(ticker.lastSize or 0)))
        else:
            # Alpaca trade object
            tick_time = ticker.timestamp if hasattr(ticker, "timestamp") else tick_time
            if tick_time.tzinfo is None:
                tick_time = tick_time.replace(tzinfo=timezone.utc)
            entries = [("trade", float(ticker.price), int(ticker.size))]

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

    provider = config.provider.lower()

    logger.info("=" * 60)
    logger.info("  AUSPEX — Market Data Collection")
    logger.info("  Astartes Primaris Imperium")
    logger.info("=" * 60)
    logger.info("Provider: %s", provider)
    logger.info("Symbols: %s", ", ".join(config.data.symbols))
    logger.info("Timeframes: %s", ", ".join(config.data.bar_timeframes))
    logger.info("Backfill: %d days", config.data.backfill_days)
    logger.info("Ticks: %s | Bars: %s", config.data.stream_ticks, config.data.stream_bars)

    # ── Components ──
    writer = LibrariumWriter(config)
    vox = VoxPublisher(config)

    if provider == "ibkr":
        from ib_insync import util as ib_util
        from .ibkr import IBKRConnector

        ib_util.patchAsyncio()
        connector = IBKRConnector(config)
        source = config.data.source
    elif provider == "alpaca":
        from .alpaca import AlpacaConnector

        connector = AlpacaConnector(config)
        source = config.alpaca.source
    else:
        logger.error("Unknown provider '%s'. Use 'ibkr' or 'alpaca'.", provider)
        return

    # ── Health server ──
    register_health(connector, writer, vox)
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

    logger.info("Connecting to %s...", provider.upper())
    await connector.connect()

    # ── Resolve contracts (IBKR only) ──
    if provider == "ibkr":
        await connector.resolve_contracts()
        if connector.contract_count == 0:
            logger.error("No contracts resolved. Check AUSPEX_SYMBOLS configuration.")
            await shutdown(connector, writer, vox)
            return

    # ── Wire up data routing ──
    connector.on_bar = make_bar_handler(writer, vox, source, provider)
    connector.on_realtime_bar = make_realtime_bar_handler(writer, vox, source, provider)
    connector.on_tick = make_tick_handler(writer, vox, source, provider)

    # ── Backfill ──
    logger.info("Starting historical backfill...")
    total_bars = await connector.backfill_bars()
    logger.info("Backfill complete: %d bars queued for writing", total_bars)

    # Flush the backfill data before starting streaming
    logger.info("Flushing backfill data...")
    await asyncio.sleep(config.librarium.flush_interval + 1)

    # ── Stream ──
    logger.info("Starting real-time streams...")
    await connector.start_streaming()

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
    await shutdown(connector, writer, vox)


async def shutdown(connector, writer: LibrariumWriter, vox: VoxPublisher) -> None:
    """Gracefully shut down all components."""
    logger.info("Shutting down Auspex...")
    await connector.disconnect()
    await writer.stop()
    await vox.disconnect()
    logger.info("Auspex offline. The Emperor protects.")


if __name__ == "__main__":
    asyncio.run(main())

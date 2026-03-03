"""Vox publisher — emits market data events to NATS JetStream.

Publishes on subjects following the Astartes Primaris channel taxonomy:
    market.{symbol}.bar.{timeframe}  — OHLCV bar events
    market.{symbol}.tick             — Tick events (bid/ask/trade)
"""

from __future__ import annotations

import json
import logging
import time
from datetime import datetime, timezone
from typing import TYPE_CHECKING
from uuid import uuid4

import nats
from nats.js.api import StreamConfig, RetentionPolicy, StorageType

if TYPE_CHECKING:
    from nats.aio.client import Client as NATSClient
    from nats.js import JetStreamContext
    from .config import Config

logger = logging.getLogger("auspex.vox")


class VoxPublisher:
    """Publishes market data events to NATS JetStream."""

    def __init__(self, config: Config):
        self.cfg = config.vox
        self._nc: NATSClient | None = None
        self._js: JetStreamContext | None = None
        self._enabled = config.vox.enabled

        # Stats
        self.messages_published = 0

    async def connect(self) -> None:
        """Connect to NATS and set up JetStream stream."""
        if not self._enabled:
            logger.info("Vox publishing disabled")
            return

        logger.info("Connecting to Vox at %s", self.cfg.url)

        self._nc = await nats.connect(self.cfg.url)
        self._js = self._nc.jetstream()

        # Ensure the MARKET stream exists
        try:
            await self._js.find_stream_name_by_subject("market.>")
            logger.info("Vox MARKET stream found")
        except nats.js.errors.NotFoundError:
            logger.info("Creating Vox MARKET stream...")
            await self._js.add_stream(
                StreamConfig(
                    name=self.cfg.stream,
                    subjects=["market.>"],
                    retention=RetentionPolicy.LIMITS,
                    max_age=7 * 24 * 60 * 60 * 1_000_000_000,  # 7 days in nanoseconds
                    storage=StorageType.FILE,
                )
            )
            logger.info("Vox MARKET stream created")

    async def disconnect(self) -> None:
        """Close NATS connection."""
        if self._nc:
            await self._nc.close()
            logger.info("Vox connection closed (%d messages published)", self.messages_published)

    async def publish_bar(
        self,
        symbol: str,
        timeframe: str,
        time_: datetime,
        open_: float,
        high: float,
        low: float,
        close: float,
        volume: int,
    ) -> None:
        """Publish a bar event to market.{symbol}.bar.{timeframe}."""
        if not self._enabled or not self._js:
            return

        subject = f"market.{symbol.upper()}.bar.{timeframe}"
        payload = {
            "id": str(uuid4()),
            "symbol": symbol,
            "timeframe": timeframe,
            "time": time_.isoformat(),
            "open": open_,
            "high": high,
            "low": low,
            "close": close,
            "volume": volume,
            "source": "auspex",
            "published_at": datetime.now(timezone.utc).isoformat(),
        }

        await self._js.publish(subject, json.dumps(payload).encode())
        self.messages_published += 1

    async def publish_tick(
        self,
        symbol: str,
        price: float,
        size: int,
        side: str,
        time_: datetime,
    ) -> None:
        """Publish a tick event to market.{symbol}.tick."""
        if not self._enabled or not self._js:
            return

        subject = f"market.{symbol.upper()}.tick"
        payload = {
            "id": str(uuid4()),
            "symbol": symbol,
            "price": price,
            "size": size,
            "side": side,
            "time": time_.isoformat(),
            "source": "auspex",
            "published_at": datetime.now(timezone.utc).isoformat(),
        }

        await self._js.publish(subject, json.dumps(payload).encode())
        self.messages_published += 1

    @property
    def connected(self) -> bool:
        if not self._enabled:
            return True  # disabled is "ok"
        return self._nc is not None and self._nc.is_connected

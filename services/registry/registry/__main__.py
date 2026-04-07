"""Registry — Strategy Marketplace for Astartes Primaris.

Serves encrypted strategy bundles, manages client subscriptions,
tracks P&L, calculates billing, and monitors client integrity.

Usage:
    python -m registry
"""

import logging
import os

import uvicorn

from .config import Config

logging.basicConfig(
    level=getattr(logging, Config.registry.log_level.upper(), logging.INFO),
    format="%(asctime)s [registry] %(levelname)s: %(message)s",
)
logger = logging.getLogger("registry")


def main():
    logger.info("╔═══════════════════════════════════╗")
    logger.info("║  REGISTRY — Strategy Marketplace   ║")
    logger.info("║  Astartes Primaris Imperium        ║")
    logger.info("╚═══════════════════════════════════╝")

    port = Config.registry.port
    uvicorn.run(
        "registry.server:app",
        host="0.0.0.0",
        port=port,
        log_level=Config.registry.log_level,
    )


if __name__ == "__main__":
    main()

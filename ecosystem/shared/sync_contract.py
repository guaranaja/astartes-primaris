"""
Astartes Ecosystem — Engine → Primaris Sync Contract

ECOSYSTEM: sync-contract

Typed dataclasses for all engine → Primaris communication.
Every engine (futures, equities, crypto) imports these.
NEVER use raw dicts at API boundaries — always typed dataclasses.

Lesson learned: astartes-futures slippage bug (Apr 7 2026) was caused by
returning dicts instead of typed objects, silently dropping fill prices.
"""

from __future__ import annotations

from dataclasses import dataclass, field, asdict
from datetime import datetime
from enum import Enum
from typing import Optional


class InstrumentType(str, Enum):
    FUTURE = "future"
    EQUITY = "equity"
    OPTION = "option"
    CRYPTO = "crypto"


class MarineStatus(str, Enum):
    DORMANT = "dormant"
    WAKING = "waking"
    ORIENTING = "orienting"
    DECIDING = "deciding"
    ACTING = "acting"
    REPORTING = "reporting"
    SLEEPING = "sleeping"
    FAILED = "failed"
    DISABLED = "disabled"


# ── Position Sync ─────────────────────────────────────────


@dataclass
class PositionSync:
    """Current open position snapshot."""
    marine_id: str
    broker_account_id: str
    symbol: str
    quantity: float
    average_price: float
    unrealized_pnl: float = 0.0
    realized_pnl: float = 0.0
    instrument_type: str = InstrumentType.FUTURE
    # Options-specific
    expiration: Optional[str] = None
    strike: Optional[float] = None
    option_type: Optional[str] = None  # "C" or "P"
    # Greeks (options only)
    delta: Optional[float] = None
    theta: Optional[float] = None
    gamma: Optional[float] = None
    vega: Optional[float] = None
    iv: Optional[float] = None

    def to_dict(self) -> dict:
        d = asdict(self)
        return {k: v for k, v in d.items() if v is not None}


# ── Trade Sync ────────────────────────────────────────────


@dataclass
class TradeSync:
    """Completed trade record."""
    id: str
    marine_id: str
    broker_account_id: str
    symbol: str
    side: str             # 'buy','sell','short','cover'
    quantity: float
    entry_price: float
    exit_price: float
    entry_time: str       # ISO 8601
    exit_time: str        # ISO 8601
    pnl: float
    fees: float = 0.0
    duration_ms: int = 0
    instrument_type: str = InstrumentType.FUTURE
    metadata: dict = field(default_factory=dict)
    # Options-specific
    underlying: Optional[str] = None
    expiration: Optional[str] = None
    strike: Optional[float] = None
    option_type: Optional[str] = None

    def to_dict(self) -> dict:
        d = asdict(self)
        return {k: v for k, v in d.items() if v is not None}


# ── Account Snapshot ──────────────────────────────────────


@dataclass
class AccountSnapshot:
    """Full account state at a point in time."""
    broker_account_id: str
    name: str
    broker: str
    account_type: str     # 'prop','personal','paper'
    balance: float
    initial_balance: float
    total_pnl: float
    daily_pnl: float
    profit_split: float = 0.90
    status: str = "active"
    instruments: list = field(default_factory=list)
    # Optional risk fields
    max_loss_limit: Optional[float] = None
    profit_target: Optional[float] = None
    daily_loss_limit: Optional[float] = None
    # Margin (equities)
    margin_used: Optional[float] = None
    buying_power: Optional[float] = None
    # Stats
    total_payouts: float = 0.0
    payout_count: int = 0
    winning_days: int = 0
    total_trading_days: int = 0
    timestamp: str = ""

    def to_dict(self) -> dict:
        d = asdict(self)
        return {k: v for k, v in d.items() if v is not None}


# ── Bar Sync ──────────────────────────────────────────────


@dataclass
class BarSync:
    """Market data bar for Librarium."""
    symbol: str
    timeframe: str       # '30s','1m','5m','1d'
    time: str            # ISO 8601
    open: float
    high: float
    low: float
    close: float
    volume: int
    asset_class: str = "futures"
    vwap: Optional[float] = None
    trade_count: Optional[int] = None

    def to_dict(self) -> dict:
        d = asdict(self)
        return {k: v for k, v in d.items() if v is not None}


# ── Strategy Metrics ──────────────────────────────────────


@dataclass
class StrategyMetrics:
    """Strategy-specific metrics snapshot."""
    marine_id: str
    regime: str = ""           # TREND, RANGE, CHAOS
    confidence: float = 0.0
    model_scores: dict = field(default_factory=dict)
    custom: dict = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)


# ── Full Sync Payload ─────────────────────────────────────


@dataclass
class EngineSyncPayload:
    """Complete sync payload from engine to Primaris.

    Every engine assembles this typed payload. Primarch API
    accepts it as JSON. Never send raw dicts.
    """
    engine_id: str             # "astartes-futures", "astartes-equities"
    fortress_id: str           # "fortress-primus", "fortress-secundus"
    timestamp: str             # ISO 8601

    # Heartbeat
    status: str = MarineStatus.DORMANT
    uptime_seconds: int = 0

    # Data (only non-empty lists are sent)
    positions: list[PositionSync] = field(default_factory=list)
    trades: list[TradeSync] = field(default_factory=list)
    accounts: list[AccountSnapshot] = field(default_factory=list)
    bars: list[BarSync] = field(default_factory=list)
    metrics: list[StrategyMetrics] = field(default_factory=list)

    def to_dict(self) -> dict:
        d = {
            "engine_id": self.engine_id,
            "fortress_id": self.fortress_id,
            "timestamp": self.timestamp,
            "status": self.status,
            "uptime_seconds": self.uptime_seconds,
        }
        if self.positions:
            d["positions"] = [p.to_dict() for p in self.positions]
        if self.trades:
            d["trades"] = [t.to_dict() for t in self.trades]
        if self.accounts:
            d["accounts"] = [a.to_dict() for a in self.accounts]
        if self.bars:
            d["bars"] = [b.to_dict() for b in self.bars]
        if self.metrics:
            d["metrics"] = [m.to_dict() for m in self.metrics]
        return d

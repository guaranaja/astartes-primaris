"""Auspex configuration — loaded from environment variables."""

import os


def _env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


def _env_int(key: str, default: int) -> int:
    v = os.environ.get(key)
    return int(v) if v else default


def _env_float(key: str, default: float) -> float:
    v = os.environ.get(key)
    return float(v) if v else default


def _env_bool(key: str, default: bool = False) -> bool:
    v = os.environ.get(key, "").lower()
    if v in ("1", "true", "yes"):
        return True
    if v in ("0", "false", "no"):
        return False
    return default


def _env_list(key: str, default: str = "") -> list[str]:
    raw = os.environ.get(key, default)
    return [s.strip() for s in raw.split(",") if s.strip()]


class IBKRConfig:
    """TWS / IB Gateway connection settings."""

    host: str = _env("IBKR_HOST", "127.0.0.1")
    port: int = _env_int("IBKR_PORT", 4002)  # 4001=TWS live, 4002=Gateway live, 7497=TWS paper, 7496=Gateway paper
    client_id: int = _env_int("IBKR_CLIENT_ID", 10)
    readonly: bool = _env_bool("IBKR_READONLY", True)
    account: str = _env("IBKR_ACCOUNT", "")
    timeout: int = _env_int("IBKR_TIMEOUT", 30)


class AlpacaConfig:
    """Alpaca Markets data API settings."""

    api_key: str = _env("ALPACA_API_KEY", "")
    secret_key: str = _env("ALPACA_SECRET_KEY", "")
    feed: str = _env("ALPACA_FEED", "iex")  # iex or sip for stocks, us for crypto
    source: str = _env("ALPACA_DATA_SOURCE", "alpaca")


class DataConfig:
    """What data to collect and how."""

    # Symbols to track
    # IBKR format: "SYMBOL:EXCHANGE:SECTYPE:CURRENCY" (e.g. "ES:CME:FUT:USD")
    # Alpaca format: plain symbols (e.g. "SPY", "AAPL", "BTC/USD")
    symbols: list[str] = _env_list("AUSPEX_SYMBOLS", "ES:CME:FUT:USD,NQ:CME:FUT:USD,MES:CME:FUT:USD,MNQ:CME:FUT:USD")

    # Bar timeframes to collect
    bar_timeframes: list[str] = _env_list("AUSPEX_BAR_TIMEFRAMES", "1 min,5 mins,15 mins,1 hour,1 day")

    # How many days of historical bars to backfill on startup
    backfill_days: int = _env_int("AUSPEX_BACKFILL_DAYS", 5)

    # Enable real-time tick streaming
    stream_ticks: bool = _env_bool("AUSPEX_STREAM_TICKS", True)

    # Enable real-time bar streaming (5-second bars from IBKR)
    stream_bars: bool = _env_bool("AUSPEX_STREAM_BARS", True)

    # Source identifier for dedup in Librarium
    source: str = _env("AUSPEX_SOURCE", "ibkr")


class LibrariumConfig:
    """TimescaleDB connection settings."""

    host: str = _env("LIBRARIUM_HOST", "127.0.0.1")
    port: int = _env_int("LIBRARIUM_PORT", 5432)
    database: str = _env("LIBRARIUM_DB", "librarium")
    user: str = _env("LIBRARIUM_USER", "librarium")
    password: str = _env("LIBRARIUM_PASSWORD", "dev_password")

    # Connection pool
    min_pool: int = _env_int("LIBRARIUM_POOL_MIN", 2)
    max_pool: int = _env_int("LIBRARIUM_POOL_MAX", 10)

    # Batch insert size — accumulate this many rows before flushing
    batch_size: int = _env_int("LIBRARIUM_BATCH_SIZE", 100)
    flush_interval: float = _env_float("LIBRARIUM_FLUSH_INTERVAL", 2.0)

    @property
    def dsn(self) -> str:
        return f"postgresql://{self.user}:{self.password}@{self.host}:{self.port}/{self.database}"


class VoxConfig:
    """NATS JetStream connection settings."""

    url: str = _env("VOX_URL", "nats://127.0.0.1:4222")
    stream: str = _env("VOX_STREAM", "MARKET")
    enabled: bool = _env_bool("VOX_ENABLED", True)


class HealthConfig:
    """Health / metrics HTTP server."""

    port: int = _env_int("AUSPEX_PORT", 8300)


class Config:
    """Top-level Auspex configuration."""

    ibkr = IBKRConfig()
    alpaca = AlpacaConfig()
    data = DataConfig()
    librarium = LibrariumConfig()
    vox = VoxConfig()
    health = HealthConfig()
    log_level: str = _env("AUSPEX_LOG_LEVEL", "info")
    provider: str = _env("AUSPEX_PROVIDER", "alpaca")  # "ibkr" or "alpaca"

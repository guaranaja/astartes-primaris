"""Registry service configuration from environment variables."""

import os


def _env(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


def _env_int(key: str, default: int = 0) -> int:
    return int(os.environ.get(key, str(default)))


def _env_bool(key: str, default: bool = False) -> bool:
    return os.environ.get(key, str(default)).lower() in ("true", "1", "yes")


class RegistryConfig:
    port: int = _env_int("REGISTRY_PORT", 8701)
    log_level: str = _env("REGISTRY_LOG_LEVEL", "info")
    master_token: str = _env("REGISTRY_MASTER_TOKEN")
    bundles_dir: str = _env("REGISTRY_BUNDLES_DIR", "/app/data/published_bundles")
    gcs_bucket: str = _env("REGISTRY_GCS_BUCKET")


class LibrariumConfig:
    host: str = _env("LIBRARIUM_HOST", "127.0.0.1")
    port: int = _env_int("LIBRARIUM_PORT", 5432)
    database: str = _env("LIBRARIUM_DB", "librarium")
    user: str = _env("LIBRARIUM_USER", "librarium")
    password: str = _env("LIBRARIUM_PASSWORD", "dev_password")

    @property
    def dsn(self) -> str:
        # Cloud SQL uses DATABASE_URL if set, otherwise build from parts
        explicit = _env("DATABASE_URL")
        if explicit:
            return explicit
        return f"postgresql://{self.user}:{self.password}@{self.host}:{self.port}/{self.database}"


class DiscordConfig:
    bot_token: str = _env("DISCORD_BOT_TOKEN")
    channel_marketplace: str = _env("DISCORD_CHANNEL_MARKETPLACE")
    channel_alerts: str = _env("DISCORD_CHANNEL_MARKETPLACE_ALERTS")


class IntegrityConfig:
    expected_hashes: str = _env("EXPECTED_CLIENT_HASHES", "{}")
    hmac_secret: str = _env("PNL_REPORT_HMAC_SECRET")


class StripeConfig:
    enabled: bool = _env_bool("STRIPE_ENABLED", False)
    webhook_secret: str = _env("STRIPE_WEBHOOK_SECRET")


class Config:
    registry = RegistryConfig()
    librarium = LibrariumConfig()
    discord = DiscordConfig()
    integrity = IntegrityConfig()
    stripe = StripeConfig()

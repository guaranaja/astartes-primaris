-- Astartes Primaris — Librarium Initial Schema
-- TimescaleDB + PostgreSQL schema for market data and platform state.

-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ─── MARKET DATA (Time-Series) ─────────────────────────────

CREATE TABLE market_bars (
    time        TIMESTAMPTZ NOT NULL,
    symbol      TEXT        NOT NULL,
    timeframe   TEXT        NOT NULL,  -- '1s','1m','5m','15m','1h','1d'
    source      TEXT        NOT NULL,  -- data source for dedup
    open        DOUBLE PRECISION NOT NULL,
    high        DOUBLE PRECISION NOT NULL,
    low         DOUBLE PRECISION NOT NULL,
    close       DOUBLE PRECISION NOT NULL,
    volume      BIGINT NOT NULL,
    vwap        DOUBLE PRECISION,
    trade_count INTEGER
);

SELECT create_hypertable('market_bars', 'time');

CREATE UNIQUE INDEX idx_bars_dedup
    ON market_bars (symbol, timeframe, time, source);

CREATE INDEX idx_bars_symbol_time
    ON market_bars (symbol, timeframe, time DESC);

-- ─── TICK DATA ─────────────────────────────────────────────

CREATE TABLE market_ticks (
    time    TIMESTAMPTZ NOT NULL,
    symbol  TEXT        NOT NULL,
    source  TEXT        NOT NULL,
    price   DOUBLE PRECISION NOT NULL,
    size    BIGINT NOT NULL,
    side    TEXT  -- 'bid','ask','trade'
);

SELECT create_hypertable('market_ticks', 'time');

-- ─── HIERARCHY ─────────────────────────────────────────────

CREATE TABLE fortresses (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    asset_class TEXT NOT NULL,
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE companies (
    id           TEXT PRIMARY KEY,
    fortress_id  TEXT NOT NULL REFERENCES fortresses(id),
    name         TEXT NOT NULL,
    type         TEXT NOT NULL,  -- 'veteran','battle','reserve','scout'
    risk_limits  JSONB DEFAULT '{}',
    created_at   TIMESTAMPTZ DEFAULT now(),
    updated_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE marines (
    id                 TEXT PRIMARY KEY,
    company_id         TEXT NOT NULL REFERENCES companies(id),
    name               TEXT NOT NULL,
    strategy_name      TEXT NOT NULL,
    strategy_version   TEXT NOT NULL,
    broker_account_id  TEXT,
    status             TEXT NOT NULL DEFAULT 'dormant',
    schedule           JSONB DEFAULT '{}',
    parameters         JSONB DEFAULT '{}',
    resources          JSONB DEFAULT '{}',
    created_at         TIMESTAMPTZ DEFAULT now(),
    updated_at         TIMESTAMPTZ DEFAULT now()
);

-- ─── ORDERS & POSITIONS ────────────────────────────────────

CREATE TABLE orders (
    id                 TEXT PRIMARY KEY,
    marine_id          TEXT NOT NULL REFERENCES marines(id),
    broker_account_id  TEXT NOT NULL,
    symbol             TEXT NOT NULL,
    side               TEXT NOT NULL,      -- 'buy','sell','short','cover'
    order_type         TEXT NOT NULL,      -- 'market','limit','stop','stop_limit'
    quantity           DOUBLE PRECISION NOT NULL,
    limit_price        DOUBLE PRECISION,
    stop_price         DOUBLE PRECISION,
    status             TEXT NOT NULL DEFAULT 'pending',
    filled_quantity    DOUBLE PRECISION DEFAULT 0,
    filled_price       DOUBLE PRECISION,
    broker_order_id    TEXT,
    created_at         TIMESTAMPTZ DEFAULT now(),
    updated_at         TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE positions (
    marine_id          TEXT NOT NULL REFERENCES marines(id),
    broker_account_id  TEXT NOT NULL,
    symbol             TEXT NOT NULL,
    quantity           DOUBLE PRECISION NOT NULL DEFAULT 0,
    average_price      DOUBLE PRECISION NOT NULL DEFAULT 0,
    unrealized_pnl     DOUBLE PRECISION DEFAULT 0,
    realized_pnl       DOUBLE PRECISION DEFAULT 0,
    updated_at         TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (marine_id, broker_account_id, symbol)
);

-- ─── STRATEGY SIGNALS ──────────────────────────────────────

CREATE TABLE signals (
    id          TEXT PRIMARY KEY,
    marine_id   TEXT NOT NULL REFERENCES marines(id),
    type        TEXT NOT NULL,          -- 'buy','sell','short','cover'
    symbol      TEXT NOT NULL,
    quantity    DOUBLE PRECISION,
    price       DOUBLE PRECISION,
    order_type  TEXT,
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT now()
);

SELECT create_hypertable('signals', 'created_at');

-- ─── MARINE EXECUTION LOG ──────────────────────────────────

CREATE TABLE marine_cycles (
    id           TEXT PRIMARY KEY,
    marine_id    TEXT NOT NULL REFERENCES marines(id),
    wake_at      TIMESTAMPTZ NOT NULL,
    sleep_at     TIMESTAMPTZ,
    status       TEXT NOT NULL,         -- 'completed','failed','timeout'
    signals_generated INTEGER DEFAULT 0,
    orders_submitted  INTEGER DEFAULT 0,
    duration_ms       INTEGER,
    error_message     TEXT,
    metadata          JSONB DEFAULT '{}'
);

SELECT create_hypertable('marine_cycles', 'wake_at');

-- ─── FORGE JOBS ────────────────────────────────────────────

CREATE TABLE forge_jobs (
    id                TEXT PRIMARY KEY,
    type              TEXT NOT NULL,
    strategy_name     TEXT NOT NULL,
    strategy_version  TEXT NOT NULL,
    config            JSONB NOT NULL,
    status            TEXT NOT NULL DEFAULT 'queued',
    result            JSONB,
    submitted_at      TIMESTAMPTZ DEFAULT now(),
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    error_message     TEXT
);

-- ─── AUDIT LOG ─────────────────────────────────────────────

CREATE TABLE audit_log (
    time        TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    details     JSONB DEFAULT '{}'
);

SELECT create_hypertable('audit_log', 'time');

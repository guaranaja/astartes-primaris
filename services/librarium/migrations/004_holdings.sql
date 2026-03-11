-- 004_holdings.sql
-- Manual stock holdings for wheel strategy analysis.

CREATE TABLE IF NOT EXISTS holdings (
    id          TEXT PRIMARY KEY,
    symbol      TEXT NOT NULL,
    quantity    DOUBLE PRECISION NOT NULL DEFAULT 0,
    avg_cost    DOUBLE PRECISION NOT NULL DEFAULT 0,
    acquired_at TEXT,
    notes       TEXT,
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_holdings_symbol ON holdings (symbol);

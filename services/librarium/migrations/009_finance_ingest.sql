-- Astartes Primaris — Finance Ingest Schema
-- Caches Firefly III + Monarch Money transactions locally so dashboard queries
-- (trends, drill-down, unified activity) are fast and work offline.

-- ─── FIREFLY TRANSACTIONS ────────────────────────────────────
CREATE TABLE IF NOT EXISTS ff_transactions (
    id              TEXT PRIMARY KEY,            -- Firefly transaction ID
    journal_id      TEXT,                        -- parent transaction journal
    txn_type        TEXT NOT NULL,               -- deposit | withdrawal | transfer
    date            DATE NOT NULL,
    amount          DOUBLE PRECISION NOT NULL,   -- positive magnitude
    currency        TEXT NOT NULL DEFAULT 'USD',
    description     TEXT,
    category        TEXT,
    budget_name     TEXT,
    bill_id         TEXT,
    source_account  TEXT,
    dest_account    TEXT,
    tags            TEXT[] DEFAULT '{}',
    notes           TEXT,
    external_url    TEXT,
    raw             JSONB,                       -- full Firefly record for audit
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ff_txn_date_idx      ON ff_transactions(date DESC);
CREATE INDEX IF NOT EXISTS ff_txn_category_idx  ON ff_transactions(category);
CREATE INDEX IF NOT EXISTS ff_txn_budget_idx    ON ff_transactions(budget_name);
CREATE INDEX IF NOT EXISTS ff_txn_type_idx      ON ff_transactions(txn_type, date DESC);

-- ─── MONARCH TRANSACTIONS ────────────────────────────────────
CREATE TABLE IF NOT EXISTS mn_transactions (
    id           TEXT PRIMARY KEY,     -- Monarch transaction ID
    date         DATE NOT NULL,
    amount       DOUBLE PRECISION NOT NULL, -- signed (negative = expense)
    merchant     TEXT,
    category     TEXT,
    account      TEXT,
    notes        TEXT,
    is_recurring BOOLEAN NOT NULL DEFAULT FALSE,
    raw          JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS mn_txn_date_idx     ON mn_transactions(date DESC);
CREATE INDEX IF NOT EXISTS mn_txn_category_idx ON mn_transactions(category);

-- ─── SYNC STATE ──────────────────────────────────────────────
-- One row per source so the worker knows how far back to look and can
-- report health in /status.
CREATE TABLE IF NOT EXISTS finance_sync_state (
    source          TEXT PRIMARY KEY,      -- 'firefly' | 'monarch'
    last_synced_at  TIMESTAMPTZ,
    last_ok_at      TIMESTAMPTZ,
    last_error      TEXT,
    last_count      INTEGER NOT NULL DEFAULT 0,
    window_days     INTEGER NOT NULL DEFAULT 180,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 007_multi_asset.sql
-- Multi-asset extensions: instrument type, options fields, Greeks, portfolio views.
-- Backward-compatible: all new columns have defaults, no existing columns altered.

-- ─── EXTEND ORDERS FOR OPTIONS ─────────────────────────────

ALTER TABLE orders ADD COLUMN IF NOT EXISTS instrument_type TEXT DEFAULT 'future';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS underlying TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS expiration DATE;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS strike NUMERIC;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS option_type TEXT;     -- 'C','P'
ALTER TABLE orders ADD COLUMN IF NOT EXISTS legs JSONB;           -- multi-leg option orders

-- ─── EXTEND POSITIONS FOR OPTIONS + GREEKS ─────────────────

ALTER TABLE positions ADD COLUMN IF NOT EXISTS instrument_type TEXT DEFAULT 'future';
ALTER TABLE positions ADD COLUMN IF NOT EXISTS expiration DATE;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS strike NUMERIC;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS option_type TEXT;  -- 'C','P'
ALTER TABLE positions ADD COLUMN IF NOT EXISTS delta DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS theta DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS gamma DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS vega DOUBLE PRECISION;
ALTER TABLE positions ADD COLUMN IF NOT EXISTS iv DOUBLE PRECISION;

-- ─── EXTEND SIGNALS FOR OPTIONS ────────────────────────────

ALTER TABLE signals ADD COLUMN IF NOT EXISTS instrument_type TEXT DEFAULT 'future';
ALTER TABLE signals ADD COLUMN IF NOT EXISTS legs JSONB;

-- ─── ADD ASSET CLASS TO MARKET BARS ────────────────────────

ALTER TABLE market_bars ADD COLUMN IF NOT EXISTS asset_class TEXT DEFAULT 'futures';

-- ─── BROKER ACCOUNTS (extend trading_accounts if exists) ───

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'trading_accounts') THEN
        EXECUTE 'ALTER TABLE trading_accounts ADD COLUMN IF NOT EXISTS fortress_id TEXT';
        EXECUTE 'ALTER TABLE trading_accounts ADD COLUMN IF NOT EXISTS marine_id TEXT';
    END IF;
END $$;

-- ─── WHEEL MANAGER ─────────────────────────────────────────

CREATE TABLE IF NOT EXISTS wheel_cycles (
    id                       TEXT PRIMARY KEY,
    underlying               TEXT NOT NULL,
    status                   TEXT NOT NULL DEFAULT 'selling_puts',
        -- 'selling_puts','assigned','selling_calls','called_away','closed'
    mode                     TEXT NOT NULL DEFAULT 'manual',
        -- 'manual','automated'
    marine_id                TEXT,           -- NULL for manual, links to Phalanx when automated
    broker                   TEXT,           -- broker name (even unsupported for manual tracking)
    started_at               TIMESTAMPTZ DEFAULT now(),
    closed_at                TIMESTAMPTZ,
    total_premium_collected  DOUBLE PRECISION DEFAULT 0,
    cost_basis               DOUBLE PRECISION,   -- assignment price
    shares_held              INTEGER DEFAULT 0,
    metadata                 JSONB DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS wheel_legs (
    id           TEXT PRIMARY KEY,
    cycle_id     TEXT NOT NULL REFERENCES wheel_cycles(id),
    leg_type     TEXT NOT NULL,
        -- 'csp','covered_call','assignment','called_away','roll','close'
    symbol       TEXT NOT NULL,              -- option symbol or underlying
    strike       NUMERIC,
    expiration   DATE,
    option_type  TEXT,                       -- 'P','C'
    quantity     INTEGER,
    premium      DOUBLE PRECISION,           -- credit (+) or debit (-)
    fill_price   DOUBLE PRECISION,
    opened_at    TIMESTAMPTZ DEFAULT now(),
    closed_at    TIMESTAMPTZ,
    status       TEXT DEFAULT 'open',
        -- 'open','expired','assigned','exercised','closed','rolled'
    notes        TEXT
);

CREATE INDEX IF NOT EXISTS idx_wheel_legs_cycle ON wheel_legs (cycle_id);
CREATE INDEX IF NOT EXISTS idx_wheel_cycles_underlying ON wheel_cycles (underlying, status);

-- ─── PAYOUT ALLOCATIONS (ledger for manual tracking) ───────

CREATE TABLE IF NOT EXISTS payout_allocations (
    id          TEXT PRIMARY KEY,
    payout_id   TEXT,                        -- references payouts(id) if exists
    category    TEXT NOT NULL,               -- 'family','bills','savings','trading_capital','taxes'
    amount      DOUBLE PRECISION NOT NULL,
    note        TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payout_alloc_category
    ON payout_allocations (category, created_at DESC);

-- ─── PORTFOLIO VIEWS ───────────────────────────────────────

CREATE OR REPLACE VIEW portfolio_summary AS
SELECT
    f.id           AS fortress_id,
    f.asset_class,
    c.id           AS company_id,
    m.id           AS marine_id,
    m.strategy_name,
    p.symbol,
    p.instrument_type,
    p.quantity,
    p.average_price,
    p.unrealized_pnl,
    p.realized_pnl,
    p.delta, p.theta, p.gamma, p.vega
FROM positions p
JOIN marines m   ON p.marine_id = m.id
JOIN companies c ON m.company_id = c.id
JOIN fortresses f ON c.fortress_id = f.id
WHERE p.quantity != 0;

CREATE OR REPLACE VIEW imperium_pnl AS
SELECT
    f.id           AS fortress_id,
    f.name         AS fortress_name,
    f.asset_class,
    SUM(p.realized_pnl)   AS total_realized,
    SUM(p.unrealized_pnl)  AS total_unrealized,
    COUNT(DISTINCT m.id)   AS active_marines
FROM positions p
JOIN marines m   ON p.marine_id = m.id
JOIN companies c ON m.company_id = c.id
JOIN fortresses f ON c.fortress_id = f.id
GROUP BY f.id, f.name, f.asset_class;

-- Astartes Primaris — Wheel Advisor Schema
-- Supports the Phalanx strategy: cash-secured puts → assignment → covered
-- calls → called away → repeat. The advisor scans a watchlist (for new CSPs)
-- and current holdings (for CCs), scores candidates via a rules engine, and
-- optionally augments the shortlist with a Claude review.

-- ─── WATCHLIST ───────────────────────────────────────────────
-- Tickers we're willing to sell puts against (and potentially own). Rules
-- override defaults per-ticker; unspecified values inherit wheel_config.
CREATE TABLE IF NOT EXISTS wheel_watchlist (
    symbol               TEXT PRIMARY KEY,
    max_position_value   DOUBLE PRECISION,   -- cap on shares worth we're willing to be assigned
    target_put_delta     DOUBLE PRECISION,   -- override default (e.g. 0.20)
    target_call_delta    DOUBLE PRECISION,   -- override default (e.g. 0.20)
    min_premium_yield    DOUBLE PRECISION,   -- override default annualized yield threshold
    notes                TEXT,
    active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ─── GLOBAL CONFIG ───────────────────────────────────────────
-- Single-row (id='default') config used when a watchlist entry doesn't
-- override. Tunable from the UI.
CREATE TABLE IF NOT EXISTS wheel_config (
    id                   TEXT PRIMARY KEY DEFAULT 'default',
    default_put_delta    DOUBLE PRECISION NOT NULL DEFAULT 0.20,
    default_call_delta   DOUBLE PRECISION NOT NULL DEFAULT 0.20,
    min_dte              INTEGER NOT NULL DEFAULT 25,
    max_dte              INTEGER NOT NULL DEFAULT 45,
    min_premium_yield    DOUBLE PRECISION NOT NULL DEFAULT 0.20,  -- annualized, 20%+
    profit_take_pct      DOUBLE PRECISION NOT NULL DEFAULT 0.50,  -- close at 50% of max profit
    roll_dte             INTEGER NOT NULL DEFAULT 21,             -- consider rolling at ≤21 DTE
    max_positions        INTEGER NOT NULL DEFAULT 10,             -- total open wheel legs cap
    claude_review        BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed default row if missing.
INSERT INTO wheel_config (id) VALUES ('default') ON CONFLICT DO NOTHING;

-- ─── RECOMMENDATIONS ─────────────────────────────────────────
-- One row per candidate trade the advisor surfaces on a run. Ephemeral —
-- superseded by newer runs; old rows are soft-expired after N days.
-- action: csp_open | cc_open | csp_close | cc_close | csp_roll | cc_roll
-- status: fresh | taken | dismissed | expired
CREATE TABLE IF NOT EXISTS wheel_recommendations (
    id                   TEXT PRIMARY KEY,
    run_id               TEXT NOT NULL,                 -- groups recs from one scan
    action               TEXT NOT NULL,
    symbol               TEXT NOT NULL,
    underlying_price     DOUBLE PRECISION,
    option_type          TEXT,                          -- P | C
    strike               DOUBLE PRECISION,
    expiration           TEXT,                          -- YYYY-MM-DD
    dte                  INTEGER,
    delta                DOUBLE PRECISION,
    bid                  DOUBLE PRECISION,
    ask                  DOUBLE PRECISION,
    mid                  DOUBLE PRECISION,
    premium              DOUBLE PRECISION,              -- credit received per contract
    collateral           DOUBLE PRECISION,              -- capital tied up (strike * 100 for CSP)
    annualized_yield     DOUBLE PRECISION,              -- (premium/collateral) * (365/dte)
    iv_rank              DOUBLE PRECISION,
    score                DOUBLE PRECISION,              -- composite rank
    rules_rationale      TEXT,
    review_note          TEXT,                          -- Claude commentary (null = not reviewed)
    review_score         DOUBLE PRECISION,              -- Claude 0..1 confidence
    status               TEXT NOT NULL DEFAULT 'fresh',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    taken_at             TIMESTAMPTZ,
    dismissed_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS wheel_rec_run_idx    ON wheel_recommendations(run_id);
CREATE INDEX IF NOT EXISTS wheel_rec_status_idx ON wheel_recommendations(status, created_at DESC);
CREATE INDEX IF NOT EXISTS wheel_rec_symbol_idx ON wheel_recommendations(symbol);

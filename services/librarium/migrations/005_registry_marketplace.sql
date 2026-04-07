-- Registry tables for the Astartes strategy marketplace.
-- Applied to the existing 'futures' database alongside trading tables.

CREATE TABLE IF NOT EXISTS registry_clients (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    contact_email   VARCHAR(200),
    api_key_hash    VARCHAR(200) NOT NULL,      -- bcrypt hash of client API key
    public_key_pem  TEXT NOT NULL,               -- X25519 public key (PEM)
    status          VARCHAR(20) DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED', 'REVOKED')),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS registry_strategies (
    id              VARCHAR(100) PRIMARY KEY,    -- e.g. "eversor_es_150k_30s"
    display_name    VARCHAR(200) NOT NULL,
    description     TEXT,
    tier            VARCHAR(20) DEFAULT 'standard',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS registry_versions (
    id              BIGSERIAL PRIMARY KEY,
    strategy_id     VARCHAR(100) NOT NULL REFERENCES registry_strategies(id),
    version         VARCHAR(50) NOT NULL,
    changelog       TEXT,
    bundle_path     TEXT NOT NULL,               -- local path or GCS URI
    manifest_hash   VARCHAR(128) NOT NULL,       -- SHA-256 of manifest
    bundle_size     BIGINT,
    compiled        BOOLEAN DEFAULT FALSE,
    published_at    TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (strategy_id, version)
);

CREATE TABLE IF NOT EXISTS registry_subscriptions (
    client_id       UUID NOT NULL REFERENCES registry_clients(id),
    strategy_id     VARCHAR(100) NOT NULL REFERENCES registry_strategies(id),
    active          BOOLEAN DEFAULT TRUE,
    granted_at      TIMESTAMPTZ DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ,
    PRIMARY KEY (client_id, strategy_id)
);

CREATE TABLE IF NOT EXISTS registry_audit_log (
    id              BIGSERIAL PRIMARY KEY,
    client_id       UUID,
    strategy_id     VARCHAR(100),
    version         VARCHAR(50),
    action          VARCHAR(50) NOT NULL,        -- DOWNLOAD, REGISTER, REVOKE, PUBLISH, GRANT, etc.
    detail          TEXT,
    ip_address      INET,
    timestamp       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_registry_versions_strategy ON registry_versions(strategy_id, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_registry_audit_log_client ON registry_audit_log(client_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_registry_audit_log_strategy ON registry_audit_log(strategy_id, timestamp DESC);

-- ── Billing & P&L Reporting ────────────────────────────────

CREATE TABLE IF NOT EXISTS billing_plans (
    id              VARCHAR(50) PRIMARY KEY,    -- e.g. "eversor_standard"
    strategy_id     VARCHAR(100) REFERENCES registry_strategies(id),
    name            VARCHAR(200) NOT NULL,
    flat_rate       NUMERIC(10,2) NOT NULL,     -- monthly $/mo
    perf_pct        NUMERIC(5,2) DEFAULT 0,     -- % of profits above HWM
    description     TEXT,
    offsets         JSONB DEFAULT '{}',         -- tier-based config offsets for liquidity sandbagging
                                                -- e.g. {"signal_delay_bars": 1, "entry_jitter_ms": [500, 1500],
                                                --       "max_contracts": 5, "entry_threshold_offset": 0.05}
    active          BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS client_billing (
    client_id       UUID NOT NULL REFERENCES registry_clients(id),
    plan_id         VARCHAR(50) NOT NULL REFERENCES billing_plans(id),
    status          VARCHAR(20) DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'SUSPENDED', 'CANCELLED')),
    hwm_equity      NUMERIC(12,2) DEFAULT 0,    -- high-water mark (peak equity for perf fee calc)
    stripe_customer VARCHAR(100),                -- Stripe customer ID (null for F&F manual billing)
    stripe_sub      VARCHAR(100),                -- Stripe subscription ID
    started_at      TIMESTAMPTZ DEFAULT NOW(),
    cancelled_at    TIMESTAMPTZ,
    notes           TEXT,                        -- "Dave's buddy", "paid via Venmo 4/7", etc.
    PRIMARY KEY (client_id, plan_id)
);

CREATE TABLE IF NOT EXISTS pnl_reports (
    id              BIGSERIAL PRIMARY KEY,
    client_id       UUID NOT NULL REFERENCES registry_clients(id),
    strategy_id     VARCHAR(100) NOT NULL,
    trade_date      DATE NOT NULL,
    gross_pnl       NUMERIC(12,2),
    net_pnl         NUMERIC(12,2),              -- after slippage/commissions
    trade_count     INT DEFAULT 0,
    ending_equity   NUMERIC(12,2),
    report_hash     VARCHAR(128),               -- HMAC for tamper detection
    reported_at     TIMESTAMPTZ DEFAULT NOW(),
    verified        BOOLEAN DEFAULT FALSE,       -- cross-checked via broker API
    verified_at     TIMESTAMPTZ,
    verified_pnl    NUMERIC(12,2),              -- broker-verified P&L (if different)
    UNIQUE (client_id, strategy_id, trade_date)
);

CREATE TABLE IF NOT EXISTS billing_periods (
    id              BIGSERIAL PRIMARY KEY,
    client_id       UUID NOT NULL REFERENCES registry_clients(id),
    plan_id         VARCHAR(50) NOT NULL REFERENCES billing_plans(id),
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    flat_fee        NUMERIC(10,2),
    perf_fee        NUMERIC(10,2) DEFAULT 0,
    net_pnl         NUMERIC(12,2),              -- aggregate P&L for period
    hwm_start       NUMERIC(12,2),              -- HWM at period start
    hwm_end         NUMERIC(12,2),              -- HWM at period end
    stripe_invoice  VARCHAR(100),                -- Stripe invoice ID (null for manual)
    status          VARCHAR(20) DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'INVOICED', 'PAID', 'WAIVED')),
    paid_at         TIMESTAMPTZ,
    notes           TEXT,                        -- "paid via Venmo", "waived first month", etc.
    UNIQUE (client_id, plan_id, period_start)
);

CREATE TABLE IF NOT EXISTS client_integrity (
    client_id       UUID NOT NULL REFERENCES registry_clients(id),
    checked_at      TIMESTAMPTZ DEFAULT NOW(),
    file_hashes     JSONB NOT NULL,             -- {"client_runner.py": "a1b2c3d4", ...}
    runtime         VARCHAR(20),                -- Python version
    os_platform     VARCHAR(20),
    status          VARCHAR(20) DEFAULT 'OK' CHECK (status IN ('OK', 'MODIFIED', 'TAMPERED')),
    detail          TEXT,                        -- which files differ
    PRIMARY KEY (client_id)                      -- latest check per client
);

CREATE INDEX IF NOT EXISTS idx_pnl_reports_client_date ON pnl_reports(client_id, trade_date DESC);
CREATE INDEX IF NOT EXISTS idx_pnl_reports_strategy_date ON pnl_reports(strategy_id, trade_date DESC);
CREATE INDEX IF NOT EXISTS idx_billing_periods_client ON billing_periods(client_id, period_start DESC);
CREATE INDEX IF NOT EXISTS idx_client_billing_status ON client_billing(status);

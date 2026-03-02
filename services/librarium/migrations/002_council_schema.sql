-- Astartes Primaris — Council Schema
-- Financial war room: accounts, payouts, goals, expenses, career progression.

-- ─── TRADING ACCOUNTS ────────────────────────────────────────

CREATE TABLE trading_accounts (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    broker          TEXT NOT NULL,
    type            TEXT NOT NULL DEFAULT 'prop',
    account_number  TEXT,
    initial_balance DOUBLE PRECISION NOT NULL DEFAULT 0,
    current_balance DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_pnl       DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_payouts   DOUBLE PRECISION NOT NULL DEFAULT 0,
    payout_count    INTEGER NOT NULL DEFAULT 0,
    profit_split    DOUBLE PRECISION NOT NULL DEFAULT 0.90,
    status          TEXT NOT NULL DEFAULT 'active',
    instruments     TEXT[] DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- ─── PAYOUTS ─────────────────────────────────────────────────

CREATE TABLE payouts (
    id           TEXT PRIMARY KEY,
    account_id   TEXT NOT NULL REFERENCES trading_accounts(id),
    gross_amount DOUBLE PRECISION NOT NULL,
    net_amount   DOUBLE PRECISION NOT NULL,
    destination  TEXT NOT NULL DEFAULT 'bank',
    status       TEXT NOT NULL DEFAULT 'completed',
    requested_at TIMESTAMPTZ DEFAULT now(),
    completed_at TIMESTAMPTZ,
    note         TEXT
);

CREATE INDEX idx_payouts_account ON payouts (account_id, requested_at DESC);

-- ─── GOALS ───────────────────────────────────────────────────

CREATE TABLE goals (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    description    TEXT,
    category       TEXT NOT NULL DEFAULT 'savings',
    target_amount  DOUBLE PRECISION NOT NULL,
    current_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    priority       INTEGER NOT NULL DEFAULT 3,
    target_date    TIMESTAMPTZ,
    status         TEXT NOT NULL DEFAULT 'active',
    icon           TEXT,
    created_at     TIMESTAMPTZ DEFAULT now(),
    updated_at     TIMESTAMPTZ DEFAULT now(),
    completed_at   TIMESTAMPTZ
);

CREATE TABLE goal_contributions (
    id         TEXT PRIMARY KEY,
    goal_id    TEXT NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    amount     DOUBLE PRECISION NOT NULL,
    source     TEXT NOT NULL DEFAULT 'manual',
    payout_id  TEXT,
    note       TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_contributions_goal ON goal_contributions (goal_id, created_at DESC);

-- ─── EXPENSES & BILLING ─────────────────────────────────────

CREATE TABLE expenses (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    category   TEXT NOT NULL DEFAULT 'other',
    amount     DOUBLE PRECISION NOT NULL,
    frequency  TEXT NOT NULL DEFAULT 'monthly',
    due_day    INTEGER DEFAULT 1,
    auto_pay   BOOLEAN DEFAULT false,
    status     TEXT NOT NULL DEFAULT 'active',
    next_due   TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE payments (
    id         TEXT PRIMARY KEY,
    expense_id TEXT NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    amount     DOUBLE PRECISION NOT NULL,
    paid_at    TIMESTAMPTZ DEFAULT now(),
    method     TEXT NOT NULL DEFAULT 'bank',
    note       TEXT
);

CREATE INDEX idx_payments_expense ON payments (expense_id, paid_at DESC);

-- ─── ROADMAP & CAREER ────────────────────────────────────────

CREATE TABLE roadmap (
    id            TEXT PRIMARY KEY DEFAULT 'default',
    current_phase TEXT NOT NULL DEFAULT 'initiate',
    data          JSONB NOT NULL DEFAULT '{}',
    started_at    TIMESTAMPTZ DEFAULT now(),
    updated_at    TIMESTAMPTZ DEFAULT now()
);

-- ─── BUDGET & ALLOCATIONS ────────────────────────────────────

CREATE TABLE budget (
    id   TEXT PRIMARY KEY DEFAULT 'current',
    data JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE allocations (
    category   TEXT PRIMARY KEY,
    percentage DOUBLE PRECISION NOT NULL,
    amount     DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Seed default allocations
INSERT INTO allocations (category, percentage) VALUES
    ('bills', 30),
    ('trading_capital', 35),
    ('taxes', 15),
    ('savings', 10),
    ('personal', 10);

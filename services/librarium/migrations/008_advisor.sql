-- Astartes Primaris — Advisor Schema
-- Claude-powered strategic advisor: threaded conversations about LLC, accounts,
-- debt payoff, hardware, and milestone briefings. All durable in Postgres.

-- ─── ADVISOR THREADS ─────────────────────────────────────────
-- topic: 'chat' (freeform), 'milestone' (triggered by SystemEvent), 'playbook'
-- playbook_key: 'llc_structure', 'account_arch', 'debt_order', 'hardware'
-- status: 'active', 'archived'
CREATE TABLE advisor_threads (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    topic            TEXT NOT NULL,
    playbook_key     TEXT,
    status           TEXT NOT NULL DEFAULT 'active',
    context_snapshot JSONB,
    metadata         JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_message_at  TIMESTAMPTZ
);

CREATE INDEX advisor_threads_updated_idx ON advisor_threads(updated_at DESC);
CREATE INDEX advisor_threads_topic_idx ON advisor_threads(topic, status);

-- ─── ADVISOR MESSAGES ────────────────────────────────────────
-- role: 'user', 'assistant', 'system'
CREATE TABLE advisor_messages (
    id         TEXT PRIMARY KEY,
    thread_id  TEXT NOT NULL REFERENCES advisor_threads(id) ON DELETE CASCADE,
    role       TEXT NOT NULL,
    content    TEXT NOT NULL,
    tool_calls JSONB,
    model      TEXT,
    tokens_in  INTEGER,
    tokens_out INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX advisor_messages_thread_idx ON advisor_messages(thread_id, created_at);

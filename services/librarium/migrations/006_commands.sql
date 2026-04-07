-- 006_commands.sql
-- Engine protocol: command queue for Primarch → engine communication.

CREATE TABLE IF NOT EXISTS commands (
    id            TEXT PRIMARY KEY,
    engine_id     TEXT NOT NULL,
    command       TEXT NOT NULL,
    scope         TEXT NOT NULL,
    params        JSONB,
    status        TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at    TIMESTAMPTZ DEFAULT now(),
    updated_at    TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_commands_engine_status ON commands (engine_id, status);

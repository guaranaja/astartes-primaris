-- Astartes Primaris — Banking Connections Schema
-- Live bank connectivity via provider APIs (Plaid, Teller, SimpleFIN).
-- Access tokens are encrypted at rest with AES-256-GCM using an env-sourced
-- key (PLAID_TOKEN_ENC_KEY). Transactions pulled here are pushed into
-- Firefly III via the existing CreateTransaction flow, so Firefly stays the
-- ledger of record. The existing finance ingest then pulls them back into
-- the local cache for dashboard queries.

CREATE TABLE IF NOT EXISTS bank_connections (
    id                   TEXT PRIMARY KEY,              -- internal id (bc-<nano>)
    provider             TEXT NOT NULL,                 -- 'plaid' | 'teller' | ...
    provider_item_id     TEXT,                          -- Plaid item_id
    institution_id       TEXT,                          -- provider's institution id
    institution_name     TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'active', -- active | error | revoked
    last_error           TEXT,
    -- Delta cursor for Plaid /transactions/sync; null = full refresh next run
    sync_cursor          TEXT,
    -- Encrypted access token (base64 ciphertext || nonce) so sole access is via the runtime
    access_token_ct      TEXT,
    -- Accounts linked under this Item (JSONB array of {id, name, mask, type, subtype, current_balance})
    accounts             JSONB DEFAULT '[]'::jsonb,
    -- Firefly asset account names this connection maps into, keyed by provider account id
    firefly_account_map  JSONB DEFAULT '{}'::jsonb,
    last_synced_at       TIMESTAMPTZ,
    last_ok_at           TIMESTAMPTZ,
    last_txn_count       INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bank_connections_status_idx ON bank_connections(status);
CREATE INDEX IF NOT EXISTS bank_connections_provider_idx ON bank_connections(provider);

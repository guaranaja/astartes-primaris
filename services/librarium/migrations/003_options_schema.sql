-- 003_options_schema.sql
-- Options chain data for wheel strategy analysis.

CREATE TABLE IF NOT EXISTS option_chains (
    time            TIMESTAMPTZ      NOT NULL,
    underlying      TEXT             NOT NULL,
    expiration      DATE             NOT NULL,
    strike          NUMERIC          NOT NULL,
    option_type     TEXT             NOT NULL,  -- 'C' or 'P'
    bid             DOUBLE PRECISION,
    ask             DOUBLE PRECISION,
    mark            DOUBLE PRECISION,
    volume          INTEGER,
    open_interest   INTEGER,
    delta           DOUBLE PRECISION,
    gamma           DOUBLE PRECISION,
    theta           DOUBLE PRECISION,
    vega            DOUBLE PRECISION,
    iv              DOUBLE PRECISION,
    source          TEXT             NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_option_chains_lookup
    ON option_chains (underlying, expiration, strike, option_type, time DESC);

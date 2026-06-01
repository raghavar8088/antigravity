-- ============================================================
-- LEDGER EVENT STORE — Phase 15C
-- PostgreSQL / TimescaleDB DDL
-- Append-only, immutable, tamper-evident.
-- ============================================================

-- ─── Per-aggregate sequence counter ──────────────────────────────────────────
-- Provides collision-free, monotonically increasing per-aggregate sequence
-- numbers even under concurrent writers. Atomically incremented via UPSERT.

CREATE TABLE IF NOT EXISTS ledger_aggregate_sequences (
    aggregate_type TEXT   NOT NULL,
    aggregate_id   TEXT   NOT NULL,
    last_sequence  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (aggregate_type, aggregate_id)
);

-- ─── Immutable event store ────────────────────────────────────────────────────
-- global_sequence   : monotonic global ordering across all events
-- event_id          : random 32-char hex, globally unique
-- schema_version    : allows rolling schema migrations on the payload shape
-- aggregate_type    : ORDER | POSITION | RISK | ACCOUNT | RECONCILIATION | ...
-- aggregate_id      : entity identifier within the aggregate type
-- aggregate_sequence: 1-based position of this event within its aggregate stream
-- event_type        : ORDER_CREATED | POSITION_OPENED | KILL_SWITCH_TRIGGERED | ...
-- account_id        : logical account (btc-paper-1, nifty-paper-1, ...)
-- strategy_id       : originating strategy name for audit trails
-- symbol            : trading symbol (BTC-USD, NIFTY50, ...)
-- correlation_id    : links events across multiple aggregates (e.g. order ↔ position)
-- causation_id      : the event_id that caused this event (for causal tracing)
-- idempotency_key   : prevents duplicate submissions; NULL when not required
-- payload           : JSONB business data; schema defined by event_type
-- payload_hash      : SHA-256 of payload bytes; verified on read to detect corruption
-- source            : originating service / component name
-- created_at        : server-side UTC timestamp; set at write time and immutable

CREATE TABLE IF NOT EXISTS ledger_events (
    global_sequence    BIGSERIAL    PRIMARY KEY,
    event_id           TEXT         NOT NULL UNIQUE,
    schema_version     INT          NOT NULL DEFAULT 1,
    aggregate_type     TEXT         NOT NULL,
    aggregate_id       TEXT         NOT NULL,
    aggregate_sequence BIGINT       NOT NULL,
    event_type         TEXT         NOT NULL,
    account_id         TEXT,
    strategy_id        TEXT,
    symbol             TEXT,
    correlation_id     TEXT,
    causation_id       TEXT,
    idempotency_key    TEXT,
    payload            JSONB        NOT NULL DEFAULT '{}',
    payload_hash       TEXT         NOT NULL,
    source             TEXT         NOT NULL DEFAULT 'unknown',
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- Prevent duplicate event sequence numbers for the same aggregate
    UNIQUE (aggregate_type, aggregate_id, aggregate_sequence)
);

-- Partial unique index enforces idempotency key uniqueness while allowing NULLs.
-- This is the correct approach for optional unique columns in PostgreSQL.
CREATE UNIQUE INDEX IF NOT EXISTS idx_ledger_idempotency
    ON ledger_events (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Fast aggregate replay (primary access pattern)
CREATE INDEX IF NOT EXISTS idx_ledger_aggregate
    ON ledger_events (aggregate_type, aggregate_id, aggregate_sequence ASC);

-- Fast account replay (all events for one account, global order)
CREATE INDEX IF NOT EXISTS idx_ledger_account
    ON ledger_events (account_id, global_sequence ASC)
    WHERE account_id IS NOT NULL;

-- Fast event-type queries (analytics, reconciliation)
CREATE INDEX IF NOT EXISTS idx_ledger_event_type
    ON ledger_events (event_type, global_sequence ASC);

-- Time-range queries for dashboards and audit logs
CREATE INDEX IF NOT EXISTS idx_ledger_created_at
    ON ledger_events (created_at DESC);

-- Symbol-scoped queries (e.g. all BTC events)
CREATE INDEX IF NOT EXISTS idx_ledger_symbol
    ON ledger_events (symbol, global_sequence ASC)
    WHERE symbol IS NOT NULL;

-- Correlation tracing (find all events in one workflow)
CREATE INDEX IF NOT EXISTS idx_ledger_correlation
    ON ledger_events (correlation_id)
    WHERE correlation_id IS NOT NULL;

-- ─── Optional: TimescaleDB hypertable ────────────────────────────────────────
-- Uncomment if TimescaleDB extension is available. Partitions by created_at
-- for efficient time-range queries and automatic chunk compression.
-- Run ONLY after the table is created and before any data is inserted.
--
-- SELECT create_hypertable('ledger_events', 'created_at',
--     chunk_time_interval => INTERVAL '7 days',
--     if_not_exists => TRUE
-- );
-- SELECT add_compression_policy('ledger_events',
--     compress_after => INTERVAL '30 days'
-- );

-- ─── Snapshot store ───────────────────────────────────────────────────────────
-- Stores serialised aggregate state at a given ledger position.
-- Bootstrap loads the latest snapshot and replays only delta events.
-- One row per (aggregate_type, aggregate_id) — newer snapshots overwrite older.

CREATE TABLE IF NOT EXISTS ledger_snapshots (
    id             BIGSERIAL    PRIMARY KEY,
    aggregate_type TEXT         NOT NULL,
    aggregate_id   TEXT         NOT NULL,
    account_id     TEXT,
    state_json     JSONB        NOT NULL,
    checksum       TEXT         NOT NULL,  -- SHA-256 of state_json bytes
    last_sequence  BIGINT       NOT NULL,  -- global_sequence at snapshot time
    event_count    BIGINT       NOT NULL DEFAULT 0,
    snapshot_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    version        INT          NOT NULL DEFAULT 1,
    UNIQUE (aggregate_type, aggregate_id)
);

CREATE INDEX IF NOT EXISTS idx_snapshots_account
    ON ledger_snapshots (account_id)
    WHERE account_id IS NOT NULL;

-- ─── Useful views ─────────────────────────────────────────────────────────────

-- View: open positions derived from position events
CREATE OR REPLACE VIEW v_open_positions AS
SELECT DISTINCT ON (aggregate_id)
    aggregate_id                          AS position_id,
    account_id,
    payload->>'symbol'                    AS symbol,
    payload->>'side'                      AS side,
    (payload->>'entry_price')::FLOAT      AS entry_price,
    (payload->>'quantity')::FLOAT         AS quantity,
    (payload->>'notional_usd')::FLOAT     AS notional_usd,
    (payload->>'stop_loss')::FLOAT        AS stop_loss,
    (payload->>'take_profit')::FLOAT      AS take_profit,
    (payload->>'strategy_name')::TEXT     AS strategy_name,
    created_at
FROM ledger_events
WHERE aggregate_type = 'POSITION'
  AND event_type     = 'POSITION_OPENED'
  AND aggregate_id NOT IN (
      SELECT aggregate_id FROM ledger_events
      WHERE aggregate_type = 'POSITION'
        AND event_type     = 'POSITION_CLOSED'
  )
ORDER BY aggregate_id, global_sequence DESC;

-- View: closed positions with PnL
CREATE OR REPLACE VIEW v_closed_positions AS
SELECT
    e.aggregate_id                            AS position_id,
    e.account_id,
    e.payload->>'strategy_name'               AS strategy_name,
    e.payload->>'symbol'                      AS symbol,
    e.payload->>'side'                        AS side,
    (e.payload->>'entry_price')::FLOAT        AS entry_price,
    (e.payload->>'exit_price')::FLOAT         AS exit_price,
    (e.payload->>'gross_pnl_usd')::FLOAT      AS gross_pnl_usd,
    (e.payload->>'net_pnl_usd')::FLOAT        AS net_pnl_usd,
    (e.payload->>'fees_usd')::FLOAT           AS fees_usd,
    e.payload->>'exit_reason'                 AS exit_reason,
    (e.payload->>'hold_minutes')::FLOAT       AS hold_minutes,
    e.created_at                              AS closed_at
FROM ledger_events e
WHERE e.aggregate_type = 'POSITION'
  AND e.event_type     = 'POSITION_CLOSED'
ORDER BY e.global_sequence DESC;

-- View: daily PnL summary
CREATE OR REPLACE VIEW v_daily_pnl AS
SELECT
    date_trunc('day', created_at AT TIME ZONE 'UTC') AS trade_date,
    account_id,
    COUNT(*)                                          AS total_trades,
    SUM(CASE WHEN (payload->>'net_pnl_usd')::FLOAT >= 0 THEN 1 ELSE 0 END) AS wins,
    SUM(CASE WHEN (payload->>'net_pnl_usd')::FLOAT < 0  THEN 1 ELSE 0 END) AS losses,
    SUM((payload->>'net_pnl_usd')::FLOAT)             AS total_pnl_usd,
    SUM((payload->>'fees_usd')::FLOAT)                AS total_fees_usd,
    MAX((payload->>'net_pnl_usd')::FLOAT)             AS best_trade_usd,
    MIN((payload->>'net_pnl_usd')::FLOAT)             AS worst_trade_usd
FROM ledger_events
WHERE aggregate_type = 'POSITION'
  AND event_type     = 'POSITION_CLOSED'
GROUP BY 1, 2
ORDER BY 1 DESC;

-- ─── Grants (uncomment and adjust for your role setup) ───────────────────────
-- GRANT SELECT, INSERT ON ledger_events            TO trading_engine;
-- GRANT SELECT, INSERT, UPDATE ON ledger_aggregate_sequences TO trading_engine;
-- GRANT SELECT, INSERT, UPDATE ON ledger_snapshots TO trading_engine;
-- GRANT SELECT ON v_open_positions   TO trading_dashboard;
-- GRANT SELECT ON v_closed_positions TO trading_dashboard;
-- GRANT SELECT ON v_daily_pnl        TO trading_dashboard;

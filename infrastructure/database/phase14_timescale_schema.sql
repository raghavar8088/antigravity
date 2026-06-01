-- Phase 14 Timescale/Postgres production trading schema.
-- Event store is source of truth. Projection tables are rebuildable read models.

create schema if not exists trading;
create schema if not exists market;

create extension if not exists timescaledb;
create extension if not exists pgcrypto;

create table if not exists trading.event_store (
  sequence_no bigserial primary key,
  event_id uuid not null unique default gen_random_uuid(),
  schema_version integer not null default 1,
  aggregate_type text not null,
  aggregate_id text not null,
  event_type text not null,
  account_id text,
  strategy_id text,
  symbol text,
  correlation_id uuid,
  causation_id uuid,
  idempotency_key text unique,
  payload jsonb not null default '{}'::jsonb,
  payload_hash text not null,
  source text not null,
  created_at timestamptz not null default now()
);

select create_hypertable('trading.event_store', 'created_at', if_not_exists => true);

create index if not exists idx_event_store_aggregate_sequence
  on trading.event_store (aggregate_type, aggregate_id, sequence_no);
create index if not exists idx_event_store_account_time
  on trading.event_store (account_id, created_at desc);
create index if not exists idx_event_store_symbol_time
  on trading.event_store (symbol, created_at desc);

create table if not exists market.ticks (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  price double precision not null check (price > 0),
  quantity double precision not null check (quantity >= 0),
  trade_id text,
  received_at timestamptz not null default now()
);

select create_hypertable('market.ticks', 'time', if_not_exists => true);

create index if not exists idx_ticks_symbol_time
  on market.ticks (symbol, time desc);
create index if not exists idx_ticks_exchange_symbol_time
  on market.ticks (exchange, symbol, time desc);

create table if not exists trading.order_projection (
  client_order_id text primary key,
  exchange_order_id text,
  account_id text not null,
  symbol text not null,
  side text not null,
  state text not null,
  quantity double precision not null,
  filled_quantity double precision not null default 0,
  average_fill_price double precision,
  fee_usd double precision not null default 0,
  updated_at timestamptz not null
);

create index if not exists idx_order_projection_account_state
  on trading.order_projection (account_id, state, updated_at desc);

create table if not exists trading.fill_projection (
  fill_id uuid primary key default gen_random_uuid(),
  client_order_id text not null,
  exchange_order_id text,
  account_id text not null,
  symbol text not null,
  side text not null,
  fill_price double precision not null,
  fill_quantity double precision not null,
  fee_usd double precision not null,
  filled_at timestamptz not null
);

select create_hypertable('trading.fill_projection', 'filled_at', if_not_exists => true);

create index if not exists idx_fill_projection_account_time
  on trading.fill_projection (account_id, filled_at desc);
create index if not exists idx_fill_projection_symbol_time
  on trading.fill_projection (symbol, filled_at desc);

create table if not exists trading.position_projection (
  account_id text not null,
  symbol text not null,
  side text not null,
  quantity double precision not null,
  average_entry double precision not null,
  realized_pnl double precision not null default 0,
  unrealized_pnl double precision not null default 0,
  updated_at timestamptz not null,
  primary key (account_id, symbol, side)
);

create materialized view if not exists market.candles_1m
with (timescaledb.continuous) as
select
  time_bucket('1 minute', time) as bucket,
  exchange,
  symbol,
  first(price, time) as open,
  max(price) as high,
  min(price) as low,
  last(price, time) as close,
  sum(quantity) as volume
from market.ticks
group by bucket, exchange, symbol
with no data;

create materialized view if not exists trading.pnl_1m
with (timescaledb.continuous) as
select
  time_bucket('1 minute', filled_at) as bucket,
  account_id,
  symbol,
  count(*) as fills,
  sum(fee_usd) as fees
from trading.fill_projection
group by bucket, account_id, symbol
with no data;

alter table market.ticks set (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'exchange,symbol'
);

alter table trading.event_store set (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'aggregate_type,account_id'
);

select add_compression_policy('market.ticks', interval '7 days', if_not_exists => true);
select add_compression_policy('trading.event_store', interval '7 days', if_not_exists => true);
select add_retention_policy('market.ticks', interval '90 days', if_not_exists => true);

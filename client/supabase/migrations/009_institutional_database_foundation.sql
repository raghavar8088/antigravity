-- Phase 7 institutional database foundation.
-- Target engine: PostgreSQL 15+ with TimescaleDB 2.x.
-- This migration is intentionally additive and does not drop legacy paper tables.

create extension if not exists pgcrypto;
create extension if not exists timescaledb;
create extension if not exists citext;

create schema if not exists market;
create schema if not exists core;
create schema if not exists audit;
create schema if not exists research;
create schema if not exists ops;

do $$ begin
  create type core.order_side as enum ('BUY', 'SELL');
exception when duplicate_object then null; end $$;

do $$ begin
  create type core.order_status as enum ('PENDING', 'SUBMITTED', 'PARTIAL', 'FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED', 'CLOSED');
exception when duplicate_object then null; end $$;

do $$ begin
  create type core.order_type as enum ('MARKET', 'LIMIT', 'POST_ONLY', 'REDUCE_ONLY');
exception when duplicate_object then null; end $$;

do $$ begin
  create type core.position_status as enum ('OPEN', 'CLOSING', 'CLOSED', 'LIQUIDATED');
exception when duplicate_object then null; end $$;

do $$ begin
  create type audit.event_type as enum (
    'OrderCreated',
    'OrderSubmitted',
    'OrderFilled',
    'OrderCancelled',
    'PositionOpened',
    'PositionClosed',
    'RiskViolation',
    'StrategyPromoted',
    'StrategyDisabled',
    'KillSwitchTriggered',
    'CircuitBreakerTriggered',
    'AITradeApproved',
    'AITradeRejected'
  );
exception when duplicate_object then null; end $$;

-- ─────────────────────────────────────────────────────────────────────────────
-- Layer 1: TimescaleDB market data source of truth.
-- ─────────────────────────────────────────────────────────────────────────────

create table if not exists market.market_ticks (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  trade_id text not null,
  price numeric(20, 8) not null check (price > 0),
  quantity numeric(20, 8) not null check (quantity >= 0),
  side text not null check (side in ('BUY', 'SELL', 'UNKNOWN')),
  received_at timestamptz not null default now(),
  source_latency_ms integer,
  primary key (time, symbol, exchange, trade_id)
);

select create_hypertable('market.market_ticks', 'time', partitioning_column => 'symbol', number_partitions => 16, chunk_time_interval => interval '1 day', if_not_exists => true);
alter table market.market_ticks set (timescaledb.compress, timescaledb.compress_segmentby = 'symbol,exchange', timescaledb.compress_orderby = 'time desc');
select add_compression_policy('market.market_ticks', interval '30 days', if_not_exists => true);
select add_retention_policy('market.market_ticks', interval '180 days', if_not_exists => true);

create table if not exists market.market_candles_1m (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  open numeric(20, 8) not null,
  high numeric(20, 8) not null,
  low numeric(20, 8) not null,
  close numeric(20, 8) not null,
  volume numeric(24, 8) not null,
  vwap numeric(20, 8),
  trades bigint not null default 0,
  source text not null default 'cagg',
  created_at timestamptz not null default now(),
  primary key (time, symbol, exchange)
);

create table if not exists market.market_candles_3m (like market.market_candles_1m including all);
create table if not exists market.market_candles_5m (like market.market_candles_1m including all);
create table if not exists market.market_candles_15m (like market.market_candles_1m including all);
create table if not exists market.market_candles_30m (like market.market_candles_1m including all);
create table if not exists market.market_candles_1h (like market.market_candles_1m including all);
create table if not exists market.market_candles_4h (like market.market_candles_1m including all);
create table if not exists market.market_candles_1d (like market.market_candles_1m including all);

do $$
declare
  candle_table text;
begin
  foreach candle_table in array array[
    'market.market_candles_1m',
    'market.market_candles_3m',
    'market.market_candles_5m',
    'market.market_candles_15m',
    'market.market_candles_30m',
    'market.market_candles_1h',
    'market.market_candles_4h',
    'market.market_candles_1d'
  ]
  loop
    execute format('select create_hypertable(%L, %L, partitioning_column => %L, number_partitions => 16, chunk_time_interval => interval %L, if_not_exists => true)', candle_table, 'time', 'symbol', '7 days');
    execute format('alter table %s set (timescaledb.compress, timescaledb.compress_segmentby = %L, timescaledb.compress_orderby = %L)', candle_table, 'symbol,exchange', 'time desc');
    execute format('select add_compression_policy(%L, interval %L, if_not_exists => true)', candle_table, '30 days');
    execute format('select add_retention_policy(%L, interval %L, if_not_exists => true)', candle_table, '5 years');
  end loop;
end $$;

create table if not exists market.funding_rates (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  funding_rate numeric(18, 10) not null,
  mark_price numeric(20, 8),
  index_price numeric(20, 8),
  next_funding_time timestamptz,
  primary key (time, symbol, exchange)
);

select create_hypertable('market.funding_rates', 'time', partitioning_column => 'symbol', number_partitions => 16, chunk_time_interval => interval '30 days', if_not_exists => true);
alter table market.funding_rates set (timescaledb.compress, timescaledb.compress_segmentby = 'symbol,exchange', timescaledb.compress_orderby = 'time desc');
select add_compression_policy('market.funding_rates', interval '30 days', if_not_exists => true);
select add_retention_policy('market.funding_rates', interval '5 years', if_not_exists => true);

create table if not exists market.liquidation_events (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  side text not null check (side in ('LONG_LIQUIDATED', 'SHORT_LIQUIDATED')),
  price numeric(20, 8) not null check (price > 0),
  quantity numeric(20, 8) not null check (quantity > 0),
  notional_usd numeric(24, 8) generated always as (price * quantity) stored,
  event_id text not null,
  primary key (time, symbol, exchange, event_id)
);

select create_hypertable('market.liquidation_events', 'time', partitioning_column => 'symbol', number_partitions => 16, chunk_time_interval => interval '7 days', if_not_exists => true);
alter table market.liquidation_events set (timescaledb.compress, timescaledb.compress_segmentby = 'symbol,exchange', timescaledb.compress_orderby = 'time desc');
select add_compression_policy('market.liquidation_events', interval '30 days', if_not_exists => true);
select add_retention_policy('market.liquidation_events', interval '2 years', if_not_exists => true);

create table if not exists market.orderbook_snapshots (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  bids numeric(20, 8)[][] not null,
  asks numeric(20, 8)[][] not null,
  best_bid numeric(20, 8) not null,
  best_ask numeric(20, 8) not null,
  mid numeric(20, 8) generated always as ((best_bid + best_ask) / 2) stored,
  spread numeric(20, 8) generated always as (best_ask - best_bid) stored,
  spread_bps numeric(20, 8) generated always as (case when (best_bid + best_ask) > 0 then ((best_ask - best_bid) / ((best_bid + best_ask) / 2)) * 10000 else 0 end) stored,
  bid_depth_20 numeric(24, 8) not null,
  ask_depth_20 numeric(24, 8) not null,
  imbalance numeric(18, 8) generated always as (case when (bid_depth_20 + ask_depth_20) > 0 then (bid_depth_20 - ask_depth_20) / (bid_depth_20 + ask_depth_20) else 0 end) stored,
  sequence_id bigint,
  primary key (time, symbol, exchange)
);

select create_hypertable('market.orderbook_snapshots', 'time', partitioning_column => 'symbol', number_partitions => 16, chunk_time_interval => interval '1 day', if_not_exists => true);
alter table market.orderbook_snapshots set (timescaledb.compress, timescaledb.compress_segmentby = 'symbol,exchange', timescaledb.compress_orderby = 'time desc');
select add_compression_policy('market.orderbook_snapshots', interval '7 days', if_not_exists => true);
select add_retention_policy('market.orderbook_snapshots', interval '30 days', if_not_exists => true);

-- Tick-derived continuous aggregates. Materialized output coexists with explicit
-- candle hypertables so exchange-native candles can be compared against cagg candles.
create materialized view if not exists market.cagg_ticks_1m
with (timescaledb.continuous) as
select
  time_bucket('1 minute', time) as time,
  exchange,
  symbol,
  first(price, time) as open,
  max(price) as high,
  min(price) as low,
  last(price, time) as close,
  sum(quantity) as volume,
  sum(price * quantity) / nullif(sum(quantity), 0) as vwap,
  count(*)::bigint as trades
from market.market_ticks
group by 1, 2, 3
with no data;

create materialized view if not exists market.cagg_ticks_3m
with (timescaledb.continuous) as
select time_bucket('3 minutes', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

create materialized view if not exists market.cagg_ticks_5m
with (timescaledb.continuous) as
select time_bucket('5 minutes', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

create materialized view if not exists market.cagg_ticks_15m
with (timescaledb.continuous) as
select time_bucket('15 minutes', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

create materialized view if not exists market.cagg_ticks_30m
with (timescaledb.continuous) as
select time_bucket('30 minutes', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

create materialized view if not exists market.cagg_ticks_1h
with (timescaledb.continuous) as
select time_bucket('1 hour', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

create materialized view if not exists market.cagg_ticks_4h
with (timescaledb.continuous) as
select time_bucket('4 hours', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

create materialized view if not exists market.cagg_ticks_1d
with (timescaledb.continuous) as
select time_bucket('1 day', time) as time, exchange, symbol, first(price, time) as open, max(price) as high, min(price) as low, last(price, time) as close, sum(quantity) as volume, sum(price * quantity) / nullif(sum(quantity), 0) as vwap, count(*)::bigint as trades
from market.market_ticks group by 1, 2, 3 with no data;

select add_continuous_aggregate_policy('market.cagg_ticks_1m', start_offset => interval '2 days', end_offset => interval '10 seconds', schedule_interval => interval '30 seconds', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_3m', start_offset => interval '3 days', end_offset => interval '30 seconds', schedule_interval => interval '1 minute', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_5m', start_offset => interval '7 days', end_offset => interval '1 minute', schedule_interval => interval '1 minute', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_15m', start_offset => interval '14 days', end_offset => interval '2 minutes', schedule_interval => interval '5 minutes', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_30m', start_offset => interval '21 days', end_offset => interval '5 minutes', schedule_interval => interval '5 minutes', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_1h', start_offset => interval '30 days', end_offset => interval '10 minutes', schedule_interval => interval '15 minutes', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_4h', start_offset => interval '90 days', end_offset => interval '30 minutes', schedule_interval => interval '1 hour', if_not_exists => true);
select add_continuous_aggregate_policy('market.cagg_ticks_1d', start_offset => interval '365 days', end_offset => interval '1 hour', schedule_interval => interval '6 hours', if_not_exists => true);

-- Monitoring view for continuous aggregate freshness.
create or replace view ops.continuous_aggregate_health as
select
  view_schema,
  view_name,
  materialized_only,
  compression_enabled,
  finalized
from timescaledb_information.continuous_aggregates
where view_schema = 'market';

-- ─────────────────────────────────────────────────────────────────────────────
-- Layer 2: PostgreSQL core trading database.
-- ─────────────────────────────────────────────────────────────────────────────

create table if not exists core.users (
  user_id uuid primary key default gen_random_uuid(),
  external_user_id uuid unique,
  email citext unique,
  display_name text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  disabled_at timestamptz
);

create table if not exists core.accounts (
  account_id uuid primary key default gen_random_uuid(),
  user_id uuid not null references core.users(user_id) on delete cascade,
  account_key text not null unique,
  account_type text not null check (account_type in ('PAPER', 'LIVE', 'RESEARCH')),
  base_currency text not null default 'USD',
  starting_equity numeric(24, 8) not null check (starting_equity >= 0),
  created_at timestamptz not null default now(),
  disabled_at timestamptz
);

create table if not exists core.strategies (
  strategy_id uuid primary key default gen_random_uuid(),
  strategy_code text not null unique,
  strategy_name text not null,
  family text not null,
  alpha_source text,
  timeframe text,
  created_at timestamptz not null default now(),
  disabled_at timestamptz
);

create table if not exists core.strategy_versions (
  strategy_version_id uuid primary key default gen_random_uuid(),
  strategy_id uuid not null references core.strategies(strategy_id) on delete cascade,
  semantic_version text not null,
  code_hash text not null,
  parameter_hash text not null,
  config_hash text not null,
  status text not null check (status in ('RESEARCH', 'PAPER', 'PROMOTED', 'LIVE', 'RETIRED')),
  created_at timestamptz not null default now(),
  unique (strategy_id, semantic_version)
);

create table if not exists core.strategy_health (
  strategy_health_id uuid primary key default gen_random_uuid(),
  strategy_version_id uuid not null references core.strategy_versions(strategy_version_id) on delete cascade,
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  total_trades integer not null default 0,
  win_rate numeric(8, 5) not null default 0,
  profit_factor numeric(16, 8) not null default 0,
  sharpe numeric(16, 8) not null default 0,
  sortino numeric(16, 8) not null default 0,
  expectancy_usd numeric(24, 8) not null default 0,
  max_drawdown_pct numeric(12, 8) not null default 0,
  risk_stability numeric(8, 5) not null default 0,
  heat_impact numeric(8, 5) not null default 0,
  capital_efficiency numeric(8, 5) not null default 0,
  category text not null check (category in ('ELITE', 'HEALTHY', 'WATCHLIST', 'RESTRICTED', 'DISABLED')),
  last_updated timestamptz not null default now(),
  unique (strategy_version_id, account_id)
);

create table if not exists core.research_tournaments (
  tournament_id uuid primary key default gen_random_uuid(),
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  name text not null,
  status text not null check (status in ('RUNNING', 'COMPLETE', 'CANCELLED')),
  started_at timestamptz not null default now(),
  completed_at timestamptz
);

create table if not exists core.orders (
  order_id uuid primary key default gen_random_uuid(),
  client_order_id text not null unique,
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  strategy_version_id uuid references core.strategy_versions(strategy_version_id) on delete set null,
  symbol text not null,
  side core.order_side not null,
  order_type core.order_type not null,
  qty numeric(24, 8) not null check (qty > 0),
  limit_price numeric(20, 8),
  status core.order_status not null,
  exchange text not null,
  reduce_only boolean not null default false,
  post_only boolean not null default false,
  submitted_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists core.executions (
  execution_id uuid primary key default gen_random_uuid(),
  order_id uuid not null references core.orders(order_id) on delete cascade,
  exchange_execution_id text,
  fill_price numeric(20, 8) not null check (fill_price > 0),
  fill_qty numeric(24, 8) not null check (fill_qty > 0),
  fees numeric(24, 8) not null default 0,
  fee_currency text not null default 'USD',
  slippage_bps numeric(16, 8) not null default 0,
  latency_ms integer not null default 0,
  liquidity_flag text check (liquidity_flag in ('MAKER', 'TAKER', 'UNKNOWN')),
  executed_at timestamptz not null,
  created_at timestamptz not null default now()
);

create table if not exists core.positions (
  position_id uuid primary key default gen_random_uuid(),
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  strategy_version_id uuid references core.strategy_versions(strategy_version_id) on delete set null,
  symbol text not null,
  side text not null check (side in ('LONG', 'SHORT')),
  status core.position_status not null default 'OPEN',
  entry_order_id uuid references core.orders(order_id) on delete set null,
  entry_price numeric(20, 8) not null check (entry_price > 0),
  size_qty numeric(24, 8) not null check (size_qty > 0),
  current_qty numeric(24, 8) not null check (current_qty >= 0),
  stop_loss numeric(20, 8),
  take_profit numeric(20, 8),
  max_favorable_excursion numeric(24, 8) not null default 0,
  max_adverse_excursion numeric(24, 8) not null default 0,
  max_drawdown_usd numeric(24, 8) not null default 0,
  risk_usd numeric(24, 8) not null default 0,
  opened_at timestamptz not null,
  updated_at timestamptz not null default now()
);

create table if not exists core.closed_positions (
  closed_position_id uuid primary key default gen_random_uuid(),
  position_id uuid not null unique references core.positions(position_id) on delete cascade,
  exit_order_id uuid references core.orders(order_id) on delete set null,
  exit_price numeric(20, 8) not null check (exit_price > 0),
  gross_pnl numeric(24, 8) not null,
  fees numeric(24, 8) not null default 0,
  funding_pnl numeric(24, 8) not null default 0,
  net_pnl numeric(24, 8) not null,
  duration_ms bigint not null,
  exit_reason text not null,
  closed_at timestamptz not null
);

create table if not exists core.position_events (
  position_event_id uuid primary key default gen_random_uuid(),
  position_id uuid not null references core.positions(position_id) on delete cascade,
  event_type text not null,
  mark_price numeric(20, 8),
  realized_pnl numeric(24, 8),
  unrealized_pnl numeric(24, 8),
  funding_pnl numeric(24, 8),
  risk_usd numeric(24, 8),
  event_at timestamptz not null default now()
);

create table if not exists core.risk_events (
  risk_event_id uuid primary key default gen_random_uuid(),
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  strategy_version_id uuid references core.strategy_versions(strategy_version_id) on delete set null,
  symbol text,
  event_type text not null,
  severity text not null check (severity in ('INFO', 'WARNING', 'CRITICAL')),
  reason text not null,
  portfolio_heat_pct numeric(12, 8),
  var95_usd numeric(24, 8),
  cvar95_usd numeric(24, 8),
  gross_exposure_usd numeric(24, 8),
  net_exposure_usd numeric(24, 8),
  drawdown_pct numeric(12, 8),
  created_at timestamptz not null default now()
);

create table if not exists core.portfolio_snapshots (
  snapshot_at timestamptz not null,
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  equity numeric(24, 8) not null,
  cash numeric(24, 8) not null,
  used_margin numeric(24, 8) not null,
  available_margin numeric(24, 8) not null,
  unrealized_pnl numeric(24, 8) not null,
  realized_pnl numeric(24, 8) not null,
  long_exposure_usd numeric(24, 8) not null default 0,
  short_exposure_usd numeric(24, 8) not null default 0,
  gross_exposure_usd numeric(24, 8) not null default 0,
  net_exposure_usd numeric(24, 8) not null default 0,
  portfolio_heat_pct numeric(12, 8) not null default 0,
  drawdown_pct numeric(12, 8) not null default 0,
  funding_paid numeric(24, 8) not null default 0,
  funding_received numeric(24, 8) not null default 0,
  var95_usd numeric(24, 8),
  cvar95_usd numeric(24, 8),
  primary key (snapshot_at, account_id)
);

select create_hypertable('core.portfolio_snapshots', 'snapshot_at', partitioning_column => 'account_id', number_partitions => 8, chunk_time_interval => interval '7 days', if_not_exists => true);
alter table core.portfolio_snapshots set (timescaledb.compress, timescaledb.compress_segmentby = 'account_id', timescaledb.compress_orderby = 'snapshot_at desc');
select add_compression_policy('core.portfolio_snapshots', interval '30 days', if_not_exists => true);
select add_retention_policy('core.portfolio_snapshots', interval '10 years', if_not_exists => true);

create table if not exists core.promotion_history (
  promotion_id uuid primary key default gen_random_uuid(),
  strategy_version_id uuid not null references core.strategy_versions(strategy_version_id) on delete cascade,
  account_id uuid not null references core.accounts(account_id) on delete cascade,
  tournament_id uuid references core.research_tournaments(tournament_id) on delete set null,
  from_status text not null,
  to_status text not null,
  risk_score numeric(8, 5),
  oos_profit_factor numeric(16, 8),
  oos_expectancy_usd numeric(24, 8),
  approved_by text not null default 'system',
  reason text not null,
  promoted_at timestamptz not null default now()
);

create table if not exists core.system_config (
  config_key text primary key,
  config_value text not null,
  value_type text not null check (value_type in ('string', 'number', 'boolean', 'json')),
  updated_at timestamptz not null default now(),
  updated_by text not null default 'system'
);

-- ─────────────────────────────────────────────────────────────────────────────
-- Layer 3/4 source-of-truth support: event sourcing, AI audit, warehouse.
-- ─────────────────────────────────────────────────────────────────────────────

create table if not exists audit.event_store (
  event_id uuid primary key default gen_random_uuid(),
  aggregate_type text not null,
  aggregate_id uuid not null,
  account_id uuid references core.accounts(account_id) on delete set null,
  event_type audit.event_type not null,
  event_version integer not null default 1,
  sequence_no bigint generated always as identity,
  causation_id uuid,
  correlation_id uuid,
  payload jsonb not null default '{}'::jsonb,
  event_hash text not null,
  created_at timestamptz not null default now(),
  unique (aggregate_type, aggregate_id, sequence_no)
);

create table if not exists audit.ai_decisions (
  decided_at timestamptz not null,
  ai_decision_id uuid not null default gen_random_uuid(),
  account_id uuid references core.accounts(account_id) on delete set null,
  strategy_version_id uuid references core.strategy_versions(strategy_version_id) on delete set null,
  symbol text not null,
  market_state_hash text not null,
  bull_agent_output text,
  bear_agent_output text,
  macro_agent_output text,
  risk_veto_output text,
  final_decision text not null check (final_decision in ('APPROVE', 'REJECT', 'HOLD', 'REVIEW')),
  latency_ms integer not null default 0,
  tokens_used integer not null default 0,
  model text not null,
  provider text not null,
  confidence numeric(8, 5) not null check (confidence >= 0 and confidence <= 1),
  created_at timestamptz not null default now(),
  primary key (decided_at, ai_decision_id)
);

select create_hypertable('audit.ai_decisions', 'decided_at', partitioning_column => 'symbol', number_partitions => 8, chunk_time_interval => interval '7 days', if_not_exists => true);
alter table audit.ai_decisions set (timescaledb.compress, timescaledb.compress_segmentby = 'symbol,provider,model', timescaledb.compress_orderby = 'decided_at desc');
select add_compression_policy('audit.ai_decisions', interval '7 days', if_not_exists => true);
select add_retention_policy('audit.ai_decisions', interval '90 days', if_not_exists => true);

create table if not exists research.backtests (
  backtest_id uuid primary key default gen_random_uuid(),
  strategy_version_id uuid not null references core.strategy_versions(strategy_version_id) on delete cascade,
  dataset_hash text not null,
  symbol text not null,
  mode text not null,
  started_at timestamptz not null,
  ended_at timestamptz not null,
  gross_pnl numeric(24, 8) not null,
  net_pnl numeric(24, 8) not null,
  sharpe numeric(16, 8),
  sortino numeric(16, 8),
  max_drawdown numeric(24, 8),
  profit_factor numeric(16, 8),
  expectancy numeric(24, 8),
  var95 numeric(24, 8),
  cvar95 numeric(24, 8),
  created_at timestamptz not null default now()
);

create table if not exists research.walk_forward_results (
  walk_forward_id uuid primary key default gen_random_uuid(),
  backtest_id uuid not null references research.backtests(backtest_id) on delete cascade,
  train_start timestamptz not null,
  train_end timestamptz not null,
  test_start timestamptz not null,
  test_end timestamptz not null,
  is_profit_factor numeric(16, 8),
  oos_profit_factor numeric(16, 8),
  is_expectancy numeric(24, 8),
  oos_expectancy numeric(24, 8),
  degradation_pct numeric(12, 8),
  stable boolean not null default false
);

create table if not exists research.monte_carlo_simulations (
  monte_carlo_id uuid primary key default gen_random_uuid(),
  backtest_id uuid not null references research.backtests(backtest_id) on delete cascade,
  paths integer not null check (paths >= 1000),
  worst_case numeric(24, 8),
  best_case numeric(24, 8),
  median numeric(24, 8),
  percentile_5 numeric(24, 8),
  percentile_95 numeric(24, 8),
  survival_rate numeric(8, 5),
  created_at timestamptz not null default now()
);

create table if not exists research.parameter_sweeps (
  sweep_id uuid primary key default gen_random_uuid(),
  strategy_version_id uuid not null references core.strategy_versions(strategy_version_id) on delete cascade,
  parameter_hash text not null,
  parameter_name text not null,
  parameter_value text not null,
  objective_score numeric(16, 8),
  robustness_score numeric(16, 8),
  created_at timestamptz not null default now()
);

create table if not exists research.strategy_comparisons (
  comparison_id uuid primary key default gen_random_uuid(),
  tournament_id uuid references core.research_tournaments(tournament_id) on delete cascade,
  strategy_version_id uuid not null references core.strategy_versions(strategy_version_id) on delete cascade,
  rank integer not null,
  execution_score numeric(8, 5),
  risk_score numeric(8, 5),
  oos_score numeric(8, 5),
  benchmark_alpha numeric(16, 8),
  promotion_eligible boolean not null default false,
  created_at timestamptz not null default now()
);

create table if not exists research.optimization_results (
  optimization_id uuid primary key default gen_random_uuid(),
  strategy_version_id uuid references core.strategy_versions(strategy_version_id) on delete cascade,
  method text not null,
  objective text not null,
  expected_return numeric(16, 8),
  expected_risk numeric(16, 8),
  sharpe numeric(16, 8),
  created_at timestamptz not null default now()
);

-- Data quality and repair queue.
create table if not exists ops.data_quality_checks (
  check_id uuid primary key default gen_random_uuid(),
  check_name text not null,
  symbol text,
  severity text not null check (severity in ('INFO', 'WARNING', 'CRITICAL')),
  health_score numeric(8, 5) not null,
  missing_candles integer not null default 0,
  duplicate_candles integer not null default 0,
  duplicate_ticks integer not null default 0,
  timestamp_drift_ms integer not null default 0,
  exchange_outage boolean not null default false,
  corrupted_records integer not null default 0,
  bad_prices integer not null default 0,
  checked_at timestamptz not null default now()
);

create table if not exists ops.data_repair_jobs (
  repair_job_id uuid primary key default gen_random_uuid(),
  check_id uuid references ops.data_quality_checks(check_id) on delete set null,
  symbol text,
  repair_type text not null,
  status text not null check (status in ('PENDING', 'RUNNING', 'COMPLETE', 'FAILED')),
  started_at timestamptz,
  completed_at timestamptz,
  details text
);

-- Core indexes for low-latency trading reads and high-volume writes.
create index if not exists market_ticks_symbol_time_idx on market.market_ticks (symbol, time desc);
create index if not exists market_ticks_exchange_symbol_time_idx on market.market_ticks (exchange, symbol, time desc);
create index if not exists funding_rates_symbol_time_idx on market.funding_rates (symbol, time desc);
create index if not exists liquidation_events_symbol_time_idx on market.liquidation_events (symbol, time desc);
create index if not exists orderbook_snapshots_symbol_time_idx on market.orderbook_snapshots (symbol, time desc);

create index if not exists orders_status_created_idx on core.orders (status, created_at desc);
create index if not exists orders_strategy_created_idx on core.orders (strategy_version_id, created_at desc);
create index if not exists orders_symbol_created_idx on core.orders (symbol, created_at desc);
create index if not exists orders_account_status_created_idx on core.orders (account_id, status, created_at desc);
create index if not exists executions_order_time_idx on core.executions (order_id, executed_at desc);
create index if not exists executions_time_idx on core.executions (executed_at desc);
create index if not exists positions_account_idx on core.positions (account_id);
create index if not exists positions_status_idx on core.positions (status);
create index if not exists positions_symbol_idx on core.positions (symbol);
create index if not exists positions_account_status_idx on core.positions (account_id, status);
create index if not exists position_events_position_time_idx on core.position_events (position_id, event_at desc);
create index if not exists strategy_health_strategy_idx on core.strategy_health (strategy_version_id);
create index if not exists strategy_health_last_updated_idx on core.strategy_health (last_updated desc);
create index if not exists portfolio_snapshots_account_time_idx on core.portfolio_snapshots (account_id, snapshot_at desc);
create index if not exists risk_events_account_time_idx on core.risk_events (account_id, created_at desc);
create index if not exists event_store_aggregate_idx on audit.event_store (aggregate_type, aggregate_id, sequence_no);
create index if not exists event_store_type_time_idx on audit.event_store (event_type, created_at desc);
create index if not exists ai_decisions_symbol_time_idx on audit.ai_decisions (symbol, decided_at desc);
create index if not exists ai_decisions_provider_model_time_idx on audit.ai_decisions (provider, model, decided_at desc);
create index if not exists research_backtests_strategy_created_idx on research.backtests (strategy_version_id, created_at desc);
create index if not exists research_walk_forward_backtest_idx on research.walk_forward_results (backtest_id, test_start);
create index if not exists research_mc_backtest_idx on research.monte_carlo_simulations (backtest_id, created_at desc);

-- Legacy Supabase paper table hardening indexes requested by Phase 7K.
create index if not exists paper_trades_strategy_closed_idx on public.paper_trades (strategy_id, closed_at desc);
create index if not exists paper_trades_strategy_status_timestamp_idx on public.paper_trades (strategy_id, exit_reason, closed_at desc);
create index if not exists paper_trades_timestamp_idx on public.paper_trades (closed_at desc);

comment on schema market is 'TimescaleDB source of truth for ticks, candles, funding, liquidations, and order book snapshots.';
comment on schema core is 'Normalized PostgreSQL source of truth for accounts, OMS, positions, risk, portfolio snapshots, and promotion history.';
comment on schema audit is 'Immutable event store and compressed AI decision audit logs.';
comment on schema research is 'Research warehouse schema for backtests, walk-forward validation, Monte Carlo, parameter sweeps, and optimization results.';
comment on schema ops is 'Operational monitoring, data quality, and repair metadata.';

-- Phase 7 operational hardening: event replay, data quality, performance targets,
-- retention cleanup helpers, and least-privilege database roles.

create extension if not exists pgcrypto;

create or replace function audit.event_payload_hash(
  p_aggregate_type text,
  p_aggregate_id uuid,
  p_event_type audit.event_type,
  p_payload jsonb,
  p_created_at timestamptz
) returns text
language sql
immutable
as $$
  select encode(digest(
    p_aggregate_type || ':' ||
    p_aggregate_id::text || ':' ||
    p_event_type::text || ':' ||
    coalesce(p_payload::text, '{}') || ':' ||
    p_created_at::text,
    'sha256'
  ), 'hex')
$$;

create or replace function audit.append_event(
  p_aggregate_type text,
  p_aggregate_id uuid,
  p_account_id uuid,
  p_event_type audit.event_type,
  p_payload jsonb default '{}'::jsonb,
  p_causation_id uuid default null,
  p_correlation_id uuid default null
) returns uuid
language plpgsql
security definer
as $$
declare
  v_event_id uuid;
  v_created_at timestamptz := now();
begin
  insert into audit.event_store (
    aggregate_type,
    aggregate_id,
    account_id,
    event_type,
    causation_id,
    correlation_id,
    payload,
    event_hash,
    created_at
  )
  values (
    p_aggregate_type,
    p_aggregate_id,
    p_account_id,
    p_event_type,
    coalesce(p_causation_id, gen_random_uuid()),
    coalesce(p_correlation_id, gen_random_uuid()),
    coalesce(p_payload, '{}'::jsonb),
    audit.event_payload_hash(p_aggregate_type, p_aggregate_id, p_event_type, coalesce(p_payload, '{}'::jsonb), v_created_at),
    v_created_at
  )
  returning event_id into v_event_id;

  return v_event_id;
end;
$$;

create or replace function audit.replay_events(
  p_aggregate_type text,
  p_aggregate_id uuid
) returns table (
  sequence_no bigint,
  event_id uuid,
  event_type audit.event_type,
  payload jsonb,
  created_at timestamptz,
  event_hash text
)
language sql
stable
as $$
  select sequence_no, event_id, event_type, payload, created_at, event_hash
  from audit.event_store
  where aggregate_type = p_aggregate_type
    and aggregate_id = p_aggregate_id
  order by sequence_no asc
$$;

create or replace view audit.event_replay_lag as
select
  aggregate_type,
  aggregate_id,
  max(sequence_no) as last_sequence_no,
  max(created_at) as last_event_at,
  now() - max(created_at) as event_lag
from audit.event_store
group by aggregate_type, aggregate_id;

create or replace function ops.run_market_data_quality_check(
  p_symbol text,
  p_start timestamptz,
  p_end timestamptz
) returns uuid
language plpgsql
as $$
declare
  v_check_id uuid;
  v_expected_minutes integer;
  v_actual_minutes integer;
  v_missing integer;
  v_duplicate_ticks integer;
  v_bad_prices integer;
  v_drift integer;
  v_score numeric(8, 5);
  v_severity text;
begin
  if p_end <= p_start then
    raise exception 'quality check end must be after start';
  end if;

  v_expected_minutes := greatest(1, floor(extract(epoch from (p_end - p_start)) / 60)::integer);

  select count(*)::integer
  into v_actual_minutes
  from market.cagg_ticks_1m
  where symbol = p_symbol
    and time >= p_start
    and time < p_end;

  v_missing := greatest(0, v_expected_minutes - v_actual_minutes);

  select count(*)::integer
  into v_duplicate_ticks
  from (
    select exchange, symbol, trade_id, count(*)
    from market.market_ticks
    where symbol = p_symbol
      and time >= p_start
      and time < p_end
    group by exchange, symbol, trade_id
    having count(*) > 1
  ) dupes;

  select count(*)::integer
  into v_bad_prices
  from market.market_ticks
  where symbol = p_symbol
    and time >= p_start
    and time < p_end
    and (price <= 0 or quantity < 0);

  select coalesce(max(abs(extract(epoch from (received_at - time)) * 1000))::integer, 0)
  into v_drift
  from market.market_ticks
  where symbol = p_symbol
    and time >= p_start
    and time < p_end;

  v_score := greatest(0, 100 - (v_missing * 0.5) - (v_duplicate_ticks * 1.0) - (v_bad_prices * 5.0) - least(v_drift / 1000.0, 25));
  v_severity := case
    when v_score < 70 then 'CRITICAL'
    when v_score < 90 then 'WARNING'
    else 'INFO'
  end;

  insert into ops.data_quality_checks (
    check_name,
    symbol,
    severity,
    health_score,
    missing_candles,
    duplicate_ticks,
    timestamp_drift_ms,
    bad_prices,
    exchange_outage,
    checked_at
  )
  values (
    'market_ticks_to_1m_candles',
    p_symbol,
    v_severity,
    v_score,
    v_missing,
    v_duplicate_ticks,
    v_drift,
    v_bad_prices,
    v_actual_minutes = 0,
    now()
  )
  returning check_id into v_check_id;

  if v_score < 90 then
    insert into ops.data_repair_jobs (check_id, symbol, repair_type, status, details)
    values (
      v_check_id,
      p_symbol,
      'BACKFILL_OR_RECONCILE_MARKET_DATA',
      'PENDING',
      format('missing=%s duplicate_ticks=%s bad_prices=%s timestamp_drift_ms=%s', v_missing, v_duplicate_ticks, v_bad_prices, v_drift)
    );
  end if;

  return v_check_id;
end;
$$;

create or replace view ops.data_quality_latest as
select distinct on (symbol, check_name)
  check_id,
  check_name,
  symbol,
  severity,
  health_score,
  missing_candles,
  duplicate_candles,
  duplicate_ticks,
  timestamp_drift_ms,
  exchange_outage,
  corrupted_records,
  bad_prices,
  checked_at
from ops.data_quality_checks
order by symbol, check_name, checked_at desc;

create table if not exists ops.performance_targets (
  target_name text primary key,
  target_ms integer not null,
  query_pattern text not null,
  recommended_index text not null
);

insert into ops.performance_targets (target_name, target_ms, query_pattern, recommended_index)
values
  ('order_lookup', 5, 'core.orders by order_id/client_order_id/status', 'orders_status_created_idx, orders_account_status_created_idx'),
  ('position_lookup', 5, 'core.positions by account_id/status/symbol', 'positions_account_status_idx, positions_symbol_idx'),
  ('strategy_health_query', 10, 'core.strategy_health by strategy_version_id/last_updated', 'strategy_health_strategy_idx, strategy_health_last_updated_idx'),
  ('portfolio_snapshot_query', 10, 'core.portfolio_snapshots by account_id/snapshot_at desc', 'portfolio_snapshots_account_time_idx'),
  ('research_query', 100, 'research.backtests and child result tables by strategy/backtest', 'research_backtests_strategy_created_idx'),
  ('dashboard_load', 500, 'Mongo analytics cache plus latest core snapshots', 'Mongo dashboard_metrics/account/timestamp TTL indexes')
on conflict (target_name) do update
set target_ms = excluded.target_ms,
    query_pattern = excluded.query_pattern,
    recommended_index = excluded.recommended_index;

create or replace view ops.database_health as
select
  (select count(*) from ops.data_quality_checks where checked_at >= now() - interval '1 hour' and severity = 'CRITICAL') as critical_data_quality_checks_1h,
  (select count(*) from ops.data_repair_jobs where status in ('PENDING', 'FAILED')) as pending_or_failed_repairs,
  (select count(*) from audit.event_store where created_at >= now() - interval '1 hour') as events_1h,
  (select count(*) from core.risk_events where created_at >= now() - interval '1 hour' and severity = 'CRITICAL') as critical_risk_events_1h;

-- Optional roles. Managed services may require running this block with elevated privileges.
do $$
begin
  if not exists (select 1 from pg_roles where rolname = 'trading_app_writer') then
    create role trading_app_writer;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'trading_app_reader') then
    create role trading_app_reader;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'trading_analytics_reader') then
    create role trading_analytics_reader;
  end if;
  if not exists (select 1 from pg_roles where rolname = 'research_warehouse_writer') then
    create role research_warehouse_writer;
  end if;
exception
  when insufficient_privilege then
    raise notice 'Skipping role creation: insufficient privileges on managed database';
end $$;

do $$
begin
  if exists (select 1 from pg_roles where rolname = 'trading_app_reader')
    and exists (select 1 from pg_roles where rolname = 'trading_app_writer')
    and exists (select 1 from pg_roles where rolname = 'trading_analytics_reader')
    and exists (select 1 from pg_roles where rolname = 'research_warehouse_writer') then
    grant usage on schema market, core, audit, research, ops to trading_app_reader, trading_app_writer, trading_analytics_reader, research_warehouse_writer;
    grant select on all tables in schema market, core, audit, research, ops to trading_app_reader, trading_analytics_reader;
    grant select, insert, update on all tables in schema core, audit, ops to trading_app_writer;
    grant select, insert, update on all tables in schema research to research_warehouse_writer;
    grant execute on function audit.append_event(text, uuid, uuid, audit.event_type, jsonb, uuid, uuid) to trading_app_writer;
    grant execute on function audit.replay_events(text, uuid) to trading_app_reader, trading_app_writer, trading_analytics_reader;
    grant execute on function ops.run_market_data_quality_check(text, timestamptz, timestamptz) to trading_app_writer;
    alter default privileges in schema core grant select on tables to trading_app_reader, trading_analytics_reader;
    alter default privileges in schema market grant select on tables to trading_app_reader, trading_analytics_reader;
    alter default privileges in schema research grant select on tables to trading_analytics_reader;
  else
    raise notice 'Skipping grants because one or more Phase 7 roles are unavailable';
  end if;
exception
  when insufficient_privilege then
    raise notice 'Skipping grants: insufficient privileges on managed database';
end $$;

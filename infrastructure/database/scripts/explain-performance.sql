-- Phase 7 performance smoke plans.
-- Run with psql against staging after loading representative data:
--   psql "$DATABASE_URL" -v account_id="'00000000-0000-0000-0000-000000000000'" -f infrastructure/database/scripts/explain-performance.sql

explain (analyze, buffers)
select *
from core.orders
where status in ('PENDING', 'SUBMITTED', 'PARTIAL')
order by created_at desc
limit 50;

explain (analyze, buffers)
select *
from core.orders
where strategy_version_id = '00000000-0000-0000-0000-000000000000'
order by created_at desc
limit 100;

explain (analyze, buffers)
select *
from core.positions
where account_id = :account_id::uuid
  and status = 'OPEN'
order by opened_at desc;

explain (analyze, buffers)
select *
from core.strategy_health
where strategy_version_id = '00000000-0000-0000-0000-000000000000'
order by last_updated desc
limit 1;

explain (analyze, buffers)
select *
from core.portfolio_snapshots
where account_id = :account_id::uuid
order by snapshot_at desc
limit 1440;

explain (analyze, buffers)
select *
from research.backtests
where strategy_version_id = '00000000-0000-0000-0000-000000000000'
order by created_at desc
limit 100;

explain (analyze, buffers)
select *
from market.market_ticks
where symbol = 'BTCUSD'
  and time >= now() - interval '5 minutes'
order by time desc
limit 1000;

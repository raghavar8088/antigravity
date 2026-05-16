-- BTC futures paper trades (server-side insert via service role API only).
-- Run in Supabase Dashboard → SQL Editor (or via Supabase CLI migrate).

create table if not exists public.paper_trades (
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  account_key text not null,
  client_trade_id uuid not null unique,
  opened_at timestamptz not null,
  closed_at timestamptz not null,
  symbol text not null,
  strategy_id integer not null,
  strategy_name text not null,
  side text not null,
  entry_price double precision not null,
  exit_price double precision not null,
  contracts double precision not null,
  notional double precision not null,
  margin_used double precision not null,
  gross_pnl double precision not null,
  fees double precision not null,
  funding_costs double precision not null default 0,
  net_pnl double precision not null,
  exit_reason text not null,
  payload jsonb
);

create index if not exists paper_trades_account_closed_at_idx
  on public.paper_trades (account_key, closed_at desc);

comment on table public.paper_trades is
  'Closed BTC futures paper trades; RLS off for v1 — access via Next.js API + service role only.';

-- v1: no RLS (service role bypasses anyway). Follow-up: enable RLS + anon read policies per account_key.

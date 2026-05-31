-- Institutional risk audit trail for portfolio-level rejections, warnings,
-- overrides, and circuit-breaker actions.

create table if not exists public.risk_audit_events (
  id uuid primary key default gen_random_uuid(),
  account_key text not null,
  event_type text not null,
  severity text not null default 'INFO',
  symbol text,
  strategy text,
  strategy_family text,
  reason text not null,
  portfolio_heat_pct double precision,
  var95_usd double precision,
  cvar95_usd double precision,
  gross_exposure_usd double precision,
  net_exposure_usd double precision,
  drawdown_pct double precision,
  payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists risk_audit_events_account_created_idx
  on public.risk_audit_events (account_key, created_at desc);

create index if not exists risk_audit_events_type_created_idx
  on public.risk_audit_events (event_type, created_at desc);

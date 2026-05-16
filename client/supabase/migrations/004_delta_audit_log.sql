-- P3-B: optional audit trail for manual testnet ops (server insert via service role).

create table if not exists public.delta_audit_log (
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  user_id text not null,
  action text not null,
  symbol text,
  side text,
  size double precision,
  order_id text,
  status text,
  payload jsonb
);

create index if not exists delta_audit_log_user_created_idx
  on public.delta_audit_log (user_id, created_at desc);

comment on table public.delta_audit_log is
  'Manual Delta testnet operator actions; always mirrored in-memory on server if insert fails.';

-- RLS off: access via service role API only (same pattern as early paper_trades v1).

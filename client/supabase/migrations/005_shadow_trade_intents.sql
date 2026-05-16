-- P3-C: paper shadow intents (compare paper desk actions vs future testnet live; no orders placed).

create table if not exists public.shadow_trade_intents (
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  user_id text not null,
  client_intent_id uuid not null default gen_random_uuid(),
  intent_kind text not null check (intent_kind in ('open', 'close')),
  symbol text not null,
  side text not null,
  notional double precision not null,
  entry_price double precision not null,
  exit_price double precision,
  exit_reason text,
  strategy_id integer not null,
  strategy_name text,
  would_place_testnet boolean not null default false
);

create unique index if not exists shadow_trade_intents_client_intent_id_idx
  on public.shadow_trade_intents (client_intent_id);

create index if not exists shadow_trade_intents_user_created_idx
  on public.shadow_trade_intents (user_id, created_at desc);

alter table public.shadow_trade_intents enable row level security;

drop policy if exists shadow_trade_intents_select_own on public.shadow_trade_intents;
create policy shadow_trade_intents_select_own
  on public.shadow_trade_intents
  for select
  to authenticated
  using (user_id = auth.uid()::text);

comment on table public.shadow_trade_intents is
  'Paper desk shadow log for signed-in users; inserts via Next.js API + service role; no Delta execution.';

-- Strategy promotions and research verdicts (Research Tournament mode).
-- Run in Supabase Dashboard → SQL Editor after 005_shadow_trade_intents.sql.

create table if not exists public.strategy_promotions (
  id uuid primary key default gen_random_uuid(),
  created_at timestamptz not null default now(),
  user_id uuid not null references auth.users(id) on delete cascade,
  strategy_id integer not null,
  status text not null check (status in ('winner', 'retired')),
  promoted_at timestamptz not null default now(),
  note text,
  -- One row per user × strategy × status (upsert friendly)
  unique (user_id, strategy_id)
);

create index if not exists strategy_promotions_user_idx
  on public.strategy_promotions (user_id, status);

comment on table public.strategy_promotions is
  'BTC FT research tournament: user-promoted winners and retired losers. Paper-only — no mainnet impact.';

-- RLS: users own their own rows
alter table public.strategy_promotions enable row level security;

create policy "users_select_own_promotions"
  on public.strategy_promotions for select
  using (auth.uid() = user_id);

create policy "users_insert_own_promotions"
  on public.strategy_promotions for insert
  with check (auth.uid() = user_id);

create policy "users_update_own_promotions"
  on public.strategy_promotions for update
  using (auth.uid() = user_id)
  with check (auth.uid() = user_id);

create policy "users_delete_own_promotions"
  on public.strategy_promotions for delete
  using (auth.uid() = user_id);

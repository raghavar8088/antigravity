-- P1-L: RLS on paper_trades — authenticated users read/insert own rows only.
-- Next.js API uses service role after session check; direct anon client access is scoped by policies.

alter table public.paper_trades enable row level security;

drop policy if exists paper_trades_select_own on public.paper_trades;
create policy paper_trades_select_own
  on public.paper_trades
  for select
  to authenticated
  using (account_key = auth.uid()::text);

drop policy if exists paper_trades_insert_own on public.paper_trades;
create policy paper_trades_insert_own
  on public.paper_trades
  for insert
  to authenticated
  with check (account_key = auth.uid()::text);

comment on table public.paper_trades is
  'Closed BTC futures paper trades; account_key = auth user id; RLS for authenticated direct access; service role for admin/API.';

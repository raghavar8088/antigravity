-- One-time manual migration (Supabase SQL Editor).
-- Copy legacy namespace rows to your authenticated user id after first sign-in.
--
-- 1. Sign in to the app once (magic link) and note your user id:
--    select id, email from auth.users order by created_at desc limit 5;
--
-- 2. Replace placeholders below, then run:

-- update public.paper_trades
-- set account_key = '<YOUR_AUTH_USER_UUID>'
-- where account_key = 'btc_future_trading_20';

-- Optional: verify counts
-- select account_key, count(*) from public.paper_trades group by 1 order by 2 desc;

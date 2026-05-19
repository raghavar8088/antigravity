-- Migration 007: Add module_key column to paper_trades for per-module filtering.
--
-- Background: paper trades now originate from multiple workspace tabs
-- (BTC Future Trading, Future Trading, BTC Option Buying/Selling, Nifty, etc.).
-- Adding module_key lets the dashboard scope leaderboards/exports per tab
-- without joining on strategy_name patterns.
--
-- Safe to run on a live table — column is nullable so existing rows stay valid.
-- Index is created CONCURRENTLY so it won't block writes during creation.

ALTER TABLE public.paper_trades
  ADD COLUMN IF NOT EXISTS module_key TEXT;

COMMENT ON COLUMN public.paper_trades.module_key IS
  'Workspace module identifier (e.g. btc_future_trading, btc_futures_scalper, btc_option_buying). Nullable for rows persisted before the column existed.';

-- Index supports the common per-module list query:
--   SELECT * FROM paper_trades
--   WHERE account_key = $1 AND module_key = $2
--   ORDER BY closed_at DESC LIMIT N;
CREATE INDEX CONCURRENTLY IF NOT EXISTS paper_trades_account_module_closed_idx
  ON public.paper_trades (account_key, module_key, closed_at DESC);

-- Optional: enforce known module keys via CHECK constraint.
-- Commented out so unknown keys persist (helpful during a new module rollout)
-- but uncomment + run after the next deploy to lock the schema.
--
-- ALTER TABLE public.paper_trades
--   ADD CONSTRAINT paper_trades_module_key_check
--   CHECK (
--     module_key IS NULL OR module_key IN (
--       'btc_future_trading',
--       'btc_futures_scalper',
--       'btc_option_buying',
--       'btc_option_selling',
--       'nifty_option_buying',
--       'nifty_option_selling',
--       'nifty_bees'
--     )
--   );

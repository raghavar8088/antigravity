# Paper Desk Runbook

## BTC FT Research Tournament

1. Enable research mode in `client/.env.local`:

```shell
NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1
NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD=22
NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=1
NEXT_PUBLIC_BTC_FT_COOLDOWN_MUL=0.5
NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL=0.85
NEXT_PUBLIC_BTC_FT_SESSION=0-24
NEXT_PUBLIC_BTC_FT_DISABLE_AUTO_KILL=1
NEXT_PUBLIC_DESK_SLIPPAGE_BPS=5
```

2. Run research for about two weeks. The desk rotates 30 active strategies per 24h batch from the verified BTC FT pool, stays paper-only, and keeps funding, slippage, fee, liquidation, spread, category, priority, and 12-slot gates.

3. Use the Strategy Research / Tournament panel to watch verdicts:
   `INSUFFICIENT_DATA`, `CANDIDATE`, `WINNER`, `LOSER`.

4. Promote the best winners manually. This writes local winners and optionally mirrors to Supabase `strategy_promotions` when the migration is installed.

5. Switch back to production/live-prep paper:

```shell
NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=0
NEXT_PUBLIC_BTC_FT_WINNER_IDS=200,204,211,224,240,251,260,271,288,299
NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD=26
NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=0
```

Keep the winners roster at 15 IDs or fewer before any real-money review. This app does not promote to Delta mainnet automatically.

## Ranking Export

Run:

```shell
npm run research:rank-btc-ft
```

The script reads Supabase `paper_trades` using the service role key, merges optional replay rankings, and writes `fixtures/research/btc_ft_verdicts.json`.

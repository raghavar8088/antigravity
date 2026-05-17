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

## After Research -> Winners Only

1. In the Strategy Research / Tournament panel, promote only strategies with enough data and positive net expectancy.
2. Use **Export winners JSON** for an audit file and **Copy env line** to copy `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=...`.
3. Switch the BTC FT desk to production winners mode:

```shell
NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=0
NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1
NEXT_PUBLIC_BTC_FT_STRATEGY_IDS=200,204,211,224,240
NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD=26
NEXT_PUBLIC_BTC_FT_RELAX_CONFIRM=0
```

Winners-only mode runs promoted/manual winners only, capped at 20 IDs. It forces production-safe gates even if research mode was accidentally left on: threshold 26, relaxed confirmation off, cooldown 1x, min-move K 1x, and auto-kill on. If no winners are available, the desk shows "No winners - run research or set BTC_FT_STRATEGY_IDS" and does not start the full library.

Keep this as live-prep paper until you manually review results. There is no Delta mainnet auto-order path in this mode.

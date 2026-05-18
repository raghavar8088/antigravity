# BTC futures paper desk — operator runbook

Operations guide for the **BTC Future Trading** module (`client/`). Desk entry/exit logic is unchanged by auth or export; this doc covers configuration, auth, tuning order, and observability.

---

## Environment variables (`NEXT_PUBLIC_DESK_*`)

All are optional unless noted. Parsed in `futuresDeskPolicy.ts` (and replay CLI mirrors several flags).

| Variable | Default | Effect |
|----------|---------|--------|
| `NEXT_PUBLIC_DESK_SLIPPAGE_BPS` | `0` | Adverse entry/exit slippage (bps), clamp 0–50 |
| `NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS` | off | `1` = fetch Supabase rolling stats and auto-disable losers |
| `NEXT_PUBLIC_DESK_KILL_WINDOW_DAYS` | `14` | Rolling window for auto-disable (1–90) |
| `NEXT_PUBLIC_DESK_KILL_MIN_TRADES` | `5` | Min closes per strat before kill rules apply |
| `NEXT_PUBLIC_DESK_KILL_MAX_EXPECTANCY_USD` | `-0.05` | Disable if mean net/trade below this |
| `NEXT_PUBLIC_DESK_KILL_MAX_SUM_NET_USD` | `-1` | Disable if sum net below this |
| `NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL` | off | `1` = size opens from SL risk % of equity |
| `NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY` | `0.01` | Risk fraction for vol sizing (0.002–0.05) |
| `NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT` | `0.05` | Skip entry if \|last−mark\|/mark % exceeds cap |
| `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY` | `3` | Max concurrent opens per strategy `category` (1–12) |
| `NEXT_PUBLIC_DESK_ENTRY_UTC_START` | `0` | UTC entry window start hour (0–23) |
| `NEXT_PUBLIC_DESK_ENTRY_UTC_END` | `24` | UTC entry window end hour (0–24); `0`+`24` = always open |
| `NEXT_PUBLIC_DESK_ENTRY_REPLACE_WEAKEST` | off | `1` = at max slots, close weakest priority for stronger signal |
| `NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY` | `0.35` | Cap same-direction notional vs equity |
| `NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K` | `1` | ATR$ vs fee hurdle multiplier |
| `NEXT_PUBLIC_DESK_MIN_TP_SL_RATIO` | `2` | Min TP/SL ratio for desk RR filter |
| `NEXT_PUBLIC_DESK_ENABLE_FAKE_DIV_STRATS` | off | `1` = include branded fake-diversity strats |
| `NEXT_PUBLIC_DESK_REPLAY_UI` | off | `1` = enable dev replay panel API in production |

**Dev-only** (require `NODE_ENV=development`):

| Variable | Default | Effect |
|----------|---------|--------|
| `NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE` | off | `window.__deskHoldTuningDump()` |
| `NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS` | `0` | Throttled auto console export |
| `NEXT_PUBLIC_DESK_REGIME_WATCH_MS` | `0` | Rolling regime histogram poll |
| `NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW` | `200` | Samples kept for regime watch |
| `NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST` | off | Persist 24h regime histogram to `localStorage` |

**Supabase (not `DESK_*` but required for cloud):**

| Variable | Purpose |
|----------|---------|
| `NEXT_PUBLIC_SUPABASE_URL` | Project URL |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Auth + RLS client |
| `SUPABASE_SERVICE_ROLE_KEY` | Server API after session check |

Example baseline: see `client/.env.local.example`.

---

## Auth setup

1. Supabase → **Authentication** → enable **Email** (magic link).
2. **Redirect URLs:** `http://localhost:3000/auth/callback` (+ production origin).
3. Set `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_ROLE_KEY` in `.env.local`.
4. Run SQL migrations in order:
   - `supabase/migrations/001_paper_trades.sql` — table + index
   - `supabase/migrations/002_paper_trades_rls.sql` — RLS (`account_key = auth.uid()::text`)
   - `003_migrate_legacy_account_key.sql` — **manual** one-time copy from `btc_future_trading_20` → your user id
5. Sign in via desk header or `/sign-in`.

### Logged-in vs logged-out

| | Logged out | Logged in |
|---|------------|-----------|
| `storageNamespace` (e.g. `btc_future_trading_20`) | `{namespace}_paper_state` in `localStorage` | Same — UI prefs / manual disables only |
| Cloud `account_key` | none | `auth.users.id` |
| Trade sync / leaderboard / stats / **CSV export** | **401** (local paper still runs) | Full cloud history, same on all devices |

---

## Tuning order (recommended)

Apply one change at a time; watch **Desk profile** skip counters and session metrics before the next step.

1. **Slippage** — `NEXT_PUBLIC_DESK_SLIPPAGE_BPS` (e.g. `5`). Aligns paper fills with realistic friction; check fee / \|gross\| and expectancy.
2. **Kill / auto-disable** — `NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS=1` plus kill window/min trades/expectancy thresholds. Confirm auto-disabled count and that manual disables still union correctly.
3. **Leaderboard disable** — Use bottom-10 **Disable** in UI (manual list in `localStorage`); does not remove P1-B auto-disable.
4. **Category cap** — `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY`; watch `deskSkippedCategoryCap`.
5. **UTC session** — `NEXT_PUBLIC_DESK_ENTRY_UTC_START` / `END`; watch `deskSkippedOutsideSession`.

Optional later: vol-sized notional, spread cap, entry replace-weakest, same-dir cap — each has its own skip stat on the desk profile panel.

---

## Strategy profile A/B

Set on `BTCFutureTradingScalper` via `strategyProfile` (passed through to `useBTCFuturesScalperEngine`). Default route uses **baseline**. For a clean compare, fork a page and use a **different** `storageNamespace` so `localStorage` paper state does not mix.

| Profile | `strategyProfile` | Signal threshold Δ | Min-move K mul | Hold × | Cooldown × |
|---------|-------------------|--------------------|----------------|--------|------------|
| Baseline | (omit) | 0 | 1.0 | 1.0 | 1.0 |
| ScalpAggro v1 | `scalp_aggro_v1` | −4 (easier) | 1.0 | 0.85 | 0.65 |
| FeeAware v1 | `fee_aware_v1` | +6 (stricter) | 1.25 | 1.0 | 1.0 |

**FeeAware v1** (`fee_aware_v1`): raises the signal bar and multiplies the desk ATR-vs-fees hurdle (`NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K × 1.25`) without changing hold or cooldown — watch `deskSkippedMinExpectedMove` and entry count vs baseline.

Example:

```tsx
<BTCFutureTradingScalper strategyProfile="fee_aware_v1" />
```

Replay: pass `strategyProfile: "fee_aware_v1"` in replay config (golden tests stay on baseline).

---

## Metrics to watch (Desk profile)

- **Session:** trades/hr, expectancy/trade, fee / \|gross\|, hold avg/median/P95.
- **Skips:** ATR vs fees, same-dir cap, regime, entry priority, spread, category cap, outside UTC session.
- **Exits:** exit-reason summary (last 400 closes); worst TIME offenders table.
- **Risk:** open positions, liquidation risk count, unrealized vs closed PnL.
- **Cloud (signed in):** leaderboard top/bottom; auto-disabled strat count when P1-B is on.

Export closed trades for spreadsheets: **Export CSV (30d)** → `GET /api/paper-trades/export?window_days=30`.

---

## Replay & compare

Live desk polls ~4s; replay is **1 bar = 1 step** — counts will not match tick-for-tick.

```bash
cd client
npm run replay:fetch -- --symbol=BTCUSD --bars=500
npm run replay
npm run replay -- --fixture=live --slippageBps=5 --volSized=1 --maxOpenPerCategory=3
npm run replay:compare -- --account_key=<YOUR_USER_UUID> --date=2026-05-16
```

Golden tests: synthetic fixture, `slippageBps=0`, session/category caps off by default.

Dev API: `GET /api/paper-replay` when `NODE_ENV=development` or `NEXT_PUBLIC_DESK_REPLAY_UI=1`.

---

## API quick reference (session required)

| Route | Notes |
|-------|--------|
| `POST /api/paper-trades` | Upsert close; `account_key` = session user id |
| `GET /api/paper-trades` | List closes (paginated) |
| `GET /api/paper-trades/leaderboard` | Strategy PnL rollup |
| `GET /api/paper-trades/strategy-stats` | Per-strat stats for auto-disable |
| `GET /api/paper-trades/export?window_days=30` | `text/csv` download |
| `GET /api/auth/session` | `{ user: { id, email } }` |

---

## Verification

```bash
cd client
npm run test
npm run build
```

---

## All trades red in research — troubleshooting

If research mode is placing many trades but net PnL is almost always negative,
work through these in order. Each step has a verification query.

### 1) PROFIT_LOCK firing on micro-gains that lose to fees

**Symptom:** trades with `exit_reason = 'PROFIT_LOCK'` and `net_pnl` between
−$0.03 and −$0.10. Strategy hits the lock at +0.3–0.5% return-on-margin (gross)
but books a loss after the round-trip taker fees and 5 bps exit slippage.

**Fix:** the engine now requires a positive projected net (after fees + slip)
before firing PROFIT_LOCK. Tune via env:

```bash
NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_NET_USD=0.05     # default; raise to be stricter
NEXT_PUBLIC_DESK_PROFIT_LOCK_MIN_PROGRESS=0.55    # default progress gate
```

**Verify:**

```sql
SELECT count(*) FILTER (WHERE exit_reason = 'PROFIT_LOCK' AND net_pnl < 0)::float /
       NULLIF(count(*) FILTER (WHERE exit_reason = 'PROFIT_LOCK'), 0) AS pl_loss_ratio
FROM paper_trades WHERE closed_at >= now() - interval '24 hours';
```
Should drop from ~80% to <20% after the fix.

### 2) Synthetic +$2.00 outliers from the win floor

**Symptom:** rare +$2.00 wins on BTCFT_VWAP_V0_SHORT_* (or any strategy) that
cluster at *exactly* $2.00. These are `MIN_ABS_NET_PNL_USD = 2` flooring tiny
gross wins (e.g. +$0.01 raw) up to $2 for display polish.

**Fix:** in research mode the floor defaults to 0 (raw expectancy). To force it
to 0 everywhere:

```bash
NEXT_PUBLIC_DESK_MIN_ABS_NET_WIN_USD=0
```

**Verify:**

```sql
SELECT count(*) AS floored_two_dollar_wins
FROM paper_trades
WHERE closed_at >= now() - interval '24 hours'
  AND net_pnl >= 1.99 AND net_pnl <= 2.01;
```
Should drop to ~0 in research mode.

### 3) Template-family churn paying fees N times for one signal

**Symptom:** within-minute concurrent opens of `BTCFT_VWAP_V0_LONG_204`,
`*_244`, `*_284` (same template, different pool slots). One signal → 3× the
round-trip fee bill.

**Fix:** research mode now dedupes by template family — at most one open
position per `BTCFT_<TPL>_V<n>_<SIDE>` family. Stat exposed in entry debug as
`deskSkippedTemplateFamily`.

**Verify:**

```sql
WITH minute_buckets AS (
  SELECT date_trunc('minute', opened_at) AS bucket,
         regexp_replace(strategy_name, '_\d+$', '') AS template_key
  FROM paper_trades WHERE opened_at >= now() - interval '24 hours'
)
SELECT count(*) FROM minute_buckets
GROUP BY template_key, bucket HAVING count(*) > 1;
```
Should be ~0 after the fix.

### 4) Strategy fee-bleeding for days before LOSER

**Symptom:** strategies with sumNet between −$0.50 and −$1.50 never auto-retire
because the old threshold required sumNet < −$2 at 15+ trades.

**Fix:** v2 LOSER threshold tightened to 12 trades + sumNet < −$1 OR
expectancy < −$0.05. WINNER now also requires `feePctOfGross < 80%` to reject
fee-dominated edges.

### 5) Same strategy firing 30× per day

**Symptom:** one strategy hits ~30 trades/day, each net −$0.05 → −$1.50 daily
fee-bleed per strategy.

**Fix:** new daily strat cap (default 8 closes/day per strategy in research):

```bash
NEXT_PUBLIC_BTC_FT_DAILY_STRAT_CAP=8
```
Stat exposed in entry debug as `deskSkippedDailyStratCap`. Cooldown multiplier
also bumped 0.5 → 0.75 to slow re-entry on the same slot.

### 6) Min-move gate letting in marginal trades

**Symptom:** entries that barely clear the ATR-vs-fees hurdle, then fee-bleed
on close.

**Fix:** research-mode `NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL` raised 0.85 → 1.0
(full fee hurdle, no relaxation). Set explicitly to override:

```bash
NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL=1.0
```

### Don't do these (they fake wins, not find edge)

- Disable fees: hides the real cost of trading, makes any noise look profitable
- Lower slippage below 5 bps: production routing pays 5–15 bps adverse
- Add more strategies: the problem is exit logic + churn, not roster size
- Raise the $2 win floor: same as disabling fees — masks losses


---

## v2 expectancy features (5 toggles)

These features ship in commit-after-c9ced1b. They are **honest amplifiers**:
each requires real trade data to engage, and none can fake profits by
hiding costs.

### #1 + #2 — Capital allocation by edge

Per-strategy notional is scaled by a combined Kelly + Sharpe-vs-cohort
multiplier in `[0.25, 3.0]`. Strategies with proven edge get more notional;
losers get less. Below 20 trades the multiplier is locked at 1.0.

```bash
NEXT_PUBLIC_DESK_ALLOCATION_BY_EDGE=1   # enable
```

**When to enable:** After ≥3 days of paper data. Earlier is safe but useless
(every strategy sits at 1.0×). Stat: `deskAllocationScaledCount` in entry debug
counts opens where the multiplier diverged from 1.0 by more than 5%.

**Verify behavior:**

```sql
SELECT strategy_name,
       avg(notional) AS avg_notional,
       count(*) AS trades
FROM paper_trades
WHERE closed_at >= now() - interval '24 hours'
GROUP BY strategy_name
ORDER BY avg_notional DESC LIMIT 10;
```

After 3+ days with `ALLOCATION_BY_EDGE=1`, top-Sharpe strategies should show
visibly larger `avg_notional` than bottom-Sharpe peers.

### #3 — Adaptive TP

`strat.tpPct` is scaled by current ATR/price:

| ATR/price | Multiplier | Effect |
|---|---|---|
| ≤ 0.10% | 0.8× | Tighten (low vol — TP would never be hit) |
| 0.25% | 1.0× | Neutral pivot |
| 0.50% | 1.2× | Widen (elevated vol) |
| ≥ 0.80% | 1.4× | Max widen (high vol — let winners run) |

**Default: ON** (`NEXT_PUBLIC_DESK_ADAPTIVE_TP=1`). Disable explicitly:

```bash
NEXT_PUBLIC_DESK_ADAPTIVE_TP=0
```

Stat: `deskAdaptiveTpAppliedCount` counts opens where the TP shifted by >5%
from the strategy's nominal.

### #4 — Time-of-day session gate

Blocks entry at UTC hours where rolling stats show winRate < 35% AND
expectancy < 0. Engages only after ≥50 total trades for the strategy AND
≥5 trades in the queried hour. Otherwise allows all entries.

```bash
NEXT_PUBLIC_DESK_SESSION_GATE=1   # enable
```

**When to enable:** After ≥1 week of paper data. Stat:
`deskSkippedOutsideProvenSession`.

### #5 — Correlation-aware caps

In addition to `MAX_OPEN_POSITIONS=12`:
- Max LONG positions concurrently: `NEXT_PUBLIC_DESK_MAX_OPEN_PER_SIDE` (default 6)
- Max SHORT positions concurrently: same env
- Max per template family (e.g. `BTCFT_VWAP_V0_LONG_*`): `NEXT_PUBLIC_DESK_MAX_OPEN_PER_TEMPLATE` (default 2)

Forces book diversity. Stats: `deskSkippedSideCap`, `deskSkippedTemplateCap`.

**Always on** — these defaults apply automatically. To loosen:

```bash
NEXT_PUBLIC_DESK_MAX_OPEN_PER_SIDE=12
NEXT_PUBLIC_DESK_MAX_OPEN_PER_TEMPLATE=4
```

### Recommended rollout

1. **Week 1**: defaults only (`ADAPTIVE_TP=1` + correlation caps active).
   Collect baseline data.
2. **Week 2**: enable `ALLOCATION_BY_EDGE=1`. Watch top-strategy notional
   diverge from bottom.
3. **Week 3**: enable `SESSION_GATE=1`. Expect 10–20% drop in trade count
   focused on weak hours.
4. **Week 4+**: review verdicts in StrategyResearchPanel. Promote winners,
   retire losers. Flip `WINNERS_ONLY=1` for production.


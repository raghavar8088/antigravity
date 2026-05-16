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

# USER TRUST REPORT — Phase 17
Generated: 2026-06-11 | Forensic Code Audit

---

## TRUST VERDICTS BY METRIC

---

### 1. PnL — PARTIALLY_TRUSTED

**Evidence trace:**

The Go engine calculates PnL in two places:

- **Realized PnL**: `engine/internal/paperpersist/portfolio_ledger.go:55` — `l.RealizedPnL += netPnL` incremented on every `RecordClose`. Written to `paper_state.realized_pnl` every 10s via `state_snapshotter.go`.
- **Unrealized PnL**: `engine/internal/trading/paperpersist_hooks.go:85-101` — `computeUnrealizedPnL` uses mark-price from `o.exec.GetLastPrice()` and sums `(mark - entry) * size` per position. Snapshot written every 10s.
- **Frontend formula**: `client/src/lib/paperDeskPositionMath.ts:16-24` — UI recalculates unrealized PnL from position docs + live BTC price. Formula matches engine (same `(mark - entry) * size` logic). Both formulas are consistent.
- **Aggregation**: `client/src/app/api/paper-desk/snapshot/route.ts:43-53` — `buildPortfolioAccountingSnapshot` recalculates unrealized PnL from MongoDB `paper_positions` collection (not the cached `paper_state.unrealized_pnl`). This means the UI can show slightly different unrealized PnL from the engine's own snapshot if positions were modified between the 10s snapshots.

**Scenarios where display could be wrong:**

1. **Staleness gap**: Engine snapshots `paper_state` every 10s; UI polls every 5s. The UI may read a `paper_state` doc that is up to 10s old while simultaneously recalculating unrealized PnL from `paper_positions` (more current). This creates a split where realized PnL (from `paper_state`) and unrealized PnL (recomputed) come from documents at different timestamps.
2. **Portfolio consistency validation tolerance**: `client/src/lib/portfolioConsistencyValidation.ts:33` — `PNL_TOLERANCE_USD = 50`. Up to $50 drift between `paper_state.realized_pnl` and `SUM(paper_trades.net_pnl)` is considered a PASS. Users can see $49.99 of phantom drift without any warning.
3. **BTC price source mismatch**: Engine uses Delta/Binance REST for marking; UI uses Binance primary → Coinbase fallback (`/api/btc/price`). If engine and UI are using different exchange prices simultaneously, unrealized PnL will differ.
4. **Terminal pages**: `/terminal/risk` and `/terminal/execution` show PnL from `initialTerminalSnapshot` (hardcoded values `$166.05`, `$40.42`). These are completely detached from real PnL.

---

### 2. Positions — PARTIALLY_TRUSTED

**Evidence trace:**

- Engine writes to `paper_positions` collection on every position open/close via `paperpersist/writer.go`.
- UI reads `paper_positions` via `/api/paper-desk/positions/route.ts` → `listPositions(accountKey, status, limit)` in `paperDeskClient.ts`.
- Position filter: only `status = "OPEN"` positions are shown in the snapshot; `listOpenPositions` uses a MongoDB query. `client/src/lib/paperDeskClient.ts`.
- Position count in snapshot: `data.open_positions ?? []` in `usePaperDesk.ts:91`.

**Scenarios where display could be wrong:**

1. **Position closed in engine but not yet reflected**: The engine closes a position and writes to MongoDB. The UI polls every 5s. During the gap, a closed position may still show as OPEN.
2. **Terminal pages**: `/terminal/execution` shows 2 hardcoded positions unrelated to actual open positions. There is no live connection.
3. **Mark price for unrealized PnL on positions table**: UI recalculates unrealized PnL per position using live BTC price from `/api/btc/price`. If BTC price API is down, all position PnL shows as `0` — same formula path in `paperDeskPositionMath.ts:19` returns 0 if `markPrice <= 0`.

---

### 3. Fills / Trade History — TRUSTED

**Evidence trace:**

- Engine writes closed trades to `paper_trades` MongoDB collection with full details: `client_trade_id`, `entry_at`, `exit_at`, `net_pnl`, `fees`, `fill_quality`, `slippage_bps`.
- UI reads via `/api/paper-desk/trades/route.ts` with pagination support.
- No client-side augmentation — data is passed through directly from MongoDB.
- Trade count cross-validation: `portfolioConsistencyValidation.ts:58-65` — `countDocuments` vs `aggregate SUM` comparison ensures no orphaned trades.

**Scenarios where display could be wrong:**

1. **Last 20 recent trades only in snapshot** — `snapshot/route.ts:35`: `listPaperTrades({ accountKey, limit: 20 })`. The snapshot only shows 20 trades; full history requires navigating to the Trades tab which calls `fetchTradesPage()`.
2. **Sort order**: Recent trades in snapshot sorted by MongoDB default insertion order (typically by `_id`/time). If a trade write is delayed, ordering could appear non-chronological.

---

### 4. Risk Metrics — UNTRUSTED (for Terminal pages) / PARTIALLY_TRUSTED (for Paper Desk)

**Evidence trace:**

**Paper Desk risk (PARTIALLY_TRUSTED):**
- Drawdown: calculated in `portfolioAccountingService.ts:29-80` from equity curve + `paper_state.peak_equity`. Uses MongoDB equity_curve collection for rolling 24h/weekly drawdown.
- Portfolio heat: `paper_state.current_drawdown` from engine; recalculated on snapshot.

**Terminal risk (UNTRUSTED):**
- VaR 95 = `$1,840`, VaR 99 = `$2,760`, CVaR 95 = `$2,380` — hardcoded in `terminalSnapshot.ts:68-70`.
- Drawdown `1.4%` — hardcoded. `terminalSnapshot.ts:73`.
- Portfolio heat `3.7%` — hardcoded. `terminalSnapshot.ts:74`.
- `NEXT_PUBLIC_TERMINAL_WS_URL` not configured in `.env.local` → WebSocket never connects → initial snapshot is permanent state.

**Scenarios where display could be wrong:**

1. The entire `/terminal/risk` page shows permanently stale values from a static snapshot built at compile time. Any user reading this page believes they are seeing live risk when the data is hardcoded.
2. The "Exposure By Family" panel uses hardcoded percentages (`Funding: 24%`, `Order Flow: 19%`, etc.) regardless of what strategies are actually running.

---

### 5. Strategy State — PARTIALLY_TRUSTED

**Evidence trace:**

- Engine writes to `strategy_health` and `strategy_scores` collections every 15 minutes via `paperpersist/strategy_health.go`.
- UI reads via `/api/paper-desk/strategy-health/route.ts`.
- Health status correctly tracks HEALTHY/WARNING/CRITICAL/INSUFFICIENT_DATA based on win rate, expectancy, profit factor, drawdown.
- Thresholds: `warnWinRate: 0.45`, `critWinRate: 0.35`, `minSampleSize: 20` — `paperpersist/strategy_health.go:34-41`.

**Scenarios where display could be wrong:**

1. **15-minute staleness**: Health computed every 15 minutes. A strategy that collapses between computation windows shows stale HEALTHY status.
2. **Terminal strategy table**: Shows 5 hardcoded strategies. User cannot determine real strategy count or actual enabled/disabled state from the terminal.
3. **`minSampleSize = 20`**: New strategies show `INSUFFICIENT_DATA` rather than being evaluated, potentially hiding early losses.

---

### 6. Portfolio Summary — PARTIALLY_TRUSTED

**Evidence trace:**

- `buildPortfolioAccountingSnapshot` in `portfolioAccountingService.ts` aggregates: MongoDB `paper_state` + `paper_positions` + `paper_trades`.
- The service independently recalculates unrealized PnL, exposure, and drawdown rather than trusting `paper_state` directly — a correct design.
- Exposed via `/api/paper-desk/portfolio/route.ts` and embedded in the snapshot.

**Scenarios where display could be wrong:**

1. **No mark price in accounting service**: `portfolioAccountingService.ts` calls `unrealizedPnlForOpenPosition` which requires a mark price parameter. The `buildPortfolioAccountingSnapshot` path uses `getPortfolioAccountingSnapshot` which fetches equity curve for drawdown but does **not** pass live BTC price to unrealized PnL — it derives mark from positions' entry data via an indirect path. Needs verification.
2. **Starting balance constant**: Both `PAPER_DESK_STARTING_BALANCE` (TypeScript) and `futuresInitialCapitalUSD = 1000000.0` (Go) must remain in sync manually. No automated check.

---

## SILENT FAILURES AND ERROR SWALLOWING

| File | Line | Pattern | Risk |
|---|---|---|---|
| `client/src/components/CommandCenter.tsx` | 62, 79 | `catch {}` — bridge status and pending signals fetch silently fail. Engine bridge appears offline without any user notification. | HIGH |
| `client/src/lib/btcFtResearch.ts` | 338, 354 | `.catch(() => {})` — research strategy metadata fetch fails silently. | MEDIUM |
| `client/src/hooks/useBTCFuturesScalperEngine.ts` | 1413 | `.catch(() => {})` — persistence write failure silently discarded. Trade may not be saved. | HIGH |
| `client/src/hooks/useNiftyCandles.ts` | 34 | `fetch("/api/nifty/seed-engine").catch(() => {})` — engine seed fails silently. | MEDIUM |
| `client/src/lib/mongoTradesClient.ts` | 433 | Index creation `.catch(() => {})` — if MongoDB index creation fails, duplicate records may accumulate without warning. | MEDIUM |
| `client/src/lib/mockTradingMongo.ts` | 278 | `p.catch(() => {})` on an array of promises — bulk write failures swallowed. | HIGH |
| `client/src/lib/btcFtResearch.ts` | 338 | Async research runner `.catch(() => {})` — entire research signal batch silently fails. | MEDIUM |

---

## ERROR STATES THAT DON'T PROPAGATE TO USERS

1. **`usePaperDesk.ts:104-105`** — On fetch error after `live` state, connection transitions to `"stale"`. A banner shows stale status but the **last known data remains visible** with no indication of how stale it is. Users see data that may be 5 minutes or 5 hours old.

2. **`useLiveBTCPrice.ts:66-67`** — On fetch error, sets `connected: false` but does NOT clear the price. The last known BTC price remains displayed with a "Offline" indicator that is easy to miss. Unrealized PnL continues to be calculated against a potentially stale price.

3. **`client/src/app/api/btc/price/route.ts:88`** — If both Binance and Coinbase fail, returns `{ ok: false }`. The hook `useLiveBTCPrice.ts:49` has `if (!data.ok || !data.price) return;` — it silently returns without updating state. The last price remains.

4. **`CommandCenter.tsx:62,79`** — Both `fetchPending` and `fetchStatus` have empty `catch {}`. If the Go engine is unreachable, the Command Center shows `bridge.online = false` but there is no visible error message — the component still renders with the default offline state.

5. **`terminalStore.tsx:42`** — WebSocket `onmessage` has `catch { /* comment */ }` — malformed delta silently discarded. No error count, no user notification.

---

## OVERALL TRUST VERDICT

| Metric | Terminal Pages | Paper Desk Pages | BTC-Future-Trading |
|---|---|---|---|
| PnL | UNTRUSTED (hardcoded) | PARTIALLY_TRUSTED | PARTIALLY_TRUSTED |
| Positions | UNTRUSTED (hardcoded) | TRUSTED | TRUSTED |
| Fills | N/A | TRUSTED | TRUSTED |
| Risk | UNTRUSTED (hardcoded) | PARTIALLY_TRUSTED | NOT_DISPLAYED |
| Strategy State | UNTRUSTED (hardcoded) | PARTIALLY_TRUSTED | N/A |
| Portfolio | UNTRUSTED (hardcoded) | PARTIALLY_TRUSTED | N/A |

**Root cause for all UNTRUSTED terminal verdicts**: `NEXT_PUBLIC_TERMINAL_WS_URL` is not set in `.env.local`, so `terminalStore.tsx:22` causes the `if (!wsUrl) return;` guard to exit the WebSocket setup. The initial hardcoded snapshot is the **permanent** state. Evidence: `client/src/lib/terminal/terminalStore.tsx:22`, `client/src/lib/terminal/terminalSnapshot.ts:17`.

# State Duplication Report — Phase 14
# Generated: 2026-06-11 | Auditor: Claude Code Forensic Audit

---

## 1. Open Position Tracking — 5 Sources

| # | Location | Type | What it stores | Updated |
|---|---|---|---|---|
| 1 | `engine/internal/positions/manager.go` — `Manager.positions map[string]Position` | Go in-memory | All open BTC futures positions | Every trade open/close |
| 2 | `engine/internal/execution/paper.go` — `PaperClient.positionBTC float64` | Go in-memory | Net signed BTC position (scalar) | Every fill |
| 3 | `engine/internal/paperpersist/` — MongoDB `open_positions` collection | MongoDB | Open position records (per account_key) | paperpersist hooks on open/close |
| 4 | `engine/internal/persistence/store.go` — PostgreSQL `engine_state.positions` JSON blob | PostgreSQL | Snapshot of open positions array | StateSaver periodic save |
| 5 | MongoDB `paper_state.positions` field | MongoDB | Open positions array in account state doc | StateSnapshotter every 10s |

**Source of truth:** `positions.Manager` (item 1) is the live runtime source. Items 2-5 are derived/persisted copies.

**Divergence risk:** HIGH. `PaperClient.positionBTC` (item 2) is a scalar net position while `Manager.positions` (item 1) tracks individual positions with IDs, SL/TP, etc. They can diverge if `RestoreOpenPosition` (called on MongoDB recovery at `main.go:701`) doesn't perfectly match what `Manager` has. Evidence: `main.go:701` calls `paperExecute.RestoreOpenPosition(side, rp.Size)` independently of `posMgr.RestorePositions(mongoPositions)`.

**Scenario for incorrect UI:** Engine restarts, MongoDB recovery restores 3 positions into `posMgr` but only 2 into `paperExecute` (e.g. one has `side == ""` and falls through the if check at `main.go:683`). The equity calculation uses `paperExecute.positionBTC` for mark-to-market, so it would be wrong.

---

## 2. PnL Calculation — 4 Locations

| # | Location | What it calculates | Input |
|---|---|---|---|
| 1 | `engine/internal/execution/paper.go:185-189` — `GetEquityUSD()` | Real-time equity = balance + (positionBTC × lastPrice) | Engine in-memory |
| 2 | `engine/internal/trading/paperpersist_hooks.go:85` — `computeUnrealizedPnL()` | Unrealized PnL per open position | positions.Manager slice |
| 3 | `client/src/lib/portfolioAccountingService.ts` — `buildPortfolioAccountingSnapshot()` | Full accounting snapshot from MongoDB data | MongoDB reads |
| 4 | `client/src/lib/futuresDeskRuntime.ts:80` — `paperLinearGrossPnl()` | Per-position unrealized PnL for UI display | Client-side BTCFuturesTrade objects |

**Source of truth ambiguity:** Engine `GetEquityUSD()` uses scalar `positionBTC` while `computeUnrealizedPnL()` uses the `Manager` slice. These should be consistent but use different state sources. The frontend `portfolioAccountingService` computes drawdown, unrealized PnL from MongoDB data (possibly 10s stale). The frontend `futuresDeskRuntime` computes per-position PnL from client-side state (4s stale from worker fetch).

**Specific divergence scenario:** A position closes at the engine at T=0. The engine settles balance at T=0. MongoDB is updated at T=10 (next StateSnapshotter tick). Frontend polling at T=4 gets stale MongoDB state showing the position still open with unrealized PnL. Portfolio accounting service computes incorrect equity. User sees incorrect balance for up to 10 seconds.

---

## 3. Account Balance Tracking — 4 Sources

| # | Location | Type | Updated |
|---|---|---|---|
| 1 | `engine/internal/execution/paper.go` — `PaperClient.balanceUSD` | Go in-memory | Every fill (immediate) |
| 2 | MongoDB `paper_state.balance` | MongoDB | StateSnapshotter every 10s |
| 3 | PostgreSQL `engine_state.balance` | PostgreSQL | StateSaver periodic (interval not verified) |
| 4 | Next.js client state — `useBTCFuturesScalperEngine` hook balance state | Browser memory | Every 4s poll from MongoDB |

**Source of truth:** `PaperClient.balanceUSD` (item 1) is the live truth. Items 2-4 are progressively staler copies.

**Maximum staleness chain:** Trade fills at engine → up to 10s for MongoDB snapshot → up to 4s for Next.js poll = up to **14 seconds** of balance staleness in the UI.

**Evidence:** StateSnapshotter interval `10*time.Second` at `main.go:818`. Frontend poll `POLL_MS = 4000` at `usePaperDesk.ts:34`.

---

## 4. Strategy Enabled/Disabled State — 3 Locations

| # | Location | Type | Note |
|---|---|---|---|
| 1 | `engine/internal/strategy/curated_registry.go` — in-memory strategy list | Go in-memory | Loaded at boot from BuildCuratedScalpers() |
| 2 | MongoDB `paper_state.disabled_strategies[]` | MongoDB | Written by cron tick / paper desk worker |
| 3 | Frontend hook `useBTCFuturesScalperEngine` state | Browser memory | Read from MongoDB on 4s poll |

**Source of truth:** Go engine strategy list (item 1) is authoritative for which strategies CAN trade. MongoDB `disabled_strategies` (item 2) is a UI-level disable list read by the Next.js worker when `ENGINE_EXECUTION_AUTHORITY=0`. When `ENGINE_EXECUTION_AUTHORITY=1`, the engine ignores MongoDB `disabled_strategies` entirely and runs all curated strategies.

**Critical bug:** When `ENGINE_EXECUTION_AUTHORITY=1` (production), there is NO way for the UI to disable individual strategies on the running engine. The MongoDB `disabled_strategies` field is ignored. The engine runs all 600+ strategies regardless. The only way to stop a strategy is to modify `curated_registry.go` and redeploy.

---

## 5. Paper Trading State — Both Client and Engine

Both the client (TypeScript) and Go engine implement paper trading execution:

| Component | File | Executes trades? |
|---|---|---|
| Go engine `PaperClient` | `engine/internal/execution/paper.go` | YES — when ENGINE_EXECUTION_AUTHORITY=1 |
| Next.js `runPaperDeskPollTick` | `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` | YES — when ENGINE_EXECUTION_AUTHORITY=0 |
| Next.js cron tick | `client/src/app/api/cron/paper-desk-tick/route.ts` | YES — as fallback when VPS worker stale |

When `ENGINE_EXECUTION_AUTHORITY=1` (default), `paper-trades POST` returns 410 and the cron tick checks `isEngineExecutionAuthority()` and skips execution. But **both systems exist simultaneously** and the guard relies on a single environment variable.

**Risk:** If `ENGINE_EXECUTION_AUTHORITY` env var is accidentally cleared/missing from Vercel, `isEngineExecutionAuthority()` returns `true` (default), so the Next.js worker stays disabled. This is safe. But if it's set to "0" on Vercel while the Go engine is running, BOTH execute trades simultaneously, double-counting fills.

---

## 6. Strategy Performance State — 2 Sources

| # | Location | Updated |
|---|---|---|
| 1 | `engine/internal/risk/strategy_tracker.go` — StrategyTracker in-memory | Every fill (immediate) |
| 2 | MongoDB `strategy_health` collection | StrategyHealthMonitor every 15 minutes |

The UI reads strategy health from MongoDB (15-minute lag). The engine makes trading decisions based on in-memory tracker (live). A strategy that has just become unprofitable will still appear healthy in the UI for up to 15 minutes.

---

## 7. BTC Options State — 2 Sources

| # | Location | Type |
|---|---|---|
| 1 | `engine/internal/options/engine.go` — in-memory | Go in-memory (positions, trades, strategies) |
| 2 | PostgreSQL / FileSnapshot | Periodic save on state change hooks |

The Next.js frontend reads options state via the engine proxy (`/api/options/*`). This reads directly from the in-memory Go engine state (live). No duplication risk here — single authoritative source served live.

---

## 8. Summary — Sources of Truth Per Key Metric

| Metric | Sources of Truth Count | Actual Authority | Max UI Staleness |
|---|---|---|---|
| BTC paper balance | 4 | Go engine RAM | ~14s |
| BTC paper equity | 4 | Go engine RAM | ~14s |
| Open positions count | 5 | positions.Manager | ~14s |
| Realized PnL | 3 | MongoDB (from engine writes) | ~10s |
| Unrealized PnL | 3 | Go engine RAM | ~14s |
| Strategy enabled/disabled | 3 | Go engine (ignores UI control when auth=1) | N/A in prod |
| Strategy performance | 2 | Go engine in-memory | ~15 min |
| Kill switch state | 2 | PostgreSQL ledger | ~0 (in-memory cache) |
| NIFTY 50 price | 3 (engine cache, SSE, NSE REST) | Go engine in-memory | 3s (SSE) |

**VERDICT:** There are 4-5 sources of truth for most critical trading metrics. The system works correctly when `ENGINE_EXECUTION_AUTHORITY=1` and MongoDB writes are healthy. The worst-case divergence scenario is a MongoDB write failure combined with a stale PostgreSQL backup: the UI shows stale data while the engine continues trading correctly.

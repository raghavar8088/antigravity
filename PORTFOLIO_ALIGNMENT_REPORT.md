# PORTFOLIO ALIGNMENT REPORT — Forensic Audit Phase 8

**Date:** 2026-06-11  
**Scope:** Engine portfolio calculation, API routes, frontend portfolio display  
**Method:** Source code reading only. No assumptions.

---

## Where Portfolio State Is Calculated

### 1. In-Engine In-Memory: `PaperClient` (RAM)
**File:** `engine/internal/execution/paper.go`  
Tracks: `balanceUSD` (float64, line 25), `positionBTC` (float64, line 26), `totalFeesUSD` (float64, line 27)  
`GetEquityUSD()` (paper.go:185): `return balanceUSD + (positionBTC * lastKnownPrice)`  
This is the authoritative real-time account state. Never persisted by itself.

### 2. In-Engine In-Memory: `portfolioLedger` (PortfolioLedger)
**File:** `engine/internal/paperpersist/portfolio_ledger.go`  
Tracks cumulative: `RealizedPnL`, `TotalTrades`, `WinningTrades`, `LosingTrades`, `TotalFees`, `PeakEquity`, `CurrentDrawdown`, `MaxDrawdown`  
Updated via `RecordClose()` on every trade close (loop.go:1761).  
This is an accounting mirror — used by `GetAccountSnapshot()` to populate MongoDB writes.

### 3. In-Engine In-Memory: `TradeJournal`
**File:** `engine/internal/execution/trade_journal.go`  
Used when `portfolioLedger` is nil — provides aggregate stats from journal entries.  
`GetAggregateStats()` returns TotalPnL, TotalTrades, TotalWins, TotalLosses.

### 4. In-Engine In-Memory: `Risk V2 Engine`
**File:** `engine/internal/risk/v2/engine.go`  
Tracks `portfolio.Account.EquityUSD`, `portfolio.Account.DailyPnLUSD`, positions, heat, drawdown.  
Equity is synced from PaperClient via `risk.SyncEquity(o.exec.GetEquityUSD())` on every tick (loop.go:1063).

### 5. MongoDB `paper_state` (primary persistence)
**File:** `engine/internal/paperpersist/state_snapshotter.go`  
Written every **10 seconds** by `StateSnapshotter.Run()`.  
Source data: `orchestrator.GetAccountSnapshot()` which reads from PaperClient + portfolioLedger.  
Single upserted document per account key.

### 6. MongoDB `equity_curve`
**File:** `engine/internal/paperpersist/equity_recorder.go:67-90`  
Written every **1 minute** by `EquityRecorder.Run()`.  
Contains: equity, balance, unrealizedPnL, realizedPnL, drawdownPct, openPositions, BTCPrice.

### 7. MongoDB `paper_trades`
Written by `TradeWriter.Write()` on every close event.  
Canonical closed-trade record. TTL index exists.

### 8. MongoDB `paper_positions`
Written on open and close events via `order_writer`.  
Status field: OPEN or CLOSED.

---

## Where Portfolio State Is Stored

| Data | Storage | Write Cadence |
|------|---------|---------------|
| Balance/Equity | RAM (PaperClient) + MongoDB paper_state | RAM: real-time; MongoDB: 10s |
| Unrealized PnL | RAM (computed on demand) | Computed on each call |
| Realized PnL | RAM (portfolioLedger) + MongoDB paper_state | RAM: on trade close; MongoDB: 10s |
| Open Positions | RAM (positions.Manager) + MongoDB paper_positions | RAM: real-time; MongoDB: async |
| Closed Trades | RAM (journal) + MongoDB paper_trades | RAM: on close; MongoDB: async |
| Equity Curve | MongoDB equity_curve | 1 minute |
| Daily PnL | MongoDB daily_pnl_history | Once per day at midnight UTC |
| Risk State | RAM (Risk V2 Engine) | Real-time, never persisted |

---

## How UI Reads Portfolio Data

### Primary: `/api/paper-desk/snapshot` (polled every 5s)
Returns `state` from `paper_state` + `open_positions` from `paper_positions` + `recent_trades` (last 20) + `health_summary`.

### Secondary: `/api/paper-desk/portfolio` (on-demand)
**File:** `client/src/app/api/paper-desk/portfolio/route.ts`  
Calls `getPortfolioAccountingSnapshot()` which fetches:
- `paper_state`
- `paper_trades` (aggregated stats)
- `paper_positions` (open)
- `equity_curve` (last 2016 points)

Then computes: balance, equity, drawdown metrics, fees, extended stats (sharpe, sortino, calmar, profit_factor).

---

## Duplicate PnL Calculations

**Yes — PnL is calculated in multiple places with different results possible:**

### Location 1: Go engine `processCloseEvents()` (authoritative)
**File:** `engine/internal/trading/loop.go:1736-1741`
```go
feeBreakdown := execution.CanonicalTradeFees(entryPrice, exitPrice, size)
netPnL := execution.CanonicalNetPnL(event.PnL, feeBreakdown)
```
Fee = notional × 0.00050 × 2 (entry + exit rounds)

### Location 2: `emitPositionClosed()` (for OMS ledger)
**File:** `engine/internal/trading/loop.go:907-908`
```go
feesUSD := notional * execution.BinanceFuturesTakerFeePct * 2
```
Same formula, but uses entry price notional only. On positions where exit price differs significantly from entry, this fee calculation is slightly different from `CanonicalTradeFees`.

### Location 3: `persistClosedTrade()` (for MongoDB paper_trades)
**File:** `engine/internal/trading/paperpersist_hooks.go:396-402`
Re-calls `execution.CanonicalTradeFees()` to populate the `ClosedTrade` struct. This duplicates the fee calculation but uses the same function, so should match Location 1.

### Location 4: TypeScript `portfolioAccountingService.ts` (for UI display)
**File:** `client/src/lib/portfolioAccountingService.ts:106-110`
```ts
const realized = safeNum(closedStats.realized_pnl);  // from paper_trades aggregate
const grossPnl = safeNum(closedStats.gross_pnl, realized + safeNum(closedStats.total_fees));
```
The UI recomputes from MongoDB aggregated stats. If the underlying `paper_trades` records are accurate, this should match.

### Location 5: Client-side unrealized PnL
**File:** `client/src/lib/portfolioAccountingService.ts:95-104`
```ts
unrealized += unrealizedPnlForOpenPosition(pos, markPrice);
```
Computed from open positions using the snapshot mark price. This is separate from both engine-computed unrealized PnL (which uses `exec.GetLastPrice()`) and MongoDB-stored unrealized PnL.

**Risk of divergence:** If the mark price used by the UI differs from the engine's `lastKnownPrice`, unrealized PnL will differ between engine and UI.

---

## Paper Trading vs Live Trading Portfolio Separation

**BTC Paper Desk:**  
Account ID: `btc-paper-1` (loop.go:40)  
All MongoDB writes use `ownerAccountKey` from env (`PAPER_DESK_ACCOUNT_KEY` or derived).  
Engine RAM: single `PaperClient` instance with starting capital $1,000,000.

**BTC Options (Delta):**  
Separate `delta.Bridge` instance. Delta live orders go through `processDeltaExecutionRequest()` via the institutional path. Positions created get a `DELTA:` prefix symbol. These positions are tracked in the same `posMgr` as paper positions — **there is no separation** at the positions.Manager level.

**NIFTY Paper Desk:**  
Not wired through the Go engine OMS v3 path. NIFTY strategies are handled by `niftystocks` and `options_selling` engines which have their own state management separate from the BTC paper trading path.

**Verdict:** BTC paper and BTC live (Delta) share the same `positions.Manager` instance but use different symbols. NIFTY is a separate subsystem.

---

## BTC Paper Desk vs NIFTY Paper Desk Separation

| Aspect | BTC Paper | NIFTY |
|--------|-----------|-------|
| Engine | Go Orchestrator + PaperClient | Separate NIFTY engine |
| Account ID | `btc-paper-1` | Separate MongoDB collections |
| API routes | `/api/paper-desk/*` | `/api/nifty/*`, `/api/nifty-options/*` |
| OMS | OMS v3 + paperpersist | No OMS v3 |
| Kill switch | Go killswitch.Service | None found |

The two desks are separate code paths. No cross-contamination found.

---

## Portfolio State Consistency Concerns

### Concern 1: Balance divergence between RAM and MongoDB
The `PaperClient.balanceUSD` in RAM is the true balance. The MongoDB `paper_state.balance` is written every 10 seconds. In the 10 seconds between writes, they diverge. On engine restart, the engine **restores from MongoDB** (via `paperpersist.RecoverOpenPositions` and state), so the RAM balance is seeded from the 10-second-old MongoDB value. Any trades that occurred in the last 10 seconds before a crash are recovered from `paper_positions` and `paper_trades` but the balance may not be perfectly reconciled.

### Concern 2: Equity computed three different ways
1. Engine RAM: `balanceUSD + positionBTC × lastKnownPrice` (real-time)
2. MongoDB `paper_state.equity`: written from #1 every 10s
3. UI `portfolioAccountingService`: `balance + unrealized_pnl` where unrealized is from position records and a snapshot mark price

All three methods should agree, but with different mark prices and timing, they will produce different values.

### Concern 3: No portfolio reconciliation between engine and UI
The `runPortfolioConsistencyValidation()` function (`/api/paper-desk/validation` route) checks portfolio consistency but only within MongoDB (comparing `paper_state` to derived stats from `paper_trades`). It does not compare against live engine RAM state.

---

## Verdict: Is Portfolio State Trustworthy?

**MOSTLY TRUSTWORTHY for closed trade accounting; APPROXIMATE for live positions.**

- Closed trade records in `paper_trades` are accurate once written
- Account state (`paper_state`) is accurate within 10 seconds
- Unrealized PnL is approximate due to mark price timing differences between engine and UI
- No portfolio reconciliation against live engine RAM state
- Delta live positions share the same positions.Manager as paper positions (mixing concern)
- Engine crash recovery relies on MongoDB state being consistent with in-memory state at time of last snapshot (10s window of potential loss)

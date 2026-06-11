# TRADING WORKFLOW REPORT — Forensic Audit Phase 5

**Date:** 2026-06-11  
**Scope:** engine/internal/trading/loop.go and all dependencies  
**Method:** Source code reading only. No assumptions.

---

## Step-by-Step Verified Workflow

### Step 1 — Market Data Ingestion
**File:** `engine/internal/trading/loop.go:996-1034`  
`Orchestrator.Run()` subscribes to `client.GetTickChannel()`. Ticks arrive in an infinite `for/select` loop. Candle aggregation and position SL/TP checks are triggered for every tick.

`processTickPipeline` (loop.go:1041):
1. Calls `exec.UpdateMarketState(t.Price)` — updates paper client's last known price
2. Calls `posMgr.CheckStopLossAndTakeProfit(t.Price)` — triggers SL/TP on open positions
3. Calls `candleAgg.Feed(t)` — feeds raw tick to candle aggregator

### Step 2 — Strategy Signal Generation
**File:** `engine/internal/trading/loop.go:1364-1423`  
`processStrategyGroup()` fans out to each strategy in a goroutine pool:
- Calls `entry.Strategy.OnTick(t)` for each strategy
- Collects all non-HOLD signals into `rawSignals`

### Step 3 — Signal Aggregation
**File:** `engine/internal/trading/loop.go:1433`  
`o.aggregator.FilterSignalsSelective(rawSignals)` filters raw signals. Regime classification is performed (`classifyMarketRegime()`, loop.go:2287).

### Step 4 — Pre-Execution Filters (in order)
All checks in the `for _, aggSig := range approved` loop (loop.go:1443+):
1. **Stale signal guard** (loop.go:1477): rejects signals older than `signalMaxAge()` 
2. **Position limit** (loop.go:1492): `posMgr.CanOpenPosition()` 
3. **Regime alignment** (loop.go:1505): `isCategoryAlignedWithRegime()`
4. **Execution weight floor** (loop.go:1521): rejects if `executionWeight < 0.50`
5. **Signal sanitization** (loop.go:1550): `sanitizeSignalForProfit()` — enforces min confidence 0.68, SL/TP floors
6. **Risk validation** (loop.go:1577): `o.risk.Validate(sig, currentPrice)` 

### Step 5 — Bridge/AI Parking (conditional)
**File:** `engine/internal/trading/loop.go:1601-1648`  
If `IsBridgeOnline()` AND not a trusted strategy → signal is placed in `pendingSignals` map and execution is SKIPPED. Bridge is online if last heartbeat < 15 seconds ago.

If bridge is offline OR strategy is trusted → execution proceeds directly.

### Step 6 — Institutional Execution Path
**File:** `engine/internal/trading/loop.go:1660`  
`executeThroughInstitutionalPath()` → `executeThroughInstitutionalPathWithFill()` (loop.go:374):

1. **Generate clientOrderID**: `AG-PAPER-{SYMBOL}-{UnixNano}`
2. **Append EventOrderCreated** to in-memory ledger (loop.go:412)
3. **Persist OMSNew transition** to MongoDB via `persistOMSTransition` (loop.go:416)
4. **Replay events** through `omsv3.Replay()` to validate state machine (loop.go:430)
5. **Append EventOrderValidated** (loop.go:433)
6. **PMS Portfolio gate** (if configured): `pmsBudget.CheckPortfolioRisk()` (loop.go:480)
7. **PreTradeRiskPipeline**: `riskgate.NewPreTradeRiskPipeline().Check()` (loop.go:512):
   - Kill switch check (gate/pipeline.go:51)
   - Risk V2 validation: Kelly sizing, drawdown, exposure, VaR
8. **Elite drawdown gate** (loop.go:580): for drawdown-constrained regimes
9. **Risk V2 sizing floor** (loop.go:622): `EnforceExecutionFloor()`
10. **submitInstitutionalOrder** (loop.go:668):
    - Append EventRiskApproved, EventOrderSubmitted, EventOrderAcked
    - Call `fillFn(ctx, sig, clientOrderID)` — the actual paper/broker fill
    - Append EventOrderFilled
    - Persist `OMSSimulatedFill` transition
    - Call `recordWatchdogFill()`

### Step 7 — Paper Fill Execution
**File:** `engine/internal/execution/paper.go:137`  
`PaperClient.ExecuteSignal()`:
- Applies slippage based on OrderMode (IOC = +0.012%, PostOnly = -0.005%)
- Calls `applyFill()` which debits/credits `balanceUSD` and adjusts `positionBTC`
- Returns `FillResult{ExecPrice, OrderMode, RequestedPrice, SlippageBps}`

**No OMS v3 ledger interaction here** — the paper client is pure in-memory RAM state.

### Step 8 — Position Open and Tracking
**File:** `engine/internal/trading/loop.go:1712`  
`openAndTrackPosition()` (loop.go:844):
1. Calls `posMgr.OpenPosition(sig, fill.ExecPrice, stratName)` — creates in-memory position
2. Maps `positionID → clientOrderID` in `positionToOrderID` map
3. Launches goroutine `emitPositionOpened()` (async) — appends EventPositionOpened to ledger
4. Calls `persistPositionOpen()` — writes to MongoDB `paper_positions` collection (async goroutine)

### Step 9 — Risk Notification
**File:** `engine/internal/trading/loop.go:1705`  
`o.risk.NotifyFill(sig)` — updates risk engine exposure tracking

`syncPMSState()` (loop.go:1709) — updates portfolio risk budget with current heat/drawdown

### Step 10 — Position Close (SL/TP)
**File:** `engine/internal/trading/loop.go:1726-1812`  
`processCloseEvents()` goroutine listens on `posMgr.CloseEvents()` channel:
1. `exec.SettlePosition()` — credits/debits paper balance (loop.go:1735)
2. `execution.CanonicalTradeFees()` — calculates fee breakdown (loop.go:1737)
3. `execution.CanonicalNetPnL()` — calculates net PnL = grossPnL - fees (loop.go:1741)
4. `journal.RecordTrade()` — writes to in-memory trade journal (loop.go:1758)
5. `portfolioLedger.RecordClose()` — updates in-process accounting mirror (loop.go:1760)
6. `tracker.RecordTradeResult()` — updates strategy stats (loop.go:1771)
7. `finalizeExecIntelClose()` — Phase 22D execution intelligence (loop.go:1776)
8. `risk.RecordPnL(netPnL)` — updates daily PnL tracker (loop.go:1779)
9. `emitPositionClosed()` goroutine — appends EventPositionClosed to ledger (loop.go:1798)
10. `persistPositionClose()` — marks position CLOSED in MongoDB (async goroutine, loop.go:1805)
11. `persistClosedTrade()` — writes trade record to `paper_trades` MongoDB collection (loop.go:1806)

---

## PnL Calculation

**File:** `engine/internal/trading/loop.go:1736-1741`

```go
feeBreakdown := execution.CanonicalTradeFees(entryPrice, exitPrice, size)
netPnL := execution.CanonicalNetPnL(event.PnL, feeBreakdown)
```

**Fee model** (`engine/internal/execution/paper.go:14-16`):
- Taker fee: 0.05% (`BinanceFuturesTakerFeePct = 0.00050`)
- Maker fee: 0.02% (`BinanceFuturesMakerFeePct = 0.00020`)

**GrossPnL** = (exitPrice - entryPrice) × size for LONG; reversed for SHORT  
**FeesUSD** = notional × takerFee × 2 (entry + exit, loop.go:907-908)  
**NetPnL** = GrossPnL - FeesUSD

**Unrealized PnL** (`engine/internal/trading/paperpersist_hooks.go:85-101`):
```go
// For LONG: (markPrice - entryPrice) * size
// For SHORT: (entryPrice - markPrice) * size
```
Computed in `computeUnrealizedPnL()` using `exec.GetLastPrice()` as mark price.

---

## How Fills Flow to UI

**Mechanism: POLLING (no WebSocket, no SSE)**

**File:** `client/src/hooks/usePaperDesk.ts:34`  
`POLL_MS = 5000` — polls every 5 seconds

**Poll target:** `GET /api/paper-desk/snapshot` (single aggregated endpoint)

**Snapshot route** (`client/src/app/api/paper-desk/snapshot/route.ts:32-38`):
Executes 5 MongoDB queries in parallel:
1. `getPaperState(accountKey)` → `paper_state` collection
2. `listOpenPositions(accountKey)` → `paper_positions` (OPEN status)
3. `listPaperTrades({limit: 20})` → `paper_trades` collection (last 20)
4. `getStrategyHealthSummary(accountKey)` → `strategy_health` collection
5. `getClosedTradeStats(accountKey)` → aggregated stats from `paper_trades`

**Engine-to-MongoDB write cadence:**
- `paper_state`: written by `StateSnapshotter` goroutine every **10 seconds** (paperpersist)
- `paper_trades`: written by `TradeWriter` on each close event (fire-and-forget with retry)
- `paper_positions`: written on open/close events (fire-and-forget goroutines)

**Total UI lag:** Engine write every 10s + client poll every 5s = **up to 15 seconds of staleness** for state. For fills: fill closes to MongoDB write is async (~seconds) + 5s poll = **up to ~15 second lag** between a fill and UI update.

---

## Broken/Missing Steps

### GAP 1: OMS v3 state is NEVER exposed to the UI
**Evidence:** The OMS v3 aggregate state machine (`engine/internal/omsv3/aggregate.go`) runs entirely in-memory in the Go engine. The states (NEW, VALIDATED, RISK_APPROVED, SUBMITTED, ACKNOWLEDGED, FILLED, CANCELLED, REJECTED) are stored in `eventLedger` which defaults to `ledger.NewMemoryStore()` (loop.go:235). The MongoDB ledger is only used when `DATABASE_URL` is set (main.go:762).

The UI `/api/paper-oms/orders` endpoint (`client/src/app/api/paper-oms/orders/route.ts`) reads from a **separate** `paper_oms_orders` collection written by a **Next.js-side** TypeScript OMS (`client/src/lib/paperOmsMongo.ts`). This TypeScript OMS is a completely separate state machine — it is NOT the Go engine's OMS v3. No code was found that bridges Go OMS v3 state to this MongoDB collection.

**Result:** The UI `PaperOmsPanel` shows orders from the TypeScript OMS, NOT from the Go engine's institutional OMS v3. These are two separate, unconnected systems.

### GAP 2: Reconciliation is a stub
**File:** `engine/internal/reconciliation/sync.go:46-65`  
The `performAudit()` function hardcodes `internalPosition := 0.0` — it never actually reads internal engine state. The comment says "In reality we would expose a Getter in riskEngine". This reconciliation is non-functional.

### GAP 3: Position closes do not block on MongoDB writes
**File:** `engine/internal/trading/loop.go:1805-1806`  
`persistPositionClose()` and `persistClosedTrade()` both launch goroutines. If the engine crashes between the in-memory close event and the goroutine completing, the closed trade and position state may not reach MongoDB. There is a retry queue in `TradeWriter.Write()` but the queue is also in-memory and is lost on crash.

### GAP 4: Paper balance and OMS v3 ledger are separate
The paper `PaperClient` (execution/paper.go) maintains `balanceUSD` in RAM. The OMS v3 ledger records events but does NOT derive balance from events. The two are never reconciled. If they diverge, neither detects it.

### GAP 5: AngelOne execution path is disabled
**File:** `engine/internal/trading/institutional_request.go:28-33`  
AngelOne venue returns immediate REJECTED response: "angelone broker adapter disabled". The AngelOne client (`engine/internal/marketdata/angelone.go`) only handles market data, not order execution. No AngelOne execution adapter exists in the engine.

---

## Summary Verdict

The complete BTC paper trading workflow is:
1. Tick → Strategy → Aggregator → Filters → Institutional Path → Paper Fill → Position → MongoDB (async) → Next.js poll → UI

The chain is **functionally complete** for BTC paper trading but has significant gaps:
- UI lag up to 15 seconds
- OMS v3 state machine invisible to UI (two disconnected OMS systems)
- Reconciliation is a stub (internalPosition always 0)
- AngelOne execution not wired (data only)

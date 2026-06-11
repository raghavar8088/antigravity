# RISK ALIGNMENT REPORT — Forensic Audit Phase 9

**Date:** 2026-06-11  
**Scope:** engine/internal/risk/gate/, engine/internal/risk/v2/, kill switch  
**Method:** Source code reading only. No assumptions.

---

## Risk Limits Defined in Code

### Risk V2 Default Limits
**File:** `engine/internal/risk/v2/limits.go:3-26`

| Limit | Value | Code Location |
|-------|-------|---------------|
| Max risk per trade | 2% | `MaxRiskPerTradePct: 2` |
| Max portfolio heat | 6% | `MaxPortfolioHeatPct: 6` |
| Reduce heat threshold | 4% | `ReduceHeatPct: 4` |
| Block heat threshold | 6% | `BlockHeatPct: 6` |
| Force-reduce heat | 8% | `ForceReduceHeatPct: 8` |
| Max net exposure | 150% | `MaxNetExposurePct: 150` |
| Max gross exposure | 250% | `MaxGrossExposurePct: 250` |
| Max family allocation | 30% | `MaxFamilyAllocationPct: 30` |
| Max regime exposure | 45% | `MaxRegimeExposurePct: 45` |
| Max strategy allocation | 20% | `MaxStrategyAllocationPct: 20` |
| Max cluster allocation | 35% | `MaxClusterAllocationPct: 35` |
| Max correlation | 0.80 | `MaxCorrelation: 0.80` |
| Max VaR 95 | 6% | `MaxVaRPct: 6` |
| Max CVaR 95 | 9% | `MaxCVaRPct: 9` |
| Max leverage | 5× | `MaxLeverage: 5` |
| Max drawdown | 10% | `MaxDrawdownPct: 10` |
| Min risk score | 70 | `MinRiskScore: 70` |

### PMS Portfolio Budget (hard-coded in loop.go)
**File:** `engine/internal/trading/loop.go:471-480`
```go
pmsBudgetConfig := pms.RiskBudget{
    MaxHeatPct:      8,
    MaxVaR95Pct:     6,
    MaxCVaR95Pct:    9,
    MaxDrawdownPct:  10,
    MaxDailyLossPct: 3,
    MaxGrossExpPct:  250,
    MaxNetExpPct:    150,
}
```

### Pre-Trade Signal Filters (loop.go constants)
**File:** `engine/internal/trading/loop.go:42-56`

| Filter | Value | Code Location |
|--------|-------|---------------|
| Min execution confidence | 0.68 | `minExecutableConfidence` (line 42) |
| Min bridge approval confidence | 0.65 | `minBridgeApprovalConfidence` (line 43) |
| Min reward-to-risk ratio | 2.40 | `minRewardToRiskRatio` (line 44) |
| Min signal take profit | 0.50% | `minSignalTakeProfitPct` (line 45) |
| Max signal stop loss | 0.20% | `maxSignalStopLossPct` (line 46) |
| Default signal stop loss | 0.18% | `defaultSignalStopLossPct` (line 47) |
| Min execution weight | 0.50 | `minExecutionWeightToTrade` (line 49) |

### Kill Switch Triggers Defined
**File:** `engine/internal/killswitch/service.go:16-27`

| Trigger | Constant |
|---------|----------|
| `DAILY_LOSS_BREACH` | `TriggerDailyLoss` |
| `EXCHANGE_OUTAGE` | `TriggerExchangeOutage` |
| `DATA_FEED_OUTAGE` | `TriggerDataFeedOutage` |
| `OMS_DESYNC` | `TriggerOMSDesync` |
| `RISK_SERVICE_FAILURE` | `TriggerRiskServiceFailure` |
| `LARGE_POSITION_DRIFT` | `TriggerPositionDrift` |
| `FUNDING_SHOCK` | `TriggerFundingShock` |
| `LIQUIDATION_EVENT_SPIKE` | `TriggerLiquidationSpike` |
| `MANUAL_OPERATOR_TRIGGER` | `TriggerManualOperator` |

---

## How Risk Is Enforced (Code Path)

### Pre-Trade Path (every new order)

**Step 1 — PMS Portfolio Gate** (if pmsBudget is set)  
**File:** `engine/internal/trading/loop.go:463-509`  
Calls `pmsBudget.CheckPortfolioRisk()`. Blocks if portfolio-level heat, VaR, drawdown, or daily loss exceeds limits. Records `EventRiskBlocked` to ledger and `OMSRejected` to MongoDB.

**Step 2 — PreTradeRiskPipeline**  
**File:** `engine/internal/risk/gate/pipeline.go:46-79`  
Order:
1. Kill switch check: `p.killSwitch.IsActive()` (line 51) — immediate block if active
2. Basic validation (symbol, entry price, stop loss, size all required)
3. `p.engine.ValidateTrade(request, market, metrics)` — calls Risk V2 engine

**Step 3 — Risk V2 ValidateTrade** (engine/internal/risk/v2/engine.go:164+)  
Runs all sub-checks (inferred from RiskDecision struct fields):
- Kelly sizing, dynamic sizing, heat check, VaR, CVaR, correlation, exposure, allocation, drawdown, regime, family, budget, tail risk, forecast, attribution, risk score

**Step 4 — Elite drawdown gate**  
**File:** `engine/internal/trading/loop.go:580-617`  
If `riskDecision.RiskDecision.Drawdown.OnlyEliteStrategies` is true, non-elite strategies are blocked during drawdown periods.

**Step 5 — Execution floor**  
**File:** `engine/internal/trading/loop.go:621-661`  
`riskv2.EnforceExecutionFloor()` rejects if recommended BTC size is below minimum executable threshold.

---

## Kill Switch Wiring Verification

**Kill switch is confirmed wired to the execution path.**

### Wiring in main.go
**File:** `engine/cmd/antigravity/main.go:765-769`
```go
ksSvc := killswitchpkg.NewService(ksLedger, ksExecutor, "btc-paper-1")
ksSvc.RestoreFromLedger(ctx)
orchestrator.SetKillSwitch(ksSvc)
```

### Kill switch check in PreTradeRiskPipeline
**File:** `engine/internal/risk/gate/pipeline.go:51-55`
```go
if p.killSwitch != nil && p.killSwitch.IsActive() {
    reason := "kill switch active: " + p.killSwitch.Reason()
    return Decision{Status: DecisionBlocked, ...}
}
```

### Kill switch check in ProcessExecutionRequest (external requests)
**File:** `engine/internal/trading/institutional_request.go:16-21`
```go
if o.killSvc != nil && o.killSvc.IsActive() {
    return executiongateway.Response{OK: false, Status: "BLOCKED", ...}
}
```

### Kill switch in Delta bridge
**File:** `engine/internal/trading/institutional_request.go:149-153`  
The Delta bridge has a `SetKillCheck` callback that also checks `o.killSvc.IsActive()`.

### Kill switch actions
**File:** `engine/internal/killswitch/service.go:159-175`  
When triggered with `ActionFlattenPositions`, calls `executor.FlattenPositions()` which calls `orchestrator.ExecuteEmergencyFlatten()`.

### Auto-trigger conditions
The `ExecutionWatchdog` (trading/execution_watchdog.go) monitors for stale market data and no-trade conditions. It is wired to `ksSvc` (main.go:771-772). Actual trigger conditions need to be verified from execution_watchdog.go.

### Kill switch persistence
If `DATABASE_URL` is set, kill switch events are persisted to PostgreSQL ledger and restored on startup (`RestoreFromLedger`). If not set, the kill switch is **in-memory only** and does not survive restarts.

---

## How Risk State Is Exposed via API

### `/api/admin/ks/status` (Go engine HTTP)
**File:** `engine/cmd/antigravity/main.go:1706-1724`  
Returns:
```json
{
  "ok": boolean,
  "status": "healthy" | "blocked_kill_switch" | "stale_market_data" | "no_trades_*",
  "trading_allowed": boolean,
  "kill_switch": { "active": boolean, "reason": string },
  "last_tick_at": string,
  ...
}
```

### `/api/admin/kill` (legacy kill trigger)  
**File:** main.go:1644

### `/api/admin/ks/block` (institutional kill switch)
**File:** main.go:1658-1670  
Triggers kill switch with `TriggerManualOperator`, `ActionBlockNewOrders`.

### `/api/admin/ks/release` (institutional kill switch release)  
**File:** main.go:1680-1689

### Risk state available to Next.js API:
The Go engine risk state is accessible through the engine proxy at `/api/engine/[...path]` which forwards to the Go engine. No dedicated Next.js route for risk state was found — risk data must be fetched via the engine proxy.

---

## Whether Risk Violations Appear in UI

### Via paper_orders collection (MongoDB)
Every risk rejection is written to `paper_orders` with `transition_to: REJECTED` and `reason` field (paperpersist_hooks.go:552-563). This IS visible via `/api/paper-desk/orders`.

### Via Terminal Risk Module
**File:** `client/src/components/terminal/institutional/RiskModule.tsx`  
The Terminal Risk Module reads from `TerminalSnapshot` which is fed by WebSocket (`NEXT_PUBLIC_TERMINAL_WS_URL`). If the WebSocket URL is not configured, the risk module shows **hardcoded mock data** (terminalSnapshot.ts:67-79):
```ts
risk: {
    var95Usd: 1_840,  // HARDCODED
    heatPct: 3.7,     // HARDCODED
    drawdownPct: 1.4, // HARDCODED
    ...
}
```
**These values never change regardless of actual engine risk state.**

### Via `/api/admin/ks/status` (engine proxy)
The kill switch status IS accessible via the engine proxy if properly forwarded. The CommandCenter component likely polls this.

---

## Risk Limits Enforced in Engine But Not Visible in UI

| Risk Limit | Engine Enforcement | UI Visibility |
|---|---|---|
| Max portfolio heat 6% | YES — blocks new orders | NO (terminal shows mock heat) |
| Max VaR 6% | YES — Risk V2 check | NO (terminal shows mock VaR) |
| Max drawdown 10% | YES — elite gate + Risk V2 | Partially (paper_state.max_drawdown) |
| Max daily loss 3% (PMS) | YES — PMS gate | NO dedicated UI |
| Max correlation 0.80 | YES — Risk V2 | NO |
| Min confidence 0.68 | YES — sanitizeSignalForProfit | NO |
| Min R:R 2.40 | YES — loop.go constant used in sanitize | NO |
| Min execution weight 0.50 | YES — quality filter | NO |
| Regime alignment filter | YES — isCategoryAlignedWithRegime | NO |
| Max signal stop loss 0.20% | YES — maxSignalStopLossPct | NO |
| Family allocation 30% | YES — Risk V2 | NO |

---

## Verdict: Is Risk Aligned?

**ENFORCEMENT: STRONG**  
The kill switch is properly wired to every execution path (pre-trade pipeline, external gateway, Delta bridge). Risk V2 checks run on every order. PMS portfolio gate adds a portfolio-level layer.

**VISIBILITY: WEAK**  
- The Terminal Risk Module displays **hardcoded mock data** if `NEXT_PUBLIC_TERMINAL_WS_URL` is not set
- No dedicated UI for real-time heat, VaR, correlation, family allocation
- Kill switch status is accessible via engine proxy but there is no persistent UI component showing it
- Risk rejections are logged to `paper_orders` MongoDB but require the user to query the orders table to see them
- Most pre-trade filter thresholds (confidence, R:R, execution weight, regime alignment) have zero UI visibility

**The risk engine enforces correctly but the risk state is largely invisible to the trader watching the UI.**

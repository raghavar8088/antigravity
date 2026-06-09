# OMS / PMS Report

**Audit date:** 2026-06-09

---

## OMS State Transitions

### Event-Sourced OMS v3

**Files:** `engine/internal/omsv3/aggregate.go`, `ledger/event.go`, `ledger/order_projection.go`

| From | Event | To | Live Path? |
|------|-------|-----|------------|
| — | `EventOrderCreated` | NEW | ✅ |
| NEW | `EventOrderValidated` | VALIDATED | ✅ |
| VALIDATED | `EventRiskApproved` | RISK_APPROVED | ✅ |
| RISK_APPROVED | `EventOrderSubmitted` | SUBMITTED | ✅ |
| SUBMITTED | `EventOrderAcked` | ACKNOWLEDGED | ✅ (synthetic) |
| ACKNOWLEDGED | `EventOrderPartial` | PARTIAL | ❌ live |
| ACKNOWLEDGED | `EventOrderFilled` | FILLED | ✅ |
| * | `EventOrderCancelled` | CANCELLED | ❌ live (reject only) |
| * | `EventOrderRejected` | REJECTED | ✅ |
| — | `EventRiskBlocked` | BLOCKED | ✅ |

### Invalid Transition Guards

`omsv3/aggregate_invariants.go` — `ValidateTransition` rejects illegal state jumps.  
**Test proof:** `order_lifecycle_test.go`, `certification/flow_certification_test.go`.

**OMS transition logic verdict:** **PASS**  
**Live path completeness verdict:** **FAIL** (partial/cancel/expiry not emitted)

---

## OMS Persistence

| Layer | File | Storage |
|-------|------|---------|
| Event ledger | `ledger/store.go` | In-memory + optional Mongo |
| Mongo OMS transitions | `paperpersist_hooks.go` | `persistOMSTransition` |
| Client paper OMS | `paperOms.ts` + `paperOmsMongo.ts` | Mongo |

### Mongo OMS States (paperpersist)

`loop.go:388–396` — transitions: `OMSNew` → `OMSRiskChecked` → `OMSAccepted` → `OMSSimulatedFill`

**Verdict:** **PASS** for paper path audit trail.

---

## Position OMS

| Event | Emitter | File |
|-------|---------|------|
| `EventPositionOpened` | `emitPositionOpened` | `loop.go:836–869` |
| `EventPositionClosed` | `emitPositionClosed` | `loop.go:874–912` |
| Position aggregate replay | `ReplayPosition` | `omsv3/position_aggregate.go:153` |

**Verdict:** **PASS**

---

## PMS — Portfolio Management System

### Risk Budget (PMS Layer)

**File:** `engine/internal/trading/loop.go:435–452`

```go
pmsBudgetConfig := pms.RiskBudget{
    MaxHeatPct: 8, MaxVaR95Pct: 6, MaxCVaR95Pct: 9,
    MaxDrawdownPct: 10, MaxDailyLossPct: 3,
    MaxGrossExpPct: 250, MaxNetExpPct: 150,
}
```

| PMS Function | Verdict | Evidence |
|--------------|---------|----------|
| Portfolio accounting | **PASS** | `portfolioLedger.RecordClose` (`loop.go:1730`) |
| Exposure tracking | **PASS** | PMS heat/VaR checks |
| Capital tracking | **PASS** | `exec.GetEquityUSD()`, `RestoreBalance` |
| Drawdown tracking | **PASS** | `riskv3` `DrawdownPct`, `PeakDrawdownPct` |
| Risk budgets | **PASS** | `CheckPortfolioRisk` blocks violations |
| Emergency bypass | **PASS** | `EmergencyFlatten` skips PMS (`loop.go:409`) |

### Risk V3 Aggregate

**File:** `engine/internal/riskv3/risk_aggregate.go`

Tracks: `DrawdownPct`, `PeakDrawdownPct`, `HeatPct`, `VaR95Pct`, `KellyFractionPct`  
Events: `EventMaxDrawdownBreached`

**Test:** `TestCheckOrder_BlockedByDrawdown`, `TestCheckOrder_KellyCapApplied`

**Verdict:** **PASS** for PMS gate logic.

---

## ExecIntel Lifecycle Tracker

**File:** `engine/internal/execintel/tracker.go`

```
SignalGenerated → SignalApproved → OrderSubmitted → OrderAcknowledged
  → OrderFilled → PositionOpened → PositionClosed
```

Wired at `loop.go:1421–1433`, `1628–1643`, `1682`, `finalizeExecIntelClose`.

**Verdict:** **PASS** for observability chain.

---

## Client PMS (Paper Desk)

| Function | File | Verdict |
|----------|------|---------|
| Balance tracking | `useBTCFuturesScalperEngine.ts` | **PASS** |
| Drawdown lock | same, `MAX_DRAWDOWN_LOCK_PCT` | **PASS** |
| Max open positions | `futuresDeskPolicy.ts` | **PASS** |
| Portfolio heat | Not implemented | **FAIL** |
| VaR/CVaR | Not implemented | **FAIL** |

**Client PMS is simpler than Go PMS — no portfolio heat/VaR gates.**

---

## Phase 8 Conclusion

| Component | Verdict |
|-----------|---------|
| OMS state machine correctness | **PASS** (tested) |
| OMS live path completeness | **FAIL** |
| OMS persistence | **PASS** (paper) |
| PMS risk gates (Go) | **PASS** |
| PMS risk gates (Client) | **FAIL** (partial) |
| Portfolio accounting | **PASS** (Go); **PASS** (client paper) |

**Overall Phase 8:** **PASS** for OMS/PMS design and paper enforcement; **FAIL** for live broker OMS parity.

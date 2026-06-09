# RECONCILIATION_CERTIFICATION.md
## Phase 5 — Reconciliation Audit

**Audit Date:** 2026-06-09  
**Verdict: FAIL** (production path)

---

## Executive Summary

Production reconciliation compares **internal state to itself**. Real broker reconciliation (`reconciliationv2`) is fully implemented but **never started** in `engine/cmd/antigravity/main.go`. The boot comment claiming kill-switch auto-trigger on CRITICAL drift is **false**.

---

## Production Wiring

```go
// main.go:880–890
reconProvider := reconciliation.NewPaperSnapshotProvider(posMgr, "btc-paper-1")
reconSvc := reconciliation.NewService(reconProvider, reconLedger, 10*time.Second)
go reconSvc.Run(ctx)
```

Comment at L883: *"On CRITICAL drift the kill switch is auto-triggered (OMS_DESYNC)"* — **NOT IMPLEMENTED**.

---

## Does Reconciliation Use REAL Broker Data?

### Production: **FAIL**

`engine/internal/reconciliation/paper_provider.go:12–13`:
```go
// For paper trading, "exchange" state == position manager state.
```

`Snapshot()` (L28–58):
- `omsPositions` and `exchPositions` built from **same** `posMgr.GetOpenPositions()` loop
- `OMSOrders` and `ExchangeOrders` — **never populated** (empty slices)
- `OMSBalance` and `ExchangeBalance` — **empty** (L55–57: "skip here")

Effect: `PositionDriftDetector`, `OrderMismatchDetector`, `BalanceDriftDetector` receive identical or empty inputs → **zero alerts possible**.

### reconciliationv2 Library: **PASS** (code exists, unwired)

| Adapter | File | Real API |
|---------|------|----------|
| Binance | `reconciliationv2/binance_reconciliation.go` | `/fapi/v2/account`, positions, orders |
| Delta | `reconciliationv2/delta_reconciliation.go` | `/v2/wallet/balances`, `/v2/positions` |
| Paper mirror | `reconciliationv2/paper_reconciliation.go` | Returns OMS as exchange (L8–10) |

`ReconciliationEngine.runPositions()` (engine.go:152–158) fetches live exchange state.

**Grep `reconciliationv2` in `main.go`: zero matches.**

---

## Reconciliation Flow (Production — Inert)

```
PaperSnapshotProvider.Snapshot()
  ├─ posMgr.GetOpenPositions() → omsPositions
  └─ posMgr.GetOpenPositions() → exchPositions (IDENTICAL)
       ↓
Service.Check() (service.go:60–75)
  ├─ OrderMismatchDetector.Detect([], []) → no alerts
  ├─ PositionDriftDetector.Detect(same, same) → no alerts
  └─ BalanceDriftDetector.Detect(empty, empty) → no alerts
       ↓
appendAlert() → ledger.EventReconciliationAlert (alert only, no action)
```

---

## Reconciliation Flow (v2 — Not Running)

```
ReconciliationAdapter.GetPositions() → REAL exchange REST
ReconciliationAdapter.GetOpenOrders() → REAL exchange REST
ReconciliationAdapter.GetFills() → REAL exchange REST
       ↓
v2 detectors (detectors.go)
  ├─ Balance drift (L58–120)
  ├─ Position ghost/missing (L147+)
  ├─ Order ghost (L327+)
  └─ Fill missing (L479+)
       ↓
RepairEngine.Repair() (repair.go:67–114)
  ├─ repairProjectionRebuild → internal ledger only
  └─ escalate → EventManualInterventionRequired
```

**v2 repair rebuilds projections from ledger — does NOT cancel ghost exchange orders or sync broker positions.**

---

## Mismatch Detection

| Detector | File:Line | Alert Types | Production Active? |
|----------|-----------|-------------|-------------------|
| OrderMismatch | `detectors.go:91–137` | MISSING_FILL, GHOST_ORDER, DUPLICATE | **NO** (empty orders) |
| PositionDrift | `detectors.go:144–163` | POSITION_DRIFT, STALE_POSITION | **NO** (identical inputs) |
| BalanceDrift | `detectors.go:170–177` | BALANCE_DRIFT | **NO** (empty balances) |
| v2 Fill missing | `reconciliationv2/detectors.go:479+` | MISSING_FILL | **NO** (unwired) |

Tests prove logic works with divergent data (`detectors_test.go:11–35`) — production never supplies divergence.

---

## Auto-Repair

| System | Behavior | Wired? |
|--------|----------|--------|
| v1 `Service.Resolve` | Manual resolve event (`service.go:101–136`) | **NO** — never called from main |
| v1 `appendAlert` | Log to ledger only (`service.go:78–95`) | Yes — alerts only |
| v2 `RepairEngine` | Rebuild projections + correction events | **NO** — unwired |
| Legacy `sync.go` | `internalPosition := 0.0` hardcoded (L52) | **NO** — dead code |

---

## Kill Switch Integration

Triggers defined (`killswitch/service.go:14–24`):
- `TriggerOMSDesync`
- `TriggerPositionDrift`

Pre-trade gate: **PASS** — `risk/gate/pipeline.go:51–54` blocks new orders when active.

Reconciliation → kill switch: **FAIL** — no code calls `ksSvc.Trigger(TriggerOMSDesync)` from any reconciliation path.

Manual trigger only: `main.go:1604` — `/api/admin/ks/trigger`.

---

## ExchangeOrderID in Reconciliation

v2 Delta adapter maps real IDs:
- `delta_reconciliation.go:226` — `ExchangeOrderID: strconv.FormatInt(o.ID, 10)`
- `delta_reconciliation.go:281` — fills mapped by `f.OrderID`

v1 order detector indexes by `ExchangeOrderID` (`detectors.go:104–111`) — but production provider supplies no orders.

---

## Certification Answers

| Requirement | Verdict | Evidence |
|-------------|---------|----------|
| Real broker data comparison | **FAIL** | `paper_provider.go:44–48` |
| Mismatch detection (prod) | **FAIL** | Identical snapshot inputs |
| Mismatch detection (library) | **PASS** | v2 detectors with tests |
| Auto-repair | **FAIL** | Alert-only in prod |
| Kill-switch on drift | **FAIL** | False comment main.go:883 |
| Orphan order detection | **FAIL** | Empty order snapshots |
| ExchangeOrderID correlation | **FAIL** | Synthetic in OMS; v2 unwired |

**RECONCILIATION CERTIFICATION: FAIL — Not safe for live capital without wiring reconciliationv2 and kill-switch bridge.**

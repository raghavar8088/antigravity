# Reconciliation Report

**Audit date:** 2026-06-09

---

## Symbol Search Results

| Symbol | Found | Location |
|--------|-------|----------|
| `Reconcile` / `Reconciliation` | ✅ | `engine/internal/reconciliation/` |
| `SyncPositions` | ❌ | Zero matches repo-wide |
| `RecoverPositions` | ❌ | `RestorePositions` in `positions/manager.go` (boot only) |
| `BrokerSnapshot` | ❌ | `Snapshot`, `BalanceSnapshot` types exist |
| `PortfolioSnapshot` | Partial | `paperpersist` equity snapshots |
| `OMSReplay` | ❌ | `ledger.Replay`, `omsv3.Replay` (different names) |

---

## Wired Production Path

**Boot wiring** (`engine/cmd/antigravity/main.go` ~880–890):

```go
reconProvider := reconciliation.NewPaperSnapshotProvider(posMgr, "btc-paper-1")
reconSvc := reconciliation.NewService(reconProvider, reconLedger, 10*time.Second)
go safeGo("Reconciliation", func() { reconSvc.Run(ctx) })
```

### PaperSnapshotProvider — Critical Defect

`engine/internal/reconciliation/paper_provider.go:28–58`:

```go
// For paper trading, "exchange" state == position manager state.
for _, pos := range openPositions {
    omsPositions = append(omsPositions, OMSPosition{...})
    exchPositions = append(exchPositions, ExchangePosition{...}) // SAME DATA
}
```

**Both OMS and exchange views are populated from the same `posMgr.GetOpenPositions()` call.** Position drift detection will **always pass** regardless of actual broker divergence.

---

## Reconciliation Service Capabilities

| File | Function | Behavior |
|------|----------|----------|
| `service.go` | `Run()` | 10s polling loop |
| `service.go` | `Check()` | Order/position/balance drift detectors |
| `service.go` | `Resolve()` | Manual audit event only — **no auto-fix** |
| `detectors.go` | `OrderMismatchDetector` | Ghost orders, missing fills, duplicates, stale |
| `detectors.go` | `PositionDriftDetector` | OMS vs exchange qty |
| `detectors.go` | `BalanceDriftDetector` | Equity/cash drift |

### Verification Matrix

| Check | Verdict | Evidence |
|-------|---------|----------|
| A. Startup reconciliation | **FAIL** | No startup broker snapshot compare. Boot restores from SQLite (`persistence/store.go`) without broker query. |
| B. Periodic reconciliation | **PARTIAL** | 10s loop runs but compares OMS to itself (`paper_provider.go`) |
| C. Forced reconciliation | **FAIL** | No API to force broker snapshot |
| D. Drift detection | **FAIL** (live) | Detectors work in unit tests; production provider prevents drift signals |
| E. Drift repair | **FAIL** | `Resolve()` records audit only (`service.go`) |
| F. Missing fill recovery | **FAIL** | Not implemented in v1 |
| G. Duplicate fill recovery | **FAIL** | Detected in tests; no repair loop |

---

## Legacy Binance Reconciler (Stub)

`engine/internal/reconciliation/sync.go`:

- Polls Binance every 60s
- `internalPosition := 0.0` **hardcoded**
- Logs mismatch only — no halt, no repair, no ledger events
- **Not wired in `main.go`**

**Verdict:** **FAIL** — dead code stub.

---

## reconciliationv2 (Exists, Not Wired)

`engine/internal/reconciliationv2/`:

| Component | File | Capability |
|-----------|------|------------|
| Delta recon | `delta_reconciliation.go` | Orders, fills, partial qty |
| Binance recon | `binance_reconciliation.go` | Futures account/orders/fills |
| Drift detectors | `detectors.go` | Ghost orders, wrong status, partial qty drift |
| Repair engine | `repair_engine.go` | `RepairTypeReplay`, `RepairTypeStateSync` |

**Grep `reconciliationv2` in `main.go`:** **No matches.**

Full repair stack exists but is **not bootstrapped**.

---

## Client-Side Consistency Checks

| File | Purpose | Auto-repair |
|------|---------|-------------|
| `portfolioConsistencyValidation.ts` | Mongo trades vs `paper_state` PnL | **No** — detect only |
| `deskSelfHealing.ts` | Recommendations (`REPAIR_STATE`, `RESTART_WORKER`) | **No** — pure logic, no I/O |
| `futuresReplayEngine.ts` | Deterministic backtest validation | **No** — not production recovery |

---

## Kill Switch Link

`main.go` comment references CRITICAL drift → `OMS_DESYNC` kill switch trigger.

**Code search:** `reconciliation.Service.Check()` appends `EventReconciliationAlert` only. **No killswitch call found** from reconciliation service.

**Verdict:** Documented but **not wired**.

---

## Ledger Replay (Adjacent)

`engine/internal/ledger/replay.go`:
- `ReplayEverything` — rebuilds projections from event store
- `ReplayFromSnapshot` — snapshot + delta replay

Used in certification tests. **Not invoked on engine restart** for OMS rebuild from events in production boot path (SQLite point-in-time restore used instead).

---

## Phase 4 Conclusion

| Requirement | Verdict |
|-------------|---------|
| Broker position vs OMS position compare | **FAIL** |
| Broker position vs portfolio compare | **FAIL** |
| Risk position sync | **FAIL** |
| Drift repair | **FAIL** |
| Missing/duplicate fill recovery | **FAIL** |

**Overall Phase 4:** **FAIL** — reconciliation infrastructure exists in code but production wiring prevents real drift detection and provides no repair.

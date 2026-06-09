# FAILURE_MATRIX.md
## Phase 7 — Failure Injection Audit

**Audit Date:** 2026-06-09  
**Method:** Source-code behavior analysis (not runtime injection)

---

## Failure Matrix

| Failure | Expected (Institutional) | Actual Behavior | Recovery Path | Verdict |
|---------|--------------------------|-----------------|---------------|---------|
| **Broker timeout** | Reject order, no position, alert | `fillFn` error → `EventOrderRejected` (loop.go:677–705) | Manual retry; no auto-retry | **PARTIAL** |
| **Broker rejection** | EventOrderRejected, OMS cancelled | Implemented (loop.go:677–705) | None automated | **PASS** |
| **Broker disconnect** | Block new orders, reconcile | Delta client HTTP error → reject; no disconnect handler | None | **FAIL** |
| **Network outage** | Kill switch / pause | No automatic trigger; strategies keep generating signals | Manual kill switch | **FAIL** |
| **Database outage** | Degrade gracefully, halt trading | Mongo: cold start (`recovery.go:96–101`); Postgres: kill switch ledger nil | Continue without persistence | **PARTIAL** |
| **Engine restart** | Replay ledger, restore positions | SQLite+Mongo snapshot restore; ledger lost | Boot sequence (see RECOVERY) | **PARTIAL** |
| **Duplicate fills** | Idempotent position update | Idempotency on order **events** only (`idempotencyKeyForOrder` loop.go:729); no fill dedup | None | **FAIL** |
| **Partial fills** | EventOrderPartial, incremental position | Full fill assumed (loop.go:708) | None | **FAIL** |
| **Late fills** | Poll and reconcile | No poller; position already opened at full size | reconciliationv2 unwired | **FAIL** |
| **Out-of-order fills** | Sequence enforcement | Single synchronous fillFn; no async fills | N/A for current model | **PASS** (by design) |
| **OMS desync** | Kill switch auto-trigger | Alert only (v1); no kill switch call | Manual operator | **FAIL** |
| **Position drift** | Halt + reconcile | Mirror provider — drift undetectable | None | **FAIL** |
| **Kill switch during order** | Block submission | `ProcessExecutionRequest` checks (institutional_request.go:16–20); `PreTradeRiskPipeline` (pipeline.go:51–54) | Block returns | **PASS** |
| **Kill switch on bridge submit** | Block Delta order | `SetKillCheck` wired (institutional_request.go:149–154) but **never called** in `SubmitOrder` (live_bridge.go:131–141) | **BYPASS** | **FAIL** |
| **Exchange ghost order** | Detect + cancel | v2 detector exists; unwired | Manual | **FAIL** |
| **SL gap (price jumps past SL)** | Close at SL or worse | Tick-level check (manager.go:192); may fill at worse price on gap | Software close at tick price | **PARTIAL** |
| **Emergency flatten failure** | Retry / alert | Log + return error (killswitch_executor.go:69–71) | Manual | **PARTIAL** |
| **Dual recovery source conflict** | Single authority | SQLite vs Mongo conditional merge | Inconsistency possible | **PARTIAL** |

---

## Detailed Failure Paths

### Broker Timeout

```
submitInstitutionalOrder → fillFn(ctx, sig, clientOrderID)
  → delta.Client.PlaceOrder → HTTP timeout
  → error returned
  → EventOrderRejected (loop.go:682)
  → OMS transition OMSCancelled (loop.go:694–704)
  → NO position opened
```

No retry. No timeout-specific handling beyond generic error.

### Engine Restart with Open Position

```
Crash (in-memory ledger lost)
  → Boot: SQLite LoadState + Mongo Recover
  → posMgr.RestorePositions
  → Trading resumes
  → LOST: in-flight orders, OMS event history, kill-switch active state, Delta LiveTrade records
```

### Duplicate Fill (Theoretical)

If broker sends duplicate fill confirmation:
- No fill-level idempotency key
- Second fill would require second `fillFn` call — not possible in current synchronous model
- **Risk emerges if async fill listener added without dedup**

### Partial Fill at Exchange

```
Delta PlaceOrder returns success with partial fill
  → OMS records FillQuantity = TargetSize (loop.go:708)
  → Position opened at full size
  → ACTUAL broker position < OMS position
  → Reconciliation cannot detect (mirror provider)
```

---

## Kill Switch Failure Modes

| Action | Implementation | Gap |
|--------|----------------|-----|
| Block new orders | `killSvc.IsActive()` in pipeline + gateway | **PASS** |
| Cancel open orders | `posMgr.CloseAllPositions` — no OMS events | **PARTIAL** |
| Flatten via institutional | `ExecuteEmergencyFlatten` — skips PMS/RiskV2 | **PARTIAL** (intentional) |
| Bridge submit block | `killCheck` defined, never invoked | **FAIL** |
| Auto-trigger on desync | Not implemented | **FAIL** |
| State survive restart | Events persist; `active` flag does not | **FAIL** |

---

## Failure Matrix Summary

| Category | PASS | PARTIAL | FAIL |
|----------|------|---------|------|
| Order-level failures | 2 | 2 | 0 |
| Infrastructure failures | 0 | 3 | 3 |
| Fill integrity failures | 1 | 0 | 4 |
| Reconciliation failures | 0 | 0 | 4 |
| Kill-switch failures | 1 | 2 | 3 |

**Overall: Material failure handling gaps — system cannot self-heal from broker/exchange divergence.**

# 08 — Reconciliation Validation Framework

**Service:** `engine/internal/reconciliation`  
**Interval:** 10 seconds  
**Wired:** `main.go` line ~873 — `reconSvc.Run(ctx)`

---

## Architecture

```
Every 10s:
  PaperSnapshotProvider.Snapshot()
    → OMS positions (from posMgr)
    → Expected OMS state
  Detectors:
    → OrderMismatchDetector (stale > 30s)
    → PositionDriftDetector (tolerance 1e-8 BTC)
    → BalanceDriftDetector (tolerance $1)
  On alert → append to ledger → CRITICAL → kill switch
```

---

## Test Scenarios

| ID | Scenario | Injection Method | Expected | Severity |
|----|----------|------------------|----------|----------|
| RECON-01 | Service starts at boot | Check logs on startup | `[RECONCILIATION] ✅ Continuous reconciliation started` | CRITICAL |
| RECON-02 | Survives restart | Restart engine, check logs | Service restarts within 10s | CRITICAL |
| RECON-03 | Position drift detected | Manually alter posMgr qty +0.001 BTC | POSITION_DRIFT alert in ledger | CRITICAL |
| RECON-04 | Trading paused on drift | Critical drift injected | Kill switch activated | CRITICAL |
| RECON-05 | Ghost order detected | Stale order > 30s in OMS | GHOST_ORDER alert | HIGH |
| RECON-06 | Balance drift | Alter balance by $5 | BALANCE_DRIFT alert | HIGH |
| RECON-07 | DB reconnect | Kill Postgres connection briefly | Service resumes after reconnect | HIGH |
| RECON-08 | Metrics emitted | Prometheus scrape | `trading_reconciliation_*` metrics | MEDIUM |

---

## Validation Script

```bash
export ENGINE_URL="https://engine-alb.example.com"
export ENGINE_ADMIN_SECRET="..."
bash scripts/production-readiness/validate-reconciliation.sh
```

### Manual Drift Injection (Staging Only)

```go
// In staging: temporarily modify PaperSnapshotProvider to return inflated position
// OR use admin endpoint to create intentional desync (Phase 4 tooling)
```

---

## Go Unit Tests

```bash
cd engine
go test ./internal/reconciliation/... -v -count=1
go test ./internal/reconciliation/... -run TestPositionDrift -v
```

Existing tests in `engine/internal/reconciliation/` validate detector logic.

---

## Alert → Kill Switch Pipeline

```
reconciliation.Alert (CRITICAL)
  → ledger.Append(RECONCILIATION_ALERT)
  → killswitch.Trigger(OMS_DESYNC)
  → orchestrator blocks new orders
  → KillSwitchExecutor flattens (if configured)
```

**Verify:**
```bash
# After drift injection
curl "$ENGINE_URL/api/admin/ks/status" -H "X-Engine-Admin-Secret: $SECRET"
# Expected: {"active":true,"reason":"OMS_DESYNC"...}
```

---

## Prometheus Metrics

| Metric | Alert Threshold |
|--------|-----------------|
| `trading_reconciliation_alerts_total` | > 0 in 5 min → page |
| `trading_reconciliation_drift_btc` | > 1e-8 → critical |
| `trading_reconciliation_last_check_timestamp` | Stale > 30s → service dead |

---

## Recovery Procedure

1. Kill switch auto-activated on CRITICAL drift
2. Operator investigates ledger: `SELECT * FROM ledger_events WHERE event_type LIKE 'RECONCILIATION%' ORDER BY global_sequence DESC LIMIT 20`
3. Identify root cause (ghost fill, missed ACK, manual state edit)
4. Repair state via `paper-state/repair` (authenticated) or ledger replay
5. Release kill switch: `POST /api/admin/ks/release`
6. Verify reconciliation clean for 5 minutes before resuming live

---

## Sign-Off Criteria

| Test | Pass |
|------|------|
| RECON-01 through RECON-04 | All PASS |
| Go unit tests | All PASS |
| Alert → kill switch < 15s | PASS |
| Survives engine restart | PASS |

**Reconciliation Readiness:** 85/100 (wired, production validation pending)

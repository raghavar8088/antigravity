# 09 — Event Replay Validation Framework

**Code:** `engine/internal/omsv3/replay_engine.go`, `engine/internal/ledger/replay.go`  
**Schema:** `infrastructure/database/event_store.sql`  
**Boot wiring:** ❌ NOT YET WIRED — required pre-production

---

## Replay Capabilities

| Function | Restores |
|----------|----------|
| `omsv3.ReplayAll()` | Full account state |
| `omsv3.ReplayOpenOrders()` | Open OMS orders |
| `omsv3.ReplayOpenPositions()` | Open positions |
| `ledger.ReplayEverything()` | All event streams |
| `omsv3.Replay(events)` | Single order aggregate |

---

## Scenario Test Matrix

### Scenario 1: Exchange Outage During Fill

**Simulation:** Disconnect exchange adapter mid-fill; reconnect after 30s.

| Verification | Pass Criteria |
|--------------|---------------|
| No order duplication | Single fill per `client_order_id` |
| No missing fills | Ledger has ORDER_FILLED for every ACK |
| Idempotency | Duplicate fill message → `ErrDuplicateEvent` |

```bash
go test ./internal/omsv3/... -run TestExchangeOutageDuringFill -v
```

### Scenario 2: OMS Restart During Execution

**Simulation:** Kill engine process while order in SUBMITTED state.

| Verification | Pass Criteria |
|--------------|---------------|
| Boot replay | `ReplayAll()` restores order to SUBMITTED |
| Resume execution | Order completes or cancels correctly |
| No state loss | Position matches pre-crash |

```bash
go test ./internal/omsv3/... -run TestOMSRestartRecovery -v
```

### Scenario 3: Process Crash Between ACK and FILL

**Simulation:** Persist ORDER_ACKED; crash before ORDER_FILLED.

| Verification | Pass Criteria |
|--------------|---------------|
| Recovery logic | On restart, poll exchange for fill status |
| Ledger gap | ACK without FILL triggers reconciliation alert |
| No phantom position | Position not opened without confirmed fill |

### Scenario 4: Duplicate Exchange Messages

**Simulation:** Send identical fill webhook/message twice.

| Verification | Pass Criteria |
|--------------|---------------|
| Idempotency key | `{aggregate_id}:{event_type}:{client_order_id}` |
| Second message | Ignored safely |
| Position qty | Unchanged after duplicate |

```go
// PostgresStore enforces via partial unique index
// Duplicate → ErrDuplicateEvent
```

### Scenario 5: Network Partition

**Simulation:** Block engine → exchange connectivity for 60s.

| Verification | Pass Criteria |
|--------------|---------------|
| Trading pause | Kill switch or risk gate blocks new orders |
| Reconciliation recovery | Drift detected and alerted post-reconnect |
| No split-brain orders | Leader election prevents dual submission (ECS) |

---

## Automated Validation

```bash
bash scripts/production-readiness/validate-event-replay.sh
# Runs:
#   go test ./internal/ledger/... -v
#   go test ./internal/omsv3/... -run 'Replay|Idempotency' -v
#   go test ./internal/certification/... -run KillSwitch -v
```

---

## Boot Wiring Requirement

Add to `main.go` after PostgresStore init:

```go
if durableLedger != nil {
    replayResult, replayErr := omsv3.ReplayAll(ctx, durableLedger, "btc-paper-1")
    if replayErr != nil {
        log.Fatalf("[REPLAY] FATAL: cannot restore OMS state: %v", replayErr)
    }
    log.Printf("[REPLAY] ✅ Restored %d orders, %d positions from ledger",
        len(replayResult.Orders), len(replayResult.Positions))
    orchestrator.RestoreFromReplay(replayResult) // wire to orchestrator
}
```

**Go-live gate:** FAIL if boot replay not wired in production builds.

---

## PITR + Replay Combined Recovery

```
1. Aurora PITR restore to T-5min
2. Engine boot → ReplayAll from restored ledger
3. Reconcile with MongoDB read models
4. Compare v_open_positions view vs posMgr
5. Operator sign-off before resuming trading
```

---

## Pass/Fail Gate

| Result | Action |
|--------|--------|
| All 5 scenarios PASS | Release allowed |
| Scenario 1 or 4 FAIL | BLOCK — capital duplication risk |
| Scenario 2 or 3 FAIL | BLOCK — state loss risk |
| Boot replay not wired | BLOCK |

**Event Replay Readiness:** 65/100 (code complete, boot wiring + prod tests pending)

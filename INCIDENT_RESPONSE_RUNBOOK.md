# Incident Response Runbook — Phase 15H

## Incident Severity Classification

| Severity | Description | Response Time | Examples |
|----------|-------------|---------------|---------|
| **CRITICAL** | Capital at risk or trading halted | Immediate (<5 min) | Kill switch, ghost positions, DB down |
| **HIGH** | Degraded execution, SLO breach | 15 minutes | Latency spike, high reject rate |
| **MEDIUM** | Operational degradation | 1 hour | Strategy disabled, snapshot failure |
| **LOW** | Informational | Next business day | Routine events |

---

## Incident Lifecycle

```
TRIGGERED → ACKNOWLEDGED → INVESTIGATING → MITIGATING → RESOLVED
```

| Status | Action Required |
|--------|----------------|
| TRIGGERED | PagerDuty fires, on-call engineer must acknowledge within 5 min (CRITICAL) |
| ACKNOWLEDGED | Engineer confirms they own the incident |
| INVESTIGATING | Root cause analysis underway |
| MITIGATING | Fix in progress, capital impact contained |
| RESOLVED | Service restored, root cause identified |

### API Usage (Incident Manager)

```go
// Trigger
im := observability.NewIncidentManager("/var/log/trading/incidents.jsonl")
inc := im.Trigger(ctx, "Kill Switch Activated", "Daily loss limit exceeded",
    observability.SeverityCritical, observability.CategoryRisk, "killswitch")

// Lifecycle
im.Acknowledge(ctx, inc.ID, "raghavar8088@gmail.com", "Taking ownership")
im.Investigate(ctx, inc.ID, "raghavar8088@gmail.com", "Checking risk engine logs")
im.Mitigate(ctx, inc.ID, "raghavar8088@gmail.com", "Reducing position sizes")
_, _ = im.Resolve(ctx, inc.ID, "raghavar8088@gmail.com", "Daily loss limit was exceeded by a correlated BTC drawdown")
```

---

## CRITICAL Incident Runbooks

---

### INC-001: Kill Switch Triggered

**Alert:** `KillSwitchTriggered`
**Impact:** All trading halted. No orders can be submitted.

**Immediate Actions (0–5 minutes):**
1. Acknowledge in PagerDuty
2. Check kill switch reason: `curl http://engine:8080/api/admin/status | jq .kill_switch`
3. Review recent PnL: check `trading_portfolio_daily_pnl_usd` in Grafana → Dashboard 01
4. If daily loss limit exceeded: do NOT reset until losses are reviewed
5. If false positive (system error): check engine logs: `{event_type="KILL_SWITCH_ACTIVATED"}`

**Resolution:**
- If legitimate loss limit: consult risk policy, reduce position sizes, reset via `POST /api/admin/reset`
- If false positive: identify the triggering condition, fix code, deploy hotfix, reset

---

### INC-002: Exchange Disconnected

**Alert:** `ExchangeDisconnected`
**Impact:** No market data. Strategy evaluations may use stale prices.

**Immediate Actions:**
1. Check exchange status page (Delta/Binance/AngelOne)
2. Check `trading_marketdata_reconnects_total` — is the engine retrying?
3. Check fallback chain: Delta → Binance → synthetic spot
4. If reconnect loop failing: check credentials in env (`DELTA_API_KEY`, `BINANCE_API_KEY`)
5. Monitor `trading_marketdata_exchange_connected{exchange=X}` for reconnection

**Resolution:**
- Engine auto-reconnects with exponential backoff
- If credentials expired: rotate via Vault, restart engine
- If exchange downtime: wait for exchange, log incident

---

### INC-003: Ghost Position Detected

**Alert:** `GhostPositionDetected`
**Impact:** Exchange holds a position not tracked by OMS. PnL incorrect. Risk calculations wrong.

**Immediate Actions:**
1. Identify ghost position: `{event_type="GHOST_POSITION_DETECTED"}` in Loki
2. Check exchange position via: `GET /api/delta-live/positions`
3. Check OMS positions via: `GET /api/paper-state`
4. **DO NOT close the position immediately** — determine if it was a legitimate trade

**Root Cause Investigation:**
- Was an order filled but the fill event was dropped before reaching the ledger?
- Was the ledger write interrupted during a crash?
- Check `trading_ledger_write_errors_total` — any errors at time of position opening?

**Resolution:**
- If ledger error: replay events from last snapshot to recover OMS state
- If irrecoverable: manually reconcile via API, create audit entry, file postmortem
- Check `engine/internal/reconciliationv2/` for auto-repair results

---

### INC-004: Missing Fill Detected

**Alert:** `MissingFillDetected`
**Impact:** Fill exists on exchange but not in OMS. PnL understated.

**Immediate Actions:**
1. Identify missing fill: Loki query `{event_type="MISSING_FILL_DETECTED"}`
2. Cross-reference exchange fills: `GET /api/delta-live/trades`
3. Check ledger events for the order: `SELECT * FROM events WHERE aggregate_id = '<order_id>'`

**Resolution:**
- Reconciliation auto-repair should handle via `RepairTypeManualFillInsert`
- If auto-repair fails: manually insert fill event via admin API
- File postmortem; check for ledger write failure root cause

---

### INC-005: Ledger Write Failure

**Alert:** `LedgerWriteFailure`
**Impact:** Events not being persisted. Event sourcing integrity compromised.

**Immediate Actions:**
1. Check database availability: `trading_db_available{db="postgres"}`
2. Check connection pool: `trading_db_pool_connections{state="in-use"}`
3. Check disk space on database server
4. Review error details: `{event_type="LEDGER_WRITE_ERROR"}` in Loki

**Resolution:**
- If DB down: follow INC-008 (Database Unavailable)
- If connection pool exhausted: increase pool size, check for connection leaks
- If disk full: emergency disk expansion, delete old WAL segments

**CRITICAL:** After restoring DB, verify no events were lost:
```sql
SELECT aggregate_type, count(*), max(sequence_number)
FROM events
WHERE created_at > now() - interval '1 hour'
GROUP BY aggregate_type;
```

---

### INC-006: Database Unavailable

**Alert:** `DatabaseUnavailable`
**Impact:** Ledger writes fail, reconciliation paused, engine runs on in-memory state.

**Immediate Actions:**
1. Check DB health: `psql $DATABASE_URL -c 'SELECT 1'`
2. Check Neon.tech status page (for PostgreSQL/TimescaleDB)
3. Check Redis: `redis-cli -u $REDIS_URL ping`
4. Engine will continue trading using in-memory state (SQLite fallback for `engine.db`)

**Resolution:**
- Neon outage: wait for Neon recovery; engine self-heals on reconnect
- If >15 minutes: evaluate activating kill switch to prevent untracked trades
- After recovery: verify all events during downtime window are present

---

### INC-007: Reconciliation Failure

**Alert:** `ReconciliationFailure`
**Impact:** Ghost positions and missing fills may accumulate undetected.

**Immediate Actions:**
1. Check reconciliation health: `trading_health_component_score{component="reconciliation"}`
2. Check exchange health: `recon_v2_exchange_healthy`
3. Inspect reconciliation logs: `{event_type=~"RECONCILIATION.*"}` in Loki
4. Check if exchange APIs are accessible

**Resolution:**
- Restart reconciliation goroutine via admin API
- Verify exchange credentials haven't expired

---

## HIGH Incident Runbooks

---

### INC-HIGH-001: Execution Latency SLO Breach

**Alert:** `ExecutionLatencySLOBreach` — P99 >150ms
**Impact:** Execution quality degraded. Strategies may get worse fills.

**Investigation:**
1. Identify which pipeline stage is slow: Grafana Dashboard 02 → "Pipeline Stage Latency"
2. Common causes:
   - Exchange round-trip slow → check `trading_oms_oms_to_exchange_latency_ms`
   - Risk gate slow → check `trading_latency_pipeline_stage_ms{stage="strategy_to_risk"}`
   - DB slow → check `trading_db_query_latency_ms`
3. Check for GC pauses: `go_gc_duration_seconds`

**Resolution:**
- If exchange network latency: switch to closer colocation / CDN
- If risk engine slow: profile and optimise hot path
- If DB slow: check indexes, add connection pool capacity

---

### INC-HIGH-002: High Order Reject Rate

**Alert:** `OrderRejectRateHigh` — reject rate >15%
**Impact:** Strategies not executing; capital sitting idle.

**Investigation:**
1. Check reject reason: `{event_type="ORDER_REJECTED"}` in Loki, field `reason`
2. Common reasons:
   - `insufficient_margin` → position sizing too aggressive
   - `risk_blocked` → risk engine blocking (see INC risk alerts)
   - `exchange_error` → exchange-side issue (see INC exchange alerts)
   - `duplicate_order` → OMS deduplication issue

---

## Postmortem Template

```markdown
# Postmortem: [INC-YYYY-MM-DD-HHMMSS-XXXXX]

**Date:** YYYY-MM-DD
**Severity:** CRITICAL / HIGH
**Duration:** X hours Y minutes
**Capital Impact:** $X,XXX USD

## Summary
One paragraph: what happened, when, and what was the resolution.

## Timeline
- HH:MM UTC — [event]
- HH:MM UTC — [event]
- HH:MM UTC — [resolution]

## Root Cause
Primary technical cause.

## Contributing Factors
- Factor 1
- Factor 2

## Impact
- Capital at risk: $X
- Trades missed: N
- SLO impact: Xmin of OMS downtime

## Detection
How was the incident detected? (Alert / manual / user report)
Time from failure to detection: X minutes

## Resolution
Steps taken to resolve.

## Action Items
| Action | Owner | Due | Priority |
|--------|-------|-----|----------|
| Fix X | @owner | YYYY-MM-DD | P1 |

## Lessons Learned
What did we learn? What should we change?
```

---

## On-Call Checklist (before every shift)

- [ ] Grafana Dashboard 01 (Executive) — AUM, PnL, Drawdown green
- [ ] Grafana Dashboard 06 (Infrastructure) — all services green
- [ ] Grafana Dashboard 05 (Reconciliation) — drift score <10, no ghost positions
- [ ] AlertManager — no active CRITICAL or HIGH alerts
- [ ] `trading_dr_readiness_score == 1.0`
- [ ] `trading_execution_kill_switch_active == 0`
- [ ] Last DR snapshot age <300s
- [ ] All exchange feeds connected

## Emergency Contacts

- Kill switch toggle: `POST /api/admin/kill` with `{ "reason": "manual_halt" }`
- Reset kill switch: `POST /api/admin/reset` (requires CRITICAL approval)
- Incident log: `/var/log/trading/incidents.jsonl`
- Audit trail: MongoDB `loop_trades.audit_log`

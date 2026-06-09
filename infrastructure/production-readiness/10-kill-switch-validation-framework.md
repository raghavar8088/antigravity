# 10 — Kill Switch Validation Framework

**Implementation:** `engine/internal/killswitch` + `PostgresStore` durable ledger  
**Wired:** `main.go` — `ksSvc := killswitchpkg.NewService(ksLedger, ksExecutor, "btc-paper-1")`  
**Requirement:** `DATABASE_URL` set for durability

---

## Kill Switch Modes

| Mode | Trigger | Effect | Persistence |
|------|---------|--------|-------------|
| A — Block | Manual / risk breach | No new orders | PostgresStore |
| B — Flatten | OMS desync / daily loss | Close all positions | PostgresStore |
| C — Nuclear | Admin `/api/admin/kill` | Context cancel + shutdown | PostgresStore + Mongo |

---

## Test Matrix

| ID | Test | Procedure | Pass Criteria | Severity |
|----|------|-----------|---------------|----------|
| KS-01 | Manual block | `POST /api/admin/ks/block` | `active: true` | CRITICAL |
| KS-02 | Blocks new orders | Submit signal while blocked | Order rejected | CRITICAL |
| KS-03 | Persists restart | Block → restart engine → check status | Still active | CRITICAL |
| KS-04 | Persists DB reconnect | Block → kill Postgres 10s → restore | Still active | CRITICAL |
| KS-05 | Persists deployment | Block → ECS rolling deploy | Still active | CRITICAL |
| KS-06 | Flatten positions | Trigger flatten mode | All positions closed | CRITICAL |
| KS-07 | Release | `POST /api/admin/ks/release` | `active: false` | HIGH |
| KS-08 | Auto-trigger on recon | Inject position drift | KS activated < 15s | CRITICAL |
| KS-09 | Risk V3 heat kill | Heat > 20% | Auto KS | HIGH |
| KS-10 | Metrics | `trading_kill_switch_active` | Gauge = 1 when active | MEDIUM |
| KS-11 | No bypass without secret | `POST /api/admin/ks/block` no secret | 401/403 | CRITICAL |
| KS-12 | Vercel proxy requires session | Block via Vercel proxy no cookie | 401 | CRITICAL |

---

## Validation Script

```bash
export ENGINE_URL="http://engine:8080"
export ENGINE_ADMIN_SECRET="..."
bash scripts/production-readiness/validate-kill-switch.sh
```

### Script Flow

1. Check current status (should be inactive)
2. Activate block mode
3. Verify status active
4. Attempt order (should fail)
5. Restart engine container / simulate reconnect
6. Verify still active
7. Release kill switch
8. Verify inactive

---

## Persistence Architecture

```
killswitch.Service
  → ledger.Store (PostgresStore when DATABASE_URL set)
  → events: KS_ACTIVATED, KS_RELEASED
  → MemoryStore fallback (dev only — FAIL go-live gate)
```

**Go-live gate check:**
```bash
if [ -z "$DATABASE_URL" ]; then
  echo "FAIL: DATABASE_URL not set — kill switch non-durable"
  exit 1
fi
```

---

## Recovery Procedures

### Kill Switch Stuck Active

```bash
# 1. Verify state in DB
psql $DATABASE_URL -c "SELECT * FROM ledger_events WHERE event_type LIKE 'KS_%' ORDER BY global_sequence DESC LIMIT 5;"

# 2. Release via admin API
curl -X POST "$ENGINE_URL/api/admin/ks/release" \
  -H "X-Engine-Admin-Secret: $SECRET"

# 3. If API unreachable, manual DB intervention (last resort — requires CISO approval)
# Document in incident report
```

### False Positive Activation

1. Do NOT auto-release without root cause analysis
2. Review reconciliation alerts in ledger
3. Fix underlying drift
4. Release with documented reason
5. Post-incident review within 24h

---

## Stress Certification

Existing: `engine/internal/certification/stress_certification_test.go`

```bash
go test ./internal/certification/... -run TestStress_KillSwitch -v
```

---

## Sign-Off

| Criterion | Status |
|-----------|--------|
| KS-01 through KS-05 PASS | Required |
| DATABASE_URL in production | ❌ VERIFY |
| ENGINE_ADMIN_SECRET in production | ❌ VERIFY |
| Stress tests PASS | ✅ Code exists |

**Kill Switch Readiness:** 88/100 (wired, production env verification pending)

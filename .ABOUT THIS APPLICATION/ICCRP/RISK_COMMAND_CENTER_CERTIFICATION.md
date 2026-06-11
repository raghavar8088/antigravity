# RISK COMMAND CENTER CERTIFICATION

**Status:** PASS (after OMS/DB ribbon remediation)  
**Surfaces:** RiskRibbon (global), `/terminal/risk`, `/terminal/events`

---

## Visibility Matrix

| Signal | Source | File:Line | Visible When Failed |
|--------|--------|-----------|---------------------|
| Kill Switch | Engine `/api/killswitch/status` | `risk-ribbon/route.ts:175-194` | RED TRIGGERED / AMBER UNREACHABLE |
| Reconciliation | `paper_state.snapped_at` age | route.ts:110-111, platformEvents.ts:153-172 | AMBER STALE / CRITICAL |
| OMS | Engine health + order recency | route.ts:116-131 (**fixed**) | RED/AMBER DEGRADED |
| Market Data | `/api/btc/price` | route.ts:46-51 | RED OFFLINE |
| Watchdog | Engine `/health` | route.ts:57-71 | RED/AMBER via engineStatus |
| Execution | Same as engine health | route.ts:70 | DEGRADED |
| Engine | `/health` probe | route.ts:57-68 | RED |

---

## June 2026 Outage Scenario

**Question:** Can similar outage occur without operator visibility?

| Failure | Operator sees within 60s? |
|---------|---------------------------|
| Engine down | ✓ ENGINE RED, EXECUTION DEGRADED, OMS RED |
| Mongo unreachable | ✓ DATABASE RED/AMBER, RECON FAIL, guard blocks ICC |
| OMS stalled w/ positions | ✓ OMS AMBER (order age > 5min) |
| Reconciliation stale | ✓ RECON STALE + RECONCILIATION events |
| Kill switch | ✓ KILL SWITCH RED + event stream |
| Market data | ✓ MARKET DATA OFFLINE |

**Prior gap (fixed):** OMS always GREEN (`route.ts:116` ternary both branches GREEN).  
**Remediation:** Order recency + engine health logic (route.ts:116-131).

---

## Risk Module (`/terminal/risk`)

- VaR/CVaR: shows `—` when 0 (`RiskModule.tsx:37-39`)
- Reconciliation panel: `/api/engine/reconciliation` (lines 117-147)
- Correlation: separate API with NO DATA fallback

**Certification:** Operator has multi-surface outage visibility within 5s ribbon poll + 3s event poll.

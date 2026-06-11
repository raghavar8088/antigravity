# OPERATOR EFFECTIVENESS REPORT — ICCF-LDAP Phase 14

**Question:** Can an operator identify critical conditions within 60 seconds?

---

## Scenario Assessment

| Scenario | Detection Path | Time to Detect | ≤60s? | Gap |
|----------|---------------|----------------|-------|-----|
| Strategy failure | Strategy dashboard CRITICAL count; Research retirement preview; RiskRibbon PORTFOLIO | ~30s (30s poll on strategies) | **YES** | — |
| Risk escalation | RiskRibbon RED; Execution alerts; EventCenter RISK_EVENT | ~5s (ribbon poll) | **YES** | Risk page alone insufficient |
| Kill switch activation | RiskRibbon KILL SWITCH; EventCenter | ~5s if engine up | **PARTIAL** | KS not on Risk page; skipped if engine down L170 |
| Data feed failure | RiskRibbon MARKET DATA RED; Shell price `—` | ~5s | **YES** | — |
| OMS outage | RiskRibbon OMS (weak heuristic) | ~5s | **PARTIAL** | False GREEN when no orders L115 |
| Portfolio deterioration | Portfolio DD tiles; Ribbon DD; Alerts | ~10s | **YES** | TODAY PnL mislabeled may confuse |
| Backend total loss | REST circuit → guard blocks pages | ~9s (3×3s fail) | **YES** | Shell still visible |
| Pseudo-Sharpe bad strategy | Research leaderboard Sharpe column | Immediate (misleading) | **FALSE POSITIVE** | Operator may trust fake Sharpe |

---

## UI Alignment Issues Affecting Operators

1. **Research Sharpe column** — decisions on non-Sharpe metric (`mapSnapshotToTerminalDelta.ts:108`)
2. **Strategy summary PF** — shows 0.00 when null (`strategy-intelligence/route.ts:123`)
3. **Risk page missing KS/OMS/market** — must look at ribbon + page
4. **Two terminal paradigms** — `/terminal/*` institutional vs `/` TerminalDashboard with mock candles

---

## Operator Workflow Map

```
Login → RiskRibbon (5s) → overall GREEN/AMBER/RED
  → /terminal/execution (authority-guarded positions)
  → /terminal/strategies (retirement view)
  → /terminal/events (fills/risk only)
```

Missing from workflow: signal-level debugging (paper desk signal trace), full OMS lifecycle.

---

## Phase 14 Verdict

**PARTIAL PASS** — Critical risk states detectable within 60s **via RiskRibbon**, not via Risk page alone.

**Operator Effectiveness Score: 74/100** (claimed 88 — not supported by code)

---

## Remediation (Operator-Focused)

1. **P0** — Fix mislabeled metrics (Sharpe, TODAY PnL, portfolio PF).
2. **P0** — Consolidate risk indicators onto `/terminal/risk` (match ribbon).
3. **P1** — Add audible/visual flash on KILL_SWITCH event in EventCenter.
4. **P1** — Single "DATA TRUST" indicator when any metric is derived vs authoritative.

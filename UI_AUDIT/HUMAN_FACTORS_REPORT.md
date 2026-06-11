# PHASE 13 — HUMAN FACTORS REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Can users make dangerous mistakes?

---

## ERROR SCENARIO ANALYSIS

### Can users misunderstand that they're looking at mock data?
**Risk Level: CRITICAL**

Evidence:
- Terminal home page (`/`) shows `connected: true` in the snapshot state
- The TopBar likely shows a connected indicator
- All numbers look plausible (BTC ~$105K, positions showing realistic PnL, alerts showing CRITICAL severity)
- Nothing on the page says "DEMO MODE" or "NOT CONNECTED"
- A trader new to this platform (or returning after weeks) would have no reason to question the data
- **Result**: Trader could make decisions based on fictional risk metrics, fictional positions, fictional heat levels

### Can users misunderstand risk?
**Risk Level: CRITICAL**

Evidence:
- Risk module shows heat 3.7% (hardcoded). Actual heat could be 7% (block zone).
- No risk limit thresholds displayed alongside metrics (trader doesn't know what 3.7% means relative to limits)
- VaR $1,840 is hardcoded. Actual VaR depends on current positions — which the UI doesn't know.
- `InstitutionalRiskCenter` (dead code) was the only component designed to show risk limits vs actuals. It never renders.

### Can users misunderstand PnL?
**Risk Level: MEDIUM**

Evidence:
- Paper Desk PnL is live and correct (from MongoDB)
- Terminal PnL is mock
- A user looking at the wrong page would see the wrong PnL
- No page clearly labels itself "PAPER TRADING" vs "LIVE TRADING" in a persistent badge
- Daily PnL in BTC Futures dashboard (`DashboardHeader`) shows `dailyPnL?: number` prop — if `undefined`, nothing renders

### Can users miss alerts?
**Risk Level: CRITICAL**

Evidence:
- No push notifications
- No sound alerts
- Alert tape in terminal: hardcoded mock, "CRITICAL" always visible (alert fatigue)
- No alert log or history
- A trader away from the screen for 4 hours has zero way to know what happened
- Strategy entering CRITICAL health: only detectable by noticing the count changed in Paper Desk summary

### Can users miss failures?
**Risk Level: CRITICAL**

Evidence:
- Go engine crash: Paper Desk shows `connection: "stale"` or `"error"` — the ONLY failure indicator
- Kill switch activation: no automatic UI indicator
- Reconciliation failure: completely invisible
- Data feed failure: completely invisible
- Broker outage: completely invisible

---

## USER MISTAKE SCENARIOS

### Mistake 1: Pressing "Kill" without understanding target
**Evidence**: `DashboardHeader.tsx:77` — `const apiUrl = resolveEngineApiUrl();`
- `resolveEngineApiUrl()` returns Go engine URL
- In development: `http://localhost:8080`
- In production (if not configured): falls back to `http://127.0.0.1:8080`
- There is no indicator showing WHICH engine the kill button targets
- A trader pressing Kill in development terminates the local engine, not production

**Mitigation**: `if (!confirm(confirmation)) return;` — there is a browser confirm dialog. This is minimal but present.

### Mistake 2: Trusting the kill switch as protection without knowing its state
**Evidence**: No status polling, no activation indicator
- Trader may press Kill, believe the engine stopped, see positions still updating (because the paper desk polls MongoDB which the engine already wrote to)
- Trader would not know if Kill succeeded or failed

### Mistake 3: Double-counting positions
**Evidence**: Paper desk shows paper positions; terminal shows (mock) BTC positions
- If a trader looks at both, they could believe they have 2 real positions when they have 0 (or vice versa)

### Mistake 4: Confusing simulation with live
**Evidence**: `/mock-trading` shows a realistic account state with real-looking PnL
- The only distinction is the route URL
- No persistent "SIMULATION" badge confirmed in MockTradingDashboard

### Mistake 5: Acting on stale strategy health
**Evidence**: Strategy health is lazy-loaded in Paper Desk
- A trader might look at the health counts (showing "2 critical") and navigate to the health tab
- The health tab fetch could return stale data if MongoDB hasn't been updated recently
- There is no "last updated" timestamp on strategy health data

---

## HUMAN FACTORS SCORECARD

| Risk | Severity | Mitigated? |
|------|----------|-----------|
| Mock data mistaken for live | CRITICAL | NO |
| Stale data no timestamp | HIGH | NO |
| Alert fatigue (always-on CRITICAL) | HIGH | NO |
| Kill switch target unclear | MEDIUM | PARTIAL (confirm dialog) |
| Kill switch state unknown | CRITICAL | NO |
| No failure notification | CRITICAL | NO |
| Simulation vs live confusion | MEDIUM | NO |
| Risk thresholds not displayed | HIGH | NO |
| Lazy-loaded state not obvious | MEDIUM | NO |

**Score: 1/10 — Extremely high risk of operational mistakes**

**The single most dangerous human factors issue**: The primary dashboard is permanently showing mock data while appearing to be connected and live. A trader who does not know to look at Paper Desk instead would operate on completely incorrect information.

# PHASE 9 — ALERTING REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Does the UI alert traders to failures?

---

## ALERTING INFRASTRUCTURE INVENTORY

### Terminal Alert Tape
**Location**: `ExecutionCenter.tsx` — "Alert Tape" section
**Data Source**: `TerminalSnapshot.alerts` — HARDCODED MOCK DATA
**Alerts in demo snapshot**:
- A-1: CRITICAL "Drawdown Guard" — "Heat remains below block threshold; monitor if BTC volatility expands" (hardcoded, always shows)
- A-2: WARNING "Funding Spike" — funding rate above session median (hardcoded)
- A-3: INFO "Trade Opened" — Funding Mean Reversion Alpha opened long (hardcoded)

**CRITICAL**: These alerts are frozen demo content. A real drawdown would not appear here. A real broker outage would not appear here. The "CRITICAL" alert badge is always present regardless of actual system state.

### Paper Desk Connection Badge
**Location**: `PaperDeskDashboard.tsx`
**Displays**: connection state (`"connecting" | "live" | "stale" | "error" | "unauthorized"`)
**LIVE**: Yes
**Alerts on**: HTTP failure reaching `/api/paper-desk/snapshot`
**Does NOT alert on**: Individual subsystem failures (strategy failures, broker issues, OMS errors)

### Admin Event Toast
**Location**: `DashboardHeader.tsx:82` — `onAdminEvent?.(successMessage, "admin")`
**Purpose**: Shows confirmation/failure of admin actions (kill switch, reset)
**Limitations**: Ephemeral toast only — no persistent alert log

### MockRiskAnalyticsPanel Kill-Switch Display
**Location**: `MockRiskAnalyticsPanel.tsx`
**Purpose**: Shows local kill-switch conditions (daily loss %, margin ratio)
**Operational**: Simulation only — no connection to Go engine

---

## FAILURE SCENARIO COVERAGE

### Scenario: No trades occurring (trading halted unexpectedly)
**Alert exists**: NO
Evidence: No component monitors "time since last trade" or alerts if no activity for N minutes. Paper desk shows recent trades but no "silence alert."

### Scenario: No fills (orders stuck pending)
**Alert exists**: NO
Evidence: No component monitors pending-order age. No stuck-order alert.

### Scenario: No signals generated
**Alert exists**: NO
Evidence: Signal trace panel shows signal funnel but no "no signals generated in last 30 minutes" alert.

### Scenario: Data feed failure
**Alert exists**: NO
Evidence: No broker connectivity monitor. If Coinbase WS drops, Binance REST fails, and the fallback also fails, no UI alert fires.

### Scenario: Broker outage (AngelOne down, Delta Exchange unreachable)
**Alert exists**: NO
Evidence: No broker health check. No component calls a broker health endpoint. No indicator shows broker connectivity.

### Scenario: Risk violation (heat > 6, drawdown > threshold)
**Alert exists**: NO (live)
Evidence: Terminal risk module shows heat — MOCK DATA. No live risk alert fires. `futuresDeskPolicy.ts` defines thresholds in code but there is no UI alert that fires when they're breached.

### Scenario: Kill switch activation
**Alert exists**: NO
Evidence: Kill switch can be triggered via `DashboardHeader` button. If it triggers automatically (circuit breaker), no UI alert is generated. No component polls `/api/admin/ks/status` to show activation state.

### Scenario: Reconciliation drift
**Alert exists**: NO
Evidence: Reconciliation visibility is zero (Phase 8 finding).

### Scenario: Strategy failure (CRITICAL health)
**Alert exists**: PARTIAL
Evidence: Paper Desk shows `healthSummary.critical` count. This is a count, not an alert. Trader must notice the count changed. No push notification. No alert banner.

### Scenario: Alpha engine failure (Go engine crash)
**Alert exists**: PARTIAL
Evidence: If Go engine crashes, the `/api/paper-desk/snapshot` poll fails after timeout. Paper desk shows `connection: "stale"` or `"error"` badge after 5s. This is the only failure mode that produces any visible indicator.

---

## ALERT DELIVERY GAPS

| Alert Type | Mechanism | Status |
|-----------|-----------|--------|
| Visual badge | Paper desk connection status | EXISTS (limited scope) |
| Alert tape | Terminal execution center | MOCK DATA ONLY |
| Toast notification | Admin actions only | EXISTS (very limited) |
| Email/SMS notification | None found | ABSENT |
| Sound alert | None found | ABSENT |
| Browser notification | None found | ABSENT |
| Persistent alert log | None found | ABSENT |
| Alert history/acknowledgement | None found | ABSENT |
| Alert severity triage | None found | ABSENT |

---

## CRITICAL FINDING: FALSE CONFIDENCE FROM MOCK ALERTS

The terminal home page shows a CRITICAL alert badge ("Drawdown Guard") at all times because it's hardcoded. A trader scanning the alerts sees "CRITICAL" and either:
1. Investigates, finds nothing real, and starts ignoring critical alerts
2. Assumes the system is fine because critical alerts are always there

Both outcomes are dangerous. The hardcoded critical alert trains traders to ignore alerts.

**Evidence**: `client/src/lib/terminal/terminalSnapshot.ts:116-118`

---

## ALERTING REPORT SCORECARD

| Alert Coverage | Status |
|---------------|--------|
| No trades alert | ABSENT |
| No fills alert | ABSENT |
| No signals alert | ABSENT |
| Data feed failure alert | ABSENT |
| Broker outage alert | ABSENT |
| Risk violation alert | ABSENT |
| Kill switch activation alert | ABSENT |
| Reconciliation drift alert | ABSENT |
| Strategy failure alert | PARTIAL (count only) |
| Engine crash alert | PARTIAL (connection badge) |

**Score: 1/10 — Alerting infrastructure is critically insufficient**

**The only genuine alert is the paper desk "stale/error" connection badge. Everything else is either absent or hardcoded demo content.**

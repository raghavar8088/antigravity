# PHASE 7 — RISK VISIBILITY REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Can a trader understand and supervise risk?

---

## RISK COMPONENTS INVENTORY

### A. Terminal Risk Module (`/terminal/risk`)
**File**: `client/src/components/terminal/institutional/RiskModule.tsx`
**Data Source**: `useTerminalSnapshot()` — HARDCODED MOCK DATA
**Displays**: VaR 95/99, CVaR 95, heat %, drawdown %, net/gross exposure, margin usage %, long/short exposure, funding paid/received

**CRITICAL**: All values are from `initialTerminalSnapshot`:
- `var95Usd: 1_840` (hardcoded)
- `var99Usd: 2_760` (hardcoded)
- `heatPct: 3.7` (hardcoded)
- `drawdownPct: 1.4` (hardcoded)
- `marginUsagePct: 18.6` (hardcoded)

**A trader looking at this page believes they are seeing live risk metrics. They are not.**

### B. MockRiskAnalyticsPanel
**File**: `client/src/components/MockRiskAnalyticsPanel.tsx`
**Data Source**: Local computation from mock account state
**Kill Switch Logic**: Checks daily loss %, max open trades, margin ratio against config thresholds
**Operational Value**: Research/simulation only — has no connection to Go engine risk gate

### C. InstitutionalRiskCenter
**File**: `client/src/components/InstitutionalRiskCenter.tsx`
**Data Source**: Props (`data: InstitutionalRiskCenterData`)
**Status**: DEAD CODE — never rendered in any page
**Contains**: VaR, CVaR, heat, Kelly sizing, exposure, attribution, forecast, capital allocation, risk budget, alerts
**Operational Value**: Zero — unreachable

### D. Paper Desk Risk Indicators
**File**: `client/src/components/PaperDeskDashboard.tsx`
**Data Source**: `usePaperDesk()` → MongoDB → LIVE
**Displays**: Peak equity, current drawdown (derived from `PaperStateDoc`)
**Operational Value**: Partial — drawdown is live but no VaR, no heat tracking, no exposure

---

## RISK METRICS AUDIT

### Risk Limits
**Verdict: ABSENT**

Evidence:
- `lib/futuresDeskPolicy.ts` contains risk limits (max position size, max daily loss, etc.)
- These are defined as code constants, not displayed in any dashboard
- Trader cannot see what the current risk limits are without reading source code

### Exposure
**Verdict: PARTIAL (mock) / ABSENT (live)**

Evidence:
- Terminal Risk Module shows net/gross exposure — MOCK DATA
- Paper Desk does not show exposure breakdown
- Go engine exposure calculated in `engine/internal/risk/gate/` — no UI connection

### Drawdown
**Verdict: PARTIAL (live for paper desk)**

Evidence:
- Paper Desk: drawdown shown in summary KPIs — live from MongoDB
- Terminal: drawdown shown — MOCK DATA
- No drawdown alert threshold visible — trader doesn't know at what drawdown % trading halts

### Leverage
**Verdict: ABSENT**

Evidence:
- `InstitutionalRiskCenter` has `leverageDashboard: number` field — DEAD CODE
- Terminal Risk Module snapshot includes leverage via gross/net exposure — MOCK DATA
- No live leverage display

### Position Concentration
**Verdict: ABSENT**

Evidence:
- `lib/futuresAttribution.ts` computes attribution by strategy/family
- No live concentration display in any active dashboard

### Portfolio Risk
**Verdict: PARTIAL (mock) / ABSENT (live)**

Evidence:
- Terminal Risk Module: portfolio summary — MOCK DATA
- `InstitutionalRiskCenter`: full portfolio risk — DEAD CODE
- Paper Desk: only drawdown, no full risk profile

### Risk Blocks (Circuit Breakers)
**Verdict: ABSENT**

Evidence:
- `engine/internal/risk/gate/` implements risk blocks
- `lib/futuresDeskPolicy.ts`: heat levels (0–4 normal, 4–6 reduce, 6+ block) defined in code
- No UI shows current heat level against these thresholds
- No UI shows whether a risk block is currently engaged
- The only risk "action" UI is the kill switch button in DashboardHeader — which performs a full halt, not a graduated block

---

## HEAT TRACKING — DEEP DIVE

The system defines heat levels:
- 0–4: Normal
- 4–6: Reduce
- 6+: Block

Evidence in `client/src/lib/futuresDeskPolicy.ts` (referenced in architecture notes).

The terminal Risk Module shows `heatPct: 3.7` — HARDCODED. A trader sees "heat 3.7% — normal" but this is demo data, not their actual heat level. If actual heat were at 5.5% (reduce zone) or 7% (block zone), the dashboard would still show 3.7%.

**This is the most operationally dangerous false confidence gap in the system.**

---

## CRITICAL FINDING: RISK MODULE IS A DEMO

The primary risk dashboard (`/terminal/risk`) shows:
- A BTC price that is 7–10+ days stale (based on hardcoded value $105,842.50)
- Positions that don't exist
- Risk metrics that have no relationship to actual portfolio

A trader who trusts this dashboard to supervise risk would be operating blind.

**Evidence**:
- `client/src/lib/terminal/terminalSnapshot.ts:17` — `initialTerminalSnapshot` with all hardcoded values
- `client/.env.local` — no `NEXT_PUBLIC_TERMINAL_WS_URL`
- `client/src/lib/terminal/terminalStore.tsx:24` — `if (!wsUrl) return;` (no connection without env var)

---

## RISK VISIBILITY SCORECARD

| Risk Metric | Status |
|-------------|--------|
| Risk limits display | ABSENT |
| Live exposure | ABSENT |
| Live drawdown | PARTIAL (paper desk only) |
| Leverage | ABSENT |
| Position concentration | ABSENT |
| Portfolio risk | PARTIAL (mock only) |
| Risk blocks / circuit breakers | ABSENT |
| Heat tracking | ABSENT (mock shows stale value) |
| VaR / CVaR live | ABSENT |
| Correlation risk | ABSENT (dead component) |
| Kelly sizing live | ABSENT (dead component) |

**Score: 1/10 — Risk visibility is critically deficient. The only live risk data is paper desk drawdown.**

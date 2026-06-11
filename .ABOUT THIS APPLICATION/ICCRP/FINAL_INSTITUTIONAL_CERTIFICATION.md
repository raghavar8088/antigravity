# ICCF-LDAP FINAL INSTITUTIONAL CERTIFICATION

**Audit date:** 2026-06-11  
**Auditor method:** Source code, API routes, React hooks, stores, reachability proofs only  
**Prior verdict challenged:** VERDICT 2 — CERTIFIED WITH MINOR GAPS

---

## Ten Certification Questions

| # | Question | Answer |
|---|----------|--------|
| 1 | Can every displayed value be trusted? | **NO** — pseudo-Sharpe, null PF as 0.00, TODAY PnL mislabel, WS zero flash |
| 2 | Are synthetic values still visible? | **YES** — Research Sharpe (`evidence_score/50`); zero defaults via `num()` |
| 3 | Can operators trust balances? | **YES on `/terminal/portfolio`** — Mongo authority; shell header does not show balance |
| 4 | Can operators trust PnL? | **MOSTLY** — portfolio/strategies yes; ribbon TODAY PnL no |
| 5 | Can operators trust positions? | **YES** — `listOpenPositions` → snapshot → ExecutionCenter |
| 6 | Can operators trust strategy analytics? | **PARTIAL** — per-strategy metrics yes; summary PF and Sharpe no |
| 7 | Can operators trust SEP rankings? | **N/A in UI** — SEP APIs exist; terminal uses strategy-intelligence |
| 8 | Can operators trust risk indicators? | **PARTIAL** — global ribbon yes; risk page incomplete |
| 9 | Can operators trust event streams? | **PARTIAL** — fills/positions/orders/risk/KS only; no signals/recon |
| 10 | Is platform institutional-grade? | **NOT YET** — authority model improving but material visibility risks remain |

---

## VERDICT

# VERDICT 3 — MATERIAL VISIBILITY RISKS REMAIN

**Not VERDICT 1.** Prior VERDICT 2 is **not supported** by code evidence due to mislabeled metrics displayed as authoritative and WS pre-authority zero render path.

---

## Scores

| Dimension | Prior Claim | Audit Score | Delta |
|-----------|-------------|-------------|-------|
| UI Alignment | 91 | **71** | -20 |
| Institutional Dashboard | 90 | **76** | -14 |
| Observability | 90 | **78** | -12 |
| Operator Effectiveness | 88 | **74** | -14 |
| Production Readiness | — | **72** | — |
| Trustworthiness | — | **68** | — |

---

## Remediation Backlog (Ranked by ROI × Risk ÷ Effort)

| Rank | Item | ROI | Risk | Effort | Files |
|------|------|-----|------|--------|-------|
| 1 | Fix pseudo-Sharpe display | HIGH | HIGH | 2h | `mapSnapshotToTerminalDelta.ts:108`, `ResearchCenter.tsx:46`, strategy-intelligence route |
| 2 | WS authority only after first delta | HIGH | HIGH | 1h | `terminalStore.tsx:48-56` |
| 3 | Fix portfolio PF null → 0.00 | HIGH | MED | 1h | `strategy-intelligence/route.ts:123`, `StrategyIntelligenceDashboard.tsx:164` |
| 4 | Fix RiskRibbon TODAY PnL | HIGH | MED | 2h | `risk-ribbon/route.ts:103` |
| 5 | Guard TerminalShell header | MED | MED | 2h | `TerminalShell.tsx`, `TerminalAuthorityGuard.tsx` |
| 6 | Unify strategy poll limit 600 | MED | LOW | 30m | `terminalStore.tsx:94` |
| 7 | Embed ribbon indicators in Risk page | MED | MED | 4h | `RiskModule.tsx` |
| 8 | Emit SIGNAL/RECON events | MED | MED | 4h | `platformEvents.ts` |
| 9 | Wire SEP API to terminal when available | MED | LOW | 3h | Research/Strategies pages |
| 10 | Replace num(v,0) with nullable | MED | MED | 4h | `mapSnapshotToTerminalDelta.ts` |
| 11 | Wire or remove useEngineState | LOW | LOW | 1h | `useEngineState.ts` |
| 12 | Connect EventCenter to SSE | LOW | LOW | 3h | `EventCenter.tsx` |

---

## Exact Code Remediation Plans

### R1 — Pseudo-Sharpe (P0)

```typescript
// mapSnapshotToTerminalDelta.ts — mapStrategies
sharpe: s.sharpe_ratio ?? NaN,  // add sharpe_ratio to StrategyIntelRow from API

// ResearchCenter.tsx — column header + cell
<th>Evidence/50</th>  // OR hide until real sharpe available
{Number.isFinite(strategy.sharpe) ? strategy.sharpe.toFixed(2) : "—"}
```

Extend `strategy-intelligence/route.ts` to include `sharpe_ratio` from Mongo scores or SEP.

### R2 — WS Pre-Authority (P0)

```typescript
// terminalStore.tsx — WS_OPEN case
case "WS_OPEN":
  return {
    ...state,
    connected: true,
    loading: true,  // keep loading until delta
    authoritySource: "ws",
    restUnavailable: false,
    hasAuthority: false,  // require WS_DELTA
  };
```

### R3 — Portfolio PF (P0)

```typescript
// strategy-intelligence/route.ts
import { computePortfolioPF } from "@/lib/paperTradeAnalyticsApi";
portfolio_stats: {
  ...
  profit_factor: closedStats.profit_factor ?? computePortfolioPF(closedStats),
}
```

### R4 — TODAY PnL (P0)

```typescript
// risk-ribbon/route.ts — replace L103
const startOfDay = new Date(); startOfDay.setUTCHours(0,0,0,0);
todayPnl = await sumClosedPnlSince(accountKey, startOfDay.toISOString());
```

### R5 — Shell Guard (P1)

Wrap metrics block in `TerminalShell.tsx:45-57` with `snapshot.hasAuthority && snapshot.updatedAt`.

---

## What Passed

- Portfolio analytics page — full Mongo accounting authority
- Strategy intelligence per-row metrics from Mongo scores
- REST circuit breaker and authority guard on 5/8 content pages
- Global RiskRibbon — composite live health (with noted bugs)
- Browser execution permanently disabled
- API authentication on all audited routes
- Explicit degraded states on portfolio/strategies/events dashboards

---

## Certification Status

**INCOMPLETE** until R1–R4 remediated and re-audited with runtime proof.

To achieve **VERDICT 1**, all displayed metrics must trace to backend authority with no mislabeled derivations, full authority guard coverage, and complete event taxonomy in UI.

---

## Reports Generated

1. LIVE_DATA_AUTHORITY_REPORT.md  
2. SYNTHETIC_DATA_CERTIFICATION.md  
3. TERMINAL_AUTHORITY_CERTIFICATION.md  
4. ENGINE_STATE_AUTHORITY_CERTIFICATION.md  
5. API_FORENSIC_CERTIFICATION.md  
6. SEP_AUTHORITY_CERTIFICATION.md  
7. STRATEGY_INTELLIGENCE_CERTIFICATION.md  
8. PORTFOLIO_ANALYTICS_CERTIFICATION.md  
9. RISK_CENTER_CERTIFICATION.md  
10. EVENT_STREAM_CERTIFICATION.md  
11. DATA_CONSISTENCY_AUDIT.md  
12. FAILURE_MODE_CERTIFICATION.md  
13. OBSERVABILITY_CERTIFICATION.md  
14. OPERATOR_EFFECTIVENESS_REPORT.md  
15. This document (FINAL)

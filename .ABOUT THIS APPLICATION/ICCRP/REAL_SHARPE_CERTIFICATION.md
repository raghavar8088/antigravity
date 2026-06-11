# REAL SHARPE CERTIFICATION

**Status:** PASS (synthetic Sharpe eliminated)  
**Audited:** 2026-06-11  
**Verdict contribution:** Trustworthiness + Observability

## Finding (Before)

| Location | Issue |
|----------|-------|
| `client/src/lib/terminal/mapSnapshotToTerminalDelta.ts:108` | `sharpe: s.evidence_score / 50` — not a Sharpe ratio |

## Remediation

### Strategy-level Sharpe — REMOVED (Option B)

Mongo `strategy_scores` has no Sharpe field (`paperDeskClient.ts:151-163`). Per-strategy Sharpe is **not persisted**.

- `mapStrategies()` now sets `sharpe: null` (`mapSnapshotToTerminalDelta.ts:108`)
- `StrategyResearchRow.sharpe` typed as `number | null` (`terminalTypes.ts:56`)
- UI shows `—` when null (`ResearchCenter.tsx:60-62`)

### Portfolio-level Sharpe — REAL SOURCE WIRED (Option A)

| Layer | File | Function / Line | Authority |
|-------|------|-----------------|-----------|
| Formula | `mockExtendedMetrics.ts:175-180` | `(averageTrade / pnlStdDev) * sqrt(n)` | Closed trade PnL series |
| Loader | `portfolioAccountingService.ts:228-241` | `getPortfolioExtendedMetrics()` | Mongo `paper_trades` |
| API | `paper-desk/snapshot/route.ts:49-51` | Enriches accounting snapshot | REST `/api/paper-desk/snapshot` |
| Mapper | `mapSnapshotToTerminalDelta.ts:167-168` | `nullableNum(portfolio?.sharpe)` | No zero fallback |
| UI | `AnalyticsCenter.tsx:57-58` | Shows value or `—` | Terminal analytics panel |

## Reachability Proof

```
Mongo paper_trades
  → getPortfolioExtendedMetrics() [portfolioAccountingService.ts:228]
  → /api/paper-desk/snapshot [snapshot/route.ts:49]
  → mapSnapshotToTerminalDelta() [mapSnapshotToTerminalDelta.ts:167]
  → terminalStore REST_OK / WS_DELTA [terminalStore.tsx:68-75]
  → AnalyticsCenter [AnalyticsCenter.tsx:57]
```

## UI Proof

- Research leaderboard Sharpe column: `—` (no synthetic value)
- Analytics Sharpe 30D/90D: portfolio Sharpe from accounting or `—`
- TerminalDashboard Sharpe KPI: `—` (synthetic expectancy-based proxy removed)

## Certification

- [x] No `evidence_score / 50` Sharpe mapping remains
- [x] Portfolio Sharpe sourced from `computeExtendedMetricsFromRecords`
- [x] Null Sharpe renders as `—`, never `0.00`

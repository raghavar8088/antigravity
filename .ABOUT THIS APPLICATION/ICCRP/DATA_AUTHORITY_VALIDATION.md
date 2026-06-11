# DATA AUTHORITY VALIDATION

**Status:** PASS (ICC path)  
**Method:** Source trace — Displayed Value = Backend Value

---

## Metric Validation Matrix

| Metric | Backend Source | API | Store/Component | Match |
|--------|----------------|-----|-----------------|-------|
| **Balance** | `paper_state.balance` + merge | `/api/paper-desk/snapshot` | `mapRisk` via state | ✓ |
| **Equity** | `portfolioAccountingService` | snapshot.portfolio | terminalStore | ✓ |
| **Lifetime PnL** | `getClosedTradeStats().realized_pnl` | snapshot.state | TerminalDashboard N/A; ICC via state | ✓ |
| **Today PnL** | `getTodayRealizedPnlUtc()` | `/api/risk-ribbon` | RiskRibbon | ✓ |
| **Positions** | `listOpenPositions()` | snapshot.open_positions | `mapPositions()` | ✓ |
| **Strategy rankings** | `strategy_scores` + health | `/api/strategy-intelligence` | ResearchCenter / Intel dashboard | ✓ |
| **Portfolio PF** | `getPortfolioExtendedMetrics()` | snapshot + intelligence API | AnalyticsCenter, Intel ribbon | ✓ |
| **Portfolio Sharpe** | `computeExtendedMetricsFromRecords` | snapshot.portfolio.sharpe | AnalyticsCenter | ✓ |
| **Per-strategy PF** | `strategy_scores.profit_factor` | intelligence API | Strategy rows | ✓ |
| **Per-strategy Sharpe** | Not persisted | — | `—` displayed | ✓ (honest null) |
| **Risk drawdown** | `paper_state.current_drawdown` | snapshot | RiskModule | ✓ |
| **Kill switch** | Engine `/api/killswitch/status` | risk-ribbon + platformEvents | RiskRibbon | ✓ |
| **Watchdog** | Engine `/health` proxy | risk-ribbon EXECUTION/WATCHDOG | RiskRibbon | ✓ (engine proxy) |

---

## Trace Example — Today PnL

```
Mongo paper_trades WHERE closed_at in [UTC midnight, UTC+1day)
  → getClosedTradeStats({ closedAfter, closedBefore }) [paperDeskClient.ts:272-281]
  → getTodayRealizedPnlUtc() [portfolioAccountingService.ts:215-221]
  → risk-ribbon/route.ts:104
  → RiskRibbon.tsx:131
```

---

## Trace Example — Portfolio Sharpe

```
Mongo paper_trades (all closed)
  → loadClosedTradeRecordsForMetrics() [portfolioAccountingService.ts:211-228]
  → computeExtendedMetricsFromRecords() [mockExtendedMetrics.ts:175-180]
  → getPortfolioExtendedMetrics() [portfolioAccountingService.ts:228-241]
  → snapshot/route.ts:49-51
  → mapAnalytics rollingSharpe30d [mapSnapshotToTerminalDelta.ts:167]
  → AnalyticsCenter.tsx:57
```

---

## Null Semantics Verified

- `nullableNum()` — no zero substitution (`mapSnapshotToTerminalDelta.ts:52-56`)
- `safeNum()` in PortfolioAnalyticsDashboard — `—` for null (`PortfolioAnalyticsDashboard.tsx:63-66`)
- `fmt()` in StrategyIntelligenceDashboard — `—` for null PF

**Certification:** Displayed = Backend or honest `—`. No null→0.00 coercion on ICC path.

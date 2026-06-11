# DATA TRUTH CERTIFICATION

**Status:** PASS (institutional terminal path)  
**Audited:** 2026-06-11

## Metric Authority Matrix

| Displayed Metric | Backend Source | UI Path | Truth Status |
|------------------|----------------|---------|--------------|
| Balance | Mongo `paper_state` + closed stats merge | snapshot → terminalStore | ✓ |
| Equity | `portfolioAccountingService` | snapshot → risk module | ✓ |
| Realized PnL | `getClosedTradeStats()` | state.realized_pnl | ✓ |
| Today PnL | `getTodayRealizedPnlUtc()` | risk-ribbon | ✓ (fixed) |
| Portfolio PF | `getPortfolioExtendedMetrics()` | strategy-intelligence, snapshot | ✓ (fixed) |
| Portfolio Sharpe | `computeExtendedMetricsFromRecords` | snapshot → analytics | ✓ (fixed) |
| Strategy PF | Mongo `strategy_scores.profit_factor` | strategy-intelligence rows | ✓ |
| Strategy Sharpe | N/A in Mongo | Shows `—` | ✓ (no synthetic) |
| Trades | Mongo `paper_trades` | journal, event center | ✓ |
| Positions | Mongo `paper_positions` | execution center | ✓ |
| Strategy count | `listStrategyScores()` count | strategy-intelligence | ✓ |

## Null Semantics

All nullable metrics render `—`, never synthetic zero:
- `nullableNum()` in `mapSnapshotToTerminalDelta.ts`
- `fmt()` in `StrategyIntelligenceDashboard.tsx`
- `safeNum()` in `PortfolioAnalyticsDashboard.tsx`

## Remaining Non-Institutional Paths

Legacy `TerminalDashboard` (non `/terminal/*`) uses direct `usePaperDesk` hook — separate authority path. Sharpe KPI disabled there to prevent synthetic display.

## Test Evidence

`iccrpImplementation.test.ts` — 7 tests passing including authority and delta mapping assertions.

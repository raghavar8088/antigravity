# PORTFOLIO CENTER BLUEPRINT — ICCRP V3

**Route:** `/terminal/portfolio`  
**Component:** `client/src/components/PortfolioAnalyticsDashboard.tsx`  
**Service:** `client/src/lib/portfolioAccountingService.ts`

---

## Display Fields

| Field | Source | Proof |
|-------|--------|-------|
| Equity Curve | `/api/paper-desk/equity` | `PortfolioAnalyticsDashboard.tsx` + `AnalyticsCenter.tsx` |
| Portfolio PnL | `portfolioAccountingService` | `portfolioAccountingService.ts` L3 comment |
| Portfolio PF | snapshot portfolio stats | `mapSnapshotToTerminalDelta.ts` L174 |
| Portfolio Sharpe | `portfolio.sharpe` | `iccrpImplementation.test.ts` L54 |
| Drawdown | `portfolio.drawdown.current_drawdown` | `mapSnapshotToTerminalDelta.ts` L142 |
| Exposure | `portfolio.exposure` | L127-145 |
| Strategy Contribution | strategy intel rows | `/api/strategy-intelligence` |
| Correlation | `/api/paper-trades/correlation-matrix` | `RiskModule.tsx` L21 |

---

## Authority

All portfolio metrics derive from MongoDB collections via `paperDeskClient.ts` — not browser-computed.

---

## Gaps

| Field | Status |
|-------|--------|
| Sortino | Not exposed in UI — available in risk-metrics API |
| Volatility | Partial via analytics |
| Regime Distribution | `/api/paper-trades/regime-analysis` exists, not on portfolio page |
| Capital Allocation tiers | In strategy intel, not aggregated on portfolio page |

---

## Status: OPERATIONAL — P2 enhancement for Sortino + regime panel

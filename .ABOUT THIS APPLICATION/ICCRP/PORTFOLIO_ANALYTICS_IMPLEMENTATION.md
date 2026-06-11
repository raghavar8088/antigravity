# OBJECTIVE 7 — PORTFOLIO ANALYTICS DASHBOARD

## Route

`/terminal/portfolio` → `PortfolioAnalyticsDashboard.tsx`

## Authority

`GET /api/paper-desk/portfolio` → `getPortfolioAccountingSnapshot()`  
`GET /api/paper-desk/equity` → equity curve sparkline

## Metrics (all backend-sourced)

Balance, Equity, Realized/Unrealized PnL, Drawdowns (current/max/daily/weekly), PF, Sharpe, Sortino, Calmar, Win Rate, Expectancy, Fees, Exposure (USD/Long/Short/Net)

## Paper Desk Header

`/api/paper-desk/risk-metrics` adds Sharpe/Sortino/Calmar to Account Summary tiles.

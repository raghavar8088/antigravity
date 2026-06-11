# PHASE 2 — DASHBOARD INVENTORY
## Forensic Audit | Trading Platform | 2026-06-11

---

## DASHBOARD REGISTER

### D-01: Terminal Execution Center
- **Route**: `/terminal/execution`
- **Component**: `client/src/components/terminal/institutional/ExecutionCenter.tsx`
- **Purpose**: Real-time BTC execution monitoring — order book depth, positions, alerts
- **Audience**: Active trader supervising live positions
- **Backend Dependency**: `NEXT_PUBLIC_TERMINAL_WS_URL` WebSocket
- **Data Dependency**: `TerminalSnapshot.bids/asks/positions/risk/alerts`
- **LIVE?**: **NO** — WebSocket env var unset. Shows hardcoded BTC-PERP-1042 and BTC-PERP-1048 positions with fixed prices.
- **Operational Value IF WIRED**: Very high — tick-level order book + positions + liquidation + alert tape
- **Operational Value CURRENT**: Zero — all numbers are fictional

### D-02: Terminal Analytics Center
- **Route**: `/terminal/analytics`
- **Component**: `client/src/components/terminal/institutional/AnalyticsCenter.tsx`
- **Purpose**: Portfolio performance analytics — equity curve vs BTC benchmark, rolling Sharpe, R-multiples
- **Audience**: Portfolio analyst
- **Backend Dependency**: `NEXT_PUBLIC_TERMINAL_WS_URL` WebSocket
- **LIVE?**: **NO** — hardcoded 6 equity curve points, Sharpe 1.84, win rate 54.2%
- **Operational Value CURRENT**: Zero

### D-03: Terminal Research Center
- **Route**: `/terminal/research`
- **Component**: `client/src/components/terminal/institutional/ResearchCenter.tsx`
- **Purpose**: Strategy leaderboard, walk-forward validation
- **Backend Dependency**: `NEXT_PUBLIC_TERMINAL_WS_URL`
- **LIVE?**: **NO** — 5 hardcoded strategies shown
- **Operational Value CURRENT**: Zero

### D-04: Terminal Risk Module
- **Route**: `/terminal/risk`
- **Component**: `client/src/components/terminal/institutional/RiskModule.tsx`
- **Purpose**: VaR/CVaR, heat tracking, correlation heatmap, Kelly sizing matrix
- **Backend Dependency**: `NEXT_PUBLIC_TERMINAL_WS_URL`
- **LIVE?**: **NO** — shows VaR $1,840, heat 3.7%, drawdown 1.4% — all hardcoded
- **Operational Value CURRENT**: Zero — numbers cannot be trusted

### D-05: Terminal Trade Journal Pro
- **Route**: `/terminal/journal`
- **Component**: `client/src/components/terminal/institutional/TradeJournalPro.tsx`
- **Purpose**: Professional trade journal — setup tags, R-multiples, exit reasons, holding time
- **Backend Dependency**: `NEXT_PUBLIC_TERMINAL_WS_URL`
- **LIVE?**: **NO** — 3 hardcoded journal entries
- **Operational Value CURRENT**: Zero

### D-06: Paper Desk Dashboard
- **Route**: `/paper-desk`, `/paperdesk`
- **Component**: `client/src/components/PaperDeskDashboard.tsx` (27KB)
- **Purpose**: Live paper trading supervision — balance, equity, PnL, positions, trades, OMS orders, strategy health
- **Audience**: Primary operator monitoring autonomous paper trading
- **Backend Dependency**: MongoDB Atlas via `/api/paper-desk/snapshot` (5s poll)
- **LIVE?**: **YES** — confirmed live polling, JWT-credentialed, real MongoDB data
- **Data Dependency**: `PaperStateDoc`, `PaperPositionDoc`, `PaperTradeDoc`, `HealthSummary`
- **Operational Value**: HIGH — the most functionally complete dashboard in the application
- **Gap**: Strategy enable/disable is not actionable from this panel; OMS is view-only

### D-07: BTC Futures Trading Dashboard
- **Route**: `/btc-future-trading`
- **Component**: `client/src/components/TradingDashboard.tsx` + scalper engines
- **Purpose**: BTC futures desk — signal trace, live positions, attribution, PnL
- **Backend Dependency**: `useBTCFuturesScalperEngine` → local state + `/api/btc/*`
- **LIVE?**: **PARTIAL** — price feed is live; signal generation runs in-browser; engine state is not pulled from Go engine
- **Operational Value**: MEDIUM — signals visible, strategy diagnostics visible, but no real Go engine order data

### D-08: Mock Trading Dashboard
- **Route**: `/mock-trading`
- **Component**: `client/src/components/MockTradingDashboard.tsx` (81KB)
- **Purpose**: Full research/simulation environment — strategy leaderboard, PnL analysis, Monte Carlo, R-multiples
- **Backend Dependency**: `useMockTradingEngine` (in-browser simulation) + 15s persist to `/api/mock-trading/*`
- **LIVE?**: Simulation only
- **Operational Value**: HIGH for research; zero for production supervision

### D-09: DeskCommandCenter
- **Component**: `client/src/components/DeskCommandCenter.tsx` (41KB)
- **Route**: Unclear — not found in page routing tree
- **Status**: **ORPHANED COMPONENT** — defined but not routed
- **Operational Value**: Cannot assess — not user-accessible

### D-10: DeskMonitorPanel
- **Component**: `client/src/components/DeskMonitorPanel.tsx` (30KB)
- **Status**: Unclear routing — likely embedded in paper desk or accessible via a sub-panel
- **Operational Value**: Unclear

### D-11: InstitutionalRiskCenter
- **Component**: `client/src/components/InstitutionalRiskCenter.tsx`
- **Status**: **DEAD CODE** — not imported in any page or panel
- **Evidence**: Grep for `InstitutionalRiskCenter` in `client/src/app/` returns zero results
- **Operational Value**: Zero — unreachable by user

### D-12: GoLiveGatesPanel
- **Component**: `client/src/components/GoLiveGatesPanel.tsx`
- **Purpose**: Pre-live validation gates
- **Status**: Component exists; routing unclear; likely embedded in mock-trading flow
- **Operational Value**: Research/validation only

---

## DASHBOARD COVERAGE MATRIX

| Operational Need | Dashboard | Coverage |
|-----------------|-----------|----------|
| Live BTC price | D-01 (mock), D-07 (live) | PARTIAL |
| Live positions | D-01 (mock), D-06 (paper, live) | PARTIAL |
| Real-time risk (VaR, heat) | D-04 (mock) | FAIL |
| Kill switch status | None | **ZERO** |
| Reconciliation status | None | **ZERO** |
| Data feed health | None | **ZERO** |
| Broker connectivity | None | **ZERO** |
| Strategy enable/disable | None | **ZERO** |
| OMS order lifecycle | D-06 (paper, view-only) | PARTIAL |
| Alert management | D-01 (mock alerts) | FAIL |
| Autonomous trading supervision | D-06 (partial) | PARTIAL |

---

## VERDICT

- **4 of 11 dashboards are live** (D-06, D-07 partial, D-08 simulation, D-09 unknown)
- **5 terminal dashboards display hardcoded mock data** (D-01 through D-05)
- **1 dashboard is dead code** (D-11)
- **1 dashboard is orphaned** (D-09)
- The most critical operational gaps (kill switch, reconciliation, data feed, broker status) have **zero dashboard coverage**

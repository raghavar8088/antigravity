# LIVE DATA AUTHORITY REPORT — ICCF-LDAP Phase 1

**Audit date:** 2026-06-11  
**Method:** Source-code trace only (no documentation trust)  
**Scope:** `/terminal/*` pages

---

## Executive Summary

| Route | Authority Guard | Primary Store | Backend APIs | Verdict |
|-------|----------------|---------------|--------------|---------|
| `/terminal/execution` | YES | `useTerminalSnapshot` | snapshot, strategy-intel, equity, btc/price | **PASS with gaps** |
| `/terminal/risk` | YES | `useTerminalSnapshot` + local fetch | above + correlation-matrix, reconciliation | **PASS with gaps** |
| `/terminal/research` | YES | `useTerminalSnapshot` + local fetch | above + strategy-intelligence | **FAIL** (mislabeled Sharpe) |
| `/terminal/strategies` | **NO** | `StrategyIntelligenceDashboard` local state | `/api/strategy-intelligence` | **PASS** (own error handling) |
| `/terminal/analytics` | YES | `useTerminalSnapshot` + local fetch | above + regime-analysis | **PASS with gaps** |
| `/terminal/portfolio` | **NO** | `PortfolioAnalyticsDashboard` local state | `/api/paper-desk/portfolio`, equity | **PASS** (own error handling) |
| `/terminal/events` | **NO** | `EventCenter` local state | `/api/event-center` | **PASS with gaps** |
| `/terminal/journal` | YES | `useTerminalSnapshot` | snapshot (recent_trades) | **PASS** |

**Global shell (`TerminalShell`):** Header metrics render **without** `TerminalAuthorityGuard` — **FAIL**.

---

## Shared Data Pipeline

```
MongoDB / Go engine
  → /api/paper-desk/snapshot (aggregated)
  → /api/strategy-intelligence
  → /api/paper-desk/equity
  → /api/btc/price
  → mapSnapshotToTerminalDelta() [client/src/lib/terminal/mapSnapshotToTerminalDelta.ts]
  → useTerminalSnapshot() reducer [client/src/lib/terminal/terminalStore.tsx]
  → page components
```

**WebSocket path (optional):** `NEXT_PUBLIC_TERMINAL_WS_URL` → `WebSocket.onmessage` → `WS_DELTA` → same merge path (`terminalStore.tsx:136-165`).

**REST fallback:** Poll every 3s when WS disconnected, 5s when connected (`terminalStore.tsx:86-88, 167-220`).

---

## Page-by-Page Metric Trace

### `/terminal/execution` — `ExecutionCenter.tsx`

| UI Metric | Store Field | API / Source | Refresh | Lines |
|-----------|-------------|--------------|---------|-------|
| BTC mark price | `snapshot.price` | `/api/btc/price` via `fetchRestAuthority` | 3–5s REST | `mapSnapshotToTerminalDelta.ts:219,225`; `terminalStore.tsx:96,122` |
| Order book | `snapshot.bids/asks` | WS delta only (not in REST mapper) | WS | `ExecutionCenter.tsx:16-24`; mapper never sets bids/asks |
| Open positions | `snapshot.positions` | `snapshot.open_positions` from Mongo | 3–5s | `mapSnapshotToTerminalDelta.ts:63-79,232`; `snapshot/route.ts:34-35` |
| Portfolio summary (Net/Gross/Margin/Heat) | `snapshot.risk.*` | `portfolio.exposure`, `state` | 3–5s | `mapSnapshotToTerminalDelta.ts:116-144` |
| Alert tape | `snapshot.alerts` | `health_summary`, `state.current_drawdown` | 3–5s | `mapSnapshotToTerminalDelta.ts:170-207` |
| Candles | `snapshot.candles` | Not populated in REST path | — | Always empty unless WS sends candles |

**Reachability:** `execution/page.tsx:7-12` → `TerminalAuthorityGuard` → `ExecutionCenter`.

**Gap:** WS `WS_OPEN` sets `hasAuthority: true` before first frame (`terminalStore.tsx:48-56`) — zero-filled snapshot may render briefly.

---

### `/terminal/risk` — `RiskModule.tsx`

| UI Metric | Source | API | Lines |
|-----------|--------|-----|-------|
| VaR 95/99, CVaR | `snapshot.risk.var*` | `portfolio.var_*` from accounting snapshot | `mapSnapshotToTerminalDelta.ts:132-134`; often 0 if not computed |
| Drawdown, heat, exposure | `snapshot.risk` | Mongo state + portfolio | `RiskModule.tsx:31-57` |
| Correlation matrix | Local `useState` | `/api/paper-trades/correlation-matrix` | `RiskModule.tsx:18-29`; `correlation-matrix/route.ts:17-30` |
| Reconciliation status | Local `useState` | `/api/engine/reconciliation` | `RiskModule.tsx:18-26`; `reconciliation/route.ts:17-86` |
| Reconciliation history | Local fetch | Same API | `RiskModule.tsx:117-127` |

**Missing from Risk page (Phase 9 targets):** Kill switch panel, OMS health, market data feed, watchdog — only in global `RiskRibbon` (`layout.tsx:41`).

---

### `/terminal/research` — `ResearchCenter.tsx`

| UI Metric | Source | Authority | Lines | Verdict |
|-----------|--------|-----------|-------|---------|
| Strategy name, expectancy, PF, evidence | `snapshot.strategies` | `/api/strategy-intelligence` → Mongo `strategy_scores` | `ResearchCenter.tsx:18-69`; `strategy-intelligence/route.ts:26-90` | PASS |
| **Sharpe column** | `strategy.sharpe` | **`evidence_score / 50`** (derived, not Sharpe) | `mapSnapshotToTerminalDelta.ts:108` | **FAIL — synthetic label** |
| Pool summary | Local fetch | `/api/strategy-intelligence?view=all&limit=600` | `ResearchCenter.tsx:21-30` | PASS |
| Retirement preview | Local fetch | `/api/strategy-intelligence?view=retirement` | `ResearchCenter.tsx:96-105` | PASS |

---

### `/terminal/strategies` — `StrategyIntelligenceDashboard.tsx`

| UI Metric | API Field | Mongo Source | Lines |
|-----------|-----------|--------------|-------|
| Strategy, status, tier, PnL, expectancy, PF, win%, DD, trades, evidence | `strategies[]` | `listStrategyScores` + `listStrategyHealth` | `strategy-intelligence/route.ts:26-90`; dashboard `271-329` |
| Summary ribbon (healthy/warning/critical) | `summary` | `getStrategyHealthSummary` | `route.ts:29`; dashboard `154-171` |
| Portfolio PF in ribbon | `portfolio_stats.profit_factor` | **Hardcoded `null`** | `strategy-intelligence/route.ts:123` | **FAIL — displays "0.00"** |

Refresh: 30s poll (`StrategyIntelligenceDashboard.tsx:111-114`).

No `TerminalAuthorityGuard`; shows explicit error state on API failure (`220-224`).

---

### `/terminal/analytics` — `AnalyticsCenter.tsx`

| UI Metric | Source | API | Lines |
|-----------|--------|-----|-------|
| Equity curve | `snapshot.analytics.equityCurve` | `/api/paper-desk/equity` | `mapSnapshotToTerminalDelta.ts:152-157`; `AnalyticsCenter.tsx:10-36` |
| Sharpe 30D / 90D | Both from `portfolio.sharpe` | Same single field duplicated | `mapSnapshotToTerminalDelta.ts:161-162` — **mislabeled 90D** |
| PF trend, win rate, fee drag | `snapshot.analytics` | portfolio + state | `mapSnapshotToTerminalDelta.ts:163-165` |
| R-multiple buckets | Always `[]` | Not implemented | `mapSnapshotToTerminalDelta.ts:166` |
| Regime breakdown | Local fetch | `/api/paper-trades/regime-analysis` | `AnalyticsCenter.tsx:72-99` |

---

### `/terminal/portfolio` — `PortfolioAnalyticsDashboard.tsx`

| UI Metric | API | Mongo Authority | Lines |
|-----------|-----|-----------------|-------|
| Balance, equity, PnL, drawdowns, PF, Sharpe, Sortino, exposure | `/api/paper-desk/portfolio` | `getPortfolioAccountingSnapshot()` | `portfolio/route.ts:17`; `portfolioAccountingService.ts:163-208` |
| Equity sparkline | `/api/paper-desk/equity` | `getEquityCurve` | `PortfolioAnalyticsDashboard.tsx:96-112` |

Full extended metrics only when `closedStats.total_trades > 0` (`portfolioAccountingService.ts:195-206`).

Error path: explicit `BACKEND AUTHORITY UNAVAILABLE` (`137-144`).

---

### `/terminal/events` — `EventCenter.tsx`

| UI Metric | API | Builder | Lines |
|-----------|-----|---------|-------|
| Event list | `/api/event-center` | `buildPlatformEvents()` | `EventCenter.tsx:65-70`; `event-center/route.ts:15`; `platformEvents.ts:34-132` |

Poll 3s. Does **not** consume `/api/engine/events` SSE.

---

## Shell Header (Unguarded)

`TerminalShell.tsx:21-94` renders price, spread, funding, regime, heat, exposure from `useTerminalSnapshot()` **without** authority guard.

- Price shows `—` when `price === 0` (line 46) — OK during load.
- Spread/funding show `—` when 0 (lines 52-53) — but mapper **hardcodes 0** (`mapSnapshotToTerminalDelta.ts:226-227`), so always `—` on REST path.
- Exposure label uses flawed logic (lines 26-30): displays gross exposure, not equity.

---

## Certification Result — Phase 1

**NOT CERTIFIED.** Multiple metrics are mislabeled, derived, zero-defaulted, or rendered before authority payload arrives (WS path).

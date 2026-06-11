# UI FORENSIC AUDIT
**Phase 8 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code review of component files in `client/src/`

---

## SCORING SYSTEM

Each screen scored on 5 axes:
- **Institutional Quality (IQ):** Does this screen show what a fund PM would expect? 0–20
- **Information Density (ID):** Data-to-noise ratio — actual metrics vs decorative elements 0–20
- **Data Fidelity (DF):** Is the data live, accurate, sourced from the engine? 0–20
- **UX/Navigation (UX):** Is it immediately legible? Does layout support action? 0–20
- **Completeness (CP):** Coverage of the full trading lifecycle 0–20

Max per screen: 100

---

## SCREEN 1: `PaperDeskDashboard`

**File:** `client/src/components/PaperDeskDashboard.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 11/20 | Shows P&L, positions, equity curve. No Sharpe, no drawdown, no vol-of-vol, no regime annotation |
| Information Density | 12/20 | Reasonable density — equity chart, trade list, position cards. But no slippage panel, no attribution |
| Data Fidelity | 9/20 | MongoDB-backed, 15s stale. No real-time price refresh on positions |
| UX/Navigation | 13/20 | Clear layout, good typography. No drill-down on individual trades |
| Completeness | 10/20 | Missing: slippage, fee attribution, win rate by regime, alpha decay, signal-to-fill latency |
| **TOTAL** | **55/100** | Functional but not institutional |

**Critical gaps:**
- No Sharpe ratio displayed
- No max drawdown displayed
- No win rate by strategy family
- No regime context on trades
- Equity chart shows equity but no benchmark comparison (BTC buy-and-hold, etc.)
- Balance shown from MongoDB snapshot, not from live engine

---

## SCREEN 2: `BTCFuturesScalper`

**File:** `client/src/components/BTCFuturesScalper.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 0/20 | Shows nothing (poll disabled) |
| Information Density | 0/20 | All panels empty |
| Data Fidelity | 0/20 | Hook disabled — zero data flow |
| UX/Navigation | 5/20 | Layout exists, no broken errors shown to user |
| Completeness | 0/20 | Complete data void |
| **TOTAL** | **5/100** | Non-functional screen |

**Issue:** The screen exists and loads in the UI, but shows empty/default data because the underlying hook is disabled. A user navigating to this screen would see an empty dashboard with no indication that it is intentionally disabled.

**Required action:** Replace with either:
1. A read-only view powered by `/api/paper-desk/snapshot`
2. A redirect to PaperDeskDashboard
3. An explicit "Deprecated — View Paper Desk" banner

---

## SCREEN 3: Terminal Suite — ExecutionCenter

**File:** `client/src/components/terminal/ExecutionCenter.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 15/20 | Shows live positions, open orders, fill history. OMS data present. |
| Information Density | 14/20 | Dense. Some panels could consolidate |
| Data Fidelity | 17/20 | Reads from live engine via proxy. 2s polling. Data is real. |
| UX/Navigation | 14/20 | Terminal aesthetic is clean. Tab navigation clear. |
| Completeness | 12/20 | Missing: fill quality (slippage in bps), fill-to-intended-price deviation, trade timing |
| **TOTAL** | **72/100** | Near-institutional. Minor gaps. |

---

## SCREEN 4: Terminal Suite — AnalyticsCenter

**File:** `client/src/components/terminal/AnalyticsCenter.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 14/20 | Has equity curve, trade distribution charts, some stat breakdown |
| Information Density | 13/20 | Good but some charts are decorative rather than actionable |
| Data Fidelity | 16/20 | Engine API backed — live |
| UX/Navigation | 13/20 | Navigation within analytics is somewhat flat |
| Completeness | 10/20 | Missing: Sharpe/Sortino/Calmar by strategy, rolling-window statistics, attribution by regime, correlation matrix |
| **TOTAL** | **66/100** | Good foundation, needs statistical depth |

---

## SCREEN 5: Terminal Suite — RiskModule

**File:** `client/src/components/terminal/RiskModule.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 16/20 | Kill switch panel, risk utilization bars, drawdown indicators present |
| Information Density | 15/20 | Dense risk display |
| Data Fidelity | 17/20 | Live engine risk API |
| UX/Navigation | 15/20 | Clear layout for risk monitoring |
| Completeness | 12/20 | Missing: real-time VaR, Kelly utilization %, strategy family heat map |
| **TOTAL** | **75/100** | Best screen in the suite |

---

## SCREEN 6: Terminal Suite — ResearchCenter

**File:** `client/src/components/terminal/ResearchCenter.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 13/20 | Strategy browser, performance by strategy |
| Information Density | 12/20 | Reasonable but some pages are sparse |
| Data Fidelity | 14/20 | Engine-backed strategy data |
| UX/Navigation | 12/20 | Could benefit from filtering by family/regime |
| Completeness | 11/20 | Missing: expectancy per strategy, profit factor, regime breakdown |
| **TOTAL** | **62/100** | Useful but research features are shallow |

---

## SCREEN 7: Strategy Leaderboard

**File:** `client/src/components/StrategyLeaderboard.tsx`

| Axis | Score | Notes |
|------|-------|-------|
| Institutional Quality | 15/20 | Win rate, PnL, trade count shown. Sort/filter available. |
| Information Density | 15/20 | Tabular — high density |
| Data Fidelity | 14/20 | MongoDB-backed with reasonable freshness |
| UX/Navigation | 14/20 | Good sort/filter. Could use heatmap coloring |
| Completeness | 12/20 | Missing: Sharpe per strategy, expectancy, profit factor, max drawdown per strategy |
| **TOTAL** | **70/100** | Strong screen, needs depth metrics |

---

## SCREEN SCORES SUMMARY

| Screen | Score | Grade |
|--------|-------|-------|
| PaperDeskDashboard | 55/100 | C+ |
| BTCFuturesScalper | 5/100 | F |
| Terminal ExecutionCenter | 72/100 | B |
| Terminal AnalyticsCenter | 66/100 | B- |
| Terminal RiskModule | 75/100 | B+ |
| Terminal ResearchCenter | 62/100 | B- |
| Strategy Leaderboard | 70/100 | B |
| **Composite Average** | **58/100** | **C+** |

---

## INSTITUTIONAL BENCHMARK

A professional algo trading dashboard (Bloomberg Terminal, Kensho, Two Sigma internal tooling) would score:
- RiskModule level (75+): **all screens**
- Strategy attribution to regime and market conditions: **every panel**
- Rolling Sharpe/Calmar on the equity chart: **standard**
- Real-time data (< 1s): **everywhere**
- Kill switch status permanently visible: **top nav**

This platform scores 58/100 composite. The highest-value improvements are:
1. Fix `BTCFuturesScalper` (broken screen) → immediate confidence gain
2. Add Sharpe/Sortino/drawdown to `PaperDeskDashboard` → biggest PM-visible gap
3. Add regime attribution to trade detail → institutional differentiation
4. Replace hardcoded balance in `useEngineState.ts` → data integrity

---

## FAKE/MISLEADING DATA CATALOG

| Screen | Element | Issue |
|--------|---------|-------|
| Any screen using `useEngineState` | Balance display | HARDCODED `1000000.0` — never updates |
| BTCFuturesScalper | All panels | Shows empty/default — no data flowing |
| PaperDeskDashboard PnL | Unrealized PnL | Up to 15s stale |
| Any chart | Equity curve | 1-min resolution — misses intraday max drawdown |

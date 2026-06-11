# PHASES 17–20 — UX REDESIGN, IMPLEMENTATION ROADMAP, ARCHITECTURE TRANSFORMATION, FINAL CERTIFICATION
## Institutional Trading Command Center Reconstruction Program (ITCCRP)
**Date:** 2026-06-11

---

# PHASE 17 — INSTITUTIONAL UX REDESIGN BLUEPRINT

## Design Principles

1. **Information Density First** — every pixel carries operational data
2. **Color Means Something** — GREEN/AMBER/RED have consistent semantic meaning system-wide
3. **No Dead Space** — no padding wasted on branding; operators don't need logos
4. **Hierarchy is Time-Critical** — most important data largest and highest
5. **State is Always Visible** — connection status, data freshness, system health always on screen

## Target Layout Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  RISK COMMAND RIBBON (48px fixed, always visible)               │
│  KILL SW: ● ARMED  │ RECON: ✓ OK  │ DD: 1.2%  │ EXP: $12,400  │
├─────────────────────────────────────────────────────────────────┤
│  NAV BAR (36px)                                                 │
│  [Overview] [Execution] [Portfolio] [Strategies] [Risk] [Logs]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  PAGE CONTENT (fluid, responsive)                               │
│                                                                 │
│  LEFT PANEL (280px)    │  CENTER (1fr)      │  RIGHT (320px)   │
│  ─────────────────     │  ─────────────     │  ────────────    │
│  Account KPIs          │  Main Chart        │  Open Positions  │
│  System Health         │  or Table          │  Order Book DOM  │
│  Quick Stats           │                    │  Recent Fills    │
│                        │                    │                  │
├─────────────────────────────────────────────────────────────────┤
│  STATUS BAR (24px)                                              │
│  BTC: $105,240  ●LIVE  │  Last Update: 2s ago  │  Engine: ● OK │
└─────────────────────────────────────────────────────────────────┘
```

## Page-by-Page Redesign

### Overview Page (`/`)

**Target:** Bloomberg command-center style main overview

```
┌──────────────────────────────────────────────────────────┐
│  OVERVIEW                                    ● LIVE       │
├──────────────────────────────────────────────────────────┤
│  [Equity $1.024M] [PnL +$24,200] [DD 1.2%] [Pos 7] [WR 58%] [PF 1.42] [Sharpe 1.84] [BTC $105,240]
├──────────────┬───────────────────────┬───────────────────┤
│ STRATEGY     │ SIGNAL FEED           │ SYSTEM            │
│ LEADERBOARD  │ (live OMS events)     │ STATUS            │
│              │                       │                   │
│ 1. Funding.. │ 14:22 OMS ACCEPTED    │ Engine    ● OK    │
│    +$8,240   │ Funding MR LONG       │ MongoDB   ● OK    │
│              │ 14:21 POSITION OPENED │ BTC Feed  ● LIVE  │
│ 2. CVD Div.. │ $105,180 → 0.18 BTC   │ Watchdog  ● OK   │
│    +$5,100   │                       │ Recon     ✓ OK    │
│              │ 14:19 POSITION CLOSED │                   │
│ 3. MSS..     │ CVD Div WIN +$124     │ Regime: TRENDING  │
│    +$3,890   │ Exit: TAKE_PROFIT     │ Strategies: 347   │
│              │                       │ Active: 312       │
├──────────────┴───────────────────────┴───────────────────┤
│  OPEN POSITIONS (7)                          Unrealized: +$1,240
│  Strategy          Side  Entry      Mark      PnL       Age
│  Funding MR Alpha  LONG  $104,920   $105,240  +$58.40   12m
│  CVD Div Alpha     SHORT $106,210   $105,240  +$42.20   8m
│  MSS Cont Alpha    LONG  $105,100   $105,240  +$25.20   5m
└──────────────────────────────────────────────────────────┘
```

### Execution Page (`/terminal/execution`)

**Target:** Real BTC chart + real order book + real positions + real alerts

```
┌────────────────────────────────────────────────────────────────┐
│  EXECUTION CENTER                          ● LIVE  BTC $105,240 │
├──────────────┬─────────────────────────────┬───────────────────┤
│ ORDER BOOK   │  BTCUSD PERP CHART          │ POSITIONS          │
│ LIVE DOM     │  (from Binance klines)      │ (from paper_pos)  │
│              │                             │                   │
│ ASK          │  [Real 1m candles]          │ LONG 0.18 BTC     │
│ 105,262 2.1  │                             │ Entry: 104,920    │
│ 105,258 1.8  │                             │ PnL: +$58.40      │
│ 105,254 3.2  │                             │                   │
│ ─────────────│                             │ SHORT 0.11 BTC    │
│ 105,240 MARK │                             │ Entry: 106,210    │
│ ─────────────│                             │ PnL: +$42.20      │
│ BID          │                             │                   │
│ 105,236 2.4  │                             │ [+ 5 more...]     │
│ 105,232 1.9  │                             │                   │
│              ├─────────────────────────────┤                   │
│              │ ALERT TAPE                  │                   │
│              │ (from alerts collection)    │                   │
└──────────────┴─────────────────────────────┴───────────────────┘
```

### Portfolio Analytics (`/terminal/analytics`)

```
┌──────────────────────────────────────────────────────────────┐
│  PORTFOLIO ANALYTICS                        ● LIVE            │
├────────────────────┬─────────────────────────────────────────┤
│ PORTFOLIO KPIs     │  EQUITY CURVE (from equity_curve coll)  │
│                    │  [Real chart: equity vs BTC benchmark]  │
│ Equity  $1.024M    │                                         │
│ PnL     +$24,200   │                                         │
│ Sharpe  1.84       │                                         │
│ Sortino 2.12       │                                         │
│ PF      1.42       ├─────────────────────────────────────────┤
│ Max DD  3.4%       │  DAILY PnL BARS                         │
│ Win%    58.2%      │  [Real bars from daily_pnl_history]     │
│ Fees    $2,140     │                                         │
├────────────────────┴─────────────────────────────────────────┤
│  R-MULTIPLE DISTRIBUTION    │  CAPITAL ALLOCATION            │
│  [Real histogram from       │  [Pie: Long / Short / Cash     │
│   paper_trades.net_pnl]     │   from paper_positions]        │
└─────────────────────────────┴──────────────────────────────┘
```

### Strategy Intelligence (`/terminal/research`)

```
┌──────────────────────────────────────────────────────────────┐
│  STRATEGY INTELLIGENCE                       600+ strategies  │
├────────────────────────────────────────────────────────────┤
│  [Search: ___________] [Family: ALL▾] [Status: ALL▾] [Sort: PnL▾]
├────────────────────────────────────────────────────────────┤
│ # │ Strategy Name          │ Family       │ Status  │ Trades │ PnL      │ WR   │ PF   │ Exp  │ Score
│───┼───────────────────────┼──────────────┼─────────┼────────┼──────────┼──────┼──────┼──────┼──────
│ 1 │ Funding MR Alpha v2   │ Funding      │ ACTIVE  │ 124    │ +$8,240  │ 64%  │ 1.92 │ $66  │ 91
│ 2 │ CVD Div Alpha          │ Order Flow   │ ACTIVE  │ 98     │ +$5,100  │ 58%  │ 1.64 │ $52  │ 87
│ 3 │ MSS Cont Alpha         │ MSS          │ ACTIVE  │ 87     │ +$3,890  │ 55%  │ 1.48 │ $45  │ 82
│ . │ ...                    │ ...          │ ...     │ ...    │ ...      │ ...  │ ...  │ ...  │ ...
│(Real data from strategy_scores + strategy_health collections)
└──────────────────────────────────────────────────────────────┘
```

### Risk Command (`/terminal/risk`)

```
┌──────────────────────────────────────────────────────────────┐
│  RISK COMMAND CENTER                         ● CRITICAL WATCH │
├──────────────┬──────────────────┬──────────────────────────┤
│ RISK METRICS │ DRAWDOWN CHART   │ EXPOSURE BREAKDOWN       │
│              │ (real from       │ (real from positions)    │
│ Daily DD: 1.2%│  equity_curve)  │                          │
│ Max DD:  3.4% │                 │ Long:  $19,052           │
│ Net Exp: $7.2K│                 │ Short: $11,668           │
│ Gross: $30.7K │                 │ Net:   +$7,384           │
│ Margin:  18.6%│                 │                          │
│               │                 │ Heat:  3.7%              │
│ Kill SW:  ✓   │                 │ Margin: 18.6%            │
│ Recon:    ✓   │                 │                          │
└──────────────┴──────────────────┴──────────────────────────┘
```

## Color System Specification

```css
/* Risk status colors — used consistently across ALL components */
--status-green: #26a69a;   /* OK / Healthy / Profit */
--status-amber: #ffb74d;   /* Warning / Monitor / Borderline */
--status-red:   #ef5350;   /* Critical / Error / Loss */
--status-blue:  #42a5f5;   /* Info / Neutral / Unknown */

/* Rules:
   1. GREEN: system online, drawdown < 2%, margin < 50%, fill quality > 80%
   2. AMBER: drawdown 2-5%, margin 50-80%, fill quality 60-80%, data > 30s old
   3. RED:   drawdown > 5%, margin > 80%, fill quality < 60%, engine offline, kill switch triggered
   4. Never use GREEN for a metric that is unknown or unverified
   5. Show "—" (dash) for metrics with no backend source; never show fake numbers
*/
```

---

# PHASE 18 — IMPLEMENTATION ROADMAP

## Priority Matrix

| Phase | Task | Effort | Risk | ROI | Sprint |
|-------|------|--------|------|-----|--------|
| P0 | Zero out `initialTerminalSnapshot` | 0.5h | LOW | CRITICAL | 1 |
| P0 | Fix `useEngineState` balance | 1h | LOW | HIGH | 1 |
| P0 | Add "No Data" guards to terminal pages | 2h | LOW | HIGH | 1 |
| P1 | Wire REST polling fallback to `terminalStore.tsx` | 4h | MED | HIGH | 1 |
| P1 | Map terminal positions → `paper_positions` | 3h | MED | HIGH | 1 |
| P1 | Map terminal risk metrics → `paper_state` | 3h | MED | HIGH | 1 |
| P1 | Wire equity curve to `AnalyticsCenter` | 2h | LOW | HIGH | 1 |
| P1 | Wire trade journal to `TradeJournalPro` | 2h | LOW | HIGH | 1 |
| P1 | Build `RiskCommandRibbon` component | 4h | MED | HIGH | 2 |
| P1 | Build `/api/risk-ribbon` aggregated endpoint | 3h | MED | HIGH | 2 |
| P2 | Build `StrategyIntelligenceCenter` | 8h | MED | HIGH | 2 |
| P2 | Build `/api/paper-desk/strategy-intelligence` | 4h | LOW | HIGH | 2 |
| P2 | Promote `usePaperDesk` to Context/Zustand | 4h | MED | MED | 2 |
| P2 | Fix `TerminalDashboard` re-render optimization | 3h | LOW | MED | 2 |
| P2 | Build `PortfolioAnalyticsCenter` | 8h | MED | HIGH | 3 |
| P2 | Build `EventConsole` (real-time event stream) | 6h | MED | HIGH | 3 |
| P2 | Build `ExecutionQualityCenter` | 6h | MED | MED | 3 |
| P3 | Build `AlphaMonitoringCenter` | 8h | LOW | MED | 4 |
| P3 | Build `HealthCenter` with 9-system grid | 6h | MED | MED | 4 |
| P3 | Add `CorrelationAnalyticsCenter` | 10h | HIGH | MED | 4 |
| P3 | Implement Go Engine WebSocket for terminal | 16h | HIGH | HIGH | 5 |
| P3 | Build `/api/sep/rankings` SEP integration | 6h | MED | MED | 5 |
| P4 | Full UX redesign (institutional density layout) | 24h | HIGH | HIGH | 5-6 |
| P4 | Zustand migration for global state | 12h | HIGH | MED | 6 |

## Sprint 1 (Week 1) — Emergency Hardcoded Value Elimination
**Goal:** No page displays fake data. Estimated: 2 days.

Tasks:
1. `terminalSnapshot.ts` — zero out initialTerminalSnapshot (HV-002)
2. `useEngineState.ts` — fix balance to fetch from API (HV-001)
3. `ExecutionCenter.tsx` — add null/loading state guard
4. `AnalyticsCenter.tsx` — add null/loading state guard
5. `ResearchCenter.tsx` — add null/loading state guard
6. `RiskModule.tsx` — add null/loading state guard
7. `TradeJournalPro.tsx` — add null/loading state guard
8. `terminalStore.tsx` — add REST polling fallback

**Definition of Done:** No terminal page renders hardcoded BTC price, position, PnL, or strategy data.

## Sprint 2 (Week 2) — Live Data Wiring for Terminal Pages
**Goal:** All 5 terminal pages display real MongoDB data. Estimated: 3 days.

Tasks:
1. `ExecutionCenter` → `usePaperDesk` + `useLiveBTCPrice` for positions, order summary
2. `AnalyticsCenter` → `/api/paper-desk/equity` for equity curve + win rate
3. `ResearchCenter` → `/api/paper-desk/strategy-intelligence` for strategy table
4. `RiskModule` → `paper_state` + portfolio accounting for risk metrics
5. `TradeJournalPro` → `/api/paper-desk/trades` for journal entries
6. `RiskCommandRibbon` component (Phase 9) built and wired
7. `/api/risk-ribbon` aggregated endpoint built

**Definition of Done:** All 5 terminal pages show real data. Risk ribbon visible on all pages.

## Sprint 3 (Week 3) — Strategy Intelligence + Portfolio Analytics
**Goal:** Operators can see all 600+ strategies ranked. Estimated: 3 days.

Tasks:
1. `/api/paper-desk/strategy-intelligence` route
2. `StrategyIntelligenceCenter` component (table, filter, sort)
3. Strategy name mapping (strategy_id → FUTURES_STRAT_DEFS name)
4. `/api/paper-desk/portfolio-analytics` route
5. `PortfolioAnalyticsCenter` component
6. `EventConsole` component + `/api/paper-desk/events` route

**Definition of Done:** Strategy leaderboard shows all 600 strategies with real metrics. Portfolio analytics page is functional.

## Sprint 4 (Week 4) — Observability + Health + Alpha
**Goal:** Operators can monitor system health and alpha engine status. Estimated: 3 days.

Tasks:
1. `HealthCenter` component (9-system health grid)
2. `/api/system-health` aggregated endpoint
3. `AlphaMonitoringCenter` component (alpha family cards)
4. `ExecutionQualityCenter` component + fill funnel
5. Performance optimizations (P1-P4 from Phase 16)

**Definition of Done:** Health center shows live status for all 9 systems. Alpha monitoring shows signal counts per family.

## Sprint 5+ (Weeks 5-8) — UX Redesign + WebSocket + SEP
- Institutional density layout redesign
- Go Engine WebSocket endpoint for terminal
- SEP integration and ranking dashboard
- Correlation analytics center
- Zustand store migration

---

# PHASE 19 — ARCHITECTURE TRANSFORMATION REPORT

## Current Architecture (As-Built, 2026-06-11)

```
┌──────────────────────────────────────────────────────────────┐
│                    CURRENT ARCHITECTURE                       │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Go Engine (AWS Lightsail)                                  │
│  ├── 600+ strategies                                        │
│  ├── OMS v3                                                 │
│  ├── Risk Gate                                              │
│  ├── Kill Switch                                            │
│  ├── Reconciliation                                         │
│  └── Writes to MongoDB Atlas:                               │
│      paper_state, paper_trades, paper_positions,            │
│      paper_orders, equity_curve, strategy_scores,           │
│      strategy_health                                        │
│                                                              │
│  MongoDB Atlas ◄──────────────────────────────             │
│  (loop_trades database)                                      │
│                                                              │
│  Next.js API Routes (Vercel)                                │
│  ├── /api/paper-desk/* ──────────────────────► UI (good)  │
│  └── /api/engine/[...path] ─────────────────► Go Engine   │
│                                                              │
│  Client UI                                                  │
│  ├── / (root) ──────── usePaperDesk ──────► MongoDB ✅     │
│  ├── /paper-desk ────── usePaperDesk ──────► MongoDB ✅     │
│  ├── /terminal/execution ── useTerminalSnapshot ──► HARDCODED ❌
│  ├── /terminal/analytics ── useTerminalSnapshot ──► HARDCODED ❌
│  ├── /terminal/research ─── useTerminalSnapshot ──► HARDCODED ❌
│  ├── /terminal/risk ──────── useTerminalSnapshot ──► HARDCODED ❌
│  └── /terminal/journal ───── useTerminalSnapshot ──► HARDCODED ❌
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Target Architecture (Post-ITCCRP)

```
┌──────────────────────────────────────────────────────────────┐
│                    TARGET ARCHITECTURE                       │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Go Engine (AWS Lightsail) — UNCHANGED                      │
│  ├── 600+ strategies                                        │
│  ├── OMS v3 / Risk Gate / Kill Switch / Reconciliation      │
│  ├── Writes 8 MongoDB collections (existing)                │
│  ├── NEW: /risk/ribbon endpoint (aggregated status)         │
│  └── NEW: WebSocket terminal feed (future, Sprint 5+)       │
│                                                              │
│  MongoDB Atlas — UNCHANGED (7 existing + 1 new)             │
│  └── NEW: alerts collection (Go Engine writes critical events)│
│                                                              │
│  Next.js API Routes — EXPANDED                              │
│  ├── /api/paper-desk/* (existing 13 routes)                 │
│  ├── /api/paper-desk/strategy-intelligence (NEW)            │
│  ├── /api/paper-desk/portfolio-analytics (NEW)              │
│  ├── /api/paper-desk/events (NEW)                           │
│  ├── /api/paper-desk/consistency-check (NEW)                │
│  ├── /api/risk-ribbon (NEW — aggregated health)             │
│  └── /api/sep/* (NEW — SEP integration)                     │
│                                                              │
│  Client State — UPGRADED                                    │
│  ├── PaperDeskContext (singleton poll, replaces N instances) │
│  ├── useRiskRibbon (10s poll)                               │
│  ├── useStrategyIntelligence (30s poll)                     │
│  ├── useEventConsole (2s poll)                              │
│  └── useTerminalSnapshot → REST fallback (10s poll) + WS    │
│                                                              │
│  Client UI — ALL REAL DATA                                  │
│  ├── / ─────────────── PaperDeskContext ──────► MongoDB ✅  │
│  ├── /paper-desk ─────── PaperDeskContext ──────► MongoDB ✅│
│  ├── /terminal/execution ─ usePaperDesk + useLiveBTCPrice ✅│
│  ├── /terminal/analytics ─ /api/paper-desk/equity ✅        │
│  ├── /terminal/research ── /api/paper-desk/strategy-intel ✅│
│  ├── /terminal/risk ─────── PaperDeskContext + portfolio ✅  │
│  ├── /terminal/journal ───── /api/paper-desk/trades ✅       │
│  ├── /terminal/strategies ── StrategyIntelligenceCenter ✅  │
│  ├── /terminal/portfolio ─── PortfolioAnalyticsCenter ✅    │
│  ├── /terminal/alpha ─────── AlphaMonitoringCenter ✅       │
│  ├── /terminal/events ─────── EventConsole ✅               │
│  └── [Global] RiskCommandRibbon (all pages) ✅              │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Data Flow Diagrams

### Current (Broken) Terminal Flow
```
MongoDB (real data) → Nothing → terminalSnapshot.ts (fake data) → Terminal Pages
```

### Target Terminal Flow
```
MongoDB (real data)
  → /api/paper-desk/snapshot (10s poll)
  → PaperDeskContext (singleton)
  → {ExecutionCenter, AnalyticsCenter, ResearchCenter, RiskModule, TradeJournalPro}
  → Displayed with real values

Coinbase WS (real-time)
  → useLiveBTCPrice
  → BTC price display + open position mark-to-market

Go Engine WebSocket (future)
  → useTerminalSnapshot (upgrade: WS preferred, REST fallback)
  → Sub-second updates for order book, price, funding rate
```

---

# PHASE 20 — FINAL CERTIFICATION

## Certification Checklist

**1. Is every displayed value backed by backend authority?**
- **Current:** ❌ NO — `/terminal/*` pages display hardcoded synthetic data
- **Post-Sprint 1:** ✅ YES — after zeroing initialTerminalSnapshot and wiring REST fallback
- **Post-Sprint 2:** ✅ YES with LIVE data on all pages

**2. Are hardcoded values eliminated?**
- **Current:** ❌ NO — 9 hardcoded value instances identified (HV-001 through HV-009)
- **Post-Sprint 1:** ✅ Sprints 1-2 eliminate all P0 and P1 hardcoded values

**3. Can displayed PnL be trusted?**
- **Root page (`/`):** ✅ YES — sourced from `paper_state.realized_pnl` + `unrealized_pnl`
- **Terminal analytics:** ❌ NO until Sprint 2
- **Post-Sprint 2:** ✅ YES — wired to `equity_curve` collection

**4. Can displayed balances be trusted?**
- **useEngineState.balance:** ❌ NO — always `$1,000,000`
- **TerminalDashboard equity:** ✅ YES — from `paper_state.equity`
- **Post-Sprint 1:** ✅ YES everywhere after HV-001 fix

**5. Can displayed positions be trusted?**
- **Root page positions table:** ✅ YES — from `paper_positions`
- **Terminal execution positions:** ❌ NO — hardcoded until Sprint 2
- **Post-Sprint 2:** ✅ YES everywhere

**6. Can operators identify losing strategies instantly?**
- **Current:** ⚠️ PARTIAL — root page leaderboard shows top 5 only
- **Post-Sprint 3:** ✅ YES — StrategyIntelligenceCenter shows all 600+, sortable by PnL

**7. Can operators identify profitable strategies instantly?**
- **Current:** ⚠️ PARTIAL — limited to top 5 in leaderboard
- **Post-Sprint 3:** ✅ YES — full sortable strategy table

**8. Can operators monitor risk in real time?**
- **Current:** ❌ NO — risk metrics are hardcoded or missing
- **Post-Sprint 2:** ✅ YES — Risk Command Ribbon + wired Risk Module

**9. Can operators detect outages immediately?**
- **Current:** ❌ NO — no health center, no outage detection beyond paper desk connection status
- **Post-Sprint 4:** ✅ YES — Health Center + Risk Ribbon shows all 9 system statuses

**10. Is the platform institutional-grade?**
- **Current:** ❌ NO — 5 broken terminal pages, hardcoded data, no health center
- **Post-Sprint 2:** ⚠️ APPROACHING — core data real, no health center yet
- **Post-Sprint 4:** ✅ YES — all data real, health monitoring operational

---

## SCORING

### Current Scores (2026-06-11)

| Dimension | Score | Key Gap |
|-----------|-------|---------|
| UI/Backend Alignment | **52/100** | 5 terminal pages 100% synthetic |
| Institutional Dashboard | **54/100** | No strategy intel, no health center |
| Observability | **55/100** | No metrics surface, no log UI, no health |
| Operator Effectiveness | **60/100** | Can't see 600 strategies, no risk ribbon |
| Production Readiness | **72/100** | Execution solid; visibility is the gap |

### Post-Sprint 2 Scores (2 weeks)

| Dimension | Score | Remaining Gap |
|-----------|-------|--------------|
| UI/Backend Alignment | **82/100** | Some advanced metrics still missing |
| Institutional Dashboard | **75/100** | Strategy intel, portfolio analytics not built yet |
| Observability | **70/100** | Health center not yet built |
| Operator Effectiveness | **78/100** | No strategy search across 600 strats yet |
| Production Readiness | **85/100** | All core data real |

### Post-Sprint 4 Scores (4 weeks)

| Dimension | Score | Remaining Gap |
|-----------|-------|--------------|
| UI/Backend Alignment | **92/100** | WS terminal feed (Sprint 5) |
| Institutional Dashboard | **90/100** | UX density redesign (Sprint 5+) |
| Observability | **90/100** | Prometheus metrics UI (Sprint 5+) |
| Operator Effectiveness | **92/100** | Full SEP integration |
| Production Readiness | **90/100** | WS endpoint in Go Engine |

---

## CERTIFICATION VERDICT (Current State)

```
VERDICT 3 — MATERIAL VISIBILITY RISKS REMAIN
```

**Reason:** Five institutional dashboard pages display hardcoded synthetic data that an operator cannot distinguish from live data unless they inspect source code. A risk event (kill switch trigger, reconciliation drift) would NOT be visible on the terminal pages. An operator watching the terminal during a drawdown event would see fake risk metrics.

### What Must Be Fixed Before VERDICT 2

1. ❌→✅ Zero out `initialTerminalSnapshot` (HV-002) — 30 minutes
2. ❌→✅ Add null-state guards to 5 terminal pages — 2 hours
3. ❌→✅ Wire terminal pages to REST APIs — 14 hours
4. ❌→✅ Fix `useEngineState.balance` (HV-001) — 1 hour
5. ❌→✅ Build `RiskCommandRibbon` — 4 hours

**Minimum effort to VERDICT 2:** ~21 hours of engineering

### What Must Be Fixed Before VERDICT 1 (CERTIFIED)

Everything above, plus:
- Strategy Intelligence Center showing all 600+ strategies
- Portfolio Analytics with real equity curve + computed Sharpe/Sortino
- Event Console showing real-time OMS events
- Health Center showing all 9 system statuses
- Risk Command Ribbon globally visible
- No component ever shows "—" for balance when MongoDB is healthy

**Total effort to CERTIFIED:** ~120 hours across Sprints 1-4

---

## TOP 10 REMEDIATION ITEMS (Ranked by ROI)

| Rank | Action | File | Effort | ROI |
|------|--------|------|--------|-----|
| 1 | Zero out hardcoded terminal snapshot | `terminalSnapshot.ts:17-121` | 30m | CRITICAL |
| 2 | Fix useEngineState balance | `useEngineState.ts:5,39` | 1h | HIGH |
| 3 | Add null guards to 5 terminal pages | `ExecutionCenter, AnalyticsCenter, ResearchCenter, RiskModule, TradeJournalPro` | 2h | HIGH |
| 4 | Wire terminal REST fallback in terminalStore | `terminalStore.tsx` | 4h | HIGH |
| 5 | Map ExecutionCenter positions → paper_positions | `ExecutionCenter.tsx` | 3h | HIGH |
| 6 | Wire AnalyticsCenter → equity_curve | `AnalyticsCenter.tsx` | 2h | HIGH |
| 7 | Wire ResearchCenter → strategy_scores | `ResearchCenter.tsx` | 2h | HIGH |
| 8 | Build Risk Command Ribbon | NEW component | 4h | HIGH |
| 9 | Fix TerminalDashboard re-render (P2 perf) | `TerminalDashboard.tsx` | 3h | MED |
| 10 | Build Strategy Intelligence Center | NEW component + route | 12h | HIGH |

---

## FINAL STATEMENT

The trading execution stack is strong. The Go Engine executes trades correctly. MongoDB stores data faithfully. The `/` root dashboard reads real data from MongoDB.

The crisis is entirely in the institutional terminal interface. Five pages that operators use for command-and-control display fake data. This is not a risk in the trading engine — it is a risk to operator decisions.

An operator watching `/terminal/risk` during a live drawdown event would see hardcoded `1.4%` drawdown while the actual drawdown could be 8%. They would not respond.

An operator watching `/terminal/execution` would see fake positions for "Funding Mean Reversion Alpha" and "CVD Divergence Alpha" that do not exist. They cannot validate execution.

This is correctable in 21 hours of focused engineering (Sprints 1-2). The Go Engine has already done the hard work. The data exists in MongoDB. It just needs to be plumbed through to the terminal.

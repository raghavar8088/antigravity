# PHASE 1 — UI ARCHITECTURE REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## EXECUTIVE SUMMARY

The UI is a Next.js 14+ App Router application deployed on Vercel. It consists of two structurally separate systems that do not share state: a **Terminal Module** (WebSocket-driven, currently serving hardcoded mock data) and a **Paper Desk / Scalper system** (HTTP polling, genuinely live). The architecture supports extensive dashboard surface area but has critical wiring gaps between backend capability and UI visibility.

---

## 1. PAGE INVENTORY & NAVIGATION GRAPH

| Route | Component | Data Source | Live? |
|-------|-----------|-------------|-------|
| `/` | `TerminalDashboard` → `AppShell` | WebSocket `NEXT_PUBLIC_TERMINAL_WS_URL` | **NO — env var unset, shows mock data** |
| `/terminal` | redirect to `/terminal/execution` | — | — |
| `/terminal/execution` | `ExecutionCenter` | `useTerminalSnapshot()` | **NO — mock data** |
| `/terminal/analytics` | `AnalyticsCenter` | `useTerminalSnapshot()` | **NO — mock data** |
| `/terminal/research` | `ResearchCenter` | `useTerminalSnapshot()` | **NO — mock data** |
| `/terminal/risk` | `RiskModule` | `useTerminalSnapshot()` | **NO — mock data** |
| `/terminal/journal` | `TradeJournalPro` | `useTerminalSnapshot()` | **NO — mock data** |
| `/paper-desk` | `PaperDeskDashboard` | `usePaperDesk()` → MongoDB 5s poll | **YES — live** |
| `/paperdesk` | same | same | **YES — live** |
| `/btc-future-trading` | `TradingDashboard` | `useBTCFuturesScalperEngine()` | **YES — live signals** |
| `/mock-trading` | `MockTradingDashboard` | `useMockTradingEngine()` | **YES — simulated** |
| `/login`, `/sign-in` | login forms | `/api/auth/*` | YES |

**CRITICAL FINDING:** `NEXT_PUBLIC_TERMINAL_WS_URL` is absent from `.env.local`. The `useTerminalSnapshot()` hook initialises from `initialTerminalSnapshot` (hardcoded values) and only overwrites via WebSocket. Without the env var the WebSocket is never opened (`if (!wsUrl) return`). Every `/terminal/*` page therefore renders hardcoded demo data including a BTC price of $105,842.50, two fictional positions, and five fictional strategies.

**Evidence:**
- `client/src/lib/terminal/terminalStore.tsx:21` — `const wsUrl = process.env.NEXT_PUBLIC_TERMINAL_WS_URL;`
- `client/src/lib/terminal/terminalStore.tsx:24` — `if (!wsUrl) return;`
- `client/src/lib/terminal/terminalSnapshot.ts:17` — `initialTerminalSnapshot` with `price: 105_842.5`
- `client/.env.local` — no `NEXT_PUBLIC_TERMINAL_WS_URL` entry

---

## 2. COMPONENT HIERARCHY

```
App (Next.js App Router)
├── / (TerminalDashboard)
│   └── AppShell
│       ├── TopBar (brand, nav)
│       ├── Sidebar (module links)
│       └── TerminalWorkspace
│           └── [child route page]
│               ├── /execution → ExecutionCenter(snapshot)
│               │   ├── InstitutionalChart (candles)
│               │   ├── Order Book (depth ladder)
│               │   ├── Open Positions table
│               │   ├── QuickTradePanel (stub — no real order placement)
│               │   └── Alert Tape
│               ├── /analytics → AnalyticsCenter(snapshot)
│               │   ├── Equity curve vs BTC benchmark
│               │   ├── Rolling Sharpe 30/90d
│               │   ├── R-multiple distribution
│               │   └── Family breakdown
│               ├── /research → ResearchCenter(snapshot)
│               │   ├── Strategy leaderboard table
│               │   └── Walk-forward fold table
│               ├── /risk → RiskModule(snapshot)
│               │   ├── VaR 95/99 / CVaR 95
│               │   ├── Heat bar
│               │   ├── Margin usage
│               │   └── Correlation heatmap
│               └── /journal → TradeJournalPro(snapshot)
│                   └── Detailed trade log
│
├── /paper-desk (PaperDeskDashboard)
│   ├── DashboardHeader (regime, kill button, reset button)
│   ├── Summary KPIs (balance, equity, PnL, drawdown)
│   ├── HealthSummary (strategy health counts)
│   ├── Open positions table
│   ├── Recent trades table
│   └── Tabs (lazy-loaded)
│       ├── Equity curve
│       ├── Strategy health
│       └── PaperOmsPanel (OMS orders — view only)
│
├── /btc-future-trading (TradingDashboard)
│   ├── DashboardHeader
│   ├── BTCFuturesScalper / BTCFutureTradingScalper
│   ├── Signal trace panel
│   ├── Attribution panel
│   └── Daily PnL ledger
│
└── /mock-trading (MockTradingDashboard)
    ├── Account summary
    ├── Active trades + signals
    ├── Rejection funnel diagnostics
    ├── Strategy leaderboard
    ├── MockRiskAnalyticsPanel (local kill-switch logic)
    ├── MockEquityCurvePanel
    ├── MockMonteCarloPanel
    └── Daily PnL ledger
```

---

## 3. STATE FLOW GRAPH

```
TERMINAL MODULE (/ and /terminal/*)
  initialTerminalSnapshot (HARDCODED MOCK)
      ↓ no WS connection (env var missing)
  useTerminalSnapshot() → stays at initial mock state forever
      ↓
  All terminal pages render mock data

PAPER DESK (/paper-desk)
  MongoDB Atlas (paper_state, paper_trades, paper_positions, ...)
      ↓ Next.js API routes
  /api/paper-desk/snapshot (5s)
      ↓
  usePaperDesk() → state, openPositions, recentTrades, healthSummary
      ↓
  PaperDeskDashboard renders live data

BTC FUTURES DESK (/btc-future-trading)
  /api/btc/price (polling)
  /api/mock-trading/signals (5s poll)
  FUTURES_STRAT_DEFS (in-memory, 600+ strategies)
      ↓
  useBTCFuturesScalperEngine() → local state machine
      ↓
  TradingDashboard renders

MOCK TRADING (/mock-trading)
  useMockTradingEngine() → fully in-browser simulation
  /api/mock-trading/* (15s persist cycle)
      ↓
  MockTradingDashboard renders
```

---

## 4. DEPENDENCY GRAPH (Key)

| Component | Depends On | Gap |
|-----------|-----------|-----|
| ExecutionCenter | `useTerminalSnapshot` | WS not wired |
| RiskModule | `useTerminalSnapshot` | WS not wired |
| InstitutionalRiskCenter | — | **DEAD CODE — never rendered in any page** |
| PaperDeskDashboard | `usePaperDesk` → MongoDB | Live, real |
| PaperOmsPanel | `/api/paper-oms/*` | View-only |
| DashboardHeader (kill button) | `resolveEngineApiUrl()` | Direct engine call, bypasses proxy |
| MockRiskAnalyticsPanel | local computation | Simulation only |

---

## 5. DESIGN SYSTEM STRUCTURE

- Custom CSS variables via `var(--green)`, `var(--red)`, `var(--accent)`, etc.
- `desk/ui/` component library: DeskButton, DeskCard, DeskChip, DeskDataTable, DeskMetricTile, DeskSectionHeader, DeskShell, DeskTabs, etc.
- Terminal components use Tailwind utility classes with zinc/emerald/rose palette
- No Radix UI or shadcn/ui dependency found — components are custom-built
- Dark-mode terminal shell, light-mode paper desk (inconsistent)

---

## 6. AUDIT VERDICT — ARCHITECTURE

| Dimension | Score | Evidence |
|-----------|-------|---------|
| Route coverage | 6/10 | 7 pages exist but primary terminal pages show mock data |
| State management | 4/10 | Two disconnected systems; terminal state never updates |
| Component reuse | 7/10 | `desk/ui/` library is consistent |
| Real-time wiring | 3/10 | Terminal WS absent; paper desk 5s poll is real |
| Dead code | FAIL | `InstitutionalRiskCenter` defined, imported nowhere |
| Data authenticity | FAIL | Home page `/` displays hardcoded demo data |

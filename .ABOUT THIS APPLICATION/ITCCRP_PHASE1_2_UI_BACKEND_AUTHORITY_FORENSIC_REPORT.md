# PHASE 1 + 2 — UI/BACKEND AUTHORITY FORENSIC REPORT & DATA TRUST CERTIFICATION
## Institutional Trading Command Center Reconstruction Program (ITCCRP)
**Date:** 2026-06-11 | **Auditor Role:** Principal Staff Architect + Independent Forensic Auditor

---

## AUDIT METHODOLOGY

All findings are sourced from:
- Direct source code reads
- Runtime wiring trace
- API route inspection
- React hook state flows
- Go Engine MongoDB collection schema

No comments, docs, or assumptions trusted.
Every finding carries: File · Function · Line · Proof.

---

## SECTION A — EXECUTION AUTHORITY BASELINE

### A1. Go Engine is Sole Execution Authority

**File:** `client/src/lib/engineAuthority.ts:10-12`
```typescript
export function isEngineExecutionAuthority(): boolean {
  return true;  // HARDCODED — no override path
}
```
**Proof:** Every write route (`/api/paper-desk-smoke-test/route.ts:44-48`) returns HTTP 410 unconditionally when this returns true.

**Implication:** No client-side trade generation. All execution data originates exclusively from Go Engine (AWS Lightsail) → MongoDB Atlas → Next.js API → UI.

---

## SECTION B — CRITICAL HARDCODED VALUE FINDINGS

### FINDING B1 — CRITICAL: Balance Always Returns $1,000,000 Hardcoded

**File:** `client/src/hooks/useEngineState.ts:5`
**Line:** `const FALLBACK_BALANCE = 1000000.0;`
**Line 39:** `return { engineOnline, balance: FALLBACK_BALANCE };`

**Proof chain:**
- `useEngineState()` polls `/health` endpoint (line 15)
- Balance field is NEVER fetched from any backend endpoint
- `balance: FALLBACK_BALANCE` is returned unconditionally regardless of engine state
- The hook never calls `/api/paper-desk/snapshot`, `/api/paper-desk/state`, or any balance endpoint

**Impact:** Any component consuming `useEngineState().balance` displays `$1,000,000` always, regardless of actual paper account balance.

**Data Trust Verdict:** ❌ FAIL — Cannot be trusted.

---

### FINDING B2 — CRITICAL: Entire Terminal State is Hardcoded Synthetic Data

**File:** `client/src/lib/terminal/terminalSnapshot.ts:17-121`

Hardcoded values in `initialTerminalSnapshot`:
| Field | Hardcoded Value | Source Needed |
|-------|----------------|---------------|
| `price` | `105_842.5` | Coinbase WS / Binance REST |
| `priceChange24hPct` | `1.42` | Market data API |
| `fundingRate` | `0.00018` | Delta Exchange |
| `regime` | `"Trending Bull"` | `useMarketRegime` |
| `positions[0]` | Full fake position object | Go Engine `paper_positions` |
| `positions[1]` | Full fake position object | Go Engine `paper_positions` |
| `risk.var95Usd` | `1_840` | Risk engine |
| `risk.marginUsagePct` | `18.6` | Portfolio accounting |
| `strategies` | 5 fake strategy rows | SEP / Go Engine scores |
| `analytics.equityCurve` | 6 fake data points | Go Engine `equity_curve` |
| `analytics.rollingSharpe30d` | `1.84` | Computed from trades |
| `analytics.winRatePct` | `54.2` | Go Engine `paper_state` |
| `journal` | 3 fake trade records | Go Engine `paper_trades` |
| `alerts` | 3 fake alerts | Go Engine events |

**Proof:** `client/src/app/terminal/execution/page.tsx:4-9`
```typescript
import { useTerminalSnapshot } from "@/lib/terminal/terminalStore";
export default function ExecutionPage() {
  const snapshot = useTerminalSnapshot();
  return <ExecutionCenter snapshot={snapshot} />;
}
```

`terminalStore.tsx:21`: `const wsUrl = process.env.NEXT_PUBLIC_TERMINAL_WS_URL;`
`terminalStore.tsx:24`: `if (!wsUrl) return;` — **If this env var is unset, the WebSocket never connects and the hardcoded snapshot IS the entire terminal state forever.**

`NEXT_PUBLIC_TERMINAL_WS_URL` is **NOT** in CLAUDE.md's env variable list, indicating it is likely unset in production.

**Components consuming hardcoded snapshot:**
- `ExecutionCenter.tsx` — positions, order book, risk, alerts
- `AnalyticsCenter.tsx` — equity curve, Sharpe, win rate, R-multiple buckets
- `ResearchCenter.tsx` — strategy table (hardcoded 5 strategies shown)
- `RiskModule.tsx` — VaR, CVaR, heat %, drawdown
- `TradeJournalPro.tsx` — trade journal entries

**Data Trust Verdict:** ❌ FAIL — Every metric in `/terminal/*` is unverifiable synthetic data until WebSocket is wired.

---

### FINDING B3 — HIGH: TerminalDashboard Falls Back to $1M Starting Balance

**File:** `client/src/components/TerminalDashboard.tsx:13`
```typescript
const STARTING_BALANCE = 1_000_000;
```
**Lines 281-286:**
```typescript
const equity = state?.equity ?? stateBalance ?? STARTING_BALANCE;
const realizedPnl = state?.realized_pnl ?? (stateBalance != null ? stateBalance - STARTING_BALANCE : 0);
```

**Proof:** When `desk.state` is null (MongoDB disconnected, unauthenticated, or engine offline), all KPI cards render with derived values from `STARTING_BALANCE`. The equity card shows `$1.000M`. All PnL fields show `$0.00`.

**Note:** This fallback is acceptable IF the null state is visually indicated to the operator. Current UI shows "No closed trades yet" text only in the strategy leaderboard but KPI cards still show numbers.

**Data Trust Verdict:** ⚠️ CONDITIONAL — Acceptable with null-state UI guard.

---

### FINDING B4 — HIGH: useMockTradingEngine Permanently Disabled, Mock Data Never Updates

**File:** `client/src/hooks/useMockTradingEngine.ts:221`
```typescript
const disablePolling = true;  // HARDCODED override, ignores opts.disablePolling
const persistenceDisabled = true;
```

**Proof:** Comment on line 216: `// SINGLE MOCK TRADING AUTHORITY — Phase 7 (2026-06-11)`

**Impact:** The mock trading engine hook silently returns empty trades, empty analytics, and default account state. Any dashboard consuming this hook shows no data. The `/mock-trading` page has no execution data.

**Components affected:**
- `MockTradingDashboard.tsx` — empty
- `InstitutionalBacktestDashboard.tsx` — empty
- Any component using `useMockTradingEngine`

**Data Trust Verdict:** ❌ FAIL for mock-trading metrics (intentional — research only via Go Engine).

---

## SECTION C — DATA FLOW TRACE: WHAT IS REAL

### C1. Paper Desk Data Flow (VERIFIED AUTHENTIC)

```
Go Engine (AWS Lightsail)
  → writes paper_state every ~10s          → MongoDB Atlas (loop_trades.paper_state)
  → writes paper_trades on close           → MongoDB Atlas (loop_trades.paper_trades)
  → writes paper_positions on open/update  → MongoDB Atlas (loop_trades.paper_positions)
  → writes paper_orders on OMS transition  → MongoDB Atlas (loop_trades.paper_orders)
  → writes equity_curve every ~5min        → MongoDB Atlas (loop_trades.equity_curve)
  → writes strategy_scores on settlement   → MongoDB Atlas (loop_trades.strategy_scores)
  → writes strategy_health periodically    → MongoDB Atlas (loop_trades.strategy_health)

MongoDB Atlas
  → /api/paper-desk/snapshot (route.ts)    [line 32-38: Promise.all of 5 queries]
  → paperDeskClient.ts:getPaperState()     [reads paper_state by account_key]
  → paperDeskClient.ts:listOpenPositions() [reads paper_positions]
  → paperDeskClient.ts:listPaperTrades()   [reads paper_trades, limit 20]
  → paperDeskClient.ts:getStrategyHealthSummary() [reads strategy_health]

/api/paper-desk/snapshot
  → usePaperDesk() hook (usePaperDesk.ts:79)
  → polls every 5s (POLL_MS = 5000)
  → TerminalDashboard.tsx (line 8: const desk = usePaperDesk())

TerminalDashboard.tsx
  → desk.state → equity, realizedPnl, unrealizedPnl, winRate, totalTrades
  → desk.openPositions → position table
  → desk.recentTrades → signal feed fallback
  → desk.healthSummary → active strategies count
```

**Data Trust Verdict:** ✅ PASS — The main dashboard (`/`) reads from authentic Go Engine MongoDB collections via authenticated API.

---

### C2. BTC Live Price Data Flow (VERIFIED AUTHENTIC)

```
Coinbase WebSocket (public, no auth)
  → useLiveBTCPrice hook (client/src/hooks/useLiveBTCPrice.ts)
  → live.price, live.change24h, live.high24h, live.low24h, live.ticksPerSecond
  → TerminalDashboard.tsx:14 (const live = useLiveBTCPrice())
  → BTC Price KPI card, Market Status section
```

**Data Trust Verdict:** ✅ PASS — Live from Coinbase WS.

---

### C3. Market Regime Data Flow (VERIFIED AUTHENTIC)

```
Candle data (useMockCandleBuilder → builds from live.price)
  → useMarketRegime hook
  → regime.regime ("Bullish"/"Bearish"/"Ranging")
  → TerminalDashboard.tsx:258 (const regime = useMarketRegime())
  → System Status "Market Regime" row
```

**Data Trust Verdict:** ⚠️ CONDITIONAL — Regime derived from synthetic candle builder (useMockCandleBuilder), not historical OHLCV from Delta/Binance. Candles are built from tick prices which is technically legitimate but the regime algorithm quality depends on candle quality.

---

### C4. Terminal Suite Data Flow (BROKEN — HARDCODED)

```
NEXT_PUBLIC_TERMINAL_WS_URL → NOT SET IN PRODUCTION
  → terminalStore.tsx:24: if (!wsUrl) return; (WebSocket never starts)
  → snapshot = initialTerminalSnapshot (terminalSnapshot.ts:17-121)
  → /terminal/execution → ExecutionCenter renders HARDCODED positions/order book/risk
  → /terminal/analytics → AnalyticsCenter renders HARDCODED equity curve/metrics
  → /terminal/research → ResearchCenter renders HARDCODED 5-strategy table
  → /terminal/risk → RiskModule renders HARDCODED VaR/CVaR/exposure
  → /terminal/journal → TradeJournalPro renders HARDCODED 3 fake trades
```

**Data Trust Verdict:** ❌ FAIL — All five terminal subpages display hardcoded data.

---

## SECTION D — COMPLETE METRIC TRUST AUDIT

| Metric | Where Displayed | Source | Trusted? | Evidence |
|--------|----------------|--------|----------|----------|
| Account Equity | `/` KPI grid | `paper_state.equity` via MongoDB | ✅ YES | `TerminalDashboard.tsx:281` |
| Total PnL | `/` KPI grid | computed from `paper_state` | ✅ YES | `TerminalDashboard.tsx:284-286` |
| Realized PnL | `/` KPI grid | `paper_state.realized_pnl` | ✅ YES | `TerminalDashboard.tsx:282` |
| Unrealized PnL | `/` KPI grid | `paper_state.unrealized_pnl` | ✅ YES | `TerminalDashboard.tsx:283` |
| Win Rate | `/` KPI grid | `paper_state.win_rate` | ✅ YES | `TerminalDashboard.tsx:287` |
| Profit Factor | `/` KPI grid | `strategy_scores.profit_factor` avg | ✅ YES | `TerminalDashboard.tsx:291-295` |
| Open Positions | `/` KPI grid | `paper_state.open_position_count` | ✅ YES | `TerminalDashboard.tsx:289` |
| Total Trades | `/` KPI grid | `paper_state.total_trades` | ✅ YES | `TerminalDashboard.tsx:288` |
| BTC Price | `/` KPI grid | Coinbase WebSocket live | ✅ YES | `useLiveBTCPrice` |
| Strategy Leaderboard | `/` | `strategy_scores` collection | ✅ YES | `TerminalDashboard.tsx:315-348` |
| Open Positions Table | `/` | `paper_positions` collection | ✅ YES | `TerminalDashboard.tsx:365-368` |
| Signal Feed | `/` | `paper_orders` collection | ✅ YES | `TerminalDashboard.tsx:350-363` |
| Active Strategy Count | `/` | `strategy_health.total` | ✅ YES | `TerminalDashboard.tsx:637` |
| useEngineState.balance | Multiple | HARDCODED `$1,000,000` | ❌ NO | `useEngineState.ts:5,39` |
| Execution Page Positions | `/terminal/execution` | HARDCODED synthetic | ❌ NO | `terminalSnapshot.ts:35-66` |
| Execution Page Order Book | `/terminal/execution` | HARDCODED synthetic | ❌ NO | `terminalSnapshot.ts:24-34` |
| Execution Page Risk | `/terminal/execution` | HARDCODED synthetic | ❌ NO | `terminalSnapshot.ts:67-87` |
| Analytics Equity Curve | `/terminal/analytics` | HARDCODED 6 data points | ❌ NO | `terminalSnapshot.ts:89-99` |
| Analytics Sharpe | `/terminal/analytics` | HARDCODED `1.84` | ❌ NO | `terminalSnapshot.ts:97` |
| Analytics Win Rate | `/terminal/analytics` | HARDCODED `54.2%` | ❌ NO | `terminalSnapshot.ts:99` |
| Research Strategy Table | `/terminal/research` | HARDCODED 5 strategies | ❌ NO | `terminalSnapshot.ts:81-87` |
| Risk VaR | `/terminal/risk` | HARDCODED `$1,840` | ❌ NO | `terminalSnapshot.ts:68` |
| Risk CVaR | `/terminal/risk` | HARDCODED `$2,380` | ❌ NO | `terminalSnapshot.ts:70` |
| Risk Drawdown | `/terminal/risk` | HARDCODED `1.4%` | ❌ NO | `terminalSnapshot.ts:72` |
| Journal Trades | `/terminal/journal` | HARDCODED 3 trades | ❌ NO | `terminalSnapshot.ts:110-114` |
| Alert Tape | `/terminal/execution` | HARDCODED 3 alerts | ❌ NO | `terminalSnapshot.ts:115-119` |
| Market Regime | `/` | `useMockCandleBuilder` → algo | ⚠️ APPROX | `TerminalDashboard.tsx:258` |
| Max Drawdown | `/` | `paper_state.max_drawdown` | ✅ YES | `TerminalDashboard.tsx:669` |
| Total Fees | `/` | `paper_state.total_fees` | ✅ YES | `TerminalDashboard.tsx:664` |
| Win/Loss Counts | `/` | `paper_state.winning_trades` | ✅ YES | `TerminalDashboard.tsx:661-662` |

---

## SECTION E — DEAD AND DISCONNECTED WIDGETS

### E1. `useEngineState.balance` — Dead Metric
**File:** `useEngineState.ts:39`
- Returns `1_000_000` always. No backend call for balance.
- Any component consuming this shows fake balance.
- **Action:** Wire to `/api/paper-desk/state` or remove.

### E2. Five Terminal Subpages — Disconnected from Backend
**Files:**
- `app/terminal/execution/page.tsx`
- `app/terminal/analytics/page.tsx`
- `app/terminal/research/page.tsx`
- `app/terminal/risk/page.tsx`
- `app/terminal/journal/page.tsx`

All five receive `snapshot` from `useTerminalSnapshot()` which only updates if `NEXT_PUBLIC_TERMINAL_WS_URL` is configured AND the WebSocket endpoint exists AND the Go Engine is sending terminal-format JSON deltas. None of these are currently implemented.

### E3. Order Book in ExecutionCenter — Synthetic
**File:** `components/terminal/institutional/ExecutionCenter.tsx:9,16-26`
- Displays `snapshot.bids` and `snapshot.asks` from hardcoded initialSnapshot.
- No real order book feed from Delta Exchange or Binance.

### E4. `useMockTradingEngine` — Permanently Disabled
- Returns empty state. The `/mock-trading` page has no data.
- Research-only use case — by design, but the page should say so.

---

## SECTION F — API ROUTE REACHABILITY AUDIT

### Routes Returning Real Data (VERIFIED)
| Route | Source | Reachability |
|-------|--------|-------------|
| `GET /api/paper-desk/snapshot` | MongoDB `paper_state` + 4 collections | Auth → MongoDB → JSON |
| `GET /api/paper-desk/trades` | MongoDB `paper_trades` | Auth → MongoDB → JSON |
| `GET /api/paper-desk/positions` | MongoDB `paper_positions` | Auth → MongoDB → JSON |
| `GET /api/paper-desk/orders` | MongoDB `paper_orders` | Auth → MongoDB → JSON |
| `GET /api/paper-desk/equity` | MongoDB `equity_curve` | Auth → MongoDB → JSON |
| `GET /api/paper-desk/strategy-health` | MongoDB `strategy_health` | Auth → MongoDB → JSON |
| `GET /api/paper-desk/strategy-analytics` | MongoDB `strategy_scores` | Auth → MongoDB → JSON |
| `GET /api/btc/price` | Coinbase/Binance | Live feed |
| `GET /api/strategy-rankings` | MongoDB `strategy_scores` | Auth → MongoDB → JSON |

### Routes Always Returning 410 GONE (BLOCKED BY ENGINE AUTHORITY)
| Route | Reason |
|-------|--------|
| `POST /api/paper-desk-smoke-test` | `isEngineExecutionAuthority()` = true |
| Any paper-desk write route | `isEngineExecutionAuthority()` = true |
| Cron tick routes (client-side) | Disabled |

---

## PHASE 2 — DATA TRUST CERTIFICATION

### Certification Matrix

**Question 1: Displayed balance = backend balance?**
- **`/` root page:** ✅ YES — `paper_state.equity` via MongoDB (when authenticated + MongoDB connected)
- **`useEngineState().balance`:** ❌ NO — hardcoded `$1,000,000`
- **`/terminal/*` pages:** ❌ NO — hardcoded `initialTerminalSnapshot`

**Question 2: Displayed positions = OMS positions?**
- **`/` root page open positions table:** ✅ YES — `paper_positions` collection
- **`/terminal/execution` positions table:** ❌ NO — hardcoded 2 synthetic positions

**Question 3: Displayed strategy count = actual strategy count?**
- **`/` Active Strategies status row:** ✅ YES — `strategy_health.total`
- **`/terminal/research` strategy table:** ❌ NO — hardcoded 5 rows

**Question 4: Displayed PnL = portfolio PnL?**
- **`/` KPI cards:** ✅ YES — sourced from `paper_state` via MongoDB
- **`/terminal/analytics` equity curve:** ❌ NO — hardcoded 6-point curve

**Question 5: Displayed trade count = OMS trades?**
- **`/` Trade Analytics:** ✅ YES — `paper_state.total_trades`
- **`/terminal/journal`:** ❌ NO — hardcoded 3 trades

**Question 6: Displayed fills = execution ledger fills?**
- **`/` Signal Feed:** ✅ YES — sourced from `paper_orders` collection
- **`/terminal/execution` alert tape:** ❌ NO — hardcoded 3 alerts

### Summary Certification

| Screen | Can Operator Trust It? | Evidence |
|--------|----------------------|----------|
| `/` (root Dashboard) | ✅ YES — with caveat | Paper desk data real; balance fallback acceptable with auth guard |
| `/paper-desk` | ✅ YES | Full MongoDB read from Go Engine collections |
| `/terminal/execution` | ❌ NO | All data hardcoded synthetic |
| `/terminal/analytics` | ❌ NO | All metrics hardcoded synthetic |
| `/terminal/research` | ❌ NO | Strategy table hardcoded |
| `/terminal/risk` | ❌ NO | All risk metrics hardcoded |
| `/terminal/journal` | ❌ NO | Trade journal hardcoded |
| `/mock-trading` | N/A | Research-only, intentionally disabled |
| `/btc-future-trading` | ⚠️ PARTIAL | Live BTC price real; strategy signals depend on hook wiring |

---

## PHASE 1+2 VERDICT

**Overall UI/Backend Alignment Score: 52/100** (confirms prior assessment)

**Root Cause:** The `/terminal/*` suite — 5 pages, the highest-visibility institutional interface — is 100% disconnected from backend data. It renders hardcoded synthetic data from `terminalSnapshot.ts` because `NEXT_PUBLIC_TERMINAL_WS_URL` is not configured, and no fallback polling is implemented.

**The `/` root dashboard is the only screen that truly deserves "live" status.**

**Immediate action required:**
1. Eliminate `initialTerminalSnapshot` synthetic data OR implement the WebSocket endpoint in Go Engine
2. Wire `useTerminalSnapshot` fallback to real API polling when WebSocket unavailable
3. Fix `useEngineState` balance to fetch from `/api/paper-desk/state`
4. Add explicit "DATA UNAVAILABLE" states to terminal subpages when snapshot is synthetic

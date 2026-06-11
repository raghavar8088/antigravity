# UPGRADE ROADMAP
**Phase 13 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11

---

## TARGET ARCHITECTURE

```
EXECUTION PATH (Go Engine — single authority)
══════════════════════════════════════════════════════════
Market Data Feeds (Coinbase WS / Binance / Delta / AngelOne)
    │
    ▼
Strategy Registry (600+ strategies, regime-gated)
    │
    ▼ Signal{side, confidence, strategyId, regime, timestamp}
    │
    ▼
Risk Gate v2 (Kelly + confidence + family limits + VaR + drawdown + kill switch)
    │
    ▼ ApprovedOrder{size, leverage, stopLoss, takeProfit, fees, slippage}
    │
    ▼
OMS v3 (event-sourced, PostgreSQL ledger, durable)
    │
    ▼
Paper Broker (fill + slippage model + fee model)
    │
    ▼
Positions Manager (in-memory, live authority)
    │
    ├─→ Reconciliation v2 (every 60s) → kill switch if drift
    │
    └─→ Persistence Pipeline
            ├─→ PostgreSQL (OMS events, durable)
            └─→ MongoDB (snapshots, trade records, analytics)

DASHBOARD PATH (Next.js — read-only)
══════════════════════════════════════════════════════════
Go Engine HTTP API (authoritative, live)
    │
    ├─→ SSE event stream → real-time position events
    ├─→ REST /api/positions → 2s polling (live)
    ├─→ REST /api/stats → live PnL (remove hardcoded balance)
    └─→ REST /api/engine/strategies → strategy registry

MongoDB (derived snapshots, analytics)
    │
    ├─→ paper_trades → strategy analytics, leaderboard
    ├─→ equity_curve → equity chart
    └─→ portfolio_metrics → 30-min aggregates

STRATEGY EVIDENCE PROGRAM (SEP)
══════════════════════════════════════════════════════════
paper_trades collection → SEP evidence builder
    │
    ├─→ Per-strategy: Sharpe, Sortino, expectancy, profit factor, max drawdown
    ├─→ Regime performance breakdown
    ├─→ Correlation matrix (pairwise strategy daily PnL)
    └─→ Alpha decay curves
```

---

## P0 FIXES (Critical — Do Immediately)

### 1. Close smoke-test execution gap
**File:** `client/src/app/api/paper-desk-smoke-test/route.ts`
**Change:** Add `isEngineExecutionAuthority()` guard at top of POST handler

```typescript
import { isEngineExecutionAuthority, ENGINE_AUTHORITY_SKIP_REASON } from "@/lib/engineAuthority";

export async function POST(req: Request) {
  if (isEngineExecutionAuthority()) {
    return NextResponse.json(
      { ok: false, code: "DEPRECATED", error: ENGINE_AUTHORITY_SKIP_REASON },
      { status: 410 },
    );
  }
  // ... existing code
}
```

### 2. Fix hardcoded balance in `useEngineState.ts`
**File:** `client/src/hooks/useEngineState.ts`
**Change:** Replace `FALLBACK_BALANCE = 1000000.0` with a poll to `/api/stats` or `/api/engine/stats`

### 3. Fix BTCFuturesScalper (empty screen)
**File:** `client/src/components/BTCFuturesScalper.tsx`
**Change:** Replace the disabled hook with a read-only dashboard consuming `/api/paper-desk/snapshot`. Or add a banner: "Paper desk disabled — View Paper Desk Dashboard"

---

## P1 IMPROVEMENTS (Institutional Quality Tier)

### 4. Add Sharpe/Sortino/drawdown to PaperDeskDashboard
**Source:** Compute from MongoDB `paper_trades` (all closed trades with PnL and timestamps)
**API:** New `/api/paper-desk/risk-metrics` endpoint that returns:
```json
{
  "sharpeRatio": 1.84,
  "sortinoRatio": 2.21,
  "calmarRatio": 0.93,
  "maxDrawdownPct": 12.4,
  "maxDrawdownDurationDays": 3,
  "annualizedReturn": 11.5
}
```
**Frontend:** Add risk metrics panel to PaperDeskDashboard header

### 5. Add kill switch status to global nav bar
**File:** `client/src/components/layout/Header.tsx` (or equivalent nav component)
**Change:** Add persistent status indicator: `● ENGINE LIVE | KS: ARMED | Positions: 3 | Today: +$1,240`

### 6. Add per-strategy metrics to leaderboard
**API:** Extend `/api/paper-trades/strategy-stats` to include: expectancy, profit factor, max drawdown, regime breakdown
**Frontend:** Add columns to StrategyLeaderboard

### 7. Add reconciliation history panel
**API:** New `/api/engine/reconciliation` endpoint exposing last 24h reconciliation events from PostgreSQL
**Frontend:** New panel in RiskModule or ObservabilityPanel component

---

## P2 IMPROVEMENTS (Deep Institutional Tier)

### 8. Strategy correlation matrix
**API:** New `/api/paper-trades/correlation-matrix` endpoint
**Computation:** Daily PnL time series per strategy → Pearson correlation matrix
**Frontend:** Heatmap component in AnalyticsCenter

### 9. Slippage dashboard
**API:** Extend `/api/paper-trades` response to include `slippageBps` field (already stored in Go struct)
**Frontend:** Slippage panel: avg entry slippage, avg exit slippage, slippage trend

### 10. Regime analysis panel
**API:** Aggregate `paper_trades` by `regimeAtEntry` field
**Frontend:** New tab in AnalyticsCenter: performance by regime

### 11. SSE event stream for positions
**API:** New `GET /api/engine/events` SSE endpoint wrapping Go engine position events
**Frontend:** Replace position polling (2s REST) with SSE push — reduces polling overhead

---

## P3 IMPROVEMENTS (Future — Phase 15+)

### 12. Alpha decay analysis
**Computation:** For each strategy, plot rolling 30-day Sharpe vs inception Sharpe
**Requires:** At least 90 days of trade data per strategy

### 13. Trade attribution engine
**API:** New attribution endpoint: decompose realized PnL by strategy family, regime, time-of-day, signal confidence
**Frontend:** Attribution panel with waterfall chart

### 14. Live price feed to PaperDeskDashboard
**Change:** Replace MongoDB snapshot unrealized PnL with live price × quantity computation in browser using price feed

### 15. WINNERS_ONLY gate audit log
**API:** Expose strategy removal log from PostgreSQL
**Frontend:** Strategy history timeline: when was each strategy added/removed and why

---

## SEP INTEGRATION TARGET

The Strategy Evidence Program (SEP) should be the data backend for the institutional analytics tier:

```
paper_trades collection
    └─→ SEP evidence builder (Python brain or Go aggregator)
            ├─→ Per-strategy: 90-day Sharpe, expectancy, profit factor, regime breakdown
            ├─→ Correlation matrix (daily)
            ├─→ Alpha decay curve (rolling Sharpe)
            └─→ SEP verdict: APPROVED / PROBATION / REMOVED
                    └─→ Drives WINNERS_ONLY gate in strategy registry
```

This closes the loop: strategies earn their place in the registry by passing SEP evidence review, not by surviving ad-hoc manual review.

---

## ARCHITECTURE SCORE TARGET

| Dimension | Current | Target (Phase 15) |
|-----------|---------|------------------|
| Execution Architecture | 95/100 | 100/100 |
| UI/Backend Alignment | 55/100 | 85/100 |
| Institutional Dashboard | 55/100 | 85/100 |
| Observability | 55/100 | 85/100 |
| Production Readiness | 70/100 | 90/100 |
| **Composite** | **66/100** | **89/100** |

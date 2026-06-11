# UI/BACKEND ALIGNMENT REPORT
**Phase 5 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification only

---

## VERDICT: SIGNIFICANT ALIGNMENT GAPS IDENTIFIED

---

## SCREEN 1: `PaperDeskDashboard`

**File:** `client/src/components/PaperDeskDashboard.tsx`
**Data hook:** `usePaperDesk.ts` — polls `/api/paper-desk/snapshot` every 5 seconds

### Open Positions

| Element | API Endpoint | Backend Source | Gap |
|---------|--------------|----------------|-----|
| Position count | `/api/paper-desk/snapshot` | `paper_state.open_position_count` (MongoDB) | 10s snap + 5s poll = up to **15s stale** |
| Position list | `/api/paper-desk/snapshot` | Reconstructed from `paper_state.positions[]` | **NOT** from `positions/manager.go` (live source). Reconstructed from snapshot. |
| SL/TP levels | `/api/paper-desk/snapshot` | Reconstructed from snapshot | May not reflect real-time risk adjustments |

**Verdict: STALE** — Positions displayed are up to 15 seconds behind actual engine state.

### PnL

| Element | API Endpoint | Backend Source | Gap |
|---------|--------------|----------------|-----|
| Realized PnL | `/api/paper-desk/snapshot` | `paper_state.realized_pnl` (MongoDB) | 10s snap lag |
| Unrealized PnL | `/api/paper-desk/snapshot` | `paper_state.unrealized_pnl` (MongoDB) | 10s snap lag + mark price staleness |
| Balance | `/api/paper-desk/snapshot` | `paper_state.balance` (MongoDB) | 10s snap lag |
| Equity | `/api/paper-desk/snapshot` | Derived: balance + unrealized | Double lag |

**Verdict: STALE** — Up to 15 seconds behind live engine state.

### Trade History

| Element | API Endpoint | Backend Source | Gap |
|---------|--------------|----------------|-----|
| Closed trades | `/api/paper-desk/trades` | MongoDB `paper_trades` | Trades written async; newly closed trades may not appear for 1–2s |

**Verdict: MINOR LAG** — Acceptable for display purposes.

### OMS Orders

| Element | API Endpoint | Backend Source | Gap |
|---------|--------------|----------------|-----|
| Order states | `/api/paper-desk/orders` | MongoDB `paper_orders` | Written on fill — relatively fresh |

**Verdict: ACCEPTABLE**

### Equity Curve

| Element | API Endpoint | Backend Source | Gap |
|---------|--------------|----------------|-----|
| Equity time series | `/api/paper-desk/equity` | MongoDB `equity_curve` | 1-min insert cadence — may miss intraday spikes |

**Verdict: ACCEPTABLE** for charting purposes.

---

## SCREEN 2: `BTCFuturesScalper`

**File:** `client/src/components/BTCFuturesScalper.tsx`
**Data hook:** `useBTCFuturesScalperEngine()` — poll is DISABLED (returns immediately)

**All data in this screen is EMPTY/STALE because:**
- The hook's `poll()` returns immediately (line 2679)
- `positions: []`, `trades: []`, `balance: INITIAL_BALANCE_DEFAULT` (hardcoded)
- No live data from engine is flowing to this component

| Element | Actual Source | Is it live? |
|---------|---------------|-------------|
| Open positions | React state (empty — poll disabled) | NO — empty |
| Closed trades | React state (empty) | NO — empty |
| Balance | INITIAL_BALANCE_DEFAULT constant | NO — hardcoded |
| Equity | INITIAL_BALANCE_DEFAULT | NO — hardcoded |
| Strategy status | React state (empty) | NO — empty |
| Quote price | React state (empty — poll disabled) | NO — empty |

**Verdict: DISCONNECTED** — This screen shows no live data. It renders with empty/default values. The UI exists but conveys no meaningful information in its current disabled state.

**NOTE:** This screen should be replaced with a read-only view that polls the Go engine API directly.

---

## SCREEN 3: `TerminalDashboard`

**File:** `client/src/components/TerminalDashboard.tsx` / Terminal Suite
**Data hooks:** `usePaperDesk.ts`, `useEngineState.ts`

### `useEngineState.ts` — Critical Issue

**Exact source code read:**
```typescript
const FALLBACK_BALANCE = 1000000.0;  // hardcoded constant
```

The hook polls `/api/health` for a boolean `engineOnline` flag but the balance/equity displayed is from `FALLBACK_BALANCE = 1000000.0` — a hardcoded constant that is never updated from the engine.

| Element | Source | Is it live? |
|---------|--------|-------------|
| Engine online status | `/api/health` | YES |
| Balance | HARDCODED `1000000.0` | **NO — FAKE** |
| Equity | HARDCODED | **NO — FAKE** |

**Verdict: FAKE DATA** — The balance displayed in `useEngineState` is never synced from the Go engine.

---

## SCREEN 4: Terminal Institutional Suite

**Screens:** ExecutionCenter, AnalyticsCenter, RiskModule, ResearchCenter, TradeJournalPro
**Data hook:** `useTerminalSnapshot()` + component-specific hooks

Based on source analysis:
- These screens read from the Go engine API proxy (`/api/engine/[...path]`)
- `usePositions.ts` polls `/api/positions` → Go engine in-memory `posMgr.GetOpenPositions()` — **LIVE**
- `useTrades.ts` polls `/api/trades` → Go engine DB (fallback: in-memory journal) — **LIVE**
- `useStrategies.ts` polls `/api/engine/strategies` → Go engine strategy registry — **LIVE**

**Verdict: LIVE** — Terminal suite reads from the authoritative engine API.

---

## PARALLEL API INCONSISTENCY

**Critical finding:** Two independent position APIs exist, both active, showing different data:

| API | Source | Data Freshness |
|-----|--------|----------------|
| `GET /api/positions` | `positions/manager.go` (in-memory, live) | Real-time |
| `GET /api/paper-desk/snapshot` → positions | `paper_state` (MongoDB, 10s lag) | Stale (≤15s) |

- `usePositions.ts` reads the live source
- `usePaperDesk.ts` reads the stale MongoDB source
- Different dashboard components use different APIs
- The same position can show different values in different parts of the UI simultaneously

**Verdict: DIVERGENCE RISK** — Two dashboard screens showing the same positions but from different sources with different freshness guarantees.

---

## MISSING FIELDS (Identified from Source)

| Field | Backend Has It | UI Shows It | Gap |
|-------|---------------|-------------|-----|
| Entry slippage (bps) | YES — `ClosedTrade.SlippageBps` | NO — not displayed | MISSING |
| Exit slippage (bps) | YES | NO | MISSING |
| Strategy family | YES | PARTIAL — leaderboard only | PARTIAL |
| Regime at entry | YES — `ClosedTrade.RegimeAtEntry` | NO — not on trade detail | MISSING |
| Risk approval reason | YES — `RiskMetrics.ApprovalReason` | NO | MISSING |
| Position age | YES — calculated from OpenedAt | PARTIAL | PARTIAL |

---

## STALE DATA SUMMARY

| Screen | Max Staleness | Acceptable? |
|--------|--------------|-------------|
| PaperDeskDashboard positions | 15s (10s snap + 5s poll) | Marginal |
| PaperDeskDashboard PnL | 15s | Marginal |
| BTCFuturesScalper | Indefinite (poll disabled) | **NO — shows nothing** |
| Terminal positions | 2s (live engine poll) | YES |
| Terminal trades | 2s (live engine poll) | YES |
| useEngineState balance | Infinite (hardcoded constant) | **NO — FAKE** |

---

## ALIGNMENT VERDICT MATRIX

| Data Type | Screen | Alignment |
|-----------|--------|-----------|
| Positions | PaperDeskDashboard | STALE (MongoDB-based) |
| Positions | BTCFuturesScalper | DISCONNECTED (poll disabled) |
| Positions | Terminal Suite | LIVE (engine API) |
| PnL | PaperDeskDashboard | STALE |
| PnL | BTCFuturesScalper | DISCONNECTED |
| Balance | useEngineState | **FAKE** (hardcoded) |
| Orders | PaperDeskDashboard | ACCEPTABLE |
| Signals | Any screen | N/A (not shown directly) |
| Risk metrics | Terminal RiskModule | LIVE |
| Portfolio | PaperDeskDashboard | STALE |

---

## REMEDIATION REQUIRED

1. **`useEngineState.ts`** — Remove hardcoded balance. Poll `/api/engine/state` or `/api/stats` for live balance.
2. **`BTCFuturesScalper.tsx`** — Since `useBTCFuturesScalperEngine` is disabled, the component should be replaced with a read-only dashboard that consumes `/api/paper-desk/snapshot` or `/api/positions`.
3. **Unify position source** — Choose either live `/api/positions` or MongoDB snapshot. Do not show both simultaneously in different screens.
4. **Add missing fields to trade detail view** — Slippage, regime at entry, risk approval reason.

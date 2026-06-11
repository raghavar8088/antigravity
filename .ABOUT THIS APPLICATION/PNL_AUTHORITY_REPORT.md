# PNL AUTHORITY REPORT
**Phase 5 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**FAIL — 6 PnL calculation sources identified. No single authoritative source enforced.**

---

## PNL CALCULATION INVENTORY

### PnL Source L1 — Go Engine Position Manager (AUTHORITATIVE — realized)

| Property | Value |
|----------|-------|
| File | `engine/internal/positions/manager.go:261` |
| Function | `calculatePnL(pos, exitPrice)` |
| Formula | `(exit - entry) × size` (long) / `(entry - exit) × size` (short) |
| When computed | At position close event |
| Authority | PRIMARY — this is the realized PnL record |

### PnL Source L2 — Go Engine Unrealized PnL (AUTHORITATIVE — unrealized)

| Property | Value |
|----------|-------|
| File | `engine/internal/trading/paperpersist_hooks.go:85` |
| Function | `computeUnrealizedPnL(openPositions, markPrice)` |
| Formula | Sum of mark-to-market across all open positions |
| When computed | Every 10 seconds in GetAccountSnapshot() |
| Authority | PRIMARY for unrealized state |
| Risk | Mark price from execution layer; if price feed stale, unrealized wrong |

### PnL Source L3 — Go Engine Ledger / OMS v3 (AUTHORITATIVE — event-sourced)

| Property | Value |
|----------|-------|
| File | `engine/internal/omsv3/authority.go` |
| Storage | PostgreSQL (durable ledger) + MongoDB (real-time) |
| When computed | On every EventPositionClosed |
| Authority | PRIMARY — event-sourced, immutable, survives restarts |

### PnL Source L4 — Browser Scalper Engine (UNAUTHORIZED)

| Property | Value |
|----------|-------|
| File | `client/src/hooks/useBTCFuturesScalperEngine.ts` |
| Library | `client/src/lib/futuresPaperMath.ts` → `paperNetPnlOnClose()` |
| When computed | On every trade close (4s poll) |
| Authority | NONE — independent browser calculation |
| Risk | PnL calculated in browser independently from Go engine |

### PnL Source L5 — Client Portfolio Accounting Service (DERIVED — READ ONLY INTENDED)

| Property | Value |
|----------|-------|
| File | `client/src/lib/portfolioAccountingService.ts` |
| Function | `buildPortfolioAccountingSnapshot()` |
| Sources | MongoDB paper_state + closed_trade_stats + open positions + equity curve |
| When computed | On demand, read from MongoDB |
| Authority | DERIVED — aggregates from MongoDB |
| Risk | 15s max lag (5s poll + 10s backend snap); if any collection stale, total wrong |

### PnL Source L6 — Mock Trading Engine PnL (UNAUTHORIZED)

| Property | Value |
|----------|-------|
| File | `client/src/lib/mockTradingEngine.ts` |
| Functions | `computeMockPnl()`, `computeMockNetPnlAtExitMark()` |
| Storage | MongoDB `mock_trades`, `mock_account_snapshots` |
| Authority | NONE (separate mock layer) |

---

## SINGLE AUTHORITATIVE PNL SOURCE

**Authorized:** L1 (realized), L2 (unrealized), L3 (ledger events)
**Read-only derived display:** L5 (portfolio accounting service — acceptable for UI)
**Must remove:** L4 (browser scalper PnL), L6 (mock trading PnL)

---

## DIVERGENCE RISK

| Scenario | Impact |
|----------|--------|
| Browser scalper closes trade, Go engine unaware | L4 shows profit, L1 unchanged, dashboard shows conflicting PnL |
| MongoDB lag (10s) while Go engine computes | L5 shows stale PnL vs L2 live |
| Mock engine runs alongside Go engine | L6 inflates apparent PnL in mock collections |

---

## REQUIRED ACTION

Remove L4 and L6. L5 is acceptable as a read-only display aggregator but must not generate its own PnL events. L1, L2, L3 are the sole PnL authorities.

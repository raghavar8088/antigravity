# STATE AUTHORITY REPORT
**Phase 4 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification only

---

## VERDICT: BACKEND IS AUTHORITATIVE FOR EXECUTION — PnL IS FRAGMENTED

---

## SIGNALS

**Authoritative source:** `engine/internal/strategy/` — Go strategy interface
**Writers:** Strategy implementations via `Strategy.OnTick()` / `Strategy.OnCandle()`
**Storage:** In-memory only — signals are ephemeral, consumed immediately by OMS
**Duplicate stores:** NONE
**Browser state authority:** NONE — `useBTCFuturesScalperEngine.ts` poll is disabled; no signals generated client-side

| Question | Answer |
|----------|--------|
| Backend sole authority? | YES — Go engine strategy registry |
| Duplicate store? | NO |
| Browser state authority? | NO — poll disabled |
| Gap | No persistent audit trail of raw signals. Only closed trade records survive. |

---

## ORDERS

**Authoritative source:** `engine/internal/omsv3/authority.go` — OMS v3
**Writers:** `RecordOrderCreated()`, `RecordOrderValidated()`, `RecordPositionOpened()`, `RecordPositionClosed()`
**Storage:** In-memory projections (orderProjs, positionProjs) + PostgreSQL ledger (durable) + MongoDB real-time
**Client-side OMS:** `client/src/lib/paperOms.ts` — state machine exists but callers are disabled (paper desk worker returns stub)
**Duplicate stores:** Client-side `paperOms.ts` exists as dead code — it is a duplicate OMS state machine that is currently inactive

| Question | Answer |
|----------|--------|
| Backend sole authority? | YES — OMS v3 event-sourced |
| Duplicate store? | DEAD CODE — `paperOms.ts` inactive |
| Browser order authority? | NO |
| Gap | `paperOms.ts` and `paperOmsMongo.ts` exist as dead code. Should be archived. |

---

## POSITIONS

**Authoritative source:** `engine/internal/positions/manager.go`
**Data structure:** `map[string]*Position` (in-process Go map)
**Writers:** `OpenPosition()`, `ClosePosition()`, `CloseAllPositions()`, `RestorePositions()`
**Storage layers:**
1. In-memory: `positions/manager.go` (authoritative, real-time)
2. MongoDB `paper_positions`: 10-second snapshot write-through (derived, lags by ≤10s)
3. PostgreSQL OMS v3 ledger: EventPositionOpened/Closed (durable, event-sourced)

**Browser position state:** `useBTCFuturesScalperEngine.ts` has `positions: BTCFuturesPosition[]` in React state — but this state is empty (poll disabled, `setPositions()` never called)

| Question | Answer |
|----------|--------|
| Backend sole authority? | YES — `positions/manager.go` |
| Duplicate store? | NO active duplicate (browser state is empty) |
| Browser state authority? | NO |
| Gap | 10-second lag between engine close event and MongoDB write. If engine crashes during this window, position is lost. Recovery via `RestorePositions()` from MongoDB snapshot exists but may see stale data. |

---

## PNL

**This is the most fragmented data type.**

**Sources identified (all in Go engine):**

| Source | File | Type | When |
|--------|------|------|------|
| Realized PnL | `positions/manager.go:261` — `calculatePnL()` | Per-trade calculation | At position close |
| Unrealized PnL | `paperpersist_hooks.go:85` — `computeUnrealizedPnL()` | Portfolio mark-to-market | Every 10s snapshot |
| Portfolio stats | `cmd/antigravity/main.go:1817` — `/api/stats` endpoint | Aggregated | On demand |
| Portfolio metrics | `paperpersist/portfolio_metrics_writer.go` | 30-min aggregation | Every 30 min |
| Equity curve | `paperpersist/equity_recorder.go` | Time-series | Every 1 min |
| Daily PnL | `paperpersist/daily_pnl_history` | Sealed daily record | Midnight UTC |

**No single source of truth.** Three in-memory objects contribute to the stats endpoint:
- `journal.GetAggregateStats()` — in-memory trade summary
- `paperExecute.GetEquityUSD()` — realized balance
- `riskEngine.GetDailyPnL()` — daily PnL

These can diverge if any one crashes between MongoDB writes.

**Browser PnL state:** Empty — `useBTCFuturesScalperEngine.ts` poll is disabled

| Question | Answer |
|----------|--------|
| Backend sole authority? | YES for calculation — but FRAGMENTED across 3 in-memory objects |
| Duplicate store? | 5 separate persistence paths for PnL data |
| Browser state authority? | NO |
| Gap | CRITICAL — No atomic PnL store. Three in-memory sources can diverge. No reconciliation on restart. |

---

## RISK

**Authoritative source:** `engine/internal/risk/gate/pipeline.go` + kill switch
**Pre-trade gate:** `pipeline.go:46` — Kelly sizing, confidence threshold, family limits, drawdown scaling
**Portfolio gate:** `trading/loop.go:463` — PMS heat/VaR/drawdown checks
**Kill switch:** `killswitch/service.go` — survives restarts via PostgreSQL ledger

**Risk decisions in browser:** NONE — the paper desk worker risk gate is dead code (worker returns stub)

| Question | Answer |
|----------|--------|
| Backend sole authority? | YES |
| Duplicate store? | NO |
| Browser risk authority? | NO |

---

## PORTFOLIO

**Authoritative sources:** Multiple (same problem as PnL)

In-memory sources:
- `posMgr.GetOpenPositions()` — live positions
- `journal.GetRecentTrades()` — trade history
- `paperExecute.GetBalanceUSD()` / `GetEquityUSD()` — balance/equity

MongoDB persistence (derived):
- `paper_state` — 10s snapshot of account state
- `paper_trades` — closed trade records
- `paper_positions` — open position cache
- `portfolio_metrics` — 30-min aggregate

**UI reads from MongoDB**, not from live engine objects (except `/api/positions` and `/api/stats` which read in-memory directly).

| Question | Answer |
|----------|--------|
| Backend sole authority? | YES for computation |
| Duplicate store? | YES — 4 MongoDB collections duplicate portfolio data; 3 in-memory objects contribute to live stats |
| Browser portfolio authority? | NO |
| Gap | No single authoritative portfolio store. Dashboard may show inconsistent totals if any write path fails. |

---

## MONGO AUTHORITY CONFLICTS

| Collection | Writer(s) | Conflict? |
|-----------|----------|-----------|
| `paper_trades` | Go engine only (paperpersist_hooks.go) | NO — single writer |
| `paper_state` | Go engine only (state_snapshotter.go) | NO — single writer |
| `paper_positions` | Go engine only | NO — single writer |
| `equity_curve` | Go engine only | NO — single writer |
| `portfolio_metrics` | Go engine only | NO — single writer |
| `paper_orders` | Go engine only (order_writer.go) | NO — single writer |
| `mock_trades` | Browser mock engine — DISABLED | NO active writer |
| `paper_oms_orders` | Worker — DISABLED | NO active writer |

**All MongoDB write paths have a single authorized writer: the Go engine.**

---

## SUMMARY

| Data Type | Backend Authority | Browser Authority | Duplicate Active | Gap Severity |
|-----------|-----------------|-----------------|-----------------|--------------|
| Signals | GO ENGINE | NONE | NO | LOW (no persistence) |
| Orders | GO ENGINE (OMS v3) | NONE | NO (dead code) | LOW |
| Positions | GO ENGINE | NONE | NO | MEDIUM (10s lag) |
| PnL | GO ENGINE | NONE | NO | HIGH (fragmented) |
| Risk | GO ENGINE | NONE | NO | LOW |
| Portfolio | GO ENGINE | NONE | NO | HIGH (fragmented) |

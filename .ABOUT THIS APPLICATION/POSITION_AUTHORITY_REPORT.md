# POSITION AUTHORITY REPORT
**Phase 4 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**FAIL — 4 position stores identified. No single authoritative source.**

---

## POSITION STORES INVENTORY

### Store P1 — Go Engine In-Memory Position Manager (AUTHORITATIVE)

| Property | Value |
|----------|-------|
| File | `engine/internal/positions/manager.go` |
| Type | `map[string]*Position` (in-process Go map) |
| Authority | PRIMARY — execution engine owns this state |
| Mutations | `OpenPosition()`, `ClosePosition()`, `CloseAllPositions()` |
| SL/TP checks | Every market tick |
| Crash recovery | `RestorePositions()` from MongoDB |
| Access | Internal engine only |

**Is this authoritative? YES**

### Store P2 — MongoDB paper_positions (DERIVED)

| Property | Value |
|----------|-------|
| Collection | `paper_positions` |
| Written by | Go engine via `persistPositionOpen()` / `persistPositionClose()` hooks |
| Written by also | Browser scalper engine (indirect, via paper-desk/positions writes) |
| Authority | DERIVED from P1 — write-through cache with 1–10s lag |
| Read by | Next.js API routes, portfolio accounting service, dashboard UI |
| Risk | Go engine closes position → crash before DB write → MongoDB shows OPEN but Go shows CLOSED |

**Is this authoritative? NO — derived, lags behind P1**

### Store P3 — Browser React State (UNAUTHORIZED)

| Property | Value |
|----------|-------|
| File | `client/src/hooks/useBTCFuturesScalperEngine.ts` |
| Type | `useState<OpenPosition[]>` |
| Authority | NONE — independent browser-side position tracking |
| Mutations | `openAndTrackPosition()`, SL/TP exit logic on every 4s poll |
| Persistence | Writes to MongoDB paper_trades on close |
| Risk | Browser positions tracked independently from Go engine positions |

**Is this authoritative? NO — duplicate, unauthorized**

### Store P4 — Browser Mock Positions (UNAUTHORIZED)

| Property | Value |
|----------|-------|
| File | `client/src/hooks/useMockTradingEngine.ts` |
| Type | `useState<MockTrade[]>` (open mock trades) |
| Authority | NONE — separate mock position store |
| Mutations | `ingestTraceRows()` creates trades, `applyPriceTickToTrade()` updates marks |
| Persistence | Writes to MongoDB `mock_trades` on close |

**Is this authoritative? NO — simulation only, unauthorized execution**

---

## AUTHORITY DETERMINATION

**Single authoritative position store: Go engine `positions/manager.go`**

All other stores are either:
- Derived (P2 — valid as read cache)
- Unauthorized duplicates that must be removed (P3, P4)

---

## DIVERGENCE CONDITIONS

| Condition | Result |
|-----------|--------|
| Go engine closes position, MongoDB write pending | P1 = CLOSED, P2 = OPEN (mismatch) |
| Browser scalper opens position | P3 = OPEN, P1 = no record (mismatch) |
| Kill switch fires in Go engine | P1 closes all, P3 continues trading |
| Browser scalper and Go engine both open for same signal | P1 + P3 = double position |

---

## REQUIRED ACTION

Remove P3 and P4. Convert client to read P2 (via API) for display only. Never write to position stores from browser.

# DATABASE CONSOLIDATION REPORT
**Phase 10 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**CONSOLIDATED — Single write path to each collection. Duplicate write paths eliminated.**

---

## TRADE STORAGE AUDIT

### Collection: `paper_trades`

| Writer | Before | After | Notes |
|--------|--------|-------|-------|
| Go engine (`paperpersist_hooks.go`) | ACTIVE | ACTIVE | **Sole authorized writer** |
| Browser scalper (`paperTradesSync.ts`) | ACTIVE | BLOCKED (HTTP 410) | Phase 7 |
| Paper desk worker | ACTIVE | BLOCKED (stub + HTTP 410) | Phase 7 |

**Authoritative writer: Go engine**

### Collection: `paper_state`

| Writer | Before | After | Notes |
|--------|--------|-------|-------|
| Go engine (`paperpersist/state_snapshotter.go`) | ACTIVE | ACTIVE | **Sole authorized writer** |
| Browser scalper (`saveToMongo()`) | ACTIVE | BLOCKED (early return) | Phase 7 |
| Paper desk worker | ACTIVE | BLOCKED (stub) | Phase 7 |

**Authoritative writer: Go engine**

### Collection: `paper_positions`

| Writer | Before | After | Notes |
|--------|--------|-------|-------|
| Go engine (`paperpersist_hooks.go`) | ACTIVE | ACTIVE | **Sole authorized writer** |
| Browser scalper (indirect) | POTENTIAL | BLOCKED | Phase 7 |

**Authoritative writer: Go engine**

### Collection: `mock_trades`

| Writer | Before | After | Notes |
|--------|--------|-------|-------|
| Browser mock engine | ACTIVE | BLOCKED (`persistenceDisabled=true`) | Phase 7 |
| `/api/mock-trading/trades` POST | BLOCKED (410) | BLOCKED (410) | Already guarded |

**Authoritative writer: NONE (mock trading disabled)**

### Collection: `mock_account_snapshots`

| Writer | Before | After | Notes |
|--------|--------|-------|-------|
| Browser mock engine | ACTIVE | BLOCKED | Phase 7 |

### Collection: `paper_oms_orders`

| Writer | Before | After | Notes |
|--------|--------|-------|-------|
| Paper desk worker | ACTIVE | BLOCKED (worker stub) | Phase 7 |
| `/api/cron/paper-desk-tick` | ACTIVE | SKIPPED (isEngineExecutionAuthority) | Phase 7 |
| Go engine (omsv3/authority.go) | ACTIVE | ACTIVE | **Sole authorized writer** |

---

## POSITION STORAGE AUDIT

| Store | Writer | Type | Status |
|-------|--------|------|--------|
| Go in-memory (`positions/manager.go`) | Go engine | Runtime | ACTIVE — authoritative |
| MongoDB `paper_positions` | Go engine | Write-through | ACTIVE — derived |
| Browser React state | Browser scalper | In-memory | DEAD CODE (poll disabled) |
| Browser mock state | Mock engine | In-memory | DEAD CODE (poll disabled) |

---

## PNL STORAGE AUDIT

| Store | Writer | Type | Status |
|-------|--------|------|--------|
| `equity_curve` collection | Go engine | Append-only time-series | ACTIVE — authoritative |
| `daily_pnl_history` | Go engine | Daily seal | ACTIVE — authoritative |
| `portfolio_metrics` | Go engine | Aggregated snapshot | ACTIVE — authoritative |
| `mock_equity_curve` | Browser mock engine | Mock | BLOCKED (Phase 7) |
| `mock_daily_pnl_history` | Browser mock engine | Mock | BLOCKED (Phase 7) |

---

## IN-MEMORY STORES

| Store | Location | Status |
|-------|----------|--------|
| Go position map | `engine/internal/positions/manager.go` | ACTIVE — authoritative |
| Go risk state | `engine/internal/risk/` | ACTIVE — authoritative |
| Go ledger projections | `engine/internal/omsv3/authority.go` | ACTIVE — authoritative |
| Browser paper account | `useBTCFuturesScalperEngine.ts` | DEAD (poll disabled) |
| Browser mock account | `useMockTradingEngine.ts` | DEAD (disabled) |

---

## BROWSER STORAGE (localStorage / sessionStorage)

| Key | Used for | Status |
|-----|----------|--------|
| `desk_regime_histogram_*` | Regime tracking for browser engine | STALE (engine disabled, writes stopped) |
| `paper_entry_burst_*` | Burst guard state | STALE (engine disabled) |

These keys may persist from prior sessions but are never written again with execution disabled. They have no effect on Go engine behavior.

---

## SQLITE

**Location:** `engine/data/engine.db` (AWS Lightsail)
**Status:** ACTIVE — fallback persistence for Go engine state if MongoDB unavailable.
**Writer:** Go engine only (`engine/internal/persistence/store.go:87`)
**Reader:** Go engine only
**Client access:** NONE

---

## POSTGRESQL (Neon TimescaleDB)

**Collections:** Ledger events (EventOrderCreated, EventPositionOpened, etc.)
**Status:** ACTIVE — durable event store for OMS v3
**Writer:** Go engine only (`engine/internal/omsv3/authority.go`, `engine/internal/ledger/`)
**Reader:** Go engine for replay/recovery
**Client access:** NONE

---

## REDIS

**Status:** ACTIVE — indicator cache, performance cache
**Writer:** Go engine
**Client access:** NONE

---

## CONCLUSION

All trade, position, and PnL data is now written by a single authority: the Go institutional engine. Browser write paths have been blocked (HTTP 410) or stubbed (early returns). No duplicate write paths exist. The MongoDB collections `paper_trades`, `paper_state`, and `paper_positions` have exactly one authorized writer.

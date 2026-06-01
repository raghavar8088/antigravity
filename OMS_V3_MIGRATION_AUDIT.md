# OMS V3 Migration Audit — Phase 15A

**Date:** 2026-06-01  
**Engineer:** Principal Staff / Exchange OMS Architect  
**Scope:** Complete audit of order-state ownership across the Go engine

---

## Executive Summary

The engine contains **three independent order/position state stores** running simultaneously with no reconciliation between them. This is the dual-truth problem. OMS v3 must become the single authoritative source.

| System | Status | Owns State? | Problem |
|--------|--------|-------------|---------|
| `PaperClient` (execution/paper.go) | **Active** | Yes — `balanceUSD`, `positionBTC` | No SL/TP, no audit trail, non-deterministic |
| `positions.Manager` (positions/manager.go) | **Active** | Yes — `positions map[string]*Position` | Independent of fill events, no ledger link |
| `PaperOMS` (execution/paper_oms.go) | **REST only** | Yes — `openPositions`, `closedTrades` | Never called from orchestrator |
| `OMS v3 aggregate` (omsv3/aggregate.go) | **Partial** | No — events written but not read for state | Event log is write-only |
| `OMSManager` (oms/manager.go) | **Dead code** | Yes — orphaned | Never instantiated in main.go |
| `Reconciliation` (reconciliation/sync.go) | **Stub** | No | `internalPosition := 0.0` hardcoded |

---

## Phase 1 — Complete Order Creation Audit

### 1. Every location where orders are created

| File | Line | Method | Description |
|------|------|--------|-------------|
| `engine/internal/trading/loop.go` | 179 | `executeThroughInstitutionalPath()` | Creates `clientOrderID`, appends `EventOrderCreated` |
| `engine/internal/execution/paper_oms.go` | 263 | `PaperOMS.OpenPosition()` | Creates `PaperPosition`, generates `P{seq}` ID |
| `engine/internal/execution/paper_oms_handler.go` | 88 | `handleOpen()` | REST entry point → calls `PaperOMS.OpenPosition()` |

**Migration Action:** All order creation must flow through `trading/loop.go:executeThroughInstitutionalPath()` which already emits `EventOrderCreated`. The `PaperOMS.OpenPosition()` path via REST remains for the browser dashboard only (it is a separate paper desk, not the orchestrator).

---

### 2. Every location where positions are modified

| File | Line | Method | Mutation |
|------|------|--------|----------|
| `execution/paper.go` | 103 | `PaperClient.applyFill()` | `balanceUSD -= totalCost`; `positionBTC += size` |
| `execution/paper.go` | 116 | `PaperClient.applyFill()` | `balanceUSD += netRevenue`; `positionBTC -= size` |
| `execution/paper.go` | 183 | `PaperClient.SettlePosition()` | Reverse position, adjust `balanceUSD` |
| `positions/manager.go` | 111 | `Manager.OpenPosition()` | Creates `Position` in `positions` map |
| `positions/manager.go` | 199 | `checkLongPosition()` | Removes from `positions` map, emits CloseEvent |
| `positions/manager.go` | 220 | `checkShortPosition()` | Removes from `positions` map, emits CloseEvent |
| `positions/manager.go` | 286 | `CheckExpiredPositions()` | Force-closes stale positions |
| `execution/paper_oms.go` | 335 | `PaperOMS.openPositions` | Appends `PaperPosition` to slice |
| `execution/paper_oms.go` | 444 | `PaperOMS.openPositions` | Removes closed positions from slice |

**Migration Action (Phase 15A):** Emit `EventPositionOpened` / `EventPositionClosed` ledger events after every `positions.Manager` mutation. This enables OMS v3 to rebuild position state from events.

**Risk:** MEDIUM — positions.Manager remains authoritative during Stage 1 (dual-write). OMS v3 projections are additive.

---

### 3. Every location where order state is stored

| File | Storage | Description |
|------|---------|-------------|
| `ledger/store.go` | `MemoryStore.events []Event` | Canonical event log — write path already wired |
| `execution/paper_oms.go` | `PaperOMS.openPositions []*PaperPosition` | In-memory slice — not in ledger |
| `execution/paper_oms.go` | `PaperOMS.closedTrades []PaperClosedTrade` | In-memory slice — not in ledger |
| `positions/manager.go` | `Manager.positions map[string]*Position` | In-memory map — not in ledger |
| `execution/paper.go` | `PaperClient.positionBTC float64` | Single signed float — no history |
| `execution/paper.go` | `PaperClient.balanceUSD float64` | Running balance — no history |

**Migration Action:** The ledger `MemoryStore` must persist to PostgreSQL (Phase 14 TimescaleDB schema). Until then, all state is lost on restart. The existing `persistence/store.go` should be extended to replay ledger events on startup.

---

### 4. Every location where fills are processed

| File | Line | Method | Description |
|------|------|--------|-------------|
| `execution/paper.go` | 90 | `PaperClient.applyFill()` | Deducts notional + fee from balance, updates positionBTC |
| `execution/paper_oms.go` | 567 | `PaperOMS.closePosition()` | Calculates gross PnL, releases margin, deducts exit fee |
| `trading/loop.go` | 277 | `executeThroughInstitutionalPath()` | Calls `o.exec.ExecuteSignal()` → PaperClient.applyFill() |
| `trading/loop.go` | 959 | `processCloseEvents()` | Calls `o.exec.SettlePosition()` on SL/TP events |

**Migration Action:** All fills must emit `EventOrderFilled` (already done for entry fills). Position close fills must also emit `EventPositionClosed` via the new event emission helpers added to the orchestrator.

---

### 5. Every location where paper_oms.go is referenced

| File | Reference |
|------|-----------|
| `engine/cmd/antigravity/main.go` | `execution.NewPaperOMS(startingUSD)` — instantiated for REST API |
| `engine/cmd/antigravity/main.go` | `execution.PaperOMSHandler` — registered at `/paper/*` |
| `execution/paper_oms_handler.go` | HTTP handler → delegates to `PaperOMS` |
| `execution/paper_oms_test.go` | Unit tests for PaperOMS |

**Migration Action (Phase 4):** Rename `paper_oms.go` → `paper_exchange_adapter.go`. Strip state ownership. Keep the simulation logic (fill price calculation, SL/TP eval) as stateless methods. The REST endpoint `/paper/*` continues to serve the browser dashboard but routes through OMS v3.

---

### 6. Every location where OMS v3 is referenced

| File | Line | Usage |
|------|------|-------|
| `trading/loop.go` | 16 | `import "antigravity-engine/internal/omsv3"` |
| `trading/loop.go` | 216 | `omsv3.Replay(events)` — validates order aggregate on each new order |
| `omsv3/aggregate.go` | — | `OrderAggregate`, `Replay()`, `ApplyEvent()` |
| `omsv3/aggregate_test.go` | — | Unit tests for state machine transitions |

**Gap:** OMS v3 is only used for order lifecycle validation. It is NEVER used to reconstruct position state or to answer "what is my current open position?"

---

### 7. Every location where execution state can diverge

| Divergence Point | Description | Impact |
|-----------------|-------------|--------|
| `PaperClient.positionBTC` vs `positions.Manager` | PaperClient tracks net signed BTC; Manager tracks individual positions with SL/TP | Balance drift on restart |
| `PaperOMS` vs `PaperClient` | Completely independent — different position IDs, different balance tracking | Dashboard shows different PnL than orchestrator |
| `reconciliation/sync.go:internalPosition = 0.0` | Hardcoded zero — reconciliation always passes | Ghost positions undetected |
| `processCloseEvents` → `SettlePosition` | Duplicates exit logic without fee consistency | Fee double-counting risk |
| `MemoryStore` lost on restart | All event history is in-memory only | Full state loss on Render restart |

---

## Risk Assessment

| Issue | Severity | Impact |
|-------|----------|--------|
| Three independent position trackers | CRITICAL | Impossible to compute authoritative PnL |
| Reconciliation hardcoded to 0.0 | CRITICAL | All position drift goes undetected |
| MemoryStore not persisted | HIGH | Full state loss on every Render restart |
| PaperOMS isolated from orchestrator | HIGH | Dashboard PnL ≠ orchestrator PnL |
| OMSManager dead code | MEDIUM | Dead code inflates codebase |
| Non-deterministic PaperClient | MEDIUM | Same signal → different fill on replay |

---

## Files Requiring Migration Action

| File | Responsibility | Migration Action | Risk |
|------|---------------|-----------------|------|
| `execution/paper_oms.go` | Paper position simulation + state ownership | Phase 4: Rename to `paper_exchange_adapter.go`, strip state ownership | HIGH |
| `execution/paper.go` | Balance tracker | Phase 10: Remove, replace with OMS v3 balance projection | HIGH |
| `positions/manager.go` | Position SL/TP tracker | Phase 3: Emit OMS v3 position events on open/close | MEDIUM |
| `trading/loop.go` | Orchestrator | Phase 10: Replace direct PaperClient calls with OMS v3 Submit | HIGH |
| `reconciliation/sync.go` | Legacy reconciliation stub | Phase 9: Replace with `reconciliation/service.go` + LedgerSnapshotProvider | HIGH |
| `oms/manager.go` | Dead code | Phase 12 Stage 5: Delete | LOW |
| `ledger/store.go` | MemoryStore | Phase 14: Wire to PostgreSQL persistence | HIGH |

---

## New Files Created (Phase 15A)

| File | Purpose |
|------|---------|
| `engine/internal/omsv3/events.go` | Rich payload types for order + position events |
| `engine/internal/omsv3/position_aggregate.go` | Position lifecycle state machine (OPEN→REDUCED→CLOSED) |
| `engine/internal/omsv3/replay.go` | Unified replay for orders and positions by account |
| `engine/internal/omsv3/client_order_id.go` | Institutional client order ID generation |
| `engine/internal/omsv3/projections.go` | CQRS read models: Order, Position, PnL, Exposure |
| `engine/internal/omsv3/snapshot_provider.go` | `reconciliation.SnapshotProvider` backed by ledger |
| `engine/internal/execution/paper_exchange_adapter.go` | Stateless fill simulator (no state ownership) |

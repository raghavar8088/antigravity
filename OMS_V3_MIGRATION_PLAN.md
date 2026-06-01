# OMS V3 Migration Plan — Phase 15A

**Date:** 2026-06-01  
**Target:** OMS v3 is the only authoritative order and position state manager

---

## Migration Overview

The migration uses a 5-stage incremental approach. Each stage is independently deployable and independently verifiable. No stage requires a full restart or data migration unless explicitly noted.

---

## Stage 1 — Dual Write (Current State after Phase 15A) ✅

**What changed:**
- `FillResult.ClientOrderID` carries the OMS v3 order ID through all execution paths
- `Orchestrator.openAndTrackPosition()` replaces all `posMgr.OpenPosition()` call-sites
- `EventPositionOpened` emitted to ledger on every position open
- `EventPositionClosed` emitted to ledger on every position close (SL/TP/time/manual)
- `positionToOrderID map[string]string` links positions.Manager IDs to OMS v3 order IDs
- `LedgerSnapshotProvider` wires OMS v3 projections to `reconciliation.Service`

**Existing paths unchanged:**
- `positions.Manager` still owns SL/TP logic and emits `CloseEvents`
- `PaperClient` still owns `balanceUSD` and `positionBTC`
- Dashboard still reads from `PaperClient` and `TradeJournal`

**Verification:**
```bash
cd engine && go test -mod=mod ./internal/omsv3/... -v
# Check logs for: [OMS V3] emitPositionOpened / emitPositionClosed
```

**Rollback:** Remove the `openAndTrackPosition` calls — zero runtime impact since events are additive.

---

## Stage 2 — Shadow Mode

**Goal:** OMS v3 projections are queryable from the dashboard. Verify they match existing PaperClient state.

**Changes required:**
1. Add a new API endpoint: `GET /engine/oms/positions` → `BuildOpenPositionProjections(events)`
2. Add a new API endpoint: `GET /engine/oms/pnl` → `BuildPnLProjection(events)`
3. Wire `LedgerSnapshotProvider` to `reconciliation.Service` in `main.go`
4. Add a shadow check in `processCloseEvents`: compare OMS v3 NetPnLUSD vs TradeJournal NetPnL — log divergences
5. Expose `GET /engine/oms/exposure` → `BuildExposureProjection(events)`

**Verification:**
- Dashboard shows OMS v3 PnL alongside existing PaperClient PnL
- PnL values match within 0.01 USD tolerance
- Reconciliation service runs every 10s with no false alerts

**Rollback:** Disable the new API endpoints.

---

## Stage 3 — OMS v3 Primary

**Goal:** `positions.Manager` SL/TP events are driven by OMS v3 position projections, not by independent tracking.

**Changes required:**
1. Restore positions on startup from OMS v3 ledger replay (`ReplayOpenPositionsForAccount`)  
   instead of from the SQLite persistence store (which currently holds `positions.Manager` state)
2. Replace `PaperClient.positionBTC` with `BuildExposureProjection(events).NetExposure["BTC-USD"]`
3. Replace `PaperClient.balanceUSD` with initial capital minus `BuildExposureProjection.TotalNotionalUSD` plus `BuildPnLProjection.TotalPnLUSD`
4. Feed `positions.Manager.RestorePositions()` from OMS v3 replay on startup

**Verification:**
- Kill and restart the engine
- Verify positions are restored from ledger (not SQLite positions table)
- Run `go test ./internal/omsv3/... -run TestReplayPosition`

**Rollback:** Revert the startup sequence to restore from SQLite.

---

## Stage 4 — paper_oms.go Retired from Orchestrator Path

**Goal:** `PaperOMS` no longer handles any orchestrator executions. It becomes a pure browser dashboard endpoint.

**Changes required:**
1. The `/paper/*` REST endpoints (`PaperOMSHandler`) remain but are now backed by OMS v3:
   - `GET /paper/state` → `LedgerSnapshotProvider.Snapshot()` + convert to `OMSStateSnapshot`
   - `POST /paper/open` → emit `EventOrderCreated` + `EventPositionOpened` via OMS v3
   - `POST /paper/tick` → rebuild position projections from ledger, check SL/TP, emit `EventPositionClosed`
   - `POST /paper/reset` → append reset marker event to ledger
2. Remove `PaperOMS.openPositions`, `PaperOMS.closedTrades`, `PaperOMS.balanceUSD` as authoritative state
3. `paper_oms.go` becomes a thin HTTP adapter over OMS v3

**Verification:**
- Browser paper trading dashboard continues to work
- `/paper/state` returns positions derived from OMS v3 ledger
- PnL matches across both paths

**Rollback:** Revert `/paper/*` handlers to use original `PaperOMS` directly.

---

## Stage 5 — Legacy Code Removed

**Goal:** Delete all pre-OMS v3 state management code.

**Files to delete:**
| File | Why |
|------|-----|
| `execution/paper.go` (`PaperClient`) | Replaced by OMS v3 balance projection |
| `oms/manager.go` (`OMSManager`) | Dead code — never instantiated |
| `reconciliation/sync.go` | Stub with hardcoded 0.0 — replaced by `LedgerSnapshotProvider` |

**Files to simplify:**
| File | Change |
|------|--------|
| `execution/paper_oms.go` | Remove all state fields — keep only simulation helpers |
| `positions/manager.go` | Reduce to SL/TP evaluation only; remove `positions` map (state now in OMS v3) |

**Verification:**
- `go build ./...` passes
- All existing tests pass
- Full end-to-end paper trading test with 100 simulated ticks

**Rollback:** Restore from git. This stage is the point of no return.

---

## Risk Register

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| MemoryStore lost on restart | HIGH | HIGH | Persist ledger to PostgreSQL (Phase 14 schema) before Stage 3 |
| `emitPositionOpened` goroutine context cancelled | LOW | MEDIUM | Use `context.Background()` for ledger appends that must outlive request |
| positionToOrderID map unbounded growth | LOW | LOW | Map cleaned up on `EventPositionClosed`; bounded by max open positions |
| OMS v3 projection diverges from positions.Manager | MEDIUM | MEDIUM | Stage 2 shadow mode catches this; reconciliation service alerts on drift |
| Vendor directory out of sync with go.mod | HIGH | LOW | Use `-mod=mod` flag; run `go mod vendor` before deploying to Render |

---

## Pre-Deployment Checklist

- [ ] `go build -mod=mod ./...` passes
- [ ] `go test -mod=mod ./internal/omsv3/... ./internal/ledger/... ./internal/reconciliation/... ./internal/execution/...` all pass
- [ ] Verify `EventPositionOpened` appears in logs after first trade
- [ ] Verify `EventPositionClosed` appears in logs after SL/TP hit
- [ ] Verify `positionToOrderID` map is cleaned up after close (no growth)
- [ ] Reconciliation service logs no false CRITICAL alerts in 60s of operation
- [ ] Dashboard PnL unchanged (Stage 1 is additive — no dashboard changes)

---

## Remaining Blockers for Stage 3+

1. **MemoryStore persistence** — OMS v3 MemoryStore loses all events on Render restart. Must implement `PostgresStore` (Phase 14 TimescaleDB schema) before Stage 3.
2. **Idempotency on startup** — When restoring positions from ledger, ensure `EventPositionOpened` events from the previous session are not re-emitted.
3. **Kill switch ↔ OMS v3** — `killswitch.Service.Executor.FlattenPositions()` must emit `EventPositionClosed` for each flattened position. Currently it calls `positions.Manager.CloseAllPositions()` which emits `CloseEvents` — those are caught by `processCloseEvents` which now emits the ledger event. This chain already works in Stage 1.

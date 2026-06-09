# POSITION_CONSISTENCY_REPORT.md
## Phase 4 — Position Consistency Audit

**Audit Date:** 2026-06-09

---

## Position Consistency Matrix

| Layer | Source of Truth | Update Trigger | Persisted? | Restart Recovery |
|-------|----------------|----------------|------------|------------------|
| **Broker Position** | Delta REST `/v2/positions` | External (exchange) | N/A | Fetched on demand (reconciliationv2 only) |
| **OMS Position** | `positions.Manager` | `openAndTrackPosition` after fill | Mongo `paper_positions` | SQLite + Mongo (`recovery.go:122–131`) |
| **Portfolio Position** | `portfolioLedger` | `RecordClose` on exit | Mongo `paper_trades` | `BootstrapPortfolioLedgerFromMongo` (main.go:748) |
| **Risk Position** | `risk/engine.go` exposure | `NotifyFill` (loop.go:1674) | In-memory | Reconstructed from `posMgr` on boot |
| **PMS Position** | `pms` budget state | `syncPMSState` (loop.go:1678) | Postgres ledger events | Partial — events only if `DATABASE_URL` set |
| **Delta Bridge** | `LiveTrade` struct | `UpdateTradeAfterFill` (live_bridge.go:177) | In-memory | **Lost on restart** |

---

## Synchronization Paths

### Open (Entry Fill)

```
fillFn success
  → openAndTrackPosition (loop.go:815)
       → posMgr.OpenPosition (manager.go:126)
       → positionToOrderID[pos.ID] = fill.ClientOrderID (loop.go:824–826)
       → emitPositionOpened (loop.go:836)
       → persistPositionOpen (paperpersist_hooks.go:232)
       → o.risk.NotifyFill (loop.go:1674)
       → syncPMSState (loop.go:1678)
```

### Close (SL/TP — Software Only)

```
CheckStopLossAndTakeProfit (manager.go:192)
  → emitClose → CloseEvents channel
  → processCloseEvents (loop.go:1695)
       → SettlePosition (paper balance)
       → journal + portfolioLedger + emitPositionClosed
```

**No broker reduce-only order on SL/TP close for BTC paper path.**

### Delta Live Close

```
OnClose → institutionalClose handler (live_bridge.go:345)
  → executeThroughInstitutionalPathWithFill
  → bridge.SubmitReduceOnlyOrder (institutional_request.go:253)
  → UpdateTradeAfterClose (live_bridge.go:161)
```

---

## Drift Scenarios

| Scenario | Can Occur? | Evidence |
|----------|------------|----------|
| OMS position > broker position | **YES** (Delta) | Full fill assumed; partial exchange fill not tracked |
| Broker position > OMS position | **YES** (Delta) | Manual exchange orders not in OMS |
| OMS = Portfolio mismatch | **LOW** (paper) | Same close event drives both |
| Risk exposure stale | **YES** | `NotifyFill` on open only; close path uses tracker |
| Double position open | **LOW** | `MaxPerStrategy` gate (manager.go:120) |
| Duplicate fill booking | **YES** (theoretical) | No idempotency on fill events beyond order event keys |
| Race: fill before ack | **NO** | Sequential in `submitInstitutionalOrder` |
| Race: concurrent signals | **POSSIBLE** | Goroutine per strategy (loop.go:1350); aggregator dedupes |

---

## Reconciliation Coverage

Production reconciliation (`main.go:885–890`):
```
reconProvider := reconciliation.NewPaperSnapshotProvider(posMgr, "btc-paper-1")
```

`paper_provider.go:28–58` copies **identical** data to OMS and Exchange sides:

```go
// Same data copied into omsPositions AND exchPositions
```

**Position drift detection is structurally impossible in production.**

reconciliationv2 Delta adapter (`delta_reconciliation.go`) fetches real positions but is **not wired** in `main.go`.

---

## Position Manager Key Functions

| Function | File:Line | Purpose |
|----------|-----------|---------|
| `OpenPosition` | `manager.go:126` | Create with SL/TP levels |
| `CheckStopLossAndTakeProfit` | `manager.go:192` | Tick-level exit eval |
| `CheckExpiredPositions` | `manager.go:288` | Time-based force close |
| `CloseAllPositions` | `manager.go` (via killswitch) | Kill-switch flatten |
| `GetOpenPositions` | `manager.go` | Snapshot for reconciliation |
| `emitPartialTakeProfit` | `manager.go:277` | **DEAD CODE** — no callers |

---

## Position Consistency Verdict

| Question | Verdict |
|----------|---------|
| Broker ↔ OMS synchronized (paper) | **PASS** (same process) |
| Broker ↔ OMS synchronized (Delta live) | **FAIL** (no live comparison) |
| OMS ↔ Portfolio synchronized | **PARTIAL** (close path aligned; open unrealized may diverge) |
| OMS ↔ Risk synchronized | **PARTIAL** (NotifyFill on open; no explicit close notify in snippet) |
| Drift detection operational | **FAIL** (mirror provider) |
| Auto-repair on drift | **FAIL** (not wired) |

**Overall: FAIL for live capital** — no real broker position verification in production.

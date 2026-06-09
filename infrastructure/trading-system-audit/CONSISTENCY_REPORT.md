# Data Consistency Report

**Audit date:** 2026-06-09

---

## Consistency Domains

| Domain | Primary Store | Sync Mechanism |
|--------|---------------|----------------|
| Database (Mongo) | `loop_trades` paper trades/state | Direct writes |
| Database (SQLite) | Engine state | `persistence/store.go` |
| OMS (Go) | Ledger event store + Mongo transitions | Event append + replay |
| OMS (Client) | `paperOms.ts` + Mongo | In-memory + persist |
| Portfolio | `portfolioLedger`, `paper_state` | Close-time updates |
| Risk | `risk/v2`, `riskv3` aggregates | Fill/close notifications |
| Broker (Delta) | `Bridge.trades` in-memory | No DB sync |

---

## Concurrency Controls Found

| Mechanism | Location | Scope |
|-----------|----------|-------|
| `sync.Mutex` | `persistence/store.go` | All SQLite ops |
| `sync.RWMutex` | `killswitch/service.go` | Kill switch state |
| `sync.RWMutex` | `delta/live_bridge.go` | Bridge trades |
| `sync.Mutex` | `positions/manager.go` | Position map |
| `signalIDMu` | `loop.go:738` | Order→signal mapping |
| `o.mu` | `loop.go` | Last price |
| Atomic file write | `writeJSONAtomic`, `atomicWriteJson` | JSON snapshots |
| Worker lease | `mongoTradesClient.ts` `findOneAndUpdate` | Single worker |
| Ledger idempotency | `ledger/store.go:55–67` | Duplicate event rejection |

---

## Missing Consistency Controls

| Mechanism | Found? | Impact |
|-----------|--------|--------|
| Schema versioning | **NO** | Migration risk on restart |
| DB transactions (`sql.Tx`) | **NO** | Multi-table updates not atomic |
| Optimistic locking (version field) | **NO** | Concurrent writes can overwrite |
| Distributed lock (Redis) | **NO** | Multi-instance engine not safe |
| Two-phase commit (OMS + broker) | **NO** | Orphan orders possible |
| Event store + broker reconcile loop | **NO** (wired) | Silent divergence |

---

## Divergence Vectors (Proven)

### 1. PaperSnapshotProvider Self-Compare

`reconciliation/paper_provider.go:34–48` — OMS positions == exchange positions from same source.

**Silent divergence possible:** YES — broker state never compared.

### 2. Dual Strategy Runtimes

- Go engine: 606 strategies, `positions.Manager`, SQLite
- Client: 108 strategies, Mongo `paper_state`, worker/hook

**No code bridges these position stores.**

### 3. Synthetic ACK vs Broker State

`loop.go:671–673` — ledger shows acked before broker confirms.

**OMS can show FILLED while exchange shows open/partial.**

### 4. Bridge In-Memory vs Ledger

`live_bridge.go:68` — `trades []LiveTrade` not in SQLite restore.

**Engine restart loses Delta position mapping.**

### 5. Worker vs Hook PnL

Worker inline PnL vs hook `paperNetPnlOnClose` with `minAbsNetWinUsd`.

**Minor booking divergence possible.**

### 6. Client Portfolio Validation

`portfolioConsistencyValidation.ts` — detects Mongo vs state drift.

**Detect only — no auto-repair, not continuous.**

---

## Transaction Evidence

| Operation | Atomic? | Evidence |
|-----------|---------|----------|
| Ledger event append | Per-event | `store.Append` with idempotency key |
| Position open + order fill | **NO** | Separate steps in `openAndTrackPosition` |
| Position close + PnL + ledger | **NO** | Sequential in `processCloseEvents` goroutine |
| Mongo trade write | **PARTIAL** | Single document writes |
| SQLite state save | **YES** (single UPDATE) | Mutex-guarded |

---

## Version / Lock Search Results

```
Version  — no schema version on EngineState
Lock     — drawdown lock (client), profit lock (client), worker lease
Mutex    — persistence, positions, killswitch, bridge, loop
Atomic   — JSON file writes, worker lease acquire
Transaction — fee label only in client; no DB transactions
```

---

## Can Stores Diverge Silently?

| Pair | Can Diverge? | Detection? | Repair? |
|------|--------------|------------|---------|
| OMS ledger ↔ positions.Manager | Unlikely (same process) | Reconciliation (self-compare) | No |
| OMS ledger ↔ Delta exchange | **YES** | reconciliationv2 (not wired) | No |
| Mongo paper_state ↔ trades | **YES** | `portfolioConsistencyValidation` | No |
| SQLite ↔ in-memory on crash | **YES** (15s RPO) | None | Boot restore |
| Bridge.trades ↔ exchange | **YES** | 5min monitor (partial) | No |
| Go engine ↔ Client paper desk | **YES** (by design) | None | No |

---

## Phase 11 Conclusion

| Requirement | Verdict |
|-------------|---------|
| Database/OMS/Portfolio/Risk/Broker cannot diverge silently | **FAIL** |
| Concurrency safety within single engine process | **PARTIAL PASS** |
| Multi-instance safety | **FAIL** |
| Cross-store consistency guarantees | **FAIL** |
| Automated drift detection (live) | **FAIL** |
| Automated drift repair | **FAIL** |

**Overall Phase 11:** **FAIL**

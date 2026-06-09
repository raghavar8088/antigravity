# RECOVERY_CERTIFICATION.md
## Phase 6 — Restart Recovery Audit

**Audit Date:** 2026-06-09  
**Verdict: PARTIAL**

---

## Simulated Scenario

```
Open Position → Engine Crash → Restart → Recovery → Trading Continues
```

---

## Boot Recovery Sequence

| Step | Mechanism | File:Function:Line | Restores |
|------|-----------|-------------------|----------|
| 1 | SQLite load | `persistence/store.go:LoadState` (224–251) | balance, positions JSON, trades, stats |
| 2 | Position restore | `main.go` → `posMgr.RestorePositions` (~537–560) | open positions |
| 3 | Mongo recover | `paperpersist/recovery.go:Recover` (90–155) | `paper_state`, `paper_positions` |
| 4 | Mongo override | `main.go` (~621–624) | balance if newer than SQLite |
| 5 | Position override | `main.go` (~636) | positions only if `posMgr.GetPositionCount() == 0` |
| 6 | Journal hydrate | `paperpersist/journal_bootstrap.go:BootstrapJournalFromMongo` (22–49) | up to 500 closed trades |
| 7 | Portfolio ledger | `main.go:748–756` | PnL from `paper_trades` |
| 8 | Position re-register | `main.go:761–762` | OMS close mapping |

### Periodic Persistence

| Writer | Interval | File |
|--------|----------|------|
| SQLite StateSaver | 15s | `persistence/saver.go:38–54` |
| Mongo StateSnapshotter | 10s | `paperpersist/state_snapshotter.go:87–109` |

**RPO window: 10–15 seconds** of in-flight state may be lost.

---

## What Is Lost on Restart

| Structure | Lost? | Evidence |
|-----------|-------|----------|
| Orchestrator event ledger (order events) | **YES** | `loop.go:232` — `ledger.NewMemoryStore()`; `SetEventLedger` never called from `main.go` |
| OMS v3 aggregate state | **YES** | No `omsv3.ReplayAll` call in `cmd/` |
| Kill-switch active flag | **YES** | `killswitch.NewService` starts `active=false` (service.go:49–56); events persist but not replayed |
| Delta `LiveTrade` bridge state | **YES** | In-memory only (`live_bridge.go`) |
| `paper_orders` OMS transitions | **YES** | Documented in `recovery.go:7–8` but `Recover()` only loads account + positions (L104–131) |
| Strategy indicator buffers | **YES** | Re-warmed from Coinbase REST (main.go ~851–872) |
| Pending signals queue | **YES** | In-memory orchestrator state |
| In-flight orders (submitted, not filled) | **YES** | No order recovery |

---

## What Persists

| Structure | Persists? | Evidence |
|-----------|-----------|----------|
| Paper balance | **YES** | SQLite + Mongo |
| Open positions (size, SL, TP) | **YES** | SQLite + Mongo `paper_positions` |
| Closed trade history | **YES** | SQLite trades + Mongo `paper_trades` |
| Portfolio metrics | **YES** | Mongo bootstrap |
| Kill-switch audit events | **PARTIAL** | Postgres if `DATABASE_URL` set — audit only, not enforced state |
| PMS ledger events | **PARTIAL** | Postgres append-only |
| BTC options state | **YES** | SQLite columns or `FileSnapshotStore` JSON |

---

## What Is Reconstructed (Not Restored)

| Structure | Method | Complete? |
|-----------|--------|-----------|
| Risk exposure | From restored positions | **PARTIAL** |
| Strategy tracker stats | From journal/Mongo trades | **PARTIAL** |
| OMS order lifecycle | Not reconstructed | **NO** |
| Exchange order correlation | Not reconstructed | **NO** |

---

## Unused Recovery Infrastructure

| Module | File | Status |
|--------|------|--------|
| `ledger.Bootstrap` | `ledger/bootstrap.go:77–148` | Tests only |
| `omsv3.ReplayAll` | `omsv3/replay_engine.go:28` | Not called from `cmd/` |
| `ha.RecoveryEngine.Recover` | `ha/recovery_engine.go:133` | Not wired |
| `reconciliationv2.ValidateCrashRecovery` | `reconciliationv2/authority.go:161–170` | Not wired |
| `backup.RestoreManager` | `backup/restore_manager.go:79+` | Not wired |

---

## Recovery Precedence Conflict

```
1. SQLite restores first (if balance != initial)
2. Mongo may override balance
3. Mongo positions only if posMgr.GetPositionCount() == 0
```

**Risk:** If SQLite has stale positions and Mongo has different ones, Mongo positions are **skipped** (main.go ~636).

---

## Recovery Certification Matrix

| Component | Recovered? | Verdict |
|-----------|------------|---------|
| Open positions | Yes (paper) | **PASS** |
| PnL / balance | Yes | **PASS** |
| OMS order state | No | **FAIL** |
| Risk state | Partial reconstruct | **PARTIAL** |
| Portfolio state | Yes (from Mongo) | **PASS** |
| Broker state (Delta) | No | **FAIL** |
| Kill-switch enforcement | No | **FAIL** |
| Ledger event replay | No | **FAIL** |

---

## Recovery Verdict: **PARTIAL**

Paper desk can resume trading after crash with ~10–15s data loss. Institutional guarantees (OMS replay, kill-switch persistence, broker correlation, order recovery) are **not production-ready** for live capital.

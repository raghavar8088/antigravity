# Failure Recovery Report

**Audit date:** 2026-06-09

---

## Recovery Mechanisms Inventory

| Failure Mode | Mechanism | File | Auto-Repair | Verdict |
|--------------|-----------|------|-------------|---------|
| Broker outage | Kill switch block | `institutional_request.go:16` | No — blocks new orders | **PARTIAL** |
| Exchange outage | Kill switch `TriggerExchangeOutage` | `killswitch/service.go` | Flatten only | **PARTIAL** |
| Engine restart | SQLite `LoadState` + restore | `persistence/store.go`, `main.go:536–565` | Boot restore | **PARTIAL** |
| Database restart | SQLite WAL + busy_timeout | `persistence/store.go:87–100` | Reconnect on open | **PARTIAL** |
| Redis restart | Not in engine hot path | — | — | **N/A** |
| WebSocket disconnect | Coinbase `keepConnected` | `marketdata/coinbase.go:41–60` | 5s retry | **PASS** (MD only) |
| Network interruption | `fetchWithRetry` backoff | `futuresKlinesFetch.ts:22–44` | Client klines only | **PARTIAL** |
| Duplicate fills | Ledger idempotency keys | `ledger/store.go:55–67` | Dedup on append | **PARTIAL** |
| Partial fills | Not handled live | — | — | **FAIL** |
| Missed fills | Not detected live | — | — | **FAIL** |
| Crash during execution | StateSaver 15s periodic | `persistence/saver.go` | Point-in-time restore | **PARTIAL** |

---

## Engine Restart Recovery

### Boot Sequence (`main.go`)

1. `persistence.LoadState` → balance, positions, trades
2. `posMgr.RestorePositions` — rehydrate open positions
3. `exec.RestoreBalance` — paper account balance
4. `WarmupStrategies` — feed historical candles (`loop.go:924–964`)

### Gaps

| Gap | Evidence |
|-----|----------|
| No OMS event replay on boot | `ledger.ReplayEverything` exists but not called in `main.go` boot |
| No broker position query on boot | `PaperSnapshotProvider` uses in-memory posMgr only |
| RPO | StateSaver 15s interval — up to 15s event loss |
| RTO | No automated failover documented in code |

**Verdict:** **PARTIAL** — state restore works for paper; no broker reconciliation on restart.

---

## Kill Switch Recovery

**File:** `engine/internal/killswitch/service.go`, `killswitch_executor.go`

| Action | Implementation |
|--------|----------------|
| Cancel orders | `CancelOpenOrders` — closes all positions at last price |
| Flatten | `FlattenPositions` → `ExecuteEmergencyFlatten` institutional path |
| Block new orders | `ProcessExecutionRequest` checks `killSvc.IsActive()` |
| Alert | `SendAlert` — log only, no Slack/PagerDuty |
| Release | Manual `Release()` only — never self-releases |

**Verdict:** **PASS** for containment; **FAIL** for automated recovery after release (no broker sync).

---

## Paper Desk Recovery (Client)

| Mechanism | File | Behavior |
|-----------|------|----------|
| Mongo state persistence | `mongoTradesClient.ts` | Trades + paper_state |
| Atomic worker lease | `findOneAndUpdate` lease | Prevents dual worker |
| Portfolio consistency check | `portfolioConsistencyValidation.ts` | Detect drift |
| Self-healing recommendations | `deskSelfHealing.ts` | Suggest restart — no auto-execute |
| Replay validation | `futuresReplayEngine.ts` | Offline go-live gate |

**Verdict:** **PARTIAL** — persistence exists; no automated repair.

---

## paperpersist Recovery (Adjacent)

**File:** `engine/internal/paperpersist/recovery.go`

- `RecoveryReport` type for crash recovery diagnostics
- Not proven wired in `main.go` boot path

---

## reconciliationv2 Repair (Not Wired)

**File:** `engine/internal/reconciliationv2/repair_engine.go`

| Repair Type | Capability |
|-------------|------------|
| `RepairTypeReplay` | Rebuild from ledger events |
| `RepairTypeStateSync` | Sync state from broker snapshot |

**Grep in `main.go`:** No matches for `reconciliationv2`.

**Verdict:** **FAIL** — repair code exists but not production-booted.

---

## Retry / Backoff Evidence

| Location | Pattern |
|----------|---------|
| `futuresKlinesFetch.ts` | Exponential backoff + jitter |
| `coinbase.go` | 5s sleep between reconnect attempts |
| `useLiveBTCMarket.ts` | WS reconnect with `RECONNECT_MAX_MS` |
| Go Delta client | 10s HTTP timeout only — no retry |
| Go engine StateSaver | Log on failure — no retry |

---

## Snapshot / Restore

| Store | Method | Atomic |
|-------|--------|--------|
| SQLite | `SaveState` / `LoadState` | `sync.Mutex` |
| JSON files | `writeJSONAtomic` | tmp + rename |
| Client localStorage | `atomicWriteJson` | tmp + rename |
| Ledger | `TakeSnapshot` | Event-sourced |

**No schema versioning** on persisted snapshots — migration risk on restart.

---

## Phase 7 Conclusion

| Scenario | Can Recover Without Capital Risk? | Verdict |
|----------|-----------------------------------|---------|
| Engine crash (paper) | Partial — 15s RPO, no broker check | **PARTIAL** |
| Engine crash (delta live) | No — orphan exchange positions possible | **FAIL** |
| Broker disconnect mid-order | No — no order status polling | **FAIL** |
| Duplicate fill | Partial — ledger dedup only | **PARTIAL** |
| Missed fill | No | **FAIL** |
| Partial fill | No | **FAIL** |
| Kill switch activation | Yes — flatten via institutional path | **PASS** |

**Overall Phase 7:** **FAIL** for live capital recovery; **PARTIAL** for paper-only operation.

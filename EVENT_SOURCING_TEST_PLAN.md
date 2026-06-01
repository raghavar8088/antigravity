# Event Sourcing Test Plan — Phase 15B
*Target: 95% coverage for ledger, omsv3, reconciliation*

---

## Test Files to Create

| File | Package | Priority |
|------|---------|----------|
| `engine/internal/ledger/replay_test.go` | ledger | P0 |
| `engine/internal/ledger/snapshots_test.go` | ledger | P0 |
| `engine/internal/omsv3/replay_engine_test.go` | omsv3 | P0 |
| `engine/internal/omsv3/strategy_aggregate_test.go` | omsv3 | P1 |
| `engine/internal/omsv3/system_aggregate_test.go` | omsv3 | P1 |
| `engine/internal/execution/paper_oms_ledger_test.go` | execution | P0 |
| `engine/internal/risk/engine_ledger_test.go` | risk | P1 |
| `engine/internal/killswitch/release_test.go` | killswitch | P1 |

---

## Test Suite 1 — 100k Event Replay

**File:** `engine/internal/ledger/replay_test.go`
**Target:** `ReplayEverything`, `ReplayOrders`, `ReplayPositions`

```
TestReplay100kEvents
  Setup:
    store := NewMemoryStore()
    Generate 100,000 events across ORDER + POSITION + RISK aggregates
    for 10,000 distinct order IDs (10 events each: CREATED → FILLED → POSITION_OPENED → CLOSED + risk + SL events)
    Append all to store

  Assert:
    ReplayEverything returns 100,000 events with zero error
    ReplayOrders returns 10,000 distinct order IDs
    ReplayPositions returns 10,000 distinct position IDs
    Every order aggregate replays to terminal state (FILLED or CLOSED)
    VerifySequence passes for every aggregate's event slice
    TotalEventCount == 100,000

  Performance:
    t.Logf elapsed — target < 2 seconds on dev hardware
    Fail if elapsed > 5 seconds
```

---

## Test Suite 2 — 1M Event Replay

**File:** `engine/internal/ledger/replay_test.go`

```
TestReplay1MEvents (requires -run=TestReplay1M -count=1)
  Same structure as 100k but 1,000,000 events
  100,000 distinct orders × 10 events each

  Performance target: < 30 seconds
  Memory target: < 2 GB RSS (measure with runtime.ReadMemStats)
```

---

## Test Suite 3 — Duplicate Event Detection

**File:** `engine/internal/ledger/replay_test.go`

```
TestDuplicateEventReplay
  Create 10 events with same EventID
  Append 10 unique events + 5 with duplicate EventIDs
  Assert store.Append returns ErrDuplicateEvent on each duplicate
  Assert DeduplicateEvents(slice) removes 5 duplicates
  Assert final event count == 10
```

---

## Test Suite 4 — Out-of-Order Detection

**File:** `engine/internal/ledger/replay_test.go`

```
TestOutOfOrderDetection
  Create 20 events with timestamps deliberately out of creation-time order
  Call DetectOutOfOrder(events)
  Assert returns non-empty indices identifying disordered events
  Assert the indices match the known positions where timestamps regress
```

---

## Test Suite 5 — Crash Recovery

**File:** `engine/internal/omsv3/replay_engine_test.go`

```
TestCrashRecovery
  Simulate engine state:
    Open 50 positions via PaperOMS.OpenPosition (with ledger wired)
    Close 20 of them via PaperOMS.Tick
    Trigger kill switch

  Simulate crash: discard all in-memory OMS state (reset PaperOMS)

  Recovery:
    result, err := ReplayAll(ctx, store, accountID)
    Assert err == nil
    Assert len(result.Positions) == 50
    open := ReplayOpenPositions(ctx, store, accountID)
    Assert len(open) == 30  (50 opened - 20 closed)
    Assert result.System is non-nil

  Cross-check:
    Build DashboardProjection from store.ReplayAccount events
    Assert PnL.TotalTrades == 20
    Assert Exposure.OpenPositions == 30
```

---

## Test Suite 6 — Snapshot Save + Restore

**File:** `engine/internal/ledger/snapshots_test.go`

```
TestSnapshotSaveAndRestore
  Setup:
    store := NewMemoryStore()
    snapStore := NewMemorySnapshotStore()
    replayStore := NewReplayStore(store, snapStore)

  Phase 1 — before snapshot:
    Append 100 events for account "test-account"
    Call replayStore.TakeSnapshot(ctx, "test-account")
    snap, err := snapStore.Latest(ctx, "test-account")
    Assert snap.GlobalSequence == 100
    Assert snap.AccountID == "test-account"

  Phase 2 — incremental events:
    Append 50 more events
    snap2, delta, err := replayStore.ReplayFromSnapshot(ctx, "test-account")
    Assert snap2.GlobalSequence == 100
    Assert len(delta) == 50

  Phase 3 — verify full replay == snapshot + delta:
    all, _ := store.ReplayAccount(ctx, "test-account")
    Assert len(all) == 150
    Assert len(delta) == 50

TestSnapshotNoSnapshotReturnsAllEvents
  No snapshot saved → ReplayFromSnapshot returns all 200 events and empty Snapshot
```

---

## Test Suite 7 — Reconciliation Mismatch + Resolve Round-Trip

**File:** `engine/internal/reconciliation/service_resolve_test.go`

```
TestReconciliationResolvedEvent
  Setup:
    store := NewMemoryStore()
    svc := NewService(provider, store, 10*time.Second)

  Step 1 — inject mismatch:
    svc.Check(ctx) → generates 1 ReconciliationAlert event

  Step 2 — resolve:
    err := svc.Resolve(ctx, "test-account", "ref-001", "MISSING_FILL",
      "OMS:FILLED", "EXCHANGE:UNKNOWN", "FORCE_RECONCILE", "OK", "auto-repair")
    Assert err == nil

  Step 3 — verify ledger:
    events, _ := store.ReplayAccount(ctx, "test-account")
    resolved := filter events by EventType == EventReconciliationResolved
    Assert len(resolved) == 1
    Unmarshal payload → ReconciliationResolvedPayload
    Assert payload.AlertReference == "ref-001"
    Assert payload.RepairResult == "OK"
    Assert payload.ResolvedBy == "auto-repair"
```

---

## Test Suite 8 — Kill Switch Replay

**File:** `engine/internal/killswitch/release_test.go`

```
TestKillSwitchTriggerAndReleaseReplay
  store := NewMemoryStore()
  svc := NewService(store, nil, "test-account")

  Trigger:
    svc.Trigger(ctx, Activation{Trigger: TriggerDailyLoss, Reason: "max loss"})
    Assert svc.IsActive() == true

  Release:
    svc.Release(ctx, TriggerDailyLoss, "operator", "manually cleared")
    Assert svc.IsActive() == false

  Verify ledger:
    events, _ := store.ReplayAccount(ctx, "test-account")
    triggered := filter by EventType == EventKillSwitchTriggered
    released := filter by EventType == EventKillSwitchReleased
    Assert len(triggered) == 1
    Assert len(released) == 1
    Unmarshal released payload → KillSwitchReleasedPayload
    Assert payload.OriginalTrigger == "DAILY_LOSS_BREACH"
    Assert payload.ReleasedBy == "operator"
```

---

## Test Suite 9 — Projection Rebuild

**File:** `engine/internal/omsv3/replay_engine_test.go`

```
TestProjectionRebuildFromReplay
  store := NewMemoryStore()
  oms := NewPaperOMS(100_000).WithLedger(store, "test-account")

  Open 100 positions, tick-close 40 of them with profits, 30 at loss

  Rebuild projections from scratch:
    events, _ := store.ReplayAccount(ctx, "test-account")
    dash := BuildDashboardProjection(events)

  Assert:
    dash.PnL.TotalTrades == 70           (40 profit + 30 loss)
    dash.PnL.Wins == 40
    dash.PnL.Losses == 30
    dash.PnL.WinRate == 0.571 (approx)
    dash.Exposure.OpenPositions == 30    (100 opened - 70 closed)
    len(dash.OpenOrders) >= 0            (paper orders are terminal)

TestProjectionIsIdenticalOnEveryRebuild
  Build projection 3 times from the same event log
  Assert all 3 results are byte-for-byte equal (determinism)
```

---

## Test Suite 10 — StrategyAggregate State Machine

**File:** `engine/internal/omsv3/strategy_aggregate_test.go`

```
TestStrategyStateMachineHappyPath
  Events: ENABLED → PAUSED → RESUMED → PROMOTED → DISABLED
  Assert each state after each event
  Assert IsActive() == false when DISABLED
  Assert IsActive() == true when ENABLED, RESUMED, PROMOTED

TestStrategyStateMachineRejectsInvalidTransition
  State: DISABLED
  Apply PAUSED event
  Assert ErrInvalidTransition returned

TestStrategyAllocationChangeDoesNotChangeState
  State: ENABLED
  ApplyAllocationChange with payload.NewAllocation = 50_000
  Assert State still == ENABLED
  Assert Allocation == 50_000

TestReplayStrategyFromEvents
  Build 6 events (full lifecycle)
  ReplayStrategy(events)
  Assert final state matches expected
```

---

## Coverage Targets

| Package | Current Est. | Target |
|---------|-------------|--------|
| `ledger` | ~45% | 95% |
| `omsv3` | ~35% | 95% |
| `reconciliation` | ~60% | 95% |
| `killswitch` | ~40% | 80% |
| `execution` (paper) | ~20% | 70% |
| `risk` | ~15% | 60% |

Run coverage:
```bash
cd engine && go test -mod=mod -coverprofile=coverage.out \
  ./internal/ledger/... \
  ./internal/omsv3/... \
  ./internal/reconciliation/... \
  ./internal/killswitch/... \
  ./internal/execution/...

go tool cover -html=coverage.out -o coverage.html
```

---

## Performance Benchmarks

```go
// BenchmarkReplay100k — target: < 50ms on dev hardware
func BenchmarkReplay100k(b *testing.B) {
    store := buildFilledStore(100_000)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = ledger.ReplayEverything(context.Background(), store, "bench-account")
    }
}

// BenchmarkProjectionRebuild — target: < 20ms for 100k events
func BenchmarkProjectionRebuild(b *testing.B) {
    events := buildEventSlice(100_000)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = omsv3.BuildDashboardProjection(events)
    }
}
```

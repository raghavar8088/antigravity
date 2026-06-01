# OMS V3 Integration Tests — Phase 15A

**Target coverage:** 90%+ for `engine/internal/omsv3/...`

---

## Test Suite Structure

```
engine/internal/omsv3/
  aggregate_test.go         (existing — ORDER state machine transitions)
  position_aggregate_test.go (NEW — POSITION state machine transitions)
  projections_test.go        (NEW — CQRS read model builders)
  replay_test.go             (NEW — account replay helpers)
  client_order_id_test.go    (NEW — ID generation + parse)
  snapshot_provider_test.go  (NEW — reconciliation snapshot)
```

---

## 1. Order Lifecycle Tests

### 1a. Happy Path: NEW → VALIDATED → RISK_APPROVED → SUBMITTED → ACKNOWLEDGED → FILLED

```go
func TestOrderAggregate_HappyPath(t *testing.T) {
    store := ledger.NewMemoryStore()
    ctx := context.Background()

    clientID := "BTC-20260601-143022-a3f7bc92"

    appendEvent := func(eventType ledger.EventType, payload any) {
        event, _ := ledger.NewEvent(ledger.NewEventInput{
            AggregateType: ledger.AggregateOrder,
            AggregateID:   clientID,
            EventType:     eventType,
            Payload:       payload,
            Source:        "test",
        })
        store.Append(ctx, event)
    }

    appendEvent(ledger.EventOrderCreated, ledger.OrderPayload{
        ClientOrderID: clientID, Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.01,
    })
    appendEvent(ledger.EventOrderValidated, ledger.OrderPayload{ClientOrderID: clientID})
    appendEvent(ledger.EventRiskApproved, ledger.OrderPayload{ClientOrderID: clientID})
    appendEvent(ledger.EventOrderSubmitted, ledger.OrderPayload{ClientOrderID: clientID})
    appendEvent(ledger.EventOrderAcked, ledger.OrderPayload{
        ClientOrderID: clientID, ExchangeOrderID: "PAPER-BTCUSDT-12345-1",
    })
    appendEvent(ledger.EventOrderFilled, ledger.OrderPayload{
        ClientOrderID: clientID, FillPrice: 65000.0, FillQuantity: 0.01,
    })

    events, _ := store.Replay(ctx, ledger.AggregateOrder, clientID)
    agg, err := omsv3.Replay(events)

    assert(t, err == nil)
    assert(t, agg.State == omsv3.StateFilled)
    assert(t, agg.FilledQuantity == 0.01)
    assert(t, agg.AverageFillPrice == 65000.0)
    assert(t, agg.ExchangeOrderID == "PAPER-BTCUSDT-12345-1")
}
```

### 1b. Partial Fills: NEW → SUBMITTED → PARTIALLY_FILLED → PARTIALLY_FILLED → FILLED

```go
func TestOrderAggregate_PartialFills(t *testing.T) {
    // Two partial fills of 0.005 BTC at different prices → average price
    // First: 0.005 @ 65000 = 325 USD
    // Second: 0.005 @ 65100 = 325.5 USD
    // Average: (325 + 325.5) / 0.01 = 65050 USD
    // ...
    assert(t, agg.State == omsv3.StateFilled)
    assert(t, math.Abs(agg.AverageFillPrice - 65050.0) < 0.01)
    assert(t, agg.FilledQuantity == 0.01)
}
```

### 1c. Cancellation: NEW → VALIDATED → ACCEPTED → CANCELLED

```go
func TestOrderAggregate_Cancellation(t *testing.T) {
    // ...
    assert(t, agg.State == omsv3.StateCancelled)
}
```

### 1d. Risk Block: NEW → VALIDATED → RISK_BLOCKED → REJECTED

```go
func TestOrderAggregate_RiskBlocked(t *testing.T) {
    // Risk pipeline blocks the order
    assert(t, agg.State == omsv3.StateRejected)
}
```

### 1e. Forbidden Transitions Must Fail

```go
func TestOrderAggregate_ForbiddenTransitions(t *testing.T) {
    tests := []struct {
        from omsv3.OrderState
        to   omsv3.OrderState
    }{
        {omsv3.StateFilled, omsv3.StateNew},
        {omsv3.StateFilled, omsv3.StateSubmitted},
        {omsv3.StateCancelled, omsv3.StateFilled},
        {omsv3.StateRejected, omsv3.StateValidated},
        {omsv3.StateNew, omsv3.StateFilled}, // must go through intermediate states
    }
    for _, tt := range tests {
        // Build aggregate at `from` state, attempt transition to `to`
        // Expect: err == omsv3.ErrInvalidTransition
        assert(t, errors.Is(err, omsv3.ErrInvalidTransition))
    }
}
```

---

## 2. Position Lifecycle Tests

### 2a. Happy Path: OPEN → CLOSED (Take Profit)

```go
func TestPositionAggregate_TP(t *testing.T) {
    store := ledger.NewMemoryStore()
    ctx := context.Background()
    posID := "POS-143022-1"

    appendPos := func(eventType ledger.EventType, payload any) {
        event, _ := ledger.NewEvent(ledger.NewEventInput{
            AggregateType: ledger.AggregatePosition,
            AggregateID:   posID,
            EventType:     eventType,
            Payload:       payload,
            Source:        "test",
        })
        store.Append(ctx, event)
    }

    appendPos(ledger.EventPositionOpened, omsv3.PositionOpenedPayload{
        PositionID:    posID,
        ClientOrderID: "BTC-20260601-143022-a3f7bc92",
        Symbol:        "BTC-USD",
        Side:          "BUY",
        EntryPrice:    65000.0,
        Quantity:      0.01,
        NotionalUSD:   650.0,
        StopLoss:      64000.0,
        TakeProfit:    66000.0,
        StopLossPct:   1.5,
        TakeProfitPct: 1.5,
        StrategyName:  "EMA_Cross_Scalp",
    })
    appendPos(ledger.EventPositionClosed, omsv3.PositionClosedPayload{
        PositionID:    posID,
        ClientOrderID: "BTC-20260601-143022-a3f7bc92",
        Symbol:        "BTC-USD",
        Side:          "BUY",
        EntryPrice:    65000.0,
        ExitPrice:     66000.0,
        Quantity:      0.01,
        NotionalUSD:   650.0,
        GrossPnLUSD:   10.0,
        NetPnLUSD:     9.35,
        FeesUSD:       0.65,
        ExitReason:    "TP",
        HoldMinutes:   12.5,
    })

    events, _ := store.Replay(ctx, ledger.AggregatePosition, posID)
    agg, err := omsv3.ReplayPosition(events)

    assert(t, err == nil)
    assert(t, agg.State == omsv3.PositionStateClosed)
    assert(t, agg.ExitReason == "TP")
    assert(t, math.Abs(agg.NetPnLUSD - 9.35) < 0.001)
}
```

### 2b. Partial Close: OPEN → REDUCED → CLOSED

```go
func TestPositionAggregate_PartialClose(t *testing.T) {
    // EventPositionChanged (reduce from 0.01 to 0.005)
    // EventPositionClosed  (close remaining 0.005)
    assert(t, agg.State == omsv3.PositionStateClosed)
}
```

### 2c. Kill Switch Flatten: OPEN → CLOSED (KILL_SWITCH)

```go
func TestPositionAggregate_KillSwitch(t *testing.T) {
    // EventPositionClosed with ExitReason = "KILL_SWITCH"
    assert(t, agg.ExitReason == "KILL_SWITCH")
    assert(t, agg.State == omsv3.PositionStateClosed)
}
```

### 2d. Forbidden Position Transitions Must Fail

```go
func TestPositionAggregate_ForbiddenTransitions(t *testing.T) {
    tests := []struct {
        from omsv3.PositionState
        to   omsv3.PositionState
    }{
        {omsv3.PositionStateClosed, omsv3.PositionStateOpen},
        {omsv3.PositionStateClosed, omsv3.PositionStateReduced},
        {omsv3.PositionStateEmpty, omsv3.PositionStateClosed}, // must open first
    }
    for _, tt := range tests {
        assert(t, errors.Is(err, omsv3.ErrInvalidTransition))
    }
}
```

---

## 3. Reconciliation Recovery Tests

### 3a. Exchange Reports FILLED, OMS Reports PARTIAL

```go
func TestReconciliation_MissingFill(t *testing.T) {
    oms := []reconciliation.OMSOrder{
        {
            ClientOrderID:  "BTC-20260601-143022-a3f7bc92",
            Symbol:         "BTCUSDT",
            State:          reconciliation.OrderStatePartiallyFilled,
            Quantity:       0.01,
            FilledQuantity: 0.005,
        },
    }
    exchange := []reconciliation.ExchangeOrder{
        {
            ClientOrderID:  "BTC-20260601-143022-a3f7bc92",
            Symbol:         "BTCUSDT",
            FilledQuantity: 0.01, // exchange is ahead
        },
    }
    detector := reconciliation.OrderMismatchDetector{StaleAfter: 30 * time.Second}
    alerts := detector.Detect(oms, exchange, time.Now())

    assert(t, len(alerts) == 1)
    assert(t, alerts[0].Type == reconciliation.AlertMissingFill)
    assert(t, alerts[0].Severity == reconciliation.SeverityCritical)
}
```

### 3b. Ghost Order — OMS Has Live Order Missing at Exchange

```go
func TestReconciliation_GhostOrder(t *testing.T) {
    oms := []reconciliation.OMSOrder{
        {
            ClientOrderID:   "BTC-20260601-143022-deadbeef",
            ExchangeOrderID: "PAPER-BTCUSDT-99999-1",
            State:           reconciliation.OrderStateSubmitted,
            UpdatedAt:       time.Now().Add(-60 * time.Second),
        },
    }
    exchange := []reconciliation.ExchangeOrder{} // empty — exchange has no record

    detector := reconciliation.OrderMismatchDetector{StaleAfter: 30 * time.Second}
    alerts := detector.Detect(oms, exchange, time.Now())

    assert(t, len(alerts) == 1)
    assert(t, alerts[0].Type == reconciliation.AlertGhostOrder)
}
```

### 3c. Position Drift — OMS 0.01 BTC, Exchange 0.02 BTC

```go
func TestReconciliation_PositionDrift(t *testing.T) {
    oms := []reconciliation.OMSPosition{{Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.01}}
    exchange := []reconciliation.ExchangePosition{{Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.02}}

    detector := reconciliation.PositionDriftDetector{ToleranceBTC: 1e-8}
    alerts := detector.Detect(oms, exchange, time.Now())

    assert(t, len(alerts) == 1)
    assert(t, alerts[0].Type == reconciliation.AlertPositionDrift)
    assert(t, alerts[0].Severity == reconciliation.SeverityCritical)
}
```

### 3d. System Repairs Itself — Projection Rebuilt from Ledger After Drift

```go
func TestReconciliation_SelfRepair(t *testing.T) {
    store := ledger.NewMemoryStore()
    ctx := context.Background()

    // 1. Append EventPositionOpened (simulates position open)
    // 2. Introduce simulated state drift (modify positions.Manager directly)
    // 3. Rebuild projections from ledger
    // 4. Verify projections match ledger, not drifted state

    events, _ := store.ReplayAccount(ctx, "btc-paper-1")
    openPos := omsv3.BuildOpenPositionProjections(events)

    assert(t, len(openPos) == 1)
    assert(t, openPos[0].Quantity == 0.01) // matches ledger, not drifted 0.02
}
```

---

## 4. Kill Switch Tests

### 4a. Kill Switch Blocks New Orders

```go
func TestKillSwitch_BlocksOrders(t *testing.T) {
    store := ledger.NewMemoryStore()
    ks := killswitch.NewService(store, nil, "btc-paper-1")
    pipeline := riskgate.NewPreTradeRiskPipeline(riskEngine, ks)

    ks.Trigger(ctx, killswitch.Activation{
        Trigger: killswitch.TriggerManualOperator,
        Reason:  "test halt",
    })

    decision := pipeline.Check(ctx, riskgate.Input{ /* valid input */ })
    assert(t, decision.Status == riskgate.DecisionBlocked)
    assert(t, strings.Contains(decision.Reason, "kill switch active"))
}
```

### 4b. Kill Switch Flatten: Open Positions → Kill Switch → All Closed

```go
func TestKillSwitch_FlattenPositions(t *testing.T) {
    // 1. Open 3 positions via positions.Manager
    // 2. Trigger kill switch with ActionFlattenPositions
    // 3. Executor.FlattenPositions() → CloseAllPositions() → CloseEvents
    // 4. processCloseEvents() → emitPositionClosed() appends EventPositionClosed
    // 5. Replay ledger → all positions CLOSED
    // 6. positions.Manager.GetOpenPositions() → empty

    events, _ := store.ReplayAccount(ctx, "btc-paper-1")
    openPos := omsv3.BuildOpenPositionProjections(events)
    assert(t, len(openPos) == 0) // all closed
}
```

---

## 5. Client Order ID Tests

```go
func TestClientOrderID_Generate(t *testing.T) {
    id, err := omsv3.GenerateClientOrderID("BTC-USD")
    assert(t, err == nil)
    parsed, ok := omsv3.ParseClientOrderID(id)
    assert(t, ok)
    assert(t, parsed.Symbol == "BTCUSD")
    assert(t, len(parsed.Suffix) == 8)
}

func TestClientOrderID_Idempotency(t *testing.T) {
    // Same idempotency key → same result, no duplicate event
    id, _ := omsv3.GenerateClientOrderID("BTC-USD")
    key := omsv3.IdempotencyKey(id, string(ledger.EventOrderCreated))

    store := ledger.NewMemoryStore()
    ctx := context.Background()

    event1, _ := ledger.NewEvent(ledger.NewEventInput{
        AggregateType:  ledger.AggregateOrder,
        AggregateID:    id,
        EventType:      ledger.EventOrderCreated,
        IdempotencyKey: key,
        Payload:        nil,
        Source:         "test",
    })
    _, err1 := store.Append(ctx, event1)
    assert(t, err1 == nil)

    // Second append with same idempotency key must fail
    event2, _ := ledger.NewEvent(ledger.NewEventInput{
        AggregateType:  ledger.AggregateOrder,
        AggregateID:    id,
        EventType:      ledger.EventOrderCreated,
        IdempotencyKey: key,
        Payload:        nil,
        Source:         "test",
    })
    _, err2 := store.Append(ctx, event2)
    assert(t, errors.Is(err2, ledger.ErrDuplicateEvent))
}
```

---

## 6. CQRS Projection Tests

### 6a. PnL Projection: 2 Wins + 1 Loss

```go
func TestBuildPnLProjection(t *testing.T) {
    // 3 EventPositionClosed events: +$10, +$5, -$3
    proj := omsv3.BuildPnLProjection(events)
    assert(t, proj.TotalTrades == 3)
    assert(t, proj.Wins == 2)
    assert(t, proj.Losses == 1)
    assert(t, math.Abs(proj.TotalPnLUSD - 12.0) < 0.001)
    assert(t, math.Abs(proj.BestTradeUSD - 10.0) < 0.001)
    assert(t, math.Abs(proj.WorstTradeUSD - (-3.0)) < 0.001)
    assert(t, math.Abs(proj.WinRate - 0.667) < 0.001)
}
```

### 6b. Exposure Projection: Long + Short Netting

```go
func TestBuildExposureProjection(t *testing.T) {
    // Open: LONG 0.01 BTC + SHORT 0.005 BTC
    // Net exposure = +0.005 BTC
    proj := omsv3.BuildExposureProjection(events)
    assert(t, proj.OpenPositions == 2)
    assert(t, math.Abs(proj.NetExposure["BTC-USD"] - 0.005) < 1e-9)
}
```

---

## 7. Snapshot Provider Tests

```go
func TestLedgerSnapshotProvider_OpenPosition(t *testing.T) {
    store := ledger.NewMemoryStore()
    ctx := context.Background()

    // Append EventPositionOpened
    // ...

    provider := omsv3.NewLedgerSnapshotProvider(store, "btc-paper-1")
    snap, err := provider.Snapshot(ctx)

    assert(t, err == nil)
    assert(t, snap.AccountID == "btc-paper-1")
    assert(t, len(snap.OMSPositions) == 1)
    assert(t, snap.OMSPositions[0].Quantity == 0.01)
    assert(t, snap.OMSPositions[0].Side == "BUY")

    // For paper trading: exchange mirrors OMS
    assert(t, len(snap.ExchangePositions) == 1)
    assert(t, snap.ExchangePositions[0].Quantity == snap.OMSPositions[0].Quantity)
}

func TestLedgerSnapshotProvider_ClosedPositionRemoved(t *testing.T) {
    // Append EventPositionOpened then EventPositionClosed
    snap, _ := provider.Snapshot(ctx)
    assert(t, len(snap.OMSPositions) == 0) // closed position not in snapshot
}
```

---

## Current Test Coverage

Run: `go test -mod=mod -cover ./internal/omsv3/...`

| Package | Statements | Coverage |
|---------|------------|----------|
| `omsv3` (aggregate.go) | existing tests | ~85% |
| `omsv3` (position_aggregate.go) | needs new tests | target: 95% |
| `omsv3` (replay.go) | needs new tests | target: 90% |
| `omsv3` (projections.go) | needs new tests | target: 95% |
| `omsv3` (client_order_id.go) | needs new tests | target: 100% |
| `omsv3` (snapshot_provider.go) | needs new tests | target: 90% |
| **Total target** | | **90%+** |

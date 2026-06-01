# OMS V3 Target Architecture — Phase 15A

**Date:** 2026-06-01

---

## Target Execution Flow

```
Market Data (Coinbase WS / Binance REST / AngelOne)
    │
    ▼
Strategy Engine (600+ strategies, grouped by timeframe)
    │  OnTick() → []Signal
    ▼
Signal Aggregator (FilterSignalsSelective)
    │  confidence gates, regime filter, execution weight
    ▼
Pre-Trade Risk Pipeline (riskgate.PreTradeRiskPipeline)
    │  kill switch check → size approval → EventRiskApproved / EventRiskBlocked
    ▼
OMS v3 Aggregate (omsv3.OrderAggregate)
    │  EventOrderCreated → EventOrderValidated → EventRiskApproved
    │  → EventOrderSubmitted → EventOrderAcked → EventOrderFilled
    ▼
Ledger Event Store (ledger.Store)
    │  immutable append-only event log
    │  every state transition writes an event before it executes
    ▼
PaperExchangeAdapter (execution.PaperExchangeAdapter)
    │  stateless fill simulator — price, slippage, fees
    │  does NOT own order or position state
    ▼
OMS v3 Position Aggregate (omsv3.PositionAggregate)
    │  EventPositionOpened → EventPositionReduced → EventPositionClosed
    ▼
Ledger Event Store (same store, AggregatePosition events)
    │
    ├─────────────────────────────────────────────┐
    ▼                                             ▼
CQRS Projections                        Reconciliation Service
(omsv3.BuildPositionProjections)        (reconciliation.Service)
(omsv3.BuildOrderProjections)           (omsv3.LedgerSnapshotProvider)
(omsv3.BuildPnLProjection)              ↔ Exchange Adapter
(omsv3.BuildExposureProjection)         │  every 10s: OMS ↔ Exchange
    │                                   │  detect: ghost orders,
    ▼                                   │  missing fills, position drift
Dashboard / Next.js API                 │  emit: EventReconciliationMismatch
(reads projections only)                │  auto-correct: projections rebuilt
(never reads aggregate directly)        │  from ledger replay
```

---

## Ownership Rules (Hard Invariants)

### OMS v3 owns:
- Order lifecycle (NEW → VALIDATED → RISK_APPROVED → SUBMITTED → ACKNOWLEDGED → FILLED)
- Position lifecycle (OPEN → REDUCED → CLOSED)
- All state transitions (validated via `ValidateTransition` before any mutation)
- The ClientOrderID → ExchangeOrderID mapping
- The authoritative PnL per order

### Ledger owns:
- Immutable event history (append-only, no deletes)
- Idempotency keys (duplicate submissions rejected)
- Payload hashes (tamper detection via SHA-256)
- The ability to rebuild ALL state from scratch via replay

### PaperExchangeAdapter owns:
- Execution price simulation (slippage by OrderMode)
- Fee calculation (taker/maker)
- Exchange order ID generation
- Partial fill simulation
- Does NOT own: balance, positions, order state, PnL

### Reconciliation Service owns:
- Divergence detection (missing fills, ghost orders, position drift, balance drift)
- Alert emission (EventReconciliationAlert in ledger)
- Auto-correction by replaying ledger into fresh projections

### Dashboard owns:
- Visualization and display
- Reads from CQRS projections ONLY
- Never reads aggregate state directly
- Never mutates state

### Risk Engine owns:
- Pre-trade approval / rejection decisions
- Daily PnL accounting
- Position limit enforcement
- Kill switch activation

---

## State Machine: Order Lifecycle

```
                    ┌─────────────────────────────────────────────┐
                    │              ORDER STATE MACHINE              │
                    └─────────────────────────────────────────────┘

           EMPTY
             │ EventOrderCreated
             ▼
            NEW ──────────────────────────────────┐
             │ EventOrderValidated                 │ EventOrderCancelled
             ▼                                     │ EventOrderRejected
         VALIDATED ───────────────────────────────►│
             │ EventRiskApproved                   │
             ▼                          ┌──────────▼──────────┐
        RISK_APPROVED ──────────────────►    CANCELLED         │
             │ EventOrderSubmitted      │    (terminal)        │
             ▼                          └─────────────────────┘
          SUBMITTED ────────────────────────────────────────────┐
             │ EventOrderAcked                                   │
             ▼                                        ┌──────────▼──────────┐
         ACKNOWLEDGED                                 │     REJECTED         │
             │ EventOrderPartial ─┐                   │    (terminal)        │
             │ EventOrderFilled   │                   └─────────────────────┘
             ▼                   │ ◄─ repeat
        PARTIALLY_FILLED ────────┘
             │ EventOrderFilled
             ▼
           FILLED
          (terminal)

Forbidden transitions: FILLED → any, CANCELLED → any, REJECTED → any
```

---

## State Machine: Position Lifecycle

```
                  ┌──────────────────────────────────┐
                  │         POSITION STATE MACHINE     │
                  └──────────────────────────────────┘

         EMPTY
           │ EventPositionOpened
           ▼
          OPEN ──────────────────────────────────────┐
           │                                         │
           │ EventPositionChanged (partial close)    │ EventPositionClosed
           ▼                                         │
         REDUCED ────────────────────────────────────►
           │ EventPositionClosed                     │
           ▼                                         │
          CLOSED ◄───────────────────────────────────┘
         (terminal)

Exit reasons encoded in EventPositionClosed payload:
  TP          — take-profit price crossed
  SL          — stop-loss price crossed
  TIME        — hold time elapsed
  TRAIL       — trailing stop triggered
  MANUAL      — operator closed via API
  KILL_SWITCH — emergency flatten
```

---

## Module Responsibility Matrix

| Module | Creates Events | Reads Events | Owns State | Mutates State |
|--------|---------------|--------------|------------|---------------|
| `trading/loop.go` (Orchestrator) | ✅ YES | ❌ No | ❌ No | ❌ No |
| `omsv3/aggregate.go` (OrderAggregate) | ❌ No | ✅ YES | ✅ YES (in-process only) | ✅ YES (via ApplyEvent) |
| `omsv3/position_aggregate.go` | ❌ No | ✅ YES | ✅ YES (in-process only) | ✅ YES (via ApplyEvent) |
| `ledger/store.go` | ❌ No | ✅ YES | ✅ YES (event log) | ✅ YES (append only) |
| `execution/paper_exchange_adapter.go` | ❌ No | ❌ No | ❌ No | ❌ No |
| `positions/manager.go` | ❌ No | ❌ No | ⚠️ Stage 1 only | ⚠️ Stage 1 only |
| `execution/paper.go` (PaperClient) | ❌ No | ❌ No | ⚠️ Stage 1 only | ⚠️ Stage 1 only |
| `reconciliation/service.go` | ✅ YES (alerts) | ✅ YES (snapshots) | ❌ No | ❌ No |
| `killswitch/service.go` | ✅ YES | ❌ No | ✅ YES (active flag) | ✅ YES |
| `execution/paper_oms.go` (PaperOMS) | ❌ No | ❌ No | ⚠️ REST only | ⚠️ REST only |

⚠️ = owned during Stage 1 dual-write, to be removed by Stage 4.

---

## CQRS Read Model Architecture

```
Ledger Events (immutable)
    │
    ├── AggregateOrder events
    │       │ BuildOrderProjections(events)
    │       └──► []OrderProjection (for open order display)
    │
    ├── AggregatePosition events
    │       │ BuildPositionProjections(events)
    │       ├──► []PositionProjection (all positions: open + closed)
    │       │ BuildOpenPositionProjections(events)
    │       ├──► []PositionProjection (open only — for risk engine)
    │       │ BuildPnLProjection(events)
    │       └──► PnLProjection (win rate, total PnL, fees)
    │
    └── AggregatePosition events (for exposure)
            │ BuildExposureProjection(events)
            └──► ExposureProjection (net BTC per symbol, total notional)
```

Dashboard reads projections rebuilt on demand from the ledger. Projections are NOT cached — they are always rebuilt fresh from events. This guarantees the dashboard is never out of sync with the ledger.

In production (Phase 14+): projections are maintained as materialized views in TimescaleDB and updated via event streaming.

---

## Idempotency Layer

Every order event carries an `IdempotencyKey`:

```
Format: {clientOrderID}:{eventType}
Example: BTC-20260601-143022-a3f7bc92:ORDER_CREATED
```

The `ledger.MemoryStore.Append()` rejects duplicate idempotency keys with `ErrDuplicateEvent`. This prevents duplicate orders from strategy bursts or network retries.

---

## ClientOrderID Format

```
Format: {SYMBOL}-{YYYYMMDD}-{HHMMSS}-{8HEX}
Example: BTCUSDT-20260601-143022-a3f7bc92

Components:
  SYMBOL:  Normalized exchange symbol (max 8 chars, uppercase)
  DATE:    UTC trade date YYYYMMDD
  TIME:    UTC trade time HHMMSS  
  SUFFIX:  4 random bytes as hex (collision resistance)
```

Properties:
- Lexicographically sortable by time
- Unique across system restarts (random suffix)
- Exchange-identifiable (symbol prefix)
- Persisted in ledger as primary order identifier

---

## Reconciliation Architecture

```
Every 10 seconds:

LedgerSnapshotProvider
    │ ReplayAccount → BuildPositionProjections
    │ → []OMSPosition (open positions from OMS v3)
    │ → []OMSOrder (live orders from OMS v3)
    ▼
reconciliation.Service.Check()
    ├── OrderMismatchDetector
    │     compare: OMSOrders ↔ ExchangeOrders
    │     detect: ghost orders, missing fills, stale orders
    │
    ├── PositionDriftDetector
    │     compare: OMSPositions ↔ ExchangePositions
    │     tolerance: 1e-8 BTC
    │     detect: position drift, stale positions
    │
    └── BalanceDriftDetector
          compare: OMSBalance ↔ ExchangeBalance
          tolerance: $1 USD
          detect: balance drift

Each alert → EventReconciliationAlert appended to ledger
              AggregateID = alert.Reference (order/position ID)
              Payload = reconciliation.Alert{Type, Severity, Message}

Auto-correction:
  On ReconciliationMismatch → rebuild projections from ledger
  → positions.Manager reconciled to ledger state
  → kills switch if drift exceeds emergency threshold
```

---

## Kill Switch Integration

```
KillSwitch.IsActive() checked by:
  └── riskgate.PreTradeRiskPipeline.Check()
        └── returns DecisionBlocked if kill switch active
              └── EventRiskBlocked appended to ledger

KillSwitch.Trigger() called by:
  └── Operator via /api/admin/kill
  └── reconciliation.Service (on critical drift)
  └── risk.Engine (on daily loss breach)
  
KillSwitch.Trigger() → EventKillSwitchTriggered in ledger
                      → Executor.FlattenPositions()
                        → positions.Manager.CloseAllPositions()
                        → EventPositionClosed for each position
                      → Executor.CancelOpenOrders()
                      → Executor.SendAlert()
```

---

## Migration Stages

| Stage | Description | State |
|-------|-------------|-------|
| 1 | **Dual Write** — OMS v3 events written alongside existing state | ✅ Phase 15A |
| 2 | **Shadow Mode** — OMS v3 projections readable but not authoritative | Phase 15B |
| 3 | **OMS v3 Primary** — positions.Manager reads from OMS v3 | Phase 15C |
| 4 | **paper_oms Retired** — PaperOMS removed from orchestrator path | Phase 15D |
| 5 | **Legacy Removed** — PaperClient, OMSManager, sync.go deleted | Phase 15E |

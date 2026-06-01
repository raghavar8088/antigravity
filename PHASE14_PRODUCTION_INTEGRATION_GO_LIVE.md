# Phase 14 Production Integration & Go-Live Hardening

Target readiness after implementation slice: 75-80/100  
Rule: Survival > Risk > Correctness > Latency > Profit  
Mission: connect institutional modules into the authoritative live path and retire legacy bypass paths before live capital is enabled.

## 1. Integration Architecture Diagram

```text
Exchange WS/REST
  |
  v
Market Data Service
  |  MarketTick / BookSnapshot / FundingSnapshot / LiquidationEvent
  v
Durable Event Bus
  |
  v
Strategy Engine
  |  SignalProposed
  v
Signal Router
  |  SignalApproved / SignalRejected
  v
PreTradeRiskPipeline
  |  RiskApproved / RiskBlocked
  v
OMS V3 Aggregate
  |  OrderCreated / OrderValidated / RiskApproved / OrderSubmitted / OrderAcknowledged / OrderFilled
  v
Execution Engine / Exchange Gateway
  |
  v
Event Ledger
  |
  v
Timescale/Postgres Projections
  |
  v
Redis Hot Cache
  |
  v
Dashboard Projection
```

Authoritative path:

`Market Data -> Event Bus -> Strategy Engine -> Risk Engine V3 -> OMS V3 -> Execution Engine -> Event Ledger -> Timescale/Postgres -> Dashboard Projection`

Forbidden for live mode:

- Browser execution path.
- Direct Mongo order/trade writes.
- Direct exchange gateway calls outside OMS.
- Dashboard state as truth.
- Strategy-to-exchange bypass.

## 2. Exact Code Modules Created

- `engine/internal/ledger/event.go`
- `engine/internal/ledger/store.go`
- `engine/internal/ledger/order_projection.go`
- `engine/internal/ledger/store_test.go`
- `engine/internal/omsv3/aggregate.go`
- `engine/internal/omsv3/aggregate_test.go`
- `engine/internal/risk/gate/pipeline.go`
- `engine/internal/risk/gate/pipeline_test.go`
- `engine/internal/reconciliation/detectors.go`
- `engine/internal/reconciliation/service.go`
- `engine/internal/reconciliation/detectors_test.go`
- `engine/internal/killswitch/service.go`
- `engine/internal/killswitch/service_test.go`

## 3. Exact Files To Modify Next For Full Cutover

- `engine/internal/trading/loop.go`
  - Replace `risk.Validate -> PaperClient.ExecuteSignal -> positions.Manager` with `PreTradeRiskPipeline -> OMS V3 -> Execution Gateway -> Ledger`.
- `engine/cmd/antigravity/main.go`
  - Construct ledger store, OMS V3, risk gate, reconciliation service, kill switch service, and dashboard projections.
- `engine/internal/execution/paper.go`
  - Deprecate direct state mutation for live/paper production. Execution must return fills as events.
- `engine/internal/positions/`
  - Convert positions to ledger projections from fill events.
- `engine/internal/persistence/`
  - Add Postgres/Timescale event store implementation behind `ledger.Store`.
- `client/src/components/terminal/institutional/`
  - Read projections only.
- `client/src/app/api/paper-*`
  - Block live-mode writes and mark as legacy paper-only.
- `client/src/internal/oms/index.ts`
  - Align states with OMS V3 names.

## 4. Event Schemas

### Event Envelope

```json
{
  "event_id": "uuid-or-random-id",
  "schema_version": 1,
  "aggregate_type": "ORDER",
  "aggregate_id": "client-order-id",
  "sequence_no": 1,
  "event_type": "ORDER_CREATED",
  "account_id": "acct-1",
  "strategy_id": "phase11-cvd",
  "symbol": "BTCUSDT",
  "correlation_id": "signal-to-order-correlation-id",
  "causation_id": "previous-event-id",
  "idempotency_key": "sha256(account,strategy,signal,side,symbol,qty)",
  "payload": {},
  "payload_hash": "sha256(payload)",
  "created_at": "2026-05-31T00:00:00Z",
  "source": "oms-service"
}
```

### Required Events

- `ORDER_CREATED`
- `ORDER_VALIDATED`
- `RISK_APPROVED`
- `RISK_BLOCKED`
- `ORDER_SUBMITTED`
- `ORDER_ACKED`
- `ORDER_PARTIAL`
- `ORDER_FILLED`
- `ORDER_CANCELLED`
- `ORDER_REJECTED`
- `POSITION_OPENED`
- `POSITION_CLOSED`
- `KILLSWITCH_TRIGGERED`
- `RECONCILIATION_ALERT`
- `RECONCILIATION_CORRECTED`
- `RECONCILIATION_RESOLVED`

## 5. Redis Schema

Keys must be derived from ledger projections, not written as truth.

```text
live:position:{account}:{symbol}              hash, TTL 10s, refreshed on projection update
live:pnl:{account}                            hash, TTL 10s
live:risk:{account}                           hash, TTL 10s
live:market:{exchange}:{symbol}               hash, TTL 3s
live:strategy_rankings:{account}              sorted set, TTL 60s
live:order:{account}:{client_order_id}         hash, TTL 24h after terminal state
dedupe:idempotency:{idempotency_key}           string event_id, TTL 24h
rate:exchange:{exchange}:{endpoint}:{window}   counter, TTL window+5s
health:service:{service_name}                  hash, TTL 15s
```

Invalidation:

- Ledger projection update invalidates corresponding `live:*` key.
- Terminal order state keeps 24h TTL.
- Market snapshots expire fast; stale cache blocks trading.
- Warm restart loads from ledger/Postgres, then repopulates Redis.

## 6. Timescale Schema

Core tables:

```sql
create table trading.event_store (
  sequence_no bigserial primary key,
  event_id uuid not null unique,
  schema_version integer not null,
  aggregate_type text not null,
  aggregate_id text not null,
  event_type text not null,
  account_id text,
  strategy_id text,
  symbol text,
  correlation_id uuid,
  causation_id uuid,
  idempotency_key text unique,
  payload jsonb not null,
  payload_hash text not null,
  source text not null,
  created_at timestamptz not null default now()
);

create table market.ticks (
  time timestamptz not null,
  exchange text not null,
  symbol text not null,
  price double precision not null,
  quantity double precision not null,
  trade_id text,
  received_at timestamptz not null default now()
);

create table trading.order_projection (
  client_order_id text primary key,
  exchange_order_id text,
  account_id text not null,
  symbol text not null,
  side text not null,
  state text not null,
  quantity double precision not null,
  filled_quantity double precision not null,
  average_fill_price double precision,
  updated_at timestamptz not null
);

create table trading.fill_projection (
  fill_id uuid primary key,
  client_order_id text not null,
  exchange_order_id text,
  account_id text not null,
  symbol text not null,
  side text not null,
  fill_price double precision not null,
  fill_quantity double precision not null,
  fee_usd double precision not null,
  filled_at timestamptz not null
);

create table trading.position_projection (
  account_id text not null,
  symbol text not null,
  side text not null,
  quantity double precision not null,
  average_entry double precision not null,
  realized_pnl double precision not null,
  unrealized_pnl double precision not null,
  updated_at timestamptz not null,
  primary key (account_id, symbol, side)
);
```

Hypertables:

- `market.ticks` by `time`
- `trading.fill_projection` by `filled_at`
- `trading.event_store` by `created_at`

Continuous aggregates:

- 1m/5m/1h candles from ticks.
- 1m/5m PnL snapshots.
- Strategy performance by hour/day.
- Risk heat by minute.

Retention:

- Raw ticks: 30-90 days depending storage.
- 1m candles: 2 years.
- Event store: never delete.
- Logs: 30 days hot, archive to object storage.

Dashboard query target:

- Hot projection reads from Redis: <20ms.
- Postgres projection reads: <100ms with account/symbol/time indexes.
- Historical chart queries via continuous aggregates: <100ms for 24h to 30d windows.

## 7. OMS State Machine Implementation

Implemented in `engine/internal/omsv3/aggregate.go`.

States:

```text
NEW
VALIDATED
RISK_APPROVED
SUBMITTED
ACKNOWLEDGED
PARTIALLY_FILLED
FILLED
CANCELLED
REJECTED
```

Invalid transitions fail:

- `FILLED -> NEW`
- `REJECTED -> PARTIALLY_FILLED`
- Any unknown event against an order aggregate.

Every replayed event is hash-validated at ledger append time and ordered by aggregate sequence.

## 8. Reconciliation Engine Design

Implemented:

- `OrderMismatchDetector`
- `PositionDriftDetector`
- `BalanceDriftDetector`
- `Service.Check()`

Detection:

- Missing fills: exchange filled quantity exceeds OMS projection.
- Ghost orders: OMS live order absent from exchange or exchange order absent from OMS.
- Duplicate orders: multiple exchange orders share one client order id.
- Stale positions: OMS position missing at exchange.
- Balance drift: exchange equity/cash differs from ledger projection.

Events:

- `RECONCILIATION_ALERT`
- Later projection self-healing must emit `RECONCILIATION_CORRECTED`.

Policy:

- Critical drift activates kill switch.
- New orders blocked until reconciliation resolved.

## 9. Kill Switch Implementation

Implemented in `engine/internal/killswitch/service.go`.

Triggers:

- Daily loss breach.
- Exchange outage.
- Data feed outage.
- OMS desync.
- Risk service failure.
- Large position drift.
- Funding shock.
- Liquidation event spike.
- Manual operator trigger.

Actions:

- Cancel open orders.
- Block new orders.
- Flatten positions when configured.
- Send alerts.
- Persist `KILLSWITCH_TRIGGERED`.

## 10. Prometheus Metrics List

```text
trading_order_latency_ms{stage}
trading_risk_latency_ms
trading_queue_latency_ms{queue}
trading_db_latency_ms{operation}
trading_cache_hit_ratio{cache}
trading_exchange_latency_ms{exchange,endpoint}
trading_orders_total{state,exchange}
trading_fills_total{exchange,symbol}
trading_reconciliation_alerts_total{type,severity}
trading_killswitch_active{account}
trading_pnl_realized_usd{account}
trading_pnl_unrealized_usd{account}
trading_drawdown_pct{account}
trading_strategy_score{strategy}
trading_market_data_staleness_ms{exchange,symbol}
trading_event_ledger_append_latency_ms
trading_event_replay_duration_ms
```

Dashboards:

- Trading Dashboard: orders, fills, PnL, positions, latency.
- Risk Dashboard: heat, VaR, CVaR, drawdown, kill switch.
- Execution Dashboard: order state funnel, rejects, exchange latency, reconciliation.
- Infrastructure Dashboard: DB, Redis, queues, pod health, event lag.

## 11. Disaster Recovery Plan

RPO target: <5 minutes.  
RTO target: <15 minutes.

Backup:

- PostgreSQL PITR with WAL archive every 5 minutes or better.
- Daily full database backup.
- Redis AOF enabled with snapshot backup.
- Event store exported to object storage daily.
- Configuration/secrets versioned and encrypted.

Recovery:

1. Activate global kill switch.
2. Cancel open orders if exchange reachable.
3. Restore Postgres to latest valid point.
4. Replay event ledger.
5. Rebuild OMS, positions, PnL, and risk state.
6. Reconcile with exchange.
7. If clean, enable paper/testnet first.
8. Enable live only after manual risk officer approval.

## 12. Go-Live Checklist

Execution:

- PASS/FAIL: OMS V3 is the only order path.
- PASS/FAIL: Direct `PaperClient.ExecuteSignal` path disabled for live.
- PASS/FAIL: All orders have client order IDs and idempotency keys.
- PASS/FAIL: Partial fills, cancels, rejects, and stale orders replay correctly.

Risk:

- PASS/FAIL: PreTradeRiskPipeline is mandatory before `ORDER_SUBMITTED`.
- PASS/FAIL: Kill switch blocks new orders.
- PASS/FAIL: Reconciliation critical alert activates kill switch.
- PASS/FAIL: VaR, CVaR, heat, leverage, correlation, funding, symbol and exchange allocation gates run.

Ledger:

- PASS/FAIL: All order/position/risk state rebuilds from event store.
- PASS/FAIL: Payload hash validation works.
- PASS/FAIL: Duplicate idempotency keys rejected.
- PASS/FAIL: Dashboard reads projections only.

Database/Redis:

- PASS/FAIL: Ticks, signals, orders, fills, positions, PnL snapshots in Timescale.
- PASS/FAIL: Hypertables and continuous aggregates installed.
- PASS/FAIL: Redis hot cache warms from projections and expires stale market data.

Observability:

- PASS/FAIL: Prometheus scrapes engine services.
- PASS/FAIL: Grafana dashboards created.
- PASS/FAIL: Loki receives structured logs.
- PASS/FAIL: Alerts fire for kill switch, reconciliation, stale data, and DB failure.

Validation:

- PASS/FAIL: March 2020 crash simulation.
- PASS/FAIL: FTX collapse simulation.
- PASS/FAIL: ETF rally simulation.
- PASS/FAIL: Funding squeeze simulation.
- PASS/FAIL: Flash crash simulation.
- PASS/FAIL: Exchange disconnect simulation.
- PASS/FAIL: Mass order rejection simulation.

## 13. Estimated Readiness Score

Current: 66/100.

After this implementation slice:

- Ledger foundation with replay: +2
- OMS V3 aggregate and invalid transition rejection: +2
- Mandatory pre-trade risk pipeline package: +2
- Reconciliation detectors and alert event generation: +2
- Kill switch service and audit event persistence: +2
- Integration blueprint and schemas: +1

Estimated after code plus final live-loop cutover: 75-80/100.

Important: score should remain below 75 until `engine/internal/trading/loop.go` stops using legacy execution calls for live mode.

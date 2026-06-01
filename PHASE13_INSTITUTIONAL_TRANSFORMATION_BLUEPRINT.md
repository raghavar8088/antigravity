# Phase 13 Institutional Transformation Blueprint

Target readiness: 80/100  
Operating principle: Survival > Risk > Execution > Latency > Profit > Features  
Scope: BTC live trading, paper trading, research, backtesting, execution, risk, observability, and UI separated into failure-isolated domains.

This is an implementation blueprint, not an audit. It uses the current foundations already present in the repository:

- Go engine: `engine/`
- Event bus: `engine/internal/events/bus.go`
- OMS: `engine/internal/oms/`
- Execution: `engine/internal/execution/`
- Risk V2 and portfolio risk: `engine/internal/risk/`, `engine/internal/risk/v2/`
- Phase 11 alpha: `engine/internal/alpha/`, `engine/internal/strategy/alpha_strategies.go`
- Backtest V2: `engine/internal/backtest/v2/`
- Next institutional terminal: `client/src/app/terminal/`, `client/src/components/terminal/institutional/`
- Client-side internal prototypes: `client/src/internal/`
- Event replay SQL: `client/supabase/migrations/010_event_replay_data_quality_and_security.sql`
- Production Docker: `docker-compose.prod.yml`, `engine/Dockerfile`

## 1. Final Target Architecture

```text
Exchange Feeds
  |
  v
Market Data Layer
  owns: raw ticks, order book, funding, liquidations, health
  protocol: NATS JetStream or Redis Streams, protobuf/json envelope
  |
  v
Durable Event Bus
  owns: transport offsets, retry state, dead letters
  protocol: at-least-once, idempotency key, monotonic sequence
  |
  v
Strategy Cluster
  owns: signal candidates only, never positions or orders
  protocol: SignalProposed events
  |
  v
Signal Router
  owns: de-duplication, correlation cluster, mode routing
  protocol: SignalApproved / SignalRejected events
  |
  v
Portfolio Risk Engine
  owns: pre-trade approval, live risk, kill switches, sizing
  protocol: RiskApproved / RiskTriggered events
  |
  v
Execution OMS
  owns: client order IDs, order state machine, idempotency
  protocol: OrderCreated / OrderAcked / OrderFilled events
  |
  v
Exchange Gateway
  owns: exchange adapters, throttling, request signing, reconnects
  protocol: REST/WebSocket adapter contracts
  |
  v
Reconciliation Engine
  owns: exchange-vs-ledger consistency, stale order cleanup
  protocol: Reconciliation events
  |
  v
Event Ledger
  owns: immutable truth, replay, recovery, audit
  protocol: append-only PostgreSQL/Timescale writes
  |
  v
Analytics Layer
  owns: projections, metrics, dashboards, alerts
  protocol: read models only
  |
  v
UI Layer
  owns: operator workflows, visibility, approvals
  protocol: API gateway read/write commands, never source of truth
```

## 2. Component Responsibilities and Data Ownership

| Component | Responsibilities | Owns | Must Not Own | Protocol |
|---|---|---|---|---|
| Market Data Layer | WebSocket lifecycle, tick normalization, order book snapshots, funding, liquidation feeds, stale data detection | Raw market data and feed health | Orders, positions, risk approvals | `MarketTick`, `BookSnapshot`, `FundingSnapshot`, `LiquidationEvent` |
| Durable Event Bus | Fanout, replay offsets, dead letters, backpressure | Message delivery state | Business state | NATS JetStream preferred, Redis Streams acceptable |
| Strategy Cluster | Evaluate 15-25 approved strategies under mode-specific configs | Candidate signals | Capital allocation or order state | `SignalProposed` |
| Signal Router | Rank, de-duplicate, cluster by alpha source, enforce strategy mode | Signal routing decisions | Portfolio equity | `SignalApproved`, `SignalRejected` |
| Portfolio Risk Engine | Kelly/fractional Kelly, heat, VaR, CVaR, correlation, exposure, kill switch | Risk state and sizing decisions | Exchange orders | `RiskApproved`, `RiskTriggered`, `KillSwitchTriggered` |
| Execution OMS | Order state machine, client order IDs, idempotency, retries, cancels | Internal order lifecycle | Exchange balances as truth | `OrderCreated`, `OrderSubmitted`, `OrderAcked`, `OrderPartial`, `OrderFilled` |
| Exchange Gateway | Auth, request signing, API throttling, exchange WebSockets, adapter isolation | Exchange sessions and rate budget | Strategy logic | Gateway request/response contract |
| Reconciliation Engine | Compare exchange orders, fills, balances, and positions to ledger projections | Reconciliation jobs and exceptions | Strategy decisions | `ReconciliationStarted`, `ReconciliationMismatch`, `ReconciliationResolved` |
| Event Ledger | Immutable append-only event store, snapshots, replay | Truth layer | UI state | PostgreSQL/Timescale append-only schema |
| Analytics Layer | Derived read models, latency SLOs, strategy scorecards, risk dashboards | Projections | Write-side state | SQL views/materialized views |
| UI Layer | Operator actions, state display, approvals, incident workflows | UI preferences | Trading truth | API gateway commands and read models |

## 3. Recommendations

### Recommendation P0-1: Replace In-Process Event Bus With Durable Bus Boundary

#### Problem
`engine/internal/events/bus.go` is non-blocking and drops events when subscriber buffers fill. That is acceptable for analytics, but not for execution, risk, or ledger truth.

#### Risk
A dropped `ORDER_FILLED`, `RISK_TRIGGERED`, or `KILLSWITCH_TRIGGERED` event can leave the OMS, positions, and dashboard out of sync, creating hidden exposure.

#### Solution
Keep the current in-process bus only for non-critical telemetry. Add a durable bus interface backed by NATS JetStream in production and Redis Streams as a small-team fallback. Every execution-critical event must include `event_id`, `idempotency_key`, `aggregate_type`, `aggregate_id`, `sequence_no`, `correlation_id`, `causation_id`, `schema_version`, and payload hash.

#### Files/Modules Affected
- `engine/internal/events/`
- `engine/internal/oms/`
- `engine/internal/risk/v2/`
- `engine/internal/execution/`
- `engine/cmd/antigravity/`
- `client/supabase/migrations/010_event_replay_data_quality_and_security.sql`

#### Architecture Impact
The system moves from best-effort pub/sub to durable at-least-once delivery. Consumers become idempotent.

#### Latency Impact
Adds 1-5ms per critical event on local Redis/NATS, but removes catastrophic state-loss risk.

#### Risk Reduction
Prevents invisible state divergence during subscriber overload, process crash, or temporary database outage.

#### Priority
P0

#### Estimated Effort
3-5 days.

### Recommendation P0-2: Make Event Ledger the Only Source of Truth

#### Problem
Current state is split across in-memory OMS, periodic snapshots, client-side UI state, and SQL event replay helpers.

#### Risk
After process crash or database write failure, open position state can be reconstructed inconsistently, causing duplicate orders or missed exits.

#### Solution
Implement append-only ledger writes before state mutation for all execution-critical events. Use snapshots as projections only. Recovery replays events into OMS, positions, and risk state. Dashboard reads projections, never mutable in-browser state.

#### Files/Modules Affected
- `engine/internal/ledger/` new
- `engine/internal/oms/`
- `engine/internal/positions/`
- `engine/internal/persistence/`
- `client/supabase/migrations/010_event_replay_data_quality_and_security.sql`
- `client/src/components/terminal/institutional/`

#### Architecture Impact
OMS and risk become deterministic projections of the event ledger. Operators can recover from corrupted state by replay.

#### Latency Impact
Adds one append call per state change. With local Postgres connection pooling, target 2-8ms.

#### Risk Reduction
Eliminates dashboard/local memory as source of truth and supports crash recovery.

#### Priority
P0

#### Estimated Effort
5-8 days.

### Recommendation P0-3: Upgrade OMS State Machine to Institutional Contract

#### Problem
Existing OMS states use `PENDING`, `SUBMITTED`, `PARTIAL`, `FILLED`, `CANCELLED`, `REJECTED`, `CLOSED`. Required live state semantics need `NEW`, `ACKNOWLEDGED`, `PARTIAL_FILL`, and `EXPIRED`, plus replace/cancel workflows.

#### Risk
Ambiguous exchange acknowledgment and stale order handling can trigger duplicate live orders or leave resting orders unmanaged.

#### Solution
Introduce OMS V3 state model:

```text
NEW
  -> SUBMITTED
  -> CANCELLED
  -> EXPIRED

SUBMITTED
  -> ACKNOWLEDGED
  -> REJECTED
  -> CANCELLED
  -> EXPIRED

ACKNOWLEDGED
  -> PARTIAL_FILL
  -> FILLED
  -> CANCELLED
  -> EXPIRED

PARTIAL_FILL
  -> PARTIAL_FILL
  -> FILLED
  -> CANCELLED
  -> EXPIRED

FILLED, CANCELLED, REJECTED, EXPIRED are terminal for order lifecycle.
Position lifecycle is separate: POSITION_OPENED, POSITION_CHANGED, POSITION_CLOSED.
```

Every transition appends ledger event first, then updates read model. Replace is modeled as cancel old order plus create new order with `replaces_client_order_id`.

#### Files/Modules Affected
- `engine/internal/oms/order.go`
- `engine/internal/oms/manager.go`
- `client/src/internal/oms/index.ts`
- `engine/internal/execution/`
- `engine/internal/ledger/`

#### Architecture Impact
Order lifecycle becomes exchange-safe and idempotent. Position lifecycle stops being overloaded into order state.

#### Latency Impact
No material compute impact; state transition validation is sub-millisecond.

#### Risk Reduction
Prevents stale order, duplicate order, and ambiguous partial-fill failures.

#### Priority
P0

#### Estimated Effort
3-4 days.

### Recommendation P0-4: Add Reconciliation Loop Before Live Trading

#### Problem
There is no hard requirement that exchange-reported open orders, fills, balances, and positions match local OMS/ledger projections.

#### Risk
If WebSocket disconnects during a fill, the engine may think risk is flat while the exchange has live BTC exposure.

#### Solution
Add reconciliation workers:
- Every 5s: open orders and recent fills.
- Every 15s: positions and margin.
- Every 60s: balances and funding.
- On startup: full exchange snapshot before strategies are enabled.
- On mismatch: pause new orders, append `RECONCILIATION_MISMATCH`, alert, and either self-heal or require operator approval.

#### Files/Modules Affected
- `engine/internal/reconciliation/` new
- `engine/internal/oms/`
- `engine/internal/exchange/` or `engine/internal/delta/`
- `engine/internal/risk/v2/`
- `client/src/components/terminal/institutional/ExecutionCenter.tsx`

#### Architecture Impact
Exchange Gateway and OMS become eventually consistent with explicit mismatch handling.

#### Latency Impact
Background only. No signal path impact except when mismatch pauses execution.

#### Risk Reduction
Protects against WebSocket loss, exchange-side fills, partial fills, and manual exchange changes.

#### Priority
P0

#### Estimated Effort
4-6 days.

### Recommendation P0-5: Split Paper, Live, Research, Backtest, Risk, and Observability Runtimes

#### Problem
The repo has separation by module, but production runtime still risks mixing concerns in one process/container.

#### Risk
A research/backtest spike or UI path can degrade live execution latency or crash the live process.

#### Solution
Deploy independent services:
- `market-data-service`
- `strategy-service`
- `risk-service`
- `oms-service`
- `exchange-gateway-service`
- `reconciliation-service`
- `ledger-writer-service`
- `analytics-service`
- `terminal-api`
- `research-worker`
- `backtest-worker`

#### Files/Modules Affected
- `engine/cmd/`
- `engine/internal/*`
- `docker-compose.prod.yml`
- `infrastructure/`
- Future `deploy/k8s/`

#### Architecture Impact
Failure domains become explicit. Live execution can run with smaller memory and tighter SLOs.

#### Latency Impact
Adds network hops if fully split. For small-fund v1, colocate market-data/strategy/risk/OMS in one pod with internal channels, and split analytics/research/backtest first.

#### Risk Reduction
Prevents non-live workloads from taking down live trading.

#### Priority
P0

#### Estimated Effort
5-10 days depending on deployment depth.

## 4. Institutional OMS Design

### Order Identity

- `client_order_id`: generated by OMS before any exchange request. Format: `AG-{env}-{symbol}-{unix_ms}-{seq}`.
- `exchange_order_id`: nullable until exchange ack.
- `idempotency_key`: hash of account, strategy, signal id, side, symbol, qty, price, time bucket.
- `correlation_id`: follows signal -> risk -> order -> fill -> position.
- `causation_id`: previous event id.

### Exact State Machine

| Current | Allowed Next | Trigger | Ledger Event |
|---|---|---|---|
| NEW | SUBMITTED | Gateway request accepted locally | ORDER_SUBMITTED |
| NEW | CANCELLED | Operator/risk cancels before send | ORDER_CANCELLED |
| NEW | EXPIRED | TTL before submission | ORDER_EXPIRED |
| SUBMITTED | ACKNOWLEDGED | Exchange ack received | ORDER_ACKED |
| SUBMITTED | REJECTED | Exchange rejects | ORDER_REJECTED |
| SUBMITTED | CANCELLED | Cancel confirmed before ack | ORDER_CANCELLED |
| SUBMITTED | EXPIRED | Ack timeout and exchange confirms absent | ORDER_EXPIRED |
| ACKNOWLEDGED | PARTIAL_FILL | Fill qty < order qty | ORDER_PARTIAL |
| ACKNOWLEDGED | FILLED | Full fill | ORDER_FILLED |
| ACKNOWLEDGED | CANCELLED | Cancel confirmed | ORDER_CANCELLED |
| ACKNOWLEDGED | EXPIRED | Time-in-force expiry | ORDER_EXPIRED |
| PARTIAL_FILL | PARTIAL_FILL | Additional partial fill | ORDER_PARTIAL |
| PARTIAL_FILL | FILLED | Cumulative fill reaches qty | ORDER_FILLED |
| PARTIAL_FILL | CANCELLED | Remaining qty cancelled | ORDER_CANCELLED |
| PARTIAL_FILL | EXPIRED | Remaining qty expires | ORDER_EXPIRED |

### Rules

1. Terminal order states are `FILLED`, `CANCELLED`, `REJECTED`, `EXPIRED`.
2. Position state is separate and driven by fills.
3. Every exchange request is idempotent by `client_order_id`.
4. Replace is cancel-plus-new; never mutate price/qty in place without an event.
5. Stale order cleanup cancels `SUBMITTED` without ack after configured TTL and reconciles with exchange before marking `EXPIRED`.

## 5. Truth Layer Event Schema

### Canonical Event Envelope

```json
{
  "event_id": "uuid",
  "schema_version": 1,
  "aggregate_type": "ORDER|POSITION|RISK|ACCOUNT|MARKET_DATA|RECONCILIATION",
  "aggregate_id": "uuid-or-client-order-id",
  "sequence_no": 123,
  "event_type": "ORDER_CREATED",
  "account_id": "uuid",
  "strategy_id": "string",
  "symbol": "BTCUSDT",
  "correlation_id": "uuid",
  "causation_id": "uuid",
  "idempotency_key": "sha256",
  "payload": {},
  "payload_hash": "sha256",
  "created_at": "timestamp",
  "source": "oms-service"
}
```

### Required Event Types

- `ORDER_CREATED`
- `ORDER_SUBMITTED`
- `ORDER_ACKED`
- `ORDER_PARTIAL`
- `ORDER_FILLED`
- `ORDER_CANCELLED`
- `ORDER_REJECTED`
- `ORDER_EXPIRED`
- `ORDER_REPLACED`
- `POSITION_OPENED`
- `POSITION_CHANGED`
- `POSITION_CLOSED`
- `RISK_APPROVED`
- `RISK_TRIGGERED`
- `KILLSWITCH_TRIGGERED`
- `RECONCILIATION_MISMATCH`
- `RECONCILIATION_RESOLVED`
- `MARKET_DATA_STALE`
- `EXCHANGE_OUTAGE`

### Storage Layer

- PostgreSQL/TimescaleDB `event_store` append-only table.
- Unique index on `(aggregate_type, aggregate_id, sequence_no)`.
- Unique index on `idempotency_key` where not null.
- Hash chain by aggregate for tamper detection.
- Read projections for orders, positions, risk, PnL, and dashboards.

### Replay Mechanism

1. Load all events for account ordered by `sequence_no`.
2. Rebuild OMS orders.
3. Rebuild positions from fills.
4. Rebuild risk/account state.
5. Compare projection hash to stored snapshot.
6. If mismatch: halt live trading and require reconciliation.

### Recovery Mechanism

1. On boot, live strategies are disabled.
2. Ledger replay rebuilds local state.
3. Exchange reconciliation compares external state.
4. If matched, enable risk then strategies.
5. If mismatched, keep kill switch active and publish incident.

## 6. Strategy Consolidation

Target active registry: 15-25 institutional strategies. Keep broad research/backtest inventory separately, but live and paper production must use a curated registry.

| Family | Action | Rationale |
|---|---|---|
| Trend | MERGE | Keep 3-4 trend strategies using EMA/ADX/MSS confirmation. Delete duplicate parameter variants. |
| Mean Reversion | MERGE | Keep 3-4 strategies that use range regime, Bollinger/Keltner, funding, and liquidity confirmation. |
| Breakout | KEEP LIMITED | Keep 2-3 volatility expansion strategies with strict false-break filters. |
| Microstructure | KEEP | Keep CVD divergence, delta absorption, liquidity sweep, and order-flow imbalance where data is reliable. |
| Funding | KEEP | Keep funding mean reversion and funding crowding as separate alpha source with tight exposure. |
| Liquidation | KEEP | Keep liquidation sweep reversal and cascade exhaustion only when liquidation feed quality passes. |
| Order Flow | MERGE | Merge CVD/delta/order-book variants into one cluster to avoid correlated trade spam. |
| Smart Money / Structure | MERGE | Merge FVG, order block, MSS/CHOCH into 3 structure strategies. |
| Generic 100+ scalper variants | DELETE FROM LIVE | Keep in research/backtest only. Too correlated and hard to validate live. |
| AI-only signal variants | WATCHLIST | Require offline validation and deterministic fallback before live eligibility. |

### Production Strategy Basket

1. Trend EMA/ADX continuation
2. Trend MSS continuation
3. Volatility breakout with ATR expansion
4. Range mean reversion
5. Keltner/Bollinger squeeze fade
6. Funding mean reversion
7. CVD divergence
8. Delta absorption
9. Liquidity pool hunt
10. Liquidation sweep reversal
11. FVG retest
12. Order block retest
13. MSS/CHOCH retest
14. VPOC bounce
15. Session expansion
16. Breakout retest
17. Regime defensive no-trade strategy

## 7. Phase 11 Alpha Deployment

| Alpha | Data Requirements | Signal Logic | Execution Rules | Stop Logic | Exit Logic | Risk Limits | Expected Edge |
|---|---|---|---|---|---|---|---|
| CVD Divergence | Trades, CVD, candles | Price makes HH/LL while CVD fails to confirm | Limit or IOC near rejection level | Beyond swing invalidation or ATR stop | VPOC/liquidity target or time stop | Max 1 active per symbol; cluster cap | Exhausted aggressive buying/selling |
| Delta Divergence | Aggressor delta, tick tape, candles | Delta flips against price extension | IOC only if spread normal | ATR plus structure invalidation | Opposite delta burst or TP | Lower size in high vol | Absorption before reversal |
| Funding Mean Reversion | Funding, OI, price, regime | Extreme funding with crowded OI | Avoid entering near funding timestamp if liquidity poor | Wider stop, smaller size | Funding normalizes or momentum invalidates | Funding exposure cap | Crowded perp positioning unwind |
| Liquidation Sweep Reversal | Liquidation feed, price, volume | Cascade spike then rejection | IOC after rejection candle | Beyond liquidation wick | First liquidity pool or VWAP | High-vol-only cap | Forced selling/buying exhaustion |
| Liquidity Pool Hunt | Order book/liquidity zones, candles | Sweep of known pool plus rejection | Limit on retest, IOC if momentum fast | Beyond swept pool | Opposite liquidity zone | Correlation cluster cap | Stop-run reversal |
| Market Structure Shift | Swing highs/lows, BOS/CHOCH | CHOCH after displacement | Enter on retest, not chase | Structure invalidation | Next structure target | Trend family cap | New local trend formation |
| Fair Value Gap | Candles, displacement, volume | Imbalance created then retested | Limit inside FVG zone | Gap invalidation | Fill continuation target | Structure cap | Imbalance continuation |
| Order Block | Candle structure, volume, trend | Retest of last opposing candle before impulse | Limit at block midpoint | Block invalidation | Prior high/low or R multiple | Smart-money cap | Institutional liquidity defense |
| Volume Profile VPOC | Volume profile, VPOC/HVN/LVN | Bounce or rejection around VPOC/HVN/LVN | Limit near profile level | Profile level break | Next HVN/LVN | Range-regime cap | Auction mean reversion |

## 8. Risk V3 Design

### Pre-Trade Risk

- Validate market data freshness.
- Validate exchange health and throttling budget.
- Enforce kill switch and reconciliation status.
- Compute fractional Kelly from strategy metrics.
- Cap by VaR 95, VaR 99, CVaR, portfolio heat, cluster exposure, funding exposure, gross/net exposure, drawdown scaling, and leverage.
- Return explicit `RiskDecision` with approved size, warnings, and block reason.

### Live Risk

- Mark positions on every tick.
- Recompute heat, drawdown, liquidation distance, funding exposure, and correlated clusters.
- Trigger `RISK_TRIGGERED` for reduce-only, hedge, or halt.
- Trigger `KILLSWITCH_TRIGGERED` on stale data, exchange outage, reconciliation mismatch, max drawdown, or ledger failure.

### Post-Trade Risk

- Attribute PnL by strategy, family, alpha source, regime, and execution venue.
- Update strategy health and fractional Kelly inputs.
- Detect slippage drift, adverse selection, and regime-specific degradation.
- Feed promotion/demotion gates.

## 9. Performance Targets

| Path | Target | Design |
|---|---:|---|
| Signal evaluation | <50ms | Strategy worker pool, per-symbol feature cache, no DB calls in hot path |
| Risk check | <10ms | In-memory portfolio snapshot, lock-minimized reads, precomputed correlation matrix |
| Order submission | <50ms | Reused HTTP clients, pre-signed adapter context, rate budget cache |
| End-to-end | <150ms | Co-locate market data, strategy, risk, OMS in one pod for v1 live runtime |
| Tick throughput | 1000+/sec | Bounded channels, ring buffers, feature cache, batch projection writes |

### Goroutine Model

- One feed goroutine per exchange stream.
- One normalizer worker pool per data type.
- One strategy worker pool per symbol/regime.
- One single-writer OMS goroutine per account.
- One risk goroutine with immutable snapshot reads and synchronous pre-trade method.
- One exchange gateway worker per venue/account.
- One reconciliation scheduler.
- One ledger writer with durable queue and batch append.

### Cache Strategy

- L1 in-process ring buffers for ticks, candles, order book, and features.
- Redis for cross-service latest state, dedupe keys, throttling counters, and hot projections.
- PostgreSQL/Timescale for durable events and historical analytics.

## 10. Security V3

### Problem
Live trading introduces API key, operator action, and network blast-radius risks.

### Risk
Compromised dashboard, leaked keys, or unauthenticated API calls can place unauthorized orders or disable risk protections.

### Solution

- Zero-trust API gateway in front of all command endpoints.
- Exchange keys stored in AWS Secrets Manager or SOPS-encrypted Kubernetes secrets, never in repo or client env.
- RBAC roles: Viewer, Researcher, Trader, RiskOfficer, Admin.
- MFA required for Trader/RiskOfficer/Admin.
- Kill switch requires signed operator identity and writes immutable audit event.
- Docker runs non-root, read-only filesystem, dropped Linux capabilities, seccomp profile.
- Network segmentation: UI cannot talk to exchange gateway directly.
- Rate limits by user, account, endpoint, and command type.
- Audit log every command, config change, key rotation, risk override, order action.

### Files/Modules Affected
- `client/src/app/api/`
- `client/src/lib/jwtSession.ts`
- `engine/cmd/antigravity/`
- `engine/internal/admin/`
- `docker-compose.prod.yml`
- Future `deploy/k8s/`

### Architecture Impact
Command plane becomes authenticated and auditable. Data plane remains isolated.

### Latency Impact
No hot-path impact except exchange gateway auth, which is required.

### Risk Reduction
Prevents UI compromise from becoming direct exchange compromise.

### Priority
P0 for live trading, P1 for paper/research.

### Estimated Effort
5-8 days.

## 11. Deployment Blueprint

### Small-Fund Production V1

- Kubernetes cluster with 3 nodes minimum.
- Namespaces: `live`, `paper`, `research`, `observability`, `data`.
- `live` namespace has strict network policies and smallest blast radius.
- Redis with AOF enabled for streams/cache.
- PostgreSQL/TimescaleDB managed service preferred.
- Prometheus, Grafana, Loki, Alertmanager.
- Object storage backups for event ledger and snapshots.

### Services

| Service | Replicas | Notes |
|---|---:|---|
| `market-data-service` | 2 | Active/passive per exchange stream; one writer elected |
| `strategy-service` | 2 | Stateless workers consume normalized market data |
| `risk-service` | 2 | One active per account, standby catches up via ledger |
| `oms-service` | 2 | Leader election per account; idempotent commands |
| `exchange-gateway-service` | 2 | Strict rate budget and adapter isolation |
| `reconciliation-service` | 1-2 | Periodic and startup reconciliation |
| `ledger-writer-service` | 2 | Idempotent append, unique keys |
| `analytics-service` | 1-2 | Read-only projections |
| `terminal-api` | 2 | Authenticated command/read API |
| `research-worker` | scale-to-zero allowed | No live account access |
| `backtest-worker` | scale-to-zero allowed | No live account access |

### Backup and Disaster Recovery

- PITR enabled for PostgreSQL/Timescale.
- Daily full backup, 5-minute WAL archive.
- Redis AOF plus periodic RDB.
- Event ledger exported to object storage daily.
- DR runbook: freeze trading, restore DB, replay ledger, reconcile exchange, only then unfreeze.
- RTO target: 30 minutes. RPO target: 5 minutes for DB, 0 for exchange state after reconciliation.

## 12. Go-Live Checklist

Every item must be PASS before live trading.

### Execution

- PASS/FAIL: OMS V3 state machine implemented.
- PASS/FAIL: Client order IDs idempotent across retries.
- PASS/FAIL: Partial fills supported.
- PASS/FAIL: Cancel and replace supported.
- PASS/FAIL: Stale order cleanup tested.
- PASS/FAIL: Reconciliation loop blocks trading on mismatch.

### Risk

- PASS/FAIL: Pre-trade risk uses fractional Kelly, heat, VaR, CVaR, correlation, funding exposure.
- PASS/FAIL: Live risk marks every position on tick.
- PASS/FAIL: Kill switch tested for stale data, drawdown, ledger failure, and exchange outage.
- PASS/FAIL: Drawdown scaling reduces size before halt.

### Security

- PASS/FAIL: No exchange secrets in client or repo.
- PASS/FAIL: MFA and RBAC enabled for trading commands.
- PASS/FAIL: Audit log immutable.
- PASS/FAIL: Docker containers hardened.
- PASS/FAIL: Network policies isolate live gateway.

### Monitoring

- PASS/FAIL: Prometheus metrics for latency, fills, rejects, risk triggers, reconciliation.
- PASS/FAIL: Grafana live dashboard.
- PASS/FAIL: Loki logs with correlation IDs.
- PASS/FAIL: Alertmanager routes P0 alerts.

### Strategy Validation

- PASS/FAIL: Live basket reduced to 15-25 strategies.
- PASS/FAIL: Every live strategy has OOS and walk-forward evidence.
- PASS/FAIL: Strategy correlation clusters enforced.
- PASS/FAIL: Degraded strategy auto-demotion enabled.

### Backtest Validation

- PASS/FAIL: Slippage, fees, funding, partial fills modeled.
- PASS/FAIL: Walk-forward and Monte Carlo reports generated.
- PASS/FAIL: No lookahead bias checks pass.
- PASS/FAIL: Regime-specific performance documented.

### Data Validation

- PASS/FAIL: Market data freshness checks.
- PASS/FAIL: Candle completeness checks.
- PASS/FAIL: Order book and liquidation feed quality checks.
- PASS/FAIL: Backfill and repair path tested.

### Exchange Validation

- PASS/FAIL: Sandbox/testnet adapter passes.
- PASS/FAIL: API throttling respected.
- PASS/FAIL: WebSocket reconnect tested.
- PASS/FAIL: Exchange outage mode blocks new orders.

## 13. Folder Structure

```text
engine/
  cmd/
    market-data-service/
    strategy-service/
    risk-service/
    oms-service/
    exchange-gateway-service/
    reconciliation-service/
    ledger-writer-service/
  internal/
    events/
    ledger/
    marketdata/
    strategy/
    alpha/
    risk/v3/
    oms/v3/
    exchange/
    reconciliation/
    projections/
    observability/
client/
  src/app/terminal/
  src/components/terminal/institutional/
  src/lib/terminal/
  src/app/api/terminal/
deploy/
  k8s/
    live/
    paper/
    research/
    observability/
infrastructure/
  database/
  performance/
  security/
```

## 14. Service Map

| Domain | Service | Inputs | Outputs |
|---|---|---|---|
| Data | `market-data-service` | Exchange WS/REST | Normalized market events |
| Alpha | `strategy-service` | Market events, feature cache | Signal proposals |
| Routing | `signal-router` | Signal proposals | Approved/rejected signals |
| Risk | `risk-service` | Signals, portfolio state | Risk decisions |
| Execution | `oms-service` | Risk-approved orders | Order commands/events |
| Exchange | `exchange-gateway-service` | Order commands | Exchange acks/fills |
| Control | `reconciliation-service` | Exchange snapshots, ledger projections | Mismatch/resolve events |
| Truth | `ledger-writer-service` | All critical events | Event store |
| Analytics | `analytics-service` | Event store | Read models |
| UI | `terminal-api` | Operator commands/read requests | Authenticated responses |

## 15. Database Schema

Core tables:

- `event_store`
- `event_snapshots`
- `orders_projection`
- `positions_projection`
- `fills_projection`
- `risk_decisions`
- `risk_alerts`
- `strategy_metrics`
- `strategy_registry_live`
- `market_ticks`
- `market_orderbook_snapshots`
- `market_funding_snapshots`
- `market_liquidations`
- `reconciliation_runs`
- `reconciliation_exceptions`
- `operator_audit_log`

## 16. 90-Day Roadmap

### Days 1-15: P0 Truth and OMS

- Implement ledger package and append API.
- Upgrade OMS to V3 state model.
- Add idempotent order creation and replay tests.
- Add startup replay.
- Add reconciliation skeleton.

### Days 16-30: P0 Risk and Exchange Safety

- Create Risk V3 facade over current Risk V2.
- Add live risk loop and kill switch events.
- Add exchange gateway contract and testnet adapter.
- Add stale market data and API throttling gates.

### Days 31-45: P1 Strategy Consolidation

- Reduce live basket to 15-25 strategies.
- Move remaining strategies to research-only registry.
- Add promotion/demotion workflow.
- Add correlation cluster enforcement.

### Days 46-60: P1 Observability and Security

- Add Prometheus metrics for hot path latency.
- Add Loki structured logs with correlation IDs.
- Add RBAC, MFA hooks, and signed operator actions.
- Harden Docker and add Kubernetes manifests.

### Days 61-75: P1 Deployment and DR

- Deploy Redis/NATS, Timescale/Postgres, Prometheus, Grafana, Loki.
- Add PITR and backup automation.
- Add failover rehearsal.
- Add disaster recovery runbook.

### Days 76-90: P2 Validation and Controlled Go-Live

- Run paper-live shadow mode.
- Run testnet mode with exchange reconciliation.
- Run production dry-run with orders disabled.
- Enable live with tiny capped notional and 24/7 monitoring.
- Scale only after checklist remains PASS for two weeks.

## 17. Readiness Score Impact

Expected score movement:

- Event ledger and replay: +4
- OMS V3 and reconciliation: +4
- Runtime separation: +3
- Risk V3 live/pre/post split: +4
- Strategy consolidation: +2
- Security V3: +3
- Observability and DR: +4

Projected readiness: 80-82/100 if P0 and P1 items are completed and tested.

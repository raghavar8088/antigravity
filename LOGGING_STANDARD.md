# Logging Standard — Phase 15H

## Format

All log output is **JSON only**. No plain text. Every log line is a single JSON object terminated by a newline.

---

## Mandatory Fields

Every log record must include:

| Field | Type | Example |
|-------|------|---------|
| `timestamp` | string (RFC3339Nano) | `2026-06-02T14:23:01.123456789Z` |
| `severity` | string | `INFO`, `WARN`, `ERROR`, `RISK`, `TRADING` |
| `service` | string | `trading-engine` |
| `message` | string | `order accepted` |
| `event_type` | string | `ORDER_ACCEPTED` |

## Optional (contextual) Fields

| Field | Type | When |
|-------|------|------|
| `component` | string | Every structured call via `WithFields()` |
| `account_id` | string | Any per-account event |
| `trace_id` | string | All request-path events |
| `request_id` | string | HTTP handlers |
| `exchange` | string | Market data, execution, reconciliation |
| `symbol` | string | Market data, execution |
| `order_id` | string | OMS events |
| `strategy` | string | Signal, evaluation events |
| `incident_id` | string | Incident lifecycle events |
| `risk_score` | float | Risk gate decisions |
| `drift_score` | float | Reconciliation cycles |

---

## Severity Levels

| Level | slog.Level value | Use case |
|-------|-----------------|----------|
| `DEBUG` | -4 | Detailed trace; disabled in production |
| `INFO` | 0 | Normal operational events |
| `WARN` | 4 | Degraded but recoverable |
| `ERROR` | 8 | Failures requiring attention |
| `SECURITY` | 12 | Auth failures, RBAC denials, JWT errors |
| `RISK` | 14 | Risk gate blocks, limit breaches |
| `TRADING` | 16 | Orders, fills, signals |
| `RECONCILIATION` | 18 | Drift detections, reconciliation cycles |
| `FATAL` | 20 | Kill switch, data integrity failures |

---

## Event Types

### Market Data
- `MARKET_DATA_TICK` — Tick received
- `MARKET_DATA_STALE` — Feed staleness detected
- `EXCHANGE_CONNECTED` / `EXCHANGE_DISCONNECTED` — Feed status

### Trading
- `TRADING_SIGNAL` — Strategy signal generated
- `ORDER_SUBMITTED` / `ORDER_ACCEPTED` / `ORDER_REJECTED` / `ORDER_CANCELLED`
- `ORDER_FILLED` / `ORDER_PARTIALLY_FILLED`

### Risk
- `RISK_APPROVED` / `RISK_BLOCKED` — Risk gate decisions
- `RISK_VIOLATION` — Limit breach detected
- `RISK_ALERT_FIRED` — Portfolio alert triggered

### Execution
- `EXECUTION_FILL` — Fill confirmed
- `KILL_SWITCH_ACTIVATED` / `KILL_SWITCH_DEACTIVATED`
- `SLIPPAGE_SPIKE` — Slippage exceeds threshold

### Ledger
- `LEDGER_EVENT_WRITTEN` / `LEDGER_WRITE_ERROR`
- `SNAPSHOT_CREATED` / `SNAPSHOT_LOADED`
- `REPLAY_STARTED` / `REPLAY_COMPLETED`

### Reconciliation
- `RECONCILIATION_CYCLE` — Cycle completed
- `GHOST_POSITION_DETECTED` — Position mismatch
- `MISSING_FILL_DETECTED` — Fill mismatch
- `RECONCILIATION_REPAIR` — Auto-repair executed

### Security
- `SECURITY_AUTH_SUCCESS` / `SECURITY_AUTH_FAILURE`
- `SECURITY_JWT_ERROR`
- `SECURITY_RBAC_DENIED`
- `SECURITY_RATE_LIMITED`
- `SECURITY_INCIDENT`

### Incident
- `INCIDENT_TRIGGERED` / `INCIDENT_ACKNOWLEDGED` / `INCIDENT_RESOLVED`

### DR
- `DR_SNAPSHOT_CREATED` / `DR_BACKUP_COMPLETED`
- `DR_STATUS` / `DR_READINESS_DEGRADED`

---

## Initialisation

```go
observability.Init(observability.Config{
    ServiceName: "trading-engine",
    LogLevel:    slog.LevelInfo,
    MetricsPath: "/metrics",
})
```

## Usage Examples

```go
// Signal event
observability.LogSignal(ctx, "EMA_CROSS_15", "BTCUSDT", "long", 0.87)

// Order event
observability.LogOrder(ctx, "ACCEPTED", "delta", "BTCUSDT", "buy", orderID, qty, price)

// Risk decision
observability.LogRiskDecision(ctx, "BLOCKED", "daily_loss_limit", "BTCUSDT", 42.3)

// Reconciliation cycle
observability.LogReconciliation(ctx, "delta", "POSITION", 0.0, 0)

// Kill switch
observability.LogKillSwitch(ctx, "daily_loss_exceeded", "ALL_TRADING")

// Security event
observability.LogSecurity(ctx, "AUTH_FAILURE", "unknown@ip", "/api/admin/kill", false)

// Component logger
log := observability.WithFields("omsv3", "BTC_PAPER")
log.Info("order created", slog.String("order_id", id))
```

---

## Loki Query Examples

```logql
# All CRITICAL events
{service="trading-engine"} | json | severity="FATAL"

# Kill switch activations
{service="trading-engine"} | json | event_type="KILL_SWITCH_ACTIVATED"

# Ghost positions
{service="trading-engine"} | json | event_type="GHOST_POSITION_DETECTED"

# Risk blocks for specific account
{service="trading-engine"} | json | event_type="RISK_BLOCKED" | account_id="BTC_PAPER"

# Trace correlation — follow a single order
{service="trading-engine"} | json | trace_id="<trace_id>"

# All trading events for a symbol
{service="trading-engine"} | json | severity="TRADING" | symbol="BTCUSDT"
```

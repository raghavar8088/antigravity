// Package observability provides the complete institutional observability stack:
// Prometheus metrics, structured logging, distributed tracing, health checks,
// latency analytics, business metrics, incident management, and DR monitoring.
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────
// Market Data Metrics
// ─────────────────────────────────────────────

var (
	// TicksReceived counts raw ticks ingested per exchange/symbol.
	TicksReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "ticks_received_total",
		Help:      "Total ticks received from exchange websocket feeds.",
	}, []string{"exchange", "symbol"})

	// TicksDropped counts ticks discarded due to queue saturation or parsing errors.
	TicksDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "ticks_dropped_total",
		Help:      "Total ticks dropped (queue full or parse error) per exchange/symbol.",
	}, []string{"exchange", "symbol", "reason"})

	// TickProcessingLatency measures end-to-end tick processing time (receive→strategy).
	TickProcessingLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "tick_processing_latency_ms",
		Help:      "Latency from tick receive to strategy evaluation, milliseconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100, 250, 500},
	}, []string{"exchange", "symbol"})

	// MarketDataStaleness tracks seconds since the last tick was received.
	MarketDataStaleness = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "staleness_seconds",
		Help:      "Seconds since the last tick was received per exchange/symbol. >5 = stale.",
	}, []string{"exchange", "symbol"})

	// ExchangeConnected is 1 when the websocket feed is up, 0 when disconnected.
	ExchangeConnected = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "exchange_connected",
		Help:      "1 if exchange websocket is connected, 0 if disconnected.",
	}, []string{"exchange"})

	// ExchangeReconnects counts websocket reconnection attempts.
	ExchangeReconnects = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "reconnects_total",
		Help:      "Total websocket reconnection attempts per exchange.",
	}, []string{"exchange", "result"})

	// TickThroughput is the current ticks/second rate per exchange.
	TickThroughput = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "throughput_tps",
		Help:      "Current tick throughput in ticks per second per exchange.",
	}, []string{"exchange"})

	// LastTickPrice is the most recent mid-price per symbol.
	LastTickPrice = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "marketdata",
		Name:      "last_price",
		Help:      "Most recent tick mid-price per exchange/symbol.",
	}, []string{"exchange", "symbol"})
)

// ─────────────────────────────────────────────
// Strategy Layer Metrics
// ─────────────────────────────────────────────

var (
	// StrategyEvaluations counts total strategy evaluation calls.
	StrategyEvaluations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "evaluations_total",
		Help:      "Total strategy evaluations per strategy family and symbol.",
	}, []string{"family", "symbol"})

	// StrategySignals counts signals generated (entry/exit) per strategy.
	StrategySignals = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "signals_total",
		Help:      "Total trading signals generated per strategy and direction.",
	}, []string{"strategy", "symbol", "direction"})

	// StrategyEvaluationLatency measures time to evaluate a single strategy.
	StrategyEvaluationLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "evaluation_latency_us",
		Help:      "Per-strategy evaluation latency in microseconds.",
		Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"family"})

	// StrategyErrors counts evaluation errors per strategy.
	StrategyErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "errors_total",
		Help:      "Total strategy evaluation errors per strategy.",
	}, []string{"strategy", "error_type"})

	// ActiveStrategies is the current count of enabled strategies.
	ActiveStrategies = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "active_count",
		Help:      "Number of currently active (enabled) strategies per family.",
	}, []string{"family"})

	// StrategyWinRate is the rolling 30-day win rate per strategy.
	StrategyWinRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "win_rate_pct",
		Help:      "Rolling 30-day win rate percentage per strategy.",
	}, []string{"strategy"})

	// SignalToRiskLatency measures time from signal generation to risk gate decision.
	SignalToRiskLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "strategy",
		Name:      "signal_to_risk_latency_ms",
		Help:      "Latency from signal generation to risk gate decision, milliseconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50},
	}, []string{"symbol"})
)

// ─────────────────────────────────────────────
// OMS (Order Management System v3) Metrics
// ─────────────────────────────────────────────

var (
	// OrdersSubmitted counts orders sent to the exchange.
	OrdersSubmitted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "orders_submitted_total",
		Help:      "Total orders submitted to exchange per symbol and side.",
	}, []string{"exchange", "symbol", "side", "order_type"})

	// OrdersAccepted counts orders acknowledged by the exchange.
	OrdersAccepted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "orders_accepted_total",
		Help:      "Total orders accepted by the exchange.",
	}, []string{"exchange", "symbol"})

	// OrdersRejected counts orders rejected by exchange or risk gate.
	OrdersRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "orders_rejected_total",
		Help:      "Total orders rejected, by rejection source and reason.",
	}, []string{"exchange", "symbol", "source", "reason"})

	// OrdersCancelled counts orders cancelled (user-initiated or timeout).
	OrdersCancelled = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "orders_cancelled_total",
		Help:      "Total orders cancelled per exchange/symbol.",
	}, []string{"exchange", "symbol", "reason"})

	// PartialFills counts partially filled orders.
	PartialFills = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "partial_fills_total",
		Help:      "Total partially-filled orders per exchange/symbol.",
	}, []string{"exchange", "symbol"})

	// FillLatency measures time from order submission to fill confirmation.
	FillLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "fill_latency_ms",
		Help:      "Order fill latency from submission to exchange confirmation, milliseconds.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
	}, []string{"exchange", "symbol", "side"})

	// OpenOrders is the current number of open (pending) orders.
	OpenOrders = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "open_orders",
		Help:      "Current number of open orders awaiting fill or cancellation.",
	}, []string{"exchange", "symbol"})

	// OrderRejectRate is the rolling reject rate (rejected / submitted).
	OrderRejectRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "reject_rate",
		Help:      "Rolling order reject rate (0.0–1.0) per exchange/symbol.",
	}, []string{"exchange", "symbol"})

	// RiskToOMSLatency measures time from risk approval to OMS order creation.
	RiskToOMSLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "risk_to_oms_latency_ms",
		Help:      "Latency from risk gate approval to OMS order created, milliseconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 25},
	}, []string{"exchange"})

	// OMSToExchangeLatency measures time from OMS order to exchange acknowledgement.
	OMSToExchangeLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "oms",
		Name:      "oms_to_exchange_latency_ms",
		Help:      "Latency from OMS dispatch to exchange order acknowledgement, milliseconds.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500},
	}, []string{"exchange"})
)

// ─────────────────────────────────────────────
// Execution Metrics
// ─────────────────────────────────────────────

var (
	// ExecutionLatency is the full signal-to-fill pipeline latency.
	ExecutionLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "execution",
		Name:      "signal_to_fill_latency_ms",
		Help:      "End-to-end latency from signal generation to exchange fill confirmation, milliseconds. Target <150ms.",
		Buckets:   []float64{5, 10, 25, 50, 75, 100, 150, 200, 300, 500, 1000},
	}, []string{"exchange", "symbol"})

	// Slippage measures signed slippage in basis points vs expected price.
	Slippage = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "execution",
		Name:      "slippage_bps",
		Help:      "Execution slippage in basis points (positive = unfavorable).",
		Buckets:   []float64{-50, -20, -10, -5, 0, 5, 10, 20, 50, 100, 200},
	}, []string{"exchange", "symbol", "side"})

	// FillQuality is the ratio of filled-qty to requested-qty (1.0 = perfect).
	FillQuality = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "execution",
		Name:      "fill_quality_ratio",
		Help:      "Fill quality: ratio of filled quantity to requested quantity (1.0 = complete fill).",
		Buckets:   []float64{0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99, 1.0},
	}, []string{"exchange", "symbol"})

	// ExecutionErrors counts execution pipeline errors.
	ExecutionErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "execution",
		Name:      "errors_total",
		Help:      "Total execution pipeline errors per exchange/error type.",
	}, []string{"exchange", "error_type"})

	// KillSwitchActivations counts kill switch trigger events.
	KillSwitchActivations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "execution",
		Name:      "kill_switch_activations_total",
		Help:      "Total kill switch activation events per trigger reason.",
	}, []string{"reason", "scope"})

	// KillSwitchActive is 1 if the kill switch is currently active.
	KillSwitchActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execution",
		Name:      "kill_switch_active",
		Help:      "1 if the kill switch is currently active (all trading halted), 0 otherwise.",
	})
)

// ─────────────────────────────────────────────
// Ledger / Event Store Metrics
// ─────────────────────────────────────────────

var (
	// EventsWritten counts events written to the ledger.
	EventsWritten = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "events_written_total",
		Help:      "Total events written to the event store per aggregate type.",
	}, []string{"aggregate_type", "event_type"})

	// EventWriteLatency measures ledger write latency.
	EventWriteLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "write_latency_ms",
		Help:      "Event store write latency in milliseconds.",
		Buckets:   []float64{0.5, 1, 2, 5, 10, 25, 50, 100, 250},
	}, []string{"aggregate_type"})

	// LedgerWriteErrors counts write failures.
	LedgerWriteErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "write_errors_total",
		Help:      "Total event write errors per aggregate type and error class.",
	}, []string{"aggregate_type", "error_class"})

	// ReplayLatency measures how long a full event replay takes.
	ReplayLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "replay_latency_ms",
		Help:      "Full event replay latency in milliseconds.",
		Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"aggregate_type"})

	// ReplayThroughput is the current event replay rate in events/sec.
	ReplayThroughput = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "replay_throughput_eps",
		Help:      "Current replay throughput in events per second.",
	}, []string{"aggregate_type"})

	// SnapshotCreations counts snapshot creation events.
	SnapshotCreations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "snapshots_created_total",
		Help:      "Total snapshots created per aggregate type.",
	}, []string{"aggregate_type", "result"})

	// SnapshotLoadLatency measures time to load a snapshot from storage.
	SnapshotLoadLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "snapshot_load_latency_ms",
		Help:      "Snapshot load latency in milliseconds.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500},
	}, []string{"aggregate_type"})

	// EventStoreSize is the approximate number of events in the store.
	EventStoreSize = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "store_size_events",
		Help:      "Approximate number of events currently in the event store.",
	}, []string{"aggregate_type"})

	// FillToLedgerLatency measures time from exchange fill to ledger write.
	FillToLedgerLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "ledger",
		Name:      "fill_to_ledger_latency_ms",
		Help:      "Latency from exchange fill confirmation to ledger event written, milliseconds.",
		Buckets:   []float64{0.5, 1, 2, 5, 10, 25, 50, 100},
	}, []string{"exchange"})
)

// ─────────────────────────────────────────────
// Infrastructure Metrics (DB / Redis / Vault)
// ─────────────────────────────────────────────

var (
	// DBQueryLatency measures database query execution time.
	DBQueryLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "db",
		Name:      "query_latency_ms",
		Help:      "Database query execution latency in milliseconds.",
		Buckets:   []float64{0.5, 1, 2, 5, 10, 25, 50, 100, 250, 500},
	}, []string{"db", "operation", "table"})

	// DBErrors counts database errors.
	DBErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "db",
		Name:      "errors_total",
		Help:      "Total database errors per db/operation/error_class.",
	}, []string{"db", "operation", "error_class"})

	// DBConnectionPool tracks pool utilisation.
	DBConnectionPool = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "db",
		Name:      "pool_connections",
		Help:      "Current database connection pool utilisation (total/idle/in-use).",
	}, []string{"db", "state"})

	// DBAvailable is 1 when the database is reachable, 0 otherwise.
	DBAvailable = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "db",
		Name:      "available",
		Help:      "1 if the database is available and health check passed, 0 otherwise.",
	}, []string{"db"})

	// RedisCommandLatency measures Redis command execution time.
	RedisCommandLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "redis",
		Name:      "command_latency_ms",
		Help:      "Redis command execution latency in milliseconds.",
		Buckets:   []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 25},
	}, []string{"command"})

	// RedisErrors counts Redis operation errors.
	RedisErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "redis",
		Name:      "errors_total",
		Help:      "Total Redis operation errors per command/error type.",
	}, []string{"command", "error_type"})

	// RedisAvailable is 1 when Redis is reachable.
	RedisAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "redis",
		Name:      "available",
		Help:      "1 if Redis is available and health check passed, 0 otherwise.",
	})

	// VaultAvailable is 1 when HashiCorp Vault is reachable.
	VaultAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "vault",
		Name:      "available",
		Help:      "1 if Vault is reachable and sealed=false, 0 otherwise.",
	})

	// VaultSecretReadLatency measures Vault secret fetch latency.
	VaultSecretReadLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "vault",
		Name:      "secret_read_latency_ms",
		Help:      "Vault secret read latency in milliseconds.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250},
	})
)

// ─────────────────────────────────────────────
// Paper Account Recovery Metrics
// ─────────────────────────────────────────────

var (
	// NegativeBalanceRecoveries counts how many times MongoDB recovery returned a
	// negative paper balance that had to be clamped to zero. A non-zero value
	// indicates a bookkeeping inconsistency and should trigger a CRITICAL alert.
	NegativeBalanceRecoveries = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "paper_oms",
		Name:      "negative_balance_recoveries_total",
		Help:      "Total times MongoDB recovery produced a negative balance clamped to 0. Non-zero = bookkeeping inconsistency.",
	})
)

// ─────────────────────────────────────────────
// Security Metrics
// ─────────────────────────────────────────────

var (
	// AuthAttempts counts authentication attempts.
	AuthAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "security",
		Name:      "auth_attempts_total",
		Help:      "Total authentication attempts per result.",
	}, []string{"method", "result"})

	// JWTErrors counts JWT validation failures.
	JWTErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "security",
		Name:      "jwt_errors_total",
		Help:      "Total JWT validation errors per error type.",
	}, []string{"error_type"})

	// RBACDenials counts RBAC access denials.
	RBACDenials = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "security",
		Name:      "rbac_denials_total",
		Help:      "Total RBAC access denials per resource and role.",
	}, []string{"resource", "role"})

	// RateLimitHits counts requests throttled by rate limiting.
	RateLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "security",
		Name:      "rate_limit_hits_total",
		Help:      "Total requests blocked by rate limiting per endpoint.",
	}, []string{"endpoint", "client_ip"})

	// SecurityIncidents counts security incident events.
	SecurityIncidents = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "security",
		Name:      "incidents_total",
		Help:      "Total security incidents per severity and type.",
	}, []string{"severity", "incident_type"})
)

// ─────────────────────────────────────────────
// API Layer Metrics
// ─────────────────────────────────────────────

var (
	// HTTPRequestsTotal counts all HTTP requests.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "api",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests handled per method/path/status.",
	}, []string{"method", "path", "status_class"})

	// HTTPRequestDuration measures HTTP handler execution time.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "api",
		Name:      "http_request_duration_ms",
		Help:      "HTTP request handler duration in milliseconds.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{"method", "path"})

	// HTTPErrorsTotal counts HTTP 4xx/5xx responses.
	HTTPErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "api",
		Name:      "http_errors_total",
		Help:      "Total HTTP error responses per method/path/status_code.",
	}, []string{"method", "path", "status_code"})

	// ActiveHTTPConnections is the current open connection count.
	ActiveHTTPConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "api",
		Name:      "active_connections",
		Help:      "Current number of active HTTP connections.",
	})
)

// ─────────────────────────────────────────────
// Background Worker Metrics
// ─────────────────────────────────────────────

var (
	// WorkerHeartbeat is updated by each background worker to indicate it is alive.
	WorkerHeartbeat = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "worker",
		Name:      "last_heartbeat_unix",
		Help:      "Unix timestamp of the last heartbeat from each background worker.",
	}, []string{"worker"})

	// WorkerIterations counts worker loop iterations.
	WorkerIterations = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "worker",
		Name:      "iterations_total",
		Help:      "Total iterations completed per background worker.",
	}, []string{"worker", "result"})

	// WorkerPanicRecoveries counts worker goroutine panic recoveries.
	WorkerPanicRecoveries = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "worker",
		Name:      "panic_recoveries_total",
		Help:      "Total panic recoveries in background worker goroutines.",
	}, []string{"worker"})
)

// Mock trading pipeline health (post-fix Sev-1 recovery observability).
var (
	MockTradingLastTickUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "last_tick_unix",
		Help:      "Unix timestamp of the last market tick processed by the orchestrator.",
	})

	MockTradingLastSignalUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "last_signal_unix",
		Help:      "Unix timestamp of the last approved signal entering institutional execution.",
	})

	MockTradingLastFillUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "last_fill_unix",
		Help:      "Unix timestamp of the last simulated paper fill.",
	})

	MockTradingFillsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "fills_total",
		Help:      "Total mock paper fills since engine boot.",
	})

	MockTradingSignalsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "signals_total",
		Help:      "Total approved signals entering institutional execution since boot.",
	})

	MockTradingNoTradeAlertTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "no_trade_alerts_total",
		Help:      "No-trade watchdog alerts by threshold window.",
	}, []string{"window"})

	MockTradingWatchdogAlertsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "mock",
		Name:      "watchdog_alerts_total",
		Help:      "Execution watchdog alerts by type.",
	}, []string{"alert_type"})
)

// ─────────────────────────────────────────────
// Phase 1–5 Upgrade Metrics
// ─────────────────────────────────────────────

var (
	// DataQualityCandlesDropped counts 1m candles dropped by the data quality gate.
	DataQualityCandlesDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "dataquality",
		Name:      "candles_dropped_total",
		Help:      "Total 1m candles dropped by the data quality validator, by action and exchange.",
	}, []string{"action", "exchange"})

	// DataQualityScore is the most recent quality score emitted by the validator.
	DataQualityScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "dataquality",
		Name:      "score",
		Help:      "Most recent data quality score (0–100) per exchange/symbol.",
	}, []string{"exchange", "symbol"})

	// DataQualityChecks counts individual quality check results.
	DataQualityChecks = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "dataquality",
		Name:      "checks_total",
		Help:      "Total data quality checks completed per check name and result.",
	}, []string{"check", "result"})

	// RegimeClassifications counts regime classification events.
	RegimeClassifications = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "regime",
		Name:      "classifications_total",
		Help:      "Total regime classification events per regime type.",
	}, []string{"regime"})

	// CurrentRegime records the current market regime as a label.
	CurrentRegime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "regime",
		Name:      "active",
		Help:      "1 if the labelled regime is currently active, 0 otherwise.",
	}, []string{"regime"})

	// StrategyGateFiltered counts strategies blocked by the regime strategy gate.
	StrategyGateFiltered = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "regime",
		Name:      "strategies_filtered_total",
		Help:      "Total strategy signals filtered by the regime strategy gate.",
	}, []string{"regime", "family"})

	// AsyncScorerCacheHits counts async scorer cache hits/misses.
	AsyncScorerCacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "aiscoring",
		Name:      "cache_lookups_total",
		Help:      "Total async scorer cache lookups, by hit/miss/stale.",
	}, []string{"result"})

	// AsyncScorerQueueDepth is the current number of items pending in the scorer queue.
	AsyncScorerQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "aiscoring",
		Name:      "queue_depth",
		Help:      "Current number of items pending in the async scorer work queue.",
	})

	// CycleGuardBlocks counts 15m cycles blocked by the cycle guard (overlap prevention).
	CycleGuardBlocks = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "cycleguard",
		Name:      "blocks_total",
		Help:      "Total 15m strategy cycles blocked because a prior cycle was still running.",
	})

	// FundingRate is the latest Binance perpetual funding rate.
	FundingRate = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "derivatives",
		Name:      "funding_rate",
		Help:      "Latest Binance perpetual funding rate (raw, e.g. 0.0001 = 0.01%).",
	})

	// OpenInterestUSD is the latest open interest in USD.
	OpenInterestUSD = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "derivatives",
		Name:      "open_interest_usd",
		Help:      "Latest Binance perpetual open interest in USD.",
	})

	// DerivativesScore is the blended derivatives confidence score [-3, +3].
	DerivativesScore = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "derivatives",
		Name:      "composite_score",
		Help:      "Blended derivatives composite score in range [-3, +3]. Positive = bullish.",
	})

	// OrderBookScore is the latest L2 order book imbalance score [-3, +3].
	OrderBookScore = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "orderbook",
		Name:      "imbalance_score",
		Help:      "Latest L2 order book bid/ask imbalance score [-3, +3]. Positive = bid-heavy.",
	})

	// OrderBookSnapshots counts depth snapshots processed by the subscriber.
	OrderBookSnapshots = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "orderbook",
		Name:      "snapshots_total",
		Help:      "Total L2 depth snapshots processed by the depth subscriber.",
	})

	// MicrostructureConfidenceAdjustment is the last confidence delta applied.
	MicrostructureConfidenceAdjustment = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "microstructure",
		Name:      "confidence_adjustment",
		Help:      "Confidence adjustment applied by microstructure weight (signed, 0–100 scale).",
		Buckets:   []float64{-20, -15, -10, -5, -2, 0, 2, 5, 10, 15, 20},
	})

	// KellySizeRatio is the fraction of portfolio allocated by Kelly per signal.
	KellySizeRatio = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "kelly",
		Name:      "size_ratio",
		Help:      "Kelly-computed position size as a fraction of portfolio (0–1).",
		Buckets:   []float64{0.005, 0.01, 0.02, 0.03, 0.05, 0.075, 0.10, 0.15, 0.20},
	})

	// SecretsResolutions counts AWS Secrets Manager fetch results.
	SecretsResolutions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "secrets",
		Name:      "resolutions_total",
		Help:      "Total secret resolution attempts, by source (aws/env) and result (hit/miss/error).",
	}, []string{"source", "result"})

	// RestartReconciliationDuration measures how long restart reconciliation takes.
	RestartReconciliationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "reconciliation",
		Name:      "restart_duration_ms",
		Help:      "Duration of restart reconciliation (ReconcileOnRestart) in milliseconds.",
		Buckets:   []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000},
	})

	// RestartReconciliationClosures counts positions retroactively closed at restart.
	RestartReconciliationClosures = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "reconciliation",
		Name:      "restart_closures_total",
		Help:      "Total positions retroactively closed during restart reconciliation.",
	})
)

// ─────────────────────────────────────────────
// Phase D — AI Intelligence Metrics
// ─────────────────────────────────────────────

var (
	// MCBlockedTotal counts signals blocked by the Monte Carlo pre-trade filter.
	MCBlockedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "montecarlo",
		Name:      "blocked_total",
		Help:      "Total signals blocked by Monte Carlo pre-trade filter, by block reason.",
	}, []string{"reason"})

	// MCExpectedValue is the probability-weighted expected value of the last MC simulation.
	MCExpectedValue = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "montecarlo",
		Name:      "expected_value",
		Help:      "Probability-weighted expected PnL % from last Monte Carlo simulation.",
	})

	// MCProbSL is the probability of stop loss hit in the last MC simulation.
	MCProbSL = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "montecarlo",
		Name:      "prob_sl",
		Help:      "Probability of stop loss hit in the last Monte Carlo simulation (0–1).",
	})

	// CalibrationFactor is the current confidence calibration factor.
	CalibrationFactor = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "calibration",
		Name:      "factor",
		Help:      "Current AI confidence calibration factor (actual/stated win rate ratio).",
	})

	// CalibrationTradesAnalysed is the number of trades in the latest calibration.
	CalibrationTradesAnalysed = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "calibration",
		Name:      "trades_analysed",
		Help:      "Number of closed trades analysed in the most recent calibration run.",
	})
)

// ─────────────────────────────────────────────
// Upgrades 1–6 Metrics
// ─────────────────────────────────────────────

var (
	// WalkForwardStatus tracks each strategy's walk-forward validator status.
	// Value: 1 = the labelled status is active for this strategy, 0 otherwise.
	WalkForwardStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "strategy",
		Name:      "walkforward_status",
		Help:      "Walk-forward validator status per strategy (1=active label, 0=inactive). Labels: PROBATIONARY, ACTIVE, DEMOTED.",
	}, []string{"strategy", "status"})
)

// ─────────────────────────────────────────────
// Scalers Pipeline Metrics
// ─────────────────────────────────────────────

var (
	// ScalersSignalsEvaluated counts total signals evaluated per strategy before gates.
	ScalersSignalsEvaluated = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trading_scalers_signals_evaluated_total",
		Help: "Total signals evaluated per strategy before gates",
	}, []string{"strategy"})

	// ScalersSignalsExecuted counts total signals executed per strategy and direction.
	ScalersSignalsExecuted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trading_scalers_signals_executed_total",
		Help: "Total signals executed per strategy and direction",
	}, []string{"strategy", "direction"})

	// ScalersSignalsRejected counts total signals rejected per strategy and reason.
	ScalersSignalsRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trading_scalers_signals_rejected_total",
		Help: "Total signals rejected per strategy and reason",
	}, []string{"strategy", "reason"})

	// ScalersStrategyStatus tracks walk-forward status per strategy: 0=PROBATIONARY 1=ACTIVE 2=DEMOTED.
	ScalersStrategyStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "trading_scalers_strategy_status",
		Help: "Walk-forward status: 0=PROBATIONARY 1=ACTIVE 2=DEMOTED",
	}, []string{"strategy"})

	// ScalersOIFeedActive is 1 if OI data present in last MarketContext, 0 if missing.
	ScalersOIFeedActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "trading_scalers_oi_feed_active",
		Help: "1 if OI data present in last MarketContext, 0 if missing",
	})

	// ScalersKellyPositionUSD records the Kelly-computed position size (USD) per strategy.
	ScalersKellyPositionUSD = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "trading_scalers_kelly_position_usd",
		Help: "Kelly-computed position size in USD per strategy",
	}, []string{"strategy"})

	// ScalersCVDSource is 1 when CVD is computed from the real Binance aggTrade
	// feed, 0 when falling back to the price-direction proxy.
	ScalersCVDSource = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "trading_scalers_cvd_source",
		Help: "1.0 = real aggTrade feed, 0.0 = price-proxy fallback",
	})

	// ScalersConcentrationBTC tracks current same-direction BTC exposure across
	// concurrently open scalers positions, per direction.
	ScalersConcentrationBTC = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "trading_scalers_concentration_btc",
		Help: "Current same-direction BTC exposure across open scalers positions",
	}, []string{"direction"})

	// ScalersMetaLabelSuppressed counts signals suppressed by the meta-labeling
	// confluence filter, per strategy and failing-axis reason.
	ScalersMetaLabelSuppressed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "trading_scalers_meta_label_suppressed_total",
		Help: "Total signals suppressed by the meta-label confluence filter, per strategy and reason",
	}, []string{"strategy", "reason"})
)

// ─────────────────────────────────────────────
// Phase E — Event Store Metrics
// ─────────────────────────────────────────────

var (
	// EventStoreWrittenTotal counts events successfully written to TimescaleDB.
	EventStoreWrittenTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "btc",
		Name:      "eventstore_events_written_total",
		Help:      "Total events successfully written to the TimescaleDB event store.",
	})

	// EventStoreWriteErrors counts events dropped due to channel full or DB errors.
	EventStoreWriteErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "btc",
		Name:      "eventstore_write_errors_total",
		Help:      "Total events dropped due to channel overflow or write errors.",
	})

	// EventStoreChannelDepth tracks the current depth of the async write buffer.
	EventStoreChannelDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "btc",
		Name:      "eventstore_channel_depth",
		Help:      "Current number of events pending in the event store write buffer.",
	})

	// MLBlockedTotal counts signals blocked by the local ML pre-scorer.
	MLBlockedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "btc",
		Name:      "ml_blocked_total",
		Help:      "Total signals blocked by the local XGBoost ML pre-scorer.",
	})

	// MLAvailable is 1 when the ML model is loaded and active, 0 otherwise.
	MLAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "btc",
		Name:      "ml_available",
		Help:      "1 if the local ML pre-scorer model is loaded and active, 0 otherwise.",
	})
)

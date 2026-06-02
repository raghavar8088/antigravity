package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"
)

// LogLevel maps to slog levels with trading-specific additions.
type LogLevel = slog.Level

const (
	LevelDebug          LogLevel = slog.LevelDebug
	LevelInfo           LogLevel = slog.LevelInfo
	LevelWarn           LogLevel = slog.LevelWarn
	LevelError          LogLevel = slog.LevelError
	LevelSecurity       LogLevel = slog.Level(12)
	LevelRisk           LogLevel = slog.Level(14)
	LevelTrading        LogLevel = slog.Level(16)
	LevelReconciliation LogLevel = slog.Level(18)
	LevelFatal          LogLevel = slog.Level(20)
)

// Logger is the global structured logger for the trading engine.
// All logs are JSON-formatted and include mandatory context fields.
// Defaults to the stdlib slog default (stderr, text format) until InitLogger is called.
var Logger = slog.Default()

// logContextKey is the unexported type for context log fields.
type logContextKey struct{}

// LogFields are mandatory fields attached to every log record.
type LogFields struct {
	Service   string
	Component string
	AccountID string
	RequestID string
	TraceID   string
}

// InitLogger initialises the global JSON structured logger.
// Call once at startup before any other logging.
func InitLogger(service string, level slog.Level, out io.Writer) {
	if out == nil {
		out = os.Stdout
	}
	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Rename "time" → "timestamp" and use RFC3339Nano for institutional precision.
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().UTC().Format(time.RFC3339Nano)),
				}
			}
			// Rename "msg" → "message".
			if a.Key == slog.MessageKey {
				return slog.Attr{Key: "message", Value: a.Value}
			}
			// Rename "level" → "severity".
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)
				return slog.Attr{Key: "severity", Value: slog.StringValue(levelName(level))}
			}
			return a
		},
	})

	Logger = slog.New(handler).With(
		slog.String("service", service),
		slog.String("version", buildVersion()),
	)

	slog.SetDefault(Logger)
}

// WithFields returns a logger enriched with the given structured fields.
func WithFields(component, accountID string) *slog.Logger {
	return Logger.With(
		slog.String("component", component),
		slog.String("account_id", accountID),
	)
}

// WithContext extracts log fields stored in the context and returns an enriched logger.
func WithContext(ctx context.Context) *slog.Logger {
	l := Logger
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		l = l.With(slog.String("trace_id", traceID))
	}
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		l = l.With(slog.String("request_id", requestID))
	}
	return l
}

// LogTick logs a market data tick at DEBUG level.
func LogTick(ctx context.Context, exchange, symbol string, price float64) {
	WithContext(ctx).Debug("tick received",
		slog.String("event_type", "MARKET_DATA_TICK"),
		slog.String("exchange", exchange),
		slog.String("symbol", symbol),
		slog.Float64("price", price),
	)
}

// LogSignal logs a strategy signal at TRADING level.
func LogSignal(ctx context.Context, strategy, symbol, direction string, confidence float64) {
	l := Logger.With(slog.String("severity", "TRADING"))
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		l = l.With(slog.String("trace_id", traceID))
	}
	l.Log(ctx, LevelTrading, "strategy signal",
		slog.String("event_type", "TRADING_SIGNAL"),
		slog.String("strategy", strategy),
		slog.String("symbol", symbol),
		slog.String("direction", direction),
		slog.Float64("confidence", confidence),
	)
}

// LogOrder logs an OMS order event at TRADING level.
func LogOrder(ctx context.Context, event, exchange, symbol, side, orderID string, qty, price float64) {
	Logger.Log(ctx, LevelTrading, "order event",
		slog.String("event_type", "ORDER_"+event),
		slog.String("exchange", exchange),
		slog.String("symbol", symbol),
		slog.String("side", side),
		slog.String("order_id", orderID),
		slog.Float64("qty", qty),
		slog.Float64("price", price),
	)
}

// LogRiskDecision logs a risk gate decision at RISK level.
func LogRiskDecision(ctx context.Context, decision, reason, symbol string, score float64) {
	Logger.Log(ctx, LevelRisk, "risk gate decision",
		slog.String("event_type", "RISK_"+decision),
		slog.String("symbol", symbol),
		slog.String("reason", reason),
		slog.Float64("risk_score", score),
	)
}

// LogReconciliation logs a reconciliation event at RECONCILIATION level.
func LogReconciliation(ctx context.Context, exchange, domain string, driftScore float64, mismatches int) {
	Logger.Log(ctx, LevelReconciliation, "reconciliation cycle",
		slog.String("event_type", "RECONCILIATION_CYCLE"),
		slog.String("exchange", exchange),
		slog.String("domain", domain),
		slog.Float64("drift_score", driftScore),
		slog.Int("mismatches", mismatches),
	)
}

// LogSecurity logs a security event at SECURITY level.
func LogSecurity(ctx context.Context, eventType, actor, resource string, allowed bool) {
	Logger.Log(ctx, LevelSecurity, "security event",
		slog.String("event_type", "SECURITY_"+eventType),
		slog.String("actor", actor),
		slog.String("resource", resource),
		slog.Bool("allowed", allowed),
	)
}

// LogKillSwitch logs a kill switch activation at ERROR level.
func LogKillSwitch(ctx context.Context, reason, scope string) {
	Logger.Log(ctx, LevelFatal, "KILL SWITCH ACTIVATED",
		slog.String("event_type", "KILL_SWITCH_ACTIVATED"),
		slog.String("reason", reason),
		slog.String("scope", scope),
	)
}

func levelName(l slog.Level) string {
	switch {
	case l == LevelFatal:
		return "FATAL"
	case l == LevelReconciliation:
		return "RECONCILIATION"
	case l == LevelTrading:
		return "TRADING"
	case l == LevelRisk:
		return "RISK"
	case l == LevelSecurity:
		return "SECURITY"
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

func buildVersion() string {
	if v := os.Getenv("APP_VERSION"); v != "" {
		return v
	}
	return "dev"
}

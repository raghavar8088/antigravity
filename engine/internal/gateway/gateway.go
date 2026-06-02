// Package gateway implements the institutional API gateway layer.
//
// All inbound traffic passes through:
//
//	Request → RequestID injection → Access log → Security Gate → Handler
//
// The gateway sits between the edge (Vercel/CDN) and the security gate,
// adding request tracing, structured access logging, and panic recovery
// before the security gate performs authn/authz.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// contextKey is unexported to prevent collisions.
type contextKey int

const requestIDKey contextKey = iota

// RequestIDHeader is the canonical header for distributed tracing.
const RequestIDHeader = "X-Request-ID"

// AccessLog is a structured access log entry emitted per request.
type AccessLog struct {
	RequestID  string `json:"request_id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	DurationMs int64  `json:"duration_ms"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent,omitempty"`
	Error      string `json:"error,omitempty"`
}

// AccessLogSink receives structured access logs for persistence/forwarding.
type AccessLogSink func(AccessLog)

// Gateway wraps an http.Handler with request tracing, access logging, and
// panic recovery. It does NOT perform authentication — that is the Security Gate's job.
type Gateway struct {
	next    http.Handler
	sinks   []AccessLogSink
	service string
}

// Config holds construction parameters for the Gateway.
type Config struct {
	// ServiceName is included in logs for multi-service deployments.
	ServiceName string
	// AccessLogSinks receive structured log entries for every request.
	AccessLogSinks []AccessLogSink
}

// New wraps next with the gateway middleware chain.
func New(next http.Handler, cfg Config) *Gateway {
	svc := cfg.ServiceName
	if svc == "" {
		svc = "raig-engine"
	}
	return &Gateway{next: next, sinks: cfg.AccessLogSinks, service: svc}
}

// ServeHTTP implements http.Handler — the gateway entry point.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Inject or propagate request ID for distributed tracing.
	reqID := r.Header.Get(RequestIDHeader)
	if reqID == "" {
		reqID = generateRequestID()
	}
	r = r.WithContext(context.WithValue(r.Context(), requestIDKey, reqID))
	w.Header().Set(RequestIDHeader, reqID)

	// Wrapped response writer to capture status code.
	rw := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

	// Panic recovery — never let a handler crash the engine.
	defer func() {
		if rec := recover(); rec != nil {
			stack := debug.Stack()
			log.Printf("[GATEWAY] PANIC in handler: %v\n%s", rec, stack)
			if !rw.written {
				http.Error(rw, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
			g.emit(AccessLog{
				RequestID:  reqID,
				Method:     r.Method,
				Path:       sanitizePath(r.URL.Path),
				StatusCode: http.StatusInternalServerError,
				DurationMs: time.Since(start).Milliseconds(),
				IP:         clientIP(r),
				UserAgent:  r.UserAgent(),
				Error:      fmt.Sprintf("panic: %v", rec),
			})
		}
	}()

	g.next.ServeHTTP(rw, r)

	g.emit(AccessLog{
		RequestID:  reqID,
		Method:     r.Method,
		Path:       sanitizePath(r.URL.Path),
		StatusCode: rw.statusCode,
		DurationMs: time.Since(start).Milliseconds(),
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
}

// RequestIDFrom extracts the request ID from a context populated by the gateway.
func RequestIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func (g *Gateway) emit(entry AccessLog) {
	// Always emit to structured log.
	b, _ := json.Marshal(entry)
	log.Printf("[ACCESS] %s", b)

	for _, sink := range g.sinks {
		sink(entry)
	}
}

// StdoutSink writes access logs to stdout as structured JSON.
// Use in production where log aggregators (Loki, CloudWatch) scrape stdout.
func StdoutSink(entry AccessLog) {
	b, _ := json.Marshal(entry)
	fmt.Println(string(b))
}

// responseRecorder captures the HTTP status code written by the downstream handler.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.written = true
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.written = true
	return rr.ResponseWriter.Write(b)
}

// generateRequestID produces a random 16-char hex request ID.
func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:gosec — not used for cryptography
	return fmt.Sprintf("%x", b)
}

// clientIP extracts the real client IP, respecting reverse proxy headers.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}

// sanitizePath strips query strings from paths before logging (avoid credential leaks).
func sanitizePath(path string) string {
	if idx := strings.IndexByte(path, '?'); idx != -1 {
		return path[:idx]
	}
	return path
}

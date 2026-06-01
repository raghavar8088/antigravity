package security

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// AuditEvent is an immutable security audit record.
type AuditEvent struct {
	EventID    string            `json:"event_id"`
	Timestamp  time.Time         `json:"timestamp"`
	UserID     string            `json:"user_id,omitempty"`
	Email      string            `json:"email,omitempty"`
	Role       string            `json:"role,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`
	ServiceName string           `json:"service_name,omitempty"`
	IP         string            `json:"ip"`
	Method     string            `json:"method"`
	Endpoint   string            `json:"endpoint"`
	Action     string            `json:"action"`
	Result     string            `json:"result"` // ALLOWED | DENIED | ATTEMPTED
	Reason     string            `json:"reason,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	DurationMs int64             `json:"duration_ms,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

const (
	AuditResultAllowed   = "ALLOWED"
	AuditResultDenied    = "DENIED"
	AuditResultAttempted = "ATTEMPTED"
	AuditResultError     = "ERROR"
)

// AuditStore is the interface for persisting audit events.
type AuditStore interface {
	AppendAudit(ctx context.Context, event AuditEvent) error
}

// AuditLogger emits immutable audit events to one or more sinks.
type AuditLogger struct {
	store  AuditStore
	buffer []AuditEvent
	mu     sync.Mutex
	maxBuf int
}

// NewAuditLogger creates an audit logger. If store is nil, events are buffered in memory.
func NewAuditLogger(store AuditStore) *AuditLogger {
	return &AuditLogger{store: store, maxBuf: 10_000}
}

// Log records an audit event. Non-blocking: writes to store async, falls back to in-memory ring.
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	if event.EventID == "" {
		event.EventID = generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Write to structured log immediately (always available).
	a.structuredLog(event)

	if a.store != nil {
		go func() {
			if err := a.store.AppendAudit(ctx, event); err != nil {
				log.Printf("[AUDIT] store error: %v", err)
			}
		}()
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.buffer = append(a.buffer, event)
	if len(a.buffer) > a.maxBuf {
		a.buffer = a.buffer[1:]
	}
}

// Recent returns the last n audit events from the in-memory buffer.
func (a *AuditLogger) Recent(n int) []AuditEvent {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n <= 0 || n > len(a.buffer) {
		n = len(a.buffer)
	}
	start := len(a.buffer) - n
	cp := make([]AuditEvent, n)
	copy(cp, a.buffer[start:])
	return cp
}

func (a *AuditLogger) structuredLog(ev AuditEvent) {
	meta := ""
	if len(ev.Meta) > 0 {
		if b, err := json.Marshal(ev.Meta); err == nil {
			meta = " " + string(b)
		}
	}
	log.Printf("[AUDIT] %s %s %s %s user=%q role=%q ip=%s %s%s",
		ev.Timestamp.Format(time.RFC3339),
		ev.Method, ev.Endpoint, ev.Result,
		ev.UserID, ev.Role, ev.IP, ev.Reason, meta)
}

// AuditFromRequest builds a baseline AuditEvent from an HTTP request and principal.
func AuditFromRequest(r *http.Request, p Principal, result, reason string, status int, start time.Time) AuditEvent {
	return AuditEvent{
		UserID:      p.UserID,
		Email:       p.Email,
		Role:        string(p.Role),
		SessionID:   p.SessionID,
		ServiceName: p.ServiceName,
		IP:          clientIP(r),
		Method:      r.Method,
		Endpoint:    r.URL.Path,
		Result:      result,
		Reason:      reason,
		StatusCode:  status,
		DurationMs:  time.Since(start).Milliseconds(),
	}
}

// generateID produces a simple hex ID from current nanoseconds.
func generateID() string {
	const hexChars = "0123456789abcdef"
	now := time.Now().UnixNano()
	b := make([]byte, 16)
	for i := range b {
		b[i] = hexChars[(now>>(i*4))&0xf]
	}
	return string(b)
}

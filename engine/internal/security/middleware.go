package security

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Gate is the central Zero Trust security middleware.
// Every request passes through authentication → rate limiting → RBAC → audit.
type Gate struct {
	policy     SecurityPolicy
	authn      *Authenticator
	limiter    *RateLimiter
	sessions   *SessionManager
	audit      *AuditLogger
	monitor    *SecurityMonitor
	projection *SecurityProjection
}

// NewGate assembles the complete security gate from a loaded policy.
func NewGate(policy SecurityPolicy, auditStore AuditStore) *Gate {
	monitor := NewSecurityMonitor()
	projection := NewSecurityProjection()
	monitor.handlers = append(monitor.handlers, func(inc SecurityIncident) {
		projection.Apply(SecurityEvent{
			Type:      EventSecurityIncidentOpened,
			IP:        inc.IP,
			Severity:  SeverityCritical,
			Details:   string(inc.Type),
			Timestamp: time.Now(),
		})
	})

	return &Gate{
		policy:     policy,
		authn:      NewAuthenticator(policy, monitor),
		limiter:    NewRateLimiter(DefaultPolicies),
		sessions:   NewSessionManager(policy.MaxSessionsPerUser),
		audit:      NewAuditLogger(auditStore),
		monitor:    monitor,
		projection: projection,
	}
}

// Wrap applies the security gate to a handler: OPTIONS passthrough, rate limit, authn, authz, audit.
func (g *Gate) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := clientIP(r)

		// OPTIONS preflight: respond immediately with correct CORS headers.
		g.setCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Rate limiting.
		policy := PolicyFor(r.URL.Path)
		if !g.limiter.Allow(policy, ip) {
			g.audit.Log(r.Context(), AuditEvent{
				IP: ip, Method: r.Method, Endpoint: r.URL.Path,
				Action: "rate_limit", Result: AuditResultDenied,
				DurationMs: time.Since(start).Milliseconds(),
			})
			g.projection.Apply(SecurityEvent{Type: EventRateLimitExceeded, IP: ip, Severity: SeverityWarning})
			RateLimitExceeded(w)
			return
		}

		// Public endpoints — skip auth.
		perm, requiresAuth := RequiredPermission(r.URL.Path, r.Method)
		if !requiresAuth {
			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)
			return
		}

		// Authentication.
		principal, authnErr := g.authn.Authenticate(r)
		if authnErr != nil && g.policy.EnforceAuth {
			g.audit.Log(r.Context(), AuditEvent{
				IP: ip, Method: r.Method, Endpoint: r.URL.Path,
				Action: "authenticate", Result: AuditResultDenied,
				Reason: authnErr.Error(), DurationMs: time.Since(start).Milliseconds(),
			})
			g.projection.Apply(SecurityEvent{Type: EventPermissionDenied, IP: ip, Severity: SeverityWarning, Endpoint: r.URL.Path})
			writeUnauthorized(w, authnErr.Error())
			return
		}
		if authnErr != nil {
			// Soft mode: log but allow.
			g.audit.Log(r.Context(), AuditEvent{
				IP: ip, Method: r.Method, Endpoint: r.URL.Path,
				Action: "authenticate_softmode", Result: AuditResultAttempted,
				Reason: authnErr.Error(),
			})
		}

		// Session revocation check.
		if principal.SessionID != "" && g.sessions.IsRevoked(principal.SessionID) {
			writeUnauthorized(w, "session revoked")
			return
		}

		// Admin endpoint: extra IP check + admin secret header.
		if IsAdminPath(r.URL.Path) {
			if g.policy.EnforceAdminIP && !IsAdminIPAllowed(r) {
				g.auditDenied(r, principal, perm, "admin IP not allowlisted", start)
				writeForbidden(w, "admin access not permitted from this IP")
				return
			}
			if !ValidateAdminSecret(r, g.policy.AdminSecret) {
				g.auditDenied(r, principal, perm, "missing or invalid admin secret", start)
				g.monitor.recordFailure(IncidentPrivEscalation, ip, r.URL.Path)
				writeForbidden(w, "admin secret required")
				return
			}
		}

		// Authorization (RBAC).
		decision := Authorize(principal, r.URL.Path, r.Method)
		if !decision.Allowed && g.policy.EnforceAuth {
			g.auditDenied(r, principal, perm, decision.Reason, start)
			g.monitor.recordFailure(IncidentPrivEscalation, ip, r.URL.Path)
			writeForbidden(w, decision.Reason)
			return
		}

		// Attach principal to request context.
		r = WithPrincipal(r, principal)
		g.sessions.Touch(principal.SessionID)

		// Execute handler with response recorder to capture status.
		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r)

		// Post-request audit.
		g.audit.Log(r.Context(), AuditFromRequest(r, principal, AuditResultAllowed, "", rw.status, start))
	})
}

// WrapFunc wraps an http.HandlerFunc for convenience.
func (g *Gate) WrapFunc(fn http.HandlerFunc) http.Handler {
	return g.Wrap(fn)
}

// Monitor returns the security monitor (for /api/security/* endpoints).
func (g *Gate) Monitor() *SecurityMonitor { return g.monitor }

// Projection returns the CQRS security projection.
func (g *Gate) Projection() *SecurityProjection { return g.projection }

// Sessions returns the session manager.
func (g *Gate) Sessions() *SessionManager { return g.sessions }

// AuditLog returns recent audit events.
func (g *Gate) AuditLog(n int) []AuditEvent { return g.audit.Recent(n) }

// setCORS writes secure CORS headers. Origin is validated against the allowlist.
func (g *Gate) setCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	allowed := origin
	if len(g.policy.AllowedOrigins) > 0 && !g.policy.IsOriginAllowed(origin) {
		allowed = g.policy.AllowedOrigins[0] // fall back to first allowed origin
	}
	w.Header().Set("Access-Control-Allow-Origin", allowed)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+ServiceAuthHeader+", "+ServiceNameHeader+", "+ServiceTimestampHeader+", "+AdminSecretHeader)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Content-Type", "application/json")
	// Security headers.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	if strings.HasPrefix(r.URL.Scheme, "https") {
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
	}
}

func (g *Gate) auditDenied(r *http.Request, p Principal, perm Permission, reason string, start time.Time) {
	g.audit.Log(r.Context(), AuditEvent{
		UserID: p.UserID, Role: string(p.Role), IP: clientIP(r),
		Method: r.Method, Endpoint: r.URL.Path,
		Action: string(perm), Result: AuditResultDenied,
		Reason: reason, DurationMs: time.Since(start).Milliseconds(),
	})
	g.projection.Apply(SecurityEvent{
		Type:      EventPermissionDenied,
		UserID:    p.UserID,
		IP:        clientIP(r),
		Endpoint:  r.URL.Path,
		Severity:  SeverityWarning,
		Result:    AuditResultDenied,
		Details:   reason,
		Timestamp: time.Now().UTC(),
	})
}

// SecurityContextKey is exported for handlers that need the principal.
func SecurityPrincipal(ctx context.Context) (Principal, bool) {
	return PrincipalFrom(ctx)
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

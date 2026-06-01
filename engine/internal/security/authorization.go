package security

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// AuthorizationResult describes an access control decision.
type AuthorizationResult struct {
	Allowed    bool
	Reason     string
	Principal  Principal
	Permission Permission
}

// Authorize checks whether principal p may access the endpoint described by
// path and method. It enforces RBAC permissions and records the decision.
func Authorize(p Principal, path, method string) AuthorizationResult {
	perm, requiresAuth := RequiredPermission(path, method)
	if !requiresAuth {
		return AuthorizationResult{Allowed: true, Reason: "public endpoint", Principal: p}
	}
	if p.UserID == "" && p.ServiceName == "" {
		return AuthorizationResult{
			Allowed:    false,
			Reason:     "unauthenticated",
			Permission: perm,
		}
	}
	if p.HasPermission(perm) {
		return AuthorizationResult{Allowed: true, Reason: "permission granted", Principal: p, Permission: perm}
	}
	return AuthorizationResult{
		Allowed:    false,
		Reason:     fmt.Sprintf("missing permission %s (role: %s)", perm, p.Role),
		Principal:  p,
		Permission: perm,
	}
}

// AdminIPAllowlist is the set of CIDRs permitted to call admin endpoints.
// Populated from ENGINE_ADMIN_CIDR env var (comma-separated). Empty = allow all authenticated.
var AdminIPAllowlist []string

// IsAdminIPAllowed returns true when the request IP is inside the admin CIDR allowlist
// OR the allowlist is empty (not configured).
func IsAdminIPAllowed(r *http.Request) bool {
	if len(AdminIPAllowlist) == 0 {
		return true
	}
	ip := clientIP(r)
	if ip == "" {
		return false
	}
	for _, cidr := range AdminIPAllowlist {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(net.ParseIP(ip)) {
			return true
		}
	}
	return false
}

// clientIP extracts the real client IP from a request, preferring X-Forwarded-For.
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
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// IsAdminPath returns true for paths under /api/admin/.
func IsAdminPath(path string) bool {
	return strings.HasPrefix(path, "/api/admin")
}

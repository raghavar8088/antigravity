package security

import (
	"os"
	"strings"
)

// SecurityPolicy is the active runtime configuration for the security layer.
type SecurityPolicy struct {
	// JWTSecret is the HS256 signing key shared with Next.js.
	JWTSecret string
	// AdminSecret is an additional header secret required for /api/admin/* endpoints.
	AdminSecret string
	// ServiceSecrets maps internal service names to their shared HMAC secrets.
	ServiceSecrets map[string]string
	// AllowedOrigins is the list of origins permitted in CORS responses.
	AllowedOrigins []string
	// AdminCIDRs restricts admin endpoint access to specific IP ranges.
	AdminCIDRs []string
	// Enforcement mode: when false, auth failures are logged but not blocked (soft rollout).
	EnforceAuth bool
	// EnforceAdminIP: when true, admin requests from non-allowlisted IPs are rejected.
	EnforceAdminIP bool
	// MaxSessionsPerUser: 0 = unlimited.
	MaxSessionsPerUser int
}

// LoadPolicy reads security policy from environment variables.
// This is called once at engine startup; secrets are NOT cached beyond this call.
func LoadPolicy() SecurityPolicy {
	p := SecurityPolicy{
		JWTSecret:   os.Getenv("AUTH_JWT_SECRET"),
		AdminSecret: os.Getenv("ENGINE_ADMIN_SECRET"),
		EnforceAuth: os.Getenv("SECURITY_ENFORCE_AUTH") != "false", // default: enforce
		EnforceAdminIP: os.Getenv("SECURITY_ENFORCE_ADMIN_IP") == "true",
		MaxSessionsPerUser: 5,
	}

	// CORS origins.
	if raw := os.Getenv("ALLOWED_ORIGINS"); raw != "" {
		p.AllowedOrigins = splitTrim(raw, ",")
	} else {
		p.AllowedOrigins = []string{"https://antigravity.vercel.app", "http://localhost:3000"}
	}

	// Admin CIDR allowlist.
	if raw := os.Getenv("ENGINE_ADMIN_CIDR"); raw != "" {
		p.AdminCIDRs = splitTrim(raw, ",")
		AdminIPAllowlist = p.AdminCIDRs
	}

	// Inter-service secrets: format "service1:secret1,service2:secret2"
	p.ServiceSecrets = parseServiceSecrets(os.Getenv("SERVICE_SECRETS"))

	// Always include Next.js frontend as a trusted service.
	if nextSecret := os.Getenv("INTERNAL_API_SECRET"); nextSecret != "" {
		if p.ServiceSecrets == nil {
			p.ServiceSecrets = make(map[string]string)
		}
		p.ServiceSecrets["nextjs"] = nextSecret
	}

	return p
}

func splitTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			result = append(result, v)
		}
	}
	return result
}

func parseServiceSecrets(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	m := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		kv := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(kv) == 2 && kv[0] != "" && kv[1] != "" {
			m[kv[0]] = kv[1]
		}
	}
	return m
}

// IsOriginAllowed returns true if the origin is in the allowlist.
func (p SecurityPolicy) IsOriginAllowed(origin string) bool {
	for _, allowed := range p.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

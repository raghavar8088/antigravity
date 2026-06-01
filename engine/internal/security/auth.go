package security

import (
	"net/http"
	"strings"
)

// Authenticator resolves the caller's identity from an incoming HTTP request.
// It tries (in order):
//  1. Service-to-service HMAC (X-Service-Auth header)
//  2. Bearer JWT in Authorization header
//  3. JWT in raig_session cookie (legacy web browser path)
type Authenticator struct {
	policy  SecurityPolicy
	svcAuth *ServiceAuthenticator
	monitor *SecurityMonitor
}

// NewAuthenticator creates an Authenticator from the loaded policy.
func NewAuthenticator(policy SecurityPolicy, monitor *SecurityMonitor) *Authenticator {
	svcAuth := NewServiceAuthenticator(policy.ServiceSecrets)
	return &Authenticator{policy: policy, svcAuth: svcAuth, monitor: monitor}
}

// Authenticate returns the Principal for the request, or an error if identity cannot be established.
// A nil error does NOT mean the principal has permission — call Authorize separately.
func (a *Authenticator) Authenticate(r *http.Request) (Principal, error) {
	// 1. Service auth (inter-service calls from Next.js or internal workers).
	if r.Header.Get(ServiceNameHeader) != "" {
		p, err := a.svcAuth.Authenticate(r)
		if err != nil {
			a.monitor.recordFailure(IncidentBadServiceAuth, clientIP(r), r.URL.Path)
			return Principal{}, err
		}
		p.IP = clientIP(r)
		return p, nil
	}

	// 2. Bearer JWT.
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		token, ok := ExtractBearerToken(authHeader)
		if !ok {
			a.monitor.recordFailure(IncidentBadToken, clientIP(r), r.URL.Path)
			return Principal{}, ErrJWTMalformed
		}
		return a.validateJWT(token, r)
	}

	// 3. Cookie (browser dashboard).
	if cookie, err := r.Cookie("raig_session"); err == nil && cookie.Value != "" {
		return a.validateJWT(cookie.Value, r)
	}

	return Principal{IP: clientIP(r)}, ErrJWTMalformed
}

func (a *Authenticator) validateJWT(token string, r *http.Request) (Principal, error) {
	claims, err := ParseJWT(token, a.policy.JWTSecret)
	if err != nil {
		a.monitor.recordFailure(IncidentBadToken, clientIP(r), r.URL.Path)
		return Principal{}, err
	}
	role := RoleFromString(claims.Role)
	if role == "" {
		role = RoleViewer
	}
	p := Principal{
		UserID:      claims.UserID,
		Email:       claims.Email,
		Role:        role,
		Permissions: PermissionsForRole(role),
		SessionID:   claims.SessionID,
		IP:          clientIP(r),
	}
	return p, nil
}

// writeUnauthorized sends a 401 JSON response.
func writeUnauthorized(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="trading-engine"`)
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"unauthorized","reason":"` + jsonEscape(reason) + `","code":401}`)) //nolint:errcheck
}

// writeForbidden sends a 403 JSON response.
func writeForbidden(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"forbidden","reason":"` + jsonEscape(reason) + `","code":403}`)) //nolint:errcheck
}

func jsonEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// ServiceAuthHeader is the header name for inter-service HMAC signatures.
	ServiceAuthHeader = "X-Service-Auth"
	// ServiceNameHeader carries the calling service's identity.
	ServiceNameHeader = "X-Service-Name"
	// ServiceTimestampHeader prevents replay attacks (must be within replayWindow).
	ServiceTimestampHeader = "X-Service-Timestamp"

	// replayWindow is the maximum age of a service request timestamp.
	replayWindow = 30 * time.Second
)

// ServiceAuthenticator validates service-to-service requests using HMAC-SHA256.
// Each internal service signs requests with a shared secret. The signature
// covers: serviceName + timestamp + method + path.
type ServiceAuthenticator struct {
	secrets map[string]string // serviceName → sharedSecret
}

// NewServiceAuthenticator creates an authenticator with a map of service secrets.
func NewServiceAuthenticator(secrets map[string]string) *ServiceAuthenticator {
	return &ServiceAuthenticator{secrets: secrets}
}

// Authenticate validates the service auth headers on an incoming request.
// Returns the service Principal on success.
func (sa *ServiceAuthenticator) Authenticate(r *http.Request) (Principal, error) {
	serviceName := r.Header.Get(ServiceNameHeader)
	if serviceName == "" {
		return Principal{}, fmt.Errorf("service_auth: missing %s header", ServiceNameHeader)
	}
	secret, ok := sa.secrets[serviceName]
	if !ok {
		return Principal{}, fmt.Errorf("service_auth: unknown service %q", serviceName)
	}

	tsStr := r.Header.Get(ServiceTimestampHeader)
	if tsStr == "" {
		return Principal{}, fmt.Errorf("service_auth: missing %s header", ServiceTimestampHeader)
	}
	tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return Principal{}, fmt.Errorf("service_auth: invalid timestamp")
	}
	ts := time.Unix(tsUnix, 0)
	if time.Since(ts) > replayWindow {
		return Principal{}, fmt.Errorf("service_auth: request timestamp expired (replay protection)")
	}

	sig := r.Header.Get(ServiceAuthHeader)
	if sig == "" {
		return Principal{}, fmt.Errorf("service_auth: missing %s header", ServiceAuthHeader)
	}

	expected := computeServiceSig(secret, serviceName, tsStr, r.Method, r.URL.Path)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return Principal{}, fmt.Errorf("service_auth: invalid signature")
	}

	return Principal{
		ServiceName: serviceName,
		IsService:   true,
		Role:        RoleService,
		Permissions: PermissionsForRole(RoleService),
	}, nil
}

// SignRequest adds service authentication headers to an outgoing request.
func SignRequest(r *http.Request, serviceName, secret string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := computeServiceSig(secret, serviceName, ts, r.Method, r.URL.Path)
	r.Header.Set(ServiceNameHeader, serviceName)
	r.Header.Set(ServiceTimestampHeader, ts)
	r.Header.Set(ServiceAuthHeader, sig)
}

// computeServiceSig builds the HMAC-SHA256 signature for a service request.
// Message format: serviceName|timestamp|METHOD|/path
func computeServiceSig(secret, serviceName, timestamp, method, path string) string {
	msg := strings.Join([]string{serviceName, timestamp, method, path}, "|")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// AdminSecretHeader is the extra header required for /api/admin/* endpoints.
// Defense-in-depth: even an admin JWT requires this header.
const AdminSecretHeader = "X-Admin-Secret"

// ValidateAdminSecret compares the admin secret header with the configured value.
func ValidateAdminSecret(r *http.Request, expectedSecret string) bool {
	if expectedSecret == "" {
		return true // not configured — skip check (dev mode only)
	}
	provided := r.Header.Get(AdminSecretHeader)
	return hmac.Equal([]byte(provided), []byte(expectedSecret))
}

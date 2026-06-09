package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── JWT tests ────────────────────────────────────────────────────────────────

func TestJWT_RoundTrip(t *testing.T) {
	secret := "supersecret-32-char-test-key!!!!!"
	claims := JWTClaims{
		UserID:    "u1",
		Email:     "trader@fund.com",
		Role:      string(RoleTrader),
		SessionID: "sess1",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := IssueJWT(claims, secret)
	if err != nil {
		t.Fatalf("IssueJWT error: %v", err)
	}
	parsed, err := ParseJWT(token, secret)
	if err != nil {
		t.Fatalf("ParseJWT error: %v", err)
	}
	if parsed.UserID != claims.UserID {
		t.Errorf("UserID: want %q got %q", claims.UserID, parsed.UserID)
	}
	if parsed.Email != claims.Email {
		t.Errorf("Email: want %q got %q", claims.Email, parsed.Email)
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	token, _ := IssueJWT(JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}, "secret-a")
	_, err := ParseJWT(token, "secret-b")
	if err != ErrJWTInvalid {
		t.Fatalf("expected ErrJWTInvalid, got %v", err)
	}
}

func TestJWT_Expired(t *testing.T) {
	token, _ := IssueJWT(JWTClaims{UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}, "secret")
	_, err := ParseJWT(token, "secret")
	if err != ErrJWTExpired {
		t.Fatalf("expected ErrJWTExpired, got %v", err)
	}
}

func TestJWT_Malformed(t *testing.T) {
	_, err := ParseJWT("not.a.jwt", "secret")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestJWT_Empty(t *testing.T) {
	_, err := ParseJWT("", "secret")
	if err != ErrJWTMalformed {
		t.Fatalf("expected ErrJWTMalformed, got %v", err)
	}
}

func TestExtractBearerToken(t *testing.T) {
	cases := []struct {
		header string
		want   string
		ok     bool
	}{
		{"Bearer abc123", "abc123", true},
		{"bearer abc123", "abc123", true},
		{"", "", false},
		{"Basic abc123", "", false},
		{"Bearer ", "", false},
	}
	for _, c := range cases {
		got, ok := ExtractBearerToken(c.header)
		if ok != c.ok || got != c.want {
			t.Errorf("ExtractBearerToken(%q): got (%q, %v) want (%q, %v)", c.header, got, ok, c.want, c.ok)
		}
	}
}

// ── RBAC tests ───────────────────────────────────────────────────────────────

func TestRBAC_SuperAdmin_HasAll(t *testing.T) {
	perms := PermissionsForRole(RoleSuperAdmin)
	if len(perms) == 0 {
		t.Fatal("SUPER_ADMIN should have permissions")
	}
	p := Principal{Role: RoleSuperAdmin, Permissions: perms}
	for _, perm := range []Permission{PermSystemShutdown, PermTradeExecute, PermRiskOverride, PermUserManage} {
		if !p.HasPermission(perm) {
			t.Errorf("SUPER_ADMIN missing permission %s", perm)
		}
	}
}

func TestRBAC_Viewer_CannotExecute(t *testing.T) {
	p := Principal{Role: RoleViewer, Permissions: PermissionsForRole(RoleViewer)}
	forbidden := []Permission{PermTradeExecute, PermSystemShutdown, PermRiskOverride, PermUserManage}
	for _, perm := range forbidden {
		if p.HasPermission(perm) {
			t.Errorf("VIEWER should NOT have %s", perm)
		}
	}
}

func TestRBAC_Trader_CanRequestExecution(t *testing.T) {
	p := Principal{Role: RoleTrader, Permissions: PermissionsForRole(RoleTrader)}
	if !p.HasPermission(PermTradeRequest) {
		t.Error("TRADER should have trade.request")
	}
	if p.HasPermission(PermTradeExecute) {
		t.Error("TRADER should NOT have trade.execute — engine service only")
	}
	if p.HasPermission(PermSystemShutdown) {
		t.Error("TRADER should NOT have system.shutdown")
	}
}

func TestRBAC_RiskManager_CanOverride(t *testing.T) {
	p := Principal{Role: RoleRiskManager, Permissions: PermissionsForRole(RoleRiskManager)}
	if !p.HasPermission(PermRiskOverride) {
		t.Error("RISK_MANAGER should have risk.override")
	}
	if p.HasPermission(PermSystemShutdown) {
		t.Error("RISK_MANAGER should NOT have system.shutdown")
	}
}

func TestRBAC_RequiredPermission(t *testing.T) {
	cases := []struct {
		path   string
		method string
		perm   Permission
		auth   bool
	}{
		{"/api/admin/kill", "POST", PermSystemShutdown, true},
		{"/api/trades", "GET", PermTradeView, true},
		{"/api/strategies", "GET", PermStrategyView, true},
		{"/health", "GET", "", false},
		{"/api/health", "GET", "", false},
	}
	for _, c := range cases {
		perm, auth := RequiredPermission(c.path, c.method)
		if auth != c.auth {
			t.Errorf("path=%s: auth want %v got %v", c.path, c.auth, auth)
		}
		if auth && perm != c.perm {
			t.Errorf("path=%s: perm want %s got %s", c.path, c.perm, perm)
		}
	}
}

func TestRoleFromString(t *testing.T) {
	cases := []struct {
		input string
		want  Role
	}{
		{"SUPER_ADMIN", RoleSuperAdmin},
		{"TRADER", RoleTrader},
		{"unknown", RoleViewer},
		{"", RoleViewer},
	}
	for _, c := range cases {
		got := RoleFromString(c.input)
		if got != c.want {
			t.Errorf("RoleFromString(%q): want %s got %s", c.input, c.want, got)
		}
	}
}

// ── Authorization tests ───────────────────────────────────────────────────────

func TestAuthorize_AdminAllowed(t *testing.T) {
	p := Principal{
		UserID:      "u1",
		Role:        RoleSuperAdmin,
		Permissions: PermissionsForRole(RoleSuperAdmin),
	}
	result := Authorize(p, "/api/admin/kill", "POST")
	if !result.Allowed {
		t.Fatalf("SUPER_ADMIN should be allowed on kill: %s", result.Reason)
	}
}

func TestAuthorize_ViewerDenied(t *testing.T) {
	p := Principal{
		UserID:      "u2",
		Role:        RoleViewer,
		Permissions: PermissionsForRole(RoleViewer),
	}
	result := Authorize(p, "/api/admin/kill", "POST")
	if result.Allowed {
		t.Fatal("VIEWER should be denied kill switch")
	}
}

func TestAuthorize_Unauthenticated(t *testing.T) {
	p := Principal{} // no UserID, no service
	result := Authorize(p, "/api/trades", "GET")
	if result.Allowed {
		t.Fatal("unauthenticated request should be denied")
	}
}

func TestAuthorize_PublicEndpoint(t *testing.T) {
	p := Principal{} // not authenticated
	result := Authorize(p, "/health", "GET")
	if !result.Allowed {
		t.Fatal("/health should be public")
	}
}

// ── Service auth tests ───────────────────────────────────────────────────────

func TestServiceAuth_ValidSignature(t *testing.T) {
	secret := "service-shared-secret-abc"
	svcAuth := NewServiceAuthenticator(map[string]string{"nextjs": secret})
	r := httptest.NewRequest("GET", "/api/trades", nil)
	SignRequest(r, "nextjs", secret)
	p, err := svcAuth.Authenticate(r)
	if err != nil {
		t.Fatalf("service auth error: %v", err)
	}
	if !p.IsService {
		t.Fatal("should be a service principal")
	}
	if p.ServiceName != "nextjs" {
		t.Errorf("service name: want nextjs got %s", p.ServiceName)
	}
}

func TestServiceAuth_WrongSecret(t *testing.T) {
	svcAuth := NewServiceAuthenticator(map[string]string{"nextjs": "correct-secret"})
	r := httptest.NewRequest("GET", "/api/trades", nil)
	SignRequest(r, "nextjs", "wrong-secret")
	_, err := svcAuth.Authenticate(r)
	if err == nil {
		t.Fatal("wrong secret should fail service auth")
	}
}

func TestServiceAuth_UnknownService(t *testing.T) {
	svcAuth := NewServiceAuthenticator(map[string]string{"nextjs": "secret"})
	r := httptest.NewRequest("GET", "/api/trades", nil)
	SignRequest(r, "unknown-service", "secret")
	_, err := svcAuth.Authenticate(r)
	if err == nil {
		t.Fatal("unknown service should fail auth")
	}
}

func TestServiceAuth_ReplayProtection(t *testing.T) {
	svcAuth := NewServiceAuthenticator(map[string]string{"nextjs": "secret"})
	r := httptest.NewRequest("GET", "/api/trades", nil)
	// Manually set an old timestamp.
	oldTs := time.Now().Add(-60 * time.Second).Unix()
	r.Header.Set(ServiceNameHeader, "nextjs")
	r.Header.Set(ServiceTimestampHeader, "123") // very old
	r.Header.Set(ServiceAuthHeader, computeServiceSig("secret", "nextjs", "123", "GET", "/api/trades"))
	_, err := svcAuth.Authenticate(r)
	if err == nil {
		t.Fatal("old timestamp should fail replay protection")
	}
	_ = oldTs
}

// ── Rate limiter tests ────────────────────────────────────────────────────────

func TestRateLimiter_AllowsBelowLimit(t *testing.T) {
	rl := NewRateLimiter(map[string]RateLimitPolicy{
		"test": {RequestsPerSecond: 10, BurstSize: 5},
	})
	for i := 0; i < 5; i++ {
		if !rl.Allow("test", "127.0.0.1") {
			t.Fatalf("request %d should be allowed (burst=5)", i)
		}
	}
}

func TestRateLimiter_BlocksAtLimit(t *testing.T) {
	rl := NewRateLimiter(map[string]RateLimitPolicy{
		"tight": {RequestsPerSecond: 1, BurstSize: 1},
	})
	rl.Allow("tight", "1.2.3.4") // consume the burst
	if rl.Allow("tight", "1.2.3.4") {
		t.Fatal("second request should be blocked when burst exhausted")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(map[string]RateLimitPolicy{
		"api": {RequestsPerSecond: 1, BurstSize: 1},
	})
	rl.Allow("api", "1.1.1.1") // exhaust IP1
	if !rl.Allow("api", "2.2.2.2") {
		t.Fatal("different IP should have independent bucket")
	}
}

func TestPolicyFor(t *testing.T) {
	cases := []struct {
		path   string
		policy string
	}{
		{"/api/admin/kill", "admin"},
		{"/api/delta-live/order", "trade"},
		{"/api/auth/signin", "auth"},
		{"/health", "health"},
		{"/api/strategies", "api"},
		{"/metrics", "global"},
	}
	for _, c := range cases {
		got := PolicyFor(c.path)
		if got != c.policy {
			t.Errorf("PolicyFor(%q): want %q got %q", c.path, c.policy, got)
		}
	}
}

// ── Session manager tests ────────────────────────────────────────────────────

func TestSessionManager_RevokeSingleSession(t *testing.T) {
	sm := NewSessionManager(10)
	sess := Session{
		SessionID: "s1", UserID: "u1",
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	sm.Register(sess)
	if sm.IsRevoked("s1") {
		t.Fatal("session should not be revoked initially")
	}
	sm.Revoke(context.Background(), "s1", "test")
	if !sm.IsRevoked("s1") {
		t.Fatal("session should be revoked after Revoke call")
	}
}

func TestSessionManager_RevokeAll(t *testing.T) {
	sm := NewSessionManager(10)
	for i := 0; i < 3; i++ {
		sm.Register(Session{
			SessionID: "s" + string(rune('0'+i)), UserID: "u1",
			CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
		})
	}
	n := sm.RevokeAll(context.Background(), "u1", "force logout")
	if n != 3 {
		t.Fatalf("expected 3 revocations, got %d", n)
	}
}

func TestSessionManager_MaxConcurrent(t *testing.T) {
	sm := NewSessionManager(2)
	sm.Register(Session{SessionID: "s1", UserID: "u1", CreatedAt: time.Now().Add(-2 * time.Minute), ExpiresAt: time.Now().Add(time.Hour)})
	sm.Register(Session{SessionID: "s2", UserID: "u1", CreatedAt: time.Now().Add(-time.Minute), ExpiresAt: time.Now().Add(time.Hour)})
	// Adding a 3rd session should revoke the oldest (s1).
	sm.Register(Session{SessionID: "s3", UserID: "u1", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
	if !sm.IsRevoked("s1") {
		t.Fatal("oldest session should be revoked when max concurrent exceeded")
	}
}

// ── Middleware gate tests ─────────────────────────────────────────────────────

func TestGate_PublicEndpointPassesThrough(t *testing.T) {
	policy := SecurityPolicy{EnforceAuth: true, AllowedOrigins: []string{"*"}}
	gate := NewGate(policy, nil)
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/health should pass through, got %d", rr.Code)
	}
}

func TestGate_ProtectedEndpoint_NoToken_Returns401(t *testing.T) {
	policy := SecurityPolicy{
		JWTSecret:   "test-secret",
		EnforceAuth: true,
		AllowedOrigins: []string{"*"},
	}
	gate := NewGate(policy, nil)
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/api/trades", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("/api/trades without token should return 401, got %d", rr.Code)
	}
}

func TestGate_ValidToken_AllowsAccess(t *testing.T) {
	secret := "jwt-test-secret-32chars!!!!!!!!"
	policy := SecurityPolicy{
		JWTSecret:   secret,
		EnforceAuth: true,
		AllowedOrigins: []string{"*"},
	}
	gate := NewGate(policy, nil)
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	token, _ := IssueJWT(JWTClaims{
		UserID:    "u1",
		Role:      string(RoleTrader),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)
	req := httptest.NewRequest("GET", "/api/trades", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid trader token should access /api/trades, got %d", rr.Code)
	}
}

func TestGate_TraderDenied_AdminEndpoint(t *testing.T) {
	secret := "jwt-test-secret-32chars!!!!!!!!"
	policy := SecurityPolicy{
		JWTSecret:   secret,
		EnforceAuth: true,
		AllowedOrigins: []string{"*"},
	}
	gate := NewGate(policy, nil)
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	token, _ := IssueJWT(JWTClaims{
		UserID:    "u2",
		Role:      string(RoleTrader),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}, secret)
	req := httptest.NewRequest("POST", "/api/admin/kill", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("TRADER accessing kill switch should get 403, got %d", rr.Code)
	}
}

func TestGate_OPTIONSPreflightPasses(t *testing.T) {
	policy := SecurityPolicy{EnforceAuth: true, AllowedOrigins: []string{"*"}}
	gate := NewGate(policy, nil)
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/trades", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS preflight should return 204, got %d", rr.Code)
	}
}

func TestGate_RateLimitEnforced(t *testing.T) {
	policy := SecurityPolicy{
		EnforceAuth: false,
		AllowedOrigins: []string{"*"},
	}
	gate := NewGate(policy, nil)
	// Override limiter with very tight limits.
	gate.limiter = NewRateLimiter(map[string]RateLimitPolicy{
		"health": {RequestsPerSecond: 1, BurstSize: 1},
	})
	handler := gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	first := httptest.NewRecorder()
	gate.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })).
		ServeHTTP(first, httptest.NewRequest("GET", "/health", nil))
	// Second request should be rate limited.
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest("GET", "/health", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be rate limited (429), got %d", second.Code)
	}
}

// ── Security monitor tests ────────────────────────────────────────────────────

func TestSecurityMonitor_RaisesIncident(t *testing.T) {
	var incident *SecurityIncident
	monitor := NewSecurityMonitor(func(inc SecurityIncident) {
		incident = &inc
	})
	for i := 0; i < 6; i++ {
		monitor.recordFailure(IncidentBruteForce, "1.2.3.4", "/api/auth/signin")
	}
	// Wait for goroutine.
	time.Sleep(10 * time.Millisecond)
	if incident == nil {
		t.Fatal("incident should have been raised after 6 brute force failures")
	}
	if incident.Type != IncidentBruteForce {
		t.Errorf("incident type: want BRUTE_FORCE got %s", incident.Type)
	}
}

func TestSecurityMonitor_ResolveIncident(t *testing.T) {
	monitor := NewSecurityMonitor()
	for i := 0; i < 6; i++ {
		monitor.recordFailure(IncidentBruteForce, "5.6.7.8", "/api/auth")
	}
	open := monitor.OpenIncidents()
	if len(open) == 0 {
		t.Fatal("should have open incidents")
	}
	monitor.ResolveIncident(open[0].ID)
	remaining := monitor.OpenIncidents()
	for _, inc := range remaining {
		if inc.ID == open[0].ID {
			t.Fatal("resolved incident should not appear in open incidents")
		}
	}
}

// ── Admin secret tests ────────────────────────────────────────────────────────

func TestValidateAdminSecret_Match(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/admin/kill", nil)
	r.Header.Set(AdminSecretHeader, "mysecret")
	if !ValidateAdminSecret(r, "mysecret") {
		t.Fatal("matching admin secret should pass")
	}
}

func TestValidateAdminSecret_Mismatch(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/admin/kill", nil)
	r.Header.Set(AdminSecretHeader, "wrongsecret")
	if ValidateAdminSecret(r, "mysecret") {
		t.Fatal("wrong admin secret should fail")
	}
}

func TestValidateAdminSecret_EmptyConfigured(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/admin/kill", nil)
	if !ValidateAdminSecret(r, "") {
		t.Fatal("empty configured secret should allow all (dev mode)")
	}
}

// ── Security projection tests ─────────────────────────────────────────────────

func TestSecurityProjection_CountsEvents(t *testing.T) {
	proj := NewSecurityProjection()
	for i := 0; i < 5; i++ {
		proj.Apply(SecurityEvent{Type: EventPermissionDenied, IP: "1.1.1.1"})
	}
	snap := proj.Snapshot()
	if snap.FailedLogins != 5 {
		t.Fatalf("expected 5 failed logins, got %d", snap.FailedLogins)
	}
}

func TestSecurityProjection_IncidentTracking(t *testing.T) {
	proj := NewSecurityProjection()
	proj.Apply(SecurityEvent{Type: EventSecurityIncidentOpened})
	proj.Apply(SecurityEvent{Type: EventSecurityIncidentOpened})
	proj.Apply(SecurityEvent{Type: EventSecurityIncidentClosed})
	snap := proj.Snapshot()
	if snap.ActiveIncidents != 1 {
		t.Fatalf("expected 1 active incident, got %d", snap.ActiveIncidents)
	}
}

# PENETRATION TEST READINESS CHECKLIST — Phase 15J
**Platform**: RAIG Institutional Trading Engine  
**Standard**: OWASP Top 10 (2021) + OWASP API Security Top 10 (2023)  
**Classification**: INTERNAL — Pre-engagement Preparation  

---

## SCOPE

| Component | In Scope | Notes |
|-----------|---------|-------|
| Go engine HTTP API (port 8080) | ✅ | All endpoints |
| Next.js client API routes (port 3000) | ✅ | All /api/* |
| WebSocket connections | ✅ | Market data feeds |
| MongoDB Atlas | ✅ | Auth bypass, injection |
| PostgreSQL (Neon) | ✅ | SQLi, privilege escalation |
| Redis | ✅ | Unauthenticated access |
| Exchange adapters (Binance, Delta, AngelOne) | ❌ | Out of scope — exchange-side |
| Vercel CDN | ❌ | Vercel-managed |

---

## OWASP TOP 10 (2021) VALIDATION

### A01 — Broken Access Control
- [ ] Access admin endpoints (`/api/admin/kill`, `/api/admin/reset`) without JWT → expect 401
- [ ] Access admin endpoints with TRADER role JWT → expect 403
- [ ] Attempt IDOR: swap account_key in paper trade requests → expect 403 or no data
- [ ] Force-browse to `/api/security/audit` without ADMIN role → expect 403
- [ ] Access `/api/probe/*` without auth → expect 401
- [ ] Test horizontal privilege escalation: user A accessing user B's positions
- [ ] Verify `/metrics` requires `metrics.view` permission

### A02 — Cryptographic Failures
- [ ] Confirm JWT uses HS256 (not HS1 or alg:none)
- [ ] Attempt JWT `alg:none` attack → expect 401
- [ ] Attempt JWT algorithm confusion (RS256 → HS256 with public key) → expect 401
- [ ] Verify all database connections use TLS (`sslmode=require`, `rediss://`, Atlas TLS)
- [ ] Confirm session cookies have `HttpOnly`, `Secure`, `SameSite=Strict`
- [ ] Verify no plaintext secrets in HTTP responses or error messages

### A03 — Injection
- [ ] SQL injection in all PostgreSQL query parameters → expect parameterized rejection
- [ ] MongoDB NoSQL injection (`{$gt: ""}` patterns) in query params → expect 400
- [ ] Command injection in any fields passed to exec/shell functions → expect 400
- [ ] LDAP injection (if LDAP used) → N/A
- [ ] Test all path parameters for path traversal (`../../etc/passwd`)
- [ ] Test JSON body fields for injection payloads

### A04 — Insecure Design
- [ ] Verify kill switch requires both JWT+role AND admin secret header
- [ ] Confirm no business logic bypass: execute trade without risk gate check
- [ ] Verify reconciliation cannot be triggered more than N times/min (rate limit)
- [ ] Test replay attack on JWT (reuse expired token) → expect 401

### A05 — Security Misconfiguration
- [ ] Verify no default credentials on any service (especially PostgreSQL `password123` ROTATED)
- [ ] Verify no debug endpoints exposed in production (`/debug/pprof`, `/api/test-*`)
- [ ] Verify CORS only allows `https://antigravity.vercel.app` (not `*`)
- [ ] Check HTTP headers: `X-Content-Type-Options`, `X-Frame-Options`, `CSP`, `HSTS`
- [ ] Verify health check doesn't expose sensitive data (strategy count OK, but not secrets)
- [ ] Confirm no stack traces in error responses

### A06 — Vulnerable and Outdated Components
- [ ] Run `go list -m -json all | nancy` for known-vulnerable Go deps
- [ ] Run `npm audit` on client dependencies
- [ ] Verify Go version ≥ 1.21 (for security fixes)
- [ ] Verify Node.js ≥ 20 LTS
- [ ] Check Alpine Linux base image CVEs

### A07 — Identification and Authentication Failures
- [ ] Brute force login: 10+ attempts on `/api/auth/signin` → expect rate limit (429 after 10)
- [ ] Attempt login with valid email + wrong password 50 times → expect lockout or continuing rate limit
- [ ] Test credential stuffing detection (multi-IP, single account)
- [ ] Verify JWT expiry is honored (test with 24h-old token) → expect 401
- [ ] Verify session revocation works: revoke session, attempt use → expect 401
- [ ] Test multi-session limit (max 5 sessions per user) → 6th session should invalidate oldest
- [ ] Verify password hashing (bcrypt/argon2, not MD5/SHA1)

### A08 — Software and Data Integrity Failures
- [ ] Verify no deserialization of untrusted data without validation
- [ ] Confirm supply chain: Go module checksums (`go.sum`) integrity
- [ ] Test JSON body size limits (expect 413 on oversized payloads)
- [ ] Verify `MaxHeaderBytes: 1<<20` enforced on engine

### A09 — Security Logging and Monitoring Failures
- [ ] Trigger a 401 on admin endpoint — verify audit log entry created
- [ ] Trigger 5 failed logins — verify brute force incident created
- [ ] Trigger kill switch — verify `KILL_SWITCH_TRIGGERED` event in audit log
- [ ] Verify `/api/security/audit` shows recent events (with ADMIN auth)
- [ ] Verify Prometheus `/metrics` includes `security_incidents_total`

### A10 — Server-Side Request Forgery (SSRF)
- [ ] Test any URL-taking parameters for SSRF (e.g., webhook URLs, proxy endpoints)
- [ ] Test `/api/angel-proxy` for SSRF — can it be redirected to internal services?
- [ ] Test any `fetch()` calls in Next.js routes that accept user-supplied URLs
- [ ] Verify engine's exchange HTTP clients have timeouts and allowlisted domains only

---

## OWASP API SECURITY TOP 10 (2023)

### API1 — Broken Object Level Authorization
- [ ] Access `/api/paper-trades?account_key=OTHER_USER_KEY` → expect empty/403
- [ ] Modify another user's positions via OMS API → expect 403
- [ ] Access options positions for account not owned by requester → expect 403

### API2 — Broken Authentication
- [ ] Use engine API without Bearer token → expect 401
- [ ] Use expired JWT → expect 401
- [ ] Use JWT with modified claims (tampered role field) → expect 401 (sig mismatch)
- [ ] Use service token (`X-Service-Auth`) from non-allowlisted service → expect 401

### API3 — Broken Object Property Level Authorization
- [ ] Attempt to set `role` field in user profile API → expect field ignored
- [ ] Attempt to submit trade with `price: 0` or negative price → expect risk gate rejection
- [ ] Attempt to set position size above MAX_POSITION_BTC → expect risk gate rejection

### API4 — Unrestricted Resource Consumption
- [ ] Send 1000 requests to `/api/strategies` in 1 second → expect 429 after rate limit
- [ ] Submit 100 simultaneous order requests → expect rate limit + OMS throttle
- [ ] Upload oversized JSON body (>1MB) → expect 413

### API5 — Broken Function Level Authorization
- [ ] Call `POST /api/admin/kill` as TRADER role → expect 403
- [ ] Call `POST /api/delta-live/enable` as VIEWER role → expect 403
- [ ] Call `GET /api/security/audit` as TRADER role → expect 403
- [ ] Call reconciliation run endpoint as ANALYST → expect 403

### API6 — Unrestricted Access to Sensitive Business Flows
- [ ] Attempt to trigger reconciliation 100 times → expect rate limit
- [ ] Attempt to submit orders during NSE off-hours → expect session gate rejection
- [ ] Attempt to bypass kill switch via alternate endpoint → verify no bypass path exists

### API7 — Server Side Request Forgery
- [ ] Test all proxy endpoints for SSRF (angel-proxy, engine proxy)

### API8 — Security Misconfiguration
- [ ] Verify CORS on engine: only `ALLOWED_ORIGINS` accepted
- [ ] Verify no `Access-Control-Allow-Origin: *` on any protected endpoint
- [ ] Verify engine error responses don't include stack traces or file paths

### API9 — Improper Inventory Management
- [ ] Enumerate all API routes and verify every one appears in RBAC endpoint table
- [ ] Verify no shadow APIs (undocumented endpoints not in policies.go)
- [ ] Check for deprecated versions of endpoints still accessible

### API10 — Unsafe Consumption of APIs
- [ ] Verify exchange API responses are validated before use (not blindly trusted)
- [ ] Test behavior when Delta Exchange returns malformed JSON → expect graceful error
- [ ] Test behavior when AngelOne returns 500 → expect circuit breaker / fallback

---

## AUTHENTICATION ATTACK VECTORS

### JWT-Specific Tests
```bash
# 1. Alg:none attack — strip signature
TOKEN=$(echo -n '{"alg":"none","typ":"JWT"}' | base64url).$(echo -n '{"userId":"admin","role":"SUPER_ADMIN","exp":9999999999}' | base64url).
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/admin/kill
# Expected: 401 Unauthorized

# 2. Expired token
EXPIRED_TOKEN="eyJhbGciOiJIUzI1NiJ9.eyJ1c2VySWQiOiJ4IiwiZXhwIjoxNjAwMDAwMDAwfQ.INVALID"
curl -H "Authorization: Bearer $EXPIRED_TOKEN" http://localhost:8080/api/trades
# Expected: 401 Unauthorized

# 3. Tampered role claim (valid token, modified payload)
# Expected: 401 — signature mismatch
```

### Rate Limit Tests
```bash
# Verify auth rate limit (10 req/min)
for i in $(seq 1 15); do
  curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:3000/api/auth/signin \
    -H "Content-Type: application/json" \
    -d '{"email":"test@test.com","password":"wrong"}'
  echo " attempt $i"
done
# Expected: 200/401 for first 10, then 429 for remaining
```

### Admin Secret Header Tests
```bash
# Kill switch without admin secret header
curl -X POST http://localhost:8080/api/admin/kill \
  -H "Authorization: Bearer $VALID_ADMIN_JWT"
# Expected: 403 — admin secret required

# Kill switch with wrong admin secret
curl -X POST http://localhost:8080/api/admin/kill \
  -H "Authorization: Bearer $VALID_ADMIN_JWT" \
  -H "X-Admin-Secret: wrongsecret"
# Expected: 403
```

---

## INFRASTRUCTURE TESTS

### Docker Container
```bash
# Verify non-root user
docker exec <container_id> id
# Expected: uid=1001(nextjs) gid=1001(nodejs)

# Verify no excessive capabilities
docker inspect <container_id> | jq '.[0].HostConfig.CapAdd'
# Expected: null

# Verify read-only filesystem (if configured)
docker exec <container_id> touch /test_write
# Expected: permission denied
```

### Network Exposure
```bash
# Verify only required ports exposed
nmap -sV localhost
# Expected: only :3000 (client), :8080 (engine), :50051 (gRPC if enabled)

# Verify database ports NOT publicly exposed
nmap -p 5432,27017,6379 <server_ip>
# Expected: all filtered/closed
```

---

## PRE-ENGAGEMENT REQUIREMENTS

Before the pen test engagement begins, verify:
- [ ] Test environment mirrors production configuration exactly
- [ ] Compromised credentials (C-01, C-02, C-03) revoked and replaced
- [ ] `SECURITY_ENFORCE_AUTH=true` in test environment
- [ ] `ENGINE_ADMIN_SECRET` set to 64-char random hex
- [ ] `ENGINE_ADMIN_CIDR` set to tester IP range
- [ ] Test accounts created with each role (SUPER_ADMIN, ADMIN, TRADER, VIEWER)
- [ ] Audit log collection confirmed working
- [ ] Test is against staging, never production

---

## PASS CRITERIA

| Category | Requirement |
|----------|------------|
| Authentication | Zero unauthenticated access to any protected endpoint |
| Authorization | No role can escalate to higher permissions |
| Rate Limiting | Login brute force blocked after 10 attempts |
| Injection | Zero SQL/NoSQL/command injection vectors |
| Secrets | No credentials in responses, logs, or error messages |
| Headers | CSP, HSTS, X-Frame-Options all present and correct |
| JWT | Alg:none, expiry bypass, signature tampering all rejected |
| Admin | Kill switch unreachable without JWT + Admin Secret |

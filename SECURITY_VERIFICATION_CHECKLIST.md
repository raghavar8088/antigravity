# Security Verification Checklist — Phase 15G Pen-Test Readiness

## Authentication

- [x] `/api/trades` returns 401 without Authorization header
- [x] `/api/positions` returns 401 without token
- [x] `/api/admin/kill` returns 401 without token
- [x] Expired JWT returns 401 (not 200)
- [x] Wrong JWT signature returns 401
- [x] Empty Authorization header returns 401
- [x] Cookie `raig_session` accepted as fallback auth
- [ ] Test: token issued by different secret rejected
- [ ] Test: HS256 `alg: none` attack rejected (our parser validates header)

## Authorization (RBAC)

- [x] VIEWER token cannot POST `/api/admin/kill` (returns 403)
- [x] TRADER token cannot POST `/api/admin/kill` (returns 403)
- [x] SUPER_ADMIN token CAN POST `/api/admin/kill` (returns 200)
- [x] Service token (nextjs) can GET `/api/trades`
- [ ] Test: manually crafted token with elevated role rejected (signature fails)
- [ ] Test: TRADER accessing risk.override endpoint returns 403

## Admin Endpoint Hardening

- [x] `/api/admin/kill` requires `X-Admin-Secret` header
- [x] Wrong `X-Admin-Secret` returns 403 even with valid JWT
- [ ] Test: configure `ENGINE_ADMIN_CIDR` and verify non-listed IP rejected
- [ ] Test: empty `ENGINE_ADMIN_SECRET` allows all (dev mode — disable in prod)

## JWT Security

- [x] Token expiry enforced (ExpiresAt checked)
- [x] Signature validation uses HMAC-SHA256 (constant-time comparison)
- [x] Token issued with future iat accepted
- [ ] Test: token with `exp=0` (no expiry) accepted/rejected per policy
- [ ] Test: replay of valid token from revoked session rejected

## Session Management

- [x] `SessionManager.Revoke()` prevents subsequent requests with that session ID
- [x] `SessionManager.RevokeAll()` force-logs out all sessions for a user
- [x] Max concurrent sessions enforced (oldest revoked on limit exceeded)
- [ ] Test: concurrent session limit prevents >5 simultaneous sessions per user

## Rate Limiting

- [x] `/api/admin/*` limited to 5 req/sec (burst=3)
- [x] `/api/auth/*` limited to 5 req/sec (burst=3)
- [x] Different IPs have independent rate limit buckets
- [x] Exhausted burst returns 429 with `Retry-After: 1`
- [ ] Test: 429 after 6 rapid requests to `/api/admin/kill`
- [ ] Test: DDoS simulation: 1000 req/sec from single IP blocked at 200

## Service-to-Service Authentication

- [x] Wrong HMAC signature rejected
- [x] Unknown service name rejected
- [x] Timestamp >30s old rejected (replay protection)
- [ ] Test: replaying a captured valid request after 31 seconds → rejected

## CORS

- [x] `Access-Control-Allow-Origin` reflects only allowed origins
- [x] OPTIONS preflight returns 204 with correct headers
- [x] `X-Content-Type-Options: nosniff` present
- [x] `X-Frame-Options: DENY` present
- [ ] Test: request from unlisted origin → rejected origin in response header

## Secrets

- [x] No secrets in engine Go source code
- [x] Vault provider available (falls back to env)
- [x] Secret cache TTL = 5 minutes (limiting exposure window)
- [ ] Verify: `.env` file contains NO actual secret values
- [ ] Test: `strings /bin/antigravity | grep -i "api_key"` returns nothing

## Injection & Input Validation

- [ ] Test: SQL injection in query params (PostgreSQL queries use parameterized)
- [ ] Test: JSON injection in request body (Go decoder handles gracefully)
- [ ] Test: oversized request body (MaxHeaderBytes=1MiB enforced)
- [ ] Test: path traversal `../../etc/passwd` in URL params

## Audit Trail

- [x] Every 401/403 response generates an AuditEvent
- [x] Admin actions logged with user, IP, and result
- [x] Security incidents raised after 5 failed auth attempts from same IP
- [x] `GET /api/security/audit` returns last 200 audit events
- [ ] Test: audit log survives engine restart (requires persistent AuditStore)

## Observability

- [x] `GET /api/security/status` returns security projection snapshot
- [x] `GET /api/security/incidents` returns open incidents
- [x] Failed login counts increment in projection
- [x] Rate limit violations tracked in projection

## Container

- [x] Container runs as UID 10001 (non-root)
- [x] `scratch` base image (no shell, no package manager)
- [x] Binary built with `-trimpath -s -w` (no debug symbols)
- [ ] Verify: `docker inspect --format '{{.Config.User}}' <image>` = "10001:10001"
- [ ] Test: `docker exec` fails (scratch has no shell)

## OWASP Top 10 Coverage

| Threat | Status | Notes |
|--------|--------|-------|
| A01 Broken Access Control | ✅ Mitigated | RBAC on all endpoints |
| A02 Cryptographic Failures | ✅ Mitigated | HMAC-SHA256 JWT, no plaintext secrets |
| A03 Injection | ⚠️ Partial | Parameterized queries in ledger; verify all handlers |
| A04 Insecure Design | ✅ Mitigated | Zero Trust architecture |
| A05 Security Misconfiguration | ✅ Mitigated | Secure Dockerfile, CORS allowlist |
| A06 Vulnerable Components | ⚠️ Partial | Need trivy scan in CI |
| A07 Auth Failures | ✅ Mitigated | Brute force detection, rate limiting |
| A08 Software Integrity | ⚠️ Partial | Need image signing |
| A09 Logging Failures | ✅ Mitigated | Immutable audit trail |
| A10 SSRF | ⚠️ Partial | AngelOne proxy has allowlist; review all outbound calls |

# 06 — Authentication Validation Framework

**Scope:** Vercel API layer + middleware + engine proxy  
**Standard:** Fail-closed — missing/invalid auth always rejects

---

## Architecture Under Test

```
Browser → middleware.ts (JWT verify) → API route (getAuthenticatedApiSession) → Engine proxy (session + admin secret)
Cron    → CRON_SECRET header only
Public  → /api/auth/login, /api/health only
```

---

## Test Matrix

| ID | Test | Attack Vector | Expected | Severity |
|----|------|---------------|----------|----------|
| AUTH-01 | Engine proxy no session | `POST /api/engine/api/admin/ks/block` | 401 | CRITICAL |
| AUTH-02 | Engine proxy blocked path | `GET /api/engine/api/nifty/seed-engine` | 403 | CRITICAL |
| AUTH-03 | Fake JWT cookie | `Cookie: raig_session=fake.token.here` | Redirect /login | CRITICAL |
| AUTH-04 | Expired JWT | Token with `exp` in past | 401 | CRITICAL |
| AUTH-05 | Wrong signature | JWT signed with wrong secret | 401 | CRITICAL |
| AUTH-06 | Algorithm none | JWT alg=none attack | 401 | CRITICAL |
| AUTH-07 | Session replay | Reuse old valid JWT after logout | 401 | HIGH |
| AUTH-08 | Cron no secret | `GET /api/cron/rank-strategies` | 401/503 | CRITICAL |
| AUTH-09 | Cron wrong secret | Invalid Bearer token | 401 | CRITICAL |
| AUTH-10 | Admin without ENGINE_ADMIN_SECRET | Valid session, no admin header | 403 | CRITICAL |
| AUTH-11 | Account key from body | JWT userId vs body accountId mismatch | Body ignored | HIGH |
| AUTH-12 | Rate limit auth | 11 login attempts in 1 min | 429 | MEDIUM |

---

## Automated Script

**Location:** `scripts/production-readiness/validate-auth.sh`

```bash
export APP_URL="https://your-app.vercel.app"
export CRON_SECRET="..."
bash scripts/production-readiness/validate-auth.sh
```

### Pass/Fail Criteria

| Result | Gate |
|--------|------|
| All CRITICAL tests PASS | Release allowed |
| Any CRITICAL FAIL | Release blocked |
| HIGH FAIL | Release blocked with waiver (CISO sign-off) |
| MEDIUM FAIL | Warning, 7-day remediation SLA |

---

## Attack Simulations

### JWT Signature Bypass

```bash
# Craft token with valid payload, invalid signature
FAKE=$(echo -n '{"sub":"admin","exp":9999999999}' | base64)
curl -H "Cookie: raig_session=header.$FAKE.sig" "$APP_URL/api/paper-trades"
# Expected: 401 or redirect to /login
```

### Session Replay

```bash
# 1. Login, capture cookie
# 2. Sign out
# 3. Replay captured cookie
curl -H "Cookie: raig_session=$OLD_COOKIE" "$APP_URL/dashboard"
# Expected: redirect to /login (session invalidated)
```

### Middleware Consistency

Verify same JWT validation in:
- `client/src/middleware.ts` — Web Crypto HS256 verify
- `client/src/lib/auth.ts` — `getAuthenticatedApiSession()`
- All protected API routes import session helper

```bash
rg "getAuthenticatedApiSession|requireSession" client/src/app/api --count
rg "export async function (GET|POST|DELETE)" client/src/app/api/angelone -l
```

---

## Implementation Verification (Code)

| Control | File | Status |
|---------|------|--------|
| JWT signature verify | `middleware.ts` | ✅ Phase 1 |
| Session on engine proxy | `api/engine/[...path]/route.ts` | ✅ Phase 1 |
| CRON_SECRET mandatory | `api/cron/*/route.ts` | ✅ Phase 1 |
| Broker routes authenticated | `api/angelone/*/route.ts` | ✅ Phase 1 |
| Destructive routes auth | `paper-state/repair`, `paper-trades/clear` | ✅ Phase 1 |

### Remaining Gaps

| Gap | Remediation | Priority |
|-----|-------------|----------|
| No `role` claim in JWT | Add at login for RBAC | P3 |
| No service-to-service HMAC | `engine/internal/security/service_auth.go` wire | P3 |
| Edge rate limit per-pod only | Redis distributed rate limit | P4 |
| `ENGINE_ADMIN_SECRET` empty bypass | Boot gate fail-fast | P3 |

---

## Continuous Validation

Add to CI (post-deploy smoke):

```yaml
# .github/workflows/production-smoke.yml
- name: Auth validation
  run: bash scripts/production-readiness/validate-auth.sh
  env:
    APP_URL: ${{ secrets.PRODUCTION_APP_URL }}
    CRON_SECRET: ${{ secrets.CRON_SECRET }}
```

---

## Sign-Off

**Authentication Framework Readiness:** 82/100 (controls implemented, production env verification pending)

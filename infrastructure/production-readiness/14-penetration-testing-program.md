# 14 — Penetration Testing Program

**Scope:** Vercel API surface + Engine HTTP + AWS infrastructure  
**Standard:** OWASP API Security Top 10 + trading-specific threats

---

## Threat Model

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Attacker   │────►│ Vercel/WAF   │────►│ ECS Engine  │
│  (external) │     │ Next.js API  │     │ Go binary   │
└─────────────┘     └──────────────┘     └─────────────┘
       │                    │                    │
       ▼                    ▼                    ▼
  Auth bypass          SSRF to engine      Admin endpoint
  Session replay       Broker abuse        Credential leak
  CSRF                 Cron abuse          Kill switch abuse
```

### Assets at Risk

1. Broker API credentials (Angel One, Delta, Binance)
2. Trading capital (paper + live)
3. Kill switch state
4. Event ledger integrity
5. User session tokens

---

## Test Matrix

| ID | Category | Test | Severity if FAIL |
|----|----------|------|------------------|
| PEN-01 | Auth bypass | Access `/api/engine/*` without session | CRITICAL |
| PEN-02 | Auth bypass | Forge JWT with weak secret | CRITICAL |
| PEN-03 | Authorization | Access admin endpoints as viewer | CRITICAL |
| PEN-04 | Session replay | Reuse token after logout | HIGH |
| PEN-05 | CSRF | State-changing request without CSRF token | HIGH |
| PEN-06 | SSRF | Engine proxy to internal metadata (169.254.169.254) | CRITICAL |
| PEN-07 | API abuse | Cron endpoint brute force | HIGH |
| PEN-08 | Credential leak | Secrets in API responses/headers | CRITICAL |
| PEN-09 | Privilege escalation | Modify JWT role claim | CRITICAL |
| PEN-10 | Broker route abuse | Place live order via anonymous API | CRITICAL |
| PEN-11 | Admin endpoint abuse | Trigger kill switch without secret | CRITICAL |
| PEN-12 | Path traversal | `/api/engine/../../../admin` | HIGH |
| PEN-13 | Rate limit bypass | Distributed IP rotation | MEDIUM |
| PEN-14 | CORS abuse | Cross-origin credentialed request | HIGH |
| PEN-15 | WAF bypass | SQLi/XSS in trading payloads | HIGH |

---

## Findings (Pre-Remediation Baseline)

| ID | Finding | Severity | Status | Remediation |
|----|---------|----------|--------|-------------|
| F-01 | Engine proxy unauthenticated | CRITICAL | ✅ FIXED Phase 1 | Session required |
| F-02 | Angel One routes anonymous | CRITICAL | ✅ FIXED Phase 1 | Session required |
| F-03 | CRON_SECRET optional | HIGH | ✅ FIXED Phase 1 | Fail-closed |
| F-04 | Middleware cookie-only check | HIGH | ✅ FIXED Phase 1 | JWT signature verify |
| F-05 | Secrets in .env git history | CRITICAL | ❌ OPEN | git filter-repo + rotate |
| F-06 | ENGINE_ADMIN_SECRET empty bypass | CRITICAL | ❌ OPEN | Boot gate + verify prod |
| F-07 | No TLS on engine (Lightsail) | HIGH | ⚠️ IN PROGRESS | ALB HTTPS (Terraform) |
| F-08 | No service-to-service HMAC | MEDIUM | ❌ OPEN | Phase 3 wire service_auth |
| F-09 | Angel One creds on Vercel | HIGH | ❌ OPEN | Migrate to engine |
| F-10 | Leader election not wired | CRITICAL | ❌ OPEN | Wire before multi-task ECS |

---

## Remediation Plan

### Immediate (Pre-Go-Live)

| Priority | Action | Owner | ETA |
|----------|--------|-------|-----|
| P0 | Purge git history + rotate all keys | Security | Week 0 |
| P0 | Verify ENGINE_ADMIN_SECRET in prod | DevOps | Week 0 |
| P0 | Wire leader election | Engineering | Week 1 |
| P0 | `terraform apply` + TLS on ALB | SRE | Week 1 |

### Short-Term (30 days)

| Priority | Action | Owner |
|----------|--------|-------|
| P1 | Migrate Angel One to engine | Engineering |
| P1 | Service-to-service HMAC | Engineering |
| P1 | External pen-test (third party) | Security |
| P2 | RBAC role claim in JWT | Engineering |

### Ongoing

- Quarterly automated pen-test (OWASP ZAP + custom scripts)
- Annual third-party pen-test for institutional investors
- Bug bounty consideration at $10M+ AUM

---

## Automated Pen-Test Scripts

```bash
# Run all security validations
bash scripts/production-readiness/validate-auth.sh
bash scripts/production-readiness/validate-broker-security.sh

# SSRF test
curl "$APP_URL/api/engine/http://169.254.169.254/latest/meta-data/"
# Expected: 403 BLOCKED

# Path traversal
curl "$APP_URL/api/engine/api/admin/../health"
# Expected: 403 or 404, not admin access
```

---

## Severity Ratings

| Level | Definition | SLA |
|-------|------------|-----|
| CRITICAL | Unauthenticated trading/credential access | Fix before go-live |
| HIGH | Auth bypass with valid low-priv session | 7 days |
| MEDIUM | Rate limit / info disclosure | 30 days |
| LOW | Best practice deviation | 90 days |

---

## Sign-Off Criteria

- Zero CRITICAL open findings
- All HIGH findings have documented waivers or fixes
- Third-party pen-test report (institutional requirement)

**Pen-Test Program Readiness:** 75/100 (Phase 1 fixes done, 4 CRITICAL open)

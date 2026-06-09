# 16 — Production Readiness Checklist

**Gate:** All CRITICAL items must be ✅ before live capital deployment.

---

## Security

| # | Item | Status | Owner |
|---|------|--------|-------|
| S-01 | Engine proxy authenticated + allowlisted | ✅ | Eng |
| S-02 | Angel One routes authenticated | ✅ | Eng |
| S-03 | CRON_SECRET mandatory (fail-closed) | ✅ | DevOps |
| S-04 | Middleware JWT signature validation | ✅ | Eng |
| S-05 | Destructive routes authenticated | ✅ | Eng |
| S-06 | ENGINE_ADMIN_SECRET set in production | ❌ VERIFY | DevOps |
| S-07 | SECURITY_ENFORCE_AUTH=true in engine | ❌ VERIFY | DevOps |
| S-08 | Git history purged of secrets | ❌ | Security |
| S-09 | All API keys rotated post-purge | ❌ | Security |
| S-10 | WAF deployed on ALB | ❌ TF apply | SRE |
| S-11 | TLS 1.3 on all engine traffic | ❌ TF apply | SRE |
| S-12 | Secrets in Secrets Manager only | ❌ | SRE |
| S-13 | Service-to-service HMAC (Vercel→Engine) | ❌ | Eng |
| S-14 | Pen-test: zero CRITICAL open | ❌ 4 open | Security |

---

## Execution & Capital Protection

| # | Item | Status | Owner |
|---|------|--------|-------|
| E-01 | Kill switch durable (PostgresStore) | ✅ code | Eng |
| E-02 | DATABASE_URL set in production | ❌ VERIFY | DevOps |
| E-03 | Reconciliation running (10s) | ✅ wired | Eng |
| E-04 | OMS boot replay wired | ❌ | Eng |
| E-05 | Leader election wired (ECS HA) | ❌ | Eng |
| E-06 | Risk V1 daily loss aligned to 3% | ⚠️ | Eng |
| E-07 | No unauthenticated execution path | ✅ | Eng |
| E-08 | Idempotency on all order events | ✅ | Eng |
| E-09 | Angel One execution on engine | ❌ | Eng |

---

## Infrastructure

| # | Item | Status | Owner |
|---|------|--------|-------|
| I-01 | Terraform applied ap-south-1 | ❌ | SRE |
| I-02 | ECS 2+ tasks healthy | ❌ | SRE |
| I-03 | Aurora operational + PITR tested | ❌ | SRE |
| I-04 | Redis operational + failover tested | ❌ | SRE |
| I-05 | Secrets Manager populated | ❌ | SRE |
| I-06 | SNS alarms confirmed | ❌ | SRE |
| I-07 | ECS deploy pipeline tested | ❌ | SRE |
| I-08 | CloudWatch dashboard active | ❌ | SRE |

---

## Validation & Resilience

| # | Item | Status | Owner |
|---|------|--------|-------|
| V-01 | Auth validation script PASS | ❌ | SRE |
| V-02 | Broker security script PASS | ❌ | SRE |
| V-03 | Kill switch persistence test PASS | ❌ | SRE |
| V-04 | Reconciliation drift test PASS | ❌ | SRE |
| V-05 | Event replay 5 scenarios PASS | ❌ | Eng |
| V-06 | HA test plan executed | ❌ | SRE |
| V-07 | DR drill completed | ❌ | SRE |
| V-08 | Chaos L2 experiments PASS | ❌ | SRE |
| V-09 | go-live-gate.sh PASS | ❌ | SRE |

---

## Automated Gate

```bash
bash scripts/production-readiness/go-live-gate.sh
# Exit 0 = READY
# Exit 1 = BLOCKED (lists failures)
```

---

## Sign-Off

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Principal Architect | | | |
| SRE Lead | | | |
| Security Lead | | | |
| Trading Ops | | | |

**Checklist Completion:** 12/38 items ✅ (32%)

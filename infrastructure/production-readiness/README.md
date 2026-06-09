# Phase 3–5 Enterprise Hardening — Production Readiness Package

**Version:** 1.0  
**Date:** 2026-06-09  
**Baseline:** Phase 1+2 complete (Security 68, Reliability 58, Capital Protection 72, Production Readiness 59)  
**Target:** Production Readiness > 90, Security > 92, Reliability > 92, Capital Protection > 95

---

## Execution Order

```
Week 0 (Pre-deploy)     → 05 Secrets Migration, git filter-repo, secret rotation
Week 1 (Infrastructure) → terraform apply, populate Secrets Manager, ECR push
Week 2 (Validation)     → 06–10 frameworks, go-live-gate.sh, shadow mode ECS
Week 3 (HA/DR)          → 11–12 HA tests + DR drill (ap-south-1)
Week 4 (Resilience)     → 13 Chaos + 14 Pen-test + 09 Event replay
Week 5 (Go-live)        → 17 Gate PASS → cut INTERNAL_API_URL to ALB
```

---

## Deliverables Index

| # | Document | Purpose |
|---|----------|---------|
| 01 | [Infrastructure Validation Report](./01-infrastructure-validation-report.md) | Terraform verification, deployment prerequisites |
| 02 | [ECS Readiness Report](./02-ecs-readiness-report.md) | Fargate Multi-AZ, scaling, health, graceful shutdown |
| 03 | [Aurora Readiness Report](./03-aurora-readiness-report.md) | Event ledger, PITR, failover, connection pooling |
| 04 | [Redis Readiness Report](./04-redis-readiness-report.md) | TLS, Multi-AZ, snapshots, failover |
| 05 | [Secrets Manager Migration Plan](./05-secrets-manager-migration-plan.md) | Secret migration, rotation, expiry validation |
| 06 | [Authentication Validation Framework](./06-authentication-validation-framework.md) | JWT, middleware, attack simulations |
| 07 | [Broker Security Validation Framework](./07-broker-security-validation-framework.md) | Angel One, Delta, anonymous access tests |
| 08 | [Reconciliation Validation Framework](./08-reconciliation-validation-framework.md) | Drift detection, trading pause, alerts |
| 09 | [Event Replay Validation Framework](./09-event-replay-validation-framework.md) | 5 failure scenarios + idempotency |
| 10 | [Kill Switch Validation Framework](./10-kill-switch-validation-framework.md) | Persistence, flatten, deployment survival |
| 11 | [High Availability Test Plan](./11-high-availability-test-plan.md) | ECS, Aurora, Redis failure injection |
| 12 | [Disaster Recovery Runbook](./12-disaster-recovery-runbook.md) | ap-south-1 ↔ ap-southeast-1, RPO/RTO |
| 13 | [Chaos Engineering Program](./13-chaos-engineering-program.md) | Quarterly failure injection schedule |
| 14 | [Penetration Testing Program](./14-penetration-testing-program.md) | Threat model, test matrix, remediation |
| 15 | [Observability Validation Report](./15-observability-validation-report.md) | CloudWatch, Prometheus, Grafana, alerts |
| 16 | [Production Readiness Checklist](./16-production-readiness-checklist.md) | Human + automated gate checklist |
| 17 | [Automated Go-Live Gate Design](./17-automated-go-live-gate-design.md) | CI/CD release blocker specification |
| 18 | [Cost Analysis](./18-cost-analysis.md) | Capacity planning and monthly projections |
| 19 | [Residual Risk Register](./19-residual-risk-register.md) | Accepted risks post-hardening |
| 20 | [Final Production Readiness Scorecard](./20-final-production-readiness-scorecard.md) | Dimension scores and sign-off criteria |

---

## Automation

| Script | Location | Usage |
|--------|----------|-------|
| Go-live gate (master) | `scripts/production-readiness/go-live-gate.sh` | Run before every production release |
| Auth validation | `scripts/production-readiness/validate-auth.sh` | Post-deploy security smoke |
| Broker security | `scripts/production-readiness/validate-broker-security.sh` | Anonymous order/cancel/funds tests |
| Kill switch | `scripts/production-readiness/validate-kill-switch.sh` | Persistence + restart tests |
| Reconciliation | `scripts/production-readiness/validate-reconciliation.sh` | Log + drift injection |
| Event replay | `scripts/production-readiness/validate-event-replay.sh` | Go integration tests |
| Infrastructure | `scripts/production-readiness/validate-infrastructure.sh` | AWS resource health checks |
| Engine boot gate | `engine/internal/validation/production/gate.go` | Fail-fast at engine startup |

---

## Hard Release Blockers

Release **MUST FAIL** if any of:

- `ENGINE_ADMIN_SECRET` missing in production
- `DATABASE_URL` missing in production (engine)
- Secrets stored in plaintext on disk or in git
- Reconciliation service not running
- Event replay tests fail
- Kill switch persistence test fails
- Broker route authentication fails
- Aurora backup/PITR drill fails
- ECS health checks fail with < 2 healthy tasks

Run: `bash scripts/production-readiness/go-live-gate.sh`

---

## Infrastructure Apply

```bash
cd infrastructure/terraform
terraform init
terraform plan -var-file=production.tfvars -out=plan.out
# Review plan.out — expect ~45 resources
terraform apply plan.out
```

Populate secrets after apply — see [05-secrets-manager-migration-plan.md](./05-secrets-manager-migration-plan.md).

# 01 — Infrastructure Validation Report

**Status:** AUTHORED — NOT DEPLOYED  
**Region:** ap-south-1 (primary)  
**IaC:** `infrastructure/terraform/` (8 modules, ~45 resources)

---

## Executive Summary

Terraform infrastructure for institutional production is **complete in code** but **not applied**. All Phase 3 blockers are deployment-execution blockers, not design gaps. This report validates the authored configuration against institutional requirements and identifies pre-apply fixes.

| Component | Code Status | Deployed | Validation |
|-----------|-------------|----------|------------|
| VPC (Multi-AZ) | ✅ | ❌ | 2 public + 2 private + 2 DB subnets |
| ECS Fargate | ✅ | ❌ | 2 tasks, circuit breaker, auto-scale |
| Aurora PostgreSQL | ✅ | ❌ | Serverless v2, writer+reader, PITR |
| ElastiCache Redis | ✅ | ❌ | TLS, Multi-AZ, 2 nodes |
| Secrets Manager | ✅ | ❌ | KMS CMK, 30-day rotation |
| ALB + WAF | ✅ | ❌ | TLS 1.3, rate limit, managed rules |
| CloudWatch | ✅ | ❌ | 10+ alarms, dashboard, SNS |
| AWS Backup | ✅ | ❌ | Daily + weekly Aurora snapshots |
| CI/CD (ECS) | ✅ | ❌ | OIDC, Trivy scan, stability wait |

---

## Pre-Apply Blockers (Must Fix Before `terraform apply`)

### B-01: Secret Rotation Lambda ZIP Missing

`security.tf` references `lambda/secret_rotation.zip` which did not exist. **Fixed:** placeholder Lambda at `infrastructure/terraform/lambda/secret_rotation/`.

### B-02: ACM Certificate ARN Required

`production.tfvars` must include valid `alb_certificate_arn` for ap-south-1. Request via ACM or import existing cert.

### B-03: Terraform State Backend Bootstrap

One-time setup required:

```bash
aws s3 mb s3://antigravity-tfstate-ap-south-1 --region ap-south-1
aws s3api put-bucket-versioning --bucket antigravity-tfstate-ap-south-1 \
  --versioning-configuration Status=Enabled
aws dynamodb create-table --table-name antigravity-tfstate-lock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST --region ap-south-1
```

### B-04: Git History Secret Purge

`.env` and credential files may exist in git history. Run `git filter-repo` before populating Secrets Manager with production values. Rotate ALL keys after purge.

### B-05: Leader Election Not Wired in Engine

`engine/internal/ha/cluster.go` exists but is **not called** from `main.go`. **MUST wire before ECS `desired_count=2`** or dual-writer risk exists. See [02-ecs-readiness-report.md](./02-ecs-readiness-report.md) §Leader Election.

### B-06: OMS Boot Replay Not Wired

`omsv3.ReplayAll()` exists but is not invoked at engine boot. Wire before production ECS cutover. See [09-event-replay-validation-framework.md](./09-event-replay-validation-framework.md).

---

## Terraform Module Validation

### VPC (`vpc.tf`)

| Requirement | Implementation | Pass |
|-------------|----------------|------|
| Multi-AZ | 2 AZs via `data.aws_availability_zones` | ✅ |
| Private subnets for compute | ECS, Redis in private subnets | ✅ |
| Isolated DB subnets | Aurora in dedicated DB subnets | ✅ |
| NAT Gateway | Per-AZ NAT for outbound broker APIs | ✅ |
| Security groups | ALB→ECS→Aurora/Redis least-privilege | ✅ |
| VPC Flow Logs | CloudWatch capture | ✅ |
| NACLs | DB subnet deny non-private | ✅ |

### ECS (`ecs.tf`)

| Requirement | Implementation | Pass |
|-------------|----------------|------|
| Minimum 2 tasks | `engine_desired_count=2` default | ✅ |
| Multi-AZ placement | Private subnets across AZs | ✅ |
| Auto-scaling | CPU target 60%, min=desired, max=6 | ✅ |
| Rolling deploy | min_healthy=50%, max=200% | ✅ |
| Circuit breaker | `enable=true, rollback=true` | ✅ |
| Health checks | Container + ALB `/health` | ✅ |
| Graceful shutdown | `deregistration_delay=30s` | ✅ |
| Secrets injection | 14 keys from Secrets Manager | ✅ |
| Read-only root FS | `readonlyRootFilesystem=true` | ✅ |
| Capabilities dropped | `drop=["ALL"]` | ✅ |
| X-Ray sidecar | Non-essential daemon | ✅ |

### Database (`database.tf`)

| Requirement | Implementation | Pass |
|-------------|----------------|------|
| Aurora Multi-AZ | Writer + reader instances | ✅ |
| Serverless v2 | 0.5–8 ACU scaling | ✅ |
| PITR | `backup_retention_period=30` | ✅ |
| Encryption | `storage_encrypted=true` | ✅ |
| Deletion protection | `deletion_protection=true` | ✅ |
| Force SSL | `rds.force_ssl=1` | ✅ |
| Redis TLS | `transit_encryption_mode=required` | ✅ |
| Redis Multi-AZ | `multi_az_enabled=true, automatic_failover_enabled=true` | ✅ |
| Redis snapshots | 7-day retention | ✅ |
| S3 backups | Versioned, KMS, lifecycle | ✅ |

### Security (`security.tf`)

| Requirement | Implementation | Pass |
|-------------|----------------|------|
| KMS key rotation | `enable_key_rotation=true` | ✅ |
| Secrets Manager | JSON bundle, KMS encrypted | ✅ |
| 30-day rotation | Lambda rotation configured | ⚠️ Placeholder logic |
| WAF on ALB | Rate limit + managed rules | ✅ |
| AWS Backup | Daily + weekly Aurora | ✅ |

### Monitoring (`monitoring.tf`)

| Requirement | Implementation | Pass |
|-------------|----------------|------|
| ECS CPU/memory alarms | Threshold 80%/85% | ✅ |
| Task count alarm | `< desired_count` | ✅ |
| ALB 5xx/latency/unhealthy | Configured | ✅ |
| Aurora CPU/connections/lag | Configured | ✅ |
| Redis CPU/memory | Configured | ✅ |
| SNS email | Requires confirmation | ⚠️ Post-apply |

---

## Deployment Validation Procedure

```bash
# 1. Plan review
cd infrastructure/terraform
terraform init
terraform validate
terraform plan -var-file=production.tfvars -out=plan.out

# 2. Apply
terraform apply plan.out

# 3. Post-apply verification
bash ../../scripts/production-readiness/validate-infrastructure.sh

# 4. Populate secrets (never in Terraform state)
aws secretsmanager put-secret-value \
  --secret-id $(terraform output -raw secrets_manager_arn) \
  --secret-string file://secrets.json

# 5. Push engine image + deploy
# GitHub Actions: deploy-ecs.yml on push to main (engine/**)
```

---

## Sign-Off Criteria

| Criterion | Required State |
|-----------|----------------|
| `terraform validate` | Exit 0 |
| `terraform plan` | No unexpected destroys |
| All 8 TF files present | Yes |
| Lambda ZIP exists | Yes |
| `production.tfvars` populated | Yes |
| State backend bootstrapped | Yes |
| SNS subscription confirmed | Yes |
| `validate-infrastructure.sh` | All checks PASS |

**Current Verdict:** INFRASTRUCTURE DESIGN VALIDATED — DEPLOYMENT PENDING

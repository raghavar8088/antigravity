# 05 — Secrets Manager Migration Plan

**Secret Name:** `antigravity/production/engine`  
**Encryption:** KMS CMK `alias/antigravity-production-secrets`  
**Rotation:** 30-day automatic (Lambda placeholder — implement per-key logic)

---

## Migration Scope

| Secret | Current Location | Target | Rotation |
|--------|------------------|--------|----------|
| `DELTA_API_KEY` | Lightsail `.env` | Secrets Manager | Manual + API rotate |
| `DELTA_API_SECRET` | Lightsail `.env` | Secrets Manager | 30-day Lambda |
| `ANGELONE_API_KEY` | Vercel + `.env` | Secrets Manager (engine only) | Manual |
| `ANGELONE_CLIENT_CODE` | Vercel + `.env` | Secrets Manager | Manual |
| `ANGELONE_PIN` | Vercel + `.env` | Secrets Manager | Manual |
| `ANGELONE_TOTP_SECRET` | Vercel + `.env` | Secrets Manager | Manual |
| `BINANCE_API_KEY/SECRET` | `.env` | Secrets Manager | 30-day |
| `DATABASE_URL` | Neon/Lightsail `.env` | Secrets Manager + Aurora managed | RDS auto-rotate |
| `MONGODB_URI` | Vercel + `.env` | Secrets Manager + Vercel | Atlas rotation |
| `REDIS_URL` | Not set | Secrets Manager (generated) | On failover |
| `ENGINE_ADMIN_SECRET` | Vercel (empty risk) | Secrets Manager + Vercel | 90-day |
| `AUTH_JWT_SECRET` | Vercel only | Vercel (not in engine bundle) | 90-day |
| `CRON_SECRET` | Vercel only | Vercel | 90-day |
| `OPENAI/GEMINI/GROQ_API_KEY` | `.env` | Secrets Manager | 30-day |

---

## Pre-Migration: Git History Purge

**MANDATORY before populating production secrets.**

```bash
# 1. Backup repo
git clone --mirror . ../trading-backup.git

# 2. Purge sensitive files from history
pip install git-filter-repo
git filter-repo --path .env --path .env.local --path .env.production --invert-paths

# 3. Force push (coordinate with team)
git push origin --force --all

# 4. Rotate ALL credentials — assume compromised
```

---

## Migration Procedure

### Phase A: Terraform Creates Empty Secret

```bash
cd infrastructure/terraform
terraform apply -var-file=production.tfvars
SECRET_ARN=$(terraform output -raw secrets_manager_arn)
```

### Phase B: Build secrets.json (NEVER commit)

```json
{
  "MONGODB_URI": "mongodb+srv://...",
  "DATABASE_URL": "postgres://antigravity_admin:PASS@CLUSTER_ENDPOINT/antigravity?sslmode=require",
  "REDIS_URL": "rediss://:TOKEN@PRIMARY.cache.amazonaws.com:6379/0",
  "BINANCE_API_KEY": "...",
  "BINANCE_API_SECRET": "...",
  "DELTA_API_KEY": "...",
  "DELTA_API_SECRET": "...",
  "ANGELONE_API_KEY": "...",
  "ANGELONE_CLIENT_CODE": "...",
  "ANGELONE_PIN": "...",
  "ANGELONE_TOTP_SECRET": "...",
  "ENGINE_ADMIN_SECRET": "<64 random hex chars>",
  "OPENAI_API_KEY": "...",
  "GEMINI_API_KEY": "...",
  "GROQ_API_KEY": "..."
}
```

```bash
aws secretsmanager put-secret-value \
  --secret-id "$SECRET_ARN" \
  --secret-string file://secrets.json

rm secrets.json  # immediately
```

### Phase C: Vercel Environment Variables

Set in Vercel dashboard (NOT in engine bundle):

| Variable | Source |
|----------|--------|
| `AUTH_JWT_SECRET` | New 64-byte random |
| `CRON_SECRET` | New 64-char hex |
| `ENGINE_ADMIN_SECRET` | Same as Secrets Manager value |
| `INTERNAL_API_URL` | ALB DNS from Terraform output |
| `MONGODB_URI` | Same Atlas URI |

### Phase D: Verify Injection

```bash
# ECS task — secrets injected at launch, not in task definition plaintext
aws ecs describe-task-definition --task-definition antigravity-production-engine \
  | jq '.taskDefinition.containerDefinitions[0].secrets'

# Engine boot gate validates ENGINE_ADMIN_SECRET and DATABASE_URL present
```

### Phase E: Decommission Plaintext

1. Remove `.env` from Lightsail instance
2. Verify no secrets in `docker inspect` environment
3. Verify no secrets in CloudWatch logs (grep audit)
4. Enable AWS Config rule: `secretsmanager-secret-unused`

---

## Rotation Implementation

### Automatic (Lambda — 30 days)

Current: placeholder at `infrastructure/terraform/lambda/secret_rotation/index.js`

**Production implementation required:**

| Secret Type | Rotation Method |
|-------------|-----------------|
| `ENGINE_ADMIN_SECRET` | Generate new → dual-write period → revoke old |
| `DATABASE_URL` | Use RDS managed rotation (already enabled via `manage_master_user_password`) |
| Broker API keys | Broker-specific API rotation + update secret |
| `REDIS_URL` | ElastiCache auth token rotation (maintenance window) |

### Rotation Monitoring

```bash
# CloudWatch alarm (add to monitoring.tf)
# SecretRotationFailed — AWS/SecretsManager RotationFailed

aws secretsmanager describe-secret --secret-id antigravity/production/engine \
  --query '{LastRotated:LastRotatedDate,NextRotation:NextRotationDate}'
```

### Expiry Validation (Go-Live Gate)

```bash
# scripts/production-readiness/go-live-gate.sh checks:
# - Secret exists and has value
# - LastRotatedDate < 90 days ago (or rotation scheduled)
# - No secret version in AWSCURRENT older than rotation policy
```

---

## Rollback

If Secrets Manager unavailable:
1. ECS tasks fail to start (fail-fast — correct behavior)
2. Rollback: update `INTERNAL_API_URL` to Lightsail
3. Lightsail `.env` as emergency fallback (documented, not committed)

**Never** store fallback secrets in git.

---

## Sign-Off Checklist

- [ ] Git history purged
- [ ] All keys rotated post-purge
- [ ] `secrets.json` populated and deleted locally
- [ ] Vercel env vars set
- [ ] ECS task starts with injected secrets
- [ ] No plaintext in task definition `environment[]`
- [ ] Rotation Lambda deployed (non-placeholder)
- [ ] SNS alert on rotation failure configured

**Secrets Manager Readiness:** 45/100 (designed, not operational)

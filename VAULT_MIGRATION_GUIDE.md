# Vault Migration Guide — Phase 15G

## Overview

This guide migrates all trading platform secrets from `.env` files to HashiCorp Vault.
After migration, no secret ever touches disk, source code, or logs.

---

## Architecture

```
Application
   │
   ▼
FallbackProvider
   ├─► VaultProvider (primary)  →  HashiCorp Vault KV v2
   └─► EnvProvider (fallback)   →  OS environment variables
```

---

## Step 1 — Install Vault

### Option A: HashiCorp Vault Cloud (HCP)
```
https://portal.cloud.hashicorp.com
Create organization → Create cluster → Enable KV v2 at path "secret"
```

### Option B: Self-hosted on AWS
```bash
# EC2 t3.small in your VPC
wget -O- https://apt.releases.hashicorp.com/gpg | gpg --dearmor > /usr/share/keyrings/hashicorp-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" > /etc/apt/sources.list.d/hashicorp.list
apt-get install vault
vault operator init -key-shares=5 -key-threshold=3
```

---

## Step 2 — Enable AppRole Authentication

```bash
vault auth enable approle

# Create trading-engine policy
vault policy write trading-engine - <<EOF
path "secret/data/trading/*" {
  capabilities = ["read"]
}
path "secret/metadata/trading/*" {
  capabilities = ["list"]
}
EOF

# Create role
vault write auth/approle/role/trading-engine \
    token_policies="trading-engine" \
    token_ttl=1h \
    token_max_ttl=4h \
    secret_id_ttl=24h
```

---

## Step 3 — Write Secrets to Vault

```bash
# Exchange keys
vault kv put secret/trading/BINANCE_API_KEY value="your-key-here"
vault kv put secret/trading/BINANCE_API_SECRET value="your-secret-here"
vault kv put secret/trading/DELTA_API_KEY value="your-key-here"
vault kv put secret/trading/DELTA_API_SECRET value="your-secret-here"

# AI keys
vault kv put secret/trading/OPENAI_API_KEY value="sk-..."
vault kv put secret/trading/GROQ_API_KEY value="gsk_..."
vault kv put secret/trading/GEMINI_API_KEY value="AIza..."

# Database
vault kv put secret/trading/DATABASE_URL value="postgresql://..."
vault kv put secret/trading/REDIS_URL value="redis://..."
vault kv put secret/trading/MONGODB_URI value="mongodb+srv://..."

# Auth
vault kv put secret/trading/AUTH_JWT_SECRET value="$(openssl rand -hex 32)"
vault kv put secret/trading/ENGINE_ADMIN_SECRET value="$(openssl rand -hex 32)"
vault kv put secret/trading/INTERNAL_API_SECRET value="$(openssl rand -hex 32)"
```

---

## Step 4 — Configure the Engine

Add to your deployment environment (NOT `.env`):

```bash
VAULT_ADDR=https://vault.internal:8200
VAULT_TOKEN=<your-approle-token>
VAULT_MOUNT=secret/trading

# Remove from .env:
# BINANCE_API_KEY, BINANCE_API_SECRET, DELTA_API_KEY, etc.
```

The `vault.LoadFromEnv()` function automatically detects Vault when `VAULT_ADDR` and
`VAULT_TOKEN` are set, using env as fallback for any key not in Vault.

---

## Step 5 — Render Deployment

In Render dashboard → Environment → Add:
```
VAULT_ADDR    = https://your-vault-host:8200
VAULT_TOKEN   = hvs.XXXXXX (AppRole token)
```

Remove all secret environment variables from Render. They should now live in Vault only.

---

## Step 6 — Rotate Vault Token

```bash
# Generate a new AppRole secret ID (tokens are short-lived)
vault write -f auth/approle/role/trading-engine/secret-id
```

---

## Verification Checklist

- [ ] `vault kv get secret/trading/BINANCE_API_KEY` returns the key
- [ ] Engine starts with `VAULT_ADDR` set and reads Binance key from Vault
- [ ] Engine starts WITHOUT `VAULT_ADDR` and reads from env (fallback works)
- [ ] Removing a key from Vault causes a logged error (not silent failure)
- [ ] Rotation log appears in `/api/security/audit` after calling `RotationEngine.Rotate()`
- [ ] `.env` file contains NO secrets (only non-sensitive config)

---

## Security Properties After Migration

| Property | Before | After |
|----------|--------|-------|
| Secrets at rest | .env file (plaintext) | Vault (AES-256-GCM encrypted) |
| Secrets in logs | Possible | Never (vault provider never logs values) |
| Secrets in memory | Forever | Evicted after 5-min cache TTL |
| Rotation | Manual | Automated (30-day schedule) |
| Audit trail | None | Every Vault read is logged |
| Emergency response | Manual delete .env | `EmergencyRotateAll()` in <60 seconds |

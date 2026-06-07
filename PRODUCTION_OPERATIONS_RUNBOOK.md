# Production Operations Runbook

AWS Lightsail hosts the Go execution engine. Vercel hosts the Next.js dashboard. MongoDB Atlas is the single source of truth.

## Architecture (post Phase 31 alignment)

| Component | Role | Account key |
|-----------|------|-------------|
| Go engine (`antigravity_engine`) | Sole execution authority | `mock_trading_default` |
| JWT session (`/api/auth/login`) | Dashboard auth | `userId = mock_trading_default` |
| Paper Desk APIs (`/api/paper-desk/*`) | Read MongoDB | JWT `userId` |
| Legacy TS worker | **Disabled** when `ENGINE_EXECUTION_AUTHORITY=1` (default) | N/A |

## Required environment variables

### Lightsail `/home/ubuntu/antigravity/.env`

```bash
OWNER_ACCOUNT_KEY=mock_trading_default
MONGODB_URI=mongodb+srv://...
MONGODB_DB=loop_trades
# Remove DESK_WORKER_ACCOUNT_KEY if present (especially anon_* values)
```

### Vercel (client project)

```bash
OWNER_ACCOUNT_KEY=mock_trading_default
AUTH_JWT_SECRET=<64-char hex>
ADMIN_USERNAME=...
ADMIN_PASSWORD_HASH=...
MONGODB_URI=...
MONGODB_DB=loop_trades
INTERNAL_API_URL=http://<lightsail-static-ip>
ENGINE_EXECUTION_AUTHORITY=1
# Remove DESK_WORKER_ACCOUNT_KEY
```

---

## SSH access recovery (P2)

### 1. Verify active key

```bash
# From your workstation — test default Lightsail key
ssh -i "LightsailDefaultKey-ap-south-1.pem" ubuntu@<LIGHTSAIL_IP> "echo ok"

# If permission denied, the instance has a different authorized key
```

### 2. Replace key via AWS Console

1. AWS Console → Lightsail → your instance → **Account** tab
2. Download default key for region OR create new key pair
3. Use Lightsail browser-based SSH (works without local key) to run:

```bash
mkdir -p ~/.ssh && chmod 700 ~/.ssh
echo "<YOUR_NEW_PUBLIC_KEY>" >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

4. Update GitHub secret `LIGHTSAIL_SSH_KEY` with the matching private key

### 3. Rotate key (recommended quarterly)

```bash
ssh-keygen -t ed25519 -f ~/.ssh/lightsail_engine -C "engine-deploy"
# Add lightsail_engine.pub to instance authorized_keys
# Replace LIGHTSAIL_SSH_KEY secret in GitHub
```

---

## Deployment checklist

### Pre-deploy

- [ ] Run `GET /api/system/production-validation` (authenticated) → `go_no_go: "GO"`
- [ ] Confirm `OWNER_ACCOUNT_KEY=mock_trading_default` on Lightsail + Vercel
- [ ] Confirm no `DESK_WORKER_ACCOUNT_KEY=anon_*` anywhere
- [ ] MongoDB Atlas IP whitelist includes Lightsail static IP

### Deploy engine (automatic on push to `main` under `engine/`)

GitHub Actions workflow: `.github/workflows/deploy.yml`

Manual deploy on host:

```bash
ssh -i LightsailDefaultKey-ap-south-1.pem ubuntu@<IP>
cd /home/ubuntu/antigravity
bash scripts/update-aws-engine.sh
```

### Post-deploy validation

```bash
curl -s http://<LIGHTSAIL_IP>/api/paper-desk/diagnostics | jq .
curl -s -b "raig_session=<cookie>" https://<vercel-app>/api/system/production-validation | jq .
```

Expected engine diagnostics:

```json
{ "account_key": "mock_trading_default", "mongo_connected": true }
```

---

## Emergency recovery

### Engine not trading / empty dashboard

1. Check account key alignment:
   ```bash
   curl http://<LIGHTSAIL_IP>/api/paper-desk/diagnostics
   ```
2. If `account_key` ≠ `mock_trading_default` → fix `.env`, restart container:
   ```bash
   docker restart antigravity_engine
   docker logs -f antigravity_engine --tail 200
   ```
3. Migrate legacy data if old `anon_*` key was used:
   ```bash
   cd client
   node scripts/migrate-collections-phase31.mjs --from=anon_e7da5e39
   node scripts/migrate-collections-phase31.mjs  # dry review first with --dry-run
   ```

### Negative balance

Engine now clamps balance ≥ 0 on settle, recovery, and snapshot. If detected:

```bash
docker logs antigravity_engine 2>&1 | grep "BALANCE INVARIANT"
```

### MongoDB disconnected

```bash
docker logs antigravity_engine 2>&1 | grep paperpersist
# Fix MONGODB_URI, verify Atlas network access, restart
docker restart antigravity_engine
```

### Rollback engine image

```bash
docker pull ghcr.io/<owner>/antigravity-engine:<previous-sha>
docker stop antigravity_engine && docker rm antigravity_engine
docker run -d --name antigravity_engine --restart always -p 80:8080 \
  --env-file /home/ubuntu/antigravity/.env \
  -v /home/ubuntu/antigravity/data:/app/data \
  ghcr.io/<owner>/antigravity-engine:<previous-sha>
```

Legacy `mock_trades` / `mock_account_snapshots` are never deleted by migration scripts (rollback-safe).

---

## Collection mapping

| Before (legacy) | After (canonical) | Writer | Reader |
|-----------------|-------------------|--------|--------|
| `mock_trades` | `paper_trades` | Go `TradeWriter` | `/api/paper-desk/trades` |
| `mock_account_snapshots` | `paper_state` | Go `StateSnapshotter` | `/api/paper-desk/state` |
| `paper_state.positions[]` | `paper_positions` | Go `OrderWriter` | `/api/paper-desk/positions` |
| `paper_oms_orders` | `paper_orders` | Go `OrderWriter` | `/api/paper-desk/orders` |
| — | `strategy_health` | Go `StrategyHealthMonitor` | `/api/paper-desk/strategy-health` |
| — | `equity_curve` | Go `EquityRecorder` | `/api/paper-desk/equity` |

---

## Useful commands

```bash
# Engine logs
docker logs -f antigravity_engine --tail 100

# Engine health
curl http://<IP>/health

# Full production validation (requires login cookie)
curl -b "raig_session=..." https://<app>/api/system/production-validation

# Mongo account key audit
mongosh "$MONGODB_URI" --eval 'db.getSiblingDB("loop_trades").paper_state.distinct("account_key")'
```

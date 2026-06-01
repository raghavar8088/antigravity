# Secrets Rotation Runbook — Phase 15G

## Rotation Schedule

| Secret | Schedule | Method | Rollback |
|--------|----------|--------|---------|
| BINANCE_API_KEY / SECRET | 30 days | Manual via exchange + Vault write | Yes (keep prev) |
| DELTA_API_KEY / SECRET | 30 days | Manual via exchange + Vault write | Yes |
| AUTH_JWT_SECRET | 7 days | Automated (RotationEngine) | No (forces re-login) |
| ENGINE_ADMIN_SECRET | 7 days | Automated | No |
| INTERNAL_API_SECRET | 14 days | Automated | No |
| OPENAI_API_KEY | 90 days | Manual via platform + Vault | Yes |
| DATABASE_URL | Manual only | DBA coordinated | Yes |
| REDIS_URL | Manual only | DBA coordinated | Yes |
| MONGODB_URI | Manual only | DBA coordinated | Yes |

---

## Scheduled Rotation (Automatic)

The `RotationEngine` invalidates the in-memory secret cache on schedule.
The next application `Get()` call fetches the new value from Vault.

```go
// In main.go (already wired):
policies := vault.DefaultRotationPolicies()
rotator := vault.NewRotationEngine(secretProvider, policies)
rotator.OnRotation(func(r vault.RotationRecord) {
    log.Printf("[ROTATION] %s rotated by %s success=%v", r.Key, r.By, r.Success)
})
rotator.Start(ctx)
```

---

## Manual Rotation Procedure

### Exchange API Keys (Binance)

1. Log into Binance → API Management → Create new key
2. Note the new key/secret pair
3. Write to Vault:
   ```bash
   vault kv put secret/trading/BINANCE_API_KEY value="new-key"
   vault kv put secret/trading/BINANCE_API_SECRET value="new-secret"
   ```
4. Call rotation API to invalidate cache immediately:
   ```bash
   curl -X POST https://engine.yourdomain.com/api/admin/rotate \
     -H "X-Admin-Secret: $ENGINE_ADMIN_SECRET" \
     -H "Authorization: Bearer $ADMIN_JWT" \
     -d '{"key":"BINANCE_API_KEY"}'
   ```
5. Verify: check `/api/security/audit` for `SECRET_ROTATED` event
6. Wait 5 minutes (cache TTL)
7. Disable old key on Binance

### JWT Secret (AUTH_JWT_SECRET)

**Warning:** Rotating JWT_SECRET immediately invalidates ALL active user sessions.
Users must re-login. Schedule during maintenance window.

1. Generate new secret:
   ```bash
   NEW_SECRET=$(openssl rand -hex 32)
   vault kv put secret/trading/AUTH_JWT_SECRET value="$NEW_SECRET"
   ```
2. Restart the engine (or call forced cache invalidation)
3. All sessions expire — users re-login
4. Verify: new logins succeed, old tokens rejected with 401

---

## Emergency Rotation (Credential Leak Suspected)

Activate when:
- API key appears in logs
- Unusual trading activity from unknown source
- Security incident raised by monitor

```bash
# Emergency rotate ALL secrets immediately
curl -X POST https://engine.yourdomain.com/api/admin/emergency-rotate \
  -H "X-Admin-Secret: $ENGINE_ADMIN_SECRET" \
  -H "Authorization: Bearer $ADMIN_JWT"
```

This calls `RotationEngine.EmergencyRotateAll()` which:
1. Invalidates the entire secret cache
2. Logs every rotation with `by="emergency"`
3. Forces all next reads to re-fetch from Vault

Then immediately:
1. Revoke old exchange API keys on each exchange platform
2. Issue new AppRole secret_id from Vault
3. Restart all engine instances
4. Review audit log for unauthorized access

---

## Rollback Procedure

If a rotated key causes failures:

```bash
# Read the previous version from Vault KV v2
vault kv get -version=N secret/trading/BINANCE_API_KEY

# Restore previous version
vault kv rollback -version=N secret/trading/BINANCE_API_KEY
```

Then invalidate the cache:
```bash
curl -X POST .../api/admin/rotate -d '{"key":"BINANCE_API_KEY"}'
```

---

## Audit

Every rotation appears in:
- `GET /api/security/audit` — last 200 audit events
- Structured logs: `[ROTATION] key rotated by=scheduled`
- Vault audit log: `vault audit enable file file_path=/var/log/vault-audit.log`

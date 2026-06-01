# Database Security Guide — Phase 15G

## 1. PostgreSQL (Neon TimescaleDB)

### Current State
- Connection via `DATABASE_URL` env var
- No TLS enforcement in application code
- Single superuser credentials

### Required Changes

#### Enforce TLS in Connection String
```
DATABASE_URL=postgresql://antigravity:<password>@<host>:5432/antigravity?sslmode=require&sslrootcert=/etc/ssl/certs/ca-certificates.crt
```

#### Create Least-Privilege Database Roles
```sql
-- Read-only role for analytics/backtest
CREATE ROLE trading_readonly;
GRANT CONNECT ON DATABASE antigravity TO trading_readonly;
GRANT USAGE ON SCHEMA public TO trading_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO trading_readonly;

-- Write role for engine (insert events, update positions)
CREATE ROLE trading_engine;
GRANT CONNECT ON DATABASE antigravity TO trading_engine;
GRANT USAGE ON SCHEMA public TO trading_engine;
GRANT SELECT, INSERT ON TABLE ledger_events TO trading_engine;
GRANT SELECT, INSERT, UPDATE ON TABLE positions TO trading_engine;
GRANT SELECT, INSERT ON TABLE audit_log TO trading_engine;
-- No DELETE, no DROP, no TRUNCATE

-- Reconciliation role (read-only + corrective writes)
CREATE ROLE trading_reconciliation;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO trading_reconciliation;
GRANT INSERT, UPDATE ON TABLE reconciliation_corrections TO trading_reconciliation;
```

#### Encrypt Sensitive Fields at Application Level
Fields to encrypt before write (AES-256-GCM):
- `ledger_events.payload` (contains fill prices, strategy names)
- `audit_log.meta` (may contain IP/user agent)

```go
// Use standard library AES-GCM
func encryptField(plaintext []byte, key [32]byte) ([]byte, error) {
    block, _ := aes.NewCipher(key[:])
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
```

---

## 2. MongoDB Atlas (Auth Sessions)

### Current State
- Connection via `MONGODB_URI` env var
- No IP whitelist documented
- Single user for all operations

### Required Changes

#### Atlas Network Access
1. Allowlist Render.com static IP (or use Atlas Private Link)
2. Allowlist Vercel Edge Function IP range (documented in Vercel dashboard)
3. Block all other IPs (`0.0.0.0/0` should be removed from Atlas allowlist)

#### Atlas Database User — Least Privilege
```
User: trading_auth
Role: readWrite on loop_trades database only
Collections: auth_users, sessions, audit_logs
```

#### Enable MongoDB Audit Log
Atlas → Security → Database Access → Audit → Enable
Log actions: authenticate, find, insert, update, delete

#### Connection String with TLS
```
MONGODB_URI=mongodb+srv://trading_auth:<password>@cluster.mongodb.net/loop_trades?authSource=admin&tls=true&tlsAllowInvalidCertificates=false
```

---

## 3. Redis (Indicator/Performance Cache)

### Current State
- Connection via `REDIS_URL`
- May be using insecure transport

### Required Changes

```
REDIS_URL=rediss://:password@host:6380/0
# Note: rediss:// (with two s) enforces TLS
```

#### Redis ACL (Redis 6+)
```
# redis.conf
ACL SETUSER trading_engine on >password123 ~* +GET +SET +DEL +EXPIRE +TTL
ACL SETUSER trading_readonly on >readpass ~* +GET +TTL
```

---

## 4. SQLite (Local Engine State)

### Current State
- Plain file at `SQLITE_PATH=./data/engine.db`
- No encryption

### Recommended
SQLite is for local fallback only. In production (Render):
1. SQLite should be disabled (`SQLITE_PATH=""`) — use MongoDB as primary
2. If SQLite is required, use SQLCipher (encrypted SQLite)
3. Ensure the data directory is on an encrypted volume (Render volumes are AES-256 encrypted by default)

---

## 5. Connection Security Checklist

| Database | TLS | Least Priv | Network ACL | Audit Log | Encryption at Rest |
|----------|-----|------------|-------------|-----------|-------------------|
| PostgreSQL (Neon) | ✅ Neon enforces TLS | ⚠️ TODO: role separation | ✅ Neon managed | ⚠️ TODO: enable | ✅ Neon default |
| MongoDB Atlas | ✅ Atlas enforces TLS | ⚠️ TODO: least-priv user | ⚠️ TODO: whitelist IPs | ⚠️ TODO: enable | ✅ Atlas default |
| Redis | ⚠️ Use rediss:// | ⚠️ TODO: ACL | ⚠️ Depends on provider | N/A | ⚠️ Provider-dependent |
| SQLite | N/A (local) | N/A | N/A | N/A | ⚠️ TODO: encrypted FS or disable |

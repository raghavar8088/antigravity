# 07 — Broker Security Validation Framework

**Principle:** No anonymous path to place, cancel, or query broker balances/orders.

---

## Route Security Matrix

| Route | Auth | Broker | Status |
|-------|------|--------|--------|
| `POST /api/angelone/order` | Session JWT | Angel One | ✅ Secured |
| `POST /api/angelone/cancel-order` | Session JWT | Angel One | ✅ Secured |
| `GET /api/angelone/funds` | Session JWT | Angel One | ✅ Secured |
| `GET /api/angelone/orders` | Session JWT | Angel One | ✅ Secured |
| `POST /api/delta-live/order` | Session JWT | Delta | ✅ Existing |
| Engine direct broker | ECS Secrets Manager | All | ⚠️ Migration pending |

---

## Test Matrix

| ID | Test | Method | Expected | Severity |
|----|------|--------|----------|----------|
| BRK-01 | Angel One order anonymous | `POST /api/angelone/order` no cookie | 401 | CRITICAL |
| BRK-02 | Angel One cancel anonymous | `POST /api/angelone/cancel-order` | 401 | CRITICAL |
| BRK-03 | Angel One funds anonymous | `GET /api/angelone/funds` | 401 | CRITICAL |
| BRK-04 | Angel One orders anonymous | `GET /api/angelone/orders` | 401 | CRITICAL |
| BRK-05 | Delta order anonymous | `POST /api/delta-live/order` | 401 | CRITICAL |
| BRK-06 | Valid session places paper order | Authenticated POST | 200/202 | Functional |
| BRK-07 | Credentials not in response | Any broker route | No API keys in body | CRITICAL |
| BRK-08 | Credentials not in logs | CloudWatch/Vercel logs | No secrets logged | CRITICAL |
| BRK-09 | SSRF via broker proxy | Malicious order URL field | Rejected | HIGH |
| BRK-10 | Rate limit broker routes | 101 req/min | 429 | MEDIUM |

---

## Automated Script

```bash
export APP_URL="https://your-app.vercel.app"
bash scripts/production-readiness/validate-broker-security.sh
```

---

## Angel One Validation Procedure

```bash
# Anonymous — must fail
curl -s -o /dev/null -w "%{http_code}" -X POST "$APP_URL/api/angelone/order" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"RELIANCE-EQ","qty":1,"side":"BUY"}'
# Expected: 401

# Authenticated — must succeed (paper/small qty)
curl -X POST "$APP_URL/api/angelone/order" \
  -H "Cookie: raig_session=$VALID_SESSION" \
  -H "Content-Type: application/json" \
  -d '{"symbol":"RELIANCE-EQ","qty":1,"side":"BUY","paper":true}'
# Expected: 200
```

---

## Delta Exchange Validation

```bash
curl -s -o /dev/null -w "%{http_code}" -X POST "$APP_URL/api/delta-live/order" \
  -d '{"symbol":"BTCUSD","size":1}'
# Expected: 401
```

---

## Future Broker Integration Template

For any new broker route:

1. **Mandatory:** `getAuthenticatedApiSession()` at route entry
2. **Mandatory:** Credentials from Secrets Manager (engine) or server env (Vercel) — never `NEXT_PUBLIC_*`
3. **Mandatory:** Add to `validate-broker-security.sh` test matrix
4. **Mandatory:** Rate limit class `trade` in middleware
5. **Recommended:** Migrate execution to Go engine (Vercel read-only plane)

---

## Migration: Angel One → Go Engine

**Target:** Vercel never holds Angel One credentials.

| Step | Action |
|------|--------|
| 1 | Add `handleAngelOneOrder` to engine HTTP server |
| 2 | Proxy Vercel `/api/angelone/*` → engine via authenticated proxy |
| 3 | Remove Angel One keys from Vercel env |
| 4 | Validate BRK-01 through BRK-04 still pass |
| 5 | Paper order end-to-end test |

---

## Credential Leakage Audit

```bash
# Repo scan (pre-commit)
rg -i "ANGELONE_|DELTA_API|BINANCE_API" --glob '!*.example' --glob '!.env*'

# Response header audit
curl -I "$APP_URL/api/angelone/funds" -H "Cookie: raig_session=$SESSION"
# Must not contain: X-API-Key, Authorization with broker creds
```

---

## Sign-Off

| Criterion | Status |
|-----------|--------|
| Anonymous broker access impossible | ✅ Code verified |
| Production smoke tests pass | ❌ Pending deploy |
| Angel One migrated to engine | ❌ Phase 3 |
| Credential rotation complete | ❌ Pending |

**Broker Security Readiness:** 80/100

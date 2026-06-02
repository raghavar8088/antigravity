# Exchange Failover Runbook
**Version:** 1.0 | **Date:** 2026-06-02 | **Severity:** P1 when primary exchange is down

---

## Exchange Priority Matrix

| Priority | Exchange | Market | Role | Fallback |
|----------|----------|--------|------|---------|
| 0 | Delta Exchange | BTC Options | Primary derivatives | Binance |
| 1 | Coinbase WS | BTC-USD | Primary price feed | Binance REST |
| 2 | Binance REST | BTC Spot | Backup derivatives + price | Synthetic |
| 0 | AngelOne | NSE/BSE Equity | Primary Indian equities | NSE REST |
| 1 | NSE REST | NIFTY Index | Backup index feed | Cached |

---

## Automatic Failover (No Human Action Required)

The `ExchangeFailover` component in `engine/internal/ha/exchange_failover.go` handles this automatically:

1. Health probe checks each exchange every **5 seconds** via HTTP GET to `HealthURL`
2. After **3 consecutive failures**, exchange is marked `DOWN`
3. Traffic is automatically routed to the next healthy exchange by priority
4. When the primary recovers, traffic rebalances automatically

---

## Detection Signals

### Automatic Detection
```
# Prometheus alerts to watch:
ha_exchange_status{exchange="delta"} == 2       # DOWN
ha_exchange_failovers_total                     # Increasing
ha_exchange_health_check_latency_seconds > 2    # Degraded
```

### Manual Detection
- Delta Exchange status page: https://status.delta.exchange
- Binance status: https://status.binance.com
- AngelOne: Check API response codes in engine logs

---

## Manual Failover Procedure

If automatic failover has not triggered (e.g., probe URL available but orders failing):

### Step 1: Identify the failing exchange
```bash
# Check engine logs
grep "exchange" /var/log/trading-engine.log | tail -50

# Check Prometheus
curl http://engine:8080/metrics | grep ha_exchange
```

### Step 2: Force failover via API
```bash
# Force engine to stop using Delta and route to Binance
curl -X POST http://engine:8080/admin/exchange/failover \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"from": "delta", "force": true}'
```

### Step 3: Verify routing
```bash
curl http://engine:8080/admin/exchange/status
# Expected: {"active": "binance", "delta": "down", "binance": "healthy"}
```

### Step 4: Monitor for stability
```bash
# Watch exchange metrics for 2 minutes
watch -n 5 'curl -s http://engine:8080/metrics | grep ha_exchange'
```

---

## Scenario: Delta Exchange Full Outage

**Expected behaviour:**
1. Health probe to Delta fails 3 times (~15 seconds)
2. Delta marked DOWN, Binance promoted to active
3. All new BTC options order flow routes to Binance
4. Open Delta positions remain in OMS — not affected by routing change
5. Alert fires: `ExchangeDown` → PagerDuty/Slack

**Manual validation checklist:**
- [ ] Delta orders not being submitted
- [ ] Binance receiving new orders
- [ ] Open Delta positions visible in OMS
- [ ] Risk engine still calculating exposure correctly
- [ ] Kill switch NOT triggered (this is a routing issue, not a risk event)

---

## Scenario: AngelOne TOTP Expiry

AngelOne requires TOTP re-authentication every session.

**Symptoms:** `401 Unauthorized` in engine logs for `/api/angelone/*`

**Recovery:**
1. Re-generate TOTP using `ANGELONE_TOTP_SECRET`
2. POST to `/api/angelone/session` via the Next.js proxy
3. Engine will automatically retry with new session token
4. NSE market data fallback already active via NSE REST endpoint

---

## Scenario: Coinbase WebSocket Disconnect

**Expected behaviour:**
1. WebSocket reconnect attempted every 5 seconds (existing backoff logic)
2. After 3 failed reconnects, switch to Binance REST for BTC price
3. Strategies dependent on Coinbase WS price feed receive Binance price instead

**No manual action required for price feed switchover.**

---

## Recovery Validation

After exchange recovery, verify:

```bash
# 1. Exchange shows healthy
curl http://engine:8080/health/exchanges
# Expected: {"delta": "healthy", "binance": "healthy", "coinbase": "healthy"}

# 2. Reconciliation confirms no missing fills
curl http://engine:8080/api/reconciliation/status
# Expected: {"status": "clean", "pending_reconciliation": 0}

# 3. Positions are accurate
curl http://engine:8080/api/positions
```

---

## Escalation

| Condition | Action |
|-----------|--------|
| All BTC exchanges down | Activate kill switch immediately |
| AngelOne + NSE REST both down during market hours | Halt NSE strategies, alert |
| Failover active > 30 minutes | Escalate to infrastructure team |

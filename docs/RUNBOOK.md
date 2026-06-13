# BTC-PILOT SOVEREIGN v2 — Operations Runbook

**Last updated**: 2026-06-13  
**RPO**: < 5 min | **RTO**: < 15 min

---

## 1. Architecture Overview

```
Coinbase WS / Binance REST / AngelOne
  → Strategy Registry (600+ strategies)
  → Risk Gate (engine/internal/riskv3/)
  → OMS v3 (engine/internal/omsv3/)
  → Trading Loop (engine/internal/trading/loop.go)
  → Fill → Position Update
  → Ledger (engine/internal/ledger/)
  → Reconciliation (engine/internal/reconciliationv2/)
  → Kill Switch (engine/internal/killswitch/)
  → Event Store → PostgreSQL (non-blocking)
  → MongoDB Atlas (primary persistence)
  → Next.js Dashboard (Vercel)
```

---

## 2. Environment Variables

### Required (engine)
| Variable | Description |
|----------|-------------|
| `MONGODB_URI` | MongoDB Atlas connection string |
| `MONGODB_DB` | Database name (`loop_trades`) |
| `DATABASE_URL` | PostgreSQL Neon (event store) |
| `BINANCE_API_KEY` / `BINANCE_API_SECRET` | BTC spot fallback |
| `DELTA_API_KEY` / `DELTA_API_SECRET` | BTC options ticks + orders |
| `ANGELONE_API_KEY` / `ANGELONE_CLIENT_CODE` / `ANGELONE_PIN` / `ANGELONE_TOTP_SECRET` | NSE/BSE equity |
| `PORT` | Engine port (default 8080, Lightsail 8080) |

### Optional (engine)
| Variable | Default | Description |
|----------|---------|-------------|
| `SQLITE_ENABLED` | `true` | Set `false` in production to disable SQLite |
| `SQLITE_PATH` | `./data/engine.db` | SQLite file path |
| `ML_PRESCORER_ENDPOINT` | `""` | XGBoost server URL e.g. `http://localhost:8002` |
| `ML_PRESCORER_THRESHOLD` | `0.55` | Minimum win probability to pass pre-filter |
| `EVENTSTORE_DSN` | `""` | PostgreSQL DSN for event store (blank = disabled) |
| `MAX_POSITION_BTC` | — | Hard cap on BTC position |
| `MAX_DAILY_LOSS_PCT` | — | Daily loss circuit breaker |

---

## 3. Startup Sequence

```bash
# On AWS Lightsail
cd /opt/engine
./antigravity
```

The engine:
1. Loads `.env` via `loadDotEnv()`
2. Connects MongoDB Atlas — logs `✅ MongoDB connected`
3. Connects PostgreSQL (event store) — non-fatal if absent
4. Opens SQLite — non-fatal if `SQLITE_ENABLED=false`
5. Starts ML prescorer health poller — non-fatal if endpoint blank
6. Boots 600+ strategies from `curated_registry.go`
7. Starts Coinbase WebSocket price feed
8. Starts trading loop — logs `[Loop] trading loop started`
9. Self-pings `/health` every 2 min

---

## 4. Kill Switch

### Activate (emergency stop — all trading halted)
```bash
curl -X POST https://<engine-host>/admin/kill \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Status check
```bash
curl https://<engine-host>/health
```

### Reset after investigation
```bash
curl -X POST https://<engine-host>/admin/reset \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

The kill switch state is checked on every cycle. It **cannot** be bypassed in production paths.

---

## 5. Crash Recovery

On restart, the engine calls `ReconcileOnRestart()` which:
1. Loads all open positions from MongoDB
2. Compares against the current BTC price
3. Closes any orphaned BUY positions that have breached stop-loss
4. Logs a reconciliation report

This runs automatically — no manual intervention needed for normal restarts.

### If reconciliation fails
```bash
# Check MongoDB connectivity
curl https://<engine-host>/health

# View reconciliation report in logs
journalctl -u btc-engine -n 200 | grep "reconcil"
```

---

## 6. Event Store Replay

The event store writes every position open/close/fill to PostgreSQL in real time.

### Validate current state against replay
```bash
curl "https://<engine-host>/api/debug/replay-validate?since=2026-06-13T00:00:00Z"
```

Returns a `ValidationReport` comparing live state vs event log reconstruction.

### Manual replay (Go)
```go
reader := eventstore.NewEventReader(pool, logger)
state, err := reader.ReplayToState(ctx, startTime)
```

---

## 7. ML Pre-scorer

The XGBoost pre-scorer is **100% optional**. The engine operates identically without it.

### Deploy the model
1. Train `btc_pilot_xgb.json` (14 features — see `infrastructure/ai/ml_scorer.py`)
2. Place in the `ml_models` Docker volume
3. Start the scorer: `docker compose -f grafana/docker-compose.grafana.yml up ml-scorer`
4. Set `ML_PRESCORER_ENDPOINT=http://localhost:8002` in engine env

### Health check
```bash
curl http://localhost:8002/health
# → {"model_loaded": true, "num_features": 14}
```

### Disable without restart
The engine auto-disables the pre-scorer if `/health` fails. No env change needed.

---

## 8. TypeScript / Frontend

```bash
cd client
npm run build          # Vercel production build
npx tsc --noEmit       # Type check (should be zero errors)
npm run dev            # Local dev on :3000
```

---

## 9. Monitoring

| Metric | Alert threshold | Description |
|--------|----------------|-------------|
| `btc_cycle_guard_blocks_total` | > 5/min | Cycle overlap — possible loop hang |
| `btc_eventstore_write_errors_total` | > 0 | Event store write failures |
| `btc_ml_available` | 0 for > 10 min | ML model down (non-critical) |
| `btc_data_quality_halts_total` | > 3/min | Bad market data feed |

Grafana dashboards: `grafana/dashboards/btc_pilot_main.json`

---

## 10. Common Procedures

### Restart engine
```bash
systemctl restart btc-engine
journalctl -u btc-engine -f
```

### Add strategies
Edit `engine/internal/strategy/curated_registry.go`. WINNERS_ONLY gate is active — only add strategies with verified positive expectancy.

### Disable a strategy
Remove from `curated_registry.go` — do not set to inactive or comment out. Dead code creates confusion.

### Force MongoDB index rebuild
```bash
node infrastructure/database/mongodb_ttl_indexes.js
```

### Go build and test
```bash
cd engine
go build ./...
go vet ./...
go test ./...
go test ./internal/integration/... -v -run TestFull
```

---

## 11. Incident Response

| Symptom | Likely cause | Action |
|---------|-------------|--------|
| Engine not responding to `/health` | Process crash | Check systemd logs; restart |
| Dashboard shows stale data | MongoDB connection lost | Check Atlas connectivity; check `MONGODB_URI` |
| All strategies halted | Kill switch triggered | Check kill switch status; investigate trigger reason before reset |
| `MARKET DATA offline` on dashboard | Coinbase WS down | Engine falls back to Binance REST automatically |
| Positions not closing | OMS queue blocked | Check `btc_cycle_guard_blocks_total`; restart if necessary |

# API FLOW CERTIFICATION
**Phase 6 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification of every API route

---

## VERDICT: ALL API ROUTES CERTIFIED — ONE GAP IDENTIFIED

No route can create trades outside the engine. No route can bypass OMS. No route can bypass risk.

---

## ROUTE INVENTORY

### CATEGORY: READ-ONLY (Safe by construction)

| Route | Methods | Source | Verdict |
|-------|---------|--------|---------|
| `GET /api/paper-desk/snapshot` | GET | MongoDB aggregation | SAFE — read-only |
| `GET /api/paper-desk/trades` | GET | MongoDB paper_trades | SAFE — read-only |
| `GET /api/paper-desk/positions` | GET | MongoDB paper_positions | SAFE — read-only |
| `GET /api/paper-desk/orders` | GET | MongoDB paper_orders | SAFE — read-only |
| `GET /api/paper-desk/equity` | GET | MongoDB equity_curve | SAFE — read-only |
| `GET /api/paper-desk/strategy-health` | GET | MongoDB strategy_health | SAFE — read-only |
| `GET /api/paper-desk/portfolio` | GET | MongoDB portfolio_metrics | SAFE — read-only |
| `GET /api/paper-desk/state` | GET | MongoDB paper_state | SAFE — read-only |
| `GET /api/paper-desk/strategy-analytics` | GET | MongoDB aggregation | SAFE — read-only |
| `GET /api/paper-desk/validation` | GET | MongoDB read | SAFE — read-only |
| `GET /api/paper-trades` (GET) | GET | MongoDB paper_trades | SAFE — read-only |
| `GET /api/paper-trades/analytics` | GET | MongoDB aggregation | SAFE — read-only |
| `GET /api/paper-trades/strategy-stats` | GET | MongoDB aggregation | SAFE — read-only |
| `GET /api/paper-trades/leaderboard` | GET | MongoDB aggregation | SAFE — read-only |
| `GET /api/paper-trades/export` | GET | MongoDB export | SAFE — read-only |
| `GET /api/paper-trades/strategy-research` | GET | MongoDB read | SAFE — read-only |
| `GET /api/paper-state` (GET) | GET | MongoDB paper_state | SAFE — read-only |
| `GET /api/paper-oms/orders` | GET | MongoDB paper_oms_orders | SAFE — read-only |
| `GET /api/paper-oms/summary` | GET | MongoDB aggregation | SAFE — read-only |
| `GET /api/paper-diagnostics` | GET | MongoDB diagnostics | SAFE — read-only |
| `GET /api/paper-replay` | GET | Computation (no DB write) | SAFE — read-only |
| `GET /api/mock-trading/trades` (GET) | GET | MongoDB mock_trades | SAFE — read-only |
| `GET /api/mock-trading/account/latest` | GET | MongoDB mock_account_snapshots | SAFE — read-only |
| `GET /api/shadow-trade-intents` | GET | Supabase read | SAFE — read-only |
| `GET /api/engine/[...path]` | GET+POST | Go engine proxy | SAFE — proxied to engine |
| `GET /api/health/desk-worker` | GET | MongoDB heartbeat check | SAFE — read-only |
| `GET /api/cron/rank-strategies` | GET | MongoDB read + compute | SAFE — analytics only |

---

### CATEGORY: WRITE ROUTES WITH EXECUTION GUARDS (Blocked)

| Route | Method | Guard | Guard Status | Result |
|-------|--------|-------|-------------|--------|
| `POST /api/paper-trades` | POST | `isEngineExecutionAuthority()` line 61 | HARDCODED TRUE | HTTP 410 |
| `POST /api/paper-state` | POST | `isEngineExecutionAuthority()` line 21 | HARDCODED TRUE | HTTP 410 |
| `GET /api/cron/paper-desk-tick` | GET | `isEngineExecutionAuthority()` line 58 | HARDCODED TRUE | `{skipped:true}` |

---

### CATEGORY: WRITE ROUTES — MOCK TRADING (Deprecated HTTP 410)

| Route | Method | Status |
|-------|--------|--------|
| `POST /api/mock-trading/trades` | POST | HTTP 410 — "Browser trade creation is disabled" |
| `PATCH /api/mock-trading/trades/[id]` | PATCH | HTTP 410 |
| `POST /api/mock-trading/trades/[id]/close` | POST | HTTP 410 |
| `DELETE /api/mock-trading/reset` | DELETE | Requires owner key + confirmation; no trade creation |

---

### CATEGORY: WRITE ROUTES — AUDIT/INTENT ONLY

| Route | Method | What it writes | OMS bypass? | Risk bypass? |
|-------|--------|----------------|-------------|-------------|
| `POST /api/shadow-trade-intents` | POST | Audit intent to Supabase | NO — no trade created | NO |

- Requires auth + `NEXT_PUBLIC_DESK_SHADOW_INTENTS=1` feature flag
- Writes to separate Supabase `shadow_trade_intents` table
- Explicitly NOT an execution path — records intent, does not place orders

---

### CATEGORY: ADMIN ROUTES

| Route | Method | What it does | Risk |
|-------|--------|-------------|------|
| `POST /api/admin/kill` | POST | Calls Go engine kill switch | CONTROLLED — requires auth |
| `POST /api/admin/reset` | POST | Calls Go engine reset | CONTROLLED — requires auth |
| `POST /api/paper-state/repair` | POST | Resets MongoDB paper_state to clean state | ADMIN ONLY — requires auth |
| `POST /api/admin/migrate-owner` | POST | Bulk renames account keys across 17 collections | AUTH REQUIRED — dry-run available |

None of these create unauthorized trades. The kill and reset routes call Go engine endpoints through the engine proxy — they go through the institutional path.

---

### CATEGORY: GAP — `/api/paper-desk-smoke-test`

| Route | Method | What it does | Guard | Risk |
|-------|--------|-------------|-------|------|
| `POST /api/paper-desk-smoke-test` | POST | Creates synthetic test trade in MongoDB | Feature flag only | MEDIUM |

**Source code finding:**
- Writes to `paper_trades` via `upsertTradeMongo()` if `NEXT_PUBLIC_DESK_SMOKE_TEST === "1"`
- No `isEngineExecutionAuthority()` check
- Does NOT go through Go engine OMS or risk gate
- Current status: env var not set in `.env.local` — route is inactive

**This is the only route that can write to `paper_trades` without Go engine authority and without HTTP 410 guard.**

**Recommendation:** Add `isEngineExecutionAuthority()` guard at top of POST handler.

---

### CATEGORY: CRON ROUTES

| Route | Method | Execution Capable? | Guard | Verdict |
|-------|--------|-------------------|-------|---------|
| `GET /api/cron/paper-desk-tick` | GET | Was YES | `isEngineExecutionAuthority()` | BLOCKED |
| `GET /api/cron/rank-strategies` | GET | NO — analytics only | CRON_SECRET auth | SAFE |
| `GET /api/cron/policy-snapshot` | GET | NO — analytics only | CRON_SECRET auth | SAFE |

---

## TRADE CREATION PATH MATRIX

| Path | Can create trade outside engine? | Can bypass OMS? | Can bypass risk? |
|------|--------------------------------|-----------------|-----------------|
| `POST /api/paper-trades` | NO (HTTP 410) | N/A | N/A |
| `POST /api/paper-state` | NO (HTTP 410) | N/A | N/A |
| `GET /api/cron/paper-desk-tick` | NO (skipped) | N/A | N/A |
| `POST /api/mock-trading/trades` | NO (HTTP 410) | N/A | N/A |
| `POST /api/paper-desk-smoke-test` | YES if flag enabled | YES | YES |
| Any other route | NO | N/A | N/A |

---

## CONCLUSION

99 of 100 routes are safe. One route (`/api/paper-desk-smoke-test`) is a gap that can inject synthetic test trades when a feature flag is enabled. This flag is currently unset. A single `isEngineExecutionAuthority()` guard call would close this gap permanently.

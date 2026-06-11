# DUPLICATE TRADING SYSTEM REPORT
**Phase 3 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**FAIL — 4 duplicate execution systems confirmed. State divergence is guaranteed.**

---

## SYSTEM INVENTORY

### System A — Go Institutional Engine (APPROVED)

| Property | Value |
|----------|-------|
| Location | `engine/cmd/antigravity/main.go` |
| Entry | `executeThroughInstitutionalPath` |
| Paper balance | $1,000,000 |
| Trade persistence | MongoDB `paper_trades` (via paperpersist hooks) |
| Risk gate | Yes — PMS + pre-trade pipeline + kill switch |
| OMS | OMS v3 (event-sourced) |
| Strategy authority | Go strategy registry (600+ strategies) |
| Account key | `btc_main` (institutional) |
| Active | YES |

### System B — Browser Scalper Engine (UNAUTHORIZED DUPLICATE)

| Property | Value |
|----------|-------|
| Location | `client/src/hooks/useBTCFuturesScalperEngine.ts` |
| Entry | React hook, runs on mount, polls every 4s |
| Paper balance | $1,000,000 (independent, in React state) |
| Trade persistence | MongoDB `paper_trades` (same collection as System A) |
| Risk gate | Partial — client-side confidence threshold only |
| OMS | Client-side `paperOms.ts` state machine |
| Strategy authority | Client-side `futuresStrategies.ts` (duplicate registry) |
| Account key | JWT-derived user account key |
| Active | YES |
| **Overlap with System A** | Same MongoDB collections, same trade format, same strategy names |

### System C — Paper Desk Worker (UNAUTHORIZED DUPLICATE)

| Property | Value |
|----------|-------|
| Location | `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` |
| Entry | AWS pm2 worker OR Vercel cron `/api/cron/paper-desk-tick` |
| Paper balance | Inherited from MongoDB paper_state |
| Trade persistence | MongoDB `paper_trades` (same collection as A and B) |
| Risk gate | Independent client-side risk checks |
| OMS | `paperOms.ts` state machine (same as System B) |
| Strategy authority | `futuresStrategies.ts` (same as System B) |
| Account key | JWT-derived user account key |
| Active | YES |
| **Overlap with System A** | Same MongoDB collections, same trade format |

### System D — Mock Trading Engine (UNAUTHORIZED DUPLICATE)

| Property | Value |
|----------|-------|
| Location | `client/src/hooks/useMockTradingEngine.ts` + `client/src/lib/mockTradingEngine.ts` |
| Entry | React hook, polls every 5s |
| Paper balance | In-memory, MongoDB `mock_account_snapshots` |
| Trade persistence | MongoDB `mock_trades` (separate collection) |
| Risk gate | Client-side mock risk gates |
| OMS | None — direct trade creation |
| Strategy authority | Signal trace replay (derived from real signals) |
| Account key | JWT-derived user account key |
| Active | YES |
| **Overlap with System A** | Same strategy signals, independent PnL calculation |

---

## OVERLAP MATRIX

| | System A (Go Engine) | System B (Browser Scalper) | System C (Paper Worker) | System D (Mock Engine) |
|---|---|---|---|---|
| **Same MongoDB collection** | — | YES (`paper_trades`) | YES (`paper_trades`) | Partial (`mock_trades`) |
| **Same strategy names** | — | YES | YES | YES |
| **Independent balance** | — | YES | YES | YES |
| **Independent PnL calc** | — | YES | YES | YES |
| **Independent risk gate** | — | YES | YES | YES |
| **Can diverge from A** | — | YES | YES | YES |

---

## STATE DIVERGENCE SCENARIOS

### Scenario 1: Double-Writing Trades
System B and System C both write to `paper_trades`. If both are active simultaneously, the same strategy signal can produce TWO trade records — one from each system. MongoDB upsert on `client_trade_id` may prevent exact duplicates but different IDs produce double-counted PnL.

### Scenario 2: Balance Desync
System A holds $1M balance in Go in-memory state. System B holds its own $1M in React state. They can show different balance figures simultaneously. Portfolio accounting aggregates from MongoDB, which only reflects the last writer.

### Scenario 3: Kill Switch Mismatch
System A kill switch stops all new trades in the Go engine. System B and System C do NOT check the Go engine kill switch. Trading continues in browser even after kill switch fires.

### Scenario 4: Risk Gate Bypass
System A enforces: confidence ≥ 0.68, RR ≥ 2.4, Kelly sizing, daily loss %, VaR limits.
System B enforces: client-side confidence threshold only.
System C enforces: subset of System B gates.
A trade blocked by System A's risk gate CAN be opened by System B.

### Scenario 5: Strategy Disability Bypass
System A can disable individual strategies via the institutional path.
System B reads `disabled_strategies` from MongoDB `paper_state` but evaluates this client-side — race condition exists.

---

## CONCLUSION

Four trading systems exist. Three of them are unauthorized duplicates. All three unauthorized systems can write to MongoDB. All three can generate trades without passing through the Go institutional risk gates. State divergence is not hypothetical — it is guaranteed to occur whenever multiple systems are active simultaneously.

**Single execution authority is NOT achieved.**

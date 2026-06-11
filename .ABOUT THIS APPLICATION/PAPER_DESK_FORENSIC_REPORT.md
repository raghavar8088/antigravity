# PAPER DESK FORENSIC REPORT
**Phase 2 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**FAIL — Browser trading is ACTIVE.**

The paper desk has THREE independent execution sources. Trades CAN be created in the browser. Positions CAN be opened in the browser. PnL IS calculated in the browser. Strategies DO execute in the browser.

---

## AUDIT: useBTCFuturesScalperEngine

**File:** `client/src/hooks/useBTCFuturesScalperEngine.ts`
**Size:** 3,967 lines
**Poll interval:** 4,000ms

### Capabilities
- Fetches klines from exchange APIs every 4 seconds
- Evaluates ALL registered futures strategies (signal generation)
- Opens paper positions via `openAndTrackPosition()`
- Manages position exits (TP / SL / TIME / TRAIL / BREAKEVEN)
- Calculates unrealized PnL from open positions on every tick
- Calculates realized PnL on every trade close
- Applies funding rate accrual to open positions
- Manages $1M paper balance independently
- Writes trades to MongoDB via `persistTradeToServer()` → `/api/paper-trades`
- Writes account state to MongoDB via `/api/paper-state`
- Maintains complete portfolio state in React state (no backend required)

### Verdict
**CAN generate trades in browser: YES**
**CAN open positions in browser: YES**
**CAN calculate PnL in browser: YES**
**CAN execute strategies in browser: YES**
**Writes to MongoDB: YES**

---

## AUDIT: useMockTradingEngine

**File:** `client/src/hooks/useMockTradingEngine.ts`
**Size:** 1,073 lines
**Poll interval:** 5,000ms (trace poll), 15,000ms (persist throttle)

### Capabilities
- Polls strategy signal trace every 5 seconds
- Ingests signal rows and converts them to mock trades
- Applies price ticks to open mock positions (unrealized PnL)
- Triggers SL/TP exits on each tick
- Maintains complete in-browser mock portfolio
- Writes mock trades to MongoDB via `/api/mock-trading/trades`
- Writes account snapshots to MongoDB every 15 seconds

### Verdict
**CAN generate trades in browser: YES**
**CAN open positions in browser: YES**
**CAN calculate PnL in browser: YES**
**CAN execute strategies in browser: YES (signal → trade pipeline)**
**Writes to MongoDB: YES**

---

## AUDIT: paperDeskWorker / runPaperDeskPollTick

**File:** `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts`
**Runners:**
- `scripts/btc-ft-paper-worker.ts` — AWS pm2 long-running process
- `client/src/app/api/cron/paper-desk-tick/route.ts` — Vercel 1-min cron (failover)

### Capabilities
- Fetches Delta Exchange klines directly (bypasses Go engine)
- Evaluates all strategies on 1m candles
- Opens positions with full risk checks (independent of Go risk gate)
- Manages exits (TP/SL/TIME)
- Creates OMS orders via client-side `paperOms.ts` state machine
- Returns closed trades for MongoDB persistence

### Verdict
**CAN generate trades server-side (outside Go engine): YES**
**CAN open positions outside Go engine: YES**
**Uses Go risk gate: NO — independent risk evaluation**
**Writes to MongoDB: YES (via caller)**

---

## AUDIT: paper-state API Route

**File:** `client/src/app/api/paper-state/route.ts`

### POST capability
- Receives account state from browser engine
- Writes directly to MongoDB `paper_state` collection
- Fields written: balance, positions, pause_entries, disabled_strategies, worker state

**Can a trade state be created via this route: YES (indirectly via balance/position fields)**

---

## AUDIT: paper-trades API Route

**File:** `client/src/app/api/paper-trades/route.ts`

### POST capability
- Receives closed trade from browser engine
- Validates via `paperTradePostBodySchema`
- Skips write if `isEngineExecutionAuthority()` → true (partial guard)
- If Go engine NOT active: writes trade directly to MongoDB

**Guard status: PARTIAL — only blocks when Go engine authority flag is set**
**Can browser create trades if Go engine down: YES**

---

## AUDIT: mongo-direct write libraries

### `client/src/lib/mongoTradesClient.ts`
- `upsertTradeMongo()` — writes trade document to `paper_trades`
- `upsertAccountState()` — writes account state to `paper_state`
- Called by: `useBTCFuturesScalperEngine` → `paperTradesSync.ts`

### `client/src/lib/paperTradesSync.ts`
- `persistTradeToServer()` — queues + fires POST `/api/paper-trades`
- `flushTradeSyncQueue()` — retries up to 3 times on failure
- In-memory queue survives page refresh failures

### `client/src/lib/paperOms.ts` + `client/src/lib/paperOmsMongo.ts`
- Client-side OMS order state machine
- `insertPaperOmsOrder()` / `updatePaperOmsOrder()` write to `paper_oms_orders`

---

## DATABASE WRITE SUMMARY (Client-Side)

| Collection | Written by | Route | Status |
|-----------|-----------|-------|--------|
| `paper_trades` | Browser scalper engine | `/api/paper-trades` POST | ACTIVE |
| `paper_state` | Browser scalper engine | `/api/paper-state` POST | ACTIVE |
| `mock_trades` | Browser mock engine | `/api/mock-trading/trades` | BLOCKED (410) |
| `mock_account_snapshots` | Browser mock engine | `/api/mock-trading/account` | ACTIVE |
| `paper_oms_orders` | Browser paper worker | `paperOmsMongo.ts` | ACTIVE |
| `shadow_trade_intents` | Browser | `shadowTradeIntentSync.ts` | ACTIVE (logging) |

---

## ANSWER TO EACH AUDIT QUESTION

| Question | Answer |
|----------|--------|
| Can any trade be created in browser? | **YES** — useBTCFuturesScalperEngine creates trades every 4s |
| Can any position be opened in browser? | **YES** — same hook manages open positions |
| Can any PnL be generated in browser? | **YES** — unrealized + realized PnL calculated client-side |
| Can any strategy execute in browser? | **YES** — all futures strategies evaluated client-side |
| Does paper desk write to MongoDB? | **YES** — paper_trades, paper_state, paper_oms_orders |

---

## CONCLUSION

The paper desk is NOT removed. It is fully active. Browser trading exists and is generating trades every 4 seconds. The Go engine authority guard (`isEngineExecutionAuthority()`) is a partial flag that does not fully prevent browser execution.

**Required action:** Full removal of E17–E24 (Phase 7).

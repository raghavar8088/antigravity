# Root Cause Analysis — Paper Trades Not Persisting to MongoDB

## Summary

Three independent root causes prevented paper trades from being executed and persisted:

---

## Root Cause 1 — htf15 Indicators All NaN (No trades generated)

**File**: `client/src/app/api/btc/futures-klines/route.ts`

**Symptom**: 224 strategies active, zero trades placed per minute.

**Cause**: The candle fetch window was `130 * 60` seconds (130 minutes). At 1-minute resolution:
- `htf15` (15m timeframe) received `130 / 15 = 8` bars — below EMA9's warmup of 9.
- `htf5` (5m timeframe) received `26` bars — below MACD(12,26,9)'s required 34 bars.
- All htf indicator values computed as `NaN`, causing `evalBtcFtTemplateSignal` to produce 0 scores.
- Generated strategy batches (IDs 300–399) rely on `btcFtTemplate` evaluation → all scored 0 → threshold 22 never met.

**Fix (commit 59f9250)**: Extended window to `400 * 60` seconds:
- htf15 now gets 26 bars (EMA9 ✓, EMA21 ✓)
- htf5 now gets 80 bars (MACD ✓)

---

## Root Cause 2 — Research Threshold Too High / No Bootstrap Probe (No trades generated in research mode)

**Files**: `client/src/lib/btcFtResearch.ts`, `client/src/components/BTCFutureTradingScalper.tsx`

**Symptom**: Research mode with 224 strategies still produced zero trades.

**Cause**:
1. `researchSignalThreshold()` defaulted to 22. Generated template strategies typically score 20–21 after htf fix → still blocked.
2. `researchEnsureTradesEnabled()` required `NEXT_PUBLIC_BTC_FT_RESEARCH_ENSURE_TRADES=1` (opt-in) but env was unset.
3. `paperBootstrapProbe` was only enabled when `paperEnsureTrades`, which required `!EFFECTIVE_RESEARCH_MODE` — so research mode never got a bootstrap probe.

**Fix (commits 59f9250, ec0e18a)**:
- Research threshold default lowered: 22 → 20, min clamp 22 → 18.
- `researchEnsureTradesEnabled()` flipped to opt-out (disable with `=0`).
- `paperBootstrapProbe={paperEnsureTrades || EFFECTIVE_RESEARCH_MODE}` — research mode now also gets a probe after 5 min with zero trades.

---

## Root Cause 3 — Persistence Failures Silent (Trades executed but not in MongoDB)

**Files**: `client/src/lib/paperTradesSync.ts`, `client/src/app/api/paper-trades/route.ts`

**Symptom**: Trades appear in the UI trades table (React state) but are missing from MongoDB Atlas.

**Cause**:
1. `persistTradeToServer` is fire-and-forget (`void`). Any POST failure (400 Zod error, 503 Mongo unconfigured, network error) was silently caught, the trade enqueued, and after 3 retries dropped with only a `console.warn` — no dev logging at the point of failure.
2. GET `/api/paper-trades` had `account_key` typed as `string` but could be `undefined` at runtime. MongoDB driver strips `undefined` from filters, resulting in queries that return all documents rather than a 400 error.

**Fix**:
- `paperTradesSync.ts`: Added `console.warn` in dev when POST returns non-2xx, logging status + response body.
- `paper-trades/route.ts` GET: Added explicit `account_key` presence check, returns `400` if missing.

---

## Additional Changes

| File | Change |
|------|--------|
| `src/lib/mongoTradesClient.ts` | Added `pingMongo()` for health endpoint |
| `src/app/api/health/paper-desk/route.ts` | New: `GET /api/health/paper-desk` → `{ mongoConfigured, mongoPingOk, supabaseConfigured, lastPostError }` |
| `src/app/api/paper-trades/route.test.ts` | New: Vitest tests (success, 503, 400, 500) |
| `scripts/test-mongo.mjs` | New: manual end-to-end MongoDB smoke test |
| `client/package.json` | Added `test:mongo` script |

---

## Verification Checklist

1. `npm run test:mongo` — exits 0, all 5 checks pass
2. `GET /api/health/paper-desk` → `{ mongoConfigured: true, mongoPingOk: true }`
3. Open paper desk → wait ≤5 min for bootstrap probe to fire
4. `GET /api/paper-trades?account_key=<anon_key>&limit=5` returns the trade
5. MongoDB Atlas `loop_trades.paper_trades` has document with `client_trade_id`, `net_pnl`, `strategy_name`, `account_key`
6. `npm run test` — all Vitest tests pass
7. `npm run build` — clean build, no TypeScript errors

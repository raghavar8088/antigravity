# MongoDB Paper-Trades Storage — End-to-End Fix

## Why trades were not appearing in Atlas

Three independent failures were stacked. Any one of them would break persistence; together they were invisible because the client swallowed errors.

### 1. Client POST failures were silent (highest-impact)

`paperTradesSync.persistTradeToServer` is fire-and-forget (`void`). When the POST returned a non-2xx — even a `400 Validation failed` or `500 Mongo write failed` — the trade was pushed to the local retry queue and the error was swallowed. After 3 retries it was dropped with a single `console.warn`. No log told the developer **which** trade failed or **why**.

**Fix**: structured `[paper-sync]` logs at every stage (start, network error, non-2xx with body snippet, ok). See `client/src/lib/paperTradesSync.ts`.

### 2. Route handler swallowed Mongo errors via `upsertTradeMongo` throwing

`upsertTradeMongo` previously threw on driver/connection errors. The route wrapped it in `try/catch` and returned a generic 500, but downstream the client couldn't distinguish "Mongo unconfigured" from "Mongo refused write" from "validation failed". Worse — `$setOnInsert: { ...row }` meant re-sending a trade with corrected fees/funding was a **no-op**, hiding partial writes.

**Fix**:
- `upsertTradeMongo` now returns `{ ok, upsertedCount, modifiedCount } | { ok:false, error }`. Never throws.
- Mongo write uses `$set` for row fields (so refreshes work) + `$setOnInsert` for `created_at` only.
- Route maps every failure to an explicit code: `MONGO_NOT_CONFIGURED` (503), `MONGO_WRITE_FAILED` (500), `AUTH_REQUIRED` (401), `VALIDATION_FAILED` (400), `INVALID_JSON` (400).
- Successful response shape: `{ ok: true, storage: "mongo", clientTradeId, upsertedCount, modifiedCount }`.

### 3. GET handler returned all docs when account_key was missing

`paperTradeGetQuerySchema` marks `account_key` optional. The route did `parsed.data.account_key as string` (a TypeScript lie) then queried `{ account_key: undefined }`. The MongoDB driver strips `undefined` values from filters, so the query degenerated to `{}` — returning **every** trade in the collection (up to `limit`). Locally this looked like "trades exist somewhere"; in reality each user/anon-key was leaking across the boundary.

**Fix**: explicit `account_key` presence check → 400 `ACCOUNT_KEY_REQUIRED` when missing.

---

## Other things buttoned down

| Change | File |
|--------|------|
| `isMongoConfigured()` validates URI scheme (`mongodb+srv://` or `mongodb://`) | `mongoTradesClient.ts` |
| `pingMongo()` for health probes | `mongoTradesClient.ts` |
| `GET /api/health/storage` → `{ mongo: { configured, pingOk, db, collection }, supabase: { configured }, serverTime }` | `app/api/health/storage/route.ts` |
| `GET /api/health/paper-desk` (earlier endpoint, kept for backwards-compat) | `app/api/health/paper-desk/route.ts` |
| Session-aware auth on POST: prefers JWT cookie user.id; falls back to body.accountKey only when `ALLOW_PAPER_TRADES_ANON=1` (or legacy `ALLOW_ANON_PAPER_TRADES=1`) | `app/api/paper-trades/route.ts` |
| `flushTradeSyncQueue` throttled to 10s globally; mount and post-signin paths use `{ force: true }` | `paperTradesSync.ts` |
| `[paper-close]` log in `closePosition` (dev only) with clientTradeId, accountKey, moduleKey, netPnl, exitReason | `useBTCFuturesScalperEngine.ts` |
| `client/scripts/test-mongo.mjs` — six-step end-to-end smoke test (connect → ping → upsert → read → count → delete) | `client/scripts/test-mongo.mjs` |
| `npm run test:mongo` → runs the script with `--env-file=.env.local` | `client/package.json` |

---

## Verification flow

```bash
# 1. CLI smoke test — proves Atlas connectivity + paper_trades collection works
cd client
npm run test:mongo
# Expected: exit 0, all 6 steps OK
```

```bash
# 2. Server health from the running Next.js dev server
curl http://localhost:3000/api/health/storage
# Expected:
#   { "mongo": { "configured": true, "pingOk": true, "db": "loop_trades", "collection": "paper_trades" },
#     "supabase": { "configured": true|false }, "serverTime": "..." }
```

3. Open `http://localhost:3000/btc-future-trading`. Either sign in (gives a user-id account_key) or wait for the anon key (cookie/localStorage `anon_*`).
4. Wait for a paper close. In DevTools Console you should see, in order:
   - `[paper-close] { clientTradeId, accountKey, moduleKey, netPnl, exitReason }`
   - `[paper-sync] POST start { tradeId, accountKey, moduleKey }`
   - `[paper-sync] POST ok { tradeId, status: 200 }`
5. Browser → Network → POST `/api/paper-trades` → **200** response body:
   ```json
   { "ok": true, "storage": "mongo", "clientTradeId": "...", "upsertedCount": 1 }
   ```
6. Atlas → `loop_trades.paper_trades` → one new document with matching `client_trade_id`, `net_pnl`, `strategy_name`, `account_key`.
7. ```bash
   curl "http://localhost:3000/api/paper-trades?account_key=<your-key>&limit=5"
   ```
   Returns `source: "mongo"` and the trade in `trades[]`.

---

## Environment requirements

Server-only (NOT `NEXT_PUBLIC_*`):

```env
MONGODB_URI=mongodb+srv://USER:PASSWORD@cluster.xxxxx.mongodb.net/?retryWrites=true&w=majority&appName=LOOP-trades
MONGODB_DB=loop_trades

# Required for sign-in via /api/auth/signin (Mongo JWT auth).
AUTH_JWT_SECRET=<32+ char random string>

# Anon writes: when set, POST accepts body.accountKey without a session.
ALLOW_PAPER_TRADES_ANON=1
# Legacy alias accepted for backwards-compat:
ALLOW_ANON_PAPER_TRADES=1
```

Atlas checklist:

1. **Network Access** → add current public IP for local dev. For Vercel either allow `0.0.0.0/0` (research only) or use [Vercel-Atlas integration with private endpoint](https://www.mongodb.com/docs/atlas/security-vercel-integration/) for production.
2. **Database Access** → user `raghavar8088_db_user` (or whoever the URI names) has `readWrite` on `loop_trades`.
3. **Cluster name** in URI host matches the cluster shown in Atlas dashboard.
4. After editing `.env.local`, **restart `npm run dev`** — Next.js reads env at boot.

---

## Failure modes still possible (and how to recognise them)

| Symptom | Cause | Where to look |
|---------|-------|---------------|
| `[paper-sync] POST failed { status: 401, code: "AUTH_REQUIRED" }` | Anon flag not set AND no session | `.env.local` `ALLOW_PAPER_TRADES_ANON=1`; restart dev server |
| `[paper-sync] POST failed { status: 503, code: "MONGO_NOT_CONFIGURED" }` | `MONGODB_URI` missing server-side | `curl /api/health/storage` to confirm; check Vercel env |
| `[paper-sync] POST failed { status: 500, code: "MONGO_WRITE_FAILED" }` | URI valid but Atlas refusing — IP whitelist, auth, or cluster name | run `npm run test:mongo` to isolate |
| `[paper-sync] POST failed { status: 400, code: "VALIDATION_FAILED" }` | Trade payload missing/invalid field (e.g. `clientTradeId` not a UUID) | the `details` object lists the failing fields |
| `[paper-close]` never logs | `closePosition` not firing → no trades being executed | check signal threshold / candle window / bootstrap probe |
| `[paper-close]` logs but no `[paper-sync]` | `cloudAccountKey` is null | `usePaperDeskAuth` returning null user AND anon key disabled |

If the `[paper-close]` log appears but you never see `[paper-sync] POST start`, the bug is in `persistTradeToServer` — it short-circuits on null/empty `accountKey`.

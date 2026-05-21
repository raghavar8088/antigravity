# Deploy Checklist — Trading Application

## Required server environment variables

Set these in your hosting provider's dashboard (Vercel → Settings → Environment Variables).
Do **not** prefix server-only secrets with `NEXT_PUBLIC_`.

```env
# MongoDB Atlas (paper trades + auth)
MONGODB_URI=mongodb+srv://USER:PASSWORD@cluster.xxxxx.mongodb.net/?retryWrites=true&w=majority&appName=LOOP-trades
MONGODB_DB=loop_trades

# JWT signing secret for the raig_session cookie (32+ random chars).
AUTH_JWT_SECRET=<random 32+ char string>

# Allow anonymous POST writes (set both to 1 for personal/research deployments).
# SECURITY: anyone with the URL can write trades. Do NOT enable on multi-tenant.
NEXT_PUBLIC_ALLOW_ANON_PAPER_TRADES=1
ALLOW_PAPER_TRADES_ANON=1
ALLOW_ANON_PAPER_TRADES=1
```

Optional (legacy / parallel store):

```env
NEXT_PUBLIC_SUPABASE_URL=https://<project>.supabase.co
NEXT_PUBLIC_SUPABASE_ANON_KEY=<anon-key>
SUPABASE_SERVICE_ROLE_KEY=<service-role-key>
```

## MongoDB Atlas checklist

1. **Network Access** → add the IPs that need to write:
   - Local dev → current public IP
   - Vercel → either `0.0.0.0/0` (research-only deployments) OR set up the [Vercel ↔ Atlas private endpoint integration](https://www.mongodb.com/docs/atlas/security-vercel-integration/)
2. **Database Access** → DB user has `readWrite` role on the `loop_trades` database
3. Cluster name in `MONGODB_URI` host matches the cluster shown in the Atlas dashboard

## Verification (run in order)

```bash
# 1. CLI smoke test — proves Atlas is reachable AND collection accepts upserts.
cd client
npm run test:mongo

# 2. Run dev server (or curl your deployed URL) and check health.
curl http://localhost:3000/api/health/storage
# Expect: { "mongo": { "configured": true, "pingOk": true, ... }, ... }

# 3. Open the paper desk in a browser, wait for a paper trade to close.
#    DevTools console should show:
#      [paper-close] { clientTradeId, accountKey, ... }
#      [paper-sync] POST start { ... }
#      [paper-sync] POST ok { tradeId, status: 200 }

# 4. Confirm the document landed in Atlas:
curl "http://localhost:3000/api/paper-trades?account_key=<your-key>&limit=5"
# Returns source:"mongo" and the trade in trades[].
```

If any step fails, see `client/docs/MONGO_STORAGE_FIX.md` for the failure-mode → cause matrix.

## Test + build before deploy

```bash
cd client
npm run test    # Vitest — must be green
npm run build   # Next.js production build — must be green
```

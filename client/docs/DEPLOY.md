# Deploy — BTC futures paper desk

Production deployment guide for the Next.js app in **`client/`** (BTC Future Trading paper desk, Supabase auth, and cloud trade sync). This document does not change trading logic — see [PAPER_DESK_RUNBOOK.md](./PAPER_DESK_RUNBOOK.md) for operator tuning, env semantics, and replay.

---

## 1) Vercel project setup

1. Import the repository into [Vercel](https://vercel.com).
2. Set **Root Directory** to `client/` (not the monorepo root).
3. **Framework Preset:** Next.js (auto-detected).
4. **Build Command:** `npm run build` (default).
5. **Install Command:** `npm install` (default).
6. **Output:** Next.js default (no custom `output` required unless you already use one).
7. Add environment variables in **Project → Settings → Environment Variables** (Production, and Preview if you want parity). See §2.
8. Deploy. After the first production URL exists, add that origin to Supabase auth redirect URLs (§3).

**Local parity**

```bash
cd client
cp .env.local.example .env.local   # fill secrets
npm install
npm run dev
```

---

## 2) Required environment variables

Set these in Vercel **Production** (and Preview if needed). Values marked **required** must be present for cloud sync, auth, and paper-trades APIs.

### Supabase (required for cloud desk)

| Variable | Public? | Required | Notes |
|----------|---------|----------|--------|
| `NEXT_PUBLIC_SUPABASE_URL` | Yes | **Yes** | Project URL from Supabase → Settings → API |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | Yes | **Yes** | `anon` / publishable key — safe in browser; used for auth cookies |
| `SUPABASE_SERVICE_ROLE_KEY` | **No** | **Yes** | **Server only** — never prefix with `NEXT_PUBLIC_` |

### Paper desk policy (`NEXT_PUBLIC_DESK_*`)

All are exposed to the browser bundle. Omit any optional row to use the **code default** in the right column; the **prod-safe** column is a recommended starting point for production.

| Variable | Prod-safe value | Code default if omitted | Effect (summary) |
|----------|-----------------|-------------------------|------------------|
| `NEXT_PUBLIC_DESK_SLIPPAGE_BPS` | `5` | `0` | Adverse entry/exit slippage (bps), clamp 0–50 |
| `NEXT_PUBLIC_DESK_AUTO_DISABLE_STRATS` | `1` | off | Rolling Supabase stats → auto-disable losers |
| `NEXT_PUBLIC_DESK_KILL_WINDOW_DAYS` | `14` | `14` | Auto-disable lookback (days, 1–90) |
| `NEXT_PUBLIC_DESK_KILL_MIN_TRADES` | `5` | `5` | Min closes per strat before kill rules |
| `NEXT_PUBLIC_DESK_KILL_MAX_EXPECTANCY_USD` | `-0.05` | `-0.05` | Disable if mean net/trade below |
| `NEXT_PUBLIC_DESK_KILL_MAX_SUM_NET_USD` | `-1` | `-1` | Disable if sum net below |
| `NEXT_PUBLIC_DESK_VOL_SIZED_NOTIONAL` | `1` | off | Size opens from SL risk % of equity |
| `NEXT_PUBLIC_DESK_RISK_PCT_OF_EQUITY` | `0.01` | `0.01` | Risk fraction for vol sizing |
| `NEXT_PUBLIC_DESK_MAX_LAST_MARK_SPREAD_PCT` | `0.05` | `0.05` | Skip entry if \|last−mark\|/mark % too wide |
| `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY` | `3` | `3` | Max concurrent opens per strategy category |
| `NEXT_PUBLIC_DESK_ENTRY_UTC_START` | `0` | `0` | UTC entry window start hour |
| `NEXT_PUBLIC_DESK_ENTRY_UTC_END` | `24` | `24` | UTC entry end (`0` + `24` = always open) |
| `NEXT_PUBLIC_DESK_ENTRY_REPLACE_WEAKEST` | `0` | off | Replace weakest slot at max positions |
| `NEXT_PUBLIC_DESK_MAX_SAME_DIR_FRAC_OF_EQUITY` | `0.35` | `0.35` | Same-direction notional cap vs equity |
| `NEXT_PUBLIC_DESK_MIN_EXPECTED_MOVE_SAFETY_K` | `1` | `1` | ATR$ vs fee hurdle multiplier |
| `NEXT_PUBLIC_DESK_MIN_TP_SL_RATIO` | `2` | `2` | Min TP/SL ratio (desk RR filter) |
| `NEXT_PUBLIC_DESK_ENABLE_FAKE_DIV_STRATS` | omit / `0` | off | Branded fake-diversity strats |
| `NEXT_PUBLIC_DESK_REPLAY_UI` | `0` or **omit** | off | See §5 — keep off in production |

**Do not set in production** (dev-only; gated by `NODE_ENV=development` in code):

- `NEXT_PUBLIC_DESK_HOLD_TUNING_ANALYSIS_MODE`
- `NEXT_PUBLIC_DESK_HOLD_TUNING_EXPORT_MS`
- `NEXT_PUBLIC_DESK_REGIME_WATCH_MS`
- `NEXT_PUBLIC_DESK_REGIME_WATCH_POLL_WINDOW`
- `NEXT_PUBLIC_DESK_REGIME_HISTOGRAM_LS_PERSIST`

Template: `client/.env.local.example`.

---

## 3) Supabase setup

### Email auth (magic link)

1. Supabase Dashboard → **Authentication** → **Providers** → enable **Email**.
2. Confirm **Site URL** matches your primary app URL (e.g. `https://YOUR_DOMAIN`).
3. **Redirect URLs** (Authentication → URL configuration) — add **both**:
   - `https://YOUR_DOMAIN/auth/callback`
   - `http://localhost:3000/auth/callback` (local dev)

Users sign in from the desk header or `/sign-in`; the app exchanges the magic-link code at `/auth/callback`.

### SQL migrations (required)

Run in **SQL Editor** (or Supabase CLI) in order:

| File | Purpose |
|------|---------|
| `client/supabase/migrations/001_paper_trades.sql` | `paper_trades` table + index |
| `client/supabase/migrations/002_paper_trades_rls.sql` | **RLS required** — policies below |

**002 is mandatory in production.** Without RLS, direct client access to `paper_trades` is not scoped per user.

### Optional: legacy `account_key` copy (003)

If you previously stored trades under a namespace key (e.g. `btc_future_trading_20`) before auth:

1. Sign in once and note your user id: `select id, email from auth.users order by created_at desc limit 5;`
2. Run the commented `UPDATE` in `client/supabase/migrations/003_migrate_legacy_account_key.sql` (replace placeholders).

---

## 4) Security

| Rule | Detail |
|------|--------|
| Service role | `SUPABASE_SERVICE_ROLE_KEY` is used **only** in server route handlers (`createServiceSupabase()`). It bypasses RLS — never import it in client components or expose via `NEXT_PUBLIC_*`. |
| Public keys | `NEXT_PUBLIC_SUPABASE_URL` and `NEXT_PUBLIC_SUPABASE_ANON_KEY` are intended for the browser and middleware session refresh. |
| API pattern | `/api/paper-trades*` routes call `getAuthenticatedPaperApiUser()` first, then query with the **service role** filtered to `account_key = session.user.id`. |
| Desk env | `NEXT_PUBLIC_DESK_*` are policy toggles, not secrets — still avoid putting service role or Delta/live keys in any `NEXT_PUBLIC_` name. |

### RLS summary (`002_paper_trades_rls.sql`)

- **RLS:** enabled on `public.paper_trades`.
- **`paper_trades_select_own`:** role `authenticated` — `SELECT` where `account_key = auth.uid()::text`.
- **`paper_trades_insert_own`:** role `authenticated` — `INSERT` with `account_key = auth.uid()::text`.
- **Service role:** bypasses RLS for server APIs after session verification (admin scripts only on the server).

---

## 5) Production flags

| Setting | Production recommendation |
|---------|---------------------------|
| `NEXT_PUBLIC_DESK_REPLAY_UI` | `0` or **unset** |
| `NODE_ENV` | `production` on Vercel (automatic) |

With `NODE_ENV=production`:

- `GET /api/paper-replay` is **disabled** unless `NEXT_PUBLIC_DESK_REPLAY_UI=1` (do not enable in prod).
- Dev-only desk dumps (`__deskHoldTuningDump`, regime LS persist, etc.) do not run even if a dev env var is mistakenly set.

**Never on the production custom domain:** do not set `NEXT_PUBLIC_DESK_SHADOW_INTENTS`, `NEXT_PUBLIC_DESK_TESTNET_OPS`, or `DELTA_TESTNET` on the **Production** environment of your live Vercel project. Use §6 Staging instead.

---

## 6) Staging (preview only)

Use a **separate Vercel project** (or strictly **Preview** deployments) for operator experiments — shadow logging, manual testnet panel, and Delta testnet keys. Point **Supabase redirect URLs** at the preview hostname only; keep the production domain on the paper-desk + auth config from §2–§5.

| Rule | Detail |
|------|--------|
| **Separate project** | Recommended: e.g. `trading-desk-staging` with root `client/`, not the same project as prod |
| **Preview env scope** | If you use one repo + two projects, set staging-only vars under **Preview** (or a dedicated “Staging” custom domain), **never** under **Production** |
| **Production domain** | `NEXT_PUBLIC_DESK_SHADOW_INTENTS`, `NEXT_PUBLIC_DESK_TESTNET_OPS`, and `DELTA_*` testnet keys must be **unset** on prod |
| **Delta keys** | Testnet API key/secret only — no mainnet keys on staging |

### Staging-only environment variables

Set on **Preview** (or staging project) only:

| Variable | Staging value | Purpose |
|----------|---------------|---------|
| `NEXT_PUBLIC_DESK_SHADOW_INTENTS` | `1` | Log paper closes to `shadow_trade_intents` when signed in ([P3-C](./LIVE_TRADING_PHASE.md)) |
| `NEXT_PUBLIC_DESK_SHADOW_LOG_OPEN` | `0` or `1` | Optional: also log paper **opens** |
| `NEXT_PUBLIC_DESK_TESTNET_OPS` | `1` | Show **TESTNET ONLY** manual ops panel ([P3-B](./LIVE_TRADING_PHASE.md)) |
| `DELTA_TESTNET` | `1` or `true` | Required for testnet adapter + `would_place_testnet` on shadow rows |
| `DELTA_API_KEY` | testnet key | **Server only** — never `NEXT_PUBLIC_*` |
| `DELTA_API_SECRET` | testnet secret | **Server only** |

Also carry over from production staging parity: `NEXT_PUBLIC_SUPABASE_*`, `SUPABASE_SERVICE_ROLE_KEY`, and the same prod-safe `NEXT_PUBLIC_DESK_*` policy table (§2) unless you are deliberately testing different desk tuning.

**Leave off staging unless debugging replay in preview:**

- `NEXT_PUBLIC_DESK_REPLAY_UI` — optional on Preview only; still avoid production domain.

### Staging migrations

Run production migrations **001** + **002**, plus staging extras:

| File | Purpose |
|------|---------|
| `client/supabase/migrations/005_shadow_trade_intents.sql` | Shadow log table + RLS |
| `client/supabase/migrations/004_delta_audit_log.sql` | Optional testnet manual-order audit |

Compare paper vs shadow with [SHADOW_VS_PAPER.md](./SHADOW_VS_PAPER.md).

### Staging Supabase auth URLs

Add the **preview** origin only (examples):

- `https://your-staging-project.vercel.app/auth/callback`
- `https://staging.your-domain.com/auth/callback` (if you use a staging subdomain)

Do **not** rely on production `Site URL` alone if it points at the live app — add each staging URL explicitly.

### Staging verification

1. Deploy preview / staging project with table above.
2. Sign in on the **staging** URL (not prod).
3. Close a paper trade → rows in `paper_trades` and `shadow_trade_intents` for your `user_id`.
4. Desk profile → **Shadow log (last 20)** shows the close; `would_place_testnet` is `true` only when `DELTA_TESTNET=1` and keys are set (see [SHADOW_VS_PAPER.md](./SHADOW_VS_PAPER.md)).
5. **Export CSV**, leaderboard, and **Testnet ops** panel work when signed in; manual place/cancel hits **testnet only** (rate-limited).

---

## 7) Post-deploy verification

Use production URL and a real email inbox for magic link.

1. **Sign in** — open **BTC Future Trading** (`/btc-future-trading`), use header **Sign in** or `/sign-in`, complete email link.
2. **Header** — shows “Signed in as …”.
3. **Close a paper trade** — let the desk open and close at least one position (or use existing session).
4. **Supabase** — Table Editor → `paper_trades` → filter `account_key` = your user **UUID** (same as `auth.users.id`) → new row with unique `client_trade_id`.
5. **Leaderboard** — Desk profile → **Strategy leaderboard** loads (no 401); **Refresh** works when signed in.
6. **Export (optional)** — **Export CSV (30d)** downloads when signed in.

**Logged out:** local paper still runs; cloud POST, leaderboard, stats, and export return **401** (expected).

---

## 8) Operations reference

Day-to-day tuning, strategy profiles A/B, replay CLI, and metrics: **[PAPER_DESK_RUNBOOK.md](./PAPER_DESK_RUNBOOK.md)**. Live/testnet phases: **[LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md)**. Shadow vs paper SQL: **[SHADOW_VS_PAPER.md](./SHADOW_VS_PAPER.md)**.

---

## Quick checklist

### Production

- [ ] Vercel root = `client/` (production project)
- [ ] `NEXT_PUBLIC_SUPABASE_*` + `SUPABASE_SERVICE_ROLE_KEY` set (**Production** env)
- [ ] Prod-safe `NEXT_PUBLIC_DESK_*` (replay UI off)
- [ ] **No** `NEXT_PUBLIC_DESK_SHADOW_INTENTS`, **no** `NEXT_PUBLIC_DESK_TESTNET_OPS`, **no** `DELTA_TESTNET` on Production
- [ ] Supabase Email auth + redirect URLs for **production** domain
- [ ] Migrations **001** + **002** applied
- [ ] Sign in on prod → trade → `paper_trades` row → leaderboard OK

### Staging (separate preview project / Preview env only)

- [ ] Staging Vercel project or Preview deployments — **not** production domain
- [ ] `NEXT_PUBLIC_DESK_SHADOW_INTENTS=1`, `NEXT_PUBLIC_DESK_TESTNET_OPS=1`
- [ ] `DELTA_TESTNET=1` + testnet `DELTA_API_KEY` / `DELTA_API_SECRET` (server)
- [ ] Supabase redirect URL for **staging** hostname
- [ ] Migration **005** (+ optional **004**) applied
- [ ] Shadow log + testnet ops panel verified on staging URL only

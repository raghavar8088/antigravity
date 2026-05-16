This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Desk UI (Material Design 3)

Paper futures desks use a shared **M3-style** layer (Tailwind v4 + CSS tokens, no MUI):

| Piece | Location |
|-------|----------|
| Color, elevation, spacing, typography | `src/styles/desk-tokens.css` (imported from `globals.css`) |
| Primitives (cards, buttons, tables, app bar) | `src/components/desk/ui/` |
| Number formatting (`en-US`, hydration-safe equity in app bar) | `src/lib/deskFormat.ts` |
| Theme toggle | `DeskThemeToggle` — sets `data-theme` on `<html>` and `body.combat-mode` for dark |

**BTC Future Trading** uses `DeskShell` + `DeskAppBar` inside `BTCFuturesScalper.tsx`. Inner panels live in `BTCFuturesDeskPanels.tsx`. The workspace shell (`TradingDashboard`) uses `WorkspaceSettingsCard` and `WorkspaceNavPanel`.

### Component map

| UI area | Primitive |
|---------|-----------|
| App bar, equity, status | `DeskAppBar`, `StatusBadge` |
| Cards / sections | `DeskCard`, `DeskSectionHeader` |
| KPIs | `DeskMetricTile` |
| Tables (watchlist, positions, trades, leaderboard) | `DeskDataTable` |
| Tags (regime, exit reason, side) | `DeskChip` |
| Actions | `DeskButton` (filled / tonal / outlined) |
| Strategy on/off | `DeskSwitch` |
| Warnings (feed, testnet, replay off) | `DeskBanner` |
| Empty states | `DeskEmptyState` |
| Loading under app bar | `DeskLinearProgress` |
| Workspace nav | `DeskTabs` in `WorkspaceNavPanel` |

Theme: `DeskThemeToggle` + existing combat/light body class; `data-theme` on `<html>`.

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

## Auth (P1-L) — Supabase magic link {#desk-auth-setup}

1. In Supabase Dashboard → **Authentication** → enable **Email** provider (magic link).
2. Add redirect URL: `http://localhost:3000/auth/callback` (and your production origin).
3. Set in `.env.local`: `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_ROLE_KEY`.
4. Run migrations in order: `001_paper_trades.sql`, `002_paper_trades_rls.sql`.
5. Sign in from the desk header or `/sign-in`; closed trades sync with `account_key` = your `auth.users` id.

**Behavior**

| State | `storageNamespace` (e.g. `btc_future_trading_20`) | Cloud `account_key` |
|-------|---------------------------------------------------|---------------------|
| Logged out | local paper state + manual disables in `localStorage` | none — `POST` / leaderboard / strategy-stats return **401** |
| Logged in | same namespace for UI prefs only | `user.id` — same history on phone and desktop |

**Legacy migration:** After first sign-in, run `supabase/migrations/003_migrate_legacy_account_key.sql` in the SQL Editor to copy rows from `btc_future_trading_20` to your user id.

## BTC futures paper trades → Supabase

1. Copy `.env.local.example` to `.env.local` and set Supabase URL + anon + service role keys.
2. Run `supabase/migrations/001_paper_trades.sql` and `002_paper_trades_rls.sql` in the Supabase SQL Editor.
3. Sign in, open **BTC Future Trading**, close one paper position.
4. In **Table Editor → `paper_trades`**, filter `account_key` = your user id and confirm a new row.

Local `localStorage` (`{storageNamespace}_paper_state`) still mirrors desk UI every ~30s; Supabase is the durable trade log via authenticated `POST /api/paper-trades`.

Strategy PnL leaderboard: `GET /api/paper-trades/leaderboard` (session cookie) — under **Desk profile** when signed in. **Disable** on bottom rows merges into manual `disabledStrategies` in localStorage.

Per-category open cap (P1-M): `NEXT_PUBLIC_DESK_MAX_OPEN_PER_CATEGORY=3` limits concurrent opens per strategy `category` (default 3). Replay CLI: `--maxOpenPerCategory=3` (off by default for golden tests).

UTC entry session (P1-J): `NEXT_PUBLIC_DESK_ENTRY_UTC_START` / `NEXT_PUBLIC_DESK_ENTRY_UTC_END` (hours 0–23 / 0–24; default `0` + `24` = always open). Supports wrap (e.g. start `22`, end `6`). Replay: `--sessionStart=12 --sessionEnd=22` (off by default for golden tests).

## Offline paper desk replay (P1-D)

Deterministic **bar-by-bar** replay of the same signal + desk policy + exit pipeline as the live hook (`futuresReplayEngine.ts` + `futuresDeskRuntime.ts`). No React poll loop; no Supabase writes.

**Important:** Live polls every **~4s** (many marks per 1m bar). Replay uses **1 bar = 1 step**. Trade counts and timing will **drift** vs a live session even with the same klines.

### Fixtures

| File | Source |
|------|--------|
| `fixtures/replay/btcusd_1m_sample.json` | Synthetic (golden tests) |
| `fixtures/replay/btcusd_1m_live.json` | Delta 1m fetch (`npm run replay:fetch`) |

```bash
npm run replay:fetch -- --symbol=BTCUSD --bars=500
```

### CLI replay

```bash
npm run replay
npm run replay -- --fixture=live --bars=500 --slippageBps=5 --volSized=1
npm run replay -- --drawdownLock=1 --autoDisable=1 --disableIds=91,92
```

| Flag | Default | Notes |
|------|---------|--------|
| `--fixture=sample\|live` | `sample` | Which JSON file to load |
| `--slippageBps` | `5` | P1-A entry/exit slip |
| `--volSized=1` | off | P1-C vol notional |
| `--drawdownLock=1` | off | 25% / 21% entry pause (same as live) |
| `--autoDisable=1` | off | P1-B losers from Supabase 14d window |
| `--disableIds=1,2` | — | Extra manual disable list |
| `--maxOpenPerCategory=3` | off | P1-M per-category concurrent cap |
| `--sessionStart=12` / `--sessionEnd=22` | off | P1-J UTC entry window (both required to enforce) |

Golden Vitest uses **synthetic 100 bars**, `slippageBps=0`, no drawdown/auto-disable (stable snapshot).

### Compare live vs replay

Requires `.env.local` Supabase credentials:

```bash
npm run replay:compare -- --account_key=btc_future_trading_20 --date=2026-05-16
```

Prints a table: live Supabase closes that UTC day vs replay on the **live** fixture (same bar window when possible). Graceful message if Supabase is missing.

### Dev API

```text
GET http://localhost:3000/api/paper-replay?symbol=BTCUSD&bars=500&fixture=live&slippageBps=5&volSized=1&drawdownLock=0&autoDisable=0&account_key=btc_future_trading_20
```

Allowed when `NODE_ENV=development` or `NEXT_PUBLIC_DESK_REPLAY_UI=1`.

### Replay panel (UI)

On **BTC Future Trading** / **Future Trading**, open **Replay And Backtest** → **Run replay**. The panel calls the dev API above (slippage/vol-sized default from desk env unless overridden), shows trade count / sum net / expectancy / exit-reason mix, and a scrollable closed-trade table. It does **not** mutate the live hook. If the live fixture is missing, run `npm run replay:fetch` first.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.

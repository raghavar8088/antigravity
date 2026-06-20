# E2E tests (Playwright)

```bash
npx playwright install chromium   # one-time, downloads the browser binary
npm run test:e2e                  # headless run
npm run test:e2e:ui               # interactive UI mode
npm run test:e2e:report           # open the last HTML report
```

## How it's wired

- `playwright.config.ts`'s `webServer` runs a **production build** (`next build && next start`) in CI, and `next dev` locally. `next dev`'s on-demand per-route compilation caused real, repeatable flakiness once enough specs hit different routes concurrently against one shared dev server — the production server has no such compile-on-request latency. If you see local flakiness running the full suite, that's expected for dev mode; run `CI=true npm run test:e2e` to exercise the same build+start path CI uses.
- It also injects
  fixed test-only auth credentials (`ADMIN_USERNAME` / `ADMIN_PASSWORD_HASH` /
  `AUTH_JWT_SECRET`) directly as env vars on that process — no `.env.local`
  edits needed, and it never touches your real dev credentials.
- **No Go engine or MongoDB is started.** `MONGODB_URI` / `INTERNAL_API_URL`
  are deliberately left unset for the spawned server. Every engine/Mongo
  -backed API route in this app already has a graceful "offline" / "not
  configured" fallback (that's what real users see when those services are
  down) — the smoke tests assert pages render cleanly in that state.
- `e2e/global-setup.ts` logs in once via the API and saves the session to
  `e2e/.auth/owner.json` (gitignored). Every authenticated spec reuses it via
  the `e2e/fixtures/authedTest.ts` fixture instead of re-driving the login
  form, which avoids tripping the 5-attempts/15-min login rate limiter.
  `e2e/auth/login.spec.ts` is the one spec that drives the real `/login` UI.
- `e2e/fixtures/marketData.ts` intercepts the browser's `/api/btc/*` calls
  (price, klines) so specs don't depend on live Coinbase/Binance data.
- `e2e/fixtures/mockTrading.ts` additionally intercepts `/api/mock-trading/*`
  with one seeded OPEN trade — used only by `e2e/mock-trading/close-trade.spec.ts`
  to exercise the real "Close" interaction, since Mongo isn't running.

## Layout

| Path | Covers |
|---|---|
| `auth/login.spec.ts` | Login form, redirect guard, logout, rate limiting |
| `mock-trading/dashboard.spec.ts` | Public `/mock-trading`, unmocked (Mongo-offline fallback) |
| `mock-trading/close-trade.spec.ts` | Closing an OPEN trade (mocked Mongo data) |
| `terminal/smoke.spec.ts` | All ~25 authenticated `/terminal/*` routes load without crashing |
| `terminal/risk-killswitch.spec.ts` | Kill-switch panel's engine-offline fallback |
| `legacy-public/paper-desk.spec.ts` | `/paper-desk`, `/paperdesk`, `/btc-future-trading` |
| `mobile/mobile-nav.spec.ts` | `/mobile` emergency view (chromium-mobile project only) |

## Extending

- Browsers/devices: edit the `projects` array in `playwright.config.ts`.
- New mocked API: add a `page.route()` call in `e2e/fixtures/marketData.ts` or
  `e2e/fixtures/mockTrading.ts`, or a new fixture file if it's spec-specific.
- Want real engine + Mongo coverage? Run `next dev` yourself with a real
  `.env.local` (engine on :8080, Mongo configured), then `npx playwright test
  --config=playwright.config.ts` with `reuseExistingServer` (already true
  outside CI) — the suite will hit your real stack instead of the fallbacks.

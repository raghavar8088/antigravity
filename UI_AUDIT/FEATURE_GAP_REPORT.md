# PHASE 18 — FEATURE GAP REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## METHODOLOGY
Compare what the Go engine supports against what the UI exposes. Only evidence from source code accepted.

---

## GAP 1: KILL SWITCH STATUS DISPLAY
**Severity: CRITICAL**

Backend has:
- `/api/admin/ks/status` (READ) in engine proxy allowlist
- `/api/admin/ks/block` and `/api/admin/ks/release` (ADMIN) in engine proxy allowlist
- `engine/internal/killswitch/` — full kill switch with circuit breakers

UI has:
- Kill button (`DashboardHeader.tsx:264`) — fire-and-forget POST to `/api/admin/kill`
- **Missing**: Status indicator that polls `/api/admin/ks/status`
- **Missing**: Block/release granular controls
- **Missing**: Kill switch activation log / history
- **Missing**: Auto-kill circuit breaker event notifications

---

## GAP 2: RECONCILIATION VISIBILITY
**Severity: CRITICAL**

Backend has:
- `engine/internal/reconciliation/` — full reconciliation engine
- `/api/paper-state/repair` — repair endpoint
- `lib/deskSelfHealing.ts` + `lib/deskSelfHealingExecutor.ts`

UI has:
- **Nothing** — zero reconciliation UI

---

## GAP 3: ENGINE_ADMIN_SECRET NOT CONFIGURED
**Severity: CRITICAL**

Backend has:
- Engine proxy ADMIN_PATHS: `ks/block`, `ks/release`, `admin/kill`, `admin/reset-stats`, OMS write paths
- All require `ENGINE_ADMIN_SECRET` to be set in env

UI has:
- `ENGINE_ADMIN_SECRET` absent from `.env.local`
- Any admin action from Vercel returns HTTP 503
- **Kill switch via proxy is currently broken** (kill button in header bypasses proxy, hits engine directly — behavior differs by environment)

---

## GAP 4: LIVE BROKER POSITION / ORDER VISIBILITY
**Severity: CRITICAL**

Backend has:
- AngelOne live equity trading (`/api/angelone/*`)
- Delta Exchange live trading (`/api/delta/*`)
- Coinbase WS feed

UI has:
- `AngelOneOrderPanel.tsx` — MANUAL order placement only
- `useAngelOneOrders()` hook — exists but not confirmed as rendered in primary dashboard
- No live broker position dashboard
- No order status monitoring from live brokers

---

## GAP 5: BROKER CONNECTIVITY MONITOR
**Severity: HIGH**

Backend has:
- Multiple broker integrations with fallback chains (CLAUDE.md: Delta→Binance, NSE→AngelOne)

UI has:
- No broker connectivity status component
- No fallback chain status display

---

## GAP 6: DATA FEED HEALTH MONITOR
**Severity: HIGH**

Backend has:
- Coinbase WS primary, Binance REST fallback, NSE REST fallback
- Synthetic spot fallback if all fail

UI has:
- No feed health indicator
- No "using fallback feed" warning

---

## GAP 7: STRATEGY ADMINISTRATION PANEL
**Severity: HIGH**

Backend has:
- 600+ strategies in registry
- WINNERS_ONLY gate — auto-disable on poor performance
- Per-strategy health scoring

UI has:
- Aggregate health counts (healthy/warning/critical)
- Leaderboard (lazy-loaded, requires tab navigation)
- **No**: Per-strategy enable/disable toggle
- **No**: Per-strategy parameter view
- **No**: Strategy hot-swap controls
- **No**: Override WINNERS_ONLY gate for specific strategies

---

## GAP 8: MARKET REGIME CONTROLS
**Severity: MEDIUM**

Backend has:
- Regime classification (TRENDING_BULL, TRENDING_BEAR, RANGE, HIGH_VOL)
- Strategies gate on regime
- `lib/internal/regime/router.ts`

UI has:
- Regime badge in DashboardHeader (live from engine if connected)
- **No**: Manual regime override
- **No**: Regime → strategy eligibility matrix
- **No**: Regime transition history

---

## GAP 9: EXECUTION QUALITY METRICS
**Severity: MEDIUM**

Backend has:
- `PaperTradeDoc` with `fill_price`, slippage fields
- `lib/internal/execution/engine_v2.ts`

UI has:
- Trade table showing fill prices
- **No**: Slippage analysis dashboard
- **No**: Execution latency display
- **No**: Fill quality scoring

---

## GAP 10: MULTI-INSTRUMENT PORTFOLIO VIEW
**Severity: MEDIUM**

Backend has:
- BTC paper account (paper desk)
- NIFTY equity trades (AngelOne)
- NIFTY options (Delta, AngelOne)
- MCX commodities

UI has:
- Paper desk: BTC paper account only
- Each instrument has separate scalper page
- **No**: Unified portfolio across all instruments
- **No**: Aggregate risk across all instruments

---

## GAP 11: TERMINAL WEBSOCKET NOT WIRED
**Severity: CRITICAL — affects ALL terminal pages**

Backend would need:
- A Go engine endpoint that streams `TerminalSnapshot` deltas via WebSocket

UI has:
- Complete WebSocket client implementation in `terminalStore.tsx`
- All terminal components designed to consume live snapshot
- `NEXT_PUBLIC_TERMINAL_WS_URL` — missing from `.env.local`

The UI client is ready. The server-side WebSocket stream does not exist (no `/ws` endpoint found in engine proxy allowlist).

---

## GAP 12: ALERT NOTIFICATION SYSTEM
**Severity: HIGH**

Backend has:
- Health scoring with CRITICAL events
- Kill switch events
- Risk gate blocks

UI has:
- Static demo alert tape (mock data)
- Connection status badge
- **No**: Push notifications
- **No**: Email/SMS alerts
- **No**: Persistent alert history
- **No**: Alert acknowledgement

---

## GAP 13: WATCHDOG / HEARTBEAT DISPLAY
**Severity: MEDIUM**

Backend has:
- Execution watchdog (`engine/cmd/antigravity/main.go`)
- Self-ping every 2 minutes on `/health`

UI has:
- **No**: Watchdog status display
- **No**: Last heartbeat timestamp
- **No**: Engine uptime counter

---

## GAP 14: LEDGER VISIBILITY
**Severity: MEDIUM**

Backend has:
- `engine/internal/ledger/` — position accounting, fee tracking, funding

UI has:
- `DailyPnlLedger.tsx` — daily PnL summary (paper desk)
- **No**: Live ledger showing running fee totals
- **No**: Funding payment history
- **No**: Realized vs unrealized reconciliation vs ledger

---

## GAP SUMMARY TABLE

| Gap | Severity | Backend Ready | UI Gap |
|-----|----------|--------------|--------|
| Kill switch status display | CRITICAL | YES | Missing status poll |
| Reconciliation visibility | CRITICAL | YES | Zero UI |
| ENGINE_ADMIN_SECRET | CRITICAL | YES | Env not set |
| Live broker position/order view | CRITICAL | PARTIAL | Missing display |
| Broker connectivity monitor | HIGH | YES | Zero UI |
| Data feed health | HIGH | YES | Zero UI |
| Strategy administration | HIGH | YES | Partial (counts only) |
| Market regime controls | MEDIUM | YES | Indicator only |
| Execution quality metrics | MEDIUM | YES | Missing UI |
| Multi-instrument portfolio | MEDIUM | PARTIAL | Missing aggregation |
| Terminal WebSocket | CRITICAL | MISSING | Client ready, server missing |
| Alert notification | HIGH | YES | Only demo |
| Watchdog display | MEDIUM | YES | Zero UI |
| Ledger visibility | MEDIUM | YES | Partial (daily PnL only) |

**14 material feature gaps identified. 4 are CRITICAL severity.**

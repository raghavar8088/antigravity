# Live Engine — Phase 2 Implementation

## Context
Phase 0 feasibility is complete (`LIVE_ENGINE_PHASE0_FEASIBILITY.md`) and the decision is made:

- **Long-premium (option BUYING) only**, on **real Delta BTC options**. The naked option-selling
  roster is excluded from live capital — unbounded tail on a $100 account. Do not add it back.
- **Perp delta-proxy is rejected.** A short put is not a long perp; it changes the traded payoff.
- **Capital: $100.** Sufficient for the buying path once the sizing bug in Phase A.2 is fixed.
- **Leverage: 10x** is the account setting, and on long options it is **inert** — buying an option
  pays the premium in full, creates no borrow and no liquidation price. Do not build any feature,
  sizing rule, or UI element that implies 10x multiplies option-buying capacity. It does not.

A code audit found the Phase 0 report's architecture assumption to be wrong. **The corrections in
Phase A are blocking: no order may be placed on any venue until they are complete.**

---

## PHASE A — Fix the execution path. BLOCKING.

### A.1 The gateway's Delta adapter is a non-functional stub
`engine/internal/execution/delta_adapter.go` is what the mandated order path terminates in, and it
cannot place an order:

- **`deltaSign` (:191) is fake.** It discards the secret (`_ = secret`) and returns
  `fmt.Sprintf("hmac_%s_%s", method, timestamp[:8])`. Every authenticated request will be rejected.
- **`NewDeltaAdapter` (:27) has zero call sites** anywhere in `engine/`. It is dead code.
- **`deltaProductID` (:210) is perp-only** — maps BTCUSD→139, ETHUSD→140, returns `0` otherwise.
  Option products are per-strike/per-expiry numeric IDs and **cannot be expressed by a static
  symbol map**. Options are unaddressable through this adapter.
- **`GetPosition` (:139) and `GetBalance` (:161) are hollow** — they issue the HTTP call, discard
  the response body, and return an empty struct with only a timestamp. `GetOrder` (:109) always
  returns `Status: "UNKNOWN"`.

By contrast `engine/internal/delta/client.go` is real and complete: genuine
`hmac.New(sha256.New, ...)` signing (:147), `FindOptionProduct` (:443) against the live
`GET /v2/products?contract_types=put_options,call_options`, `PlaceOrder` (:189), `GetPositions`
(:333) against `/v2/positions/margined`, `GetWallet` (:306), `GetOpenOrders` (:399).

**Task:** make the institutional order path terminate in working code that can address an option.
Choose one and justify it in writing before implementing:
- **(i)** Rewrite `execution/delta_adapter.go` — real HMAC-SHA256 signing, option product
  resolution via `FindOptionProduct` rather than a static map, and real response parsing for
  position/balance/order status; or
- **(ii)** Route the gateway's Delta leg to the existing `delta.Client`, and delete
  `execution/delta_adapter.go` so a broken adapter cannot be wired up later by accident.

Do not leave two Delta adapters where one is a trap. Whichever path you take, **delete or fix the
stub — do not leave `deltaSign` in the tree returning a fabricated signature.**

### A.2 Verify what reconciliation actually reads — this is the dangerous one
The spec requires reconciling against Delta's real positions before accepting any signal after a
restart. There is a second, apparently working adapter at
`engine/internal/reconciliationv2/delta_reconciliation.go` (`GetPositions` :169, with a test
`TestGetPositions_UsesMarginedEndpointAndParsesStringFields` confirming it parses the margined
endpoint).

**Determine which adapter the live reconciliation path actually calls.** If any reconciliation or
restart-recovery path reads through `execution.DeltaAdapter.GetPosition`, it receives an **empty
position set and reports success against no data** — a silent false-green on the exact control
meant to prevent double-positions after a crash. Trace it, prove which one is wired, and write a
test that fails if reconciliation ever passes against an unparsed or empty broker response.

A control that cannot fail is not a control.

### A.3 Close the direct-order enforcement gap
`delta/live_bridge.go` already delegates decisions to the institutional stack via
`SetInstitutionalOpenHandler` / `SetInstitutionalCloseHandler` (:117, :124), and `SubmitOrder`
(:131) / `SubmitReduceOnlyOrder` (:145) are documented *"callable only from institutional fill
callbacks."* **That is a comment, not enforcement.** Nothing prevents a direct call to
`SubmitOrder` or `PlaceManualOrder` (:576) bypassing the risk gate.

Make it structural: unexported effectors, a callback-identity/token check, an internal-only
package boundary, or equivalent. Add a test proving an order cannot reach Delta without having
passed the risk gate and the kill-switch check.

Also confirm the retired Next.js routes stay retired — `/api/delta/testnet/place-order` and
friends must keep returning `blockedDirectExecutionRoute()`. Never restore a frontend broker path.

---

## PHASE B — Fix sizing. BLOCKING.

`engine/internal/delta/bridge_buy_sizing.go` has three defects that make the $100 risk budget
decorative:

1. **Tier boundary excludes the ceiling.** `case walletUSD < 100` (:49) means a wallet of exactly
   $100 — the configured account size — falls through to `default` (12% risk, 5 contracts), not
   the intended 10%/3 tier. Decide the correct tier for exactly $100 and make the boundary
   inclusive of the intended side.

2. **The premium estimate is not a market number.** `DELTA_BUY_EST_PREMIUM_USD` defaults to **35**
   (:85). A Delta BTC option contract is 0.001 BTC; $35/contract implies $35,000/BTC of premium
   ≈ 55% of spot — deep ITM or long-dated, not the short-dated OTM contracts these strategies
   buy. First-principles ATM pricing (`0.4 × S × σ × √T`) puts a weekly ATM contract near
   **$1.78** and ~3% OTM weeklies near **$0.30–$0.45**. **Replace the hardcoded guess with the
   real quote** from `FindOptionProduct` + `/v2/tickers` mark price (and the L2 book once wired).
   Keep an env override as a fallback only, and log loudly whenever the fallback is used.

3. **The floor overrides the risk budget.** `n := int(math.Round(spend / est)); if n < 1 { n = 1 }`
   (:90–93) buys one contract even when the computed budget cannot afford it. At $12 budget and
   the $35 default, this places a position at ~35% of the account against a 12% budget. **If the
   risk budget cannot afford one contract, the correct result is zero contracts and a logged skip
   reason — never a forced position.** Add a test for exactly this case.

Sizing must never place a position larger than the risk budget it just computed. Add a
server-side assertion that rejects any order violating this, independent of the sizing function.

---

## PHASE C — Build the module

Route `client/src/app/live-engine/`. Sidebar entry **"Live Engine"** directly under Command Center
in the TRADING group.

**Capital ceiling.** Enforce the **$100 cap server-side in the Go engine.** The UI must not be able
to raise it. Equity above the ceiling is not tradeable. This is a hard limit, not a default.

**Order path — no exceptions:**
```
signal → POST /api/execution/request (venue=delta, authenticated)
       → Go institutional gateway
       → risk gate (engine/internal/risk/gate/pipeline.go)
       → OMS v3 (engine/internal/omsv3/)
       → Delta options adapter (the working one from Phase A)
       → fill → ledger → reconciliation → persistence
```
The kill switch (`engine/internal/killswitch/`) stays wired and is checked before every order —
repo Hard Rule #5, never bypassed. Reuse `engine/internal/risk/daily_loss.go` and
`circuit_breakers.go`; do not write new breakers.

**Safety behaviour, in code and not merely config:**
- Ships **disarmed**. `LIVE_ENGINE_AUTO_ENABLE` stays `false`. Arming requires an authenticated,
  typed-confirmation action in the UI — never a bare toggle, never a URL, never on boot.
- **Auto-disarm** on: daily-loss breaker trip, N consecutive broker rejects, reconciliation
  mismatch, market data staler than a set age, or price-feed loss. Auto-disarm is one-way;
  re-arming is always a human action.
- **Panic CLOSE ALL** in one click from the module header and from `/mobile`, working even if the
  strategy loop is wedged.
- **Idempotency:** every order carries a client order ID derived from the signal. Retry,
  reconnect, or process restart must never double-fill. Prove it with a test.
- **Crash recovery:** reconcile against Delta's real positions before accepting any new signal.
  Never resume from local state alone.

**Roster eligibility** — a gate, not a leaderboard slice. Read the ranked leaderboard from
`/api/options-buying/[...path]`; do not re-implement scoring. A strategy goes live only if it
passes, in order:
1. Long-premium only (every naked short is excluded by decision).
2. The existing go-live gates — reuse `client/src/lib/risk/futuresGoLiveGates.ts` and
   `LIVE_TRADING_PHASE.md §3`. Do not invent thresholds. Codified: ≥200 trades / ≥30d for full
   live (50 / 7d paper-ready), expectancy > 0, PF ≥ 1.1, fees ≤ 0.5%.
3. **A real-fill gate.** Every number on those desks came from a synthetic Black-Scholes chain
   (`engine/internal/options/pricer.go`). Synthetic performance does not satisfy a real-money
   gate. A strategy must earn a real-fill or testnet-fill record meeting the same bar first.

Show per strategy: which gates pass, which fail, current value vs. requirement, and the reason it
is or is not live. **No strategy is ever live without a visible, inspectable reason.** Log roster
changes with actor and timestamp. Respect WINNERS_ONLY (Hard Rule #3).

**Cost transparency — surface this, do not bury it.** Delta charges 0.03% of notional
(~$0.019/side on a $64 contract) capped at 10% of premium. On a $0.30 premium that is ~13%
round-trip; at $0.19 the cap binds at 20%; at $1.78 it is ~2%. Plus real bid/ask. Display
**round-trip cost as a percentage of premium** per candidate contract, and flag any fill where
cost exceeds a configurable threshold. This ratio is per-contract and does not improve with
account size — it is the live edge question, so make it impossible to overlook.

**UI.** Build only from existing primitives — `DeskShell`, `DeskAppBar`, `PageHeader`, `DeskCard`,
`DeskMetricTile`, `DeskDataTable`, `DeskChip`, `StatusBadge`, `DeskBanner`, `DeskEmptyState`,
`DeskLinearProgress`, `DeskSwitch`, `ConfirmModal`, `PnlValue` — and the tokens in
`client/src/styles/desk-tokens.css`. Match the Options Buying desk exactly: Sora display, Manrope
UI, JetBrains Mono tabular numerals on every number, right-aligned numerics, sticky headers,
contained horizontal scroll, skeleton (never zero-value) loading states. Verify against
`/terminal/design-system` in both themes.

**This module must never be mistaken for a paper desk.** Persistent `REAL MONEY · $100` badge in
the app bar, armed/disarmed state visible without scrolling, `--loss`/`--warn` tones for armed.
Any surface that can show both paper and real labels each explicitly.

Sections in order: (1) arm/disarm + CLOSE ALL; (2) live account strip — real equity, available and
used margin, open risk, realized P&L today, distance to breaker, each labeled with source and age,
never showing a stale value without saying so; (3) live positions with entry, mark, unrealized
P&L, margin, and originating strategy; (4) orders/fills with full lifecycle including reject
reason text; (5) live roster with gate status and real-vs-synthetic record side by side;
(6) reconciliation panel — engine state vs. Delta truth, mismatches shown loudly; (7) audit log —
every arm, disarm, roster change, breaker trip, manual intervention.

---

## PHASE D — Staged rollout. Do not skip or compress.
1. **Shadow** — signals logged, no orders. Verify signal → intent → would-be order, and idempotency.
2. **Testnet** (`DELTA_TESTNET=true`) — full order path. Verify fills, rejects, reconciliation,
   restart recovery, kill switch, breaker trips, close-all. **Capture the real premium and real
   bid/ask** of the contracts these strategies actually select, plus the real short-option margin
   number — the Phase 0 open items.
3. **Live minimum size** — smallest tradeable size, tight daily breaker, until the real-fill gate
   has meaningful data.
4. **Live $100** — only after stage 3 evidence passes the gates, and only with my explicit approval.

Written evidence required before each advance.

---

## Constraints
- Branch off; never commit to `main`. Never push or deploy without asking.
- **Never commit, echo, or log API keys.** `DELTA_API_KEY`/`DELTA_API_SECRET` come from the
  environment only; keep them out of diagnostics output.
- **Do not arm live trading, and do not set `LIVE_ENGINE_AUTO_ENABLE=true`.** Arming is mine alone.
- Do not weaken, bypass, or temporarily disable any risk control to make the module work. If a
  control blocks you, report it — that is the control working.
- Go: `cd engine && go build ./... && go test ./...`. Use real test DBs — Hard Rule #2 forbids DB
  mocking in engine tests.
- The trading loop runs on the **Go engine (AWS Lightsail)**, never a Vercel cron — Hard Rule #1
  caps Vercel crons at 2 total and excess silently breaks webhooks. Never reference Render.
- Read `node_modules/next/dist/docs/` before writing Next.js code — per `client/AGENTS.md` this
  version has breaking changes vs. your training data.
- Start from `.ai-context/README_FOR_AI.md` and Graphify scoped queries
  (`python scripts/graphify_workflow.py query --scope engine-internal "..."`) rather than
  scanning the repo.
- No new UI dependencies without asking.

## Required tests
Idempotency under retry; restart mid-order; broker reject handling; kill-switch interception;
daily-breaker trip; each auto-disarm trigger; reconciliation mismatch detection; reconciliation
failing loudly on an empty/unparsed broker response; sizing returning zero rather than a forced
contract when the budget cannot afford one; an order being unable to reach Delta without passing
the risk gate; close-all under a wedged strategy loop. Client: typecheck plus Playwright specs
under `client/e2e/` for arm/disarm, confirmation modal, close-all, and real-money differentiation.

## Definition of done
Phase A and B complete with no stub adapter and no fabricated signature left in the tree; sizing
provably cannot exceed its own risk budget; reconciliation can fail; the $100 ceiling is enforced
in the engine and unreachable from the UI; every order passes the risk gate and OMS with the kill
switch wired; auto-disarm and close-all proven under test; the roster shows a per-strategy gate
reason; round-trip cost as a percentage of premium is visible per contract; paper and real money
are visually unmistakable; engine and client suites green; stages 1–3 evidence documented before
any live capital.

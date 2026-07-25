# Live Engine — Phase 0 Feasibility Report

**Status:** Awaiting approval. No implementation code written.
**Author:** engine investigation, grounded in the current repo + Delta API surface already in code.
**BTC spot used for arithmetic:** $64,217 (Coinbase, live at time of writing).
**Bottom line up front:** Buildable, but **only as a long-premium (option-buying) module** on **real Delta BTC options**. The naked option-selling roster is **ineligible for live capital at $100** and must be excluded (or deferred behind proven defined-risk spreads). Recommendation detail below.

---

## 0.1 — The instrument mismatch

### What the code actually is today
- `engine/internal/livemirror/mirror.go` — package doc: *"the LIVE ENGINE … clones every trade … into real orders on Delta Exchange BTC **perpetual futures**."* It resolves products via `FindPerpProduct`, sizes in perp contracts (`0.001 BTC`), and is what `LIVE_ENGINE_*` env vars drive. **This is a perp mirror, not an options router.**
- The Options Selling / Buying desks price against a **synthetic Black-Scholes chain** — `engine/internal/options/pricer.go::PriceOption` + a modeled spread in `chain.go` (`SpreadBase + SpreadSlope·|logMoneyness|`). **No real quote touches these numbers.**
- **However** — real Delta options plumbing already exists and is more complete than the spec assumes:
  - `engine/internal/delta/client.go` → `FindOptionProduct(strike, expiry, "call"/"put")` hits the real endpoint `GET /v2/products?contract_types=put_options,call_options`; plus `PlaceOrder`, `CancelOrder`, `GetPositions` (`/v2/positions/margined`, returns real `mark_price` + `margin`), `GetWallet`, `GetOpenOrders`.
  - `engine/internal/delta/live_bridge.go` → a `Bridge` with **buying mode** ("BUY calls/puts on signals, ~$0.50/lot") and **selling mode** ("SELL options, requires large margin"), `OnOpen`/`OnClose`, reduce-only closes, kill-check hook, and a position monitor.
  - `engine/internal/marketdata/delta.go` → `GET /v2/tickers/{symbol}` returning real `mark_price` — the **real-quote source** that replaces the synthetic chain.

### The three paths

**(A) Real Delta BTC options — RECOMMENDED (buying only, initially).**
- Available: confirmed in-code. Delta lists `call_options` / `put_options` BTC products; the client can find them, price them off `/v2/tickers` mark price, place orders, read positions and margin, and close reduce-only.
- What replaces the synthetic chain: real product lookup (`FindOptionProduct`) + real mark/bid-ask from `/v2/tickers` and `/v2/l2orderbook` (the latter not yet wired — a known Phase-2 task).
- What happens to strategy behaviour when synthetic → real: **this is the biggest unknown and must be treated as one.** The desks favour cheap, short-dated, far-OTM contracts (paper premiums $0.20–$1.00). On the real book those are exactly where bid/ask spread is widest relative to premium — a modeled spread of a few bp becomes a real spread that can be 20–100% of a $0.30 premium. **Expect much of the synthetic edge to disappear under real fills.** This is not a reason to abandon (A); it is the reason the spec's real-fill gate exists and must be enforced.

**(B) Perp delta-proxy — NOT RECOMMENDED.**
- A short put is **not** a long perp. Selling a put is short-vol, positive-theta, capped-upside/large-downside near the strike; a long perp is linear delta-one with no theta and symmetric P&L. Replacing one with the other **changes the traded payoff entirely** and throws away the whole reason these are option strategies.
- Divergence is not marginal: the option desks' P&L is dominated by theta decay and the premium/strike relationship (see the selling desk's TP/PROFIT_LOCK exits). A perp has none of that. You would be live-trading a different strategy than the one that was validated.
- Also fails on its own terms at this size — see 0.2.

**(C) Not viable —** applies specifically to the **selling** roster; see 0.3.

### 0.1 recommendation
Take **(A), long-premium only.** The **Options Buying** desk becomes the live candidate source. The synthetic chain is replaced by real `FindOptionProduct` + `/v2/tickers` mark price for sizing/marking, and real fills for truth. Reject (B) outright.

---

## 0.2 — Capital feasibility at $100

Delta BTC contract (perp **and** options) = **0.001 BTC**. At spot $64,217, that is **$64.22 notional per contract**.

### Perp path (if it were used — it should not be)
| Quantity | Value |
|---|---|
| Notional / contract | $64.22 = **64.2% of the $100 account** |
| Initial margin / contract @ `LIVE_ENGINE_LEVERAGE=10` | $6.42 |
| **Liquidation distance** | ≈ **1/leverage ≈ 10% of spot**, minus maintenance-margin buffer → realistically **~8–9%** |

BTC routinely moves 3–5% intraday and 8–10% on a bad day. **A single perp contract at 10× is liquidatable by an ordinary daily move.** One contract is already 64% of the account notional, so there is no room to diversify or scale down. Perp at $100 is fragile-to-unviable. (Confirms 0.1's rejection of (B).)

### Options BUY path — feasible
`engine/internal/delta/bridge_buy_sizing.go::TieredBuySizing` is already tuned for small wallets. For a **$100 wallet**: `riskPct = 0.10`, `maxContracts = 3`, `minWallet = $5`.
- Budget per signal = $100 × 10% = **$10**.
- Real premium for the desks' short-dated OTM contracts ≈ **$0.20–$2.00/contract** (0.001 BTC × option price). So $10 buys the 1–3 contract cap comfortably.
- **Max loss per position = premium paid = bounded and tiny** ($0.20–$6 across 1–3 contracts). This is the defining safety property of buying.
- Fees: Delta options fee ≈ 0.03% of notional, capped at 10% of premium → ≈ **$0.02/contract/side**, round-trip ≈ $0.04. In absolute dollars vs a $100 account: **immaterial.** As a % of a $0.30 premium: ~13% round-trip — **material to edge, not to capital.**

### Options SELL path — margin fits, risk does not
- Delta initial margin for a short 0.001 BTC option ≈ **$10–16/contract** (roughly premium + ~15–25% of the $64 notional; **must be confirmed against the live `/v2/positions/margined` response in Phase 2**, not assumed). So 1–3 short contracts *fit* inside $100 on margin alone.
- But margin fitting is the wrong test. See 0.3 — the tail does not fit.

### Trade frequency & fee drag
The live roster caps at 13 active strategies (per desk) and the desks fire on the order of tens of trades/hour combined. At real per-trade fees of ~$0.04 the **absolute** fee drag on $100 is trivial. The real problem is **relative**: on $0.30 premiums, fees + real spread can exceed the modeled edge. This is a strategy-selection problem (the real-fill gate), not a capital problem.

### 0.2 recommendation
**$100 is feasible for the buying path only.** Size via the existing tiered wallet logic (1–3 contracts, premium-only risk). It is **not** feasible for naked selling at any sane per-trade risk budget, and perp is fragile. The binding constraint at $100 is not margin — it is (a) the selling tail (0.3) and (b) whether any strategy's synthetic edge survives real spreads (the real-fill gate).

---

## 0.3 — Undefined risk on the short leg

The Options Selling roster is **naked short premium**: 73–88% win rates (visible on the live desk) with **unbounded loss on a short call** and large loss on a short put. High win rate + unbounded tail is the textbook account-killer, and **this project already codified the rule: qualify on tail risk, never on win rate** (the NIFTY selling pre-live desk work; memory `project_nifty_selling_prelive_prompt` / `project_options_selling_desk`).

Concretely, at $100 with 0.001 BTC contracts:
- A short call in a +20% BTC move (a real, recurring event) loses ≈ 0.20 × $64 ≈ **$12.8/contract of intrinsic**, plus vol expansion, plus a margin-call/liquidation cascade. Three contracts ≈ **−$40+ on a $100 account from one move** — and Delta liquidates before you choose to exit.
- The 79–88% win rate means this looks excellent right up until the one move that removes the account. The paper desk cannot show this because synthetic premiums never gap.

### The defined-risk filter
Real-money short legs must be **defined-risk only** — expressed as a spread (sell the near strike, buy a further-OTM strike to cap the loss), or hard-stopped with a verified-reachable stop and pre-checked margin headroom. Applying that filter to the two desks:

| Desk / leg type | Max loss | Live-eligible at $100? |
|---|---|---|
| **Options Buying** — long call / long put | Premium paid (bounded) | **Yes** — inherently defined-risk |
| **Options Selling** — naked short call | Unbounded | **No** |
| **Options Selling** — naked short put | Strike − premium (large) | **No** |
| Options Selling **as a vertical spread** | Strike width × 0.001 BTC (capped) | **Conditional** — see below |

- A spread version *is* expressible on Delta (adjacent strikes exist), and would cap loss at `(strike width) × 0.001 BTC`. But the desks' credits are thin ($0.20–$1.00); after paying the long leg **and** crossing two real bid/ask spreads, the net credit likely collapses or goes negative. **My expectation: most selling strategies fail the fee/spread gate once made defined-risk at this premium scale.** That must be *measured* on testnet, not assumed — but it should not be on the day-one roster.

### Strategies that survive the 0.3 filter (day-one live candidate set)
**Long-premium (Options Buying) strategies only.** From the current live desk, the top-ranked, gate-relevant names are the natural candidates (illustrative, not a promotion — each must still pass 0.2's gates + the real-fill gate):
`Swing_PutBuy_OverextensionFadeUp_600m`, `Intraday_PutBuy_RSIOverboughtExtreme_150m`, `Intraday_PutBuy_SharpReversalDown_150m`, `Swing_CallBuy_OverextensionFadeDown_600m`, `Intraday_CallBuy_CapitulationRecovery_180m`.
**Every naked short in the Selling roster is filtered out.**

---

## Consolidated recommendation

1. **Build the Live Engine as a long-premium (option-buying) module on real Delta BTC options — path (A).** The client + bridge plumbing already exists; extend it, do not fork.
2. **Exclude the entire naked-selling roster from live capital.** Revisit only if/when defined-risk spreads are proven to keep positive net edge under real fills — a later, separate decision.
3. **Treat all desk P&L as synthetic and unproven for real money.** Nothing trades live until it earns a real-fill (testnet → min-size) track record meeting the same go-live gates (`futuresGoLiveGates.ts`: ≥200 trades / ≥30d full-live, expectancy > 0, PF ≥ 1.1, fees ≤ 0.5%). Synthetic performance does not satisfy the real-money gate.
4. **$100 ceiling, disarmed-by-default, typed-confirmation arm, auto-disarm, close-all, kill-switch-in-path, engine-enforced cap** — all as specified. No control weakened.
5. **Order path:** signal → `POST /api/execution/request (venue=delta)` → risk gate → OMS v3 → Delta options adapter (extend the existing `live_bridge` buying path) → ledger → reconciliation. No frontend/Next.js broker calls; the retired direct routes (`/api/delta/*` → `blockedDirectExecutionRoute`) stay retired.

### Open items to resolve in Phase 2 (flagged, not assumed here)
- Real Delta **short-option initial-margin** number, read from `/v2/positions/margined` on testnet (my $10–16 estimate is not authoritative).
- Real **bid/ask spread** on the short-dated OTM contracts the desks favour → the actual edge-survival test.
- `/v2/l2orderbook` wiring for real fill modelling (currently only mark price via `/v2/tickers` is wired).

---

**No code written. No branch created. Nothing armed. Awaiting your go-ahead on the recommendation above before any implementation.**

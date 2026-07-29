# Strategy-Hunt Upgrade Plan

Turn Crypto Options Selling, Crypto Options Buying and Crypto Scalp Desk into
**strategy-hunting engines**: every registered strategy runs on real Delta
Exchange market data, each funded with $1,000, and the ones that genuinely grow
that capital become candidates for real money on the Live Engine.

---

## 1. Goal, stated precisely

| | |
|---|---|
| **Universe** | Every strategy registered in the application, running concurrently |
| **Capital** | $1,000 per independent strategy account, tracked separately |
| **Data** | Real Delta Exchange market data — candles for perps, live chain for options |
| **Costs** | Real Delta fees, real spread, real strikes. No synthetic pricing |
| **Output** | A ranked, statistically defensible shortlist promoted to the Live Engine |

---

## 2. Current state (measured, not assumed)

| Module | Strategies | Data source today | Pricing today |
|---|---|---|---|
| Crypto Scalp Desk | 100 × 8 symbols = **800 streams** | **Binance** 1m klines | Real perp candles |
| Crypto Options Buying | ~50 | Engine tick loop | **Synthetic Black-Scholes chain** |
| Crypto Options Selling | ~50 | Engine tick loop | **Synthetic Black-Scholes chain** |
| Live Engine | 5 (buying only) | Delta | Real Delta, real money |

Verified available on Delta India (2026-07-29):

- **220 live perpetuals** — all 8 scalp symbols present (BTCUSD, ETHUSD, SOLUSD,
  BNBUSD, XRPUSD, DOGEUSD, ADAUSD, AVAXUSD)
- **`/v2/history/candles`** works — 1m resolution confirmed, 119 bars for a 2h window
- **456 live BTC option contracts** across **7 expiries**

So the plan is feasible. Two things are missing from the Go Delta client and
must be built: **candle fetch** and **option-chain listing**.

---

## 3. The one thing that must change about the qualification rule

The request says: *whichever strategy grows the capital faster and more, qualify
it for live trading*. **Ranking by growth alone will promote luck, not edge**,
and this application already has the evidence.

The Scalp Desk's own pre-registered gate says it in the code:

> Under the null hypothesis (no edge), with 800 streams a few days of trading is
> EXPECTED to produce dozens of profitable-looking leaders by variance alone; the
> gate exists so luck cannot be promoted. Picking early leaderboard toppers
> without the gate = funding coin flips.

Supporting evidence already gathered in this codebase:

- All 100 scalp strategies failed offline qualification on all 8 symbols — **0/400**
- Two earlier fee-honest sweeps lost on **17/17** and **66/66**
- The 26 "qualified" BTC strategies returned **3.31% over 9.2 months** at best,
  which is ~3.7 bps of edge per trade against ~10 bps of round-trip fees

With ~900 concurrent strategies, roughly half will be profitable by chance
alone, and the top handful will look outstanding. Growth rate is a fine
**screen**. It cannot be the **gate**.

**Resolution used throughout this plan:** growth ranks the leaderboard; a
pre-registered, unchangeable gate decides promotion. Both are shown in the UI,
and only gate survivors reach the Live Engine.

---

## 4. Phase 0 — Shared Delta market-data layer *(blocking prerequisite)*

Nothing else works until this exists. ~900 strategies must never poll Delta
individually; that is an instant rate-limit ban.

**Build `engine/internal/marketdata/deltafeed`:**

- One poller per `(symbol, resolution)` pair, not per strategy
- In-memory ring buffer of closed bars per pair; strategies read, never fetch
- Fan-out via subscription, so 800 streams share 8 pollers
- Backfill on boot (Delta caps ~1800 bars/request; chunk and de-duplicate)
- **Closed bars only** — drop the in-progress candle, or every strategy
  look-aheads on a bar that has not finished
- Staleness detector: if a pair stops updating, mark its strategies degraded
  rather than silently trading on a frozen book

**Extend `engine/internal/delta/client.go`:**

```go
GetCandles(ctx, symbol, resolution string, from, to time.Time) ([]Candle, error)
ListOptionChain(ctx, underlying string) ([]OptionContractInfo, error)  // 456 contracts
GetTickers(ctx, symbols []string) (map[string]Ticker, error)           // batch marks
```

**Option-chain cache:** one snapshot per cycle (strikes, expiries, marks, bid/ask),
shared by all ~100 option strategies. Refresh on a fixed interval, never per
strategy.

**Rate-limit budget:** 8 perp pollers + 1 chain poller + 1 ticker batch. Roughly
10 requests per cycle regardless of strategy count. That is the whole point of
this phase.

---

## 5. Phase 1 — Scalp Desk onto Delta data

Currently Binance. Moving to Delta matters because the Live Engine trades Delta:
a strategy validated on Binance prices is validated on the wrong book.

1. Swap `fetchKlines` for the shared `deltafeed` subscription
2. Keep the existing maker/post-only fill model — it already mirrors the backtest
3. Re-base each stream to a **$1,000 account** (see Phase 3)
4. **Reset all history at cutover.** Binance-era statistics are not comparable to
   Delta-era statistics; mixing them silently corrupts the leaderboard
5. Expect different results. Delta's book is thinner than Binance's on the alt
   symbols — that is real information about what you can actually trade

**Watch for:** Delta 1m volume on ADAUSD/AVAXUSD may be too thin for a maker fill
model to be realistic. If fill rates collapse versus Binance, that is a genuine
finding, not a bug — those symbols are not tradeable at size on this venue.

---

## 6. Phase 2 — Options desks onto the real Delta chain

The largest change, and the one that will most change the numbers.

**Today:** both desks price against a synthetic Black-Scholes chain with a
configurable IV — no spread, every strike available, perfect fills.

**After:** real Delta option marks, real strikes, real expiries, real spread.

Concrete consequences to expect and design for:

| Synthetic today | Real Delta |
|---|---|
| Any strike exists | Only 456 listed contracts, 7 expiries |
| Fills at theoretical mid | Pays the spread both ways |
| Smooth IV surface | Real, gappy, sometimes stale marks |
| No fee cap effects | 10%-of-premium cap → up to ~28% round trip on cheap options |

Implementation:

1. Strategy asks for a strike/expiry → **snap to the nearest listed contract**;
   if nothing is within tolerance, **skip the signal and record it as skipped**
   (a skipped signal is data — it tells you the strategy needs strikes the venue
   does not list)
2. Price entries and exits from the cached chain, not a model
3. Charge fees through the existing `internal/delta/fees.go` — the same code the
   Live Engine uses, so paper and live agree
4. Apply the existing **entry-economics guard**: decline options too cheap to
   round-trip profitably. This already exists and already found the ~28% problem
5. Keep the synthetic chain behind a flag for A/B comparison — the gap between
   synthetic and real *is the measurement* of how much the old numbers lied

**Selling desk, explicitly:** real-chain selling means real margin and, for naked
shorts, unbounded loss. Paper trading it is fine. **Promotion to live is not**,
unless the position is a defined-risk spread or margin-capped. This is called out
again in Phase 5.

---

## 7. Phase 3 — $1,000 per strategy accounting

**Sizing decision:** $1,000 per **independent hypothesis**, not per module.

| Module | Accounts | Paper capital |
|---|---|---|
| Scalp Desk | 100 strategies × 8 symbols = 800 | $800,000 |
| Options Buying | ~50 | $50,000 |
| Options Selling | ~50 | $50,000 |
| **Total** | **~900** | **~$900,000** |

Each account needs, persisted independently:

- Starting balance $1,000, current balance, peak balance
- Realised and unrealised P&L, **gross and net of fees separately**
- Trade count, win count, max drawdown, exposure %
- Fee drag as a share of capital deployed
- First and last trade timestamps (needed by the gate's time requirement)

Reuse the reset/clear plumbing already shipped: `ResetAccountWith(capital)` on
both options engines and `/scalp/reset` accept a starting capital, so re-basing
all ~900 accounts to $1,000 is already a solved problem.

**Growth metric for the leaderboard:** CAGR-equivalent on the $1,000 base, shown
next to trade count and max drawdown so a 40% gain on 3 trades cannot masquerade
as an edge.

---

## 8. Phase 4 — Qualification (the part that protects real money)

Two separate things, never conflated:

### 8.1 The leaderboard (a screen)
Ranks all ~900 accounts by capital growth. Sortable, filterable, exactly what was
asked for. Its job is to draw attention, not to authorise capital.

### 8.2 The gate (the authority)

Pre-registered before the hunt starts and **not changed afterwards** — changing a
gate after seeing results is how you fit the gate to the noise:

| Criterion | Threshold | Why |
|---|---|---|
| Trades | ≥ 200 | Below this, win rate is not measurable |
| Duration | ≥ 30 days live-forward | Spans more than one micro-regime |
| Profit factor | ≥ 1.2 **net of fees** | Gross PF is the trap this codebase already fell into |
| Max drawdown | ≤ 25% | Survivability, not just return |
| Both halves | net-positive in each half of its window | Kills strategies carried by one lucky streak |
| Fee drag | ≤ 30% of gross profit | An edge that exists only pre-fee is not an edge |
| Expectancy | > 0 net | Per-trade edge after costs |

### 8.3 Multiple-comparison correction

With ~900 simultaneous tests, the conventional thresholds are not enough. Add:

- **Deflated Sharpe** or an equivalent penalty for the number of trials
- A **hold-out window**: qualification computed on the first N days, then
  confirmed on a later untouched window. A strategy that passes the first and
  fails the second was curve-fit by the search itself
- Report **how many strategies would be expected to pass by chance** alongside
  how many actually passed. If 900 strategies run and 12 pass a gate that noise
  alone would clear 9 times, that is not a shortlist — that is noise

This is the difference between a strategy hunt and a lottery with 900 tickets.

---

## 9. Phase 5 — Promotion to the Live Engine

1. Gate survivors appear as **candidates**, not as live strategies
2. **Re-validate at live size.** The Live Engine has a **$100 ceiling** and buys
   exactly 1 contract per trade. A strategy proven on a $1,000 account may be
   untradeable at $100: the execution floor and contract rounding do not scale
   down. Re-run the candidate at $100-equivalent sizing before promotion —
   otherwise you promote a strategy that cannot place its own orders
3. Promotion is a **human action**, via the existing allow-list API + typed
   confirmation. No automatic path from leaderboard to real money
4. Turn on `LIVE_ENGINE_ENFORCE_GATE` once real-fill records exist, so the
   go-live gate blocks rather than merely reports
5. **Selling strategies are not promotable as naked shorts.** Either they trade
   as defined-risk spreads or they stay in paper. Unbounded downside on a $100
   account is not a strategy, it is a countdown

---

## 10. What this does not fix, and must be fixed alongside

From the live-engine forensics earlier in this project — **77% of live signals
never reached the exchange**:

| Cause | Count | Status |
|---|---|---|
| MongoDB Atlas over quota | 22 | Fixed (purge + drop + daily cap cron) |
| Risk gate: size below execution floor | 16 | **Open — needs your decision** |
| Delta: invalid_contract | 5 | Open — Phase 2's strike-snapping fixes this |

Running 900 strategies changes nothing if the winners' orders still cannot reach
the market. **The execution-floor decision gates the value of this entire plan.**

Also unchanged: **capital**. A validated 4%-annualised edge on a $100 live
ceiling earns about $4/year. The hunt can tell you *what* to trade; it cannot
make $100 material.

---

## 11. Sequencing

| Phase | Work | Depends on |
|---|---|---|
| 0 | Delta market-data layer + client extensions | — |
| 1 | Scalp Desk → Delta data, reset history | 0 |
| 2 | Options desks → real chain, strike snapping | 0 |
| 3 | $1,000 per-strategy accounting + persistence | 1, 2 |
| 4 | Leaderboard + pre-registered gate + trials correction | 3 |
| 5 | Promotion pipeline + live-size re-validation | 4 |

Phase 0 is genuinely blocking. Phases 1 and 2 are independent of each other and
can run in parallel. **Nothing promotes to real money before Phase 5**, and the
30-day gate window means the earliest honest promotion is 30 days after Phase 3
lands.

---

## 12. Open decisions for the owner

1. **Execution floor** — teach the risk gate about fixed-contract instruments, or
   leave 16 orders per cycle blocked? This gates everything downstream.
2. **Scalp capital basis** — $1,000 per *stream* (800 accounts, as planned above)
   or per *strategy* (100 accounts, split across symbols)?
3. **Selling promotion** — defined-risk spreads only, or keep the selling desk
   permanently paper?
4. **Gate strictness** — the 30-day/200-trade window is deliberately slow. Shorter
   means faster answers and materially more false positives.
5. **Atlas tier** — ~900 accounts persisting state will not fit comfortably in the
   512 MB free tier that just blocked writes and took the engine down.

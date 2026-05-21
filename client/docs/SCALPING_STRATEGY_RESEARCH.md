# Scalping Strategy Research — BTC Futures Paper Desk

**Status:** Research blueprint. **Not financial advice.** Every strategy here is a paper-trade hypothesis; promotion to the WINNERS_ONLY production roster requires positive expectancy verified via the existing ranking pipeline ([PAPER_DESK_RUNBOOK.md](./PAPER_DESK_RUNBOOK.md) §3 + MongoDB `paperTrades` aggregation). No language in this document should be read as a claim of guaranteed profit.

> **Hard disclaimer:** Crypto perpetual scalping has structurally negative expectancy under realistic taker fees unless the strategy clears the fee + slip hurdle on average. This document explicitly catalogs that hurdle and rejects specs that cannot meet it.

---

## Executive Summary (for the product owner)

1. The desk runs 20 templateKeys today (IDs 200–399 research pool, 500–503 premium). There is no written rationale for the *next* batch — this doc fills that gap.
2. We map three scalping families (chart pattern, indicator, market sentiment) to **what our stack can actually compute** — 1m OHLCV + mark + funding rate. No L2 / tape / open interest is fetched, so order-flow strategies must use volume proxies.
3. 15 strategy specifications are presented, 5 per family, each with computable rules, regimes, fee-clearance math, and references.
4. A ranked Top-5 is recommended for first implementation as new templateKeys, sized to 10 LONG/SHORT `FuturesStratDef` rows in ID block **510–519** (504–509 left as a buffer above the premium tier).
5. Top-5 are picked for distinct mechanics (no overlap with existing 20 templates) and low pairwise correlation.
6. Fee math: at 0.2% round-trip taker (`TAKER_FEE_PCT = 0.001` per leg, see [futuresReplayEngine.ts:51](../src/lib/futuresReplayEngine.ts#L51)) plus configurable slip, a break-even TP=SL trade requires ≥ 0.4% gross move. Every spec must show how it clears this.
7. CORE winners and the WINNERS_ONLY production gate are untouched. New 510–519 ride the research pool until promoted by [applyWinnersOnlyGate](../src/lib/futuresDeskPolicy.ts#L888).
8. Funding-fade is the only true sentiment input we can ship today — the funding rate is already fetched ([useBTCFuturesScalperEngine.ts:163](../src/hooks/useBTCFuturesScalperEngine.ts#L163)) but never gated on. Implementing it requires extending `FuturesSignalInputs` with a `fundingRate` field.
9. Engineering effort is documented in [SCALPING_TOP5_IMPLEMENTATION.md](./SCALPING_TOP5_IMPLEMENTATION.md). 5 templateKeys × {meta + scorer + confirm + 2 defs + Vitest fixture}.
10. Honest expectation: replay sumNet on the first 1500-bar sample is likely flat-to-negative. Research pool exists precisely to filter; we measure, we do not promise.

---

## 1. Stack Constraints (gates every selection)

| Constraint | Source | Implication for strategy design |
|---|---|---|
| 1m OHLCV only; 5m/15m built via aggregation | [buildHtfFields](../src/lib/futuresSignals.ts#L431), [useBTCFuturesScalperEngine.ts:156](../src/hooks/useBTCFuturesScalperEngine.ts#L156) | All entry rules must be derivable from O/H/L/C/V + lagged windows |
| Funding rate + next_funding_time available, **not yet in signal inputs** | [useBTCFuturesScalperEngine.ts:163](../src/hooks/useBTCFuturesScalperEngine.ts#L163) | `FUNDING_FADE` requires extending [FuturesSignalInputs](../src/lib/futuresSignals.ts) and threading the field through `buildSignalInputs` |
| No L2 / tape / open interest / bid-ask spread | confirmed in [futuresKlinesFetch.ts](../src/lib/futuresKlinesFetch.ts) | Order-flow strategies must use volume-z-score and close-vs-range proxies |
| 0.2% RT taker fee | [futuresReplayEngine.ts:51](../src/lib/futuresReplayEngine.ts#L51) (`TAKER_FEE_PCT = 0.001` per leg) + [paperRoundTripTakerFees](../src/lib/futuresPaperMath.ts#L112) | Break-even on a TP=SL trade needs ≥ 0.4% gross move. TP/SL ≥ 2 stretches that to ≈0.27% per-side average |
| Slippage configurable, default 0 bps | [DESK_SLIPPAGE_BPS_DEFAULT](../src/lib/futuresDeskPolicy.ts#L262) | Spec uses 5 bps for fee math (conservative) |
| Min-expected-move gate | [paperMinExpectedMoveVsFees](../src/lib/futuresPaperMath.ts#L143) (K × roundTripFees) | Specs declare K and tie TP target to ATR$ scale |
| Regimes classified from 15m ADX+ATR | [classifyRegimeTagFrom15mBars](../src/lib/futuresSignals.ts#L385) returns `chop \| trendLow \| trendHigh` | Every spec declares its `regimes` array |
| Production gate = WINNERS_ONLY | [applyWinnersOnlyGate](../src/lib/futuresDeskPolicy.ts#L888) | New 510–519 are research-pool only until promoted |

---

## 2. Family Taxonomy

| Family | Subtypes | Typical TF | Data required | Fits our stack? |
|---|---|---|---|---|
| **Chart pattern** | flag/pennant, wedge, double top/bottom, head & shoulders, liquidity sweep / Wyckoff spring, FVG fill, opening range breakout | 1m–5m | OHLCV + swing high/low detection | **Y** |
| **Indicator** | trend (EMA, ADX), momentum (RSI, MACD, stoch), volatility (BB, ATR, Keltner), volume (VWAP, OBV) | 1m + 15m bias | OHLCV | **Y** |
| **Market sentiment** | funding rate fade, basis (perp vs index), session momentum, volume climax (proxy), fear/greed external | slow–fast | ticker funding (we have); external APIs (we do not) | **Partial** — funding-fade and volume-climax-as-proxy are shippable; external Fear/Greed API deferred |
| **Order flow** | delta, imbalance, absorption, tape reading | tick / L2 | footprint / L2 — **we have neither** | **Partial / proxy only** — must reconstruct from candle volume + close location |

References for family fundamentals: [Liquidity Sweep Reversals](https://dailypriceaction.com/blog/liquidity-sweep-reversals/) (marketing), [BB Squeeze Optimization across crypto regimes](https://pyquantlab.medium.com/optimizing-the-bollinger-band-keltner-channel-squeeze-strategy-volatility-breakout-trading-in-70b49101cb30) (tested), [Funding Rate Mechanism (Zhang, SSRN)](https://papers.ssrn.com/sol3/papers.cfm?abstract_id=6185958) (academic).

---

## 3. What makes scalping profitable — fee-aware math

Scalping edge requires positive net expectancy after **fees + slip + funding accrual**. Three honest constraints:

### 3.1 The fee floor

Per round-trip on a notional `N` at price `P`:
- Taker fees: `2 × 0.001 × N = 0.002 N` (0.2% of notional)
- Slippage (5 bps default in replay): `2 × 0.0005 × N = 0.001 N` (0.1%)
- Total cost floor on a TP=SL trade: **0.3% gross price move just to break even.** If TP and SL are symmetric, that means *every winner must beat 0.3%* and the trade-off is asymmetric: a 50/50 strategy hitting exactly 0.3% TP / 0.3% SL nets **zero** before edge.

### 3.2 Why TP/SL ≥ 2 is non-negotiable

With TP=2×SL and win rate `p`, expected per-trade move is `p·TP − (1−p)·SL = SL·(2p − (1−p)) = SL·(3p − 1)`.
For E[move] to exceed the 0.3% fee floor, with SL = 0.3%, we need `3p − 1 > 1`, i.e. `p > 0.67`. **No retail intraday strategy reliably wins 67%.**
Push SL to 0.4% and TP to 1.0% (TP/SL = 2.5), and we need `p·0.7 > 0.3`, i.e. `p > 0.43`. That's defensible. This is the design point every spec aims at.

### 3.3 Why most published "scalping strategies" fail on small notional

- Fee drag dominates: see [CoinTracker on crypto scalping fees](https://www.cointracker.io/blog/crypto-scalping-strategy) — 0.2% RT eats 20 bps of every trade regardless of size.
- Chop kills mean-reversion edge: see [ROOT_CAUSE.md §1](./ROOT_CAUSE.md) — PROFIT_LOCK plus template-family churn paid 3× fees on the same signal.
- Overfit windows: most public backtests publish only one regime. The [TORB academic study](https://www.researchgate.net/publication/331076454_Assessing_the_Profitability_of_Timely_Opening_Range_Breakout_on_Index_Futures_Markets) shows ORB clears 8% p.a. *only when applied across multiple index regimes with p < 3%*.

### 3.4 What we keep from existing gates

- `paperMinExpectedMoveVsFees(K)` — defaults K=1; new specs use K=1.1 for safety margin.
- HTF bias filter (5m EMA stack via `htf5_fast`/`htf5_slow` or `htf5_trend`) cuts counter-trend losses.
- `confluenceMin` set to 5–6 (raised from the generated-pool default of 4) for the new templates, matching premium-tier discipline.
- Cross-link: [ROOT_CAUSE.md §3](./ROOT_CAUSE.md) explains why PROFIT_LOCK now subtracts fees before locking.

---

## 4. 15 Strategy Specifications

Each spec lists computable rules, regime, score function pseudocode, false-signal modes, and the fee-clearance argument. Strategies marked **TOP-5** in §5 are the ones recommended for implementation.

### 4A. Chart Pattern Family

#### 4A.1 — Double Bottom / Double Top + Neckline Break **[TOP-5 → templateKey `DOUBLE_REV`]**

```yaml
id: 510 (LONG) / 511 (SHORT)
name: DoubleReversal_NeckBreak
family: chart_pattern
subtype: double_bottom_top_neckline
direction: both
htf_bias: 5m trend either flat or counter to local extreme (no fighting a 5m impulse)
entry_rules:
  - Detect two local lows (or highs) within last 30 bars, separation ≥ 5 bars
  - Second extreme within 0.15% of first (touches the floor/ceiling)
  - Neckline = intervening swing (max between the two lows for double-bottom)
  - Trigger: close breaks neckline by 0.05% with volume ≥ 1.2× 20-bar avg
  - HTF gate: htf5_trend ≥ 0 for LONG, ≤ 0 for SHORT (no fighting macro)
exit_rules:
  slPct: 0.36  # widened to clear fees on chop
  tpPct: 0.92  # measured move from neckline to second extreme; TP/SL ≈ 2.55
  holdMinutes: 28
regimes: [chop, trendLow]   # works best when 15m regime not screaming trend
score_function: |
  let s = 0
  if doubleBottomDetected(closes, 30, 0.0015): s += 28; reason += "DB_pattern"
  if close > neckline * 1.0005: s += 22; reason += "neck_break"
  if volRatio > 1.2: s += 14
  if rsi14 between (35, 55) for LONG (45, 65) for SHORT: s += 10
  if htf5_trend aligned: s += 10
  return s   # passes if ≥ confluenceMin=6 weighted
false_signals:
  - chop with no follow-through (filter via volRatio gate + holdMinutes hard cap)
  - second low fails (HTF strongly against the bias)
fee_note: TP 0.92% beats 0.4% fee+slip with margin ~0.5%; needs price > neckline by ≥0.05% pre-entry
references: |
  https://www.quantifiedstrategies.com/double-bottom-strategy/ (tested)
implementation_effort: M
```

#### 4A.2 — Liquidity Sweep + Reclaim (Wyckoff Spring/UTAD scalp)

```yaml
id_proposed: deferred (overlaps existing WYCKOFF_SPRING — see §6)
name: LiquiditySweep_Reclaim
family: chart_pattern
subtype: spring_or_upthrust
direction: both
htf_bias: required — 5m must be flat/counter to the sweep side
entry_rules:
  - Prior bar pierces last 20-bar low (or high) by ≥ 0.07%
  - Current bar closes BACK INSIDE the prior 20-bar range (rejection wick)
  - volRatio on the sweep bar > 1.5
  - rsi14 < 35 (LONG) or > 65 (SHORT)
exit_rules: slPct 0.30 / tpPct 0.85 / hold 24
regimes: [chop, trendLow]
score_function: similar to existing WYCKOFF_SPRING but adds wick-length gate
false_signals: news-driven sweeps with no reclaim; filter via close-inside-range
fee_note: TP 0.85% clears 0.4% floor by 0.45%
references: |
  https://dailypriceaction.com/blog/liquidity-sweep-reversals/ (marketing)
implementation_effort: M
status: SKIP for v1 — existing WYCKOFF_SPRING already covers this mechanic.
```

#### 4A.3 — Fair Value Gap Fill + Continuation

```yaml
id_proposed: deferred (overlaps existing SMART_MONEY_FVG)
name: FVG_Fill_Continuation
family: chart_pattern
subtype: fvg_fill
direction: both
htf_bias: 5m EMA stack aligned
entry_rules:
  - 3-bar FVG: |bar[t-2].high - bar[t].low| > 0.4 × ATR14 for bullish FVG
  - Price returns to fill ≥50% of gap but holds the lower bound (LONG)
  - OBV slope positive over last 6 bars
exit_rules: slPct 0.30 / tpPct 0.85 / hold 24
regimes: [trendLow, trendHigh]
score_function: extends existing SMC_FVG with explicit fill-but-hold gate
fee_note: TP 0.85% clears fees by 0.45%
references: |
  https://forextester.com/blog/fair-value-gap/ (marketing)
implementation_effort: M
status: SKIP for v1 — existing SMART_MONEY_FVG already covers fill mechanic with ATR-gap proxy.
```

#### 4A.4 — Opening Range Breakout (5m / 15m session box) **[runner-up; SHELF]**

```yaml
id_proposed: deferred (overlaps existing SESSION_RANGE_BREAK + SESSION_OPEN)
name: ORB_5m_Box
family: chart_pattern
subtype: opening_range_breakout
direction: both
entry_rules:
  - Define first-5m or first-15m bar of UTC 08:00 (London) or 13:00 (NY)
  - Break box high (LONG) or low (SHORT) by ≥ 0.05% with volRatio > 1.3
  - HTF15 trend must agree (htf15_trend ≥ 0 for LONG)
exit_rules: slPct 0.34 / tpPct 0.85 / hold 22
regimes: [trendLow, trendHigh]
false_signals: breaks during low-vol Asian hours; gated by UTC window
fee_note: TP 0.85% clears 0.4% floor by 0.45%
references: |
  https://www.researchgate.net/publication/331076454_Assessing_the_Profitability_of_Timely_Opening_Range_Breakout_on_Index_Futures_Markets (academic)
  https://ungeracademy.com/blog/bitcoin-trading-systems-intraday-breakout-strategies-on-the-cme-regulated-futures-market (tested)
implementation_effort: S
status: SKIP for v1 — covered well by SESSION_RANGE_BREAK; consider as a variant tweak.
```

#### 4A.5 — Flag / Pennant Continuation After Impulse

```yaml
id_proposed: deferred — high false-positive in chop
name: Flag_Continuation
family: chart_pattern
subtype: flag_pennant
direction: both
entry_rules:
  - Impulse leg: |close[t-N] − close[t]| ≥ 1.5 × ATR14 over N=6 bars
  - Pullback: 3–8 bars within 0.4× impulse range, no new extreme
  - Trigger: close breaks pullback in impulse direction
exit_rules: slPct 0.30 / tpPct 0.78 / hold 18
regimes: [trendLow, trendHigh]
fee_note: TP 0.78% needs > 0.38% net move; tight margin → de-prioritized
references: classic continuation lit; no strong academic source found
implementation_effort: M
status: SKIP for v1 — tight fee margin; MOMENTUM_IMPULSE / MTF_MACD_HIST already capture this thesis.
```

---

### 4B. Indicator Family

#### 4B.1 — Bollinger Band Squeeze Breakout (BBW percentile) **[TOP-5 → templateKey `BB_SQUEEZE_BO`]**

```yaml
id: 512 (LONG) / 513 (SHORT)
name: BBSqueeze_Breakout
family: indicator
subtype: volatility_compression_break
direction: both
htf_bias: required — htf5_trend agrees with breakout direction
entry_rules:
  - Compute bbWidth = (bbUpper - bbLower) / mean20 on each bar; maintain trailing 120-bar window
  - Squeeze condition: current bbWidth ≤ 20th percentile of last 120 bars
  - Trigger: close breaks bbUpper (LONG) or bbLower (SHORT) by ≥ 0.05%, volRatio > 1.3
  - HTF gate: htf5_fast cross > htf5_slow (LONG)
exit_rules: slPct 0.34 / tpPct 0.92 / hold 26 (TP/SL ≈ 2.7)
regimes: [trendLow, trendHigh]   # squeeze→trend transition; NOT chop
score_function: |
  let s = 0
  if bbWidthPctile(120) <= 0.20: s += 24; reason += "squeeze"
  if breakout_direction confirmed: s += 22
  if volRatio > 1.3: s += 14
  if htf5_trend aligned: s += 12
  if adxProxy > 22: s += 10
  return s
false_signals:
  - false break: squeeze releases but reverses within 2 bars (handled by holdMinutes + SL)
  - chop regime: explicitly excluded
fee_note: TP 0.92% beats fee+slip 0.4% by 0.52% — strongest fee margin of the indicator set
references: |
  https://www.quantifiedstrategies.com/bollinger-band-squeeze-strategy/ (tested)
  https://pyquantlab.medium.com/optimizing-the-bollinger-band-keltner-channel-squeeze-strategy-volatility-breakout-trading-in-70b49101cb30 (tested, crypto regimes)
implementation_effort: S
```

#### 4B.2 — VWAP Reclaim from Below / Above **[TOP-5 → templateKey `VWAP_RECLAIM`]**

```yaml
id: 514 (LONG) / 515 (SHORT)
name: VWAP_Reclaim
family: indicator
subtype: vwap_reclaim
direction: both
htf_bias: required — 5m trend must support reclaim direction
entry_rules:
  - For LONG: price traded < session VWAP for ≥ 5 of last 10 bars, current close > VWAP by ≥ 0.06%
  - For SHORT: symmetric — price > VWAP for ≥ 5 of 10, current close < VWAP by ≥ 0.06%
  - vwapDev sign-flip with magnitude > 0.05% (uses existing field)
  - Volume on the reclaim bar > 1.2× 20-bar avg
  - rsi14 ∈ (40, 65) for LONG — avoids exhaustion
exit_rules: slPct 0.30 / tpPct 0.80 / hold 22 (TP/SL ≈ 2.67)
regimes: [chop, trendLow]   # reclaim works in range/early-trend; NOT in raging trends where VWAP gets left behind
score_function: |
  let s = 0
  if vwapDev sign-flip with mag > 0.0005 × price: s += 24; reason += "VWAP_flip"
  if volRatio > 1.2: s += 14
  if rsi14 in target band: s += 12
  if htf5_trend aligned: s += 12
  if recent dwell-below count ≥ 5: s += 10
  return s
false_signals:
  - reclaim immediately reversed (handled by SL + hold)
  - high-funding squeeze blowing past VWAP (deferred — see FUNDING_FADE for the related thesis)
fee_note: TP 0.80% beats fee+slip 0.4% by 0.40%; conservative margin — confluence gate raised to 6
references: |
  https://www.quantifiedstrategies.com/vwap-trading-strategy/ (tested)
  https://www.snappchart.app/blog/strategy-playbooks/vwap-momentum-trading-strategy (marketing)
implementation_effort: S
distinct_from: PRM_VWAP_REJECT is *rejection* off VWAP; this is *reclaim* through VWAP — opposite mechanic, no overlap.
```

#### 4B.3 — RSI(2) Extreme Mean Reversion + HTF Trend Filter

```yaml
id_proposed: deferred (overlaps existing MEANREV_RSI)
name: RSI2_Connors_HTF
family: indicator
subtype: rsi2_mean_revert
direction: both
htf_bias: required — long only above 200-bar SMA proxy; short only below
entry_rules:
  - rsi14 < 8 (relaxed Connors RSI2 proxy, since we use rsi14 not rsi2) → LONG
  - rsi14 > 92 → SHORT
  - HTF: htf5_trend > 0 for LONG, < 0 for SHORT
  - Price < mean20 × 0.998 (LONG)
exit_rules: slPct 0.32 / tpPct 0.65 / hold 18 (TP/SL ≈ 2.03)
regimes: [chop, trendLow]
fee_note: TP 0.65% beats fee+slip 0.4% by 0.25% — TIGHT; needs > 60% win rate
references: |
  https://www.quantifiedstrategies.com/rsi-2-strategy/ (tested) — Connors RSI2 typically wins 75% on equities
  https://chartschool.stockcharts.com/table-of-contents/trading-strategies-and-models/trading-strategies/rsi-2 (academic-tested)
implementation_effort: S
status: SKIP for v1 — existing MEANREV_RSI covers this with similar gates; new variant would not be distinct enough.
```

#### 4B.4 — MACD Histogram Flip + ADX Trend Filter

```yaml
id_proposed: deferred (overlaps existing MTF_MACD_HIST)
name: MACD_Flip_ADX
family: indicator
subtype: momentum_flip
direction: both
entry_rules:
  - macdHist sign-flip (prev ≤ 0, curr > 0 for LONG)
  - adxProxy > 22 (gates out chop)
  - htf5_macdHist same sign
exit_rules: slPct 0.30 / tpPct 0.78 / hold 22 (TP/SL ≈ 2.6)
fee_note: TP 0.78% beats fee+slip 0.4% by 0.38%
references: classic Appel MACD lit; QuantifiedStrategies MACD article
implementation_effort: S
status: SKIP for v1 — MTF_MACD_HIST already exists with similar mechanic.
```

#### 4B.5 — Stochastic Cross in Range (chop-regime only)

```yaml
id_proposed: deferred — low edge after fees
name: Stoch_Cross_Chop
family: indicator
subtype: oscillator_cross
direction: both
regimes: [chop]
fee_note: typical chop targets 0.4–0.5% — too tight to clear fee floor at any reasonable win rate
status: REJECT — fails fee math.
```

---

### 4C. Market Sentiment Family

#### 4C.1 — Funding Rate Fade **[TOP-5 → templateKey `FUNDING_FADE`]**

```yaml
id: 516 (LONG, fades negative funding) / 517 (SHORT, fades positive funding)
name: Funding_Fade
family: sentiment
subtype: funding_rate_mean_revert
direction: both
htf_bias: not required; the funding extreme IS the thesis
entry_rules:
  - LONG: fundingRate ≤ -0.0006 (i.e. ≤ -0.06% per 8h) AND minutes-to-next-funding < 90
  - SHORT: fundingRate ≥ +0.0006 AND minutes-to-next-funding < 90
  - Price action gate: rsi14 < 40 (LONG) or > 60 (SHORT) — confirms positioning extreme aligns with price
  - Volume gate: volRatio > 1.05 (no dead-tape entries)
  - htf5_trend not strongly opposed (≥ -1 for LONG, ≤ +1 for SHORT)
exit_rules: slPct 0.36 / tpPct 0.95 / hold 35 (TP/SL ≈ 2.64) — wide hold to capture funding-event move
regimes: [chop, trendLow]   # explicitly avoid strong-trend regimes where funding can persist
score_function: |
  let s = 0
  if abs(fundingRate) >= 0.0006: s += 26; reason += "extreme_funding"
  if minutesToNextFunding < 90: s += 14; reason += "near_funding"
  if rsi14 confluence: s += 12
  if vol confluence: s += 10
  if htf not opposed: s += 10
  return s
false_signals:
  - strong-trend funding (perp can stay negative for days during dumps); regime filter rejects
  - funding flip just before next event (gated by minutesToNextFunding)
data_dependency: |
  REQUIRES extending FuturesSignalInputs with:
    - fundingRate: number          // -1..1 raw (e.g. -0.0008 = -0.08%)
    - minutesToNextFunding: number // computed from next_funding_time
  AND threading these through buildSignalInputs + the hook's per-tick eval.
  See SCALPING_TOP5_IMPLEMENTATION.md §4 for the wiring.
fee_note: TP 0.95% beats fee+slip 0.4% by 0.55%
references: |
  https://papers.ssrn.com/sol3/papers.cfm?abstract_id=6185958 (academic) — funding rate as feedback mechanism
  https://arxiv.org/pdf/2506.08573 (academic) — funding rate design induces mean-reverting basis
  https://arxiv.org/pdf/2212.06888 (academic) — fundamentals of perpetual futures
  https://www.coinbase.com/learn/perpetual-futures/understanding-funding-rates-in-perpetual-futures (academic, exchange research)
implementation_effort: M   # data plumbing is the cost; signal logic is small
```

#### 4C.2 — Volume Climax Reversal (order-flow proxy) **[TOP-5 → templateKey `VOL_CLIMAX_REV`]**

```yaml
id: 518 (LONG, capitulation low) / 519 (SHORT, blow-off top)
name: VolumeClimax_Reversal
family: sentiment_proxy
subtype: exhaustion_reversal_proxy
direction: both
htf_bias: optional — climax is its own thesis, but extreme-counter-trend climaxes get a stricter score
entry_rules:
  - Climax bar: volume ≥ 2.0 × 20-bar avg AND bar range ≥ 1.5 × ATR14
  - Close location: close in lower-25% of bar range → LONG (sellers exhausted at low); upper-25% → SHORT
  - Follow-through gate: next bar (current) closes back inside the climax bar range
  - rsi14 < 30 (LONG) or > 70 (SHORT)
  - Cooldown: no other reversal signal in prior 6 bars (dedupe vs DOUBLE_REV)
exit_rules: slPct 0.36 / tpPct 0.95 / hold 30 (TP/SL ≈ 2.64)
regimes: [chop, trendLow, trendHigh]   # exhaustion can appear in any regime
score_function: |
  let s = 0
  if volRatio >= 2.0 AND abs(prevHigh - prevLow) >= 1.5 × atr14: s += 28; reason += "climax_bar"
  if close_in_extreme_quartile: s += 18; reason += "rejection_close"
  if rsi14 extreme: s += 14
  if HTF NOT same direction as climax: s += 10  # counter-trend exhaustion stronger
  if obvSlope opposes climax direction: s += 10  # divergence
  return s
false_signals:
  - news spike with continuation (filter via close-inside-range gate)
  - thin liquidity false climax (filter via 20-bar avg comparison; ratio threshold)
fee_note: TP 0.95% beats fee+slip 0.4% by 0.55%
references: |
  https://www.zeiierman.com/indicators/climax-volume (marketing-tested)
  PRM_VOL_DIVERGENCE template in this codebase is conceptually related but uses NO-OBV-confirmation as the thesis;
  VOL_CLIMAX_REV uses the climax bar itself as thesis.
implementation_effort: S
distinct_from: PRM_VOL_DIVERGENCE = new 20-bar extreme WITHOUT volume; VOL_CLIMAX_REV = climax bar WITH huge volume + rejection close. Inverse mechanics.
```

#### 4C.3 — Funding + Price Divergence (perp premium vs index)

```yaml
id_proposed: deferred
name: Funding_PerpPremium_Divergence
family: sentiment
entry_rules:
  - perpPremium = (markPrice - indexPrice) / indexPrice
  - Premium > +0.0015 AND price falling 3 bars → SHORT (longs paying for nothing)
  - Premium < -0.0015 AND price rising 3 bars → LONG
data_dependency: needs indexPrice threaded into signal inputs (currently fetched but unused in eval)
fee_note: target TP 0.85%
references: same SSRN / arXiv funding papers
implementation_effort: M
status: DEFER to v2 — uses available data but requires signal-input extension AND has tight correlation with FUNDING_FADE (both fire on positioning extremes); add only after FUNDING_FADE has data to validate.
```

#### 4C.4 — Session Sentiment: US-Open Momentum + Volume Expansion

```yaml
id_proposed: deferred (overlaps SESSION_OPEN, SESSION_RANGE_BREAK)
name: USOpen_Momentum
family: sentiment_proxy
entry_rules: utc hour 13:00–14:00 + momentum3 > 0.4×ATR + volRatio > 1.3
status: SKIP — SESSION_OPEN + SESSION_RANGE_BREAK already cover this; no novel mechanic.
```

#### 4C.5 — External Fear/Greed Index Fade (optional v2)

```yaml
id_proposed: deferred
name: FearGreed_Fade
family: sentiment_external
data_dependency: alternative.me / cmc Fear & Greed API; NOT currently fetched
status: REJECT for v1 — no data plumbing; defer to v2 when external sentiment integration lands.
```

---

## 5. Top-5 Shortlist for v1 (510–519 block)

Selection rationale: five **distinct** templateKeys (no overlap with existing 20), mixed regimes (3 reversal/MR, 2 breakout), uses every available data field (funding included once via the data extension), pairwise correlation kept LOW–MED.

| # | TemplateKey | IDs | Family | Why top-5 | Fee margin |
|---|---|---|---|---|---|
| 1 | `DOUBLE_REV` | 510 L / 511 S | chart pattern | New mechanic (swing-low pair + neckline break); no existing template; measured-move naturally yields TP/SL ≥ 2.5 | TP 0.92% vs fee+slip 0.40% = **0.52% margin** |
| 2 | `BB_SQUEEZE_BO` | 512 L / 513 S | indicator | New mechanic (BBW percentile compression → break); distinct from `MTF_BREAK` (Donchian) and `MEAN_REVERT_BB` (band touch) | TP 0.92% = **0.52% margin** |
| 3 | `VWAP_RECLAIM` | 514 L / 515 S | indicator | Symmetric counterpart to `PRM_VWAP_REJECT` (rejection ↔ reclaim); fills a clear gap | TP 0.80% = **0.40% margin** |
| 4 | `FUNDING_FADE` | 516 L / 517 S | sentiment | Uses funding rate (already fetched, never gated on); only true sentiment input we can ship today; academic backing | TP 0.95% = **0.55% margin** |
| 5 | `VOL_CLIMAX_REV` | 518 L / 519 S | sentiment proxy | Order-flow proxy without L2 (climax bar + rejection close); distinct from `PRM_VOL_DIVERGENCE` (which uses *no* volume) | TP 0.95% = **0.55% margin** |

### 5.1 Pairwise Correlation Matrix

|  | DOUBLE_REV | BB_SQ_BO | VWAP_RCL | FUND_FADE | VOL_CLX |
|---|---|---|---|---|---|
| **DOUBLE_REV** | — | LOW (compression-breakout vs swing-pattern) | MED (both can fire at session lows) | LOW | **HIGH** (both reversal-at-exhaustion) |
| **BB_SQ_BO** | — | — | LOW | LOW | LOW |
| **VWAP_RCL** | — | — | — | LOW | MED (both reversal-biased in chop) |
| **FUND_FADE** | — | — | — | — | LOW (different horizon) |
| **VOL_CLX** | — | — | — | — | — |

**Dedupe rule to add** (implementation note for [futuresDeskPolicy.ts](../src/lib/futuresDeskPolicy.ts)): if `DOUBLE_REV` and `VOL_CLIMAX_REV` fire same-direction on the same bar, retain only the higher-score signal. The existing intra-templateFamily dedupe doesn't reach cross-family — verify and extend in the implementation PR.

### 5.2 What we are *not* recommending

- `LIQUIDITY_SWEEP_RECLAIM` — existing `WYCKOFF_SPRING` covers it.
- `FVG_FILL` — existing `SMART_MONEY_FVG` covers it.
- `ORB_5m_BOX` — existing `SESSION_RANGE_BREAK` + `SESSION_OPEN` cover it.
- `RSI2_CONNORS` — existing `MEANREV_RSI` covers it; tight fee margin (0.25%).
- `MACD_FLIP_ADX` — existing `MTF_MACD_HIST` covers it.
- `STOCH_CROSS` — fails fee math.
- `FUNDING_PREMIUM_DIVERGENCE` — defer to v2 (data plumbing + tight correlation with FUNDING_FADE).
- `FEAR_GREED` — no data; defer to v2.
- `FLAG_CONTINUATION` — tight fee margin; existing momentum templates cover it.

---

## 6. Implementation Constraints (preview of [SCALPING_TOP5_IMPLEMENTATION.md](./SCALPING_TOP5_IMPLEMENTATION.md))

- New file `client/src/lib/btcFtScalpingStrategies.ts` mirrors the [btcFtPremiumStrategies.ts](../src/lib/btcFtPremiumStrategies.ts) pattern: exports `BTC_FT_SCALPING_DEFS: ReadonlyArray<FuturesStratDef>` with the 10 rows.
- Append to `FUTURES_STRAT_DEFS` array in [futuresStrategies.ts:210](../src/lib/futuresStrategies.ts#L210) — analogous to the `...BTC_FT_PREMIUM_DEFS` spread.
- Extend the `BtcFtTemplateId` union in [futuresStratTypes.ts:9](../src/lib/futuresStratTypes.ts#L9) with the 5 new keys.
- Add 5 entries to `BTC_FT_TEMPLATE_META` in [btcFtStrategyTemplates.ts:40](../src/lib/btcFtStrategyTemplates.ts#L40).
- Add 5 `switch` arms each in `evalBtcFtTemplateSignal` and `passesBtcFtTemplateConfirmation` in [futuresSignals.ts:893](../src/lib/futuresSignals.ts#L893).
- **Data extension**: `FUNDING_FADE` requires adding `fundingRate` and `minutesToNextFunding` to `FuturesSignalInputs` and threading via `buildSignalInputs`. The hook already has the data ([useBTCFuturesScalperEngine.ts:163](../src/hooks/useBTCFuturesScalperEngine.ts#L163)) — only the bridge is missing.
- Vitest fixtures under `client/src/lib/__tests__/futuresSignals.scalping.spec.ts` per the [btcFtPremiumStrategies.test.ts](../src/lib/btcFtPremiumStrategies.test.ts) pattern.
- Replay sanity: `npm run replay -- --ids=510-519 --bars=1500 --slippageBps=5`.
- Production safety: 510–519 are NOT in any `promotedIds` set; [applyWinnersOnlyGate](../src/lib/futuresDeskPolicy.ts#L888) filters them out in WINNERS_ONLY mode. They run in research mode only until validated.

---

## 7. Verification Checklist

After implementation:

1. `cd client && npm run typecheck` — must pass. New `BtcFtTemplateId` members must compile in every `switch` exhaustiveness check.
2. `npm test -- futuresSignals.scalping` — each new templateKey has at least one pass and one fail fixture (wrong regime, wrong HTF, wrong fee math).
3. `npm run replay -- --ids=510-519 --bars=1500 --slippageBps=5` — report sumNet honestly. Negative is acceptable for research pool; we measure, we do not promise.
4. Production gate isolation: `applyWinnersOnlyGate(defs, [])` must NOT return any of 510–519. Verify in a unit test alongside [btcFtPremiumStrategies.test.ts](../src/lib/btcFtPremiumStrategies.test.ts).
5. Doc lint: this document and `SCALPING_TOP5_IMPLEMENTATION.md` contain no occurrence of "guaranteed", "always profitable", "risk-free", or "will make money".

---

## 8. Sources

Labelled `academic` (papers, exchange research), `tested` (public backtest with code or numbers), `marketing` (course/blog without numbers).

1. [Funding Rate Mechanism in Perpetual Futures (Zhang, SSRN)](https://papers.ssrn.com/sol3/papers.cfm?abstract_id=6185958) — **academic**
2. [Designing funding rates for perpetual futures in cryptocurrency markets (arXiv 2506.08573)](https://arxiv.org/pdf/2506.08573) — **academic**
3. [Fundamentals of Perpetual Futures (arXiv 2212.06888)](https://arxiv.org/pdf/2212.06888) — **academic**
4. [Assessing the Profitability of Timely Opening Range Breakout on Index Futures Markets (ResearchGate)](https://www.researchgate.net/publication/331076454_Assessing_the_Profitability_of_Timely_Opening_Range_Breakout_on_Index_Futures_Markets) — **academic**
5. [Coinbase: Understanding Funding Rates in Perpetual Futures](https://www.coinbase.com/learn/perpetual-futures/understanding-funding-rates-in-perpetual-futures) — **academic** (exchange research)
6. [QuantifiedStrategies: Double Bottom Strategy backtest](https://www.quantifiedstrategies.com/double-bottom-strategy/) — **tested**
7. [QuantifiedStrategies: Bollinger Band Squeeze Strategy](https://www.quantifiedstrategies.com/bollinger-band-squeeze-strategy/) — **tested**
8. [PyQuantLab: BB + Keltner Squeeze optimization in crypto regimes](https://pyquantlab.medium.com/optimizing-the-bollinger-band-keltner-channel-squeeze-strategy-volatility-breakout-trading-in-70b49101cb30) — **tested**
9. [QuantifiedStrategies: VWAP Trading Strategy backtest](https://www.quantifiedstrategies.com/vwap-trading-strategy/) — **tested**
10. [QuantifiedStrategies: RSI 2 Connors strategy](https://www.quantifiedstrategies.com/rsi-2-strategy/) — **tested**
11. [Unger Academy: ORB strategies on CME Bitcoin futures](https://ungeracademy.com/blog/bitcoin-trading-systems-intraday-breakout-strategies-on-the-cme-regulated-futures-market) — **tested**
12. [Zeiierman: Climax Volume indicator](https://www.zeiierman.com/indicators/climax-volume) — **marketing** (best available; no public backtest)
13. [Daily Price Action: Liquidity Sweep Reversals](https://dailypriceaction.com/blog/liquidity-sweep-reversals/) — **marketing**
14. [CoinTracker: Crypto Scalping Strategy and Fees](https://www.cointracker.io/blog/crypto-scalping-strategy) — **marketing**
15. [Snappchart: VWAP Momentum / Reclaim playbook](https://www.snappchart.app/blog/strategy-playbooks/vwap-momentum-trading-strategy) — **marketing**

Internal cross-references:
- [ROOT_CAUSE.md](./ROOT_CAUSE.md) — fee/expectancy ground truth; load-bearing for the §3 fee floor argument
- [PAPER_DESK_RUNBOOK.md](./PAPER_DESK_RUNBOOK.md) — operational workflow; add a "Scalping strategy research blueprint" sub-section linking this doc
- [SCALPING_TOP5_IMPLEMENTATION.md](./SCALPING_TOP5_IMPLEMENTATION.md) — engineering tickets for the 5 chosen templates

# PHASE 2 — STRATEGY FAMILY CLUSTERING REPORT

**Date:** 2026-06-10  
**Method:** Cluster by actual signal behavior, not by name or file.  
**Question answered:** How many truly distinct signal engines exist across 714 strategies?

---

## Clustering Methodology

Strategies are grouped by their **signal generation mechanism** — the actual computation that produces a buy/sell decision. Strategies sharing the same signal type but different parameters are in the same family. The question is whether different parameters produce statistically distinct signals on BTC 1m, which requires walk-forward testing to confirm. In the absence of such testing, all parameter variants are treated as a single family.

---

## Signal Family Clusters

### Family 1: EMA Crossover
**Signal:** Fast EMA crosses slow EMA  
**Variants counted:** 70+  
- Elite V2: 15 (pairs: 3/8, 5/13, 10/30, 13/34, 21/55, etc.)
- Elite V3: 5 additional
- Intraday: 10 (5m/15m versions)
- Expansion pack: 40 procedural
- Base: EMA_Cross_Scalp (base 8/21)

**Unique signal engines:** **1** (all compute the same crossover with different period values)  
**Family verdict:** Oversaturated. 70 variants where 3-5 validated pairs suffice.

---

### Family 2: RSI Threshold / Mean Reversion
**Signal:** RSI below oversold or above overbought threshold  
**Variants counted:** 43+  
- Elite V2: 8
- Elite V3: 5
- Expansion pack: 30 procedural (5 periods × 6 zone configs)

**Unique signal engines:** **1** (RSI crosses a threshold)  
**Family verdict:** Severely oversaturated. RSI mean reversion on BTC 1m is a crowded, well-known signal with documented slippage.

---

### Family 3: RSI Slope / Momentum
**Signal:** RSI slope (derivative) changes direction  
**Variants counted:** 25+  
- Elite V2: 5
- Expansion pack: 20 procedural

**Unique signal engines:** **1** (RSI slope sign change)  
**Family verdict:** More refined than threshold but still a single mechanism.

---

### Family 4: Bollinger Bands
**Signal:** Price touches/breaks Bollinger Band, or band width signals squeeze  
**Sub-types (3):** Bounce (mean reversion), Breakout, Mid-cross  
**Variants counted:** 25+ Go + 8 Client  
- Elite V2: 12
- Elite V3: 5
- Intraday: 8
- Expansion pack: ~30

**Unique signal engines:** **3** (bounce, breakout, width/squeeze are distinct mechanisms)  
**Family verdict:** Three legitimate sub-families but each is still massively duplicated by parameters.

---

### Family 5: VWAP
**Signal:** Price relationship to Volume-Weighted Average Price  
**Sub-types (3):** Cross (directional), Deviation (mean reversion), Pullback (trend continuation)  
**Variants counted:** 21+ Go + 6 Client  

**Unique signal engines:** **3** (cross, deviation, pullback are distinct)  
**Family verdict:** VWAP is a legitimate intraday anchor. Deviation-based mean reversion has documented edge in short samples.

---

### Family 6: MACD
**Signal:** MACD signal line cross, zero-line cross, histogram momentum  
**Sub-types (3):** Cross, Zero-cross, Histogram  
**Variants counted:** 10 Elite V2 + 8 Intraday + Expansion pack  
**Live losses documented:** MACD_VWAP_Flip (-$10.90), MACD_ZeroCross_Confluence (-$3.71)

**Unique signal engines:** **3** but all are lagging on 1m BTC  
**Family verdict:** MACD is a lagging indicator. On BTC 1m it generates delayed entries after moves. Two documented losers. **Suspect family.**

---

### Family 7: Statistical / Z-Score / Linear Regression
**Signal:** Price deviation from statistical bands (Z-score, LinReg, standard deviation)  
**Variants counted:** ~5  
- ZScoreBand_MeanRev_Scalp (+$4.32 live)
- LinReg_Statistical_Scalp (+$0.56 live)

**Unique signal engines:** **2** (z-score normalization, linear regression fit)  
**Family verdict:** **Best-performing non-multi-signal family.** Statistical approach generates non-indicator-grid edge. Expand, not reduce.

---

### Family 8: Multi-Signal Confluence
**Signal:** Score across 2+ independent indicator types  
**Variants counted:** ~10  
- TripleFilter_Alpha_Scalp: EMA + MACD + ADX (+$20 live — top performer)
- TrendMomentum_Score_Scalp: 5-component score
- SentimentConfluence_Pro_Scalp: RSI + MACD + EMA + VWAP + Volume
- RSI_BB_Confluence_Scalp: RSI + BB (+$3 live)

**Unique signal engines:** Multiple (each combines different indicator types)  
**Family verdict:** **Best Go strategy family.** All documented winners require ≥2 independent signals. This is the correct design direction.

---

### Family 9: Order Flow / Microstructure
**Signal:** Cumulative volume delta, delta absorption, order flow pressure  
**Variants counted:** ~5  
- CVDDivergence_Alpha (dispatch bug — not firing)
- DeltaAbsorption_Alpha (dispatch bug — not firing, synthetic PF 0.91)
- OrderFlow_Pressure_Pro_Scalp (+$2 live)

**Unique signal engines:** **3** (CVD divergence, delta absorption, order flow pressure are distinct)  
**Family verdict:** High theoretical edge. Broken plumbing. Fix priority HIGH.

---

### Family 10: Funding Rate
**Signal:** Funding rate extreme → mean reversion of basis  
**Variants counted:** 1 (FundingMeanReversion_Alpha)  
**Data status:** `funding.ndjson` empty — cannot fire  

**Unique signal engines:** **1**  
**Family verdict:** Genuine institutional alpha source. **Completely dead due to missing data feed.** Fix priority HIGHEST.

---

### Family 11: Liquidity / Smart Money
**Signal:** Stop-loss hunts above/below structure, order block retests, MSS shifts  
**Variants counted:** ~5  
- LiquiditySweepReversal_Alpha (dispatch bug)
- OrderBlockRetest_Alpha (dispatch bug)
- MSSContinuation_Alpha (dispatch bug, synthetic PF 2.92 — highest)

**Unique signal engines:** **3** (sweep reversal, order block retest, market structure shift)  
**Family verdict:** Highest-potential alpha family based on synthetic PF rankings. All non-functional due to dispatch bug. Fix priority HIGHEST.

---

### Family 12: Liquidation Cascade
**Signal:** Large liquidation event ($50k+ notional) → reversal  
**Variants counted:** 1 (LiquidationCascade_Alpha)  
**Data status:** Feed completely unwired in main.go  

**Unique signal engines:** **1**  
**Family verdict:** Legitimate signal with documented institutional use. **Never executed once.** Fix priority HIGH.

---

### Family 13: Volatility / Squeeze
**Signal:** ATR expansion, Keltner/BB squeeze → directional break  
**Variants counted:** 30+  
- Elite V3 ATR: 10
- Elite V3 Keltner: 12
- VolSqueeze_Explosion_Scalp (boosted)
- Intraday Keltner: 5

**Unique signal engines:** **3** (ATR threshold, Keltner break, BB squeeze)  
**Family verdict:** Volatility expansion after compression is a legitimate mechanism. But 30 variants of essentially 3 signals is extreme duplication.

---

### Family 14: Stochastic Oscillator
**Signal:** Stochastic cross in oversold/overbought or trend mode  
**Variants counted:** 17+  
- Elite V3: 12
- Intraday: 5

**Unique signal engines:** **2** (cross mode, trend mode)  
**Family verdict:** Stochastic is a mean-reversion indicator. Useful in range-bound conditions. Ineffective in trending BTC markets.

---

### Family 15: CCI (Commodity Channel Index)
**Signal:** CCI threshold cross (zero, ±100, ±200)  
**Variants counted:** 13+  

**Unique signal engines:** **2** (zero cross, extreme level)  
**Family verdict:** CCI is a displaced oscillator. On BTC 1m it adds minimal information beyond RSI. Retire most variants.

---

### Family 16: Williams %R
**Signal:** %R extreme bounce or trend mode  
**Variants counted:** 8  

**Unique signal engines:** **2** (bounce, trend)  
**Family verdict:** Mathematically equivalent to Stochastic (inverse scale). Retire as redundant unless walk-forward proves distinctness.

---

### Family 17: ROC (Rate of Change)
**Signal:** ROC threshold → momentum direction  
**Variants counted:** 8  

**Unique signal engines:** **1** (momentum threshold)  
**Family verdict:** ROC is a simple momentum oscillator. Largely redundant with MACD histogram momentum signal.

---

### Family 18: Parabolic SAR + EMA
**Signal:** PSAR flip direction + EMA trend agreement  
**Variants counted:** 8  

**Unique signal engines:** **1** (SAR flip)  
**Family verdict:** PSAR is a trailing stop indicator repurposed as entry signal. On 1m BTC it generates excessive whipsaws. **Retire family.**

---

### Family 19: Hull MA
**Signal:** Hull moving average direction change  
**Variants counted:** 8  

**Unique signal engines:** **1** (Hull direction)  
**Family verdict:** Hull MA reduces lag vs EMA but is still a lagging crossover family. Marginal at best on 1m. Retire most variants.

---

### Family 20: N-bar Breakout (Donchian-style)
**Signal:** N-bar high/low break  
**Variants counted:** 15+  

**Unique signal engines:** **1** (N-bar channel break)  
**Family verdict:** Donchian_Breakout was removed with -$7.84. N-bar breakouts with tight SL are breakout-family losers. Retire unless walk-forward proves otherwise.

---

### Family 21: Momentum Divergence
**Signal:** Price makes new extreme, oscillator does not (divergence)  
**Variants counted:** 6  

**Unique signal engines:** **1** (divergence detection)  
**Family verdict:** Divergence is a valid analytical tool but difficult to define precisely in code. This family requires investigation before verdict.

---

### Family 22: Consecutive Candles
**Signal:** N consecutive same-direction candles with ADX gate  
**Variants counted:** 8  

**Unique signal engines:** **1** (streak count + trend filter)  
**Family verdict:** Consecutive candle signals are noise on 1m BTC. High false positive rate. Retire.

---

### Family 23: Chart Pattern (Price Action)
**Signal:** Visual pattern detection: double bottom/top, wedge, exhaustion  
**Variants counted:** ~5  
- Chart_DoubleTap_Reversal_Scalp (+$1.63 live)
- Chart_Wedge_Breakout_Scalp

**Unique signal engines:** **4** (double tap, wedge, pin bar, exhaustion — each distinct)  
**Family verdict:** Double tap has documented positive PnL. Price action patterns are less crowded than indicator signals on BTC. Retain and expand.

---

### Family 24: Session / Time-of-Day
**Signal:** Session open momentum, opening range breakout, session expansion  
**Variants counted:** ~5  
- OpeningRange_Breakout_Scalp (boosted)
- SessionExpansion_Alpha (dispatch bug)
- SessionOpen_Momentum_Scalp (-$1.40 — borderline)

**Unique signal engines:** **3** (ORB, session expansion, open momentum)  
**Family verdict:** Session-based signals are legitimate. Session expansion alpha is broken. Opening range breakout is boosted but unvalidated. Mixed results.

---

### Family 25: Volume Profile / POC
**Signal:** Price rejection from Point of Control  
**Variants counted:** 1 (POCBounce_Alpha, dispatch bug)  

**Unique signal engines:** **1**  
**Family verdict:** Volume profile is an institutional tool. Currently non-functional. Fix if data available.

---

## Summary: True Signal Engine Count

| Cluster | Signal Families | Distinct Engines | Platform Instances | Compression Ratio |
|:--------|:--------------:|:----------------:|:------------------:|:-----------------:|
| Lagging crossovers | EMA, Triple EMA, Hull, PSAR | 4 engines | 100+ strategies | 25:1 |
| Oscillators | RSI, Stoch, CCI, Williams, ROC, MACD | 8 engines | 120+ strategies | 15:1 |
| Statistical | Z-score, LinReg | 2 engines | 5 strategies | 2.5:1 |
| Volatility | ATR, Keltner, BB squeeze | 3 engines | 45+ strategies | 15:1 |
| Volume/Order flow | VWAP, Volume breakout, CVD, Delta | 5 engines | 30+ strategies | 6:1 |
| Price action | Chart patterns | 4 engines | 5 strategies | 1.25:1 |
| Institutional alpha | Funding, Liquidity, MSS, FVG, OB, Liquidation, POC, Session | 8 engines | 17 strategies | 2:1 |
| Multi-signal confluence | Various composites | 5 engines | 10 strategies | 2:1 |
| Expansion pack | Duplicates of above | 0 new engines | 301 strategies | ∞ |

**Total distinct signal engines: 39**  
**Total strategy instances: 714**  
**Average duplication ratio: 18:1**

---

## Phase 2 Verdict

The platform contains **39 truly distinct signal engines** deployed across **714 strategy instances** at an **18:1 average duplication ratio**.

The expansion pack (301 strategies) contributes **0 new signal engines** — it is purely parameter inflation of existing mechanisms.

The most underrepresented families (relative to their expected edge) are:
1. Statistical (2 engines, 5 strategies — should be 10+)
2. Institutional alpha (8 engines, 17 strategies — all non-functional)
3. Multi-signal confluence (5 engines, 10 strategies — should be 20+)

The most overrepresented families (relative to their expected edge):
1. EMA crossover: 70+ instances of 1 engine
2. RSI threshold: 43+ instances of 1 engine
3. Expansion pack: 301 instances of 0 new engines

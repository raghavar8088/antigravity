# PHASE 18 — RECONSTRUCTED ALPHA STACK

**Date:** 2026-06-10  
**Goal:** Design a 10-20 strategy institutional-grade stack capable of live capital deployment.

---

## Design Principles

1. **Signal independence over strategy count** — 10 truly independent signals beat 100 correlated variants
2. **Information hierarchy** — order flow > price action > statistical > indicators
3. **Regime awareness** — every strategy has defined operating regimes
4. **Validated first, deployed second** — no strategy in production without OOS evidence
5. **Institutional edge** — focus on signals that retail algos cannot easily replicate
6. **Portfolio coherence** — strategies must complement each other's regime coverage

---

## The Reconstructed Alpha Stack: 12 Strategies

### LAYER 1: TREND ALPHA (Regime: Trend, Strong Trend)

**Strategy A1: Trend Confluence Engine**

Design: 3-component trend confluence
- Component 1: EMA ribbon alignment (EMA9 > EMA21 > EMA55 for long)
- Component 2: MACD histogram positive and expanding
- Component 3: ADX > 25 (trend strength confirmed)
- Volume gate: Volume > 1.2× 20-bar average

Entry: All 3 components confirm + volume gate  
SL: 2× ATR(14) behind entry  
TP: 5× ATR(14) ahead of entry  
TP2 (trail): 1× ATR trailing after TP1 (40% position)  
TIME exit: 30 candles  
Regime: Trend, Strong Trend, Breakout only

**Prototype:** `TripleFilter_Alpha_Scalp` (proven +$20 live) — rebuild with ATR stops

---

**Strategy A2: Volume-Weighted Institutional Trend**

Design: Volume-weighted price trend signal
- Primary: Price trend confirmed by volume-weighted average price behavior
- Filter: Price consistently above VWAP in trend direction
- Volume: Increasing volume on trend candles, decreasing on pullbacks
- Entry: VWAP reclaim after pullback with volume confirmation

SL: 2× ATR(14)  
TP: 4× ATR(14)  
TIME exit: 25 candles  
Regime: Trend, Strong Trend

**Prototype:** `VolumeWeighted_Trend_Scalp` (proven +$16 live) — rebuild with ATR stops

---

### LAYER 2: MEAN REVERSION ALPHA (Regime: Range, Weak Trend)

**Strategy B1: Statistical Mean Reversion**

Design: Z-score band reversion + trend filter
- Entry: Price Z-score < -2.0 (extreme deviation below mean)
- Filter: ADX < 20 (range condition — don't fade strong trends)
- Volume: Normal or below (not institutional accumulation)
- Statistical basis: 30-bar Z-score with 2.0 SD threshold

SL: 1.5× ATR(14)  
TP: 3× ATR(14)  
TIME exit: 20 candles  
Regime: Range, Weak Trend, Compression

**Prototype:** `ZScoreBand_MeanRev_Scalp` (proven +$4.32 live) — rebuild with ATR stops and regime gate

---

**Strategy B2: Confluence Mean Reversion**

Design: RSI extremes + Bollinger Band touch + volume
- Entry: RSI < 28 (oversold) + price at lower BB + volume spike (institutional accumulation signal)
- Filter: Not in strong downtrend (EMA20 slope > -0.5%)
- Dual confirmation requires both RSI AND BB to agree

SL: 1.5× ATR(14)  
TP: 3.5× ATR(14)  
TIME exit: 20 candles  
Regime: Range, Weak Trend

**Prototype:** `RSI_BB_Confluence_Scalp` (proven +$3.00 live)

---

### LAYER 3: BREAKOUT ALPHA (Regime: Breakout, Volatile)

**Strategy C1: Volatility Squeeze Explosion**

Design: Bollinger Band + Keltner Channel squeeze → directional breakout
- Setup: BB inside Keltner (squeeze = compression)
- Entry: First bar BB expands outside Keltner in direction of break
- Volume: Volume > 1.5× average at breakout bar
- Momentum: MACD histogram crosses zero in direction of break

SL: 2× ATR(14) on opposite side of breakout  
TP1: 3× ATR(14) = 50% exit  
TP2: Trail at 1.5× ATR  
TIME exit: 40 candles  
Regime: Compression → Breakout transition only

**Prototype:** `VolSqueeze_Explosion_Scalp` (boosted, needs validation)

---

**Strategy C2: Opening Range Breakout**

Design: Session-specific breakout
- Session: Bitcoin volatility peaks on US market open (14:30-15:30 UTC) and Asian open (01:00-02:00 UTC)
- Range: First 15 minutes of session defines high/low
- Entry: Breakout above/below with volume > 1.5×

SL: 50% of range distance  
TP: 200% of range distance  
TIME exit: Session close (2 hours max)  
Regime: Any (session time overrides regime)

**Prototype:** `OpeningRange_Breakout_Scalp` (boosted, needs validation)

---

### LAYER 4: INSTITUTIONAL ALPHA (Regime: All / Specific)

**Strategy D1: Market Structure Shift (MSS) Continuation**

Design: Smart money market structure
- Identify: Break of Structure (BOS) — new higher high in uptrend or lower low in downtrend
- Wait: Price retests the BOS level (retest of the breakout point)
- Confirm: Retest holds (momentum bounce from structure level)
- Entry: Candle close confirming hold of structure

SL: Below/above the structure level (hard invalidation)  
TP1: Next swing target (1.5× structure range)  
TP2: Trail at 1× ATR  
TIME exit: 50 candles  
Regime: Trend, Strong Trend, Breakout  
**Requires:** OnCandle dispatch fix

---

**Strategy D2: Order Block Institutional Retest**

Design: Smart money supply/demand zones
- Identify: High-volume consolidation candles (order blocks) where institutional orders accumulate
- Wait: Price returns to the order block zone
- Entry: Reversal signal at order block edge (pin bar, engulfing, or strong wick rejection)
- Confirm: Volume spike at zone entry

SL: Beyond order block (zone invalidation)  
TP: Next liquidity level above/below  
TIME exit: 40 candles  
Regime: Trend, Strong Trend, Breakout  
**Requires:** OnCandle dispatch fix

---

**Strategy D3: Fair Value Gap (FVG) Continuation**

Design: Price imbalance filling
- Identify: Three-candle pattern with gap between candle 1 high and candle 3 low (bullish FVG)
- Wait: Price returns to fill the FVG
- Entry: Partial fill of FVG (50% level = optimal entry)
- Direction: In direction of pre-FVG trend

SL: Below/above entire FVG range  
TP: High of impulse candle that created the FVG  
TIME exit: 30 candles  
Regime: Trend, Strong Trend  
**Requires:** OnCandle dispatch fix

---

**Strategy D4: Funding Rate Mean Reversion**

Design: Perpetual futures funding arbitrage
- Signal: Funding rate > +0.03% (extremely positive = retail longs crowded)
- Entry: SHORT (fade the crowded trade) + momentum confirmation (price shows weakness)
- Signal: Funding rate < -0.03% (extremely negative = retail shorts crowded)
- Entry: LONG + momentum confirmation

SL: 2× ATR(14)  
TP: Funding reversion to neutral (rate returns to ±0.01%)  
Position size: Proportional to funding extreme (larger when more extreme)  
Regime: All regimes (funding extremes occur in any regime)  
**Requires:** Populate funding.ndjson from Binance/Delta API

---

**Strategy D5: Liquidation Cascade Reversal**

Design: Post-liquidation mean reversion
- Signal: Liquidation event ≥$50k notional (abnormally large tick)
- Pattern: Large liquidation creates price overshoot beyond true value
- Entry: After cascade slows (volume decelerates after spike), enter against direction of cascade
- Confirm: Order flow shows absorption of cascaded positions

SL: Beyond cascade low/high  
TP: Return to pre-cascade level  
TIME exit: 10 candles max (fast reversion or invalid)  
Regime: Volatile, Panic only  
**Requires:** Wire liquidation feed in main.go

---

### LAYER 5: ORDER FLOW ALPHA (Regime: Trend, Breakout)

**Strategy E1: CVD Divergence Institutional Flow**

Design: Cumulative Volume Delta vs price divergence
- Signal: Price makes higher high but CVD does not (institutional sellers using rallies to exit)
- Entry: Short when CVD diverges bearishly; Long when CVD diverges bullishly
- Confirm: Volume spike on divergent bar (institutional activity)

SL: 2× ATR(14)  
TP: 4× ATR(14)  
TIME exit: 20 candles  
Regime: Trend, Strong Trend  
Current status: Partially working (barely passes quality gate)

---

### LAYER 6: VOLATILITY ALPHA (Regime: Volatile, Panic)

**Strategy F1: ATR Momentum Breakout**

Design: Pure volatility expansion signal
- Signal: ATR(14) > 1.5× ATR(14,50-bar average) → elevated volatility
- Direction: From short-term price momentum (3-bar)
- Entry: Direction of momentum when volatility spikes
- Volume: High volume confirms institutional trigger event

SL: 2× ATR(14)  
TP: 4× ATR(14)  
TIME exit: 15 candles  
Regime: Volatile, Strong Trend

---

## The Complete 12-Strategy Stack

| # | Strategy | Layer | Regime | Status | Priority |
|--:|:---------|:------|:-------|:-------|:---------|
| 1 | Trend Confluence Engine | Trend | Trend/StrongTrend | Rebuild from TripleFilter | HIGHEST |
| 2 | Volume-Weighted Institutional Trend | Trend | Trend/StrongTrend | Rebuild from VolumeWeighted | HIGHEST |
| 3 | Statistical Mean Reversion | Mean Rev | Range/WeakTrend | Rebuild from ZScore | HIGH |
| 4 | Confluence Mean Reversion | Mean Rev | Range/WeakTrend | Rebuild from RSI_BB | HIGH |
| 5 | Volatility Squeeze Explosion | Breakout | Compression→Break | Validate VolSqueeze | HIGH |
| 6 | Opening Range Breakout | Breakout | Session/Any | Validate ORB | MEDIUM |
| 7 | MSS Continuation | Smart Money | Trend/Break | Fix dispatch | CRITICAL |
| 8 | Order Block Retest | Smart Money | Trend/Break | Fix dispatch | CRITICAL |
| 9 | FVG Continuation | Smart Money | Trend | Fix dispatch | HIGH |
| 10 | Funding Rate Mean Reversion | Institutional | All | Fix data feed | CRITICAL |
| 11 | Liquidation Cascade Reversal | Institutional | Volatile/Panic | Fix feed | HIGH |
| 12 | CVD Divergence Flow | Order Flow | Trend/Break | Tune quality gate | MEDIUM |

---

## Portfolio Coverage Assessment

| Regime | Strategies Active | Coverage |
|:-------|:----------------:|:--------:|
| Strong Trend | 1, 2, 7, 8, 9, 10 | ✅ |
| Trend | 1, 2, 3*, 4*, 7, 8, 9, 10, 12 | ✅ |
| Weak Trend | 3, 4, 6 | ✅ |
| Range | 3, 4 | ⚠️ Thin |
| Volatile | 1, 2, 5, 10, 11, 12 | ✅ |
| Panic | 10, 11 | ⚠️ Only alpha |
| Compression | 5 | ⚠️ Thin |
| Breakout | 1, 2, 5, 6, 7, 8, 12 | ✅ |

*Range participation only with ADX < 20 gate

**Gap:** Range regime needs 1-2 more pure range strategies. Stochastic_Range_Scalp (Tier A) fills this gap.

---

## Phase 18 Verdict

The reconstructed 12-strategy alpha stack is **architecturally complete but not yet operational.** It requires:
1. Dispatch bug fix (unlocks strategies 7, 8, 9)
2. Funding data feed (unlocks strategy 10)
3. Liquidation feed (unlocks strategy 11)
4. ATR stop reconstruction (all strategies)
5. 30-day paper validation before capital allocation

**When operational, this stack represents a qualitatively different platform from the current 714-strategy registry** — one where every active strategy has a defined regime, a theoretical edge, and a validation path to capital.

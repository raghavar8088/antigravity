# PHASE 1 — STRATEGY MASTER REGISTRY

**Program:** Institutional Alpha Reconstruction & Profitability Certification  
**Date:** 2026-06-10  
**Auditors:** Principal Quant Researcher / Head of Systematic Trading / Independent Forensic Auditor  
**Standard:** Evidence-only. Every claim traceable to source code or live data.

---

## Registry Overview

| Stack | Total Defined | Production-Active | Executes/Batch | Asset |
|:------|:------------:|:-----------------:|:--------------:|:------|
| Go `BuildCuratedScalpers()` | **606** | 606 wired | ≤25 (aggregator cap) | BTC-USD |
| Client `FUTURES_STRAT_DEFS` | **108** | 48 (default worker) | All 48 | BTCUSD perp |
| **Grand total** | **714** | **654** | **≤73/candle** | BTC |

**Critical runtime constraint discovered in `loop.go`:**  
All Go strategy signals are sanitized before execution:
- `minSignalTakeProfitPct = 0.50%` — TP floors at 0.50% regardless of strategy setting
- `maxSignalStopLossPct = 0.20%` — SL caps at 0.20% regardless of strategy setting
- `minRewardToRiskRatio = 2.40` — signals below 2.4:1 RR are rejected
- `minExecutableConfidence = 0.68` — signals below 0.68 confidence are dropped

**Critical runtime discovery in `positions/manager.go`:**  
- `MaxPositionAgeMins = 45` — positions auto-close after 45 minutes (TIME exit IS present)
- `PartialTPRatio = 1.0` — full position closes at TP (no partial scale-out despite field existing)
- `MinTakeProfitPct = 0.30%` — additional floor after signal sanitization

**Net effective geometry at execution:** SL ≤0.20%, TP ≥0.50%, RR ≥2.40:1, TIME 45 min

---

## Group A: EMA Crossover Family

**Signal type:** Fast EMA crosses slow EMA — momentum/trend signal  
**Asset:** BTC-USD | **Timeframe:** 1m (base), 5m/15m (intraday variants)  
**Unique logic:** 1 (all share same crossover mechanism — only parameters differ)

| ID | Name | File | SL | TP | Live Evidence | Status |
|---:|:-----|:-----|:--:|:--:|:-------------|:-------|
| 1 | EMA_Cross_Scalp | `scalpers.go` | 0.15% | 0.25% | **+$4.51** | ACTIVE, Tier A |
| 2-16 | EMA_X_Y_Cross_Scalp (15 variants) | `elite_v2.go` | 0.16-0.20% | 0.38-0.46% | None | ACTIVE, Tier C |
| 17-21 | EMA_extra (5 variants, V3) | `elite_v3.go` | varies | varies | None | ACTIVE, Tier C |
| 22-31 | ID_EMA (10 intraday variants) | `intraday_strategies.go` | 0.22-0.35% | 0.55-0.88% | None | ACTIVE, Tier C |
| 32-71 | XP_EMA_X_Y_Cross (40 variants) | `curated_expansion_pack.go` | 0.15-0.18% | 0.33-0.40% | None | RETIRE |

**Backtest evidence:** None for any variant (only base has live PnL)  
**Regime:** Best in Trend/Strong Trend; fails in Range  
**Entry logic:** `prevAbove != above` — only fires on the crossover bar itself (not steady state)  
**ADX guard:** `adxMin` 18-28 depending on variant  
**RSI band:** Buy zone 45-68 (no oversold entry)

---

## Group B: RSI Threshold Family

**Signal type:** RSI exits oversold/overbought zone — mean reversion  
**Unique logic:** 1 (all compute RSI threshold crossing)

| ID | Name | File | RSI Zone | SL | TP | Live Evidence | Status |
|---:|:-----|:-----|:---------|:--:|:--:|:-------------|:-------|
| 1-8 | RSI_Oversold/Cross/Bull (8 variants) | `elite_v2.go` | 22-60 | 0.17-0.19% | 0.40-0.44% | None | ACTIVE, Tier D |
| 9-13 | RSI extra (5 variants, V3) | `elite_v3.go` | 20-60 | varies | varies | None | ACTIVE, Tier D |
| 14-43 | XP_RSI (30 variants) | `curated_expansion_pack.go` | 22-60 | 0.17-0.19% | 0.40-0.44% | None | RETIRE |
| — | RSI_Reversal_Scalp | `scalpers.go` | <30 | 0.15% | 0.25% | Unknown | ACTIVE, Tier C |

---

## Group C: RSI Slope / Momentum Family

**Signal type:** RSI slope (1st derivative) changes direction  
**Unique logic:** 1

| ID | Name | File | Lookback | Live Evidence | Status |
|---:|:-----|:-----|:---------|:-------------|:-------|
| 1-5 | RSI_Slope variants | `elite_v2.go` | 3-10 | None | ACTIVE, Tier D |
| 6-25 | XP_RSI_Slope (20 variants) | `curated_expansion_pack.go` | varies | None | RETIRE |

---

## Group D: Bollinger Bands Family

**Signal type:** BB touch (mean reversion), BB break (momentum), BB mid-cross, BB width/squeeze  
**Unique logic:** 3 (bounce, breakout, squeeze — mechanically distinct)

| Subtype | Count | SL | TP | Live Evidence | Status |
|:--------|------:|:--:|:--:|:-------------|:-------|
| BB_Bounce | 12 | 0.17-0.20% | 0.40-0.50% | None | Tier C (1 survivor) |
| BB_Breakout | 8 | 0.17-0.22% | 0.40-0.55% | None | Tier D (breakout family) |
| BB_Width/Squeeze | 6 | 0.16-0.18% | 0.40-0.44% | None | Tier B (VolSqueeze) |
| BB+RSI confluence | 1 (RSI_BB_Confluence) | varies | varies | **+$3.00** | Tier A |
| BollingerWalk | 1 | varies | varies | positive | Tier A |
| Bollinger_RSI_Fade | 1 | varies | varies | boosted | Tier B |
| Expansion pack BB | ~30 | 0.15-0.18% | 0.33-0.40% | None | RETIRE |

---

## Group E: VWAP Family

**Signal type:** VWAP cross (directional), VWAP deviation (mean reversion), VWAP pullback (trend continuation)  
**Unique logic:** 3

| Subtype | Count | Live Evidence | Status |
|:--------|------:|:-------------|:-------|
| VWAP_Cross | 7 | None | Tier D |
| VWAP_Dev (mean rev) | 6 | -$1.42 (VWAP_RSI2) | Borderline — REMOVE |
| VWAP_Pullback | 5 | -$1.07 (VWAP_Bounce) | Borderline — REMOVE |
| VWAP_Scalp (base) | 1 | Unknown | Tier C |

---

## Group F: MACD Family

**Signal type:** MACD line cross signal, MACD zero-line cross, MACD histogram momentum  
**Unique logic:** 3 (all lagging)

| Subtype | Count | Live Evidence | Status |
|:--------|------:|:-------------|:-------|
| MACD_Cross | 6 | 0 | RETIRE (documented losers) |
| MACD_Zero | 4 | -$3.71 (ZeroCross removed) | RETIRE |
| MACD_HistMom | 4 | -$10.90 (MACD_VWAP removed) | RETIRE |
| Intraday MACD (5m/15m) | 8 | 0 | RETIRE |
| **Total** | **22** | **Net negative** | **RETIRE ENTIRE FAMILY** |

---

## Group G: Statistical Mean Reversion Family

**Signal type:** Price deviation from statistical fit — objective, non-arbitrary  
**Unique logic:** 2 (Z-score normalization, linear regression band)

| Name | File | Signal | SL | TP | Live Evidence | Status |
|:-----|:-----|:-------|:--:|:--:|:-------------|:-------|
| ZScoreBand_MeanRev_Scalp | `scalpers.go` | Z < -2.0 from 30-bar mean | varies | varies | **+$4.32** | **Tier A** |
| LinReg_Statistical_Scalp | `scalpers.go` | Price below LinReg band | varies | varies | **+$0.56** | **Tier A** |

**Assessment:** Only 2 strategies in this family. Both are winners. Family is underrepresented.

---

## Group H: Multi-Signal Confluence Family

**Signal type:** Score from 2+ independent indicator types — reduces false signals  
**Unique logic:** 5 (each combines different indicator sets)

| Name | File | Components | Live Evidence | Status |
|:-----|:-----|:-----------|:-------------|:-------|
| TripleFilter_Alpha_Scalp | `scalpers_elite2.go` | EMA(20) + MACD hist + ADX>25 | **+$20.00** | **Tier A #1** |
| VolumeWeighted_Trend_Scalp | `profit_composites.go` | Volume × EMA trend | **+$16.00** | **Tier A #2** |
| RSI_BB_Confluence_Scalp | `scalpers.go` | RSI oversold + BB touch | **+$3.00** | **Tier A** |
| TrendMomentum_Score_Scalp | `scalpers_pro.go` | EMA ribbon+MACD+RSI+ADX+VWAP | boosted | **Tier B** |
| SentimentConfluence_Pro_Scalp | `sentiment_confluence_scalper.go` | RSI+MACD+EMA+VWAP+volume | unknown | **Tier C** |

---

## Group I: Order Flow / Microstructure Family

**Signal type:** Causal information — actual order activity, not price derivatives  
**Unique logic:** 3 (pressure accumulation, CVD divergence, delta absorption)

| Name | File | Mechanism | SL | TP | Live Evidence | Status |
|:-----|:-----|:----------|:--:|:--:|:-------------|:-------|
| OrderFlow_Pressure_Pro_Scalp | `scalpers_pro.go` | 80-bar flow pressure imbalance | 0.20% | 0.40% | **+$2.00** | **Tier A** |
| CVDDivergence_Alpha | `alpha_strategies.go` | Price vs CVD divergence (tick-level) | 0.30% | 0.75% | $0 (firing partial) | **Tier B** |
| DeltaAbsorption_Alpha | `alpha_strategies.go` | Delta absorbed by counter-trend | 0.30% | 0.75% | $0 (dispatch bug) | **Tier B** |

---

## Group J: Institutional Alpha Family (Smart Money)

**Signal type:** Market structure, order blocks, fair value gaps — institutional footprints  
**Unique logic:** 5 (MSS, OB, FVG, Liquidity Sweep, POC — each distinct)  
**Status:** ALL BROKEN — dispatch bug prevents execution

| Name | Mechanism | Synthetic PF | Dispatch | Data | Status |
|:-----|:----------|:------------:|:--------:|:----:|:-------|
| MSSContinuation_Alpha | Structure break + retest | **2.92** | ❌ Bug | ✅ | Fix priority 1 |
| OrderBlockRetest_Alpha | Institutional OB retest | **1.79** | ❌ Bug | ✅ | Fix priority 2 |
| FVGRetest_Alpha | Fair value gap fill | **1.48** | ❌ Bug | ✅ | Fix priority 3 |
| LiquiditySweepReversal_Alpha | Stop-hunt reversal | 1.02 | ❌ Bug | ✅ | Fix priority 4 |
| POCBounce_Alpha | Volume profile POC reject | 1.19 | ❌ Bug | ✅ | Fix priority 5 |
| SessionExpansion_Alpha | Session expansion bias | — | ❌ Bug | ✅ | Fix priority 5 |

---

## Group K: Funding / Carry Alpha

**Signal type:** Perpetual futures funding rate extreme → mean reversion  
**Status:** DEAD — data file empty

| Name | Mechanism | Synthetic PF | Data | Status |
|:-----|:----------|:------------:|:----:|:-------|
| FundingMeanReversion_Alpha | Funding extreme fade | **2.09** | ❌ Empty | Fix data feed |
| Phase11FundingMeanReversion_Alpha | Same + microstructure score | — | ❌ Empty | Fix data feed |

---

## Group L: Liquidation Alpha

**Signal type:** Large forced liquidation event → reversal  
**Status:** PARTIALLY OPERATIONAL (tick-heuristic in OnTick, but feed unwired)

| Name | Mechanism | Status |
|:-----|:----------|:-------|
| LiquidationCascade_Alpha | ≥$50k notional single tick → reversal | Partial (heuristic) |
| Phase11LiquidationCascadeReversal_Alpha | Same + microstructure quality score | Partial |

---

## Group M: Phase 11 Unified Microstructure (7 strategies)

Unified quality-gated versions of the 7 institutional alpha modules. Same signal logic, combined quality scorer (threshold 70). All share dispatch bug. All $0 live PnL.

---

## Group N: Stochastic Family

| Count | Subtype | Status |
|------:|:--------|:-------|
| 12 | Elite V3 stochastic cross/oversold/trend | Tier C (1 survivor) |
| 5 | Intraday 5m stochastic | Tier C (1 survivor) |
| 1 | Stochastic_Range_Scalp (base) | **Tier A — +$1.77 live** |

---

## Group O: Volatility / ATR / Keltner Family

| Count | Subtype | Status |
|------:|:--------|:-------|
| 10 | Elite V3 ATR signal | Tier C (1 survivor — ATR_Momentum) |
| 12 | Elite V3 Keltner | Tier C (2 survivors) |
| 1 | VolSqueeze_Explosion_Scalp | **Tier B** — boosted |
| 1 | AdaptiveRSI_Dynamic_Scalp | Tier C |
| 2 | ATR_Breakout, ATR_Volume_Impulse | **REMOVED** (-$15.43, -$19.65) |

---

## Group P: Chart Pattern / Price Action Family

| Name | Mechanism | SL | TP | Live Evidence | Status |
|:-----|:----------|:--:|:--:|:-------------|:-------|
| Chart_DoubleTap_Reversal_Scalp | Double bottom/top pattern | varies | varies | **+$1.63** | **Tier A** |
| Chart_Wedge_Breakout_Scalp | Range compression + directional break | varies | varies | Unknown | Tier C |
| FractalScalper | Williams fractal H/L breakout | 0.40% | 1.20% | Unknown | Tier C |
| OpeningRange_Breakout_Scalp | 15m session range break | varies | varies | Boosted | Tier B |

---

## Group Q: Session / Time-of-Day Family

| Name | Regime | Status |
|:-----|:-------|:-------|
| SessionOpen_Momentum_Scalp | Session open | **-$1.40 — REMOVE** |
| SessionExpansion_Alpha | Dispatch bug | Tier B (fix required) |
| OpeningRange_Breakout_Scalp | ORB | Tier B |

---

## Group R: Retire Families (Complete)

| Family | Count | Reason |
|:-------|------:|:-------|
| XP_* Expansion Pack | 301 | Parameter-grid overfit, zero edge |
| CCI family | 13 | Redundant with RSI, no evidence |
| Williams %R family | 8 | Redundant with Stochastic |
| ROC family | 8 | Redundant with MACD histogram |
| Parabolic SAR family | 8 | Whipsaw-prone |
| N-bar Breakout family | 15 | Donchian proven loser |
| Consecutive Candles family | 8 | Noise signal |
| Hull MA family | 8 | Lagging, redundant with EMA |
| Triple EMA (beyond 2) | 14 | Parameter inflation |

---

## Unique Signal Engine Count

| Signal Class | Distinct Engines | Platform Instances | Duplication |
|:-------------|:----------------:|:-----------------:|:-----------:|
| EMA crossover | 1 | 71+ | 71:1 |
| RSI threshold | 1 | 43+ | 43:1 |
| RSI slope | 1 | 25+ | 25:1 |
| Bollinger Bands | 3 | 55+ | 18:1 |
| VWAP | 3 | 21+ | 7:1 |
| MACD | 3 | 22 | 7:1 |
| Statistical | 2 | 2 | 1:1 |
| Multi-signal confluence | 5 | 5 | 1:1 |
| Order flow | 3 | 3 | 1:1 |
| Smart money alpha | 5 | 17 | 3:1 |
| Funding alpha | 1 | 2 | 2:1 |
| Liquidation | 1 | 2 | 2:1 |
| Volatility/ATR | 3 | 25+ | 8:1 |
| Price action | 4 | 5 | 1.25:1 |
| Session/Time | 3 | 3 | 1:1 |
| Expansion pack | **0** | **301** | **∞** |

**Total unique signal engines: 39**  
**Total strategies: 714**  
**System-wide duplication ratio: 18.3:1**

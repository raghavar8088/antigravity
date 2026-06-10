# PHASE 1 — STRATEGY DECOMPOSITION REPORT

**Audit Authority:** CIO / Head of Quantitative Research / Independent System Auditor  
**Date:** 2026-06-10  
**Scope:** All 714 defined strategies (606 Go engine + 108 Client desk)  
**Standard:** Evidence-only. No synthetic certification. No assumption of edge.

---

## 1. Total Strategy Universe

| Stack | Defined | Production-Reachable | Executes Per Batch |
|:------|--------:|:--------------------:|:------------------:|
| Go `BuildCuratedScalpers()` | 606 | 606 (registered) | ≤25 (aggregator cap) |
| Client `FUTURES_STRAT_DEFS` | 108 | 48 (default worker) | All 48 |
| Go legacy `BuildAllScalpers()` | ~108 | Not live | 0 |
| **Total audited** | **714** | **654** | **≤73** |

**Critical finding:** Of 606 Go strategies, a maximum of 25 are evaluated per candle close due to the aggregator capacity cap. Execution probability per strategy per candle = 4.1%. In practice, boosted strategies consume most slots, meaning the bottom 500 strategies execute near-zero trades per week.

---

## 2. Complete Strategy Decomposition by Family

### 2A. Base Scalpers (24 active, 11 permanently removed)

**Source:** `engine/internal/strategy/scalpers.go`, `scalpers_elite*.go`, `scalpers_pro*.go`  
**Asset:** BTC-USD  
**Timeframe:** 1m  

| # | Strategy Name | Indicators | Entry Logic | SL% | TP% | Live PnL | Status |
|--:|:-------------|:-----------|:------------|----:|----:|:---------|:-------|
| 1 | EMA_Cross_Scalp | EMA(8,21) | Fast > slow crossover | 0.15 | 0.25 | +$4.51 | ACTIVE |
| 2 | RSI_Reversal_Scalp | RSI(14) | RSI < 30 bounce | 0.15 | 0.25 | UNKNOWN | ACTIVE |
| 3 | Bollinger_Scalp | BB(20,2) | Price touches lower band | 0.15 | 0.25 | UNKNOWN | ACTIVE |
| 4 | VWAP_Scalp | VWAP,EMA(50) | Price below VWAP, EMA cross | 0.15 | 0.25 | UNKNOWN | ACTIVE |
| 5 | Mean_Reversion_Scalp | EMA,RSI | Price deviation from EMA | 0.15 | 0.25 | UNKNOWN | ACTIVE |
| 6 | ZScoreBand_MeanRev_Scalp | ZScore(30,2) | Z < -2 reversion | 0.15 | 0.25 | +$4.32 | ACTIVE |
| 7 | RSI_BB_Confluence_Scalp | RSI,BB | RSI oversold + BB touch | 0.15 | 0.25 | +$3.00 | ACTIVE |
| 8 | LinReg_Statistical_Scalp | LinReg(30) | Price below LinReg band | 0.15 | 0.25 | +$0.56 | ACTIVE |
| 9 | TripleFilter_Alpha_Scalp | EMA(20),MACD,ADX | EMA + MACD hist > 0 + ADX > 25 | varies | varies | +$20.00 | ACTIVE |
| 10 | VolumeWeighted_Trend_Scalp | Volume,EMA | Vol-weighted trend confirm | varies | varies | +$16.00 | ACTIVE |
| 11 | OrderFlow_Pressure_Pro_Scalp | Order flow (80-bar) | Flow pressure imbalance | 0.20 | 0.40 | +$2.00 | ACTIVE |
| 12 | Stochastic_Range_Scalp | Stoch(14,3) | Stoch cross in range | 0.15 | 0.25 | +$1.77 | ACTIVE |
| 13 | Chart_DoubleTap_Reversal_Scalp | Price action | Double bottom/top | varies | varies | +$1.63 | ACTIVE |
| 14 | BollingerWalk_Trend_Scalp | BB(20,2) | BB walk continuation | 0.15 | 0.25 | positive | ACTIVE |
| — | TrendMomentum_Score_Scalp | EMA ribbon,MACD,RSI,ADX,VWAP (5 components) | Composite score ≥ 3 | varies | varies | boosted | ACTIVE |
| — | SentimentConfluence_Pro_Scalp | RSI,MACD,EMA ribbon,VWAP,volume | Multi-indicator sentiment | varies | varies | UNKNOWN | ACTIVE |
| — | OpeningRange_Breakout_Scalp | Session range | 16h 0m, 15m range break | varies | varies | boosted | ACTIVE |
| — | VolSqueeze_Explosion_Scalp | ATR,BB | Volatility squeeze → expansion | varies | varies | boosted | ACTIVE |
| — | Bollinger_RSI_Fade_Scalp | BB,RSI | BB extreme + RSI fade | varies | varies | boosted | ACTIVE |
| — | RSI_MACD_Divergence_Scalp | RSI,MACD | Divergence pattern | varies | varies | -$2.06 | BORDERLINE |
| — | SessionOpen_Momentum_Scalp | Session,EMA | First-candle session momentum | varies | varies | -$1.40 | BORDERLINE |
| — | VWAP_RSI2_Reversion_Scalp | VWAP,RSI(2) | Stretch from VWAP + RSI(2) | varies | varies | -$1.42 | BORDERLINE |
| — | VWAP_Bounce_Pro_Scalp | VWAP | VWAP bounce | varies | varies | -$1.07 | BORDERLINE |
| — | TripleTrend_Confluence_Scalp | 3× EMA trend filters | Triple trend alignment | varies | varies | -$1.43 | BORDERLINE |

**Permanently removed (11) — live loss evidence:**

| Strategy | Documented Loss | Primary Failure Mode |
|:---------|----------------:|:--------------------|
| ATR_Volume_Impulse_Scalp | -$19.65 | Vol spike entry → noise SL |
| ATR_Breakout | -$15.43 | Breakout false signal |
| KAMA_Adaptive | -$14.36 | Adaptive lag in fast market |
| PriceChannel_Breakout | -$11.29 | Channel break revert |
| MACD_VWAP_Flip | -$10.90 | Lagging MACD + VWAP conflict |
| Donchian_Breakout | -$7.84 | Range breakout failure |
| ADX_Trend_Scalp | -$7.86 | ADX lagging trend detection |
| VolumeBreakout_Impulse | -$5.34 | Volume spike fade |
| Pullback_Continuation_Pro | -$4.27 | Pullback overshot |
| MACD_ZeroCross_Confluence | -$3.71 | MACD zero-line whipsaw |
| VolumeDelta_Spike | -$3.44 | Delta spike reversal |

---

### 2B. Elite V2 Strategies (95 strategies)

**Source:** `engine/internal/strategy/elite_v2.go`  
**Architecture:** 15 generic Go structs instantiated with parameter combinations  
**Guards:** ADX minimum + RSI band on all entries  
**R:R:** ≥ 1.5 enforced  

| Family | Count | Key Struct | Timeframe | Entry Logic |
|:-------|------:|:----------|:---------|:------------|
| EMA Cross | 15 | `EMACrossV2` | 1m | Fast/slow EMA cross + ADX + RSI band |
| RSI Threshold | 8 | `RSIThresholdScalper` | 1m | RSI in buy zone + ADX guard |
| RSI Slope | 5 | `RSISlopeScalper` | 1m | RSI slope reversal |
| Bollinger Bands | 12 | `BBSignalScalper` | 1m | BB touch/break/mid-cross |
| VWAP | 10 | `VWAPSignalScalper` | 1m | VWAP cross/deviation/pullback |
| MACD | 10 | `MACDSignalScalperV2` | 1m | MACD cross/zero/histogram |
| Volume+Price | 8 | `VolumePriceScalper` | 1m | Volume impulse + price confirm |
| N-bar Breakout | 10 | `NBarBreakoutScalper` | 1m | N-bar high/low break |
| Triple EMA | 8 | `TripleEMAScalperV2` | 1m | 3-EMA alignment |
| CCI | 8 | `CCISignalScalper` | 1m | CCI zero-cross/extreme/trend |

**Assessment:** All 95 strategies are **parameter-grid variants**. Identical signal logic with different period numbers. Zero individually validated. Zero live PnL evidence.

---

### 2C. Elite V3 Strategies (105 strategies)

**Source:** `engine/internal/strategy/elite_v3.go`  
**Architecture:** Same generic struct pattern as V2  

| Family | Count | Timeframe |
|:-------|------:|:---------|
| Stochastic | 12 | 1m |
| ATR Signal | 10 | 1m |
| ROC | 8 | 1m |
| Williams %R | 8 | 1m |
| Parabolic SAR + EMA | 8 | 1m |
| Hull MA | 8 | 1m |
| Keltner Channel | 12 | 1m |
| Momentum Divergence | 6 | 1m |
| Consecutive Candles | 8 | 1m |
| Additional mixed | 25 | 1m/5m |

**Assessment:** 105 parameter-grid variants. None individually validated. None with live PnL evidence.

---

### 2D. Intraday Strategies (65 strategies)

**Source:** `engine/internal/strategy/intraday_strategies.go`  
**Prefix:** `ID_*`  
**Timeframe:** 5m / 15m  
**SL/TP:** 0.22-0.45% SL / 0.55-1.10% TP (R:R ≥ 2.0)  

| Family | Count | Key Distinction vs 1m |
|:-------|------:|:----------------------|
| EMA Cross 5m/15m | 10 | Wider SL/TP, longer hold |
| Triple EMA | 8 | 3-EMA on 5m/15m |
| MACD | 8 | 5m/15m variants |
| VWAP | 6 | 5m VWAP cross/dev/pullback |
| Bollinger Bands | 8 | 5m/15m |
| RSI | 5 | 5m oversold |
| Keltner | 5 | 5m break/bounce |
| Stochastic | 5 | 5m cross |
| Hull MA | 5 | 5m 20-100 periods |
| CCI | 5 | 5m zero-cross/extreme |

**Assessment:** Functionally identical to Elite V2/V3 strategies — same indicators, different timeframes and adjusted SL/TP. Zero individual live PnL. Some may have edge advantage over 1m versions due to wider SL, but untested.

---

### 2E. Institutional Alpha Strategies (17 strategies)

**Source:** `engine/internal/strategy/alpha_strategies.go`  
**SL/TP:** 0.30-0.35% / 0.75-0.85% (wider, as appropriate)  
**Data sources:** Internal alpha packages  

| # | Strategy | Alpha Type | Data Status | Fires? | Live PnL |
|--:|:---------|:-----------|:------------|:------:|:---------|
| 1 | FundingMeanReversion_Alpha | Funding rate | EMPTY file | NO | $0 |
| 2 | CVDDivergence_Alpha | Cumulative Vol Delta | Partial | PARTIAL | $0 |
| 3 | DeltaAbsorption_Alpha | Delta imbalance | Dispatch bug | NO | $0 |
| 4 | LiquiditySweepReversal_Alpha | Stop-hunt sweeps | Dispatch bug | NO | $0 |
| 5 | FVGRetest_Alpha | Fair value gaps | Dispatch bug | NO | $0 |
| 6 | OrderBlockRetest_Alpha | Order blocks | Dispatch bug | NO | $0 |
| 7 | MSSContinuation_Alpha | Market structure shift | Dispatch bug | NO | $0 |
| 8 | POCBounce_Alpha | Volume profile POC | Dispatch bug | NO | $0 |
| 9 | SessionExpansion_Alpha | Session timing | Dispatch bug | NO | $0 |
| 10 | LiquidationCascade_Alpha | Liquidation events | Feed unwired | NO | $0 |
| 11-17 | Phase11_* variants | Same 7 modules unified | Mixed broken | NO | $0 |

**Assessment:** Highest-quality theoretical alpha sources. **0/17 producing any live PnL.** Infrastructure bugs prevent execution. These are the strategies most worth fixing.

---

### 2F. Expansion Pack (301 strategies)

**Source:** `engine/internal/strategy/curated_expansion_pack.go`  
**Generation method:** Procedural loops over parameter arrays  

| Family | Count | Generation Pattern |
|:-------|------:|:------------------|
| EMA cross | 40 | fast ∈ {2,3,4,5,6} × slow ∈ {8,10,12,15,18,21,26,34} |
| RSI threshold | 30 | period ∈ {7,9,14,21,28} × 6 zone configs |
| RSI slope | 20 | period × slope period combinations |
| Bollinger Bands | ~30 | Various period/multiplier combinations |
| Volume breakout | ~20 | Various threshold combinations |
| N-bar breakout | ~30 | Various N values |
| Mixed composites | ~131 | MACD+EMA, BB+RSI, etc. |

**Assessment:** **Definitionally overfit.** 301 strategies generated from nested loops over parameter spaces. No walk-forward validation. No out-of-sample testing. No individual live PnL for any. These are the primary source of aggregator noise.

---

### 2G. Client Desk Strategies (108 defined, 48 active)

**Source:** `client/src/lib/futuresStrategies.ts`  
**Asset:** BTCUSD perpetual futures  
**SL/TP:** 0.50-0.55% / 1.50-1.65% (R:R ≈ 3:1)  

| Tier | IDs | Count | Status |
|:-----|:----|------:|:-------|
| Core | 91–152 | 20 | Always active |
| Extended | 200–499 | ~28 | Active by default |
| Premium | 500–527 | 28 | Active, 2× notional |
| Research | 600–659 | 60 | `researchOnly: true` — never executes |
| Stubs | 660–759 | 40 | Empty definition arrays |

**Key characteristic:** Client desk uses **wider SL (0.50-0.55%)** vs Go engine (0.10-0.20%). Client strategies breathe through 1m noise. This is the single largest structural advantage of the client stack.

---

## 3. Dead Code Inventory

| File | Contents | Registered? | Status |
|:-----|:---------|:-----------:|:-------|
| `research_scalpers.go` | 7 research prototypes | NO | Dead |
| `external_ai.go` | AI gRPC bridge placeholder | NO | Dead |
| `profit_composites.go` | 4 composite strategies | NO | Dead |
| `moving_average.go` | Sample MovingAverageCrossover | NO | Dead |
| `scalpers_advanced.go` | 20+ microstructure strategies | PARTIAL | Check needed |

---

## 4. Key Structural Problems Identified

| Problem | Impact | Evidence |
|:--------|:-------|:---------|
| 301 procedurally generated strategies consume 49.7% of registry | Aggregator noise, zero edge | `curated_expansion_pack.go` |
| 0.10-0.20% SL inside 1m BTC ATR (typically 0.12-0.18%) | Majority of stops are noise-stops | ATR analysis |
| 6 of 17 alpha engines blocked by dispatch bug | Zero alpha throughput | `ALPHA_REPORT.md` |
| Dual independent stacks (Go ≠ Client) with no strategy overlap | Attribution confusion | `STRATEGY_INVENTORY.md` |
| No individual PnL for 592 of 606 strategies | Cannot rank or retire rationally | This audit |
| Test_Execution_Dumb_Scalper registered in production | Wastes aggregator slot | `curated_registry.go` |

---

## Phase 1 Verdict

**FAIL** on edge justification for 96.6% of strategies.  
**PASS** on enumeration completeness.

Of 714 audited strategies:
- 10 have positive live PnL evidence (1.4%)
- 4 are borderline losers still active (0.6%)
- 11 are proven losers permanently removed (1.5%)
- 689 have zero live PnL evidence (96.5%)

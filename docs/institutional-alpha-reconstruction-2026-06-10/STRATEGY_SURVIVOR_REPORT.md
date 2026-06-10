# PHASE 17 — STRATEGY SURVIVOR REPORT

**Date:** 2026-06-10  
**Method:** Evidence-based tier assignment. No assumptions. No synthetic metrics for positive tier assignment.

---

## Tier Definitions

| Tier | Criteria | Action | Capital |
|:-----|:---------|:-------|:--------|
| **A — Capital Ready** | Live positive PnL, multi-signal or statistical design, n≥5 documented | Allocate capital, validate further | Full allocation |
| **B — Paper Validation** | Live positive PnL but low n, OR institutional alpha post-fix | Paper only, gather data | Zero capital |
| **C — Experimental** | No PnL data but Tier 1/2 code quality, non-redundant signal | Paper only | Zero capital |
| **D — Retire Immediately** | Proven loser, parameter-grid overfit, redundant mechanism, no evidence | Remove from registry | Zero capital |

---

## TIER A — Capital Ready (10 strategies)

These are the only strategies with sufficient evidence to receive capital allocation.

| # | Strategy | Live PnL | Evidence Basis | Capital Tier |
|--:|:---------|:---------|:--------------|:------------|
| 1 | TripleFilter_Alpha_Scalp | +$20.00 | Multi-signal (EMA+MACD+ADX), top performer | **Full A** |
| 2 | VolumeWeighted_Trend_Scalp | +$16.00 | Volume-validated trend, second performer | **Full A** |
| 3 | EMA_Cross_Scalp | +$4.51 | Base version, simplest; holds edge at base level | **A-** |
| 4 | ZScoreBand_MeanRev_Scalp | +$4.32 | Statistical, non-indicator edge | **Full A** |
| 5 | RSI_BB_Confluence_Scalp | +$3.00 | Multi-signal confluence | **Full A** |
| 6 | OrderFlow_Pressure_Pro_Scalp | +$2.00 | Order flow (causal signal) | **Full A** |
| 7 | Stochastic_Range_Scalp | +$1.77 | Range-regime appropriate | **A-** |
| 8 | Chart_DoubleTap_Reversal_Scalp | +$1.63 | Price action (non-indicator) | **A-** |
| 9 | BollingerWalk_Trend_Scalp | positive | BB walk (continuation mechanism) | **A-** |
| 10 | LinReg_Statistical_Scalp | +$0.56 | Statistical LinReg band | **A-** |

**TIER A total: 10 strategies**  
**Capital recommendation:** 0.10-0.15 BTC each, sized by relative PnL rank  
**Condition:** SL must be reconstructed to ATR-based (Phase 6) before capital deployment

---

## TIER B — Paper Validation Required (22 strategies)

These strategies have either small live PnL or represent high-quality alpha engines that require plumbing fixes before they can be validated.

### B1 — Positive Evidence, Low Sample

| # | Strategy | Basis |
|--:|:---------|:------|
| 11 | OpeningRange_Breakout_Scalp | Aggregator boost +1.35; session-specific edge concept |
| 12 | VolSqueeze_Explosion_Scalp | Aggregator boost +1.35; volatility squeeze is legitimate mechanism |
| 13 | TrendMomentum_Score_Scalp | Aggregator boost; 5-component composite (high signal quality) |
| 14 | Bollinger_RSI_Fade_Scalp | Aggregator boost +1.30; multi-signal (BB + RSI) |

### B2 — Alpha Engines (Require Dispatch Fix)

| # | Strategy | Synthetic PF | Fix Required |
|--:|:---------|:-----------:|:------------|
| 15 | MSSContinuation_Alpha | 2.92 | OnCandle dispatch bug |
| 16 | FundingMeanReversion_Alpha | 2.09 | funding.ndjson data feed |
| 17 | OrderBlockRetest_Alpha | 1.79 | OnCandle dispatch bug |
| 18 | FVGRetest_Alpha | 1.48 | OnCandle dispatch bug |
| 19 | SessionExpansion_Alpha | — | OnCandle dispatch bug |
| 20 | LiquidationCascade_Alpha | — | Liquidation feed unwired |
| 21 | LiquiditySweepReversal_Alpha | 1.02 | OnCandle dispatch bug |
| 22 | POCBounce_Alpha | 1.19 | OnCandle dispatch bug |
| 23 | CVDDivergence_Alpha | 0.91 | Partial (barely passes quality gate) |
| 24 | Phase11_MSS | — | Duplicate of MSS, same fix |
| 25 | Phase11_FVG | — | Duplicate of FVG, same fix |
| 26 | Phase11_OrderBlock | — | Duplicate of OB, same fix |
| 27 | Phase11_LiquiditySweep | — | Duplicate of LS, same fix |
| 28 | Phase11_Funding | — | Duplicate of Funding, same fix |
| 29 | Phase11_CVD | — | Duplicate of CVD, same fix |
| 30 | Phase11_Liquidation | — | Duplicate of Liquidation, same fix |
| 31 | DeltaAbsorption_Alpha | 0.91 | OnCandle + low synthetic PF |
| 32 | SentimentConfluence_Pro_Scalp | — | High-quality design, unvalidated |

**Condition for B → A promotion:** Fix dispatch + data feeds → 30 days paper trading → n≥50 trades → PF≥1.25 live

**TIER B total: 22 strategies**

---

## TIER C — Experimental (Selected Candidates Only)

These are the best candidates from the unvalidated universe — non-redundant signal mechanisms with reasonable code quality.

| # | Strategy | Mechanism | Reason to Keep |
|--:|:---------|:----------|:--------------|
| 33 | AdaptiveRSI_Scalper | RSI with adaptive period | Adaptive mechanism — less overfit than fixed RSI |
| 34 | Momentum Divergence (MomDiv_14_8) | Price/osc divergence | Distinct mechanism |
| 35 | ATR_Momentum_Signal | ATR expansion momentum | One representative ATR signal |
| 36 | Keltner_Break_20_14_2 | Keltner channel break | One representative Keltner |
| 37 | RSI_Slope_14_5 | RSI slope reversal | More refined than RSI threshold |
| 38 | BB_Bounce_20_2 | BB touch (one variant only) | Classic mean reversion |
| 39 | VWAP_Dev_0p3 | VWAP deviation | VWAP deviation legitimate in range |
| 40 | Triple_EMA_8_21_55 | Triple EMA alignment | One representative Triple EMA |
| 41 | Hull_MA_20 | Hull MA direction | One representative |
| 42 | Stoch_Cross_14_3 | Stochastic cross (1 variant) | One representative |
| 43 | ID_EMA_9_21_5m | EMA cross on 5m | Different timeframe edge hypothesis |
| 44 | ID_BB_Bounce_20_2_5m | BB bounce on 5m | Wider SL better for this family |
| 45 | ID_MACD_Cross_12_26_9_5m | MACD on 5m | 5m MACD less lagged than 1m |
| 46 | ID_Stoch_Cross_14_3_5m | Stochastic on 5m | 5m oscillators less noisy |
| 47 | ID_Kelt_Break_20_2_5m | Keltner on 5m | 5m breakouts more meaningful |

**TIER C total: 15 strategies**  
**Action:** Run 90-day paper validation. If PF < 1.20 at end of 90 days, retire.

---

## TIER D — Retire Immediately (458+ strategies)

### D1: Expansion Pack (ALL 301)
All `XP_*` strategies. Parameter-grid overfit. Zero evidence. Zero new signal engines.  
**Action: Remove `buildExpansionPack()` call from `curated_registry.go`.**

### D2: Proven Loser Families (Retire Entire Family)
- MACD family (18): 2 documented losers; lagging indicator
- CCI family (13): No evidence; redundant
- Williams %R family (8): Redundant with Stochastic
- ROC family (8): Redundant with MACD histogram
- Parabolic SAR family (8): Whipsaw-prone
- Hull MA family beyond 1 representative (7 of 8): Lagging redundancy
- N-bar breakout (15): Breakout family documented losers
- Consecutive candles (8): Noise signal

### D3: Borderline Active Losers (Remove Immediately)
- RSI_MACD_Divergence_Scalp: -$2.06
- TripleTrend_Confluence_Scalp: -$1.43
- VWAP_RSI2_Reversion_Scalp: -$1.42
- SessionOpen_Momentum_Scalp: -$1.40
- VWAP_Bounce_Pro_Scalp: -$1.07
**Combined removal benefit: +$7.38**

### D4: Elite V2/V3 Without Evidence (Beyond Single Representatives)
All variants not selected as single representatives in Tier C. Approximately 180 strategies.
- All EMA_X_Y_Cross except representative in Tier C
- All RSI_Threshold except representative in Tier C
- All CCI (all retire — Tier D2)
- All duplicate parameter variants

### D5: Client Research Pool
- IDs 600-659 (researchOnly: true) — never executes anyway; clean registry
- IDs 660-759 (empty stubs) — remove stub definitions

**TIER D total: ~458 Go strategies + 100 client stubs**

---

## Complete Survivor Count

| Tier | Count | Action |
|:-----|------:|:-------|
| A — Capital Ready | 10 | Allocate, continue validating |
| B — Paper Validation | 22 | Fix + paper trade 30 days |
| C — Experimental | 15 | Paper trade 90 days |
| D — Retire | ~458 Go + ~100 client | Remove from registry |
| **Total retained** | **47** | **Representing 39 unique signal engines** |
| **Total retired** | **~667** | **85.6% of defined universe** |

---

## Client Desk Strategy Survival

The client 48 active strategies (Core 20 + Extended 28) are retained as a block because:
- Portfolio-level replay shows positive expectancy (+$0.91/trade)
- Wider SL geometry (0.50%+) is correct
- Individual strategy validation is needed but client desk is not the priority problem

**Recommendation:** Continue client desk as-is. Reduce leverage from 25× to 15×. Extend replay validation to 90 days minimum.

---

## Phase 17 Verdict

**10 strategies warrant capital allocation. 47 total survive after elimination. 667 should be retired.**

The platform will function better with 47 strategies than with 714. Less noise, better aggregator utilization, cleaner portfolio attribution, and faster learning cycles.

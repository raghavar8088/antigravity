# ALPHA_INVENTORY_REPORT — Phase 22C
Generated: 2026-06-04

## Scope
Complete inventory of all institutional alpha modules found in the codebase, their implementation status, and registration state before Phase 22C fixes.

---

## Module A: InstitutionalAlphaScalper (9 strategies)
File: `engine/internal/strategy/alpha_strategies.go` (lines 36–315)
File: `engine/internal/strategy/registry.go` (lines 130–139)

| Strategy Name | Module | Timeframe | Constructor | Implemented |
|---|---|---|---|---|
| FundingMeanReversion_Alpha | alphaConfluence | 1m | NewFundingMeanReversionAlpha() | ✅ Full |
| CVDDivergence_Alpha | alphaCVD | tick | NewCVDDivergenceAlpha() | ✅ Full |
| DeltaAbsorption_Alpha | alphaDelta | tick | NewDeltaAbsorptionAlpha() | ✅ Full |
| LiquiditySweepReversal_Alpha | alphaLiquidity | 1m | NewLiquiditySweepReversalAlpha() | ✅ Full |
| FVGRetest_Alpha | alphaFVG | 1m | NewFVGRetestAlpha() | ✅ Full |
| OrderBlockRetest_Alpha | alphaOrderBlock | 1m | NewOrderBlockRetestAlpha() | ✅ Full |
| MSSContinuation_Alpha | alphaMSS | 1m | NewMSSContinuationAlpha() | ✅ Full |
| POCBounce_Alpha | alphaPOC | 1m | NewPOCBounceAlpha() | ✅ Full |
| SessionExpansion_Alpha | alphaSession | 1m | NewSessionExpansionAlpha() | ✅ Full |

---

## Module B: Phase11MicrostructureAlpha (7 strategies)
File: `engine/internal/strategy/alpha_strategies.go` (lines 331–410)
File: `engine/internal/strategy/registry.go` (lines 141–148)

| Strategy Name | Kind | Timeframe | Constructor | Implemented |
|---|---|---|---|---|
| Phase11LiquiditySweepReversal_Alpha | LIQUIDITY_SWEEP_REVERSAL | 1m | NewPhase11LiquiditySweepAlpha() | ✅ Full |
| Phase11FundingMeanReversion_Alpha | FUNDING_RATE_MEAN_REVERSION | 1m | NewPhase11FundingMeanReversionAlpha() | ✅ Full |
| Phase11CVDDivergence_Alpha | CVD_DIVERGENCE | tick | NewPhase11CVDDivergenceAlpha() | ✅ Full |
| Phase11LiquidationCascadeReversal_Alpha | LIQUIDATION_CASCADE_REVERSAL | 1m | NewPhase11LiquidationCascadeAlpha() | ✅ Full |
| Phase11FairValueGap_Alpha | FAIR_VALUE_GAP_CONTINUATION | 1m | NewPhase11FVGAlpha() | ✅ Full |
| Phase11OrderBlock_Alpha | ORDER_BLOCK_RETEST | 1m | NewPhase11OrderBlockAlpha() | ✅ Full |
| Phase11MSSCHOCH_Alpha | MSS_CHOCH_RETEST | 1m | NewPhase11MSSAlpha() | ✅ Full |

---

## Alpha Engine Sub-modules (engine/internal/alpha/)

| Module | Path | Status | Key Output |
|---|---|---|---|
| CVD Engine | alpha/cvd/ | ✅ Implemented | CVD series, divergence signal |
| Delta Absorption | alpha/delta/ | ✅ Implemented | Delta accumulation/distribution signal |
| FVG Detector | alpha/fvg/ | ✅ Implemented | Gap retest signal (35%–85% fill) |
| Liquidity Sweep | alpha/liquidity/ | ✅ Implemented | Sweep+rejection signal |
| MSS/CHOCH | alpha/mss/ | ✅ Implemented | BOS/CHOCH/MSS structural shift |
| Order Block | alpha/orderblock/ | ✅ Implemented | Impulse retest signal |
| Funding | alpha/funding/ | ✅ Implemented | Mean reversion via rate + RSI |
| Session | alpha/session/ | ✅ Implemented | Asia/London/NY expansion bias |
| Liquidations | alpha/liquidations/ | ✅ Implemented | Cascade exhaustion reversal |
| Volume Profile | alpha/volumeprofile/ | ✅ Implemented | POC bounce + LVN breakout |
| Quality Engine | alpha/quality/ | ✅ Implemented | 9-component weighted score (0–100) |
| Microstructure | alpha/microstructure/ | ✅ Implemented | All-in-one Phase11 feature engine |

---

## Registration Status BEFORE Phase 22C

| Registry | Contains Alpha? | Called From |
|---|---|---|
| BuildAllScalpers() (registry.go) | ✅ Yes — all 16 entries lines 131–148 | NEVER called by main.go |
| BuildCuratedScalpers() (curated_registry.go) | ❌ NO — completely absent | main.go:415 |

**Conclusion**: All 16 alpha strategies were fully implemented but zero of them were loaded in live trading. The curated registry (the one actually used) contained only EMA/RSI/BB/VWAP/MACD indicator-based strategies.

---

## Registration Status AFTER Phase 22C

| Registry | Contains Alpha? |
|---|---|
| BuildCuratedScalpers() | ✅ YES — all 16 added (lines 377–407) |

Total strategies loaded: 589 → **605** (+16 institutional alpha)

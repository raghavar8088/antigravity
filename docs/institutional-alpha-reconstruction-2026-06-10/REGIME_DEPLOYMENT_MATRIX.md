# PHASE 9 — REGIME DEPLOYMENT MATRIX

**Date:** 2026-06-10

---

## Strategy → Regime Deployment Matrix

The following matrix defines which strategies should be allowed to trade in each market regime. Strategies not listed default to unrestricted (pending validation data).

### Color Code
- **ACTIVE**: Deploy in this regime
- **RESTRICT**: Monitor but reduce size 50%
- **BLOCKED**: Do not trade in this regime

| Strategy | Strong Trend | Trend | Weak Trend | Range | Volatile | Panic | Compression | Breakout |
|:---------|:-----------:|:-----:|:----------:|:-----:|:--------:|:-----:|:-----------:|:--------:|
| **TIER A (Proven Winners)** | | | | | | | | |
| TripleFilter_Alpha_Scalp | ACTIVE | ACTIVE | RESTRICT | BLOCKED | RESTRICT | BLOCKED | BLOCKED | RESTRICT |
| VolumeWeighted_Trend_Scalp | ACTIVE | ACTIVE | RESTRICT | BLOCKED | ACTIVE | BLOCKED | BLOCKED | ACTIVE |
| EMA_Cross_Scalp | ACTIVE | ACTIVE | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| ZScoreBand_MeanRev_Scalp | RESTRICT | ACTIVE | ACTIVE | ACTIVE | BLOCKED | BLOCKED | ACTIVE | RESTRICT |
| RSI_BB_Confluence_Scalp | RESTRICT | ACTIVE | ACTIVE | ACTIVE | BLOCKED | BLOCKED | ACTIVE | RESTRICT |
| OrderFlow_Pressure_Pro_Scalp | ACTIVE | ACTIVE | ACTIVE | RESTRICT | ACTIVE | RESTRICT | BLOCKED | ACTIVE |
| Stochastic_Range_Scalp | BLOCKED | RESTRICT | ACTIVE | ACTIVE | BLOCKED | BLOCKED | ACTIVE | BLOCKED |
| Chart_DoubleTap_Reversal_Scalp | ACTIVE | ACTIVE | ACTIVE | ACTIVE | RESTRICT | BLOCKED | RESTRICT | ACTIVE |
| LinReg_Statistical_Scalp | RESTRICT | ACTIVE | ACTIVE | ACTIVE | BLOCKED | BLOCKED | ACTIVE | RESTRICT |
| **TIER B (Promising / Boosted)** | | | | | | | | |
| OpeningRange_Breakout_Scalp | RESTRICT | ACTIVE | ACTIVE | RESTRICT | ACTIVE | BLOCKED | BLOCKED | ACTIVE |
| VolSqueeze_Explosion_Scalp | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | ACTIVE | ACTIVE |
| Bollinger_RSI_Fade_Scalp | BLOCKED | RESTRICT | ACTIVE | ACTIVE | BLOCKED | BLOCKED | ACTIVE | BLOCKED |
| TrendMomentum_Score_Scalp | ACTIVE | ACTIVE | RESTRICT | BLOCKED | ACTIVE | BLOCKED | BLOCKED | ACTIVE |
| **ALPHA (Broken — Post-Fix)** | | | | | | | | |
| MSSContinuation_Alpha | ACTIVE | ACTIVE | ACTIVE | BLOCKED | ACTIVE | BLOCKED | BLOCKED | ACTIVE |
| OrderBlockRetest_Alpha | ACTIVE | ACTIVE | ACTIVE | BLOCKED | ACTIVE | BLOCKED | BLOCKED | ACTIVE |
| FVGRetest_Alpha | ACTIVE | ACTIVE | ACTIVE | BLOCKED | RESTRICT | BLOCKED | BLOCKED | ACTIVE |
| FundingMeanReversion_Alpha | ACTIVE | ACTIVE | RESTRICT | RESTRICT | ACTIVE | ACTIVE | BLOCKED | RESTRICT |
| LiquidationCascade_Alpha | BLOCKED | BLOCKED | BLOCKED | BLOCKED | ACTIVE | ACTIVE | BLOCKED | BLOCKED |
| LiquiditySweepReversal_Alpha | ACTIVE | ACTIVE | RESTRICT | BLOCKED | ACTIVE | RESTRICT | BLOCKED | ACTIVE |
| SessionExpansion_Alpha | BLOCKED | ACTIVE | ACTIVE | RESTRICT | ACTIVE | BLOCKED | BLOCKED | ACTIVE |
| CVDDivergence_Alpha | ACTIVE | ACTIVE | ACTIVE | RESTRICT | ACTIVE | RESTRICT | BLOCKED | ACTIVE |
| **RETIRE — DO NOT DEPLOY** | | | | | | | | |
| All XP_* expansion pack | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All MACD family | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All CCI family | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All Williams %R | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All Hull MA | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All PSAR family | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All N-bar breakout | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |
| All Consecutive candles | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED | BLOCKED |

---

## Critical Regime Constraints

### Range Regime (30% of time — highest priority to gate)

BTC 1m range regime is the **most common regime and the most destructive for the current strategy mix.** PF 0.83 is confirmed in live data. All trend-following strategies MUST be blocked in range.

**Blocked in Range:** EMA family, MACD family, Triple EMA, Hull MA, Breakout family, N-bar, Consecutive candles, PSAR, Momentum score

**Active in Range:** Statistical (Z-score, LinReg), Oscillators (RSI, Stochastic — range mode), BB squeeze, VWAP deviation

---

### Panic Regime (2% of time — capital preservation priority)

During panic, most normal strategies generate whipsaw losses. Only two types should trade:
1. **Funding rate mean reversion** — panic creates extreme funding → highest expected edge
2. **Liquidation cascade reversal** — panic creates cascade events → reversal opportunity

All other strategies: BLOCKED during panic.

---

### Compression Regime (10% of time — wait for breakout)

Compression is the pre-breakout state. Most directional strategies should wait.

**Active in Compression:**
- Bollinger squeeze (`VolSqueeze_Explosion_Scalp`) — detect compression, prepare for break
- Statistical mean reversion — range-bound but tight, statistical signals still valid
- RSI/Stochastic extremes

**After Compression → Breakout transition:** Activate trend and breakout strategies immediately.

---

## Regime Capital Allocation

| Regime | Strategy Count Active | Capital Per Strategy | Rationale |
|:-------|:--------------------:|:--------------------:|:---------|
| Strong Trend | 8-10 | Full | High confidence directional |
| Trend | 12-15 | Full | Normal operating regime |
| Weak Trend | 8-10 | 75% | Reduced certainty |
| Range | 4-6 | 75% | Only range-appropriate strategies |
| Volatile | 6-8 | 50% | Higher risk; reduce size |
| Panic | 2 | 25% | Funding + Liquidation only |
| Compression | 3-4 | 50% | Waiting mode |
| Breakout | 8-10 | Full | High-confidence expansion |

---

## Implementation Notes

The current aggregator does not gate by regime. This is a direct code change to `aggregator_selective.go`:

```go
// Add to evaluateAndBoost() after signal quality check:
if !regimePermitted(signal.StrategyName, currentRegime) {
    continue // skip this signal
}
```

Regime needs to be updated each candle by the regime classifier (Phase 8) before the aggregator runs.

---

## Phase 9 Verdict

**NOT IMPLEMENTED.** The deployment matrix is designed but not wired.

Implementing regime gating is estimated to:
- Eliminate the Range regime loss cluster (PF 0.83 → 1.0+ target)
- Reduce total signal noise by 25-35%
- Improve portfolio Sharpe by reducing drawdown during unfavorable regimes
- Require 1-2 weeks implementation + 30 days live validation

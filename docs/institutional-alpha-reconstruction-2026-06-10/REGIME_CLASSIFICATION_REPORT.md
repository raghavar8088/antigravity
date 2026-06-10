# PHASE 8 — REGIME CLASSIFICATION ENGINE

**Date:** 2026-06-10

---

## Current Regime Infrastructure

The platform has regime classification in:
- `engine/internal/regime/` — Go regime classifier
- Client: `regime` field in `daily_pnl_history` MongoDB collection
- Phase 22E synthetic regime performance: only VOLATILE regime tested

**Documented regime evidence:** One regime tested synthetically (VOLATILE, 1,245 trades). All other regimes untested.

**Regime coverage gap:** Range performance PF = 0.83 (negative) documented. BULL/BEAR regime performance: completely unknown.

---

## Regime Classification Framework

### 8 Proposed Regimes

| Regime | Definition | BTC 1m Indicators | Frequency (Est.) |
|:-------|:-----------|:-----------------|:---------------:|
| **Strong Trend** | ADX > 40, directional for 20+ bars | ADX, EMA ribbon aligned | 5% |
| **Trend** | ADX 25-40, consistent HL/HH or LL/LH | ADX 25-40, EMA slope | 15% |
| **Weak Trend** | ADX 18-25, slight directional bias | ADX 18-25, partial EMA alignment | 20% |
| **Range** | ADX < 18, oscillating between defined levels | ADX < 18, BB squeeze, low ATR | 30% |
| **Volatile** | ATR > 2× 20-day average, high amplitude moves | ATR ratio, large candle bodies | 10% |
| **Panic** | ATR > 4× 20-day average, extreme unidirectional | Spike ATR, high volume | 2% |
| **Compression** | ATR < 0.5× 20-day average, Bollinger squeeze | BB width < 1%, ATR at multi-day low | 10% |
| **Breakout** | Exit from Compression + volume confirmation | BB squeeze followed by expansion | 8% |

---

## Current Regime Performance Evidence

| Regime | Strategy Count Tested | PF (Synthetic) | PF (Live) | Evidence |
|:-------|:--------------------:|:--------------:|:---------:|:---------|
| VOLATILE | 12 strategies, 1,245 trades | 1.49 | UNKNOWN | Phase 22E only |
| RANGE | 5 trades documented | — | 0.83 | Live evidence |
| BULL | 0 strategies | UNKNOWN | UNKNOWN | No data |
| BEAR | 0 strategies | UNKNOWN | UNKNOWN | No data |
| CHOP/Compression | 0 strategies | UNKNOWN | UNKNOWN | Inferred poor |
| Breakout | Removed strategies | — | NEGATIVE | 5 removed losers |
| Panic | 0 strategies | UNKNOWN | UNKNOWN | No data |
| Strong Trend | 0 strategies | UNKNOWN | UNKNOWN | No data |

**Critical finding:** The platform has operated primarily in VOLATILE regime conditions. Range regime performance is confirmed negative (PF 0.83). No other regime data exists.

---

## Regime-Strategy Fitness Analysis (Theoretical, No Live Validation)

### Which Strategies Work in Which Regimes

| Strategy Family | Strong Trend | Trend | Weak Trend | Range | Volatile | Panic | Compression | Breakout |
|:----------------|:-----------:|:-----:|:----------:|:-----:|:--------:|:-----:|:-----------:|:--------:|
| EMA crossover | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ❌ | ❌ | ⚠️ |
| RSI threshold | ⚠️ | ⚠️ | ✅ | ✅ | ⚠️ | ❌ | ✅ | ⚠️ |
| Statistical (Z-score) | ⚠️ | ✅ | ✅ | ✅ | ⚠️ | ❌ | ✅ | ⚠️ |
| VWAP mean reversion | ⚠️ | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ⚠️ |
| Bollinger squeeze | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ | ✅ |
| Volatility expansion | ✅ | ✅ | ⚠️ | ❌ | ✅ | ✅ | ❌ | ✅ |
| MSS/FVG/OrderBlock | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |
| Funding mean rev | ✅ | ✅ | ⚠️ | ⚠️ | ✅ | ✅ | ❌ | ⚠️ |
| Liquidation cascade | ⚠️ | ⚠️ | ❌ | ❌ | ✅ | ✅ | ❌ | ⚠️ |
| Session expansion | ⚠️ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ❌ | ✅ |
| Order flow | ✅ | ✅ | ✅ | ⚠️ | ✅ | ⚠️ | ⚠️ | ✅ |
| Chart patterns | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ | ⚠️ | ✅ |

Legend: ✅ Expected positive | ⚠️ Uncertain | ❌ Expected negative

---

## Regime Classifier Implementation Requirement

### Inputs Required (per-candle)

```go
type RegimeInputs struct {
    ADX14         float64  // trend strength
    ATR14         float64  // current volatility
    ATR14_20d_avg float64  // 20-day average ATR for ratio
    BBWidth        float64  // Bollinger Band width %
    EMASlope_fast  float64  // EMA(9) slope
    EMASlope_slow  float64  // EMA(21) slope
    VolumeRatio    float64  // current vol / 20-bar avg vol
}

type Regime int
const (
    RegimeStrongTrend Regime = iota
    RegimeTrend
    RegimeWeakTrend
    RegimeRange
    RegimeVolatile
    RegimePanic
    RegimeCompression
    RegimeBreakout
)
```

### Classification Logic (Priority Order)

```go
func ClassifyRegime(r RegimeInputs) Regime {
    atrRatio := r.ATR14 / r.ATR14_20d_avg
    
    // Panic: extreme volatility
    if atrRatio > 4.0 {
        return RegimePanic
    }
    // Volatile: elevated volatility
    if atrRatio > 2.0 {
        return RegimeVolatile
    }
    // Compression: very low volatility
    if atrRatio < 0.5 && r.BBWidth < 0.8 {
        return RegimeCompression
    }
    // Breakout: exiting compression with volume
    if atrRatio > 1.2 && r.VolumeRatio > 1.5 {
        return RegimeBreakout
    }
    // Trend-based by ADX
    if r.ADX14 > 40 {
        return RegimeStrongTrend
    }
    if r.ADX14 > 25 {
        return RegimeTrend
    }
    if r.ADX14 > 18 {
        return RegimeWeakTrend
    }
    return RegimeRange
}
```

---

## Regime Gate Implementation

After regime detection, strategies should be gated:

```go
// Per-strategy regime permission map
var regimeGates = map[string][]Regime{
    "EMA_Cross_Scalp":           {RegimeStrongTrend, RegimeTrend},
    "ZScoreBand_MeanRev_Scalp":  {RegimeRange, RegimeWeakTrend, RegimeTrend},
    "VolSqueeze_Explosion_Scalp":{RegimeCompression, RegimeBreakout},
    "LiquidationCascade_Alpha":  {RegimeVolatile, RegimePanic},
    "FundingMeanReversion_Alpha":{RegimeStrongTrend, RegimeTrend, RegimeVolatile},
}

func (a *Aggregator) gateByRegime(signal Signal, currentRegime Regime) bool {
    allowed, exists := regimeGates[signal.StrategyName]
    if !exists {
        return true // default: no gate
    }
    for _, r := range allowed {
        if r == currentRegime {
            return true
        }
    }
    return false
}
```

---

## Expected Regime Impact

When regime gating is applied:
- Range-incompatible strategies (EMA crossover) blocked during 30% of time → expected -30% false signals
- Breakout strategies (compressed → expansion) only fire at correct time → expected +0.5 PF
- Liquidation/panic strategies only fire during extreme moves → fewer trades, higher quality
- TOTAL expected improvement in portfolio PF: +0.20–0.40

---

## Phase 8 Verdict

**PARTIAL — regime infrastructure exists but is unused for strategy gating.**

The platform's most documented regime finding: **Range regime produces PF 0.83** on current strategy mix. This means the platform loses money 30% of the time (range frequency ~30%). Blocking all non-range-appropriate strategies during Range regime would eliminate this loss cluster.

**Priority implementation:**
1. Wire `ClassifyRegime()` output to aggregator gate
2. Assign regime permissions to top 20 surviving strategies
3. Log regime for every candle close for post-trade analysis
4. After 30 days: compute actual per-regime PF to validate theoretical assignments

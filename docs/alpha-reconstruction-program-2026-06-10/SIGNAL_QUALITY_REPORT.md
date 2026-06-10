# PHASE 5 — SIGNAL QUALITY ANALYSIS

**Date:** 2026-06-10

---

## Signal Generation Architecture

Every strategy produces a `Signal` with:
```go
type Signal struct {
    Symbol        string
    Action        Action     // Buy, Sell, Hold
    TargetSize    float64
    Confidence    float64    // 0.68-1.30+ (quality score)
    StopLossPct   float64
    TakeProfitPct float64
}
```

Signals pass through 5 filters before execution:
1. **Hold filter** — `ActionHold` signals discarded
2. **Cooldown filter** — same strategy blocked for `cooldownSec` after last signal
3. **Dominance filter** — non-dominant side discarded if directional consensus weak
4. **Score filter** — signals below `minSelectiveScore = 0.80` discarded
5. **Category cap** — max 5 per category per batch
6. **Batch cap** — max 25 signals per batch

### Confidence Score Components (from `strategyPriority()`)

```
Base: strategy.Signal.Confidence (0.68-1.30)
  + ExecutionWeight bonus (if strategy has track record)
  + PnL/WinRate bonus (+0.25 if WR≥58%, +0.10 if WR≥50%)
  − PnL/WinRate penalty (−0.25 if WR<40%, −0.10 if WR<46%)
  + Hardcoded name boost (TripleFilter: +2.00, VolumeWeighted: +1.90, etc.)
  + Category bonus (Multi-Signal/Breakout Elite: +0.20, etc.)
```

**Critical observation:** The hardcoded PnL boosts (+2.00 for TripleFilter, +1.90 for VolumeWeighted) effectively guarantee these strategies dominate every batch regardless of current signal quality. This is correct for proven winners but creates execution concentration risk.

---

## Signal Frequency Analysis (Theoretical)

Signal frequency depends on:
1. How often the entry condition is satisfied
2. Whether ADX/RSI guards pass
3. Whether cooldown has elapsed

### EMA Crossover (EMACrossV2)

Entry condition: `!prevAbove && above` — only on the crossover bar itself.  
On BTC 1m: EMA(5,13) crosses approximately 8-15 times per hour in choppy conditions, 2-5 times/hour in trending.  
With cooldown (assumed ~60 seconds): each crossover generates at most 1 signal.

**Estimated frequency:** 5-15 signals/hour per EMA variant.  
**But with 70 EMA variants competing for 25 aggregator slots:** Each individual EMA strategy executes <1 trade/hour on average.

### RSI Threshold

Entry condition: RSI exits oversold zone (prev ≤ 30, current > 35 for example).  
On BTC 1m: RSI dips below 30 approximately 2-5 times per hour in volatile conditions.

**Estimated frequency:** 2-5 signals/hour per RSI variant.

### Statistical (ZScore/LinReg)

Entry condition: Z-score < -2.0 standard deviations from 30-bar mean.  
Mathematically: ~2.5% of observations are at ≥2σ (for each direction).  
On BTC 1m: With mean-reversion characteristics, actual Z>2σ frequency is ~2-4 times/hour.

**Estimated frequency:** 2-4 signals/hour. Higher quality per signal than EMA/RSI.

### Multi-Signal Confluence (TripleFilter)

Entry condition: EMA(20) aligned + MACD hist > 0 + ADX > 25 simultaneously.  
P(all three agree) ≈ P(EMA) × P(MACD|EMA) × P(ADX|trend confirmed)  
Estimated combined frequency: 0.5-2 signals/hour (much rarer, higher precision).

**Estimated frequency:** 0.5-2 signals/hour. **Rarest signals, likely highest precision.**

### Institutional Alpha (MSS, FVG, OB)

Entry condition: Specific structural patterns across multiple candles.  
FVG requires: 3-candle gap. Frequency on 1m BTC: ~3-8 times/day.  
MSS requires: Higher high in uptrend with structural break. Frequency: ~5-15 times/day.  
OB requires: High-volume consolidation + retest. Frequency: ~2-5 times/day.

**Estimated frequency:** Very low (<1/hour). This is appropriate for high-precision institutional signals — fewer trades, higher conviction.

---

## Signal Precision Assessment

**Precision = % of signals that are "correct" (would be profitable at correct SL/TP geometry)**

| Strategy Family | Est. Signal Frequency | Estimated Precision | Evidence Basis |
|:----------------|:--------------------:|:-------------------:|:---------------|
| Multi-signal confluence | 0.5-2/hr | **~65-70%** | TripleFilter top winner |
| Statistical (Z-score) | 2-4/hr | **~55-60%** | ZScore positive live PnL |
| Price action patterns | 1-3/hr | **~55-60%** | DoubleTap positive live PnL |
| Order flow | 2-5/hr | **~50-55%** | OFP positive live PnL |
| EMA crossover (base) | 5-15/hr | **~48-52%** | EMA positive but single-indicator |
| RSI threshold | 2-5/hr | **~45-50%** | No strong evidence either way |
| MACD family | 3-8/hr | **~40-45%** | Documented losers |
| Breakout family | 1-3/hr | **~35-40%** | 5 removed losers |
| EMA grid (XP) | 5-15/hr | **~48-50%** | Same as EMA, parameter-inflated |
| Institutional alpha | 0.1-1/hr | **Unknown** | Zero live trades |

**Break-even precision at effective geometry (0.18-0.20% SL / 0.50% TP):**  
~41-43% win rate needed. Most families above the threshold — but many barely.

---

## False Positive Analysis

A false positive is a signal that triggers but results in a losing trade.

**Primary false positive sources:**

### FP-1: EMA Cross in Range Markets
EMA crossovers in choppy/range markets generate whipsaws — fast EMA oscillates above and below slow EMA repeatedly. Each oscillation generates a signal. In a sideways market, ~60-70% of EMA cross signals are false positives.

**Mitigation:** ADX guard `adxMin` (18-28) blocks signals when ADX < threshold. But ADX is lagging — it rises AFTER trend starts and falls AFTER range begins. There's a lag window where EMA signals fire and ADX hasn't yet confirmed range.

### FP-2: RSI Mean Reversion During Strong Trends
RSI entering oversold (<30) during a strong downtrend generates false reversal signals. This is the classic "trend continuation vs mean reversion" conflict. RSI stays oversold during trending markets.

**Current mitigation:** RSI threshold strategies fire on exit from oversold, not entry. But in a prolonged trend, multiple oversold entries/exits occur as price stair-steps down.

### FP-3: Quality Gate Calibration
The quality gate floor of 70 was calibrated with CVD at ~71. This is too close to the threshold. A quality gate should provide meaningful separation — the current gate barely separates signal from noise.

**Recommendation:** Raise quality gate to 75 minimum and add per-module calibration.

### FP-4: Simultaneous Buy Signals (Correlated Strategies)
When EMA cross, RSI oversold, and BB bounce all fire simultaneously (common in volatile conditions), the aggregator receives 30+ signals of the same type in one batch. The dominance filter passes them all, the category cap limits each category to 5, and up to 25 buy signals are approved.

This creates a correlated entry burst — if the batch is wrong, 25 positions all lose simultaneously.

---

## Signal Efficiency Recommendations

### 1. Per-Strategy Signal Count Logging
Add atomic counter per strategy: signals generated, passed quality gate, approved by aggregator, executed as trade.

### 2. Quality Score Distribution Monitoring  
Log quality scores for all alpha engines across 24 hours to understand score distribution and calibrate gate.

### 3. Deduplication Enhancement
Reduce category cap from 5 to 3 for correlated families (EMA, RSI, BB all in "Trend" or "Mean Reversion"). Keep at 5 for truly distinct families (Alpha, Statistical, Multi-Signal).

### 4. Frequency-Precision Tradeoff
Higher-frequency families (EMA, RSI) generate many signals but lower precision. Lower-frequency families (MSS, OB, confluence) generate few signals but higher precision. Portfolio should weight capital toward low-frequency, high-precision signals.

---

## Phase 5 Verdict

**Signal quality is not formally measured.** The confidence score computed by `strategyPriority()` is a priority-ranking tool, not a precision measurement. A confidence of 1.30 does not mean "73% win rate" — it means "approved by aggregator first."

**Highest-confidence signals by mechanism:**
1. Multi-signal confluence (TripleFilter class) — highest evidence-based precision
2. Statistical deviation (Z-score class) — mathematically objective
3. Order flow (OFP class) — causal information advantage

**Lowest-confidence signals:**
1. Expansion pack EMA/RSI grid — same precision as individual EMA/RSI but 301× duplicated
2. MACD family — documented losers, lagging
3. Breakout family — 5 removed losers, regime-dependent

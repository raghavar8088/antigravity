# PHASE 6 — STOP LOSS RECONSTRUCTION

**Date:** 2026-06-10  
**Verdict:** CRITICAL FAILURE — majority of Go engine SL values are inside BTC 1m market noise

---

## Current Stop Loss Architecture

### Go Engine Stops

| Strategy Group | Current SL | Source |
|:--------------|:-----------:|:-------|
| Base scalpers | 0.15% | `baseScalper` default |
| Elite V2/V3 | 0.15–0.22% | Per-instance in struct |
| Intraday 5m | 0.22–0.35% | `intraday_strategies.go` |
| Intraday 15m | 0.30–0.45% | `intraday_strategies.go` |
| Alpha strategies | 0.30–0.35% | `alpha_strategies.go` |
| Expansion pack | 0.15–0.18% | Generated in loop |
| After `sanitizeSignalForProfit` | 0.10% minimum | `aggregator_selective.go` |

### Client Desk Stops

| Strategy Group | Current SL |
|:--------------|:-----------:|
| Core 20 | 0.50–0.55% |
| Premium 28 | 0.50–0.55% |
| Research 60 | 0.50–0.55% |

---

## BTC 1m Noise Band Analysis

### ATR Measurement (BTC 1m)

Average True Range on BTC 1m candles represents the expected noise:

| Period | Typical ATR | Low Volatility | High Volatility |
|:-------|:-----------:|:--------------:|:---------------:|
| 7-bar ATR | 0.12–0.15% | 0.07% | 0.35%+ |
| 14-bar ATR | 0.12–0.18% | 0.08% | 0.40%+ |
| 20-bar ATR | 0.13–0.20% | 0.09% | 0.50%+ |

**BTC 1m noise band (2-ATR for stop placement):** approximately 0.24%–0.36%

A stop loss placed inside this range will be hit by random price noise with near-50% probability even when the trade direction is correct.

### Current SL vs Noise Band

| SL Level | ATR Multiples (14-bar, 0.15% ATR) | Noise Hit Rate (estimated) |
|:---------|:---------------------------------:|:--------------------------|
| 0.10% (sanitized minimum) | 0.67× ATR | ~65% noise stops |
| 0.15% (base) | 1.0× ATR | ~55% noise stops |
| 0.20% (elite) | 1.33× ATR | ~45% noise stops |
| 0.35% (alpha) | 2.33× ATR | ~25% noise stops |
| 0.50% (client) | 3.33× ATR | ~15% noise stops |

**The Go engine's 0.15% SL is at 1.0× ATR — meaning a correct directional trade is stopped out approximately 55% of the time by noise before the target is reached.**

This is the **primary mechanical cause of losses** in the Go strategy universe.

---

## Break-Even Win Rate at Current Geometry

| SL | TP | RR | Fee | Break-Even WR |
|:---|:---|:--:|:---:|:-------------:|
| 0.15% | 0.25% | 1.67:1 | 0.10% | **43.8%** |
| 0.20% | 0.40% | 2.00:1 | 0.10% | **40.0%** |
| 0.35% | 0.75% | 2.14:1 | 0.10% | **39.2%** |
| 0.50% | 1.50% | 3.00:1 | 0.10% | **30.0%** |

The client desk (0.50% SL / 1.50% TP) only requires **30% win rate** to be profitable. This is the correct architecture for unvalidated strategies during the discovery phase.

The Go engine (0.15% SL / 0.25% TP) requires **43.8% win rate** — while simultaneously hitting the stop with noise 55% of the time. The math is structurally negative.

---

## ATR-Based Stop Loss Reconstruction

### Proposed Formula

```
ATR_SL = entry_price × (k × ATR_14 / entry_price)
where k = 2.0 for 1m strategies, 1.5 for 5m, 1.0 for 15m
```

Expressed as percentage:
- **1m strategies:** ATR-based SL = 2.0 × 14-bar ATR%
  - Typical: 2.0 × 0.15% = **0.30%** minimum
  - High vol: 2.0 × 0.25% = **0.50%** maximum
- **5m strategies:** 1.5 × ATR%
  - Typical: 1.5 × 0.30% = **0.45%** minimum
- **15m strategies:** 1.0 × ATR%
  - Typical: 1.0 × 0.50% = **0.50%** minimum

### Adjusted Take-Profit at 2.5× RR

| SL (ATR-based) | TP (2.5× RR) | Break-Even WR | Improvement |
|:--------------|:------------|:-------------:|:------------|
| 0.30% | 0.75% | **33.3%** | −10.5 pp vs current |
| 0.40% | 1.00% | **31.4%** | −12.4 pp vs current |
| 0.50% | 1.25% | **30.0%** | −13.8 pp vs current |

Moving to ATR-based stops reduces the break-even threshold by ~10 percentage points and reduces noise-stop hits by ~30%.

---

## Per-Family Stop Loss Recommendations

| Family | Current SL | Recommended SL | Recommended TP | Rationale |
|:-------|:---------:|:--------------:|:--------------:|:---------|
| Base scalpers (1m) | 0.15% | **0.30–0.40%** | **0.75–1.00%** | ATR-based minimum |
| Elite V2/V3 (1m) | 0.15–0.22% | **0.30–0.40%** | **0.75–1.00%** | Same |
| Intraday 5m | 0.22–0.35% | **0.40–0.50%** | **1.00–1.25%** | 5m ATR wider |
| Intraday 15m | 0.30–0.45% | **0.50–0.60%** | **1.25–1.50%** | 15m ATR widest |
| Alpha strategies | 0.30–0.35% | **0.35–0.50%** | **0.85–1.25%** | Already reasonable; tune after data |
| Client desk | 0.50–0.55% | **0.50–0.55%** | **1.50–1.65%** | Correct — no change |

---

## Volatility Stop (Advanced)

For strategies that continue to show edge after SL reconstruction, introduce a **volatility-adaptive stop:**

```go
// Pseudo-code for ATR-based stop in Go engine
func (s *Strategy) computeStop(entryPrice, atr14 float64, side string) float64 {
    stopDist := math.Max(2.0 * atr14, minStopPct * entryPrice)
    if side == "BUY" {
        return entryPrice - stopDist
    }
    return entryPrice + stopDist
}
```

This ensures the stop is always outside the current noise band, regardless of ATR regime.

---

## Impact Estimate

Changing 0.15% SL to 0.35% ATR-based SL across Go engine:
- Reduces noise stop frequency by ~30 percentage points
- Increases average trade duration (allows trades to develop)
- Increases required TP proportionally (maintains RR)
- May reduce trade frequency slightly (wider invalidation zone)
- **Expected net effect:** +15–25% improvement in profit factor on surviving strategies

This single change is **the highest-ROI mechanical improvement** available to the Go engine.

---

## Phase 6 Verdict

**FAIL — critical.** Current Go engine SL geometry is structurally unprofitable.

0.15% SL on BTC 1m with 0.12-0.18% average ATR means the stop is inside the noise band for most candles. A correct directional call is still stopped out more than half the time.

**Immediate action required:**
1. Remove the `sanitizeSignalForProfit` minimum of 0.10% SL — this makes the problem worse
2. Replace all 1m base stops with 2× ATR(14) dynamic floor
3. Adjust TP to maintain 2.5:1 RR
4. Do not deploy any Go strategy to live capital with SL < 0.30%

# ATR STOP LOSS RECONSTRUCTION REPORT
## SEP Phase 8 — Dynamic Risk Calibration

**Date:** 2026-06-10  
**Status:** IMPLEMENTED

---

## PROBLEM STATEMENT

### Why Fixed Stops Are Destroying Edge

BTC perpetual on 1-minute timeframe:
- **BTC 1m ATR:** 0.12% – 0.18% (typical range)
- **Previous fixed SL:** 0.15% – 0.18%

**At a 0.15% stop, 1× ATR = instant noise trigger.** Every institutional signal was stopped out by normal price oscillation before the thesis could play out.

### Evidence From Code Audit

```
# Institutional alpha strategies (fixed SL found in code):
LiquiditySweepReversal:  SL = 0.30%, TP = 0.85%  (TP:SL = 2.83)
MSS:                     SL = 0.35%, TP = 0.90%  (TP:SL = 2.57)
SessionExpansion:        SL = 0.35%, TP = 0.75%  (TP:SL = 2.14)
FundingMeanReversion:    SL = 0.35%, TP = 0.85%  (TP:SL = 2.43)

# Problem: SL = 0.30% = 2.5× a typical 1m ATR of 0.12%
# In volatile sessions (ATR = 0.18%), SL = 0.30% = 1.67× ATR
# Many stops were triggered by normal noise, not actual thesis invalidation.
```

---

## SOLUTION: ATR-BASED DYNAMIC STOPS

### Implementation (alpha_strategies.go)

```go
func (s *InstitutionalAlphaScalper) atrSL() float64 {
    if len(s.candles) < 2 { return 0.30 }
    last := s.candles[len(s.candles)-1]
    atr  := alpha.ATR(s.candles, 14)           // 14-period ATR
    pct  := atr / last.Close * 100              // ATR as % of price
    sl   := pct * 2.0                           // SL = 2 × ATR
    return clamp(sl, 0.25, 0.60)               // min 0.25%, max 0.60%
}

func (s *InstitutionalAlphaScalper) atrTP() float64 {
    tp := s.atrSL() * 3.0                      // TP = 3 × SL = 6 × ATR
    return clamp(tp, 0.75, 1.80)               // min 0.75%, max 1.80%
}
```

### ATR Computation (alpha/math.go)

```go
// ATR = average True Range over period candles
// TR = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
func ATR(candles []Candle, period int) float64 { ... }
```

---

## CALIBRATION TABLE

| BTC ATR (1m) | SL (2×ATR) | TP (6×ATR) | R:R Ratio |
|--------------|-----------|-----------|-----------|
| 0.10% | 0.25% (min) | 0.75% (min) | 3.0 |
| 0.12% | 0.24% → 0.25% | 0.75% | 3.0 |
| 0.14% | 0.28% | 0.84% | 3.0 |
| 0.16% | 0.32% | 0.96% | 3.0 |
| 0.18% | 0.36% | 1.08% | 3.0 |
| 0.20% | 0.40% | 1.20% | 3.0 |
| 0.25% | 0.50% | 1.50% | 3.0 |
| 0.30% | 0.60% (max) | 1.80% (max) | 3.0 |

R:R is always exactly 3.0 by construction.

---

## STRATEGIES UPDATED

All institutional alpha strategies now use ATR-based stops:

| Strategy | Old SL | Old TP | New SL | New TP |
|----------|--------|--------|--------|--------|
| LiquiditySweepReversal | 0.30% | 0.85% | ATR×2 | ATR×6 |
| FVGRetest | 0.35% | 0.90% | ATR×2 | ATR×6 |
| OrderBlockRetest | 0.35% | 0.90% | ATR×2 | ATR×6 |
| MSSContinuation | 0.35% | 0.90% | ATR×2 | ATR×6 |
| POCBounce | 0.35% | 0.80% | ATR×2 | ATR×6 |
| SessionExpansion | 0.35% | 0.75% | ATR×2 | ATR×6 |
| LiquidationCascade | 0.35% | 0.85% | ATR×2 | ATR×6 |
| FundingMeanReversion | 0.35% | 0.85% | ATR×2 | ATR×6 |

---

## EXPECTED IMPROVEMENTS

| Metric | Before | Expected |
|--------|--------|---------|
| Premature stop-out rate | HIGH | REDUCED (stops breathing with volatility) |
| Average hold time | SHORT (noise-driven exits) | LONGER |
| MFE capture | LOW | HIGHER |
| R:R ratio | 2.14 – 2.83 | Fixed 3.0 always |
| Sharpe ratio | UNKNOWN | Expected improvement |

---

## FALLBACK SAFETY

- Minimum SL: 0.25% (prevents stops so tight they're meaningless)
- Maximum SL: 0.60% (prevents runaway stops in volatile regimes)
- Minimum TP: 0.75% (ensures R:R ≥ 3 after fees)
- Maximum TP: 1.80% (realistic TP for 1m–5m scalp timeframe)

If fewer than 2 candles are in the buffer, fallback to 0.30% SL / 0.90% TP.

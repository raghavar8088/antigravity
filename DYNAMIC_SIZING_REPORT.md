# DYNAMIC SIZING REPORT — Phase 22B

**Date:** 2026-06-04  
**File:** `engine/internal/risk/v2/dynamic_sizing.go` — `DynamicSize()`

---

## How DynamicSize Works

`DynamicSize` applies a cascade of multiplicative adjustments to the base position size. Each layer either reduces or keeps the size. The final `RecommendedSizeBTC` is the product of all multipliers applied.

```
Base size = req.RequestedSizeBTC (or $1000/price fallback)

Layer 1: strategy_health   — if HealthScore < 50         → ×0.50
Layer 2: sharpe            — if Sharpe > 0 AND < 1.0     → ×0.75
Layer 3: profit_factor     — if PF > 0 AND < 1.2         → ×0.50
Layer 4: drawdown          — from EvaluateDrawdown()      → ×0–1.0
Layer 5: portfolio_heat    — from CalculateHeat()         → ×0–1.0
Layer 6: volatility        — HIGH_VOL or vol ≥ 3%         → ×0.50
Layer 7: funding           — extreme funding rate          → ×0.75
Layer 8: correlation       — MaxCorr ≥ 0.70               → ×0.50
Layer 9: tail_risk         — REDUCE_RISK action            → ×0.25
                           — HALT or CLOSE action          → ×0.00
```

**Before Phase 22B:** Layers 1–3 (strategy health, Sharpe, profit factor) always received hardcoded values that were tuned to avoid triggering any reduction. With Sharpe=1.2 (≥1.0) and PF=1.2 (≥1.2), Layers 2 and 3 never fired. With HealthScore=70 (≥50), Layer 1 never fired. **Three of the nine sizing layers were permanently disabled.**

**After Phase 22B:** Layers 1–3 receive real measured values. A strategy with HealthScore=35, Sharpe=0.4, PF=0.9 now triggers all three:
- Layer 1: ×0.50
- Layer 2: ×0.75  
- Layer 3: ×0.50
- Combined: ×0.1875 → Size10 scale (near-disabled) ✓

---

## Fixed Position Size Analysis

### Pre-Fix
Every strategy executed at exactly `fixedTradeCapitalUSD = $10,000` (1% of $1M paper account), regardless of performance. This was enforced by:
1. The Kelly output being discarded (RecommendedSizeBTC never applied)
2. The DynamicSize output being discarded (same reason)

The `sig.TargetSize` entering `ExecuteSignal` was always the base 1% size.

### Post-Fix
```
Base size  = $10,000 worth of BTC (fixed 1% capital rule — unchanged)
Kelly size = base × Kelly fraction (based on real WinRate, PF, AvgWin/Loss)
Dynamic    = Kelly size × product of all multipliers
Final      = min(Kelly, Dynamic), applied to sig.TargetSize before execution
```

**Winning strategies** (high WinRate, high PF, good Sharpe, low drawdown):
- Kelly fraction near maxRiskPct (2%) → Kelly size ≈ base or slightly reduced
- Dynamic multipliers mostly 1.0 → minimal reduction
- Final size close to base ✓

**Losing strategies** (low WinRate, PF < 1.0):
- Kelly fraction = 0 → Kelly size = 0 → blocks trade entirely ✓

**Marginal strategies** (WinRate 50%, PF 1.1):
- Kelly fraction small → reduced size
- Dynamic: PF < 1.2 → ×0.50 additional
- Final size significantly reduced ✓

**Unproven strategies** (< 30 trades):
- Kelly: stability ≈ 0 → quarter-Kelly selected
- Allocation: recommended × 0.50 (TotalTrades < 30 cold-start penalty)
- Capital constrained without permanently suppressing ✓

---

## Dynamic Sizing Activation Status

| Layer | Pre-Fix Active? | Post-Fix Active? | Condition |
|---|---|---|---|
| strategy_health | Never (score=70) | When HealthScore < 50 | Real measured score |
| sharpe | Never (1.2≥1.0) | When Sharpe < 1.0 | Real measured Sharpe |
| profit_factor | Never (1.2≥1.2) | When PF < 1.2 | Real measured PF |
| drawdown | Active (from portfolio equity) | Active | Portfolio-level, unchanged |
| portfolio_heat | Active | Active | Portfolio-level, unchanged |
| volatility | Active | Active | Market state, unchanged |
| funding | Active | Active | Market state, unchanged |
| correlation | Active | Active | Portfolio positions, unchanged |
| tail_risk | Active | Active | Market state, unchanged |

**Net effect:** 3 previously dead layers now alive.

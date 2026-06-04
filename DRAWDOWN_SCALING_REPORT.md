# DRAWDOWN SCALING REPORT — Phase 22B

**Date:** 2026-06-04

---

## Two Layers of Drawdown-Aware Scaling

### Layer 1: Portfolio-Level Drawdown (`engine/internal/risk/v2/drawdown.go`)

This layer operates on the paper account's equity curve and was **already active before Phase 22B**.

```
Portfolio Drawdown = (HighWatermark – CurrentEquity) / HighWatermark × 100

DD ≥ 2%   → SizeMultiplier = 0.75  (reduce size 25%)
DD ≥ 4%   → SizeMultiplier = 0.50  (reduce size 50%)
DD ≥ 6%   → SizeMultiplier = 0.25  (reduce size 75%)
DD ≥ 8%   → SizeMultiplier = 0.10  (elite strategies only)
DD ≥ 10%  → TradingHalt = true     (all trading stopped)
```

This multiplier feeds into `DynamicSize` as Layer 4. It was active but operating on dummy metrics for Layers 1–3.

### Layer 2: Per-Strategy Drawdown (`StrategyStats.MaxDrawdownPct` → `BuildRiskMetrics`)

**New in Phase 22B.** Each strategy now has its own `MaxDrawdownPct` computed from its trade history.

```
PeakTotalPnL = max PnL ever achieved by this strategy
DrawdownPct  = (PeakTotalPnL – CurrentTotalPnL) / max(Allocation, 1000) × 100
MaxDrawdownPct = max(MaxDrawdownPct, DrawdownPct)  ← updated on every trade
```

This flows into Risk V2 via three paths:

**Path A — Kelly `ddRisk`:**
```go
// v2/kelly.go:54
ddRisk := clamp(metrics.MaxDrawdownPct/10, 0, 1)
if stability < 0.45 || ddRisk > 0.6 {
    selected = quarter  // reduce from half-Kelly to quarter-Kelly
}
```
A strategy with MaxDrawdownPct > 6% is forced to quarter-Kelly.

**Path B — Allocation `ddScore`:**
```go
// v2/allocation.go:22
ddScore := clamp(1 - metrics.MaxDrawdownPct/15, 0, 1)
```
A strategy with MaxDrawdownPct = 15% receives ddScore = 0 (worst possible allocation score for this component).

**Path C — Health score:**
```go
// strategy_tracker.go:trackerHealthScore
score -= math.Min(30, s.MaxDrawdownPct * 2)
```
A 15% drawdown deducts 30 points from the health score.

---

## Consecutive Loss Scaling

### StrategyTracker Level (pre-existing, unchanged)

```go
if s.ConsecutiveLosses >= maxConsecutiveLosses {  // 5 losses
    disableStrategy(...)   // COOLDOWN for 10 minutes
}
```

### Health Score Integration (new in Phase 22B)

```go
score -= math.Min(15, float64(s.ConsecutiveLosses) * 3)
```

A strategy on a 5-loss streak loses 15 health points. This flows into:
- DynamicSize Layer 1 (health < 50 → ×0.50)
- Allocation healthScore component

### Kelly Stability Factor (new connection)

```go
// v2/kelly.go:52
stability := clamp(float64(metrics.TotalTrades)/100, 0,1) × clamp(metrics.ProfitFactor/1.5, 0,1)
```

A strategy with PF < 1.5 due to a losing streak reduces its `stability` which may push sizing to quarter-Kelly.

---

## Profit Factor Collapse Scaling

When a strategy's profit factor collapses below 1.2 (e.g. after a run of bad trades):

```
DynamicSize Layer 3: PF < 1.2 → ×0.50
```

When PF collapses below 1.0:
```
Kelly: b×p–q < 0 → f* = 0 → sizeBTC = 0 → trade blocked
```

The system automatically reduces and eventually blocks a strategy whose PF is deteriorating.

---

## Drawdown Scaling Activation Status

| Mechanism | Data Source | Pre-Fix | Post-Fix |
|---|---|---|---|
| Portfolio drawdown halt | Account equity HWM | Active | Active (unchanged) |
| Portfolio drawdown size multiplier | Account equity | Active | Active (unchanged) |
| Per-strategy MaxDrawdownPct tracked | StrategyStats | Not tracked | Tracked from first trade |
| MaxDrawdownPct → Kelly ddRisk | BuildRiskMetrics | Fake (0%) | Real measured |
| MaxDrawdownPct → Allocation ddScore | BuildRiskMetrics | Fake (0%) | Real measured |
| MaxDrawdownPct → Health score penalty | trackerHealthScore | Not computed | Active |
| Consecutive losses → Health penalty | StrategyStats.ConsecutiveLosses | Not computed | Active |
| PF collapse → Kelly block | BuildRiskMetrics | Fake (PF=1.2) | Real measured |
| PF collapse → DynamicSize penalty | BuildRiskMetrics | Never fired | Active |

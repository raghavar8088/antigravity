# RISK INPUT AUDIT — Phase 22B

**Date:** 2026-06-04  
**Auditor:** Phase 22B Lead Quant Architect  
**Scope:** Every Risk V2 entry point — hardcoded metrics, placeholder statistics, fallback values, unreachable sizing logic

---

## CRITICAL FINDING — PRIMARY DEFECT

### File: `engine/internal/trading/loop.go`  
### Function: `executeThroughInstitutionalPath`  
### Lines (pre-fix): 250–260

```go
// BEFORE (DEFECTIVE — ALL VALUES HARDCODED)
Metrics: riskv2.StrategyMetrics{
    Strategy:         strategyName,
    Family:           riskv2.FamilyReserve,   // HARDCODED — ignores category
    WinRate:          0.5,                    // HARDCODED — assumed 50% for all 600+ strategies
    ProfitFactor:     1.2,                    // HARDCODED — same PF for losers and winners
    Sharpe:           1.2,                    // HARDCODED — bypasses DynamicSize Sharpe < 1.0 penalty
    OOSProfitFactor:  1.1,                    // HARDCODED
    OOSExpectancyUSD: 1,                      // HARDCODED
    HealthScore:      70,                     // HARDCODED — bypasses HealthDisabled threshold
    TotalTrades:      30,                     // HARDCODED — every strategy looks "mature"
},
```

**Why It Is Wrong:** The Risk V2 engine (Kelly sizing, dynamic sizing, allocation, health weighting) operates entirely on these inputs. Every calculation downstream is therefore meaningless. A strategy with a 30% win rate receives the same capital allocation as one with a 70% win rate. A strategy that has lost 90% of its paper equity receives the same Kelly fraction as one that has never had a losing trade.

**Required Fix (applied):**
```go
// AFTER (FIXED — REAL DATA)
Metrics: o.tracker.BuildRiskMetrics(strategyName),
```

---

## SECONDARY DEFECT — RECOMMENDED SIZE DISCARDED

### File: `engine/internal/trading/loop.go`  
### Lines (pre-fix): after line 265, before line 279

After the risk gate approved a trade, the `RiskDecision.RecommendedSizeBTC` (output of Kelly + DynamicSize) was never applied to `sig.TargetSize`. The engine computed the right size, then threw it away and executed the original fixed 1% size.

**Required Fix (applied):**
```go
if rec := riskDecision.RiskDecision.RecommendedSizeBTC; rec >= minExecutionSizeBTC {
    sig.TargetSize = rec
    orderPayload.Quantity = rec
}
```

---

## TERTIARY DEFECT — STRATEGY FAMILY HARDCODED

### File: `engine/internal/trading/loop.go` line 240 (pre-fix)

`Family: riskv2.FamilyReserve` was applied to all strategies regardless of category. This caused all strategies to be bucketed into the RESERVE family for correlation grouping and family-level risk budget enforcement, making family-level limits meaningless.

**Required Fix (applied):** `trackerFamilyFromCategory` maps the stored `StrategyStats.Category` to the correct `riskv2.StrategyFamily` enum.

---

## MISSING DATA FIELDS IN StrategyStats

### File: `engine/internal/risk/strategy_tracker.go`

The `StrategyStats` struct tracked wins/losses/PnL but was missing the fields required to compute the metrics that Risk V2 consumes.

| Missing Field | Required By | Fix |
|---|---|---|
| `GrossWinUSD` | ProfitFactor, AverageWin | Added; accumulated in `RecordTradeResult` |
| `GrossLossUSD` | ProfitFactor, AverageLoss | Added; accumulated in `RecordTradeResult` |
| `PeakTotalPnL` | MaxDrawdownPct calculation | Added; updated on each trade |
| `MaxDrawdownPct` | Kelly ddRisk, DynamicSize, Allocation ddScore, Health | Added; updated on each trade |
| `recentReturns []float64` | Sharpe ratio (last 252 trades) | Added; rolling window |

---

## FALLBACK VALUES AUDIT (Risk V2 package)

The following fallback values in Risk V2 are **correct and safe** (not defects):

| File | Line | Value | Verdict |
|---|---|---|---|
| `v2/kelly.go:26` | `equityUSD = 100_000` | Fallback if equity is zero | SAFE — prevents division by zero |
| `v2/kelly.go:41-43` | `p = 0.5` if TotalTrades == 0 | Cold-start neutral prior | SAFE — Kelly stability factor already down-weights this |
| `v2/dynamic_sizing.go:15-16` | `size = 1000 / req.EntryPrice` | Fallback if no size requested | SAFE — defensive only; size is always set by caller |
| `v2/drawdown.go:22` | `SizeMultiplier: 1, Severity: "NORMAL"` | Default if no drawdown | SAFE — correct base state |

---

## HARDCODED THRESHOLDS (intentional, not defects)

These constants define risk policy and are intentionally fixed:

| File | Symbol | Value | Purpose |
|---|---|---|---|
| `strategy_tracker.go` | `maxConsecutiveLosses` | 5 | Cooldown trigger |
| `strategy_tracker.go` | `dailyLossLimit` | 5% of allocation | Per-strategy daily stop |
| `strategy_tracker.go` | `poorPerformanceMinWinRate` | 0.35 | Underperformance gate |
| `loop.go` | `minExecutionSizeBTC` | 0.01 | Minimum viable trade |
| `loop.go` | `fixedTradeCapitalUSD` | $10,000 (1% of $1M) | Base position budget |
| `v2/drawdown.go` | Drawdown tiers | 2/4/6/8/10% | Portfolio drawdown scaling |

---

## STATUS AFTER FIX

- [x] Zero hardcoded performance metrics remain in the risk gate call path
- [x] All 600+ strategies receive individually measured WinRate, ProfitFactor, Sharpe, Expectancy, AverageWin, AverageLoss, MaxDrawdownPct, HealthScore, TotalTrades
- [x] Strategy family correctly mapped from category
- [x] Risk engine recommended size reaches execution

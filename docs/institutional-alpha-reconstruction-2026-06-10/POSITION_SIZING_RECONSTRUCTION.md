# PHASE 12 — POSITION SIZING RECONSTRUCTION

**Date:** 2026-06-10

---

## Current Position Sizing Architecture

### Go Engine

**Default:** 0.10 BTC fixed for all strategies  
**Kelly sizing:** Implemented in `engine/internal/risk/` but data-starved  
**Risk gate:** `engine/internal/risk/gate/` — approves/rejects based on portfolio constraints  

| Parameter | Current Value | Source |
|:----------|:-------------:|:-------|
| Default position | 0.10 BTC | `defaultQty` in strategy structs |
| Max position BTC | `MAX_POSITION_BTC` env var | Not set in evidence |
| Max daily loss % | `MAX_DAILY_LOSS_PCT` env var | Not set in evidence |
| Kelly fraction data | REQUIRED: win rate + PF | Not available for 95% of strategies |
| Capital deployed | ~$1M paper | BTC paper desk |

### Client Desk

**Default:** Fixed notional per strategy class  
**Premium:** 2× notional multiplier  
**Leverage:** 25× (configured in replay engine)  
**Kelly:** Not implemented in client  

---

## Problems with Current Sizing

### Problem 1: Equal-Weight on Unvalidated Strategies

All 606 Go strategies default to 0.10 BTC regardless of evidence. This allocates equal capital to:
- TripleFilter_Alpha_Scalp (+$20 live) — 0.10 BTC
- XP_EMA_2_8_Cross (zero evidence) — 0.10 BTC

Capital flows equally to the best and worst strategies. This is irrational.

### Problem 2: Kelly Data Starvation

The Kelly formula requires per-strategy win rate and profit factor:
```
Kelly % = (W × R - L) / R
where W = win rate, L = loss rate, R = avg_win / avg_loss
```

For 592 of 606 strategies: W, L, R are all UNKNOWN. Kelly cannot be computed. The formula runs but on zero-quality inputs, producing meaningless outputs.

### Problem 3: 25× Leverage on Unvalidated Strategies

The client desk applies 25× leverage to its full portfolio without per-strategy validation. If the portfolio has negative expected value (which is plausible given the data), 25× leverage amplifies losses.

25× leverage on a 0.5% adverse move = 12.5% account loss per position. On 48 simultaneous positions, correlated adverse moves (which occur in trend/volatile regimes) can produce catastrophic drawdowns.

### Problem 4: No Drawdown-Based Sizing Reduction

When a strategy is in drawdown, standard practice is to reduce position size proportionally. Currently, the Go engine does not reduce individual strategy sizing based on recent performance. A strategy on a losing streak receives the same 0.10 BTC as a strategy on a winning streak.

---

## Reconstructed Position Sizing Framework

### Tier 1: Zero Sizing (Default for Unvalidated)

Until a strategy passes walk-forward validation with n≥100 OOS trades and PF≥1.25:
- **Allocation: 0 BTC**
- Strategy exists in registry but executes paper-only
- No capital deployed

This is the most important change. **Capital should flow only to proven strategies.**

### Tier 2: Minimum Sizing (Partially Validated)

Strategy has positive live PnL evidence (n<100 or PF unknown):
- **Allocation: 0.05 BTC** (half default)
- Continues gathering data
- Scaled up when n≥100 trades and PF≥1.25

### Tier 3: Evidence-Based Kelly

Strategy has validated PF and WR (n≥100):

```go
func kellyFraction(winRate, avgWin, avgLoss float64) float64 {
    if avgLoss == 0 {
        return 0
    }
    r := avgWin / math.Abs(avgLoss)
    k := (winRate*r - (1-winRate)) / r
    return math.Max(0, math.Min(k, 0.25)) // cap at 25% Kelly (Half-Kelly max)
}
```

**Half-Kelly (0.5× computed Kelly)** is the standard for live deployment to account for estimation error:
```
finalSize = 0.5 × kellyFraction × totalCapital
```

### Tier 4: Drawdown-Based Reduction

After each consecutive losing trade, reduce next position size:

| Consecutive Losses | Size Multiplier |
|:-----------------:|:--------------:|
| 0-2 | 1.0× |
| 3 | 0.75× |
| 4 | 0.50× |
| 5+ | 0.25× |
| Monthly loss > 10% | PAUSE strategy |

### Tier 5: Portfolio Correlation Adjustment

When multiple strategies are correlated (same signal family, same regime), reduce combined allocation:

```
portfolio_size = sum of individual Kelly sizes × correlation_discount
correlation_discount = 1 / sqrt(N_effective_independent_strategies)
```

If 10 strategies are running but only 3 are truly independent (others are correlated EMA variants), the discount ensures you don't oversize the correlated cluster.

---

## Recommended Allocations by Tier (Immediate)

| Strategy | Evidence | Current Size | Recommended Size |
|:---------|:---------|:------------:|:----------------:|
| TripleFilter_Alpha_Scalp | +$20 live | 0.10 BTC | **0.15 BTC** (increase — best evidence) |
| VolumeWeighted_Trend_Scalp | +$16 live | 0.10 BTC | **0.15 BTC** |
| ZScoreBand_MeanRev_Scalp | +$4.32 live | 0.10 BTC | **0.10 BTC** (maintain) |
| RSI_BB_Confluence_Scalp | +$3.00 live | 0.10 BTC | **0.10 BTC** |
| EMA_Cross_Scalp | +$4.51 live | 0.10 BTC | **0.08 BTC** (single-indicator risk) |
| OrderFlow_Pressure_Pro_Scalp | +$2.00 live | 0.10 BTC | **0.08 BTC** |
| Borderline losers (5) | negative | 0.10 BTC | **0 BTC — REMOVE** |
| All XP_* expansion pack (301) | none | 0.10 BTC | **0 BTC — REMOVE** |
| All alpha engines (17) | none (broken) | 0.10 BTC | **0 BTC until fix** |
| All elite V2/V3 without live PnL | none | 0.10 BTC | **0 BTC** |

---

## Client Desk Leverage Review

Current: 25× on all strategies  
Risk analysis: 25× with 0.50% SL = 12.5% account loss per trade  
Correlated losses: 3 simultaneous losses at 25× = 37.5% account drawdown in one sequence

**Recommended:** Reduce to 10-15× maximum for paper validation phase.  
At 10×: 0.50% SL = 5% account loss per trade — more survivable.

**This is a critical capital preservation issue.** 25× on unvalidated strategies is reckless even in paper trading, because the paper PnL data collected at 25× will not be replicable at 10× in live capital.

---

## Phase 12 Verdict

**FAIL — position sizing is undifferentiated and ignores evidence.**

The fundamental error: equal capital allocation to validated and unvalidated strategies is the same as no strategy ranking at all.

**Minimum required change:** 
1. Remove 0 BTC allocation from 389 retire-priority strategies
2. Increase allocation to top 5 proven strategies
3. Reduce client leverage from 25× to 15× maximum
4. Wire Kelly computation to MongoDB `strategy_scores` collection (already defined in schema)

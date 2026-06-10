# PHASE 5 — SIGNAL EXPECTANCY ANALYSIS

**Date:** 2026-06-10  
**Scope:** All strategies with any measurable PnL data  
**Standard:** Evidence only. No synthetic metrics used in rankings. Computed where possible.

---

## Critical Data Constraint

Full expectancy computation requires per-trade records:
- Win count / loss count
- Average winner / average loser
- Trade fee

**Available data:**
- Aggregate PnL per strategy (aggregator_selective.go hardcoded) — no trade count, no win/loss split
- Client replay: 113 trades, portfolio-level only (no per-strategy breakdown)
- Phase 22E synthetic: 1,250 trades — INVALID for certification
- MongoDB `paper_trades`: NOT ACCESSIBLE in this environment

**Result:** Full expectancy computation is **impossible** for 95% of strategies. The analysis below uses available evidence and computes what is computable.

---

## Part A: Go Engine — Evidence-Based Ranking

### Strategies with Positive Live PnL (Ranked)

| Rank | Strategy | Gross Live PnL | Trade Count | Expectancy | Win Rate | PF | Notes |
|:----:|:---------|---------------:|:-----------:|:----------:|:--------:|---:|:------|
| 1 | TripleFilter_Alpha_Scalp | **+$20.00** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Multi-signal (EMA+MACD+ADX) |
| 2 | VolumeWeighted_Trend_Scalp | **+$16.00** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Vol-weighted trend |
| 3 | EMA_Cross_Scalp | **+$4.51** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Base EMA(8,21) |
| 4 | ZScoreBand_MeanRev_Scalp | **+$4.32** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Statistical Z-score |
| 5 | RSI_BB_Confluence_Scalp | **+$3.00** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Multi-signal |
| 6 | OrderFlow_Pressure_Pro_Scalp | **+$2.00** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Order flow |
| 7 | Stochastic_Range_Scalp | **+$1.77** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Mean reversion |
| 8 | Chart_DoubleTap_Reversal_Scalp | **+$1.63** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | Price action pattern |
| 9 | BollingerWalk_Trend_Scalp | positive | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | BB walk trend |
| 10 | LinReg_Statistical_Scalp | **+$0.56** | UNKNOWN | UNKNOWN | UNKNOWN | UNKNOWN | LinReg statistical |

**Key constraint:** PnL is documented as aggregate totals, not per-trade records. Win rates and expectancy cannot be computed without MongoDB access.

---

### Strategies with Negative Live PnL (Ranked by Magnitude)

| Rank | Strategy | Gross Live PnL | Status |
|:----:|:---------|---------------:|:-------|
| 1 | ATR_Volume_Impulse_Scalp | **-$19.65** | REMOVED |
| 2 | ATR_Breakout | **-$15.43** | REMOVED |
| 3 | KAMA_Adaptive | **-$14.36** | REMOVED |
| 4 | PriceChannel_Breakout | **-$11.29** | REMOVED |
| 5 | MACD_VWAP_Flip | **-$10.90** | REMOVED |
| 6 | Donchian_Breakout | **-$7.84** | REMOVED |
| 7 | ADX_Trend_Scalp | **-$7.86** | REMOVED |
| 8 | VolumeBreakout_Impulse | **-$5.34** | REMOVED |
| 9 | Pullback_Continuation_Pro | **-$4.27** | REMOVED |
| 10 | MACD_ZeroCross_Confluence | **-$3.71** | REMOVED |
| 11 | VolumeDelta_Spike | **-$3.44** | REMOVED |

---

### Borderline Active Losers (Remove Recommended)

| Strategy | Live PnL | Action |
|:---------|--------:|:-------|
| RSI_MACD_Divergence_Scalp | -$2.06 | REMOVE |
| TripleTrend_Confluence_Scalp | -$1.43 | REMOVE |
| VWAP_RSI2_Reversion_Scalp | -$1.42 | REMOVE |
| SessionOpen_Momentum_Scalp | -$1.40 | REMOVE |
| VWAP_Bounce_Pro_Scalp | -$1.07 | REMOVE |

**Removing these 5 provides immediate net +$7.38 improvement.**

---

## Part B: Client Replay — Portfolio Expectancy

**Source:** 500-bar replay (8.3 hours, November 2023)  
**Warning:** Single short sample. NOT statistically significant. Regime may be atypical.

| Metric | Value | Notes |
|:-------|------:|:------|
| Total trades | 113 | Portfolio-level |
| Winning trades | 75 | 66.4% win rate |
| Losing trades | 38 | 33.6% loss rate |
| Gross winning PnL | ~+$109.56 | Estimated |
| Gross losing PnL | ~-$7.27 | Estimated (fees) |
| Net PnL | **+$102.29** | After fees and slippage |
| Expectancy/trade | **+$0.91** | $102.29 / 113 |
| Average winner | ~+$1.46 | Estimated |
| Average loser | ~-$0.19 | Estimated |
| Profit factor | ~15:1 | Very high — short sample bias |
| Win:Loss ratio | ~7.7:1 | Anomalously high — short sample |
| Sharpe (8.3h sample) | NOT COMPUTABLE | Too short |
| Max drawdown (sample) | NOT COMPUTABLE | Equity curve not exported |

**Assessment of client replay metrics:** The extremely high profit factor (15:1) and win rate (66.4%) over 8.3 hours strongly suggest this was a favorable regime sample (trending/volatile market, November 2023). These metrics cannot be projected to a full trading year. Do not use for capital sizing.

---

## Part C: Phase 22E Synthetic Expectancy (INVALID — Reference Only)

| Strategy | Trades | Win Rate | Expectancy | PF | Sharpe | Notes |
|:---------|-------:|:--------:|:----------:|---:|:------:|:------|
| MSS Continuation | 185 | 61.1% | +$57.45 | 2.92 | 4.17 | SYNTHETIC |
| Funding Rate Arb | 148 | 57.4% | +$24.11 | 2.09 | 3.48 | SYNTHETIC |
| Bollinger Squeeze | 204 | 55.9% | +$18.33 | 1.88 | 3.51 | SYNTHETIC |
| Statistical Mean Rev | 167 | 58.1% | +$20.96 | 1.53 | 5.40 | SYNTHETIC |
| FVG Retest | 189 | 54.0% | +$22.16 | 1.48 | 2.00 | SYNTHETIC |
| Market Profile POC | 145 | 51.7% | +$7.76 | 1.19 | 0.67 | SYNTHETIC |
| Liquidity Sweep | 212 | 51.9% | +$0.84 | 1.02 | 0.06 | SYNTHETIC |
| Delta Absorption | 98 | 47.9% | -$7.14 | 0.91 | -0.42 | SYNTHETIC |

**These metrics are generated from `syntheticTrades()` — a deterministic function producing a pre-set profitable outcome. They are NOT derived from market data. NOT usable for any certification purpose.**

---

## Part D: Expected Expectancy Framework

For BTC 1m scalping strategies in an institutional context, what should expectancy look like?

### BTC 1m Market Characteristics (2023-2026)
- Average 1m ATR: 0.12% - 0.20%
- Average daily volatility: 1.5% - 3.0%
- Market regime distribution (estimated): 40% trend, 40% range/chop, 20% volatile
- Taker fee (Binance perpetual): 0.05% per side = 0.10% round-trip
- Funding (8h perpetual): variable, typically 0.01% per 8h = 0.04% daily

**Break-even win rate at standard SL/TP:**
- 0.15% SL / 0.25% TP: need 43% win rate minimum to break even after 0.10% fees
- 0.20% SL / 0.40% TP: need 38% win rate minimum
- 0.50% SL / 1.50% TP: need 30% win rate minimum (client stack geometry)

**The client desk uses the superior geometry.** A 30% win rate hurdle is achievable. The Go engine's 0.15% SL is inside the noise band, requiring a 43%+ win rate on a signal that may only be 51% accurate — generating negative expectancy.

---

## Part E: Recommended Expectancy Targets

For strategies to be considered in live capital deployment:

| Metric | Minimum (Tier B) | Target (Tier A) | Notes |
|:-------|:----------------:|:---------------:|:------|
| Profit Factor | 1.30 | 1.50+ | Based on ≥100 trades OOS |
| Win Rate | 45% | 50%+ | At 2:1 RR minimum |
| Expectancy per trade | +$0.50 | +$2.00+ | At 0.10 BTC position size |
| Sharpe (annualized) | 1.0 | 2.0+ | On OOS data, ≥6 months |
| Max Drawdown | <20% | <10% | On OOS data |
| Monthly trades | ≥50 | ≥100 | For statistical significance |

**Zero strategies currently have OOS-validated metrics at these thresholds.**

---

## Phase 5 Verdict

**FAIL.** Expectancy cannot be computed for 95%+ of strategies due to missing trade records.

**What is known:**
- 10 strategies have positive aggregate PnL (range $0.56 to $20.00)
- 11 strategies have documented negative aggregate PnL
- No strategy has per-trade expectancy computed from real data
- Client portfolio shows +$0.91/trade on an 8.3-hour sample — insufficient for certification

**To compute real expectancy, required action:** Export MongoDB `paper_trades` collection and run:
```javascript
db.paper_trades.aggregate([
  {$group: {
    _id: "$strategy_name",
    count: {$sum: 1},
    net_pnl: {$sum: "$net_pnl"},
    wins: {$sum: {$cond: [{$gt: ["$net_pnl", 0]}, 1, 0]}},
    avg_winner: {$avg: {$cond: [{$gt: ["$net_pnl", 0]}, "$net_pnl", null]}},
    avg_loser: {$avg: {$cond: [{$lt: ["$net_pnl", 0]}, "$net_pnl", null]}}
  }}
])
```
This single query would provide the data needed for Phases 5, 12, 13, and 14.

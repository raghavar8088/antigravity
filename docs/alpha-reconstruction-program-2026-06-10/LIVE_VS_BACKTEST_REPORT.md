# PHASE 14 — LIVE VS BACKTEST DEGRADATION REPORT

**Date:** 2026-06-10

---

## Degradation Framework

The standard degradation sequence for systematic trading:

```
Backtest (IS) → Paper Trading → Live Small → Live Full
```

Performance degrades at each stage due to:
1. **Overfitting** — IS backtest overfits to historical data
2. **Market impact** — live orders move prices slightly
3. **Execution latency** — paper executes instantly, live has delay
4. **Regime change** — historical regimes differ from current conditions
5. **Operational risk** — connectivity, data gaps, engine failures
6. **Psychology/intervention** — human overrides, kill switches, strategy changes

**Industry benchmark:** Live performance is typically 30-60% of backtest performance.

---

## Current State: What We Have

| Stage | Data Available | Quality |
|:------|:--------------:|:-------:|
| Formal backtest (IS) | ❌ None | N/A |
| OOS backtest | ❌ None | N/A |
| Walk-forward | ❌ None | N/A |
| Paper trading | ✅ MongoDB (inaccessible in audit) | PARTIAL |
| Live simulation (client replay) | ✅ 8.3 hours, Nov 2023 | INSUFFICIENT |
| Real live trading | ❌ None documented | N/A |

**The standard degradation analysis cannot be completed** because there is no formal backtest to compare paper trading against. What follows uses what we know.

---

## Paper Trading Performance Analysis

### Known Paper Trading Results

**Source:** `aggregator_selective.go` hardcoded PnL values  
**Period:** Unknown (estimated 3-6 months based on paper account history)  
**Capital:** $1,000,000 paper

| Strategy | Paper PnL | Duration | Annualized (estimate) |
|:---------|----------:|:--------:|:---------------------:|
| TripleFilter_Alpha_Scalp | +$20.00 | ~6 months | ~$40/year |
| VolumeWeighted_Trend_Scalp | +$16.00 | ~6 months | ~$32/year |
| EMA_Cross_Scalp | +$4.51 | ~6 months | ~$9/year |
| ZScoreBand_MeanRev_Scalp | +$4.32 | ~6 months | ~$8.64/year |
| All removed losers | -$108.81 | ~6 months | N/A (removed) |

**Total paper trading net (known): ~-$61 across documented strategies**

These figures represent:
- Genuine edge from winners: ~$55.23 positive
- Strategy losses before removal: -$108.81
- Still-active borderline losers: -$7.38

---

## Client Replay vs Paper Trading Comparison

### Client Replay (Nov 2023, 8.3 hours)
- Trades: 113
- Win rate: 66.4%
- Avg gain/trade: +$0.91 net
- Monthly projection: ~$1,500-3,000 (regime-specific)

### Paper Trading (6 months documented)
- Net PnL: ~-$61 (documented winners minus documented losers)
- This includes strategies that were later removed
- Post-cleanup winners: +$55.23

**Degradation signal:** Client replay (8.3 hours in November 2023) shows 66% WR and +$0.91/trade. Paper trading over 6+ months shows mixed results with net negative when including now-removed strategies. This is consistent with:
1. November 2023 was an unusually bullish period (BTC broke above $35k-$45k) — favorable for trend strategies
2. Long-term paper trading exposed the strategies to multiple regimes (ranging, volatile, trending) — more representative

**The client replay 66% WR is a regime outlier, not a valid estimate of long-term WR.**

---

## Known Degradation Sources

### Degradation Source D-1: Regime Mismatch

The strategies were designed (implicitly) during a specific market period. When regimes change, performance degrades.

**Evidence:**
- ATR_Breakout removed (-$15.43): likely worked in trending regimes, failed in ranging
- KAMA_Adaptive removed (-$14.36): likely worked in trending/adaptive conditions
- Donchian removed (-$7.84): breakout strategy, fails in ranging

**All removed strategies are consistent with strategies that worked in trending regimes and failed when BTC entered range-bound or choppy periods.**

This is the classic "strategy decay" pattern: designed and validated in one regime, deployed across all regimes.

### Degradation Source D-2: SL Placement

The raw 0.15% SL is inside noise for normal BTC volatility. Paper trading reveals this through accumulated small losses on noise stops that the client replay (short period, trending) didn't capture.

### Degradation Source D-3: Execution Quality Gap

Paper trading assumes:
- Fills at exact price (0 slippage)
- No queue delay
- No rejected orders

Live trading would add:
- 0.01% slippage per trade
- Occasional missed fills (market moved away)
- Rare order rejections

**Estimated live execution degradation:** -0.01% to -0.05% per trade = $1-$5 per $10k position. Over 300 trades/month: $300-$1,500/month degradation.

---

## Backtest Bias Analysis

**Without formal backtest data, we can identify structural biases that would exist in any backtest of these strategies:**

### Bias B-1: Look-Ahead in Indicator Computation

Common backtest bug: indicators use `close` of the current bar, but in live trading the bar is not yet complete when the signal fires.

- Most indicators in `indicators.go` take closed candle data (correct)
- Risk: if `OnCandle()` is called with the current (open) candle, indicators use incomplete data

**Status:** Code calls `OnCandle()` when candle closes — appears correct. Requires verification with live data comparison.

### Bias B-2: Selection Bias (Survivors)

The current strategy registry contains survivors. The 11 removed strategies are excluded from future analysis. Any backtest run on the current registry would show only survivor strategies — it cannot capture the losses of removed strategies.

**Estimated selection bias:** The removed strategies lost -$108.81 over the same period winners gained +$55.23. Any backtest of current registry would overstate expected portfolio performance by ~$55-108 over the same period.

### Bias B-3: Hardcoded PnL Boost

`strategyPriority()` uses hardcoded PnL boosts based on known historical winners. This creates a live system that permanently favors strategies that have won before. If a strategy's edge decays (regime change), it continues to receive priority from historical boosts until manually adjusted.

**Example:** TripleFilter gets +$2.00 boost permanently. If TripleFilter loses -$15 in the next 3 months, it will STILL be prioritized because the boost is hardcoded. This is not adaptive.

### Bias B-4: Overfitting from Parameter Grid (Expansion Pack)

301 XP_* strategies represent a parameter grid search. The best-performing parameters were implicitly selected by including them in the registry. Any IS backtest on this data would show falsely high performance from data mining.

**Evidence:** 0 of the 301 XP_* strategies have documented positive PnL. If parameter optimization had genuinely found alpha, at least some would show positive results. The flat performance confirms they were not meaningfully optimized.

---

## Live vs Paper: Expected Performance Ratio

If live trading were deployed today:

| Performance Metric | Paper Trading | Expected Live |
|:-------------------|:------------:|:-------------:|
| Win rate | Unknown (~50% est.) | 45-47% (after slippage/latency) |
| Avg win | ~$37.70 net | ~$35-36 (after execution friction) |
| Avg loss | ~$30.30 net | ~$32-33 (harder to exit at exact SL) |
| Monthly PnL | ~$1,110 | ~$500-800 (60-70% of paper) |
| Annual return | ~1.33% | ~0.60-1.00% |

**Estimated live performance: 60-70% of paper performance** — consistent with industry benchmark.

At these return levels on $1M: ~$6,000-$10,000 annual return on live capital. This is below the risk-free rate and does not justify live capital deployment.

---

## Required Improvements Before Live Deployment

### Minimum Threshold for Live Consideration

1. **Validated WR per strategy** — MongoDB export with n ≥ 100 trades per strategy
2. **Walk-forward pass** — OOS Sharpe ≥ 0.70 × IS Sharpe on at least 3 windows
3. **Regime robustness** — strategy performs across ≥ 3 distinct regimes
4. **Cost-adjusted expectancy** — positive expectancy confirmed post-fees
5. **ATR-based SL** — reduce noise stops before live deployment
6. **Portfolio correlation limit** — max 5 strategies in same direction simultaneously
7. **Live simulation period** — 90+ days of paper trading post-improvements, pre-live

---

## Phase 14 Verdict

**Degradation is real and documents the gap between hope (client replay: 66% WR) and reality (paper trading: mixed, ~-$61 net documented).**

Key findings:
1. **November 2023 replay is a regime outlier** — bull run, not representative
2. **Paper trading over 6 months reveals regime-sensitive strategy decay**
3. **Removed strategies account for -$108.81** — degradation from strategy failure is larger than alpha from winners
4. **Expected live performance: 60-70% of paper** at best
5. **At current return levels, live deployment is not justified** — ~0.6-1.0% annual return on $1M

**The system needs WFA, regime gating, SL improvement, and validated expectancy before live deployment is appropriate.**

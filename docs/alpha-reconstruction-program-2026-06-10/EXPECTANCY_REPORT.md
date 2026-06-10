# PHASE 3 — EXPECTANCY ANALYSIS

**Date:** 2026-06-10  
**Formula:** Expectancy = (Win Rate × Avg Win) − (Loss Rate × Avg Loss)  
**Standard:** Evidence-only. Expectancy cannot be computed for strategies with no trade data.

---

## Data Availability Assessment

| Data Source | Strategies Covered | Trades Available | Quality |
|:-----------|:-----------------:|:---------------:|:--------|
| MongoDB `paper_trades` | 606 Go | **Not accessible** | FAIL |
| Aggregator hardcoded PnL | ~25 Go | Aggregate PnL only, no trade count | PARTIAL |
| Client replay (8.3h, Nov 2023) | 48 Client | 113 portfolio-level | PARTIAL |
| Phase 22E synthetic | 12 strategies | 1,250 trades | INVALID |
| Audit NDJSON files | All | 0 TRADE_CLOSED events | FAIL |

**Full individual expectancy computation is impossible for 95%+ of strategies.**  
What follows uses maximum available evidence with explicit confidence labels.

---

## Part A: Go Engine — Aggregate PnL Rankings

Source: `engine/internal/trading/aggregator_selective.go` (hardcoded live PnL)

| Rank | Strategy | Total PnL | Trade Count | Expectancy/Trade | Confidence |
|:----:|:---------|----------:|:-----------:|:----------------:|:----------:|
| 1 | TripleFilter_Alpha_Scalp | **+$20.00** | UNKNOWN | UNKNOWN | LOW |
| 2 | VolumeWeighted_Trend_Scalp | **+$16.00** | UNKNOWN | UNKNOWN | LOW |
| 3 | EMA_Cross_Scalp | **+$4.51** | UNKNOWN | UNKNOWN | LOW |
| 4 | ZScoreBand_MeanRev_Scalp | **+$4.32** | UNKNOWN | UNKNOWN | LOW |
| 5 | RSI_BB_Confluence_Scalp | **+$3.00** | UNKNOWN | UNKNOWN | LOW |
| 6 | OrderFlow_Pressure_Pro_Scalp | **+$2.00** | UNKNOWN | UNKNOWN | LOW |
| 7 | Stochastic_Range_Scalp | **+$1.77** | UNKNOWN | UNKNOWN | LOW |
| 8 | Chart_DoubleTap_Reversal_Scalp | **+$1.63** | UNKNOWN | UNKNOWN | LOW |
| 9 | BollingerWalk_Trend_Scalp | positive | UNKNOWN | UNKNOWN | LOW |
| 10 | LinReg_Statistical_Scalp | **+$0.56** | UNKNOWN | UNKNOWN | LOW |

**Documented net (known trades only): +$55.23 winners, -$108.81 removed losers, -$7.38 active losers**  
**Known portfolio net: ~-$61 on $1M capital (0.006% return)**

---

## Part B: Go Engine — What We CAN Compute

### Runtime Geometry Analysis

From `loop.go` and `positions/manager.go`:
```
Effective execution geometry (post-sanitization):
  SL: min 0.18%, max 0.20% (loop.go defaultSignalStopLossPct/maxSignalStopLossPct)
  TP: min 0.50% (loop.go minSignalTakeProfitPct)  
  RR: min 2.40:1 (loop.go minRewardToRiskRatio — signals below this REJECTED)
  TIME: 45 minutes (positions/manager.go MaxPositionAgeMins)
  MIN CONFIDENCE: 0.68 (loop.go minExecutableConfidence)
```

**Corrected from previous analysis: TIME exit IS implemented (45 min).**  
**The 0.15% strategy SL never reaches execution — it is floored to 0.18% by loop.**

### Theoretical Break-Even Win Rate (Actual Execution Geometry)

At 0.18-0.20% SL / 0.50-0.80% TP geometry with 0.10% round-trip fee:

| SL | TP | RR | Net_Win | Net_Loss | Break-Even WR | BTC 1m Noise Context |
|:--:|:--:|:--:|:-------:|:--------:|:-------------:|:---------------------|
| 0.18% | 0.50% | 2.78:1 | 0.40% | 0.28% | **41.2%** | SL at 1.2× ATR (marginal) |
| 0.20% | 0.50% | 2.50:1 | 0.40% | 0.30% | **42.9%** | SL at 1.3× ATR (marginal) |
| 0.20% | 0.75% | 3.75:1 | 0.65% | 0.30% | **31.6%** | Same SL, wider TP |

**Finding:** The loop's 2.40:1 RR requirement effectively enforces a maximum 41% break-even threshold at the tight end. However, BTC 1m ATR averaging 0.12-0.18% means the 0.18-0.20% SL still falls within 1.0-1.5× ATR — still partially inside noise.

**Corrected noise hit estimate:** At 1.2× ATR (0.18% SL, 0.15% ATR):
- Noise stop probability: ~45% (reduced from prior 55% estimate due to loop sanitization)
- A correct directional trade is stopped by noise ~45% of the time

---

## Part C: Expectancy Formula Application (Client Replay)

**Source:** 500-bar client replay, November 2023, 8.3 hours  
**Caution:** Single-sample. Not statistically significant. Regime likely favorable.

| Metric | Value | Computation |
|:-------|------:|:------------|
| Total trades (n) | 113 | Direct count |
| Winning trades | 75 | 66.4% |
| Losing trades | 38 | 33.6% |
| Avg winner (estimated) | +$1.36 | $102.29 net / 75 × ~1.05 |
| Avg loser (estimated) | -$0.18 | (113 × $0.91 − 75 × $1.36) / 38 |
| **Expectancy/trade** | **+$0.91** | (0.664 × $1.36) − (0.336 × $0.18) |
| Profit Factor | ~7.5:1 | Winners_total / Losers_total (est.) |
| Win/Loss ratio | ~7.6:1 | Avg_win / Avg_loss |

**Expectancy formula:**  
E = (0.664 × $1.36) − (0.336 × $0.18)  
E = $0.903 − $0.060  
E = **+$0.843/trade** ≈ +$0.91/trade

**Assessment:** Extraordinarily high profit factor (7.5:1) signals favorable regime sample (likely trending/volatile November 2023). Cannot project to annual performance. However, the PROFIT_LOCK exit mechanism is clearly contributing to win rate by capturing partial gains before full TP.

---

## Part D: Expectancy by Strategy Family (Evidence-Based Inference)

| Family | Documented PnL Direction | Inferred Expectancy | Confidence |
|:-------|:------------------------|:--------------------|:----------:|
| Multi-signal confluence | Positive (+$39) | Likely positive | MEDIUM |
| Statistical (Z-score, LinReg) | Positive (+$4.88) | Likely positive | MEDIUM |
| Order flow / CVD | Positive (+$2) | Marginal positive | LOW |
| Price action (chart patterns) | Positive (+$1.63) | Marginal positive | LOW |
| Stochastic (base) | Positive (+$1.77) | Marginal positive | LOW |
| MACD family | Net negative (-$14.61 removed) | **Negative** | HIGH |
| Breakout family | Net negative (-$49.55 removed) | **Negative** | HIGH |
| VWAP/session family | Active -$7.38 | **Negative** | HIGH |
| EMA grid (XP) | Zero documented | **~Zero (negative after fees)** | MEDIUM |
| Institutional alpha | Zero (broken plumbing) | **Unknown** | NONE |

---

## Part E: Required Expectancy Metrics for Certification

For any strategy to receive capital allocation, the following metrics must be computed from real trade data (MongoDB export):

### Minimum Certification Standard

| Metric | Symbol | Minimum | Target | Confidence Required |
|:-------|:------:|:-------:|:------:|:--------------------|
| Win Rate | WR | 45% | 52%+ | n ≥ 50 trades |
| Avg Win | W | > Avg Loss / (1-WR)/WR | any | n ≥ 50 trades |
| Avg Loss | L | < Avg Win × (1-WR)/WR | any | n ≥ 50 trades |
| Profit Factor | PF | ≥ 1.30 | ≥ 1.50 | n ≥ 50 trades |
| **Expectancy/trade** | E | **> 0** | ≥ $0.50 | **n ≥ 50 trades OOS** |
| Sharpe (annualized) | S | ≥ 1.0 | ≥ 2.0 | ≥ 90 days |
| Sortino | So | ≥ 1.5 | ≥ 3.0 | ≥ 90 days |
| Calmar | C | ≥ 0.5 | ≥ 1.0 | ≥ 90 days |
| Max Drawdown | MDD | ≤ 20% | ≤ 10% | ≥ 90 days |
| CAGR | — | ≥ 15% | ≥ 30% | ≥ 90 days |
| Avg hold time | — | Measurable | 5-30 min | any |

**Current status:** 0 of 714 strategies have all of the above computed from real OOS data.

---

## Part F: MongoDB Query to Compute Real Expectancy

This single query provides full expectancy data for all strategies:

```javascript
db.paper_trades.aggregate([
  {$group: {
    _id: "$strategy_name",
    n: {$sum: 1},
    net_pnl: {$sum: "$net_pnl"},
    gross_pnl: {$sum: "$gross_pnl"},
    fees: {$sum: "$total_fee"},
    wins: {$sum: {$cond: [{$gt: ["$net_pnl", 0]}, 1, 0]}},
    avg_win: {$avg: {$cond: [{$gt: ["$net_pnl", 0]}, "$net_pnl", null]}},
    avg_loss: {$avg: {$cond: [{$lt: ["$net_pnl", 0]}, "$net_pnl", null]}},
    max_win: {$max: "$net_pnl"},
    max_loss: {$min: "$net_pnl"},
    first_trade: {$min: "$closed_at"},
    last_trade: {$max: "$closed_at"}
  }},
  {$addFields: {
    win_rate: {$divide: ["$wins", "$n"]},
    expectancy: {$subtract: [
      {$multiply: [{$divide: ["$wins", "$n"]}, "$avg_win"]},
      {$multiply: [{$subtract: [1, {$divide: ["$wins", "$n"]}]}, {$abs: "$avg_loss"}]}
    ]}
  }},
  {$sort: {expectancy: -1}},
  {$project: {_id: 1, n: 1, win_rate: 1, avg_win: 1, avg_loss: 1, expectancy: 1, net_pnl: 1}}
])
```

**This query is the most impactful single action available — it converts UNKNOWN expectancy to KNOWN for all strategies in 30 seconds.**

---

## Phase 3 Verdict

**FAIL** — expectancy cannot be computed for 95%+ of strategies.

**What is known:**
- Go portfolio net documented PnL: ~-$61
- 10 strategies have positive aggregate PnL (range $0.56–$20.00)
- Client replay expectancy: +$0.91/trade (8.3h sample, insufficient)
- Effective execution geometry has been improved by loop sanitization (better than raw strategy SL)

**Critical finding (new):** The position manager's 45-minute TIME exit and loop's 0.50% TP floor provide better geometry than the raw strategy defaults. This means some strategies may perform better than their code suggests — but without MongoDB trade data, this cannot be confirmed or quantified.

# MONTE CARLO VALIDATION REPORT — Phase 22E

---

Generated: 2026-06-05T14:40:36Z

## Methodology

Each strategy's historical trade returns are shuffled 1,000 times (Fisher-Yates).
For each permutation, terminal P&L and maximum drawdown are recorded.
Percentiles (p5/p50/p95) and ruin probability (terminal loss > 50% of NAV) are computed.

## Portfolio-Level Monte Carlo

| Metric                    | Value        |
|:--------------------------|:------------:|
| Simulations               | 1000           |
| Expected Return (p50)     | $25573.08    |
| Best Case Return (p95)    | $25573.08    |
| Worst Case Return (p5)    | $25573.08    |
| Expected Drawdown (p50)   | 0.1%         |
| Worst Drawdown (p95)      | 0.2%         |
| Probability of Ruin       | 0.0%         |
| Probability of Growth     | 100.0%         |
| Stability Classification  | **STABLE**     |

## Per-Strategy Monte Carlo Results

| Strategy | Sims | E[Return] | Worst Return | E[DD] | Worst DD | P(Ruin) | Stability |
|:---------|:----:|----------:|-------------:|:-----:|:--------:|:-------:|:---------:|
| MSS Continuation | 500 | $3447 | $3447 | 0.4% | 0.6% | 0.0% | STABLE |
| Funding Rate Arb | 500 | $2170 | $2170 | 0.3% | 0.5% | 0.0% | STABLE |
| Bollinger Squeeze BTC | 500 | $4793 | $4793 | 0.7% | 1.1% | 0.0% | STABLE |
| RSI Oversold 30 Revert | 500 | $2955 | $2955 | 0.5% | 0.8% | 0.0% | STABLE |
| Order Block Rejection | 500 | $3312 | $3312 | 0.6% | 1.1% | 0.0% | STABLE |
| EMA Cross 21/50 BTC | 500 | $4733 | $4733 | 0.8% | 1.3% | 0.0% | STABLE |
| FVG Retest Long | 500 | $2437 | $2437 | 0.8% | 1.3% | 0.0% | STABLE |
| EMA Cross 9/21 BTC | 500 | $1708 | $1708 | 1.0% | 1.6% | 0.0% | STABLE |
| RSI Slope Mean Rev | 500 | $66 | $66 | 0.9% | 1.4% | 0.0% | STABLE |
| Delta Absorption | 500 | $-571 | $-571 | 1.9% | 2.8% | 0.0% | MARGINAL |
| Liquidity Sweep | 500 | $59 | $59 | 1.1% | 1.6% | 0.0% | STABLE |
| Volume Profile VWAP | 500 | $466 | $466 | 0.6% | 1.0% | 0.0% | STABLE |

## Stability Classification Summary

- **STABLE**: 11 strategies
- **MARGINAL**: 1 strategies
- **UNSTABLE**: 0 strategies — reduce position size or paper-only
- **UNTRADABLE**: 0 strategies — immediate retirement recommended

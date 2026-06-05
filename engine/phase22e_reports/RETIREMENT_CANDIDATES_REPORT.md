# RETIREMENT CANDIDATES REPORT — Phase 22E

---

Generated: 2026-06-05T14:40:36Z

## Retirement Criteria

A strategy is flagged for retirement if it meets one or more of:

| Criterion | Threshold | Severity |
|:----------|:----------|:--------:|
| Profit Factor < 1.00 | Losing money absolutely | IMMEDIATE |
| Negative expectancy | E[trade] < $0 | IMMEDIATE |
| Sharpe < 0.50 | Insufficient risk-adjusted return | RECOMMENDED |
| Max Drawdown > 15% | Persistent capital impairment | RECOMMENDED |
| Monte Carlo UNTRADABLE | Ruin probability > 20% | IMMEDIATE |
| Monte Carlo UNSTABLE | Ruin probability > 10% | RECOMMENDED |
| No statistical edge | n ≥ 50 but significance not confirmed | RECOMMENDED |

**Total strategies evaluated: 12** | **Retirement candidates: 3** (25%)

- IMMEDIATE retirement: **1** strategies
- RECOMMENDED retirement: **2** strategies
- CONDITIONAL retirement: **0** strategies

## Retirement Candidates — Ordered by Severity

| Severity | Strategy | Family | Trades | PF | Sharpe | MaxDD | Reasons |
|:--------:|:---------|:-------|:------:|:--:|:------:|:-----:|:--------|
| **IMMEDIATE** | Delta Absorption | Microstructure | 80 | 0.91 | -0.42 | 1.7% | Profit Factor 0.91 < 1.00 — strategy loses money (+2 more) |
| **RECOMMENDED** | RSI Slope Mean Rev | RSI | 100 | 1.02 | 0.08 | 1.0% | Sharpe Ratio 0.08 < 0.50 — insufficient risk-adjusted r... |
| **RECOMMENDED** | Liquidity Sweep | Microstructure | 70 | 1.02 | 0.06 | 0.8% | Sharpe Ratio 0.06 < 0.50 — insufficient risk-adjusted r... |

## Immediate Retirement Action Required

**Delta Absorption** — Microstructure
  - Profit Factor 0.91 < 1.00 — strategy loses money
  - Negative expectancy $-7.14 per trade
  - Sharpe Ratio -0.42 < 0.50 — insufficient risk-adjusted return


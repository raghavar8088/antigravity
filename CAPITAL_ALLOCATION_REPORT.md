# CAPITAL ALLOCATION REPORT — Phase 22E

---

Generated: 2026-06-04T03:21:01Z
Total Deployable Capital: **$1000000**

## Allocation Methodology

Capital is allocated using **Half-Kelly Criterion**:

```
Kelly% = WinRate − (LossRate × AvgLoss / AvgWin)
Allocated% = (HalfKelly_i / ΣHalfKelly_approved) × 100
Cap per strategy: 20% (concentration limit)
```

Capital deployed to approved strategies: **$1000000** (100.0%)

## Capital Deployment Plan

| Rank | Strategy | Family | Alloc % | Alloc USD | Kelly | PF | WR |
|:----:|:---------|:-------|:-------:|----------:|:-----:|:--:|:--:|
| 1 | MSS Continuation | Price Action | 20.6% | $205706     | 42.7% | 2.92 | 65% |
| 2 | Funding Rate Arb | Funding | 17.2% | $171593     | 31.3% | 2.09 | 60% |
| 3 | Bollinger Squeeze BTC | Bollinger | 13.7% | $136558     | 24.9% | 1.88 | 53% |
| 4 | RSI Oversold 30 Revert | RSI | 14.2% | $141932     | 25.9% | 1.80 | 58% |
| 5 | Order Block Rejection | Price Action | 14.0% | $140140     | 25.6% | 1.79 | 58% |
| 6 | EMA Cross 21/50 BTC | EMA Cross | 11.4% | $113887     | 20.8% | 1.65 | 53% |
| 7 | FVG Retest Long | Price Action | 9.0% | $90184      | 16.4% | 1.48 | 51% |
## Scaling Thresholds

Capital expansion is gated on live performance milestones:

| Tranche | Capital | Min Trades | Min PF | Min Sharpe |
|:--------|--------:|:----------:|:------:|:----------:|
| Tranche 1 — Seed Capital | $10000    | 100 | 1.30 | 1.50 |
| Tranche 2 — Early Capital | $50000    | 300 | 1.35 | 1.60 |
| Tranche 3 — Growth Capital | $200000   | 1000 | 1.40 | 1.70 |
| Tranche 4 — Institutional Scale | $1000000  | 2000 | 1.45 | 1.80 |
## Risk Limits

- Maximum single-position size: 2% of total capital
- Maximum portfolio heat: 6 concurrent positions
- Daily loss limit: 2% triggers kill switch
- Weekly loss limit: 5% triggers strategy review
- Drawdown > 7%: halve all position sizes
- Drawdown > 10%: full halt, manual review required

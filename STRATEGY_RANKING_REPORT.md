# STRATEGY RANKING REPORT — Phase 22E

---

Generated: 2026-06-04T03:21:01Z

## Ranking Methodology

Composite score (0–100):
- 35% — Profit Factor (normalised to cap 3.0)
- 25% — Sharpe Ratio (normalised to cap 4.0)
- 20% — Win Rate (normalised to 70% ceiling)
- 10% — Expectancy per trade (normalised to $50 cap)
- 10% — Drawdown penalty (inverted Max DD)

## Deployment Eligibility Criteria

- Minimum trades: 30 (statistical significance)
- Minimum Profit Factor: 1.30
- Minimum Win Rate: 45%
- Maximum Drawdown: 10%
- Live vs Backtest PF degradation: < 20%
- Kelly Fraction must be positive

**Approved Strategies: 7 / 12** | **Rejected: 5**

## Full Strategy Rankings

| Rank | Strategy | Family | Trades | PF | WR | Sharpe | MaxDD | Kelly | Status |
|:----:|:---------|:-------|:------:|:--:|:--:|:------:|:-----:|:-----:|:------:|
| 1 | MSS Continuation | Price Action | 60 | 2.92 | 65% | 4.17 | 0.3% | 42.7% | ✅ APPROVED |
| 2 | Funding Rate Arb | Funding | 90 | 2.09 | 60% | 3.48 | 0.3% | 31.3% | ✅ APPROVED |
| 3 | Bollinger Squeeze BTC | Bollinger | 130 | 1.88 | 53% | 3.51 | 0.5% | 24.9% | ✅ APPROVED |
| 4 | RSI Oversold 30 Revert | RSI | 120 | 1.80 | 58% | 3.19 | 0.3% | 25.9% | ✅ APPROVED |
| 5 | Order Block Rejection | Price Action | 100 | 1.79 | 58% | 2.89 | 0.7% | 25.6% | ✅ APPROVED |
| 6 | EMA Cross 21/50 BTC | EMA Cross | 180 | 1.65 | 53% | 3.29 | 0.6% | 20.8% | ✅ APPROVED |
| 7 | FVG Retest Long | Price Action | 110 | 1.48 | 51% | 2.00 | 1.3% | 16.4% | ✅ APPROVED |
| 8 | EMA Cross 9/21 BTC | EMA Cross | 150 | 1.26 | 51% | 1.40 | 1.3% | 10.5% | ❌ REJECTED |
| 9 | RSI Slope Mean Rev | RSI | 100 | 1.02 | 50% | 0.08 | 1.0% | 0.8% | ❌ REJECTED |
| 10 | Liquidity Sweep | Microstructure | 70 | 1.02 | 44% | 0.06 | 0.8% | 0.7% | ❌ REJECTED |
| 11 | Delta Absorption | Microstructure | 80 | 0.91 | 44% | -0.42 | 1.7% | 0.0% | ❌ REJECTED |
| 12 | Volume Profile VWAP | Volume | 60 | 1.19 | 53% | 0.67 | 0.5% | 8.5% | ❌ REJECTED |
## Rejected Strategies — Reasons

- **EMA Cross 9/21 BTC**: profit factor 1.26 < 1.30 threshold
- **RSI Slope Mean Rev**: profit factor 1.02 < 1.30 threshold
- **Liquidity Sweep**: profit factor 1.02 < 1.30 threshold
- **Delta Absorption**: profit factor 0.91 < 1.30 threshold
- **Volume Profile VWAP**: profit factor 1.19 < 1.30 threshold

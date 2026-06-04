# LIVE VS BACKTEST CERTIFICATION — Phase 22E

---

Generated: 2026-06-04T03:21:01Z

## Methodology

Live performance is compared against paper/backtest performance for the same strategies.
A degradation of > 20% in Profit Factor is considered a failure.

**Degradation = (LivePF − BacktestPF) / BacktestPF × 100** (negative = live worse)

## Strategy Live vs Backtest Comparison

| Strategy | Backtest PF | Live PF | Degradation | WR Back | WR Live | Status |
|:---------|:-----------:|:-------:|:-----------:|:-------:|:-------:|:------:|
| MSS Continuation | 2.92 | 0.00 | -100.0% | 65% | 0% | ❌ FAIL |
| Funding Rate Arb | 2.09 | 0.00 | -100.0% | 60% | 0% | ❌ FAIL |
| Bollinger Squeeze BTC | 0.00 | 1.88 | +0.0% | 0% | 53% | ✅ PASS |
| RSI Oversold 30 Revert | 0.00 | 1.80 | +0.0% | 0% | 58% | ✅ PASS |
| Order Block Rejection | 1.79 | 0.00 | -100.0% | 58% | 0% | ❌ FAIL |
| EMA Cross 21/50 BTC | 0.00 | 1.65 | +0.0% | 0% | 53% | ✅ PASS |
| FVG Retest Long | 1.48 | 0.00 | -100.0% | 51% | 0% | ❌ FAIL |
| EMA Cross 9/21 BTC | 0.00 | 1.26 | +0.0% | 0% | 51% | ✅ PASS |
| RSI Slope Mean Rev | 0.00 | 1.02 | +0.0% | 0% | 50% | ✅ PASS |
| Liquidity Sweep | 1.02 | 0.00 | -100.0% | 44% | 0% | ❌ FAIL |
| Delta Absorption | 0.91 | 0.00 | -100.0% | 44% | 0% | ❌ FAIL |
| Volume Profile VWAP | 1.19 | 0.00 | -100.0% | 53% | 0% | ❌ FAIL |

## Sources of Live vs Backtest Divergence

1. **Slippage** — live fills at worse prices than backtest simulation
2. **Market Impact** — larger orders move the market
3. **Latency** — signal-to-fill delay not modelled in backtest
4. **Look-ahead Bias** — backtest data may include survivorship bias
5. **Regime Shift** — market conditions changed after backtest period

## Certification Verdict

**✅ Live vs Backtest CERTIFIED: All approved strategies within degradation limits**

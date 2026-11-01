# Walk-forward swing test: chosen on January-June 2026, tested on July-October 2026

Protocol fingerprint 82a2d915f67f3a5c recorded 2026-11-01T17:15:10+00:00 before the first run. 16 USDT perpetuals, daily decisions, Delta India taker fee + slippage + real funding, no leverage.

- 390 variants in 15 families; 50 chosen on January-June (profit, 4+ of 6 months up, 6+ entries, drawdown <= 25%); one finalist per family.
- **2 of 13 finalists passed** July-October (profit and 3+ of 4 months up).
- Chosen variants made money in the test 18.0% of the time (median -12.81%), against 45.0% for the rest (median -2.40%); rank correlation between January-June and July-October returns -0.429.
- Buy and hold all 16 coins: January-June -43.94%, July-October +34.59%.

| Strategy | Capital | Jan-Jun | Months up | Jul-Oct | Months up | Result |
|---|---:|---:|---:|---:|---:|---|
| Funding carry, 7-day average, 2 each side, weekly (`X3|7|2|weekly`) | $40,000 | +11.17% | 5/6 | +28.20% | 3/4 | PASS |
| Cross-coin 5-day reversal, 1 each side, weekly (`X2|5|1|weekly`) | $20,000 | +22.96% | 4/6 | +10.05% | 3/4 | PASS |
| Cross-coin 56-day momentum, 1 each side, daily (`X1|56|1|daily`) | $20,000 | +36.18% | 5/6 | -1.70% | 1/4 | fail |
| 20-day trend (long and short) only when volatility is low (`V1|20|LS|low`) | $160,000 | +29.46% | 4/6 | -2.37% | 2/4 | fail |
| 5-day low reversal, hold 3 days (long and short) (`M3|5|3|LS`) | $160,000 | +11.88% | 4/6 | -4.58% | 1/4 | fail |
| One-day momentum after a 0.5-sd day (`S1|0.5|mom`) | $160,000 | +15.55% | 5/6 | -6.20% | 2/4 | fail |
| Monday long (`D1|0|long`) | $160,000 | +22.89% | 5/6 | -7.06% | 2/4 | fail |
| BTC 20-day trend steers all coins (short only) (`B1|20|SO`) | $160,000 | +41.47% | 5/6 | -14.00% | 1/4 | fail |
| Donchian 30/15 breakout (short only) (`T4|30/15|SO`) | $160,000 | +21.71% | 4/6 | -14.16% | 0/4 | fail |
| RSI(2) dip-buying under 20, exit above 70, above the 200-day average (long and short) (`M1|2|20|70|sma200|LS`) | $160,000 | +18.20% | 4/6 | -20.51% | 1/4 | fail |
| Price vs 100-day average (short only) (`T1|100|SO`) | $160,000 | +38.56% | 4/6 | -22.61% | 0/4 | fail |
| 20/100-day moving-average crossover (short only) (`T2|20/100|SO`) | $160,000 | +31.11% | 4/6 | -23.53% | 1/4 | fail |
| 90-day momentum, daily (short only) (`T5|90|daily|SO`) | $160,000 | +36.88% | 4/6 | -27.38% | 0/4 | fail |

**Funding carry, 7-day average, 2 each side, weekly**: Every Monday sell the 2 coins with the highest average funding rate over the last 7 days and buy the 2 with the lowest; hold until the next rebalance. The shorts collect funding from crowded longs.

| 01 | 02 | 03 | 04 | 05 | 06 | 07 | 08 | 09 | 10 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| +0.29% | +6.99% | +0.36% | -0.03% | +0.29% | +3.29% | +4.99% | +13.52% | +10.37% | -0.68% |

**Cross-coin 5-day reversal, 1 each side, weekly**: Every Monday buy the 1 coin that fell most over the last 5 days and sell the 1 that rose most; hold until the next rebalance.

| 01 | 02 | 03 | 04 | 05 | 06 | 07 | 08 | 09 | 10 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| +5.21% | -0.80% | +2.98% | -4.84% | +13.38% | +7.03% | +9.38% | +5.63% | -7.71% | +2.75% |


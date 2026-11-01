# Swing strategy lab: multi-day crypto strategies under a locked protocol

Generated 2026-11-01. Protocol fingerprint 9690f7acd1cf4342 recorded before the first run.

## Result

- 83 variants in 9 families; discovery 2018-2022, hold-out 2023 to Oct 2026 (finalists only).
- **0 of 6 finalists passed** (profit, daily t >= 2, both hold-out halves positive, 4 of 6 coins for single-coin families).

| Strategy | Discovery %/yr | t | Hold-out %/yr | t | Halves ($) | Max DD | Pass |
|---|---:|---:|---:|---:|---|---:|---|
| 20-day moving-average trend (long only) (`T1|20|LO`) | 64.41% | 3.23 | 32.77% | 1.86 | 80,429 / -5,073 | 49.24% | no |
| 10/50-day moving-average crossover (long and short) (`T2|10/50|LS`) | 84.57% | 2.66 | 17.74% | 0.72 | 20,806 / 19,981 | 66.73% | no |
| Donchian 20/10 breakout (long and short) (`T3|20/10|LS`) | 53.74% | 2.1 | 3.74% | 0.18 | 25,249 / -16,644 | 59.27% | no |
| 28-day time-series momentum, daily (long and short) (`T4|28|daily|LS`) | 89.88% | 2.94 | 18.76% | 0.79 | 42,632 / 511 | 71.01% | no |
| Cross-coin 28-day momentum, top/bottom 1 (`X1|28|1`) | 69.02% | 2.45 | -23.38% | -1.05 | 11,090 / -29,012 | 150.98% | no |
| Funding carry, 7-day average, 2 each side, daily (`X3|7|2|daily`) | 29.34% | 2.1 | 15.43% | 1.2 | 26,710 / -3,049 | 36.66% | no |

### 20-day moving-average trend (long only)

Every day, hold each coin long when yesterday's close is above its 20-day average, otherwise stay out. Capital $60,000.

| 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 6.72% | 32.19% | 92.28% | 223.82% | -33.02% | 50.71% | 83.79% | -12.78% | 3.88% |

### 10/50-day moving-average crossover (long and short)

Long while the 10-day average is above the 50-day average, short while below. Capital $60,000.

| 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.2% | 29.03% | 58.32% | 303.51% | 31.72% | -2.08% | 30.95% | 36.83% | 2.27% |

### Donchian 20/10 breakout (long and short)

Buy a close above the highest high of the last 20 days and sell when it closes below the lowest low of the last 10 (and the mirror for shorts). Capital $60,000.

| 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 5.91% | 5.8% | 69.39% | 183.63% | 3.95% | 7.99% | 32.29% | -19.56% | -6.38% |

### 28-day time-series momentum, daily (long and short)

Hold each coin in the direction of its last 28 days' return, rebalanced daily. Capital $60,000.

| 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 12.81% | 27.29% | 68.42% | 292.75% | 48.06% | 31.43% | 41.72% | 9.63% | -10.87% |

### Cross-coin 28-day momentum, top/bottom 1

Each Monday rank 12 coins by their last 28 days' return; long the top 1, short the bottom 1. Capital $20,000.

| 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 127.57% | 92.06% | 42.37% | 23.09% | 59.98% | 14.78% | 32.72% | -51.44% | -85.68% |

### Funding carry, 7-day average, 2 each side, daily

Rebalanced daily: rank coins by their average perpetual funding over 7 day(s); short the 2 highest (collect funding), long the 2 lowest. Capital $40,000.

| 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 0.0% | 0.0% | 3.43% | 144.6% | -1.34% | 31.11% | 45.51% | -28.45% | 10.98% |

## Benchmarks (always long, same costs and funding)

- BTC buy and hold: discovery 17.61%/yr (max DD 145.47%), hold-out 43.65%/yr (max DD 68.93%).
- 6-coin buy and hold: discovery 75.61%/yr (max DD 128.9%), hold-out 48.77%/yr (max DD 87.75%).

## Families

| Family | Variants | Profitable in discovery | Eligible |
|---|---:|---:|---:|
| T1 Price vs moving average | 8 | 8 | 3 |
| T2 Moving-average crossover | 6 | 6 | 1 |
| T3 Donchian breakout | 6 | 6 | 1 |
| T4 Time-series momentum | 20 | 20 | 3 |
| X1 Cross-coin momentum | 12 | 12 | 1 |
| X2 Cross-coin reversal | 9 | 0 | 0 |
| X3 Funding carry | 8 | 8 | 1 |
| M1 Daily RSI(2) dip-buying | 8 | 6 | 0 |
| S1 One-day momentum / reversal | 6 | 3 | 0 |

## Checks

- Positions and P&L of every finalist recomputed by independent loops: 0 position and 0 P&L mismatches.
- No look-ahead: re-run on data cut at 2024-07-01: 0 mismatches.
- Funding before each perpetual existed (2018-2019 mostly) is a synthetic 0.01% per 8 h: 44% of coin-days in discovery.

## Reproduce
```
cd engine/research/swing_lab_2026_11
python -I sw_fetch.py && python -I sw_lab.py && python -I sw_verify.py && python -I sw_report.py
```

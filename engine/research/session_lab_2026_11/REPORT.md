# Session strategy lab: searching for intraday crypto strategies that survive costs

Generated 2026-11-01. Protocol and grid fingerprinted before the first run (42a0edf94fe51590).

## Result

- 880 variants in 5 families were run on BTC, ETH, SOL, XRP, DOGE and BNB perpetuals with Delta India costs.
- Discovery (2021-2023): 1 variants made money in each of the three years with daily t >= 2; 1 finalists were chosen by the pre-set rule.
- Hold-out (Jan 2024 - Sep 2025, unseen until then): **0 of 1 passed** (profit, daily t >= 1.64, profit in both halves, and on 4 of 6 coins).

## Finalists

| Strategy | Discovery 2021-23 | t | Hold-out 2024-Sep 25 | t | Halves | Coins + | Pass | Recent Oct 25-Oct 26 |
|---|---:|---:|---:|---:|---|---:|---|---:|
| US-open 60-min range breakout (`A|US|60|stop|far|1.5|nr4`) | +$21,778 | 2.69 | −$12,517 | −2.15 | −$6,445 / −$6,072 | 1 | no | +$5,694 |

### US-open 60-min range breakout 

Opening range = the first 60 minutes from the NYSE open (13:30 UTC in US summer time, 14:30 in winter), weekdays only. Rest buy-stop and sell-stop orders at the two range edges; the first one touched is the trade, until 3 h 15 min after the open. Stop at the far side of the range, a limit target at 1.5x the risk; exit at the NYSE close. Only after a day whose range was the narrowest of the last four.

| 2021 | 2022 | 2023 | 2024 | 2025 | 2026 |
|---:|---:|---:|---:|---:|---:|
| +$12,804 | +$5,310 | +$3,664 | −$10,026 | +$2,626 | +$577 |

Hold-out by coin: BTC −$1,016, ETH −$672, SOL −$2,852, XRP −$3,149, DOGE −$4,895, BNB +$66

## Families

| Family | Variants | Profitable in discovery | Profitable before costs | Eligible |
|---|---:|---:|---:|---:|
| Session opening-range breakout | 648 | 66 | 481 | 1 |
| Trend-day continuation | 48 | 33 | 44 | 0 |
| Cross-coin intraday | 24 | 4 | 12 | 0 |
| Volatility breakout | 24 | 9 | 24 | 0 |
| Time-of-day drift | 136 | 0 | 68 | 0 |

## Checks

- Independent re-fill of all finalist trades: 1,664 trades, 0 mismatches.
- Engine check on 12 variants covering every entry type: 79,229 trades, 0 mismatches.
- No look-ahead: re-run on data cut at 2024-07-01: 1,028 earlier trades, 0 mismatches.

## Reproduce

```
cd engine/research/session_lab_2026_11
python -I sl_fetch.py && python -I sl_run.py && python -I sl_verify.py && python -I sl_report.py
```

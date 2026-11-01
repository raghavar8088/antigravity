# BTC deep backtest: 334 registry strategies, January 2018 to October 2026

Generated 2026-11-01 by `engine/research/btc_deep_2026_10/deep_report.py`. Every number below is reproducible from the scripts in this folder and `engine/cmd/deep_backtest`.

## Verdict

- Every OHLCV-compatible strategy in the scalpers registry (334) was replayed on 4,832,478 Binance BTCUSDT one-minute bars, 2017-08-17 to 2026-10-30, trading from 2018-01-01: 339,793 trades.
- Before costs 240 strategies make money. After Delta India's taker fee (0.05% + 18% GST) and 2 bp of spread and slippage a leg, 107 do. The average trade goes from 2.41 bp gross to −13.16 bp net.
- No strategy clears the multiple-testing bar. The best full-period t-statistic is 2.84; the largest t that luck alone produces among 334 tries is about 3.41, and the Bonferroni 5% bar is 3.62.
- The ten best over the whole period, as an equal-weight book ($10,000 each, 1x notional), made +53.1% in 8.8 years, max drawdown 3.6%. That list is chosen with hindsight.
- Chosen the honest way, on 2018-2022 only, the ten best made +47.1% there and −4.9% over 2023-01 to 2026-10. Ranking a backtest like this does not pick strategies that keep working.
- The engine's existing backtest harness (the one that qualified the live 49-strategy whitelist) evaluates every strategy on data 3h30m old (100% of 309,188 evaluations), and on 12.5% of them the newest 4h candle closes after the entry. Under it 203 strategies look profitable; on the corrected clock at the same cost, 122.

## The top 10, ranked on the whole period (hindsight)

Rule: t-statistic of net return per trade over 2018-01-01 to 2026-10-30, at least 80 trades, positive net; a strategy whose monthly returns correlate above 0.7 with one already chosen is skipped (Fisher_Zero_Cross_Short, WR_RSI_Bear_Confirm_Short, WR_Mid_EFI_ADX_Short were near-copies of a higher one). Costs: Delta India taker both legs plus 2 bp, signed Binance funding. $10,000 per strategy, every trade $10,000 notional.

| # | Strategy | Trades | Per year | Win rate | Profit factor | Net 2018-26 | Max DD | t | 2018-22 | 2023-26 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | `WMA5_Cross_WMA21_EFI_ADX_Short` | 174 | 19.7 | 66.1% | 1.59 | +92.3% | 25.3% | 2.84 | +65.4% | +26.9% |
| 2 | `WR_Mid_Cross_Bear_Short` | 198 | 22.4 | 78.3% | 1.66 | +74.2% | 10.4% | 2.65 | +62.9% | +11.2% |
| 3 | `EMA8_Cross_WMA21_EFI_ADX_Short` | 102 | 11.6 | 69.6% | 1.67 | +54.5% | 11.2% | 2.47 | +32.1% | +22.4% |
| 4 | `KST_Below_Zero_Bear_Short` | 317 | 35.9 | 75.1% | 1.29 | +58.3% | 20.8% | 1.73 | +47.3% | +11.0% |
| 5 | `OBV_Slope_Bear_Short` | 205 | 23.2 | 77.6% | 1.42 | +80.9% | 23.5% | 1.69 | +65.7% | +15.2% |
| 6 | `Price_EMA21_Breakdown_Short` | 119 | 13.5 | 78.2% | 1.48 | +31.4% | 10.4% | 1.63 | +32.5% | −1.1% |
| 7 | `MACD_Hist_Sign_Flip_Short` | 160 | 18.1 | 75.6% | 1.36 | +37.5% | 21.4% | 1.58 | +27.4% | +10.1% |
| 8 | `WMA5_Cross_EMA13_EFI_ADX_Short` | 137 | 15.5 | 62.0% | 1.32 | +43.5% | 14.8% | 1.49 | +25.3% | +18.2% |
| 9 | `ADX_Strong_Trend_Bear_Short` | 180 | 20.4 | 77.2% | 1.30 | +32.3% | 18.7% | 1.47 | +1.7% | +30.6% |
| 10 | `MACD_Line_Zero_Cross_Short` | 136 | 15.4 | 77.2% | 1.34 | +26.2% | 18.8% | 1.46 | +14.7% | +11.5% |
| | **Book of ten** | 1728 | | | | **+53.1%** | 3.6% | | +37.7% | +15.4% |

What each one does (the registry's own description):

1. `WMA5_Cross_WMA21_EFI_ADX_Short`: 4h WMA5 cross↓WMA21 + EFI<0+ADX>20 — fast pure-WMA cross, Elder Force confirmed (4h/1h; long share 0.0%, average hold 22.9 h, target hit on 66.1% of trades).
2. `WR_Mid_Cross_Bear_Short`: 4h WR cross↓-50 midline+EMA down+MACD-+ADX>18 (4h/1h; long share 0.0%, average hold 11.5 h, target hit on 78.3% of trades).
3. `EMA8_Cross_WMA21_EFI_ADX_Short`: 4h EMA8 cross↓WMA21 + EFI<0+ADX>22+MACD- — mixed-MA cross with Elder Force Index confirmation (4h/1h; long share 0.0%, average hold 24.7 h, target hit on 69.6% of trades).
4. `KST_Below_Zero_Bear_Short`: 4h KST cross↓0+EMA down+MACD- (4h/1h; long share 0.0%, average hold 14.7 h, target hit on 75.1% of trades).
5. `OBV_Slope_Bear_Short`: 4h OBV declining 5 bars+EMA down+MACD-+ADX>20 (4h/1h; long share 0.0%, average hold 13.0 h, target hit on 77.1% of trades).
6. `Price_EMA21_Breakdown_Short`: 4h close crosses below EMA21+EMA8 already below+MACD- (4h/1h; long share 0.0%, average hold 15.5 h, target hit on 78.2% of trades).
7. `MACD_Hist_Sign_Flip_Short`: 4h MACD hist flips +→−+EMA8<EMA21+ADX>20+RSI4h 30-68 (4h/1h; long share 0.0%, average hold 9.4 h, target hit on 75.6% of trades).
8. `WMA5_Cross_EMA13_EFI_ADX_Short`: 4h WMA5 cross↓EMA13 + EFI<0+ADX>20 — fast mixed-MA cross, Elder Force confirmed (4h/1h; long share 0.0%, average hold 22.7 h, target hit on 61.3% of trades).
9. `ADX_Strong_Trend_Bear_Short`: 4h ADX cross↑28+EMA down+MACD-+RSI4h 25-58 (4h/1h; long share 0.0%, average hold 14.0 h, target hit on 77.2% of trades).
10. `MACD_Line_Zero_Cross_Short`: 4h MACD line cross↓0+EMA down+ADX>18+RSI4h<60 (4h/1h; long share 0.0%, average hold 13.7 h, target hit on 77.2% of trades).

### Yearly net return, % of the strategy's $10,000

| Strategy | 2018 | 2019 | 2020 | 2021 | 2022 | 2023 | 2024 | 2025 | 2026 to 30 Oct |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `WMA5_Cross_WMA21_EFI_ADX_Short` | +30.4% | +5.3% | +20.6% | −2.2% | +11.3% | −2.4% | +18.0% | +1.2% | +10.2% |
| `WR_Mid_Cross_Bear_Short` | +14.8% | +9.4% | −0.8% | +40.2% | −0.7% | +1.9% | +4.3% | +5.8% | −0.8% |
| `EMA8_Cross_WMA21_EFI_ADX_Short` | +6.7% | −4.8% | +18.6% | +3.6% | +7.9% | −3.0% | +10.4% | +7.2% | +7.8% |
| `KST_Below_Zero_Bear_Short` | +19.5% | +11.8% | +6.6% | +6.3% | +3.1% | −3.9% | +18.0% | +4.8% | −7.9% |
| `OBV_Slope_Bear_Short` | +46.5% | +4.0% | −7.3% | +10.3% | +12.2% | +2.0% | +2.9% | +3.5% | +6.8% |
| `Price_EMA21_Breakdown_Short` | +3.7% | +6.3% | +5.4% | +10.6% | +6.5% | +5.8% | −5.0% | −4.6% | +2.7% |
| `MACD_Hist_Sign_Flip_Short` | +12.7% | −0.5% | +11.9% | +18.9% | −15.5% | +4.1% | +1.5% | +6.3% | −1.7% |
| `WMA5_Cross_EMA13_EFI_ADX_Short` | +13.9% | +3.7% | +2.7% | −3.6% | +8.5% | −1.9% | +17.8% | +5.7% | −3.5% |
| `ADX_Strong_Trend_Bear_Short` | +11.3% | −2.4% | −0.3% | −2.7% | −2.2% | +7.7% | +5.4% | +7.5% | +8.1% |
| `MACD_Line_Zero_Cross_Short` | +12.7% | −7.5% | −3.1% | +14.7% | −2.1% | +5.1% | +7.2% | +4.2% | −5.0% |
| **Book of ten** | **+17.2%** | **+2.5%** | **+5.4%** | **+9.6%** | **+2.9%** | **+1.5%** | **+8.1%** | **+4.2%** | **+1.7%** |
| BTC buy and hold | −73% | +94% | +302% | +60% | −64% | +156% | +121% | −6% | −12% |

### Monthly net return, % of the strategy's $10,000

Months are booked on the exit date. A blank month had no closed trade.

#### 1. WMA5_Cross_WMA21_EFI_ADX_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +5.5% |  | +7.2% | +3.1% | +3.4% | −0.4% |  | +4.6% | +3.8% | +1.8% | +3.3% | −1.9% | +30.4% |
| 2019 | −1.9% | −1.9% | +1.6% | −3.7% |  | −2.7% | +8.9% | +1.7% |  | +3.3% |  |  | +5.3% |
| 2020 | +3.3% | +3.4% | +6.0% | +1.6% | +1.9% |  | −2.0% | +3.6% | −3.0% | +1.6% | +4.0% |  | +20.6% |
| 2021 | −5.3% |  | +7.2% | 0.0% | +2.8% | +4.8% |  | +1.7% | −1.1% |  | −2.5% | −9.7% | −2.2% |
| 2022 | −2.9% | +1.6% | −2.9% | +3.3% | +2.4% |  | +2.1% | +2.1% | −2.9% | +1.7% | +5.0% | +1.7% | +11.2% |
| 2023 |  |  | +1.3% | −0.8% | −1.9% | +3.3% | −3.9% | −0.2% |  |  | −1.9% | +1.7% | −2.4% |
| 2024 | −2.6% |  | −1.1% | +9.1% | −2.8% | +6.8% | +1.8% |  | +1.7% | +1.7% | +1.6% | +1.7% | +18.0% |
| 2025 | −6.7% | +2.3% | +1.6% | +1.6% | −2.4% | +1.7% | −2.2% | +4.9% | +3.3% |  | −0.3% | −2.7% | +1.2% |
| 2026 |  | +4.4% | +1.6% |  | +3.0% | +1.6% | −2.0% | +1.7% |  | −0.2% |  |  | +10.1% |

#### 2. WR_Mid_Cross_Bear_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +9.0% |  |  |  |  |  | +2.2% |  | −1.2% | +2.6% | +2.2% |  | +14.8% |
| 2019 |  | +0.8% | +1.1% | −1.6% |  | +1.9% | +1.3% | +4.5% | +1.6% | +0.4% | −2.9% | +2.2% | +9.4% |
| 2020 | +1.5% | −3.3% |  |  |  | −1.2% | −0.3% | −2.1% |  | +0.8% | −2.3% | +6.2% | −0.8% |
| 2021 | +3.2% | +10.2% | +3.4% | −3.0% | +9.8% | +10.0% | +2.1% | +3.4% | +1.2% |  | −0.3% |  | +40.2% |
| 2022 | +4.1% | −2.1% | −3.6% |  | −1.7% | +2.2% | +0.9% | −1.9% |  | +1.5% |  |  | −0.7% |
| 2023 |  |  | −2.5% | −1.2% |  |  | +1.5% | +0.8% | +3.0% |  | −1.9% | +2.3% | +1.9% |
| 2024 | −1.8% | −1.9% | +6.6% |  |  | +0.7% |  | +3.2% |  | −1.9% | +2.2% | −2.9% | +4.3% |
| 2025 | +1.6% | +1.5% | +0.1% | +0.8% | +1.5% |  | −2.4% | +0.7% |  |  | −0.5% | +2.4% | +5.8% |
| 2026 | +0.7% |  |  | −1.2% | +0.8% | −0.1% |  | −1.9% | +1.5% | −0.4% |  |  | −0.8% |

#### 3. EMA8_Cross_WMA21_EFI_ADX_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 |  |  |  | +3.1% | +3.4% | +1.7% | −2.0% | −6.5% | +3.8% | −0.2% | +3.3% |  | +6.7% |
| 2019 |  | −1.9% | +1.7% | −3.7% |  | −2.4% | −0.8% | +1.7% |  | +3.3% |  | −2.7% | −4.8% |
| 2020 | +1.6% |  | +4.2% | +1.6% | +1.9% |  | −0.1% | +3.6% |  | +1.6% | +4.0% |  | +18.6% |
| 2021 |  |  |  | +1.9% | +2.0% | −4.1% |  | +1.7% | +1.7% |  |  | +0.5% | +3.6% |
| 2022 | +5.6% |  | +3.0% |  | +0.1% |  |  | +2.1% | −5.8% |  | −0.3% | +3.3% | +7.9% |
| 2023 |  |  |  | +1.6% | −2.2% | +1.6% | −1.9% | −1.9% | +1.7% |  | −1.9% |  | −3.0% |
| 2024 |  |  |  | +4.6% | −2.8% | +3.4% | +1.8% |  | +1.7% |  |  | +1.7% | +10.4% |
| 2025 | −0.7% | +1.6% | +1.6% | +1.6% |  | +1.7% | −3.9% | +3.3% | +1.6% |  | −1.9% | +2.3% | +7.2% |
| 2026 | −1.9% | +1.6% | +3.3% |  | +3.3% |  |  | −0.2% |  | +1.7% |  |  | +7.7% |

#### 4. KST_Below_Zero_Bear_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +4.8% | +3.0% | +3.1% | +1.5% | +0.8% | +4.9% | +2.2% |  | −3.7% | +0.7% | −1.2% | +3.3% | +19.5% |
| 2019 | +2.5% | −3.3% | +0.8% | −1.6% | −3.7% | +7.9% | +3.3% | +1.0% | +3.0% | +1.5% | −0.3% | +0.8% | +11.8% |
| 2020 | +1.5% | +0.3% | +1.4% | −0.9% | −1.0% | +2.6% | +1.6% | −0.2% | +2.3% | −5.0% | +3.2% | +0.8% | +6.6% |
| 2021 | −19.0% | +11.9% | +1.6% | +2.9% | +2.3% | +5.1% | +2.6% | −6.2% | +4.9% | +2.4% | −1.9% | −0.2% | +6.3% |
| 2022 | +1.5% | +0.6% | +2.9% | −3.6% | −4.2% | −1.7% | +3.8% | 0.0% | +1.1% | +0.3% | +2.8% | −0.4% | +3.1% |
| 2023 |  | −2.0% | +1.9% | −4.6% | −0.5% | −1.2% | +1.8% | −0.4% | −0.4% | +0.7% | −4.5% | +5.4% | −3.9% |
| 2024 |  | −3.5% | +2.5% | +5.0% | −4.4% | +0.8% | +3.3% | +1.5% |  | +2.3% | +2.2% | +8.3% | +18.0% |
| 2025 | +1.2% | +4.5% | −0.2% | −2.3% | −2.4% | −1.3% | −1.6% | +2.5% | +0.8% | +3.6% | +0.9% | −0.8% | +4.8% |
| 2026 | +2.2% | −3.9% | +0.3% | −1.2% | −1.2% | −2.1% | −5.0% | −1.2% | +1.5% | +2.7% |  |  | −7.9% |

#### 5. OBV_Slope_Bear_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +10.3% | +25.4% | +5.8% | −4.7% | +8.6% | −1.2% |  | 0.0% |  | +1.0% | +2.4% | −1.2% | +46.5% |
| 2019 | +3.3% | −1.9% |  |  |  |  | +2.9% | +1.1% | −2.0% |  | −0.9% | +1.5% | +4.0% |
| 2020 |  | +1.0% | −3.3% |  | +1.8% | −1.2% | +0.8% | −3.2% | +1.3% |  | −4.4% |  | −7.3% |
| 2021 |  |  |  | +3.4% | −1.9% | +2.6% | +1.2% | −0.6% | +2.1% |  | +2.3% | +1.1% | +10.3% |
| 2022 | +8.4% |  | +1.3% | −2.5% | −5.7% | +8.5% | +1.4% | +0.2% | −3.4% | +1.6% | +1.7% | +0.8% | +12.2% |
| 2023 |  |  | −3.7% | +4.0% | −0.6% | +0.7% |  | +4.0% |  | +0.8% |  | −3.2% | +2.0% |
| 2024 | +4.1% | +0.8% | −3.3% | +0.8% | +1.3% | −0.3% | +0.8% |  |  | −1.2% |  |  | +2.9% |
| 2025 | −1.7% |  | 0.0% | −0.9% |  |  |  | +0.3% | +2.2% | +2.6% | +0.1% | +0.7% | +3.5% |
| 2026 | −1.9% | +1.2% | +3.1% |  | +2.2% |  | +0.8% | +0.8% | −1.9% | +2.6% |  |  | +6.8% |

#### 6. Price_EMA21_Breakdown_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | −0.5% |  |  |  | +1.2% |  |  |  |  | +0.8% | +2.2% |  | +3.7% |
| 2019 |  | +0.8% | −0.3% | −1.6% |  | +3.3% |  | +2.8% | +3.6% | +0.8% | −2.9% |  | +6.3% |
| 2020 |  | +2.6% |  |  |  | +1.5% | +0.8% | −1.0% |  | +0.8% |  | +0.8% | +5.4% |
| 2021 | +4.5% |  | +1.3% | +1.8% | −0.1% | +3.1% |  | +2.2% |  |  | −2.4% |  | +10.6% |
| 2022 | +0.8% | +2.7% |  |  | −2.7% | +3.0% | +3.2% |  |  | +1.5% |  | −2.0% | +6.5% |
| 2023 |  | +1.6% | −1.1% | +0.7% |  |  | +1.1% | +0.8% |  |  | −1.9% | +4.6% | +5.8% |
| 2024 |  | −1.9% |  |  |  |  |  | +0.9% |  | −1.9% |  | −2.1% | −5.0% |
| 2025 | +1.6% |  | −1.9% | −2.3% | +2.2% |  | −2.4% | +0.7% |  | +0.8% | −1.4% | −1.9% | −4.6% |
| 2026 | +1.5% |  |  | −1.2% | +0.8% | +1.0% | −0.4% | −1.9% | +1.5% | +1.5% |  |  | +2.7% |

#### 7. MACD_Hist_Sign_Flip_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | −4.5% | +4.0% | +5.0% | +3.5% | +1.7% |  |  |  |  | +1.6% | +1.5% |  | +12.7% |
| 2019 |  | +0.8% | +0.8% | −2.8% |  | −0.8% |  | +0.8% |  |  | +0.8% |  | −0.5% |
| 2020 | +1.5% |  | +6.1% |  | +3.2% | −1.2% | +1.5% |  |  | +0.8% |  |  | +11.8% |
| 2021 |  | +2.1% |  | +2.2% | +13.2% | +4.1% | −0.5% |  | 0.0% |  | +1.4% | −3.8% | +18.9% |
| 2022 | −2.8% | −5.7% | −3.6% | +1.3% | −3.0% |  |  | +1.1% | −2.5% | +0.8% |  | −1.2% | −15.5% |
| 2023 |  | +0.8% | +1.1% | +1.5% | +1.5% | +0.8% | +0.7% | −1.9% | −0.5% |  |  |  | +4.1% |
| 2024 |  |  | +4.1% | +0.4% |  | +2.2% | +0.1% |  | −3.1% |  |  | −2.4% | +1.5% |
| 2025 |  | −2.7% |  | +1.9% |  | +1.5% | −1.2% | +1.5% | +2.2% | +1.7% | +1.4% |  | +6.3% |
| 2026 |  | −1.1% |  | −1.2% | +1.5% | +0.3% |  | −3.1% | +2.4% | −0.4% |  |  | −1.7% |

#### 8. WMA5_Cross_EMA13_EFI_ADX_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +5.5% |  |  |  | +1.6% | −0.4% |  | +2.3% | +3.5% | −1.9% | +3.3% |  | +14.0% |
| 2019 | −1.9% | −1.9% | −1.9% | −3.5% |  | −5.0% | +14.9% | +1.7% |  | +1.4% |  |  | +3.7% |
| 2020 |  | +1.8% | −1.9% |  | +1.9% | −1.9% | −3.2% | +3.4% | −3.0% | +1.6% | +4.0% |  | +2.7% |
| 2021 | −8.0% |  | +3.3% | −0.7% | +5.5% |  |  |  | −1.1% |  | −2.7% |  | −3.6% |
| 2022 | +0.8% |  |  | +1.7% | +2.4% |  | +2.1% | +2.1% | −3.8% |  | +1.6% | +1.7% | +8.5% |
| 2023 |  |  |  | +1.6% | −4.1% | +1.6% | −3.9% | −2.1% |  | +1.7% | −1.9% | +5.2% | −1.9% |
| 2024 | −2.6% | −1.9% | +2.2% | +6.3% | +1.6% | +5.4% | +1.8% |  | +1.7% |  | +1.6% | +1.7% | +17.8% |
| 2025 | −0.7% | −0.3% | +1.6% | +3.7% | −2.4% | +3.3% | −5.8% | +3.3% | +3.3% |  | −2.0% | +1.6% | +5.7% |
| 2026 |  | −2.3% |  |  | +1.3% |  | −3.9% | +1.7% |  | −0.2% |  |  | −3.5% |

#### 9. ADX_Strong_Trend_Bear_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 |  |  | +1.7% | −4.7% | +1.2% | +1.1% |  |  |  | +0.8% | +3.0% | +8.1% | +11.3% |
| 2019 |  | −1.9% |  |  |  |  |  |  | −2.1% | −0.9% | +0.9% | +1.6% | −2.4% |
| 2020 | +1.5% | +2.6% |  | −3.8% |  | −1.4% | +0.8% |  |  |  |  |  | −0.3% |
| 2021 | +2.4% |  | −4.7% |  | +7.9% | −0.1% | −2.2% | +1.4% |  | −3.3% | +2.3% | −6.3% | −2.7% |
| 2022 | +6.3% | −2.1% | −3.6% | +2.1% | −0.5% | −1.1% | +1.4% | +2.0% |  |  | −1.6% | −5.1% | −2.3% |
| 2023 | −1.9% | +3.7% | +1.5% | +3.1% | +1.1% | −1.3% | −1.8% | +2.3% |  |  |  | +0.8% | +7.7% |
| 2024 |  |  |  | −2.0% | +0.4% | +2.4% |  | +1.2% | +2.4% | +0.8% | +1.5% | −1.2% | +5.4% |
| 2025 | +0.8% | −1.0% |  | +1.5% | +0.8% | +0.8% | −1.2% | +2.2% | −1.9% | +0.8% | +4.9% |  | +7.5% |
| 2026 | +0.7% | +0.9% | +3.8% | −2.0% | +0.3% | +3.7% |  | −2.4% | +0.7% | +2.2% |  |  | +8.1% |

#### 10. MACD_Line_Zero_Cross_Short

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +1.5% |  | +1.8% |  | +2.6% | +3.1% |  |  | +0.8% | +4.0% | +3.0% | −4.2% | +12.7% |
| 2019 |  | +0.1% | +0.8% | −2.3% | −5.3% |  |  | +1.3% | −2.0% |  |  |  | −7.5% |
| 2020 | −1.1% | −0.3% | +1.7% | −7.5% |  | +2.5% | +0.8% |  |  |  |  | +0.8% | −3.1% |
| 2021 |  |  |  | +2.1% | +4.3% | +2.8% |  |  | +3.4% |  | +2.1% |  | +14.6% |
| 2022 | +0.8% | +0.9% |  | −0.1% |  | −3.3% | +0.4% | −2.7% | +1.4% | +0.8% | +3.6% | −3.9% | −2.1% |
| 2023 |  | +1.5% |  | +2.2% | +2.4% | −1.2% |  |  | +0.7% |  | −2.9% | +2.3% | +5.1% |
| 2024 |  | −2.2% |  | +2.3% | −2.5% |  | +4.5% |  | +0.7% | +0.8% | +1.4% | +2.2% | +7.2% |
| 2025 | +1.5% |  | +2.0% |  | −1.2% | −0.1% | −1.2% | +1.5% |  | +1.6% |  |  | +4.2% |
| 2026 | −1.2% |  | +2.2% | −4.0% | +0.8% |  | −1.9% | −1.2% | −1.2% | +1.5% |  |  | −5.0% |

#### Book of ten (equal weight)

| Year | Jan | Feb | Mar | Apr | May | Jun | Jul | Aug | Sep | Oct | Nov | Dec | Year |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 2018 | +3.2% | +3.2% | +2.5% | +0.2% | +2.5% | +0.9% | +0.2% | 0.0% | +0.7% | +1.1% | +2.3% | +0.4% | +17.2% |
| 2019 | +0.2% | −1.1% | +0.4% | −2.1% | −0.9% | +0.2% | +3.0% | +1.6% | +0.2% | +1.0% | −0.5% | +0.3% | +2.6% |
| 2020 | +1.0% | +0.8% | +1.4% | −0.9% | +1.0% | 0.0% | +0.1% | +0.4% | −0.2% | +0.2% | +0.9% | +0.9% | +5.4% |
| 2021 | −2.2% | +2.4% | +1.2% | +1.1% | +4.6% | +2.8% | +0.3% | +0.4% | +1.1% | −0.1% | −0.2% | −1.9% | +9.6% |
| 2022 | +2.3% | −0.4% | −0.7% | +0.2% | −1.3% | +0.8% | +1.5% | +0.5% | −1.6% | +0.8% | +1.3% | −0.5% | +2.9% |
| 2023 | −0.2% | +0.6% | −0.1% | +0.8% | −0.4% | +0.5% | −0.6% | +0.1% | +0.5% | +0.3% | −1.7% | +1.9% | +1.6% |
| 2024 | −0.3% | −1.1% | +1.1% | +2.6% | −0.9% | +2.2% | +1.4% | +0.7% | +0.5% | 0.0% | +1.1% | +0.7% | +8.1% |
| 2025 | −0.3% | +0.6% | +0.5% | +0.6% | −0.4% | +0.8% | −2.2% | +2.1% | +1.2% | +1.1% | +0.1% | +0.2% | +4.2% |
| 2026 | 0.0% | +0.1% | +1.4% | −1.1% | +1.3% | +0.4% | −1.3% | −0.8% | +0.5% | +1.1% |  |  | +1.6% |

## Does the ranking predict anything?

The same rule run on 2018-2022 only (104 eligible strategies), then frozen:

| # | Strategy | 2018-22 trades | 2018-22 net | t | 2023-26 trades | 2023-26 net | t |
|---|---|---:|---:|---:|---:|---:|---:|
| 1 | `WR_RSI_Bear_Confirm_Short` | 124 | +63.0% | 2.49 | 65 | +8.8% | 0.78 |
| 2 | `WMA5_Cross_WMA21_EFI_ADX_Short` | 105 | +65.4% | 2.32 | 69 | +26.9% | 1.65 |
| 3 | `Donchian_EMA_MACD_Bear_Short` | 202 | +69.8% | 2.03 | 148 | −21.1% | −1.12 |
| 4 | `Keltner_EMA_MACD_Bear_Short` | 170 | +51.1% | 1.97 | 123 | −30.9% | −1.83 |
| 5 | `EMA9_Cross_EFI_ADX_Long` | 52 | +26.1% | 1.95 | 57 | −9.0% | −0.78 |
| 6 | `Donchian_Mid_Break_Short` | 85 | +44.3% | 1.95 | 55 | −7.7% | −0.68 |
| 7 | `Price_EMA21_Breakdown_Short` | 70 | +32.5% | 1.94 | 49 | −1.1% | −0.12 |
| 8 | `Supertrend_Flip_Short` | 130 | +40.0% | 1.69 | 85 | −25.5% | −1.91 |
| 9 | `WMA21_Cross_EMA55_EFI_ADX_Short` | 87 | +45.1% | 1.68 | 58 | −10.0% | −0.61 |
| 10 | `EMA8_Cross_WMA21_EFI_ADX_Short` | 62 | +32.1% | 1.68 | 40 | +22.4% | 1.97 |
| | **Book of ten** | | **+47.1%** | | | **−4.9%** | |

3 of the 10 stayed positive out of sample; the book's monthly Sharpe went from 1.08 over the whole run to −0.35 in the hold-out. Across the universe, 139 strategies were net-positive in 2018-2022, 71 in 2023-2026, and 40 in both.

## The existing harness trades on 3.5-hour-old data

`cmd/deep_backtest -mode probe` runs a probe strategy through `backtest.ScalerV3Adapter` + the v3 engine and records what it can see when it is evaluated (the entry fills at that tick):

- The newest 15m bar in the context is 3h30m old at the entry tick, on 100% of 309,188 evaluations. The v3 engine skips strategies until its 60-price warm-up is full (15 candles), but the adapter numbers candles by counting the calls it receives, so it builds the context 14 candles behind the tick.
- On 12.5% of evaluations the newest 4h candle closes after the entry tick: `ContextBuilder.BuildAt` includes the 1h and 4h candles still forming at its cut-off, with their final high, low and close.
- `ctx.Now` is the open of the context bar, 15 minutes before the data's cut-off; `ctx.Price` is that bar's close.
- Effect over 2018-2026 at the engine's own Binance cost: 203 of 334 strategies net-positive under the harness against 122 on the corrected clock; rank correlation 0.58. The six best results under the harness:

| Strategy | Harness trades | Harness net | Corrected net |
|---|---:|---:|---:|
| `Close_High_Range_ADX_Long` | 1856 | +343.4% | −97.0% |
| `Close_Low_Range_Bear_Short` | 961 | +274.8% | −62.9% |
| `BB_Width_Expand_Bear_Short` | 1326 | +272.6% | +57.3% |
| `Close_Low_Range_ADX_Short` | 1162 | +268.0% | −79.5% |
| `BB_Squeeze_EFI_ADX_Short` | 873 | +249.1% | +69.6% |
| `Big_Bear_Candle_Short` | 319 | +235.7% | −18.4% |

`Close_High_Range_ADX_Long` buys when the latest 4h candle closes in the top 20% of its range. Under the harness that candle can still be forming yet already carry its final close, so the rule knows the bar will finish strong before it buys: +343.4% there, −97.0% on the corrected clock. `Big_Bear_Candle_Short` is the same mechanism on the short side.

## The live 49-strategy whitelist on the corrected clock

`data/btc_prelive_whitelist.json` was qualified by `btc_qualify_25` on the harness above (Binance 2021-07 to 2026-07, validate 2025-01-15 to 2026-07-17). On the corrected clock with Delta costs, 32 of 49 are net-positive in that validate window and 24 of 49 over 2018-2026. In the top 10 above: `OBV_Slope_Bear_Short`, `ADX_Strong_Trend_Bear_Short`, `Price_EMA21_Breakdown_Short`, `EMA8_Cross_WMA21_EFI_ADX_Short`.

| Strategy | Trades 2018-26 | Net 2018-26 | Validate-window net | Harness net 2018-26 |
|---|---:|---:|---:|---:|
| `OBV_Slope_Bear_Short` | 205 | +80.9% | +10.5% | +141.2% |
| `WR_Mid_EFI_ADX_Short` | 173 | +70.4% | +6.3% | +97.1% |
| `Three_Bear_Candles_ADX_Short` | 677 | +55.8% | +1.7% | +183.6% |
| `EMA8_Cross_WMA21_EFI_ADX_Short` | 102 | +54.5% | +13.6% | +38.6% |
| `BB_Squeeze_EFI_ADX_Short` | 683 | +50.5% | +11.2% | +249.1% |
| `Three_Bear_Candles_Short` | 713 | +49.7% | +3.2% | +168.0% |
| `OBV_Bear_CMF_Short` | 461 | +37.0% | −2.2% | +169.7% |
| `ADX_Strong_Trend_Bear_Short` | 180 | +32.3% | +14.2% | +52.7% |
| `Price_EMA21_Breakdown_Short` | 119 | +31.4% | −2.6% | +80.8% |
| `ATR_CMF_Expand_Bear_Short` | 73 | +26.5% | +5.7% | +89.4% |
| `Bearish_Momentum_EFI_ADX_Short` | 545 | +20.1% | +6.8% | +213.2% |
| `WMA13_WMA34_EFI_Bear_Short` | 250 | +19.7% | −5.7% | +32.4% |
| `WMA13_WMA34_Bear_Cross_Short` | 260 | +17.3% | −10.8% | +31.2% |
| `EMA9_Cross_EMA21_Short` | 205 | +12.3% | −6.0% | +54.4% |
| `Chandelier_Bear_Break_Short` | 173 | +12.2% | +4.9% | +76.7% |
| `EMA8_Cross_CMF_ADX_Short` | 63 | +11.0% | +1.3% | +25.8% |
| `Coppock_ADX_Bear_Short` | 194 | +9.2% | +2.3% | +55.6% |
| `WMA13_CMF_Bear_Short` | 75 | +8.7% | +3.7% | +27.2% |
| `ZLEMA_CMF_Bear_Short` | 190 | +8.0% | −4.3% | +56.4% |
| `WMA9_EFI_ADX_Bear_Short` | 72 | +7.9% | −1.8% | +28.7% |
| `WMA13_WMA34_ADX_Bear_Short` | 224 | +7.5% | −6.3% | +15.4% |
| `EMA9_EFI_Bear_Short` | 107 | +6.6% | −0.9% | +35.7% |
| `EMA8_Cross_EMA50_Short` | 214 | +6.6% | −4.5% | +17.9% |
| `BB_Squeeze_Breakout_Short` | 163 | +3.8% | +0.7% | +63.9% |
| `Fisher_Zero_CMF_ADX_Short` | 878 | −2.8% | +7.6% | +176.4% |
| `Lower_High_Confirm_Short` | 1128 | −9.3% | +12.5% | +168.5% |
| `DEMA_EFI_Bear_Short` | 257 | −10.9% | +0.7% | +84.6% |
| `DEMA_CMF_Bear_Short` | 111 | −19.9% | +1.1% | +11.8% |
| `CMF_Extreme_Bear_Short` | 329 | −22.1% | +7.0% | +93.0% |
| `CMF_Deep_Bear_Short` | 187 | −22.1% | +0.6% | +53.8% |
| `WMA_Fisher_Bear_Short` | 208 | −23.5% | −13.4% | +8.7% |
| `MTF_CMF_Both_Neg_Short` | 133 | −27.5% | +9.9% | +6.5% |
| `Donchian_CMF_Bear_Short` | 508 | −34.8% | −10.7% | +230.1% |
| `HMA_Slope_Turn_Short` | 432 | −35.0% | +5.2% | +102.6% |
| `Keltner_CMF_Bear_Short` | 470 | −37.4% | +10.9% | +113.6% |
| `Lower_High_CMF_Bear_Short` | 760 | −40.3% | +4.8% | +184.7% |
| `Bearish_Engulf_EFI_ADX_Short` | 234 | −44.8% | +4.2% | +50.1% |
| `1h_Fisher_Zero_Cross_Short` | 830 | −45.8% | −18.2% | +37.6% |
| `EMA21_EMA50_Death_Cross_Short` | 196 | −46.8% | +2.7% | −54.5% |
| `Bearish_Engulfing_Short` | 242 | −47.2% | +2.1% | +53.2% |
| `CMF_Cross_Bullish_Long` | 340 | −48.0% | −3.4% | +76.4% |
| `OBV_Slope_EFI_ADX_Short` | 759 | −48.5% | +2.5% | +205.5% |
| `CMF_Cross_Bearish_Short` | 329 | −55.1% | −1.8% | +11.7% |
| `Two_Bar_Bear_Reversal_ADX_Short` | 768 | −57.1% | +10.1% | +215.0% |
| `Two_Bar_Bear_Reversal_Short` | 781 | −57.9% | +2.3% | +231.7% |
| `Close_Low_Range_Bear_Short` | 703 | −82.6% | +14.1% | +274.8% |
| `HMA_CMF_Bear_Short` | 1144 | −93.3% | −14.8% | +182.9% |
| `Close_Low_Range_ADX_Short` | 891 | −104.5% | +5.4% | +268.0% |
| `Fisher_Deep_Bear_Short` | 806 | −172.6% | −34.4% | +105.4% |

## Not ranked: the registry's demo strategies

Three strategies are labelled DEMO ONLY in the registry and were left out of both rankings. `Big_Up_Break_Wide_RR_Long_CURVEFIT_DEMO` (DEMO ONLY: 4h Donchian(20) breakout, wide ~1:4 R:R, fit to 2026-07-10's rally. Not a real candidate.) would have placed in the full-period list: 414 trades, +196.3%, t 1.88, max drawdown 54.1%, +118.9% in 2018-22 and +77.4% in 2023-26. It is a long-only 4h Donchian(20) breakout, so most of that is BTC's own rise from about $13,716 to $77,369.

## Data

- Binance Vision spot BTCUSDT 1m klines, every archive SHA-256 checked: 4,832,478 bars, 2017-08-17 04:00 to 2026-10-30 23:59 UTC. 2017-08 to 2017-12 is indicator warm-up; decisions run 2018-01-01 to 2026-10-30.
- 34 holes, 8,562 missing minutes (0.18%), all Binance outages; the largest is 2,010 minutes from 2018-02-08 00:28. A position open across a hole exits at the first bar after it if that bar opens beyond a level.
- 21,602 bars in Dec 2017 and Feb 2018 were stamped 14-20 s past the minute and were put back on the grid; 24,002 minutes have zero volume, almost all in 2017.
- Cross-check against the engine's existing cache: 48,526 of 48,532 complete hours have identical OHLC.
- Funding: 7,395 Binance USDT-M 8h prints, 2020-01-01 to 2026-09-30; the engine's synthetic 0.01% per 8h before 2020 and for October 2026.

## Method

- Decision at every 15m close, the live desk's cycle. The context holds only bars closed by then on every timeframe (1m 100, 5m 100, 15m 60, 1h 72, 4h 100, the backtest caps); `Price` is the last 1m close and `Now` the decision time. CVD is cumulative Binance taker-buy minus taker-sell volume, compared cycle to cycle as the live desk does.
- Entry at the open of the next 1m bar. Exits walk the 1m bars: stop first when one minute reaches both levels, the bar's open when it opens beyond a level, a 7-day time stop, as in DefaultRunConfig.
- Kept from the engine: one position per strategy, at most 5 entries per UTC day, the strategy's first declared regime pinned, SL/TP distances as absolute values, 0.35%/0.70% when a signal carries none.
- Costs per leg: Delta 7.9 bp (primary), Binance 6.5 bp. Funding signed: a long pays positive funding.
- Checks: every one of the 3,169 trades of the 17 listed strategies re-walked on the raw bars in numpy (0 mismatches); a re-run on data cut at 2022-07-01 reproduces all 1,694 trades that closed before the cut (identical).

## Caveats

- Spot prices stand in for the perpetual; the basis and Delta's own funding are not modelled.
- Context the history cannot supply (order book, open interest, liquidations, DVOL, ETH) is empty, as in the engine. Strategies that need it only fire on their OHLCV branch.
- The regime is pinned; the live desk passes its classifier's label instead.
- 1x notional, no compounding. Leverage multiplies returns and drawdowns alike.
- The full-period list is chosen with hindsight from 331 candidates; read its numbers as the best case.

## Reproduce

```
# from the repository root
python -I engine/research/btc_deep_2026_10/deep_fetch.py              # 9 years of verified 1m data
cd engine && go build -o deep_backtest ./cmd/deep_backtest
./deep_backtest -mode fixed && ./deep_backtest -mode engine           # trades -> data/historical/deep/runs
./deep_backtest -mode probe > research/btc_deep_2026_10/results/probe.txt
cd research/btc_deep_2026_10 && python -I deep_analyze.py && python -I deep_verify.py ../../deep_backtest && python -I deep_report.py
```

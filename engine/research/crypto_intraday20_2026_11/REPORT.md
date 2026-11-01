# Twenty intraday crypto strategies, October 2026

Generated 2026-11-01 by `ci_report.py`. Rules fingerprint ec341ddf8b5fa45d registered before the first backtest; one amendment (a9d3ffeb4e6e9aaf), explained under Checks.

## Verdict

- 1 of 20 strategies made money in October 2026, and that one (BTC-to-alt catch-up) on 1 trade. All 20 together: 8,942 trades, −$168,948 net, −$3,029 before costs.
- Over the 11 months before (Nov 2025 to Sep 2026), 0 of 20 made money.
- Only 2 show a before-cost edge that clears the multiple-testing bar (t 2.81): US-open range breakout +13.2 bp a trade (t 3.27); RSI(2) pullback in trend +2.8 bp a trade (t 3.37). A Delta taker round trip costs 15.8 to 19.8 bp, so neither covers it.
- Trading each month's top 5 the next month lost money in 9 of 11 months. The five best over Nov–Sep (S19, S18, S14, S11, S05) made −$3,023 in October.

## October 2026, all 20 strategies

$10,000 on each of the six coins per strategy ($50,000 for S18, which skips BTC); every trade $10,000 notional, 1x. 28 October excluded (Binance never published that day's perpetual data).

| Strategy | Trades | Win rate | Before costs / trade | Net / trade | Net | Return | Max DD | t |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| S18 BTC-to-alt catch-up | 1 | 100.0% | +55.7 bp | +35.9 bp | +$36 | +0.07% | $0 | – |
| S19 Open-interest-confirmed breakout | 24 | 41.7% | +13.1 bp | −5.8 bp | −$139 | −0.23% | $530 | −0.21 |
| S05 US-open range breakout | 126 | 41.3% | +12.4 bp | −6.1 bp | −$770 | −1.28% | $2,558 | −0.58 |
| S11 Absorption fade | 44 | 29.5% | −2.6 bp | −21.6 bp | −$950 | −1.58% | $962 | −3.91 |
| S14 Pre-funding premium fade | 39 | 17.9% | −11.7 bp | −30.8 bp | −$1,200 | −2.00% | $1,200 | −4.37 |
| S08 Abnormal-hour continuation | 58 | 46.6% | −6.0 bp | −24.3 bp | −$1,408 | −2.35% | $2,611 | −1.53 |
| S04 First 5-minute direction | 65 | 10.8% | −24.0 bp | −42.4 bp | −$2,755 | −4.59% | $2,755 | −7.56 |
| S15 Prior-day high/low sweep reversal | 155 | 34.8% | −1.1 bp | −19.7 bp | −$3,052 | −5.09% | $3,064 | −3.94 |
| S03 Dual Thrust | 72 | 36.1% | −24.9 bp | −43.2 bp | −$3,111 | −5.18% | $5,170 | −1.61 |
| S01 First-half-hour momentum | 177 | 13.6% | −2.6 bp | −21.1 bp | −$3,731 | −6.22% | $3,731 | −10.05 |
| S06 Asia range, London breakout | 75 | 25.3% | −32.7 bp | −51.3 bp | −$3,850 | −6.42% | $3,850 | −4.87 |
| S02 Williams volatility breakout | 120 | 31.7% | −28.4 bp | −46.9 bp | −$5,630 | −9.38% | $9,088 | −2.38 |
| S09 Jump reversal | 338 | 50.6% | −2.6 bp | −21.0 bp | −$7,087 | −11.81% | $7,162 | −7.05 |
| S16 Calm-market Bollinger fade | 464 | 42.2% | +2.8 bp | −15.6 bp | −$7,240 | −12.07% | $7,366 | −6.22 |
| S20 Crowded-account contrarian | 240 | 32.1% | −14.1 bp | −32.5 bp | −$7,796 | −12.99% | $7,796 | −5.12 |
| S10 Taker-flow momentum | 727 | 32.6% | +5.7 bp | −12.5 bp | −$9,060 | −15.10% | $9,608 | −5.51 |
| S17 RSI(2) pullback in trend | 433 | 30.3% | −5.0 bp | −23.7 bp | −$10,245 | −17.08% | $10,245 | −10.27 |
| S13 Round-number rejection | 1378 | 51.7% | +2.7 bp | −16.1 bp | −$22,232 | −37.05% | $22,243 | −13.05 |
| S07 Session VWAP trend | 1902 | 13.9% | −0.4 bp | −18.8 bp | −$35,666 | −59.44% | $35,712 | −11.96 |
| S12 Round-number breakout | 2504 | 36.4% | +1.5 bp | −17.2 bp | −$43,061 | −71.77% | $43,061 | −26.97 |
| **All 20** | 8,942 | | | | **−$168,948** | | | |

### October by coin (net $ on that coin's $10,000)

| Strategy | BTC | ETH | SOL | XRP | DOGE | BNB |
|---|---:|---:|---:|---:|---:|---:|
| S18 BTC-to-alt catch-up | – | $0 | $0 | $0 | $0 | +$36 |
| S19 Open-interest-confirmed breakout | −$230 | +$106 | +$105 | +$252 | −$77 | −$295 |
| S05 US-open range breakout | −$351 | +$221 | −$208 | +$196 | −$544 | −$85 |
| S11 Absorption fade | −$165 | −$70 | −$128 | +$72 | −$305 | −$352 |
| S14 Pre-funding premium fade | −$44 | −$205 | −$207 | −$300 | −$89 | −$355 |
| S08 Abnormal-hour continuation | −$73 | −$137 | −$862 | −$83 | −$118 | −$135 |
| S04 First 5-minute direction | −$253 | −$415 | −$495 | −$732 | −$248 | −$612 |
| S15 Prior-day high/low sweep reversal | −$357 | −$217 | −$932 | −$730 | −$533 | −$283 |
| S03 Dual Thrust | −$418 | −$1,467 | +$85 | −$577 | −$802 | +$68 |
| S01 First-half-hour momentum | −$521 | −$563 | −$503 | −$467 | −$906 | −$772 |
| S06 Asia range, London breakout | −$369 | −$319 | −$830 | −$616 | −$1,310 | −$406 |
| S02 Williams volatility breakout | −$208 | −$1,431 | −$1,952 | −$996 | −$490 | −$553 |
| S09 Jump reversal | −$1,114 | −$1,038 | −$1,193 | −$975 | −$1,655 | −$1,113 |
| S16 Calm-market Bollinger fade | −$1,311 | −$960 | −$1,723 | −$1,083 | −$643 | −$1,520 |
| S20 Crowded-account contrarian | −$1,238 | −$761 | −$2,227 | −$1,369 | −$1,530 | −$671 |
| S10 Taker-flow momentum | −$1,482 | −$1,353 | −$1,853 | −$2,271 | −$939 | −$1,163 |
| S17 RSI(2) pullback in trend | −$1,196 | −$1,439 | −$2,634 | −$1,498 | −$1,603 | −$1,876 |
| S13 Round-number rejection | −$2,712 | −$2,044 | −$2,012 | −$5,610 | −$6,766 | −$3,086 |
| S07 Session VWAP trend | −$6,004 | −$6,154 | −$6,987 | −$6,903 | −$5,422 | −$4,197 |
| S12 Round-number breakout | −$5,081 | −$3,970 | −$6,744 | −$10,191 | −$12,617 | −$4,459 |

## Edge before costs

Average move captured per trade before any cost. A Delta taker round trip costs 15.8 bp (BTC, ETH) to 19.8 bp (alts).

| Strategy | Nov–Sep trades | Nov–Sep before costs | t | October before costs | Break-even cost a side |
|---|---:|---:|---:|---:|---:|
| S19 Open-interest-confirmed breakout | 465 | +13.79 bp | 1.33 | +13.11 bp | +6.89 bp |
| S05 US-open range breakout | 1390 | +13.19 bp | 3.27 | +12.40 bp | +6.59 bp |
| S06 Asia range, London breakout | 972 | +10.62 bp | 2.11 | −32.72 bp | +5.31 bp |
| S03 Dual Thrust | 764 | +7.31 bp | 0.81 | −24.92 bp | +3.65 bp |
| S02 Williams volatility breakout | 1407 | +6.51 bp | 0.91 | −28.45 bp | +3.25 bp |
| S11 Absorption fade | 416 | +5.16 bp | 1.77 | −2.60 bp | +2.58 bp |
| S14 Pre-funding premium fade | 351 | +3.96 bp | 1.39 | −11.72 bp | +1.98 bp |
| S17 RSI(2) pullback in trend | 4528 | +2.84 bp | 3.37 | −5.04 bp | +1.42 bp |
| S20 Crowded-account contrarian | 2568 | +1.49 bp | 0.62 | −14.07 bp | +0.74 bp |
| S18 BTC-to-alt catch-up | 253 | +0.89 bp | 0.17 | +55.74 bp | +0.45 bp |
| S10 Taker-flow momentum | 7926 | +0.49 bp | 0.62 | +5.70 bp | +0.24 bp |
| S08 Abnormal-hour continuation | 865 | +0.25 bp | 0.05 | −5.99 bp | +0.12 bp |
| S15 Prior-day high/low sweep reversal | 1582 | −0.13 bp | −0.06 | −1.10 bp | −0.07 bp |
| S12 Round-number breakout | 37909 | −0.45 bp | −2.84 | +1.51 bp | −0.23 bp |
| S16 Calm-market Bollinger fade | 5006 | −0.70 bp | −0.78 | +2.83 bp | −0.35 bp |
| S13 Round-number rejection | 21865 | −0.82 bp | −2.40 | +2.70 bp | −0.41 bp |
| S07 Session VWAP trend | 22024 | −1.56 bp | −2.43 | −0.37 bp | −0.78 bp |
| S04 First 5-minute direction | 656 | −1.79 bp | −0.30 | −23.99 bp | −0.90 bp |
| S09 Jump reversal | 4058 | −1.86 bp | −1.99 | −2.60 bp | −0.93 bp |
| S01 First-half-hour momentum | 1993 | −2.17 bp | −2.94 | −2.63 bp | −1.08 bp |

## Twelve months, same rules (net $)

| Strategy | Nov 25 | Dec 25 | Jan 26 | Feb 26 | Mar 26 | Apr 26 | May 26 | Jun 26 | Jul 26 | Aug 26 | Sep 26 | Oct 26 | Total |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| S19 Open-interest-confirmed breakout | −$692 | −$29 | +$2,891 | +$734 | −$1,191 | −$2,320 | −$2,002 | +$159 | +$326 | −$2,080 | +$1,877 | −$139 | −$2,464 |
| S18 BTC-to-alt catch-up | −$90 | +$509 | −$122 | −$1,971 | −$381 | −$597 | −$116 | −$61 | −$212 | −$1,629 | −$27 | +$36 | −$4,661 |
| S14 Pre-funding premium fade | −$693 | −$721 | −$685 | +$11 | −$562 | −$42 | −$357 | +$29 | −$155 | −$1,600 | −$535 | −$1,200 | −$6,510 |
| S11 Absorption fade | −$768 | −$934 | −$902 | −$160 | −$351 | −$514 | −$1,166 | −$181 | −$611 | −$236 | +$51 | −$950 | −$6,722 |
| S05 US-open range breakout | +$7,661 | −$2,362 | +$485 | +$4,158 | −$5,913 | −$2,568 | −$4,978 | +$1,144 | −$3,605 | −$763 | −$638 | −$770 | −$8,151 |
| S06 Asia range, London breakout | −$4,086 | −$249 | −$1,606 | +$4,712 | +$1,229 | −$3,152 | −$3,278 | −$1,742 | −$390 | +$814 | +$75 | −$3,850 | −$11,523 |
| S03 Dual Thrust | −$722 | −$5,053 | +$3,975 | +$6,235 | −$3,674 | −$1,423 | −$6,054 | −$2,407 | −$1,145 | +$2,326 | −$800 | −$3,111 | −$11,852 |
| S04 First 5-minute direction | −$3,309 | +$379 | +$454 | +$753 | −$1,354 | −$902 | −$41 | −$1,675 | −$3,428 | −$2,490 | −$1,729 | −$2,755 | −$16,097 |
| S08 Abnormal-hour continuation | +$22 | +$3,759 | −$1,292 | −$3,129 | +$1,190 | +$433 | −$2,262 | −$11,868 | −$1,219 | −$903 | −$474 | −$1,408 | −$17,152 |
| S02 Williams volatility breakout | −$10,035 | −$6,353 | +$9,802 | −$796 | +$804 | −$5,850 | −$5,827 | +$1,808 | −$3,878 | −$1,630 | +$4,721 | −$5,630 | −$22,865 |
| S15 Prior-day high/low sweep reversal | −$4,460 | +$409 | −$3,011 | −$3,152 | −$3,642 | −$2,701 | −$3,542 | −$2,327 | −$2,494 | −$1,577 | −$2,919 | −$3,052 | −$32,468 |
| S01 First-half-hour momentum | −$4,460 | −$2,664 | −$4,339 | −$4,270 | −$5,547 | −$3,513 | −$3,501 | −$3,691 | −$2,772 | −$3,619 | −$2,747 | −$3,731 | −$44,853 |
| S20 Crowded-account contrarian | −$3,665 | −$5,109 | −$2,933 | +$275 | −$6,340 | −$5,114 | −$7,027 | −$5,128 | −$5,539 | +$1,654 | −$4,481 | −$7,796 | −$51,203 |
| S17 RSI(2) pullback in trend | −$4,062 | −$8,732 | −$9,303 | −$2,523 | −$3,996 | −$7,244 | −$9,837 | −$4,329 | −$7,883 | −$7,490 | −$5,465 | −$10,245 | −$81,109 |
| S09 Jump reversal | −$5,170 | −$9,025 | −$9,246 | −$8,713 | −$3,651 | −$7,089 | −$8,745 | −$9,135 | −$5,582 | −$9,244 | −$6,271 | −$7,087 | −$88,958 |
| S16 Calm-market Bollinger fade | −$8,390 | −$12,190 | −$7,438 | −$6,883 | −$13,202 | −$8,742 | −$10,498 | −$5,557 | −$7,057 | −$8,149 | −$8,181 | −$7,240 | −$103,527 |
| S10 Taker-flow momentum | −$6,497 | −$6,321 | −$9,704 | −$12,999 | −$16,428 | −$12,279 | −$13,531 | −$14,519 | −$17,130 | −$16,698 | −$15,369 | −$9,060 | −$150,533 |
| S13 Round-number rejection | −$60,086 | −$37,025 | −$39,141 | −$59,203 | −$46,063 | −$28,740 | −$23,297 | −$42,964 | −$24,986 | −$27,159 | −$37,861 | −$22,232 | −$448,756 |
| S07 Session VWAP trend | −$43,266 | −$31,536 | −$43,075 | −$35,739 | −$45,918 | −$39,057 | −$36,097 | −$53,463 | −$39,491 | −$30,233 | −$43,320 | −$35,666 | −$476,861 |
| S12 Round-number breakout | −$94,838 | −$68,684 | −$68,586 | −$87,715 | −$78,581 | −$51,264 | −$40,117 | −$72,672 | −$47,841 | −$51,782 | −$61,618 | −$43,061 | −$766,759 |

## Does last month's top 5 win next month?

| Picked in | Top 5 | Their next month | Any 5, next month, average |
|---|---|---:|---:|
| 2025-11 | S05 S08 S18 S19 S14 | +$1,156 | −$47,983 |
| 2025-12 | S08 S18 S15 S04 S19 | −$1,079 | −$45,944 |
| 2026-01 | S02 S03 S19 S05 S04 | +$11,084 | −$52,594 |
| 2026-02 | S03 S06 S05 S04 S19 | −$10,903 | −$58,393 |
| 2026-03 | S06 S08 S02 S11 S18 | −$9,682 | −$45,670 |
| 2026-04 | S08 S14 S11 S18 S04 | −$3,942 | −$45,568 |
| 2026-05 | S04 S18 S14 S11 S19 | −$1,729 | −$57,145 |
| 2026-06 | S02 S05 S19 S14 S18 | −$7,523 | −$43,773 |
| 2026-07 | S19 S14 S18 S06 S11 | −$4,731 | −$40,622 |
| 2026-08 | S03 S20 S06 S11 S05 | −$5,792 | −$46,427 |
| 2026-09 | S02 S19 S06 S11 S18 | −$10,533 | −$42,237 |

Month-to-month rank correlation averages 0.74: the rules that trade least lose least every month, so the ranking is stable without being useful.

## The 20 strategies

### S01 First-half-hour momentum (Time-series momentum)

- **Idea:** The return of the first half-hour of the UTC day predicts the return of the last half-hour.
- **Rules:** Take the 00:00-00:30 UTC return. At 23:30 UTC trade in its direction; exit at 00:00. No stop. One trade a day.
- **Source:** Shen, Urquhart & Wang (2022), 'Bitcoin intraday time series momentum', Financial Review; Gao, Han, Li & Zhou (2018), 'Market intraday momentum', Journal of Financial Economics
- **Result:** October −$3,731 on 177 trades; Nov–Sep −$41,122 on 1993 trades; before costs −2.17 bp a trade (t −2.94).

### S02 Williams volatility breakout (Volatility breakout)

- **Idea:** A move of half of yesterday's range away from today's open signals a directional day.
- **Rules:** Long when a 1m close is above the 00:00 open + 0.5 x yesterday's range; short below the open - 0.5 x range. First trigger only; exit at 00:00. No stop.
- **Source:** Williams (1999), 'Long-Term Secrets to Short-Term Trading'; widely run on Bitcoin as the k = 0.5 volatility breakout
- **Result:** October −$5,630 on 120 trades; Nov–Sep −$17,235 on 1407 trades; before costs +6.51 bp a trade (t 0.91).

### S03 Dual Thrust (Volatility breakout)

- **Idea:** Breakouts beyond a range built from the last four days continue within the day.
- **Rules:** Range = max(HH - LC, HC - LL) over the prior 4 days. Long above open + 0.5 x range, short below open - 0.5 x range, reversing on the opposite line; exit at 00:00.
- **Source:** Michael Chalek's Dual Thrust system, a standard intraday breakout in futures and crypto trading
- **Result:** October −$3,111 on 72 trades; Nov–Sep −$8,741 on 764 trades; before costs +7.31 bp a trade (t 0.81).

### S04 First 5-minute direction (Opening range)

- **Idea:** The direction of the first 5-minute candle, on days with heavy early volume, carries through the session.
- **Rules:** If the 00:00-00:05 candle is up (down) and its volume is at least its 14-day average, buy (sell) at 00:05. Stop 10% of the 14-day ATR; exit at 00:00.
- **Source:** Zarattini, Barbon & Aziz (2024), 'A Profitable Day Trading Strategy for the U.S. Equity Market' (5-minute opening-range rule)
- **Result:** October −$2,755 on 65 trades; Nov–Sep −$13,342 on 656 trades; before costs −1.79 bp a trade (t −0.30).

### S05 US-open range breakout (Opening range)

- **Idea:** Crypto volume and volatility peak when US equities open; the first 30 minutes set a range whose break carries on.
- **Rules:** Range = first 30 min after the NYSE open (13:30 or 14:30 UTC). Weekdays only. Trade the first 1m close outside it within 4 h; stop at the other side, target 2x the range, exit at the NYSE close.
- **Source:** Crabel (1990), 'Day Trading with Short Term Price Patterns and Opening Range Breakout'; Eross, McGroarty, Urquhart & Wolfe (2019), 'The intraday dynamics of bitcoin', Research in International Business and Finance (activity peaks in US hours)
- **Result:** October −$770 on 126 trades; Nov–Sep −$7,381 on 1390 trades; before costs +13.19 bp a trade (t 3.27).

### S06 Asia range, London breakout (Opening range)

- **Idea:** The quiet Asian-hours range breaks when Europe arrives, and the break continues.
- **Rules:** Range = 00:00 UTC to the London open (07:00 or 08:00 UTC). Weekdays only. Trade the first 1m close outside it within 5 h; stop at the range middle, target 1x the range, exit 9 h after the London open.
- **Source:** FX 'London breakout' practice; Eross et al. (2019) on activity rising at the European open
- **Result:** October −$3,850 on 75 trades; Nov–Sep −$7,673 on 972 trades; before costs +10.62 bp a trade (t 2.11).

### S07 Session VWAP trend (Trend)

- **Idea:** Price above the session VWAP means buyers are in control, below means sellers.
- **Rules:** Every 15 min from 00:15 to 23:30 UTC hold long if the close is above the session VWAP and short if below, flipping when it crosses; exit at 00:00.
- **Source:** Zarattini & Aziz (2023), 'Volume Weighted Average Price (VWAP): The Holy Grail for Day Trading Systems'
- **Result:** October −$35,666 on 1902 trades; Nov–Sep −$441,195 on 22024 trades; before costs −1.56 bp a trade (t −2.43).

### S08 Abnormal-hour continuation (Overreaction)

- **Idea:** After an abnormally large hourly move, price keeps moving the same way for a while.
- **Rules:** If an hour's return exceeds 3 standard deviations of the prior 30 days of hourly returns, trade in its direction for 2 h; stop at that hour's open.
- **Source:** Caporale & Plastun (2019), 'Price overreactions in the cryptocurrency market', Journal of Economic Studies
- **Result:** October −$1,408 on 58 trades; Nov–Sep −$15,744 on 865 trades; before costs +0.25 bp a trade (t 0.05).

### S09 Jump reversal (Liquidity provision)

- **Idea:** A violent 5-minute move is often forced flow that partly reverses once it is absorbed.
- **Rules:** If a 5m return exceeds 4 standard deviations of the prior 3 days, fade it. Target half the bar's move back, stop half the jump beyond its extreme, exit after 1 h.
- **Source:** Nagel (2012), 'Evaporating liquidity', Review of Financial Studies (short-term reversal as paid liquidity provision)
- **Result:** October −$7,087 on 338 trades; Nov–Sep −$81,871 on 4058 trades; before costs −1.86 bp a trade (t −1.99).

### S10 Taker-flow momentum (Order flow)

- **Idea:** Heavy, one-sided aggressive buying or selling keeps pushing price the same way.
- **Rules:** Every 15 min: if the last 30 min had taker imbalance above +0.10 (below -0.10), a return in the same direction, and volume at least 1.5x its 7-day average, follow it for 1 h. Stop 1 ATR (15m, 14).
- **Source:** Cont, Kukanov & Stoikov (2014), 'The price impact of order book events', Journal of Financial Econometrics; Silantyev (2019), 'Order flow analysis of cryptocurrency markets', Digital Finance
- **Result:** October −$9,060 on 727 trades; Nov–Sep −$141,473 on 7926 trades; before costs +0.49 bp a trade (t 0.62).

### S11 Absorption fade (Order flow)

- **Idea:** Strong aggressive buying that fails to lift price is being absorbed by a large passive seller, so price turns down (and the mirror).
- **Rules:** Every 15 min: if the last 30 min had taker imbalance above +0.15 but no gain (below -0.15 but no loss) on 1.5x average volume, trade against the flow for 1 h. Stop 1 ATR (15m, 14).
- **Source:** Kyle (1985) price-impact framework; order-flow 'absorption' as practitioners read it
- **Result:** October −$950 on 44 trades; Nov–Sep −$5,772 on 416 trades; before costs +5.16 bp a trade (t 1.77).

### S12 Round-number breakout (Round numbers)

- **Idea:** Crossing a round number triggers the stops parked just beyond it, which speeds the move up.
- **Rules:** Round step = largest 1-2-5 step at most 1% of yesterday's close. When a 15m bar closes through a round level, follow it: stop 0.25 step back, target 0.75 step beyond, exit after 2 h.
- **Source:** Osler (2003), 'Currency orders and exchange rate dynamics', Journal of Finance (stop orders cluster just past round numbers); Urquhart (2017), 'Price clustering in Bitcoin', Economics Letters
- **Result:** October −$43,061 on 2504 trades; Nov–Sep −$723,698 on 37909 trades; before costs −0.45 bp a trade (t −2.84).

### S13 Round-number rejection (Round numbers)

- **Idea:** A test of a round level that closes well away from it is a rejection worth fading.
- **Rules:** A 15m bar that reaches within 0.1 step of the round level above, then closes 0.3 step below it and down on the bar, is sold (mirror at support). Stop 0.2 step past the level, target 0.6 step, exit after 2 h.
- **Source:** Osler (2003): take-profit orders cluster at round numbers, so price tends to stall and reverse there
- **Result:** October −$22,232 on 1378 trades; Nov–Sep −$426,525 on 21865 trades; before costs −0.82 bp a trade (t −2.40).

### S14 Pre-funding premium fade (Perpetual mechanics)

- **Idea:** An unusually rich (cheap) perpetual just before a funding settlement gets sold (bought) by traders avoiding the payment and by arbitrage.
- **Rules:** 30 min before each 08:00, 16:00 and 00:00 UTC settlement: if the 30-min mean premium is above the 90th (below the 10th) percentile of the prior 7 days, short (buy) the perpetual. Exit 15 min after the settlement, or at 23:59 for the 00:00 one.
- **Source:** He, Manela, Ross & von Wachter (2022), 'Fundamentals of Perpetual Futures' (funding ties the perpetual to spot)
- **Result:** October −$1,200 on 39 trades; Nov–Sep −$5,310 on 351 trades; before costs +3.96 bp a trade (t 1.39).

### S15 Prior-day high/low sweep reversal (Liquidity sweep)

- **Idea:** A brief push through yesterday's high or low that closes back inside has run the stops and tends to reverse.
- **Rules:** A 15m bar that trades above yesterday's high but closes below it is sold (mirror at the low), once per side a day. Stop 0.1% beyond the bar's extreme, target 2x the risk, exit after 4 h.
- **Source:** Osler (2003) on stop orders resting beyond prior highs and lows; the 'stop run' reversal
- **Result:** October −$3,052 on 155 trades; Nov–Sep −$29,416 on 1582 trades; before costs −0.13 bp a trade (t −0.06).

### S16 Calm-market Bollinger fade (Mean reversion)

- **Idea:** When the market is not trending, closes far outside the band snap back to the middle.
- **Rules:** 5m Bollinger(20, 2.5). When the hourly ADX(14) is below 20, buy a close under the lower band (sell above the upper). Target the middle band, stop 1.5 band-widths away, exit after 2 h.
- **Source:** Bollinger (2001), 'Bollinger on Bollinger Bands'; Wilder (1978) ADX as the regime filter
- **Result:** October −$7,240 on 464 trades; Nov–Sep −$96,287 on 5006 trades; before costs −0.70 bp a trade (t −0.78).

### S17 RSI(2) pullback in trend (Mean reversion)

- **Idea:** A sharp short-term pullback inside a trend is bought (sold) and quickly recovers.
- **Rules:** 15m bars. Above the 200-bar average, buy RSI(2) under 5 and exit when RSI(2) goes above 70; below it, sell RSI(2) over 95 and exit under 30. Stop 2 ATR(14), exit after 2 h.
- **Source:** Connors & Alvarez (2009), 'Short Term Trading Strategies That Work'
- **Result:** October −$10,245 on 433 trades; Nov–Sep −$70,863 on 4528 trades; before costs +2.84 bp a trade (t 3.37).

### S18 BTC-to-alt catch-up (Cross-asset)

- **Idea:** When BTC jumps and an alt has not followed yet, the alt catches up within minutes.
- **Rules:** Every 5 min: if BTC moved more than 0.5% and the alt moved less than half of that, trade the alt in BTC's direction for 15 min. Stop 1%. Alts only.
- **Source:** Bitcoin's lead in crypto price discovery; tested here as a hypothesis on 5-minute bars
- **Result:** October +$36 on 1 trades; Nov–Sep −$4,697 on 253 trades; before costs +0.89 bp a trade (t 0.17).

### S19 Open-interest-confirmed breakout (Positioning)

- **Idea:** A breakout that brings in new positions (rising open interest) is real money behind the move.
- **Rules:** Hourly close above the prior 24-hour high (below the low) while open interest rose at least 1% in that hour: trade the break. Stop 1 ATR(1h, 14); exit at 00:00.
- **Source:** Bessembinder & Seguin (1993), 'Price volatility, trading volume, and market depth', Journal of Financial and Quantitative Analysis (open interest as new positioning)
- **Result:** October −$139 on 24 trades; Nov–Sep −$2,325 on 465 trades; before costs +13.79 bp a trade (t 1.33).

### S20 Crowded-account contrarian (Positioning)

- **Idea:** When an extreme share of accounts is long (short) and price is already weak (strong), the crowd gets squeezed.
- **Rules:** Hourly: if the account long/short ratio is at its 7-day 95th percentile and price is below the session VWAP, short (mirror at the 5th percentile). Stop 1.5 ATR(1h, 14), exit after 4 h.
- **Source:** Contrarian reading of retail positioning (Kumar & Lee (2006), 'Retail investor sentiment and return comovements', Journal of Finance); Binance's all-account long/short ratio
- **Result:** October −$7,796 on 240 trades; Nov–Sep −$43,407 on 2568 trades; before costs +1.49 bp a trade (t 0.62).

## Data, method and checks

- Binance Vision USDT-M perpetuals (SHA-256 checked): 1m candles with taker-buy volume, 1m premium index, 5m open interest and long/short ratios, funding prints; BTC ETH SOL XRP DOGE BNB, Oct 2025 (warm-up) to Oct 2026.
- 28 Oct 2026 perpetual candles were never published: rebuilt from Binance spot (volume scaled to perpetual size) for the indicators only; no trades that day. The premium index is missing on 28-29 Oct; those minutes stay empty.
- Signals read closed bars only; entries fill at the next 1m open; positions are walked minute by minute (gap fills at the open, stop first when a minute reaches both levels); flat by 00:00 UTC; one position per strategy per coin.
- Costs a side: Delta India taker 0.05% + 18% GST = 5.9 bp, plus 2 bp (BTC, ETH) or 4 bp (alts) spread and slippage; Binance funding prints, signed.
- Fills re-checked by a separately written simulator: 125,940 trades, 0 mismatches. No look-ahead: a re-run on data cut at 2026-10-16 reproduces all 121,560 earlier trades, 0 mismatches.
- Amendment after the first run (ec341ddf8b5fa45d -> a9d3ffeb4e6e9aaf): Found by the truncation test after the first full run: S05, S06 and S14 looped over _days(mk)[:-1], which dropped the last day of data, so they never traded 2026-10-31. Now every day is used. No rule or parameter changed. First-run October results for the 17 untouched strategies are unchanged by this edit.

## Limits

- Binance prices and order flow stand in for Delta, where the trades would go; Delta's alt spreads are usually wider than 4 bp.
- One month is a small sample; the 11 earlier months show which results repeat.
- Taker fills on every side; resting limit orders would pay Delta's 0.02% maker fee, but breakouts and stops rarely fill that way.

## Reproduce

```
cd engine/research/crypto_intraday20_2026_11
python -I ci_fetch.py && python -I ci_run.py && python -I ci_verify.py && python -I ci_report.py
```

# Profitable intraday crypto strategies, October 2026

Generated 2026-11-01 16:21 UTC. October 2026 only (no trading on the 28th). Delta India fees, slippage and funding; $10,000 per trade, no leverage, flat by 00:00 UTC.

- Tested 102,583 strategies over 211 rounds on BTC ETH SOL XRP DOGE BNB: 33,430 profitable, 13,111 consistent (both halves of October, 3+ of 6 coins, 10+ trades).
- **11,635 also made money on 10 other coins** (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT; 5+ of 10).

## Best per family

| Family | Strategy | October return, 16 coins |
|---|---|---:|
| Bollinger fade | Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|2.0|12|high`) | +14.83% |
| Hour-of-day | Sell 09:00 UTC for 12 h (weekdays) (`HZX|9|12|short|none|wkday`) | +13.67% |
| Stochastic extreme | Stochastic(27) 240m <29/>71, all day (`STX|240|27|29|3.0|16|all`) | +13.27% |
| RSI fade | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.5|6|high`) | +12.76% |
| EMA z-score fade | EMA13 z-score 2.1 fade 240m, stop 4.0 ATR, 12 h, high-volume bars only, all day (`ZEX|240|13|2.1|4.0|12|high|all`) | +12.50% |
| 4h band + fast RSI | 4h Bollinger(1.7) extreme + 60m RSI(21) 39, 06:00 UTC +24 h (`MXX|1.7|60|21|39|6|24|3.0|12`) | +12.50% |
| Keltner channel | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.75|3.0|8|none`) | +12.10% |
| RSI fade by hour and VWAP | RSI(28) 1h fade 35/65 from 18:00 UTC for 14 h, VWAP against, exit 60 (`CZ|18|14|against|28|35|60`) | +10.95% |
| 4h Bollinger + 1h RSI | 4h Bollinger(2.0) extreme + 1h RSI(21) 35 (`MTF|2.0|35|21|1.0|6`) | +9.42% |
| Day-of-week window | Friday 00:00 UTC for 6 h, long (`DOW|4|0|6|long`) | +7.68% |
| Fisher transform | Fisher(12) 180m fade beyond 1.4, all day (`FSX|180|12|1.4|2.5|16|all`) | +6.93% |
| Williams %R | Williams %R(17) 240m beyond -94/-6, all day (`WRX|240|17|6|2.0|8|all`) | +6.80% |
| Money Flow Index | MFI(24) 60m <26/>74, all day (`MFX|60|24|26|3.5|6|all`) | +6.32% |
| Fade from yesterday's close | Fade 0.5 ATR from yesterday's close (60m check), target the anchor, all day (`PCF|0.5|60|anchor|0.5|0|all`) | +6.29% |
| Asian-range limit fade | Asian range 00:00-06:00 UTC limit fade until 20:00, edge +0.0, stop 0.5, target far edge (`RNG|6|20|0.0|0.5|far`) | +6.23% |
| Fade from the day's open | Fade 0.5 ATR from the 00:00 UTC open (60m check), target the anchor, all day (`DOF|0.5|60|anchor|0.5|0|all`) | +5.99% |
| Pivot fade | Pivot fade at R1/S1, all day (`N7f|1|none|all`) | +5.53% |
| Pivot levels | Classic pivot fade at level 1, all day (`PIV|classic|1|fade|all`) | +5.53% |
| Session-open breakout | Hong Kong (01:30 UTC) 10-min breakout, retest entry, 4.0R target (`ORX|hk|10|retest|far|4.0|none`) | +4.96% |
| Sharp-move momentum/reversal | Fade a 3.0 ATR move over 6 x 60m (`ROC|60|6|3.0|4|rev|none`) | +4.53% |
| CCI extreme | CCI(24) 180m fade at 120, US hours (`CCX|180|24|120|fade|16|us`) | +4.16% |
| Connors RSI | Connors RSI 240m <15/>85, exit 50, all day (`CRS|240|15|50|2.0|4|all`) | +3.68% |
| Bollinger breakout | Bollinger(20,2.0) 30m breakout, US hours, with trend (`BBB|30|20|2.0|atr|1.5|trend|us`) | +2.18% |
| Band limit fade (maker) | Bollinger(31,1.8) 30m limit fade, 1 bar(s), stop 3.5 ATR, 3 h, European hours (`BLX|30|bb|31|1.8|1|3.5|3|eu`) | +2.12% |
| Bollinger squeeze breakout | Bollinger(11,1.75) squeeze breakout 120m (width in lowest 10% of 100 bars), US hours (`SQZ|120|11|1.75|10|100|4|1.5|0.0|6|us`) | +2.05% |
| DMI cross | DMI(27) cross 180m, ADX>10, US hours, with trend (`DMX|180|27|10|us|trend`) | +1.98% |
| Cross-coin intraday | Cross-coin reversal at 06:00 UTC, 1 each side, to 00:00 (`XS|6|4h|rev|1|0`) | +1.77% |
| Donchian breakout | Donchian(6) 90m breakout, US hours, with trend (`DNX|90|6|atr|3.0|trend|us`) | +1.77% |
| US pre-market breakout | US pre-market breakout, retest entry, stop far, target 3R (`N10|retest|far|3R|none`) | +1.70% |
| Previous-day high/low failure | Previous-day high/low failure 60m, stop 1.0 ATR beyond, target 1.5R, 8 h, all day (`PDF|60|1.0|1.5|8|all`) | +1.13% |
| EMA pullback | EMA75 pullback 180m, 1.0R target, European hours (`EPX|180|75|1.0|1.0|eu`) | +0.87% |
| Candle-streak fade | Fade 6 same-colour 30m candles, Asian hours (`STK|30|6|4|2.0|asia`) | +0.85% |
| Trend-day follow | Trend-day follow at 1 h after the NYSE open (`B|US1h|0.5|trend|open`) | +0.77% |
| Volume spike | Fade a 60m volume spike (3.25x volume, 3.0 ATR range), stop 3.0 ATR, 8 h, US hours (`VSP|60|3.25|3.0|fade|3.0|8|us`) | +0.58% |
| Turtle soup (failed breakout) | Turtle soup 42-bar 180m, stop 0.0 ATR beyond, target channel middle, 6 h, Asian hours (`TSP|180|42|0.0|mid|6|none|asia`) | +0.34% |

## Top 100

| # | Strategy | Trades | October, 16 coins | 6 coins | 10 new coins | Coins up | Worst DD |
|---|---|---:|---:|---:|---:|---:|---:|
| 1 | Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|2.0|12|high`) | 276 | **+14.83%** ($23,734) | +8.08% | +18.89% | 16/16 | 4.65% |
| 2 | Bollinger(24,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|4.0|12|high`) | 275 | **+14.38%** ($23,007) | +7.87% | +18.29% | 16/16 | 5.08% |
| 3 | Bollinger(24,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|3.0|12|high`) | 275 | **+14.14%** ($22,623) | +7.27% | +18.26% | 16/16 | 5.67% |
| 4 | Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|4.0|12|high`) | 229 | **+13.67%** ($21,876) | +9.21% | +16.35% | 16/16 | 2.71% |
| 5 | Sell 09:00 UTC for 12 h (weekdays) (`HZX|9|12|short|none|wkday`) | 336 | **+13.67%** ($21,864) | +12.29% | +14.49% | 15/16 | 6.24% |
| 6 | Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|3.0|12|high`) | 229 | **+13.63%** ($21,803) | +9.46% | +16.13% | 16/16 | 2.58% |
| 7 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|2.0|12|high`) | 233 | **+13.61%** ($21,775) | +9.13% | +16.30% | 16/16 | 2.94% |
| 8 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|1.8|2.0|12|none`) | 274 | **+13.30%** ($21,273) | +6.71% | +17.25% | 16/16 | 3.26% |
| 9 | Stochastic(27) 240m <29/>71, all day (`STX|240|27|29|3.0|16|all`) | 589 | **+13.27%** ($21,229) | +8.89% | +15.89% | 14/16 | 6.51% |
| 10 | Sell 07:00 UTC for 12 h, only against the last hour's RSI (weekdays) (`HZX|7|12|short|rsi1h|wkday`) | 164 | **+13.24%** ($21,181) | +7.68% | +16.57% | 15/16 | 2.98% |
| 11 | Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|1.8|3.0|12|none`) | 266 | **+13.12%** ($20,993) | +6.64% | +17.01% | 16/16 | 3.72% |
| 12 | Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|1.8|4.0|12|none`) | 266 | **+13.10%** ($20,964) | +6.21% | +17.24% | 16/16 | 4.15% |
| 13 | Bollinger(24,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|2.0|12|high`) | 240 | **+13.01%** ($20,819) | +7.71% | +16.19% | 16/16 | 4.49% |
| 14 | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.5|6|high`) | 237 | **+12.76%** ($20,409) | +9.25% | +14.86% | 15/16 | 3.49% |
| 15 | RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.5|6|high`) | 156 | **+12.74%** ($20,378) | +9.68% | +14.57% | 15/16 | 3.15% |
| 16 | RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|2.5|6|high`) | 156 | **+12.66%** ($20,256) | +9.67% | +14.45% | 15/16 | 3.15% |
| 17 | Bollinger(24,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|4.0|12|high`) | 238 | **+12.65%** ($20,242) | +7.72% | +15.61% | 15/16 | 4.08% |
| 18 | Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 12 h (`BZ|180|24|1.8|4.0|12|none`) | 315 | **+12.62%** ($20,198) | +7.21% | +15.87% | 15/16 | 3.46% |
| 19 | RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|2.5|6|high`) | 237 | **+12.52%** ($20,033) | +8.94% | +14.67% | 15/16 | 3.49% |
| 20 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|2.0|3.0|12|none`) | 202 | **+12.51%** ($20,024) | +9.36% | +14.41% | 16/16 | 2.50% |
| 21 | RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h (`RZ|30|28|30|55|2.5|6|none`) | 159 | **+12.51%** ($20,009) | +9.62% | +14.24% | 15/16 | 3.21% |
| 22 | EMA13 z-score 2.1 fade 240m, stop 4.0 ATR, 12 h, high-volume bars only, all day (`ZEX|240|13|2.1|4.0|12|high|all`) | 209 | **+12.50%** ($20,001) | +8.11% | +15.13% | 15/16 | 3.31% |
| 23 | 4h Bollinger(1.7) extreme + 60m RSI(21) 39, 06:00 UTC +24 h (`MXX|1.7|60|21|39|6|24|3.0|12`) | 300 | **+12.50%** ($20,001) | +7.53% | +15.48% | 15/16 | 4.68% |
| 24 | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h (`RZ|30|24|30|60|2.5|6|none`) | 243 | **+12.46%** ($19,939) | +9.11% | +14.47% | 15/16 | 3.89% |
| 25 | RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h (`RZ|30|28|30|60|2.5|6|none`) | 159 | **+12.43%** ($19,887) | +9.61% | +14.12% | 15/16 | 3.21% |
| 26 | EMA17 z-score 1.5 fade 240m, stop 1.5 ATR, 20 h, high-volume bars only, all day (`ZEX|240|17|1.5|1.5|20|high|all`) | 306 | **+12.43%** ($19,882) | +4.45% | +17.21% | 12/16 | 5.37% |
| 27 | Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|2.0|4.0|12|none`) | 202 | **+12.42%** ($19,871) | +9.10% | +14.41% | 16/16 | 2.75% |
| 28 | RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|2.5|6|high`) | 229 | **+12.42%** ($19,872) | +9.75% | +14.02% | 15/16 | 3.41% |
| 29 | Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|3.0|12|high`) | 239 | **+12.41%** ($19,861) | +7.14% | +15.58% | 14/16 | 5.22% |
| 30 | Bollinger(20,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|2.0|12|high`) | 283 | **+12.39%** ($19,821) | +6.50% | +15.92% | 15/16 | 4.30% |
| 31 | Bollinger(16,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|2.0|12|high`) | 285 | **+12.39%** ($19,825) | +7.45% | +15.36% | 14/16 | 3.48% |
| 32 | RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|1.5|6|high`) | 181 | **+12.37%** ($19,787) | +9.27% | +14.22% | 15/16 | 3.47% |
| 33 | Bollinger(14,1.4) 240m fade, stop 4.5 ATR, 16 h, high-volume bars only, all day (`BZX|240|14|1.4|4.5|16|high|all`) | 324 | **+12.37%** ($19,793) | +7.88% | +15.06% | 14/16 | 3.50% |
| 34 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h (`RZ|30|28|30|55|2.0|6|none`) | 168 | **+12.36%** ($19,769) | +9.77% | +13.91% | 15/16 | 2.85% |
| 35 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.0|6|high`) | 164 | **+12.35%** ($19,760) | +9.82% | +13.87% | 15/16 | 2.80% |
| 36 | 4h Bollinger(1.7) extreme + 30m RSI(19) 31, 18:00 UTC +8 h (`MXX|1.7|30|19|31|18|8|1.0|6`) | 153 | **+12.34%** ($19,744) | +10.18% | +13.63% | 15/16 | 2.19% |
| 37 | 4h Bollinger(1.9) extreme + 30m RSI(21) 40, 10:00 UTC +24 h (`MXX|1.9|30|21|40|10|24|3.0|12`) | 263 | **+12.30%** ($19,677) | +7.02% | +15.47% | 15/16 | 4.61% |
| 38 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|1.5|6|high`) | 181 | **+12.29%** ($19,664) | +9.26% | +14.11% | 15/16 | 3.47% |
| 39 | RSI(33) 30m fade <33/>67, exit 70, stop 3.5 ATR, 20 h, all day (`RZX|30|33|33|70|3.5|20|none|all`) | 175 | **+12.29%** ($19,662) | +7.40% | +15.22% | 15/16 | 3.99% |
| 40 | RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h (`RZ|30|28|30|60|2.0|6|none`) | 168 | **+12.28%** ($19,647) | +9.76% | +13.79% | 15/16 | 2.85% |
| 41 | RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|2.0|6|high`) | 164 | **+12.27%** ($19,638) | +9.81% | +13.75% | 15/16 | 2.80% |
| 42 | RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h (`RZ|30|28|30|55|1.5|6|none`) | 185 | **+12.26%** ($19,610) | +9.24% | +14.06% | 15/16 | 3.50% |
| 43 | Bollinger(24,1.2) 240m fade, stop 2.0 ATR, 20 h, high-volume bars only, all day (`BZX|240|24|1.2|2.0|20|high|all`) | 340 | **+12.25%** ($19,601) | +4.78% | +16.73% | 13/16 | 4.77% |
| 44 | Bollinger(24,1.8) 180m fade, stop 3.0 ATR, 12 h (`BZ|180|24|1.8|3.0|12|none`) | 318 | **+12.24%** ($19,591) | +6.89% | +15.46% | 15/16 | 3.57% |
| 45 | RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h (`RZ|30|24|30|55|2.5|6|none`) | 243 | **+12.23%** ($19,563) | +8.80% | +14.28% | 15/16 | 3.89% |
| 46 | Bollinger(20,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|4.0|12|high`) | 280 | **+12.19%** ($19,496) | +6.46% | +15.62% | 14/16 | 4.31% |
| 47 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|28|30|60|1.5|6|none`) | 185 | **+12.18%** ($19,488) | +9.23% | +13.95% | 15/16 | 3.50% |
| 48 | RSI(28) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|2.5|6|high`) | 156 | **+12.18%** ($19,484) | +9.22% | +13.95% | 15/16 | 3.15% |
| 49 | RSI(28) 30m fade <32/>68, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|2.5|6|high`) | 247 | **+12.14%** ($19,424) | +7.77% | +14.76% | 15/16 | 3.70% |
| 50 | Bollinger(16,2.0) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|2.0|2.0|12|none`) | 205 | **+12.13%** ($19,410) | +8.97% | +14.03% | 15/16 | 2.29% |
| 51 | Bollinger(16,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|4.0|12|high`) | 282 | **+12.12%** ($19,386) | +7.39% | +14.95% | 13/16 | 4.05% |
| 52 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.75|3.0|8|none`) | 222 | **+12.10%** ($19,352) | +6.57% | +15.41% | 15/16 | 5.04% |
| 53 | RSI(28) 30m fade <32/>68, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|2.5|6|high`) | 247 | **+12.08%** ($19,322) | +7.61% | +14.75% | 15/16 | 3.70% |
| 54 | RSI(18) 30m fade <26/>74, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|18|26|60|2.5|6|high`) | 242 | **+12.00%** ($19,193) | +9.13% | +13.72% | 15/16 | 3.31% |
| 55 | RSI(24) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.0|6|high`) | 250 | **+11.99%** ($19,184) | +9.63% | +13.41% | 15/16 | 3.77% |
| 56 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.5|3.0|8|none`) | 340 | **+11.98%** ($19,172) | +5.41% | +15.93% | 15/16 | 6.23% |
| 57 | RSI(24) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|1.5|6|high`) | 276 | **+11.97%** ($19,156) | +8.71% | +13.93% | 15/16 | 3.68% |
| 58 | RSI(21) 30m fade <28/>72, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|55|2.5|6|high`) | 229 | **+11.96%** ($19,143) | +9.53% | +13.42% | 15/16 | 3.41% |
| 59 | Bollinger(20,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|3.0|12|high`) | 281 | **+11.95%** ($19,115) | +5.87% | +15.59% | 15/16 | 5.14% |
| 60 | RSI(28) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h (`RZ|30|28|30|50|2.5|6|none`) | 159 | **+11.95%** ($19,116) | +9.15% | +13.62% | 15/16 | 3.21% |
| 61 | RSI(18) 30m fade <26/>74, exit 60, stop 2.5 ATR, 6 h (`RZ|30|18|26|60|2.5|6|none`) | 245 | **+11.94%** ($19,104) | +9.14% | +13.62% | 15/16 | 3.72% |
| 62 | 4h Bollinger(1.4) extreme + 30m RSI(23) 36, 18:00 UTC +12 h (`MXX|1.4|30|23|36|18|12|1.0|6`) | 358 | **+11.93%** ($19,093) | +8.86% | +13.78% | 16/16 | 3.47% |
| 63 | RSI(24) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h (`RZ|30|24|30|60|2.0|6|none`) | 258 | **+11.92%** ($19,071) | +9.69% | +13.26% | 15/16 | 4.15% |
| 64 | EMA14 z-score 2.0 fade 240m, stop 2.5 ATR, 12 h, all day (`ZEX|240|14|2.0|2.5|12|none|all`) | 291 | **+11.91%** ($19,062) | +3.96% | +16.68% | 14/16 | 4.85% |
| 65 | RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h (`RZ|30|21|28|60|2.5|6|none`) | 231 | **+11.90%** ($19,035) | +9.49% | +13.34% | 15/16 | 3.66% |
| 66 | Bollinger(16,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|3.0|12|high`) | 282 | **+11.88%** ($19,014) | +7.18% | +14.71% | 13/16 | 4.31% |
| 67 | Bollinger(24,1.8) 180m fade, stop 2.0 ATR, 12 h (`BZ|180|24|1.8|2.0|12|none`) | 328 | **+11.88%** ($19,006) | +5.50% | +15.70% | 14/16 | 3.22% |
| 68 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|3.0|12|high`) | 183 | **+11.85%** ($18,961) | +9.19% | +13.45% | 15/16 | 2.46% |
| 69 | RSI(28) 30m fade <30/>70, exit 50, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|1.5|6|high`) | 181 | **+11.85%** ($18,953) | +8.90% | +13.61% | 15/16 | 3.47% |
| 70 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 2.0 ATR, 8 h (`KZ|240|20|1.5|2.0|8|none`) | 342 | **+11.84%** ($18,948) | +4.52% | +16.23% | 14/16 | 5.56% |
| 71 | 4h Bollinger(1.7) extreme + 30m RSI(12) 34, 12:00 UTC +24 h (`MXX|1.7|30|12|34|12|24|3.0|12`) | 315 | **+11.84%** ($18,942) | +6.85% | +14.83% | 16/16 | 4.76% |
| 72 | RSI(28) 30m fade <32/>68, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|1.5|6|high`) | 286 | **+11.76%** ($18,818) | +7.71% | +14.19% | 15/16 | 4.01% |
| 73 | RSI(24) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|24|30|60|1.5|6|none`) | 284 | **+11.76%** ($18,822) | +8.80% | +13.54% | 15/16 | 4.03% |
| 74 | 4h Bollinger(1.5) extreme + 30m RSI(24) 30, 02:00 UTC +24 h (`MXX|1.5|30|24|30|2|24|2.5|6`) | 156 | **+11.76%** ($18,812) | +8.95% | +13.44% | 15/16 | 3.80% |
| 75 | Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|4.0|12|high`) | 183 | **+11.75%** ($18,808) | +8.93% | +13.45% | 15/16 | 2.72% |
| 76 | RSI(24) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|2.0|6|high`) | 250 | **+11.75%** ($18,808) | +9.32% | +13.22% | 15/16 | 3.77% |
| 77 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|1.5|6|high`) | 276 | **+11.75%** ($18,797) | +8.40% | +13.76% | 15/16 | 3.68% |
| 78 | RSI(28) 30m fade <30/>70, exit 50, stop 1.5 ATR, 6 h (`RZ|30|28|30|50|1.5|6|none`) | 185 | **+11.74%** ($18,776) | +8.87% | +13.45% | 15/16 | 3.50% |
| 79 | RSI(28) 30m fade <32/>68, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|1.5|6|high`) | 286 | **+11.71%** ($18,732) | +7.56% | +14.20% | 15/16 | 4.01% |
| 80 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|2.0|6|high`) | 164 | **+11.70%** ($18,718) | +9.35% | +13.11% | 15/16 | 2.80% |
| 81 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 6 h (`RZ|30|28|30|50|2.0|6|none`) | 168 | **+11.70%** ($18,727) | +9.30% | +13.15% | 15/16 | 2.85% |
| 82 | RSI(21) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|2.0|6|high`) | 242 | **+11.69%** ($18,708) | +9.87% | +12.78% | 15/16 | 3.77% |
| 83 | RSI(24) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h (`RZ|30|24|30|55|2.0|6|none`) | 258 | **+11.68%** ($18,695) | +9.38% | +13.07% | 15/16 | 4.15% |
| 84 | RSI(24) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|50|2.5|6|high`) | 237 | **+11.67%** ($18,678) | +8.43% | +13.62% | 15/16 | 3.49% |
| 85 | 4h Bollinger(1.4) extreme + 30m RSI(24) 34, 20:00 UTC +8 h (`MXX|1.4|30|24|34|20|8|1.0|6`) | 174 | **+11.65%** ($18,635) | +8.63% | +13.46% | 16/16 | 3.36% |
| 86 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 2.0 ATR, 8 h (`KZ|240|20|1.75|2.0|8|none`) | 223 | **+11.63%** ($18,612) | +5.40% | +15.37% | 15/16 | 4.79% |
| 87 | 4h Bollinger(1.5) extreme + 30m RSI(10) 22, 16:00 UTC +24 h (`MXX|1.5|30|10|22|16|24|2.0|6`) | 229 | **+11.62%** ($18,593) | +9.55% | +12.86% | 15/16 | 3.25% |
| 88 | Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 8 h (`BZ|180|24|1.8|4.0|8|none`) | 343 | **+11.61%** ($18,574) | +5.70% | +15.15% | 16/16 | 3.94% |
| 89 | 4h Bollinger(1.9) extreme + 15m RSI(17) 40, 22:00 UTC +24 h (`MXX|1.9|15|17|40|22|24|3.0|12`) | 279 | **+11.61%** ($18,579) | +5.71% | +15.15% | 15/16 | 4.02% |
| 90 | RSI(20) 30m fade <27/>73, exit 60, stop 4.0 ATR, 20 h, high-volume bars only, all day (`RZX|30|20|27|60|4.0|20|high|all`) | 183 | **+11.59%** ($18,541) | +9.06% | +13.11% | 15/16 | 3.08% |
| 91 | 4h Bollinger(1.8) extreme + 30m RSI(19) 39, 10:00 UTC +24 h (`MXX|1.8|30|19|39|10|24|3.0|12`) | 312 | **+11.56%** ($18,497) | +5.89% | +14.96% | 16/16 | 5.28% |
| 92 | RSI(24) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h (`RZ|30|24|28|60|2.5|6|none`) | 161 | **+11.55%** ($18,486) | +9.54% | +12.76% | 15/16 | 3.12% |
| 93 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h (`RZ|30|24|30|55|1.5|6|none`) | 284 | **+11.54%** ($18,463) | +8.48% | +13.37% | 15/16 | 4.03% |
| 94 | RSI(21) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h (`RZ|30|21|28|60|2.0|6|none`) | 248 | **+11.52%** ($18,438) | +9.69% | +12.62% | 15/16 | 4.04% |
| 95 | RSI(28) 30m fade <32/>68, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|2.0|6|high`) | 261 | **+11.51%** ($18,419) | +8.29% | +13.45% | 15/16 | 3.53% |
| 96 | Bollinger(16,2.0) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|2.0|12|high`) | 186 | **+11.49%** ($18,383) | +8.86% | +13.07% | 15/16 | 2.38% |
| 97 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 3.0 ATR, 8 h, target EMA (`KZ|240|20|1.5|3.0|8|ema`) | 340 | **+11.49%** ($18,388) | +5.42% | +15.14% | 15/16 | 6.23% |
| 98 | RSI(24) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|28|60|2.5|6|high`) | 157 | **+11.48%** ($18,370) | +9.53% | +12.65% | 15/16 | 3.12% |
| 99 | 4h Bollinger(1.6) extreme + 30m RSI(20) 37, 16:00 UTC +12 h (`MXX|1.6|30|20|37|16|12|1.0|6`) | 387 | **+11.48%** ($18,376) | +7.60% | +13.82% | 15/16 | 2.87% |
| 100 | Bollinger(30,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|30|1.6|2.0|12|high`) | 245 | **+11.47%** ($18,352) | +6.14% | +14.67% | 15/16 | 4.61% |

## Rules (top 30)

1. **Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
2. **Bollinger(24,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
3. **Bollinger(24,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
4. **Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
5. **Sell 09:00 UTC for 12 h (weekdays)**: Weekdays at 09:00 UTC sell; close 12 h later.
6. **Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
7. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
8. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h.
9. **Stochastic(27) 240m <29/>71, all day**: Buy when %K is under 29 (short over 71); exit at 50, after 16 h or at a 3.0 ATR stop.
10. **Sell 07:00 UTC for 12 h, only against the last hour's RSI (weekdays)**: Weekdays at 07:00 UTC sell, only against the last hour's RSI; close 12 h later.
11. **Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
12. **Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
13. **Bollinger(24,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
14. **RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
15. **RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop, high-volume bars only.
16. **RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
17. **Bollinger(24,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
18. **Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 12 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
19. **RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop, high-volume bars only.
20. **Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
21. **RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop.
22. **EMA13 z-score 2.1 fade 240m, stop 4.0 ATR, 12 h, high-volume bars only, all day**: Fade price 2.1 standard deviations from its EMA13 on 240-min bars; exit on the cross back, after 12 h or at a 4.0 ATR stop.
23. **4h Bollinger(1.7) extreme + 60m RSI(21) 39, 06:00 UTC +24 h**: When the last 4h close is beyond its Bollinger 1.7 band, fade a 60-min RSI(21) extreme (39/61) between 06:00 and 06:00 UTC; stop 3.0 ATR, up to 12 h.
24. **RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop.
25. **RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop.
26. **EMA17 z-score 1.5 fade 240m, stop 1.5 ATR, 20 h, high-volume bars only, all day**: Fade price 1.5 standard deviations from its EMA17 on 240-min bars; exit on the cross back, after 20 h or at a 1.5 ATR stop.
27. **Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
28. **RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(21) under 28 (short over 72); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
29. **Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
30. **Bollinger(20,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.

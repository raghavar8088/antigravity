# Profitable intraday crypto strategies, October 2026

Generated 2026-11-01 17:03 UTC. October 2026 only (no trading on the 28th). Delta India fees, slippage and funding; $10,000 per trade, no leverage, flat by 00:00 UTC.

- Tested 146,083 strategies over 298 rounds on BTC ETH SOL XRP DOGE BNB: 43,072 profitable, 16,568 consistent (both halves of October, 3+ of 6 coins, 10+ trades).
- **14,311 also made money on 10 other coins** (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT; 5+ of 10).

## Best per family

| Family | Strategy | October return, 16 coins |
|---|---|---:|
| Bollinger fade | Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|2.0|12|high`) | +14.83% |
| Band limit fade (maker) | Keltner(EMA18, 3.0 ATR) 180m limit fade, 1 bar(s), stop 4.0 ATR, 8 h, all day (`BLX|180|kel|18|3.0|1|4.0|8|all`) | +14.49% |
| Hour-of-day | Sell 09:00 UTC for 12 h (weekdays) (`HZX|9|12|short|none|wkday`) | +13.67% |
| Stochastic extreme | Stochastic(27) 240m <29/>71, all day (`STX|240|27|29|3.0|16|all`) | +13.27% |
| RSI fade | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.5|6|high`) | +12.76% |
| 4h band + fast RSI | 4h Bollinger(1.9) extreme + 30m RSI(21) 39, 06:00 UTC +24 h (`MXX|1.9|30|21|39|6|24|3.0|12`) | +12.54% |
| EMA z-score fade | EMA13 z-score 2.1 fade 240m, stop 4.0 ATR, 12 h, high-volume bars only, all day (`ZEX|240|13|2.1|4.0|12|high|all`) | +12.50% |
| Keltner channel | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.75|3.0|8|none`) | +12.10% |
| RSI fade by hour and VWAP | RSI(32) 1h fade 38/62 from 18:00 UTC for 10 h, VWAP any, exit 50 (`CZX|18|10|any|32|38|50`) | +11.14% |
| Fade from the weekly open | Fade 1.75 ATR from the weekly open (30m check), target the open, all day (`WOF|1.75|30|anchor|1.5|0|all`) | +9.77% |
| Relative value vs BTC | Relative value vs BTC: fade a 2.25-sd move of coin/BTC (72 x 60m), exit at 0.5 sd, stop 3.5 ATR, 16 h, all day (`RVF|60|72|2.25|0.5|3.5|16|all`) | +9.44% |
| 4h Bollinger + 1h RSI | 4h Bollinger(2.0) extreme + 1h RSI(21) 35 (`MTF|2.0|35|21|1.0|6`) | +9.42% |
| Day-of-week window | Friday 01:00 UTC for 8 h, long (`DWX|4|1|8|long`) | +8.93% |
| Williams %R | Williams %R(28) 240m beyond -75/-25, all day (`WRX|240|28|25|3.0|8|all`) | +8.25% |
| Sharp-move momentum/reversal | Fade a 2.5 ATR move over 10 x 180m (`RCX|180|10|2.5|4|rev|none`) | +7.47% |
| Volume spike | Fade a 120m volume spike (1.75x volume, 0.75 ATR range), stop 3.0 ATR, 6 h, all day (`VSP|120|1.75|0.75|fade|3.0|6|all`) | +7.20% |
| Asian-range limit fade | Asian range 00:00-06:00 UTC limit fade until 11:00, edge +0.0, stop 1.0, target far edge (`RNG|6|11|0.0|1.0|far`) | +7.05% |
| Fisher transform | Fisher(13) 180m fade beyond 1.4, all day (`FSX|180|13|1.4|1.0|12|all`) | +7.00% |
| Candle pattern | Pin 180m, 4.0R target, with trend (`CDX|180|pin|trend|4.0|10`) | +6.98% |
| Candle-streak fade | Fade 4 same-colour 120m candles, all day (`SKX|120|4|12|3.0|all`) | +6.83% |
| Previous-day high/low failure | Previous-day high/low failure 60m, stop 0.5 ATR beyond, target 2.0R, 20 h, all day (`PDF|60|0.5|2.0|20|all`) | +6.83% |
| Bollinger fade + regime filter | Bollinger(34,2.4) 90m fade, only against the 10-day trend, stop 2.5 ATR, 20 h, all day (`BZR|90|34|2.4|2.5|20|none|against|all`) | +6.79% |
| RSI divergence | RSI(10) divergence 240m (37-bar extreme, 4 pts), exit 50, stop 2.5 ATR, 16 h, all day (`DIV|240|37|10|4|50|2.5|16|all`) | +6.41% |
| CCI extreme | CCI(15) 60m fade at 150, all day (`CCX|60|15|150|fade|12|all`) | +6.35% |
| Money Flow Index | MFI(24) 60m <26/>74, all day (`MFX|60|24|26|3.5|6|all`) | +6.32% |
| Fade from yesterday's close | Fade 0.5 ATR from yesterday's close (60m check), target the anchor, all day (`PCF|0.5|60|anchor|0.5|0|all`) | +6.29% |
| EMA pullback | EMA10 pullback 15m, 4.0R target, US hours (`EPX|15|10|2.5|4.0|us`) | +6.27% |
| Fade from the day's open | Fade 0.5 ATR from the 00:00 UTC open (60m check), target the anchor, all day (`DOF|0.5|60|anchor|0.5|0|all`) | +5.99% |
| Pivot fade | Pivot fade at R1/S1, all day (`N7f|1|none|all`) | +5.53% |
| Pivot levels | Classic pivot fade at level 1, all day (`PIV|classic|1|fade|all`) | +5.53% |
| Session-open breakout | Hong Kong (01:30 UTC) 10-min breakout, retest entry, 4.0R target (`ORX|hk|10|retest|far|4.0|none`) | +4.96% |
| Turtle soup (failed breakout) | Turtle soup 27-bar 180m, stop 0.75 ATR beyond, target 2.0R, 20 h, high-volume bars only, all day (`TSP|180|27|0.75|2.0|20|high|all`) | +4.82% |
| Funding-rate tilt | 01:00 UTC for 3 h against the last funding print (|rate| >= 0.005%) (`FND|1|3|5e-05|fade|0.0`) | +4.45% |
| Session-range breakout | 00:00-08:00 UTC range breakout until 21:00, edge +0.25, stop far side, 1.5R target, with the 10-day trend (`RBX|8|21|0.25|far|1.5|trend`) | +3.75% |
| Connors RSI | Connors RSI 240m <15/>85, exit 50, all day (`CRS|240|15|50|2.0|4|all`) | +3.68% |
| DMI cross | DMI(18) cross 60m, ADX>26, European hours (`DMX|60|18|26|eu|none`) | +3.61% |
| VWAP band limit fade (maker) | VWAP 2.25-sd band limit fade on 60m (from 4 h into the day), stop 2.0 sd, 6 h, European hours (`VWB|60|2.25|4|2|2.0|6|eu`) | +3.12% |
| Flush reversal | Flush reversal: fade a 1.5 ATR move over 5 x 30m on 3.0x volume, target 1.0 retrace, 4 h, Asian hours (`FLX|30|5|1.5|3.0|0.0|1.0|4|asia`) | +3.05% |
| Volatility breakout from the open | Volatility breakout 0.4 x range from the NYSE open, 1.5R target, after a narrow-range day (`VBO|us|0.4|range|0|atr|1.5|nr4`) | +2.95% |
| Parabolic SAR | Parabolic SAR(0.05) 60m, US hours (`SRX|60|0.05|us|none`) | +2.65% |
| Bollinger breakout | Bollinger(20,2.0) 30m breakout, US hours, with trend (`BBB|30|20|2.0|atr|1.5|trend|us`) | +2.18% |
| Range-expansion bar | Fade a 2.9 ATR 180m bar (`ATY|180|2.9|fade|2|none`) | +2.16% |
| Bollinger squeeze breakout | Bollinger(18,1.75) squeeze breakout 120m (width in lowest 20% of 100 bars), 1.0R target, Asian hours (`SQZ|120|18|1.75|20|100|5|3.0|1.0|16|asia`) | +2.15% |
| Supertrend | Supertrend 7x3.0 on 15m, US hours, with 10-day trend (`STR|15|7|3.0|us|trend`) | +2.10% |
| Cross-coin intraday | Cross-coin reversal at 06:00 UTC, 1 each side, to 00:00 (`XS|6|4h|rev|1|0`) | +1.77% |
| Donchian breakout | Donchian(6) 90m breakout, US hours, with trend (`DNX|90|6|atr|3.0|trend|us`) | +1.77% |
| US pre-market breakout | US pre-market breakout, retest entry, stop far, target 3R (`N10|retest|far|3R|none`) | +1.70% |
| BTC lead-lag | Lead-lag: catch up with a 2.75 ATR BTC move over 4 x 5m, lag <0.5, 2 h, US hours (`LLG|5|4|2.75|0.5|1.5|2|us`) | +1.52% |
| Trend-day follow | Trend-day follow at 1 h after the NYSE open (`B|US1h|0.5|trend|open`) | +0.77% |

## Top 100

| # | Strategy | Trades | October, 16 coins | 6 coins | 10 new coins | Coins up | Worst DD |
|---|---|---:|---:|---:|---:|---:|---:|
| 1 | Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|2.0|12|high`) | 276 | **+14.83%** ($23,734) | +8.08% | +18.89% | 16/16 | 4.65% |
| 2 | Keltner(EMA18, 3.0 ATR) 180m limit fade, 1 bar(s), stop 4.0 ATR, 8 h, all day (`BLX|180|kel|18|3.0|1|4.0|8|all`) | 181 | **+14.49%** ($23,187) | +9.07% | +17.74% | 16/16 | 1.52% |
| 3 | Bollinger(24,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|4.0|12|high`) | 275 | **+14.38%** ($23,007) | +7.87% | +18.29% | 16/16 | 5.08% |
| 4 | Bollinger(24,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|3.0|12|high`) | 275 | **+14.14%** ($22,623) | +7.27% | +18.26% | 16/16 | 5.67% |
| 5 | Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|4.0|12|high`) | 229 | **+13.67%** ($21,876) | +9.21% | +16.35% | 16/16 | 2.71% |
| 6 | Sell 09:00 UTC for 12 h (weekdays) (`HZX|9|12|short|none|wkday`) | 336 | **+13.67%** ($21,864) | +12.29% | +14.49% | 15/16 | 6.24% |
| 7 | Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|3.0|12|high`) | 229 | **+13.63%** ($21,803) | +9.46% | +16.13% | 16/16 | 2.58% |
| 8 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|2.0|12|high`) | 233 | **+13.61%** ($21,775) | +9.13% | +16.30% | 16/16 | 2.94% |
| 9 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|1.8|2.0|12|none`) | 274 | **+13.30%** ($21,273) | +6.71% | +17.25% | 16/16 | 3.26% |
| 10 | Stochastic(27) 240m <29/>71, all day (`STX|240|27|29|3.0|16|all`) | 589 | **+13.27%** ($21,229) | +8.89% | +15.89% | 14/16 | 6.51% |
| 11 | Sell 07:00 UTC for 12 h, only against the last hour's RSI (weekdays) (`HZX|7|12|short|rsi1h|wkday`) | 164 | **+13.24%** ($21,181) | +7.68% | +16.57% | 15/16 | 2.98% |
| 12 | Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|1.8|3.0|12|none`) | 266 | **+13.12%** ($20,993) | +6.64% | +17.01% | 16/16 | 3.72% |
| 13 | Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|1.8|4.0|12|none`) | 266 | **+13.10%** ($20,964) | +6.21% | +17.24% | 16/16 | 4.15% |
| 14 | Stochastic(29) 240m <28/>72, all day (`STX|240|29|28|2.5|20|all`) | 518 | **+13.05%** ($20,887) | +6.26% | +17.13% | 13/16 | 5.75% |
| 15 | Bollinger(24,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|2.0|12|high`) | 240 | **+13.01%** ($20,819) | +7.71% | +16.19% | 16/16 | 4.49% |
| 16 | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.5|6|high`) | 237 | **+12.76%** ($20,409) | +9.25% | +14.86% | 15/16 | 3.49% |
| 17 | RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.5|6|high`) | 156 | **+12.74%** ($20,378) | +9.68% | +14.57% | 15/16 | 3.15% |
| 18 | RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|2.5|6|high`) | 156 | **+12.66%** ($20,256) | +9.67% | +14.45% | 15/16 | 3.15% |
| 19 | Bollinger(24,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|4.0|12|high`) | 238 | **+12.65%** ($20,242) | +7.72% | +15.61% | 15/16 | 4.08% |
| 20 | Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 12 h (`BZ|180|24|1.8|4.0|12|none`) | 315 | **+12.62%** ($20,198) | +7.21% | +15.87% | 15/16 | 3.46% |
| 21 | 4h Bollinger(1.9) extreme + 30m RSI(21) 39, 06:00 UTC +24 h (`MXX|1.9|30|21|39|6|24|3.0|12`) | 261 | **+12.54%** ($20,060) | +7.42% | +15.61% | 15/16 | 4.52% |
| 22 | RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|2.5|6|high`) | 237 | **+12.52%** ($20,033) | +8.94% | +14.67% | 15/16 | 3.49% |
| 23 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|2.0|3.0|12|none`) | 202 | **+12.51%** ($20,024) | +9.36% | +14.41% | 16/16 | 2.50% |
| 24 | RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h (`RZ|30|28|30|55|2.5|6|none`) | 159 | **+12.51%** ($20,009) | +9.62% | +14.24% | 15/16 | 3.21% |
| 25 | EMA13 z-score 2.1 fade 240m, stop 4.0 ATR, 12 h, high-volume bars only, all day (`ZEX|240|13|2.1|4.0|12|high|all`) | 209 | **+12.50%** ($20,001) | +8.11% | +15.13% | 15/16 | 3.31% |
| 26 | 4h Bollinger(1.7) extreme + 60m RSI(21) 39, 06:00 UTC +24 h (`MXX|1.7|60|21|39|6|24|3.0|12`) | 300 | **+12.50%** ($20,001) | +7.53% | +15.48% | 15/16 | 4.68% |
| 27 | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h (`RZ|30|24|30|60|2.5|6|none`) | 243 | **+12.46%** ($19,939) | +9.11% | +14.47% | 15/16 | 3.89% |
| 28 | RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h (`RZ|30|28|30|60|2.5|6|none`) | 159 | **+12.43%** ($19,887) | +9.61% | +14.12% | 15/16 | 3.21% |
| 29 | EMA17 z-score 1.5 fade 240m, stop 1.5 ATR, 20 h, high-volume bars only, all day (`ZEX|240|17|1.5|1.5|20|high|all`) | 306 | **+12.43%** ($19,882) | +4.45% | +17.21% | 12/16 | 5.37% |
| 30 | Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|2.0|4.0|12|none`) | 202 | **+12.42%** ($19,871) | +9.10% | +14.41% | 16/16 | 2.75% |
| 31 | RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|2.5|6|high`) | 229 | **+12.42%** ($19,872) | +9.75% | +14.02% | 15/16 | 3.41% |
| 32 | Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|3.0|12|high`) | 239 | **+12.41%** ($19,861) | +7.14% | +15.58% | 14/16 | 5.22% |
| 33 | Bollinger(20,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|2.0|12|high`) | 283 | **+12.39%** ($19,821) | +6.50% | +15.92% | 15/16 | 4.30% |
| 34 | Bollinger(16,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|2.0|12|high`) | 285 | **+12.39%** ($19,825) | +7.45% | +15.36% | 14/16 | 3.48% |
| 35 | RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|1.5|6|high`) | 181 | **+12.37%** ($19,787) | +9.27% | +14.22% | 15/16 | 3.47% |
| 36 | Bollinger(14,1.4) 240m fade, stop 4.5 ATR, 16 h, high-volume bars only, all day (`BZX|240|14|1.4|4.5|16|high|all`) | 324 | **+12.37%** ($19,793) | +7.88% | +15.06% | 14/16 | 3.50% |
| 37 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h (`RZ|30|28|30|55|2.0|6|none`) | 168 | **+12.36%** ($19,769) | +9.77% | +13.91% | 15/16 | 2.85% |
| 38 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.0|6|high`) | 164 | **+12.35%** ($19,760) | +9.82% | +13.87% | 15/16 | 2.80% |
| 39 | 4h Bollinger(1.7) extreme + 30m RSI(19) 31, 18:00 UTC +8 h (`MXX|1.7|30|19|31|18|8|1.0|6`) | 153 | **+12.34%** ($19,744) | +10.18% | +13.63% | 15/16 | 2.19% |
| 40 | 4h Bollinger(1.9) extreme + 30m RSI(21) 40, 10:00 UTC +24 h (`MXX|1.9|30|21|40|10|24|3.0|12`) | 263 | **+12.30%** ($19,677) | +7.02% | +15.47% | 15/16 | 4.61% |
| 41 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|1.5|6|high`) | 181 | **+12.29%** ($19,664) | +9.26% | +14.11% | 15/16 | 3.47% |
| 42 | RSI(33) 30m fade <33/>67, exit 70, stop 3.5 ATR, 20 h, all day (`RZX|30|33|33|70|3.5|20|none|all`) | 175 | **+12.29%** ($19,662) | +7.40% | +15.22% | 15/16 | 3.99% |
| 43 | RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h (`RZ|30|28|30|60|2.0|6|none`) | 168 | **+12.28%** ($19,647) | +9.76% | +13.79% | 15/16 | 2.85% |
| 44 | RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|2.0|6|high`) | 164 | **+12.27%** ($19,638) | +9.81% | +13.75% | 15/16 | 2.80% |
| 45 | RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h (`RZ|30|28|30|55|1.5|6|none`) | 185 | **+12.26%** ($19,610) | +9.24% | +14.06% | 15/16 | 3.50% |
| 46 | Bollinger(24,1.2) 240m fade, stop 2.0 ATR, 20 h, high-volume bars only, all day (`BZX|240|24|1.2|2.0|20|high|all`) | 340 | **+12.25%** ($19,601) | +4.78% | +16.73% | 13/16 | 4.77% |
| 47 | Bollinger(24,1.8) 180m fade, stop 3.0 ATR, 12 h (`BZ|180|24|1.8|3.0|12|none`) | 318 | **+12.24%** ($19,591) | +6.89% | +15.46% | 15/16 | 3.57% |
| 48 | RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h (`RZ|30|24|30|55|2.5|6|none`) | 243 | **+12.23%** ($19,563) | +8.80% | +14.28% | 15/16 | 3.89% |
| 49 | Bollinger(20,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|4.0|12|high`) | 280 | **+12.19%** ($19,496) | +6.46% | +15.62% | 14/16 | 4.31% |
| 50 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|28|30|60|1.5|6|none`) | 185 | **+12.18%** ($19,488) | +9.23% | +13.95% | 15/16 | 3.50% |
| 51 | RSI(28) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|2.5|6|high`) | 156 | **+12.18%** ($19,484) | +9.22% | +13.95% | 15/16 | 3.15% |
| 52 | RSI(28) 30m fade <32/>68, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|2.5|6|high`) | 247 | **+12.14%** ($19,424) | +7.77% | +14.76% | 15/16 | 3.70% |
| 53 | Bollinger(16,2.0) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|2.0|2.0|12|none`) | 205 | **+12.13%** ($19,410) | +8.97% | +14.03% | 15/16 | 2.29% |
| 54 | Bollinger(16,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|4.0|12|high`) | 282 | **+12.12%** ($19,386) | +7.39% | +14.95% | 13/16 | 4.05% |
| 55 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.75|3.0|8|none`) | 222 | **+12.10%** ($19,352) | +6.57% | +15.41% | 15/16 | 5.04% |
| 56 | RSI(28) 30m fade <32/>68, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|2.5|6|high`) | 247 | **+12.08%** ($19,322) | +7.61% | +14.75% | 15/16 | 3.70% |
| 57 | RSI(18) 30m fade <26/>74, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|18|26|60|2.5|6|high`) | 242 | **+12.00%** ($19,193) | +9.13% | +13.72% | 15/16 | 3.31% |
| 58 | RSI(24) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.0|6|high`) | 250 | **+11.99%** ($19,184) | +9.63% | +13.41% | 15/16 | 3.77% |
| 59 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.5|3.0|8|none`) | 340 | **+11.98%** ($19,172) | +5.41% | +15.93% | 15/16 | 6.23% |
| 60 | RSI(24) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|1.5|6|high`) | 276 | **+11.97%** ($19,156) | +8.71% | +13.93% | 15/16 | 3.68% |
| 61 | RSI(21) 30m fade <28/>72, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|55|2.5|6|high`) | 229 | **+11.96%** ($19,143) | +9.53% | +13.42% | 15/16 | 3.41% |
| 62 | Bollinger(20,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|3.0|12|high`) | 281 | **+11.95%** ($19,115) | +5.87% | +15.59% | 15/16 | 5.14% |
| 63 | RSI(28) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h (`RZ|30|28|30|50|2.5|6|none`) | 159 | **+11.95%** ($19,116) | +9.15% | +13.62% | 15/16 | 3.21% |
| 64 | RSI(18) 30m fade <26/>74, exit 60, stop 2.5 ATR, 6 h (`RZ|30|18|26|60|2.5|6|none`) | 245 | **+11.94%** ($19,104) | +9.14% | +13.62% | 15/16 | 3.72% |
| 65 | 4h Bollinger(1.4) extreme + 30m RSI(23) 36, 18:00 UTC +12 h (`MXX|1.4|30|23|36|18|12|1.0|6`) | 358 | **+11.93%** ($19,093) | +8.86% | +13.78% | 16/16 | 3.47% |
| 66 | RSI(24) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h (`RZ|30|24|30|60|2.0|6|none`) | 258 | **+11.92%** ($19,071) | +9.69% | +13.26% | 15/16 | 4.15% |
| 67 | EMA14 z-score 2.0 fade 240m, stop 2.5 ATR, 12 h, all day (`ZEX|240|14|2.0|2.5|12|none|all`) | 291 | **+11.91%** ($19,062) | +3.96% | +16.68% | 14/16 | 4.85% |
| 68 | RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h (`RZ|30|21|28|60|2.5|6|none`) | 231 | **+11.90%** ($19,035) | +9.49% | +13.34% | 15/16 | 3.66% |
| 69 | 4h Bollinger(1.9) extreme + 60m RSI(15) 40, 18:00 UTC +12 h (`MXX|1.9|60|15|40|18|12|2.5|12`) | 218 | **+11.89%** ($19,030) | +6.60% | +15.07% | 15/16 | 2.03% |
| 70 | Bollinger(16,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|3.0|12|high`) | 282 | **+11.88%** ($19,014) | +7.18% | +14.71% | 13/16 | 4.31% |
| 71 | Bollinger(24,1.8) 180m fade, stop 2.0 ATR, 12 h (`BZ|180|24|1.8|2.0|12|none`) | 328 | **+11.88%** ($19,006) | +5.50% | +15.70% | 14/16 | 3.22% |
| 72 | 4h Bollinger(2.0) extreme + 60m RSI(19) 40, 12:00 UTC +24 h (`MXX|2.0|60|19|40|12|24|2.5|12`) | 223 | **+11.86%** ($18,982) | +8.32% | +13.99% | 15/16 | 2.77% |
| 73 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|3.0|12|high`) | 183 | **+11.85%** ($18,961) | +9.19% | +13.45% | 15/16 | 2.46% |
| 74 | RSI(28) 30m fade <30/>70, exit 50, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|1.5|6|high`) | 181 | **+11.85%** ($18,953) | +8.90% | +13.61% | 15/16 | 3.47% |
| 75 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 2.0 ATR, 8 h (`KZ|240|20|1.5|2.0|8|none`) | 342 | **+11.84%** ($18,948) | +4.52% | +16.23% | 14/16 | 5.56% |
| 76 | 4h Bollinger(1.7) extreme + 30m RSI(12) 34, 12:00 UTC +24 h (`MXX|1.7|30|12|34|12|24|3.0|12`) | 315 | **+11.84%** ($18,942) | +6.85% | +14.83% | 16/16 | 4.76% |
| 77 | Bollinger(26,1.9) 240m fade, stop 4.5 ATR, 12 h, high-volume bars only, all day (`BZX|240|26|1.9|4.5|12|high|all`) | 202 | **+11.84%** ($18,951) | +7.46% | +14.47% | 15/16 | 3.70% |
| 78 | RSI(28) 30m fade <32/>68, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|1.5|6|high`) | 286 | **+11.76%** ($18,818) | +7.71% | +14.19% | 15/16 | 4.01% |
| 79 | RSI(24) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|24|30|60|1.5|6|none`) | 284 | **+11.76%** ($18,822) | +8.80% | +13.54% | 15/16 | 4.03% |
| 80 | 4h Bollinger(1.5) extreme + 30m RSI(24) 30, 02:00 UTC +24 h (`MXX|1.5|30|24|30|2|24|2.5|6`) | 156 | **+11.76%** ($18,812) | +8.95% | +13.44% | 15/16 | 3.80% |
| 81 | Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|4.0|12|high`) | 183 | **+11.75%** ($18,808) | +8.93% | +13.45% | 15/16 | 2.72% |
| 82 | RSI(24) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|2.0|6|high`) | 250 | **+11.75%** ($18,808) | +9.32% | +13.22% | 15/16 | 3.77% |
| 83 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|1.5|6|high`) | 276 | **+11.75%** ($18,797) | +8.40% | +13.76% | 15/16 | 3.68% |
| 84 | RSI(28) 30m fade <30/>70, exit 50, stop 1.5 ATR, 6 h (`RZ|30|28|30|50|1.5|6|none`) | 185 | **+11.74%** ($18,776) | +8.87% | +13.45% | 15/16 | 3.50% |
| 85 | RSI(28) 30m fade <32/>68, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|1.5|6|high`) | 286 | **+11.71%** ($18,732) | +7.56% | +14.20% | 15/16 | 4.01% |
| 86 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|2.0|6|high`) | 164 | **+11.70%** ($18,718) | +9.35% | +13.11% | 15/16 | 2.80% |
| 87 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 6 h (`RZ|30|28|30|50|2.0|6|none`) | 168 | **+11.70%** ($18,727) | +9.30% | +13.15% | 15/16 | 2.85% |
| 88 | RSI(21) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|2.0|6|high`) | 242 | **+11.69%** ($18,708) | +9.87% | +12.78% | 15/16 | 3.77% |
| 89 | RSI(24) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h (`RZ|30|24|30|55|2.0|6|none`) | 258 | **+11.68%** ($18,695) | +9.38% | +13.07% | 15/16 | 4.15% |
| 90 | RSI(24) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|50|2.5|6|high`) | 237 | **+11.67%** ($18,678) | +8.43% | +13.62% | 15/16 | 3.49% |
| 91 | 4h Bollinger(1.4) extreme + 30m RSI(24) 34, 20:00 UTC +8 h (`MXX|1.4|30|24|34|20|8|1.0|6`) | 174 | **+11.65%** ($18,635) | +8.63% | +13.46% | 16/16 | 3.36% |
| 92 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 2.0 ATR, 8 h (`KZ|240|20|1.75|2.0|8|none`) | 223 | **+11.63%** ($18,612) | +5.40% | +15.37% | 15/16 | 4.79% |
| 93 | 4h Bollinger(1.5) extreme + 30m RSI(10) 22, 16:00 UTC +24 h (`MXX|1.5|30|10|22|16|24|2.0|6`) | 229 | **+11.62%** ($18,593) | +9.55% | +12.86% | 15/16 | 3.25% |
| 94 | Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 8 h (`BZ|180|24|1.8|4.0|8|none`) | 343 | **+11.61%** ($18,574) | +5.70% | +15.15% | 16/16 | 3.94% |
| 95 | 4h Bollinger(1.9) extreme + 15m RSI(17) 40, 22:00 UTC +24 h (`MXX|1.9|15|17|40|22|24|3.0|12`) | 279 | **+11.61%** ($18,579) | +5.71% | +15.15% | 15/16 | 4.02% |
| 96 | RSI(20) 30m fade <27/>73, exit 60, stop 4.0 ATR, 20 h, high-volume bars only, all day (`RZX|30|20|27|60|4.0|20|high|all`) | 183 | **+11.59%** ($18,541) | +9.06% | +13.11% | 15/16 | 3.08% |
| 97 | 4h Bollinger(1.8) extreme + 30m RSI(19) 39, 10:00 UTC +24 h (`MXX|1.8|30|19|39|10|24|3.0|12`) | 312 | **+11.56%** ($18,497) | +5.89% | +14.96% | 16/16 | 5.28% |
| 98 | RSI(24) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h (`RZ|30|24|28|60|2.5|6|none`) | 161 | **+11.55%** ($18,486) | +9.54% | +12.76% | 15/16 | 3.12% |
| 99 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h (`RZ|30|24|30|55|1.5|6|none`) | 284 | **+11.54%** ($18,463) | +8.48% | +13.37% | 15/16 | 4.03% |
| 100 | RSI(21) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h (`RZ|30|21|28|60|2.0|6|none`) | 248 | **+11.52%** ($18,438) | +9.69% | +12.62% | 15/16 | 4.04% |

## Rules (top 30)

1. **Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
2. **Keltner(EMA18, 3.0 ATR) 180m limit fade, 1 bar(s), stop 4.0 ATR, 8 h, all day**: After each 180-min close, rest a buy limit at the lower band and a sell limit at the upper band for 1 bar(s) (the first fill cancels the other; maker fee); target the middle, stop 4.0 ATR, up to 8 h.
3. **Bollinger(24,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
4. **Bollinger(24,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
5. **Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
6. **Sell 09:00 UTC for 12 h (weekdays)**: Weekdays at 09:00 UTC sell; close 12 h later.
7. **Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
8. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
9. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h.
10. **Stochastic(27) 240m <29/>71, all day**: Buy when %K is under 29 (short over 71); exit at 50, after 16 h or at a 3.0 ATR stop.
11. **Sell 07:00 UTC for 12 h, only against the last hour's RSI (weekdays)**: Weekdays at 07:00 UTC sell, only against the last hour's RSI; close 12 h later.
12. **Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
13. **Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
14. **Stochastic(29) 240m <28/>72, all day**: Buy when %K is under 28 (short over 72); exit at 50, after 20 h or at a 2.5 ATR stop.
15. **Bollinger(24,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
16. **RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
17. **RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop, high-volume bars only.
18. **RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
19. **Bollinger(24,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
20. **Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 12 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
21. **4h Bollinger(1.9) extreme + 30m RSI(21) 39, 06:00 UTC +24 h**: When the last 4h close is beyond its Bollinger 1.9 band, fade a 30-min RSI(21) extreme (39/61) between 06:00 and 06:00 UTC; stop 3.0 ATR, up to 12 h.
22. **RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop, high-volume bars only.
23. **Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
24. **RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop.
25. **EMA13 z-score 2.1 fade 240m, stop 4.0 ATR, 12 h, high-volume bars only, all day**: Fade price 2.1 standard deviations from its EMA13 on 240-min bars; exit on the cross back, after 12 h or at a 4.0 ATR stop.
26. **4h Bollinger(1.7) extreme + 60m RSI(21) 39, 06:00 UTC +24 h**: When the last 4h close is beyond its Bollinger 1.7 band, fade a 60-min RSI(21) extreme (39/61) between 06:00 and 06:00 UTC; stop 3.0 ATR, up to 12 h.
27. **RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop.
28. **RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop.
29. **EMA17 z-score 1.5 fade 240m, stop 1.5 ATR, 20 h, high-volume bars only, all day**: Fade price 1.5 standard deviations from its EMA17 on 240-min bars; exit on the cross back, after 20 h or at a 1.5 ATR stop.
30. **Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.

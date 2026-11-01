# Profitable intraday crypto strategies, October 2026

Generated 2026-11-01 15:53 UTC. October 2026 only (no trading on the 28th). Delta India fees, slippage and funding; $10,000 per trade, no leverage, flat by 00:00 UTC.

- Tested 71,583 strategies over 149 rounds on BTC ETH SOL XRP DOGE BNB: 23,308 profitable, 8,569 consistent (both halves of October, 3+ of 6 coins, 10+ trades).
- **7,599 also made money on 10 other coins** (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT; 5+ of 10).

## Best per family

| Family | Strategy | October return, 16 coins |
|---|---|---:|
| Bollinger fade | Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|2.0|12|high`) | +14.83% |
| RSI fade | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.5|6|high`) | +12.76% |
| Keltner channel | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.75|3.0|8|none`) | +12.10% |
| Hour-of-day | Sell 15:00 UTC for 9 h, only after a busy hour (weekdays) (`HZ|15|9|short|highvol|wkday`) | +10.95% |
| RSI fade by hour and VWAP | RSI(28) 1h fade 35/65 from 18:00 UTC for 14 h, VWAP against, exit 60 (`CZ|18|14|against|28|35|60`) | +10.95% |
| 4h Bollinger + 1h RSI | 4h Bollinger(2.0) extreme + 1h RSI(21) 35 (`MTF|2.0|35|21|1.0|6`) | +9.42% |
| 4h band + fast RSI | 4h Bollinger(1.5) extreme + 30m RSI(14) 25 (`MT2|bb|1.5|30|14|25|2.0|4`) | +9.33% |
| EMA z-score fade | EMA20 z-score 2.5 fade on 240m, all day (`ZEM|240|20|2.5|2.0|12|all`) | +7.81% |
| Day-of-week window | Friday 00:00 UTC for 6 h, long (`DOW|4|0|6|long`) | +7.68% |
| Fade from yesterday's close | Fade 0.5 ATR from yesterday's close (60m check), target the anchor, all day (`PCF|0.5|60|anchor|0.5|0|all`) | +6.29% |
| Money Flow Index | MFI(21) 240m <30/>70, all day (`MFI|240|21|30|2.0|8|all`) | +6.02% |
| Fade from the day's open | Fade 0.5 ATR from the 00:00 UTC open (60m check), target the anchor, all day (`DOF|0.5|60|anchor|0.5|0|all`) | +5.99% |
| Pivot fade | Pivot fade at R1/S1, all day (`N7f|1|none|all`) | +5.53% |
| Pivot levels | Classic pivot fade at level 1, all day (`PIV|classic|1|fade|all`) | +5.53% |
| Williams %R | Williams %R(21) 240m beyond -95/-5, all day (`WPR|240|21|5|2.0|8|all`) | +5.12% |
| Sharp-move momentum/reversal | Fade a 3.0 ATR move over 6 x 60m (`ROC|60|6|3.0|4|rev|none`) | +4.53% |
| Fisher transform | Fisher(14) 30m fade beyond 2.5, all day (`FSH|30|14|2.5|2.0|8|all`) | +4.07% |
| Session-open breakout | US-open 60-min breakout, stop entry, stop at far side, 3R target, with 10-day trend (`A|US|60|stop|far|3.0|trend`) | +4.05% |
| Connors RSI | Connors RSI 240m <15/>85, exit 50, all day (`CRS|240|15|50|2.0|4|all`) | +3.68% |
| Bollinger breakout | Bollinger(20,2.0) 30m breakout, US hours, with trend (`BBB|30|20|2.0|atr|1.5|trend|us`) | +2.18% |
| Cross-coin intraday | Cross-coin reversal at 06:00 UTC, 1 each side, to 00:00 (`XS|6|4h|rev|1|0`) | +1.77% |
| DMI cross | DMI(14) cross 60m, ADX>25, European hours, with trend (`DMI|60|14|25|eu|trend`) | +1.73% |
| US pre-market breakout | US pre-market breakout, retest entry, stop far, target 3R (`N10|retest|far|3R|none`) | +1.70% |
| Stochastic extreme | Stochastic(21) 30m <10/>90, European hours (`STO|30|21|10|2.0|4|eu`) | +1.39% |
| Candle-streak fade | Fade 6 same-colour 30m candles, Asian hours (`STK|30|6|4|2.0|asia`) | +0.85% |
| Trend-day follow | Trend-day follow at 1 h after the NYSE open (`B|US1h|0.5|trend|open`) | +0.77% |

## Top 100

| # | Strategy | Trades | October, 16 coins | 6 coins | 10 new coins | Coins up | Worst DD |
|---|---|---:|---:|---:|---:|---:|---:|
| 1 | Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|2.0|12|high`) | 276 | **+14.83%** ($23,734) | +8.08% | +18.89% | 16/16 | 4.65% |
| 2 | Bollinger(24,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|4.0|12|high`) | 275 | **+14.38%** ($23,007) | +7.87% | +18.29% | 16/16 | 5.08% |
| 3 | Bollinger(24,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.6|3.0|12|high`) | 275 | **+14.14%** ($22,623) | +7.27% | +18.26% | 16/16 | 5.67% |
| 4 | Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|4.0|12|high`) | 229 | **+13.67%** ($21,876) | +9.21% | +16.35% | 16/16 | 2.71% |
| 5 | Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|3.0|12|high`) | 229 | **+13.63%** ($21,803) | +9.46% | +16.13% | 16/16 | 2.58% |
| 6 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.8|2.0|12|high`) | 233 | **+13.61%** ($21,775) | +9.13% | +16.30% | 16/16 | 2.94% |
| 7 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|1.8|2.0|12|none`) | 274 | **+13.30%** ($21,273) | +6.71% | +17.25% | 16/16 | 3.26% |
| 8 | Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|1.8|3.0|12|none`) | 266 | **+13.12%** ($20,993) | +6.64% | +17.01% | 16/16 | 3.72% |
| 9 | Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|1.8|4.0|12|none`) | 266 | **+13.10%** ($20,964) | +6.21% | +17.24% | 16/16 | 4.15% |
| 10 | Bollinger(24,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|2.0|12|high`) | 240 | **+13.01%** ($20,819) | +7.71% | +16.19% | 16/16 | 4.49% |
| 11 | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.5|6|high`) | 237 | **+12.76%** ($20,409) | +9.25% | +14.86% | 15/16 | 3.49% |
| 12 | RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.5|6|high`) | 156 | **+12.74%** ($20,378) | +9.68% | +14.57% | 15/16 | 3.15% |
| 13 | RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|2.5|6|high`) | 156 | **+12.66%** ($20,256) | +9.67% | +14.45% | 15/16 | 3.15% |
| 14 | Bollinger(24,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|4.0|12|high`) | 238 | **+12.65%** ($20,242) | +7.72% | +15.61% | 15/16 | 4.08% |
| 15 | Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 12 h (`BZ|180|24|1.8|4.0|12|none`) | 315 | **+12.62%** ($20,198) | +7.21% | +15.87% | 15/16 | 3.46% |
| 16 | RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|2.5|6|high`) | 237 | **+12.52%** ($20,033) | +8.94% | +14.67% | 15/16 | 3.49% |
| 17 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|2.0|3.0|12|none`) | 202 | **+12.51%** ($20,024) | +9.36% | +14.41% | 16/16 | 2.50% |
| 18 | RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h (`RZ|30|28|30|55|2.5|6|none`) | 159 | **+12.51%** ($20,009) | +9.62% | +14.24% | 15/16 | 3.21% |
| 19 | RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h (`RZ|30|24|30|60|2.5|6|none`) | 243 | **+12.46%** ($19,939) | +9.11% | +14.47% | 15/16 | 3.89% |
| 20 | RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h (`RZ|30|28|30|60|2.5|6|none`) | 159 | **+12.43%** ($19,887) | +9.61% | +14.12% | 15/16 | 3.21% |
| 21 | Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|2.0|4.0|12|none`) | 202 | **+12.42%** ($19,871) | +9.10% | +14.41% | 16/16 | 2.75% |
| 22 | RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|2.5|6|high`) | 229 | **+12.42%** ($19,872) | +9.75% | +14.02% | 15/16 | 3.41% |
| 23 | Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|3.0|12|high`) | 239 | **+12.41%** ($19,861) | +7.14% | +15.58% | 14/16 | 5.22% |
| 24 | Bollinger(20,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|2.0|12|high`) | 283 | **+12.39%** ($19,821) | +6.50% | +15.92% | 15/16 | 4.30% |
| 25 | Bollinger(16,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|2.0|12|high`) | 285 | **+12.39%** ($19,825) | +7.45% | +15.36% | 14/16 | 3.48% |
| 26 | RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|1.5|6|high`) | 181 | **+12.37%** ($19,787) | +9.27% | +14.22% | 15/16 | 3.47% |
| 27 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h (`RZ|30|28|30|55|2.0|6|none`) | 168 | **+12.36%** ($19,769) | +9.77% | +13.91% | 15/16 | 2.85% |
| 28 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.0|6|high`) | 164 | **+12.35%** ($19,760) | +9.82% | +13.87% | 15/16 | 2.80% |
| 29 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|1.5|6|high`) | 181 | **+12.29%** ($19,664) | +9.26% | +14.11% | 15/16 | 3.47% |
| 30 | RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h (`RZ|30|28|30|60|2.0|6|none`) | 168 | **+12.28%** ($19,647) | +9.76% | +13.79% | 15/16 | 2.85% |
| 31 | RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|60|2.0|6|high`) | 164 | **+12.27%** ($19,638) | +9.81% | +13.75% | 15/16 | 2.80% |
| 32 | RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h (`RZ|30|28|30|55|1.5|6|none`) | 185 | **+12.26%** ($19,610) | +9.24% | +14.06% | 15/16 | 3.50% |
| 33 | Bollinger(24,1.8) 180m fade, stop 3.0 ATR, 12 h (`BZ|180|24|1.8|3.0|12|none`) | 318 | **+12.24%** ($19,591) | +6.89% | +15.46% | 15/16 | 3.57% |
| 34 | RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h (`RZ|30|24|30|55|2.5|6|none`) | 243 | **+12.23%** ($19,563) | +8.80% | +14.28% | 15/16 | 3.89% |
| 35 | Bollinger(20,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|4.0|12|high`) | 280 | **+12.19%** ($19,496) | +6.46% | +15.62% | 14/16 | 4.31% |
| 36 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|28|30|60|1.5|6|none`) | 185 | **+12.18%** ($19,488) | +9.23% | +13.95% | 15/16 | 3.50% |
| 37 | RSI(28) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|2.5|6|high`) | 156 | **+12.18%** ($19,484) | +9.22% | +13.95% | 15/16 | 3.15% |
| 38 | RSI(28) 30m fade <32/>68, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|2.5|6|high`) | 247 | **+12.14%** ($19,424) | +7.77% | +14.76% | 15/16 | 3.70% |
| 39 | Bollinger(16,2.0) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|2.0|2.0|12|none`) | 205 | **+12.13%** ($19,410) | +8.97% | +14.03% | 15/16 | 2.29% |
| 40 | Bollinger(16,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|4.0|12|high`) | 282 | **+12.12%** ($19,386) | +7.39% | +14.95% | 13/16 | 4.05% |
| 41 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.75|3.0|8|none`) | 222 | **+12.10%** ($19,352) | +6.57% | +15.41% | 15/16 | 5.04% |
| 42 | RSI(28) 30m fade <32/>68, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|2.5|6|high`) | 247 | **+12.08%** ($19,322) | +7.61% | +14.75% | 15/16 | 3.70% |
| 43 | RSI(18) 30m fade <26/>74, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|18|26|60|2.5|6|high`) | 242 | **+12.00%** ($19,193) | +9.13% | +13.72% | 15/16 | 3.31% |
| 44 | RSI(24) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|2.0|6|high`) | 250 | **+11.99%** ($19,184) | +9.63% | +13.41% | 15/16 | 3.77% |
| 45 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 3.0 ATR, 8 h (`KZ|240|20|1.5|3.0|8|none`) | 340 | **+11.98%** ($19,172) | +5.41% | +15.93% | 15/16 | 6.23% |
| 46 | RSI(24) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|60|1.5|6|high`) | 276 | **+11.97%** ($19,156) | +8.71% | +13.93% | 15/16 | 3.68% |
| 47 | RSI(21) 30m fade <28/>72, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|55|2.5|6|high`) | 229 | **+11.96%** ($19,143) | +9.53% | +13.42% | 15/16 | 3.41% |
| 48 | Bollinger(20,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.6|3.0|12|high`) | 281 | **+11.95%** ($19,115) | +5.87% | +15.59% | 15/16 | 5.14% |
| 49 | RSI(28) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h (`RZ|30|28|30|50|2.5|6|none`) | 159 | **+11.95%** ($19,116) | +9.15% | +13.62% | 15/16 | 3.21% |
| 50 | RSI(18) 30m fade <26/>74, exit 60, stop 2.5 ATR, 6 h (`RZ|30|18|26|60|2.5|6|none`) | 245 | **+11.94%** ($19,104) | +9.14% | +13.62% | 15/16 | 3.72% |
| 51 | RSI(24) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h (`RZ|30|24|30|60|2.0|6|none`) | 258 | **+11.92%** ($19,071) | +9.69% | +13.26% | 15/16 | 4.15% |
| 52 | RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h (`RZ|30|21|28|60|2.5|6|none`) | 231 | **+11.90%** ($19,035) | +9.49% | +13.34% | 15/16 | 3.66% |
| 53 | Bollinger(16,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|1.6|3.0|12|high`) | 282 | **+11.88%** ($19,014) | +7.18% | +14.71% | 13/16 | 4.31% |
| 54 | Bollinger(24,1.8) 180m fade, stop 2.0 ATR, 12 h (`BZ|180|24|1.8|2.0|12|none`) | 328 | **+11.88%** ($19,006) | +5.50% | +15.70% | 14/16 | 3.22% |
| 55 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|3.0|12|high`) | 183 | **+11.85%** ($18,961) | +9.19% | +13.45% | 15/16 | 2.46% |
| 56 | RSI(28) 30m fade <30/>70, exit 50, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|1.5|6|high`) | 181 | **+11.85%** ($18,953) | +8.90% | +13.61% | 15/16 | 3.47% |
| 57 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 2.0 ATR, 8 h (`KZ|240|20|1.5|2.0|8|none`) | 342 | **+11.84%** ($18,948) | +4.52% | +16.23% | 14/16 | 5.56% |
| 58 | RSI(28) 30m fade <32/>68, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|1.5|6|high`) | 286 | **+11.76%** ($18,818) | +7.71% | +14.19% | 15/16 | 4.01% |
| 59 | RSI(24) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|24|30|60|1.5|6|none`) | 284 | **+11.76%** ($18,822) | +8.80% | +13.54% | 15/16 | 4.03% |
| 60 | Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|4.0|12|high`) | 183 | **+11.75%** ($18,808) | +8.93% | +13.45% | 15/16 | 2.72% |
| 61 | RSI(24) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|2.0|6|high`) | 250 | **+11.75%** ($18,808) | +9.32% | +13.22% | 15/16 | 3.77% |
| 62 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|55|1.5|6|high`) | 276 | **+11.75%** ($18,797) | +8.40% | +13.76% | 15/16 | 3.68% |
| 63 | RSI(28) 30m fade <30/>70, exit 50, stop 1.5 ATR, 6 h (`RZ|30|28|30|50|1.5|6|none`) | 185 | **+11.74%** ($18,776) | +8.87% | +13.45% | 15/16 | 3.50% |
| 64 | RSI(28) 30m fade <32/>68, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|1.5|6|high`) | 286 | **+11.71%** ($18,732) | +7.56% | +14.20% | 15/16 | 4.01% |
| 65 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|50|2.0|6|high`) | 164 | **+11.70%** ($18,718) | +9.35% | +13.11% | 15/16 | 2.80% |
| 66 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 6 h (`RZ|30|28|30|50|2.0|6|none`) | 168 | **+11.70%** ($18,727) | +9.30% | +13.15% | 15/16 | 2.85% |
| 67 | RSI(21) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|2.0|6|high`) | 242 | **+11.69%** ($18,708) | +9.87% | +12.78% | 15/16 | 3.77% |
| 68 | RSI(24) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h (`RZ|30|24|30|55|2.0|6|none`) | 258 | **+11.68%** ($18,695) | +9.38% | +13.07% | 15/16 | 4.15% |
| 69 | RSI(24) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|30|50|2.5|6|high`) | 237 | **+11.67%** ($18,678) | +8.43% | +13.62% | 15/16 | 3.49% |
| 70 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 2.0 ATR, 8 h (`KZ|240|20|1.75|2.0|8|none`) | 223 | **+11.63%** ($18,612) | +5.40% | +15.37% | 15/16 | 4.79% |
| 71 | Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 8 h (`BZ|180|24|1.8|4.0|8|none`) | 343 | **+11.61%** ($18,574) | +5.70% | +15.15% | 16/16 | 3.94% |
| 72 | RSI(24) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h (`RZ|30|24|28|60|2.5|6|none`) | 161 | **+11.55%** ($18,486) | +9.54% | +12.76% | 15/16 | 3.12% |
| 73 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h (`RZ|30|24|30|55|1.5|6|none`) | 284 | **+11.54%** ($18,463) | +8.48% | +13.37% | 15/16 | 4.03% |
| 74 | RSI(21) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h (`RZ|30|21|28|60|2.0|6|none`) | 248 | **+11.52%** ($18,438) | +9.69% | +12.62% | 15/16 | 4.04% |
| 75 | RSI(28) 30m fade <32/>68, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|32|60|2.0|6|high`) | 261 | **+11.51%** ($18,419) | +8.29% | +13.45% | 15/16 | 3.53% |
| 76 | Bollinger(16,2.0) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.0|2.0|12|high`) | 186 | **+11.49%** ($18,383) | +8.86% | +13.07% | 15/16 | 2.38% |
| 77 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 3.0 ATR, 8 h, target EMA (`KZ|240|20|1.5|3.0|8|ema`) | 340 | **+11.49%** ($18,388) | +5.42% | +15.14% | 15/16 | 6.23% |
| 78 | RSI(24) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|28|60|2.5|6|high`) | 157 | **+11.48%** ($18,370) | +9.53% | +12.65% | 15/16 | 3.12% |
| 79 | Bollinger(30,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|30|1.6|2.0|12|high`) | 245 | **+11.47%** ($18,352) | +6.14% | +14.67% | 15/16 | 4.61% |
| 80 | RSI(24) 30m fade <28/>72, exit 55, stop 2.5 ATR, 6 h (`RZ|30|24|28|55|2.5|6|none`) | 161 | **+11.47%** ($18,357) | +9.45% | +12.68% | 15/16 | 3.12% |
| 81 | Keltner(EMA20, 1.75 ATR) fade on 240m, stop 3.0 ATR, 8 h, target EMA (`KZ|240|20|1.75|3.0|8|ema`) | 222 | **+11.47%** ($18,359) | +6.46% | +14.48% | 15/16 | 5.04% |
| 82 | RSI(21) 30m fade <28/>72, exit 60, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|21|28|60|1.5|6|high`) | 266 | **+11.46%** ($18,344) | +9.13% | +12.87% | 15/16 | 3.68% |
| 83 | RSI(28) 30m fade <32/>68, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|32|55|2.0|6|high`) | 261 | **+11.45%** ($18,317) | +8.13% | +13.44% | 15/16 | 3.53% |
| 84 | RSI(28) 30m fade <32/>68, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|28|32|50|2.5|6|high`) | 247 | **+11.45%** ($18,316) | +7.11% | +14.05% | 15/16 | 3.67% |
| 85 | RSI(21) 30m fade <28/>72, exit 55, stop 2.5 ATR, 6 h (`RZ|30|21|28|55|2.5|6|none`) | 231 | **+11.44%** ($18,306) | +9.28% | +12.74% | 15/16 | 3.66% |
| 86 | RSI(18) 30m fade <26/>74, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|18|26|55|2.5|6|high`) | 242 | **+11.44%** ($18,308) | +8.89% | +12.97% | 15/16 | 3.31% |
| 87 | RSI(16) 120m fade <34/>66, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|120|16|34|60|2.5|6|high`) | 256 | **+11.43%** ($18,291) | +4.62% | +15.52% | 16/16 | 6.30% |
| 88 | RSI(18) 30m fade <26/>74, exit 55, stop 2.5 ATR, 6 h (`RZ|30|18|26|55|2.5|6|none`) | 245 | **+11.40%** ($18,247) | +8.90% | +12.91% | 15/16 | 3.72% |
| 89 | RSI(24) 30m fade <28/>72, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|30|24|28|55|2.5|6|high`) | 157 | **+11.40%** ($18,240) | +9.45% | +12.57% | 15/16 | 3.12% |
| 90 | RSI(18) 30m fade <26/>74, exit 60, stop 2.0 ATR, 6 h (`RZ|30|18|26|60|2.0|6|none`) | 262 | **+11.39%** ($18,220) | +9.46% | +12.55% | 15/16 | 4.27% |
| 91 | RSI(24) 30m fade <28/>72, exit 60, stop 2.0 ATR, 6 h (`RZ|30|24|28|60|2.0|6|none`) | 171 | **+11.39%** ($18,227) | +9.89% | +12.30% | 15/16 | 2.79% |
| 92 | RSI(24) 30m fade <30/>70, exit 50, stop 2.5 ATR, 6 h (`RZ|30|24|30|50|2.5|6|none`) | 243 | **+11.38%** ($18,203) | +8.28% | +13.23% | 15/16 | 3.89% |
| 93 | Keltner 1.5 ATR fade on 240m, all day (`KEL|240|1.5|fade|2.0|8|all`) | 342 | **+11.35%** ($18,164) | +4.53% | +15.44% | 14/16 | 5.56% |
| 94 | RSI(16) 120m fade <34/>66, exit 55, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|120|16|34|55|2.5|6|high`) | 256 | **+11.35%** ($18,161) | +4.62% | +15.39% | 16/16 | 6.30% |
| 95 | Keltner(EMA20, 1.5 ATR) fade on 240m, stop 2.0 ATR, 8 h, target EMA (`KZ|240|20|1.5|2.0|8|ema`) | 342 | **+11.35%** ($18,164) | +4.53% | +15.44% | 14/16 | 5.56% |
| 96 | Bollinger(30,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|30|1.8|2.0|12|high`) | 202 | **+11.32%** ($18,118) | +5.12% | +15.05% | 15/16 | 5.09% |
| 97 | RSI(24) 30m fade <28/>72, exit 55, stop 2.0 ATR, 6 h (`RZ|30|24|28|55|2.0|6|none`) | 171 | **+11.31%** ($18,097) | +9.80% | +12.22% | 15/16 | 2.79% |
| 98 | RSI(18) 30m fade <26/>74, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|18|26|60|2.0|6|high`) | 257 | **+11.29%** ($18,063) | +9.45% | +12.39% | 15/16 | 3.89% |
| 99 | Bollinger(24,1.8) 180m fade, stop 3.0 ATR, 8 h (`BZ|180|24|1.8|3.0|8|none`) | 347 | **+11.28%** ($18,052) | +5.81% | +14.56% | 15/16 | 3.52% |
| 100 | RSI(16) 120m fade <34/>66, exit 60, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|120|16|34|60|2.0|6|high`) | 258 | **+11.28%** ($18,046) | +5.80% | +14.57% | 16/16 | 5.51% |

## Rules (top 30)

1. **Bollinger(24,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
2. **Bollinger(24,1.6) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
3. **Bollinger(24,1.6) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
4. **Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
5. **Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
6. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
7. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h.
8. **Bollinger(16,1.8) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
9. **Bollinger(16,1.8) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
10. **Bollinger(24,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
11. **RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
12. **RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop, high-volume bars only.
13. **RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
14. **Bollinger(24,1.8) 240m fade, stop 4.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h, high-volume bars only.
15. **Bollinger(24,1.8) 180m fade, stop 4.0 ATR, 12 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
16. **RSI(24) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop, high-volume bars only.
17. **Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
18. **RSI(28) 30m fade <30/>70, exit 55, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.5 ATR stop.
19. **RSI(24) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop.
20. **RSI(28) 30m fade <30/>70, exit 60, stop 2.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.5 ATR stop.
21. **Bollinger(16,2.0) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
22. **RSI(21) 30m fade <28/>72, exit 60, stop 2.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(21) under 28 (short over 72); exit at 60, after 6 h or at a 2.5 ATR stop, high-volume bars only.
23. **Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
24. **Bollinger(20,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
25. **Bollinger(16,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
26. **RSI(28) 30m fade <30/>70, exit 55, stop 1.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 1.5 ATR stop, high-volume bars only.
27. **RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.0 ATR stop.
28. **RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.0 ATR stop, high-volume bars only.
29. **RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 1.5 ATR stop, high-volume bars only.
30. **RSI(28) 30m fade <30/>70, exit 60, stop 2.0 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 2.0 ATR stop.

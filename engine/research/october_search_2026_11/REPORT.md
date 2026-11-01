# Profitable intraday crypto strategies, October 2026

Generated 2026-11-01 15:41 UTC. October 2026 only (no trading on the 28th). Delta India fees, slippage and funding; $10,000 per trade, no leverage, flat by 00:00 UTC.

- Tested 57,432 strategies over 119 rounds on BTC ETH SOL XRP DOGE BNB: 14,189 profitable, 3,662 consistent (both halves of October, 3+ of 6 coins, 10+ trades).
- **2,867 also made money on 10 other coins** (ADA LINK AVAX LTC DOT TRX BCH NEAR SUI APT; 5+ of 10).

## Best per family

| Family | Strategy | October return, 16 coins |
|---|---|---:|
| Bollinger fade | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|1.8|2.0|12|none`) | +13.30% |
| RSI fade | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.0|6|high`) | +12.35% |
| Keltner channel | Keltner 1.5 ATR fade on 240m, all day (`KEL|240|1.5|fade|2.0|8|all`) | +11.35% |
| Hour-of-day | Sell 15:00 UTC for 8 h (weekdays) (`TD2|15|8|short|none|wkday`) | +10.45% |
| RSI fade by hour and VWAP | RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP against, exit 50 (`CMB|18|12|against|30|50`) | +10.37% |
| 4h Bollinger + 1h RSI | 4h Bollinger(2.0) extreme + 1h RSI(21) 35 (`MTF|2.0|35|21|1.0|6`) | +9.42% |
| EMA z-score fade | EMA20 z-score 2.5 fade on 240m, all day (`ZEM|240|20|2.5|2.0|12|all`) | +7.81% |
| Day-of-week window | Friday 00:00 UTC for 6 h, long (`DOW|4|0|6|long`) | +7.68% |
| Pivot fade | Pivot fade at R1/S1, all day (`N7f|1|none|all`) | +5.53% |
| Pivot levels | Classic pivot fade at level 1, all day (`PIV|classic|1|fade|all`) | +5.53% |
| Williams %R | Williams %R(21) 240m beyond -95/-5, all day (`WPR|240|21|5|2.0|8|all`) | +5.12% |
| Sharp-move momentum/reversal | Fade a 3.0 ATR move over 6 x 60m (`ROC|60|6|3.0|4|rev|none`) | +4.53% |
| Session-open breakout | US-open 60-min breakout, stop entry, stop at far side, 3R target, with 10-day trend (`A|US|60|stop|far|3.0|trend`) | +4.05% |
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
| 1 | Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|16|1.8|2.0|12|none`) | 274 | **+13.30%** ($21,273) | +6.71% | +17.25% | 16/16 | 3.26% |
| 2 | Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|1.8|3.0|12|high`) | 239 | **+12.41%** ($19,861) | +7.14% | +15.58% | 14/16 | 5.22% |
| 3 | RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|30|28|30|55|2.0|6|high`) | 164 | **+12.35%** ($19,760) | +9.82% | +13.87% | 15/16 | 2.80% |
| 4 | RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h (`RZ|30|28|30|60|1.5|6|none`) | 185 | **+12.18%** ($19,488) | +9.23% | +13.95% | 15/16 | 3.50% |
| 5 | Keltner 1.5 ATR fade on 240m, all day (`KEL|240|1.5|fade|2.0|8|all`) | 342 | **+11.35%** ($18,164) | +4.53% | +15.44% | 14/16 | 5.56% |
| 6 | Bollinger(20,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|20|1.8|2.0|12|high`) | 230 | **+11.20%** ($17,923) | +7.53% | +13.40% | 14/16 | 3.63% |
| 7 | Bollinger(20,1.8) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|20|1.8|4.0|12|none`) | 273 | **+11.17%** ($17,866) | +4.59% | +15.11% | 14/16 | 4.79% |
| 8 | RSI(24) 30m fade <28/>72, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|24|28|55|1.5|6|high`) | 181 | **+11.04%** ($17,667) | +9.55% | +11.94% | 15/16 | 2.95% |
| 9 | Bollinger(24,2.0) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|24|2.0|2.0|12|none`) | 216 | **+10.94%** ($17,497) | +5.43% | +14.24% | 13/16 | 3.99% |
| 10 | Bollinger(16,1.6) 240m fade, stop 4.0 ATR, 12 h (`BZ|240|16|1.6|4.0|12|none`) | 349 | **+10.85%** ($17,358) | +2.63% | +15.78% | 12/16 | 4.68% |
| 11 | Bollinger(24,2.0) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|240|24|2.0|3.0|12|high`) | 182 | **+10.59%** ($16,946) | +7.68% | +12.34% | 14/16 | 3.33% |
| 12 | Bollinger(16,1.6) 240m fade, stop 3.0 ATR, 12 h (`BZ|240|16|1.6|3.0|12|none`) | 350 | **+10.59%** ($16,939) | +2.37% | +15.52% | 13/16 | 4.94% |
| 13 | Bollinger(24,1.8) 180m fade, stop 2.0 ATR, 8 h (`BZ|180|24|1.8|2.0|8|none`) | 353 | **+10.46%** ($16,739) | +4.18% | +14.23% | 13/16 | 3.19% |
| 14 | Sell 15:00 UTC for 8 h (weekdays) (`TD2|15|8|short|none|wkday`) | 336 | **+10.45%** ($16,726) | +9.30% | +11.15% | 15/16 | 4.05% |
| 15 | Sell 15:00 UTC for 8 h (weekdays) (`HZ|15|8|short|none|wkday`) | 336 | **+10.45%** ($16,726) | +9.30% | +11.15% | 15/16 | 4.05% |
| 16 | RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP against, exit 50 (`CMB|18|12|against|30|50`) | 92 | **+10.37%** ($16,589) | +9.51% | +10.88% | 16/16 | 0.80% |
| 17 | Bollinger(24,1.6) 180m fade, stop 4.0 ATR, 8 h (`BZ|180|24|1.6|4.0|8|none`) | 435 | **+10.35%** ($16,564) | +5.05% | +13.53% | 12/16 | 4.26% |
| 18 | RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP against, exit 60 (`CMB|18|12|against|30|60`) | 92 | **+10.31%** ($16,504) | +9.67% | +10.70% | 16/16 | 0.80% |
| 19 | Bollinger(24,1.8) 180m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|180|24|1.8|3.0|12|high`) | 278 | **+10.25%** ($16,400) | +6.67% | +12.40% | 14/16 | 3.61% |
| 20 | Bollinger(30,2.0) 180m fade (`BB2|180|30|2.0|3.0|12`) | 246 | **+10.22%** ($16,346) | +6.65% | +12.35% | 15/16 | 2.51% |
| 21 | Bollinger(20,1.8) 180m fade, stop 4.0 ATR, 4 h (`BZ|180|20|1.8|4.0|4|none`) | 405 | **+10.19%** ($16,298) | +4.75% | +13.45% | 16/16 | 2.55% |
| 22 | RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP any, exit 50 (`CMB|18|12|any|30|50`) | 109 | **+10.18%** ($16,287) | +8.62% | +11.11% | 16/16 | 2.15% |
| 23 | RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP any, exit 60 (`CMB|18|12|any|30|60`) | 109 | **+10.13%** ($16,202) | +8.79% | +10.93% | 16/16 | 2.15% |
| 24 | Bollinger(24,1.6) 180m fade, stop 3.0 ATR, 8 h (`BZ|180|24|1.6|3.0|8|none`) | 440 | **+10.13%** ($16,205) | +5.37% | +12.98% | 13/16 | 4.49% |
| 25 | Bollinger(40,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|40|1.6|2.0|12|high`) | 223 | **+10.04%** ($16,065) | +5.42% | +12.82% | 15/16 | 4.15% |
| 26 | 15:00 UTC for 6 h, sell (weekdays) (`TOD|15|6|short|wkday`) | 336 | **+9.97%** ($15,947) | +9.81% | +10.06% | 15/16 | 3.34% |
| 27 | Bollinger(30,2.0) 180m fade (`BB2|180|30|2.0|2.0|12`) | 251 | **+9.97%** ($15,951) | +5.24% | +12.81% | 14/16 | 2.85% |
| 28 | Sell 15:00 UTC for 6 h (weekdays) (`TD2|15|6|short|none|wkday`) | 336 | **+9.97%** ($15,947) | +9.81% | +10.06% | 15/16 | 3.34% |
| 29 | RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 4 h, high-volume bars only (`RZ|30|24|30|55|1.5|4|high`) | 276 | **+9.82%** ($15,712) | +7.29% | +11.34% | 15/16 | 3.65% |
| 30 | RSI(21) 30m fade <28/>72, exit 50, stop 1.5 ATR, 6 h (`RZ|30|21|28|50|1.5|6|none`) | 272 | **+9.78%** ($15,647) | +8.37% | +10.63% | 15/16 | 4.06% |
| 31 | Bollinger(20,1.6) 240m fade, stop 4.0 ATR, 8 h, high-volume bars only (`BZ|240|20|1.6|4.0|8|high`) | 296 | **+9.61%** ($15,369) | +4.04% | +12.95% | 15/16 | 5.10% |
| 32 | Bollinger(20,2.0) 240m fade, all day (`BBF|240|20|2.0|3.0|8|all`) | 223 | **+9.51%** ($15,217) | +6.00% | +11.62% | 15/16 | 2.98% |
| 33 | Bollinger(20,1.8) 180m fade, stop 2.0 ATR, 4 h (`BZ|180|20|1.8|2.0|4|none`) | 408 | **+9.51%** ($15,223) | +4.18% | +12.72% | 14/16 | 1.99% |
| 34 | RSI(28) 30m fade <30/>70, exit 50, stop 2.0 ATR, 4 h (`RZ|30|28|30|50|2.0|4|none`) | 170 | **+9.49%** ($15,186) | +7.57% | +10.64% | 14/16 | 2.88% |
| 35 | Bollinger(24,2.0) 240m fade, stop 3.0 ATR, 8 h (`BZ|240|24|2.0|3.0|8|none`) | 237 | **+9.48%** ($15,163) | +5.50% | +11.86% | 13/16 | 3.86% |
| 36 | Bollinger(24,2.0) 180m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|180|24|2.0|3.0|12|high`) | 226 | **+9.44%** ($15,108) | +7.01% | +10.90% | 15/16 | 3.71% |
| 37 | 4h Bollinger(2.0) extreme + 1h RSI(21) 35 (`MTF|2.0|35|21|1.0|6`) | 394 | **+9.42%** ($15,079) | +3.78% | +12.81% | 14/16 | 7.06% |
| 38 | RSI(21) 60m fade <30/>70, exit 60, all day (`RSI|60|21|30|60|3.0|6|none|all`) | 138 | **+9.41%** ($15,049) | +6.47% | +11.17% | 16/16 | 1.86% |
| 39 | Bollinger(30,1.5) 180m fade (`BB2|180|30|1.5|3.0|8`) | 506 | **+9.38%** ($15,009) | +1.60% | +14.05% | 12/16 | 6.73% |
| 40 | RSI(21) 60m fade <30/>70, exit 50, all day (`RSI|60|21|30|50|3.0|6|none|all`) | 138 | **+9.36%** ($14,972) | +6.30% | +11.19% | 16/16 | 1.86% |
| 41 | Bollinger(24,2.0) 180m fade, stop 4.0 ATR, 4 h (`BZ|180|24|2.0|4.0|4|none`) | 295 | **+9.36%** ($14,974) | +6.21% | +11.25% | 16/16 | 2.55% |
| 42 | 15:00 UTC for 4 h, sell (weekdays) (`TOD|15|4|short|wkday`) | 336 | **+9.34%** ($14,943) | +8.28% | +9.98% | 15/16 | 3.49% |
| 43 | Sell 15:00 UTC for 4 h (weekdays) (`TD2|15|4|short|none|wkday`) | 336 | **+9.34%** ($14,943) | +8.28% | +9.98% | 15/16 | 3.49% |
| 44 | Sell 15:00 UTC for 4 h (weekdays) (`HZ|15|4|short|none|wkday`) | 336 | **+9.34%** ($14,943) | +8.28% | +9.98% | 15/16 | 3.49% |
| 45 | 4h Bollinger(2.0) extreme + 1h RSI(14) 35 (`MTF|2.0|35|14|1.0|6`) | 540 | **+9.32%** ($14,913) | +3.79% | +12.64% | 13/16 | 7.72% |
| 46 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 8 h (`BZ|240|16|2.0|3.0|8|none`) | 215 | **+9.29%** ($14,859) | +7.19% | +10.54% | 15/16 | 2.67% |
| 47 | Bollinger(24,2.0) 180m fade, stop 3.0 ATR, 4 h (`BZ|180|24|2.0|3.0|4|none`) | 296 | **+9.29%** ($14,867) | +6.36% | +11.05% | 16/16 | 2.40% |
| 48 | Sell 15:00 UTC for 7 h (weekdays) (`TD2|15|7|short|none|wkday`) | 336 | **+9.28%** ($14,852) | +8.18% | +9.94% | 15/16 | 4.02% |
| 49 | Bollinger(24,1.6) 180m fade, stop 3.0 ATR, 12 h, high-volume bars only (`BZ|180|24|1.6|3.0|12|high`) | 318 | **+9.28%** ($14,841) | +4.33% | +12.25% | 15/16 | 5.04% |
| 50 | Bollinger(24,2.0) 240m fade, stop 4.0 ATR, 8 h, high-volume bars only (`BZ|240|24|2.0|4.0|8|high`) | 196 | **+9.21%** ($14,736) | +5.64% | +11.35% | 14/16 | 3.61% |
| 51 | Bollinger(24,2.0) 180m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|180|24|2.0|4.0|12|high`) | 225 | **+9.20%** ($14,715) | +6.57% | +10.78% | 15/16 | 4.15% |
| 52 | RSI(21) 60m fade <30/>70, exit 50, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|60|21|30|50|2.0|6|high`) | 145 | **+9.10%** ($14,560) | +5.47% | +11.28% | 15/16 | 2.63% |
| 53 | Bollinger(24,2.2) 240m fade, stop 2.0 ATR, 12 h (`BZ|240|24|2.2|2.0|12|none`) | 165 | **+9.06%** ($14,500) | +6.30% | +10.72% | 15/16 | 2.48% |
| 54 | Bollinger(24,2.2) 180m fade, stop 3.0 ATR, 12 h (`BZ|180|24|2.2|3.0|12|none`) | 188 | **+9.05%** ($14,486) | +6.57% | +10.54% | 16/16 | 2.30% |
| 55 | Bollinger(16,2.0) 240m fade, stop 3.0 ATR, 8 h, high-volume bars only (`BZ|240|16|2.0|3.0|8|high`) | 193 | **+9.01%** ($14,413) | +6.93% | +10.26% | 15/16 | 2.67% |
| 56 | RSI(24) 60m fade <34/>66, exit 50, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|60|24|34|50|1.5|6|high`) | 264 | **+8.90%** ($14,237) | +4.89% | +11.30% | 15/16 | 4.61% |
| 57 | RSI(18) 60m fade <28/>72, exit 50, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|60|18|28|50|2.5|6|high`) | 139 | **+8.83%** ($14,125) | +6.15% | +10.43% | 16/16 | 1.74% |
| 58 | Bollinger(20,2.0) 240m fade, all day (`BBF|240|20|2.0|2.0|8|all`) | 226 | **+8.80%** ($14,088) | +4.91% | +11.14% | 15/16 | 2.73% |
| 59 | Bollinger(20,2.0) 180m fade (`BB2|180|20|2.0|3.0|12`) | 250 | **+8.77%** ($14,030) | +4.59% | +11.28% | 15/16 | 3.09% |
| 60 | Sell 15:00 UTC for 6 h, only with the 10-day trend (weekdays) (`TD2|15|6|short|trend10|wkday`) | 252 | **+8.76%** ($14,009) | +8.11% | +9.14% | 15/16 | 3.43% |
| 61 | Bollinger(24,2.0) 180m fade, stop 3.0 ATR, 4 h, high-volume bars only (`BZ|180|24|2.0|3.0|4|high`) | 259 | **+8.76%** ($14,022) | +6.66% | +10.03% | 16/16 | 1.79% |
| 62 | RSI(28) 60m fade <34/>66, exit 50, stop 2.0 ATR, 6 h, high-volume bars only (`RZ|60|28|34|50|2.0|6|high`) | 157 | **+8.76%** ($14,023) | +5.96% | +10.45% | 15/16 | 3.25% |
| 63 | 4h Bollinger(2.0) extreme + 1h RSI(14) 30 (`MTF|2.0|30|14|1.0|6`) | 346 | **+8.73%** ($13,967) | +5.23% | +10.83% | 16/16 | 5.50% |
| 64 | Bollinger(30,1.8) 120m fade, stop 4.0 ATR, 8 h, high-volume bars only (`BZ|120|30|1.8|4.0|8|high`) | 369 | **+8.72%** ($13,960) | +7.27% | +9.60% | 16/16 | 5.41% |
| 65 | RSI(21) 60m fade <30/>70, exit 60, all day (`RSI|60|21|30|60|2.0|6|none|all`) | 155 | **+8.71%** ($13,937) | +5.54% | +10.61% | 15/16 | 2.98% |
| 66 | Sell 15:00 UTC for 8 h, only with the 10-day trend (weekdays) (`TD2|15|8|short|trend10|wkday`) | 252 | **+8.68%** ($13,881) | +8.01% | +9.08% | 15/16 | 3.93% |
| 67 | Bollinger(24,1.8) 180m fade, stop 2.0 ATR, 4 h, high-volume bars only (`BZ|180|24|1.8|2.0|4|high`) | 331 | **+8.67%** ($13,876) | +5.24% | +10.73% | 14/16 | 1.49% |
| 68 | RSI(21) 60m fade <30/>70, exit 60, all day (`RSI|60|21|30|60|1.5|6|none|all`) | 166 | **+8.66%** ($13,864) | +5.96% | +10.29% | 15/16 | 3.07% |
| 69 | RSI(21) 60m fade <30/>70, exit 50, all day (`RSI|60|21|30|50|2.0|6|none|all`) | 155 | **+8.66%** ($13,860) | +5.38% | +10.63% | 15/16 | 2.98% |
| 70 | RSI(24) 60m fade <32/>68, exit 55, stop 2.0 ATR, 6 h (`RZ|60|24|32|55|2.0|6|none`) | 165 | **+8.66%** ($13,861) | +6.73% | +9.83% | 14/16 | 3.46% |
| 71 | Sell 15:00 UTC for 7 h, only after a busy hour (weekdays) (`HZ|15|7|short|highvol|wkday`) | 256 | **+8.65%** ($13,836) | +7.74% | +9.19% | 14/16 | 3.60% |
| 72 | Bollinger(20,1.8) 240m fade, stop 2.0 ATR, 8 h, high-volume bars only (`BZ|240|20|1.8|2.0|8|high`) | 243 | **+8.65%** ($13,833) | +5.19% | +10.72% | 14/16 | 3.63% |
| 73 | RSI(21) 60m fade <30/>70, exit 50, all day (`RSI|60|21|30|50|1.5|6|none|all`) | 166 | **+8.62%** ($13,787) | +5.80% | +10.31% | 15/16 | 3.07% |
| 74 | Bollinger(20,2.0) 180m fade (`BB2|180|20|2.0|2.0|12`) | 258 | **+8.46%** ($13,537) | +3.46% | +11.46% | 14/16 | 2.63% |
| 75 | Sell 15:00 UTC for 4 h, only with the 10-day trend (weekdays) (`TD2|15|4|short|trend10|wkday`) | 252 | **+8.46%** ($13,541) | +8.96% | +8.16% | 15/16 | 3.18% |
| 76 | RSI(18) 60m fade <32/>68, exit 60, stop 2.5 ATR, 6 h, high-volume bars only (`RZ|60|18|32|60|2.5|6|high`) | 297 | **+8.46%** ($13,539) | +5.90% | +10.00% | 16/16 | 3.98% |
| 77 | 02:00 UTC for 6 h, buy if above VWAP (weekdays) (`TOD|2|6|long_above|wkday`) | 137 | **+8.40%** ($13,439) | +4.37% | +10.82% | 15/16 | 0.93% |
| 78 | Bollinger(30,2.0) 180m fade (`BB2|180|30|2.0|3.0|8`) | 264 | **+8.40%** ($13,432) | +5.27% | +10.27% | 14/16 | 2.97% |
| 79 | Bollinger(20,2.0) 180m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|180|20|2.0|4.0|12|high`) | 234 | **+8.39%** ($13,431) | +4.83% | +10.53% | 15/16 | 3.26% |
| 80 | Bollinger(30,1.6) 180m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|180|30|1.6|2.0|12|high`) | 322 | **+8.38%** ($13,410) | +2.68% | +11.80% | 14/16 | 4.96% |
| 81 | RSI(24) 30m fade <30/>70, exit 50, stop 2.5 ATR, 8 h, high-volume bars only (`RZ|30|24|30|50|2.5|8|high`) | 233 | **+8.36%** ($13,377) | +5.94% | +9.81% | 15/16 | 3.85% |
| 82 | Sell 12:00 UTC for 7 h, only against the last hour's RSI (weekdays) (`TD2|12|7|short|rsi1h|wkday`) | 147 | **+8.34%** ($13,350) | +7.22% | +9.02% | 15/16 | 2.38% |
| 83 | RSI(21) 60m fade <30/>70, exit 50, all day (`RSI|60|21|30|50|3.0|12|none|all`) | 127 | **+8.32%** ($13,313) | +3.83% | +11.01% | 15/16 | 3.16% |
| 84 | Bollinger(20,1.6) 180m fade, stop 4.0 ATR, 4 h (`BZ|180|20|1.6|4.0|4|none`) | 563 | **+8.30%** ($13,287) | +2.69% | +11.67% | 12/16 | 3.22% |
| 85 | 4h Bollinger(2.0) extreme + 1h RSI(21) 35 (`MTF|2.0|35|21|2.0|6`) | 324 | **+8.29%** ($13,268) | +3.41% | +11.22% | 13/16 | 5.90% |
| 86 | RSI(24) 30m fade <28/>72, exit 50, stop 2.5 ATR, 8 h (`RZ|30|24|28|50|2.5|8|none`) | 160 | **+8.24%** ($13,185) | +6.29% | +9.41% | 13/16 | 3.57% |
| 87 | Bollinger(20,1.8) 180m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|180|20|1.8|2.0|12|high`) | 297 | **+8.24%** ($13,188) | +4.11% | +10.72% | 14/16 | 3.61% |
| 88 | RSI(21) 60m fade <30/>70, exit 50, all day (`RSI|60|21|30|50|1.0|6|none|all`) | 191 | **+8.22%** ($13,159) | +4.32% | +10.57% | 14/16 | 4.42% |
| 89 | Bollinger(16,2.2) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|16|2.2|2.0|12|high`) | 143 | **+8.21%** ($13,135) | +7.18% | +8.83% | 15/16 | 2.41% |
| 90 | Bollinger(30,2.0) 120m fade, stop 4.0 ATR, 12 h, high-volume bars only (`BZ|120|30|2.0|4.0|12|high`) | 267 | **+8.20%** ($13,119) | +4.45% | +10.45% | 15/16 | 3.89% |
| 91 | RSI(21) 60m fade <30/>70, exit 60, all day (`RSI|60|21|30|60|1.0|6|none|all`) | 191 | **+8.19%** ($13,101) | +4.48% | +10.41% | 14/16 | 4.42% |
| 92 | Bollinger(30,2.0) 180m fade (`BB2|180|30|2.0|3.0|4`) | 312 | **+8.13%** ($13,008) | +4.56% | +10.27% | 14/16 | 1.99% |
| 93 | 4h Bollinger(1.5) extreme + 1h RSI(14) 30 (`MTF|1.5|30|14|1.0|6`) | 424 | **+8.12%** ($12,990) | +3.73% | +10.75% | 15/16 | 6.14% |
| 94 | RSI(21) 30m fade <30/>70, exit 60, Asian hours (`RSI|30|21|30|60|1.0|6|none|asia`) | 130 | **+8.09%** ($12,938) | +6.94% | +8.77% | 15/16 | 2.87% |
| 95 | Sell 15:00 UTC for 7 h (every day) (`TD2|15|7|short|none|all`) | 480 | **+8.08%** ($12,934) | +6.36% | +9.12% | 14/16 | 4.64% |
| 96 | Bollinger(24,2.2) 240m fade, stop 3.0 ATR, 8 h (`BZ|240|24|2.2|3.0|8|none`) | 174 | **+8.07%** ($12,913) | +5.69% | +9.50% | 14/16 | 2.77% |
| 97 | 4h Bollinger(1.5) extreme + 1h RSI(21) 30 (`MTF|1.5|30|21|2.0|6`) | 146 | **+8.05%** ($12,886) | +5.18% | +9.78% | 15/16 | 3.20% |
| 98 | RSI(16) 30m fade <26/>74, exit 60, stop 2.0 ATR, 4 h, high-volume bars only (`RZ|30|16|26|60|2.0|4|high`) | 333 | **+8.04%** ($12,872) | +7.03% | +8.66% | 15/16 | 5.08% |
| 99 | Bollinger(24,2.2) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only (`BZ|240|24|2.2|2.0|12|high`) | 145 | **+8.01%** ($12,816) | +7.07% | +8.58% | 13/16 | 2.10% |
| 100 | RSI(16) 30m fade <26/>74, exit 55, stop 1.5 ATR, 6 h, high-volume bars only (`RZ|30|16|26|55|1.5|6|high`) | 368 | **+8.00%** ($12,795) | +5.91% | +9.25% | 15/16 | 5.78% |

## Rules (top 30)

1. **Bollinger(16,1.8) 240m fade, stop 2.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h.
2. **Bollinger(24,1.8) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
3. **RSI(28) 30m fade <30/>70, exit 55, stop 2.0 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 55, after 6 h or at a 2.0 ATR stop, high-volume bars only.
4. **RSI(28) 30m fade <30/>70, exit 60, stop 1.5 ATR, 6 h**: On 30-min bars buy RSI(28) under 30 (short over 70); exit at 60, after 6 h or at a 1.5 ATR stop.
5. **Keltner 1.5 ATR fade on 240m, all day**: Fade a 240-min close beyond EMA20 +/- 1.5 ATR; stop 2.0 ATR, up to 8 h, target EMA20.
6. **Bollinger(20,1.8) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
7. **Bollinger(20,1.8) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
8. **RSI(24) 30m fade <28/>72, exit 55, stop 1.5 ATR, 6 h, high-volume bars only**: On 30-min bars buy RSI(24) under 28 (short over 72); exit at 55, after 6 h or at a 1.5 ATR stop, high-volume bars only.
9. **Bollinger(24,2.0) 240m fade, stop 2.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h.
10. **Bollinger(16,1.6) 240m fade, stop 4.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 12 h.
11. **Bollinger(24,2.0) 240m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
12. **Bollinger(16,1.6) 240m fade, stop 3.0 ATR, 12 h**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
13. **Bollinger(24,1.8) 180m fade, stop 2.0 ATR, 8 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 8 h.
14. **Sell 15:00 UTC for 8 h (weekdays)**: Weekdays at 15:00 UTC sell; close 8 h later.
15. **Sell 15:00 UTC for 8 h (weekdays)**: Weekdays at 15:00 UTC sell; close 8 h later.
16. **RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP against, exit 50**: Between 18:00 and 06:00 UTC, fade 1h RSI(21) under 30 / over 70 only against price's side of VWAP; exit at RSI 50, 6 h or a 2 ATR stop.
17. **Bollinger(24,1.6) 180m fade, stop 4.0 ATR, 8 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 8 h.
18. **RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP against, exit 60**: Between 18:00 and 06:00 UTC, fade 1h RSI(21) under 30 / over 70 only against price's side of VWAP; exit at RSI 60, 6 h or a 2 ATR stop.
19. **Bollinger(24,1.8) 180m fade, stop 3.0 ATR, 12 h, high-volume bars only**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h, high-volume bars only.
20. **Bollinger(30,2.0) 180m fade**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 12 h.
21. **Bollinger(20,1.8) 180m fade, stop 4.0 ATR, 4 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 4.0 ATR, up to 4 h.
22. **RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP any, exit 50**: Between 18:00 and 06:00 UTC, fade 1h RSI(21) under 30 / over 70; exit at RSI 50, 6 h or a 2 ATR stop.
23. **RSI(21) 1h fade 30/70 from 18:00 UTC for 12 h, VWAP any, exit 60**: Between 18:00 and 06:00 UTC, fade 1h RSI(21) under 30 / over 70; exit at RSI 60, 6 h or a 2 ATR stop.
24. **Bollinger(24,1.6) 180m fade, stop 3.0 ATR, 8 h**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 3.0 ATR, up to 8 h.
25. **Bollinger(40,1.6) 240m fade, stop 2.0 ATR, 12 h, high-volume bars only**: Buy a 240-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h, high-volume bars only.
26. **15:00 UTC for 6 h, sell (weekdays)**: Weekdays at 15:00 UTC sell; close 6 h later.
27. **Bollinger(30,2.0) 180m fade**: Buy a 180-min close under the lower band (short over the upper); target the middle band, stop 2.0 ATR, up to 12 h.
28. **Sell 15:00 UTC for 6 h (weekdays)**: Weekdays at 15:00 UTC sell; close 6 h later.
29. **RSI(24) 30m fade <30/>70, exit 55, stop 1.5 ATR, 4 h, high-volume bars only**: On 30-min bars buy RSI(24) under 30 (short over 70); exit at 55, after 4 h or at a 1.5 ATR stop, high-volume bars only.
30. **RSI(21) 30m fade <28/>72, exit 50, stop 1.5 ATR, 6 h**: On 30-min bars buy RSI(21) under 28 (short over 72); exit at 50, after 6 h or at a 1.5 ATR stop.

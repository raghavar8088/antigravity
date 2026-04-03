package strategy

// =============================================================================
// INTRADAY STRATEGIES — 65 strategies optimised for 5m/15m candles.
// Uses generic structs from elite_v2.go with wider periods and SL/TP geometry
// appropriate for longer timeframes (SL 0.22-0.45%, TP 0.55-1.10%, R:R ≥ 2).
// =============================================================================

// ── Intraday EMA Cross (10) ───────────────────────────────────────────────────
func NewID_EMA5_20_5m()   *EMACrossV2 { return newEMACrossV2("ID_EMA5_20_5m",    5, 20, 20, 44, 72, 0.22, 0.55) }
func NewID_EMA8_21_5m()   *EMACrossV2 { return newEMACrossV2("ID_EMA8_21_5m",    8, 21, 22, 44, 72, 0.24, 0.58) }
func NewID_EMA9_50_5m()   *EMACrossV2 { return newEMACrossV2("ID_EMA9_50_5m",    9, 50, 24, 44, 72, 0.26, 0.62) }
func NewID_EMA12_26_5m()  *EMACrossV2 { return newEMACrossV2("ID_EMA12_26_5m",  12, 26, 22, 44, 72, 0.24, 0.58) }
func NewID_EMA20_50_5m()  *EMACrossV2 { return newEMACrossV2("ID_EMA20_50_5m",  20, 50, 25, 43, 73, 0.28, 0.66) }
func NewID_EMA21_55_5m()  *EMACrossV2 { return newEMACrossV2("ID_EMA21_55_5m",  21, 55, 25, 43, 73, 0.28, 0.68) }
func NewID_EMA50_200_5m() *EMACrossV2 { return newEMACrossV2("ID_EMA50_200_5m", 50,200, 28, 42, 74, 0.35, 0.85) }
func NewID_EMA20_100_5m() *EMACrossV2 { return newEMACrossV2("ID_EMA20_100_5m", 20,100, 26, 43, 73, 0.30, 0.72) }
func NewID_EMA10_30_15m() *EMACrossV2 { return newEMACrossV2("ID_EMA10_30_15m", 10, 30, 22, 44, 72, 0.28, 0.68) }
func NewID_EMA13_34_15m() *EMACrossV2 { return newEMACrossV2("ID_EMA13_34_15m", 13, 34, 22, 44, 72, 0.30, 0.72) }

// ── Intraday Triple EMA (8) ───────────────────────────────────────────────────
func NewID_Triple5_13_34_5m()    *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple5_13_34_5m",     5, 13, 34, 22, 44, 72, 0.24, 0.60) }
func NewID_Triple8_21_55_5m()    *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple8_21_55_5m",     8, 21, 55, 24, 43, 73, 0.26, 0.64) }
func NewID_Triple10_30_60_5m()   *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple10_30_60_5m",   10, 30, 60, 25, 43, 73, 0.28, 0.68) }
func NewID_Triple20_50_100_5m()  *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple20_50_100_5m",  20, 50,100, 28, 42, 74, 0.32, 0.78) }
func NewID_Triple5_10_20_5m()    *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple5_10_20_5m",     5, 10, 20, 20, 44, 72, 0.22, 0.56) }
func NewID_Triple9_18_36_5m()    *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple9_18_36_5m",     9, 18, 36, 22, 43, 73, 0.24, 0.60) }
func NewID_Triple10_20_40_15m()  *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple10_20_40_15m",  10, 20, 40, 22, 44, 72, 0.28, 0.68) }
func NewID_Triple20_50_200_15m() *TripleEMAScalperV2 { return newTripleEMAV2("ID_Triple20_50_200_15m", 20, 50,200, 28, 42, 74, 0.38, 0.92) }

// ── Intraday MACD (8) ─────────────────────────────────────────────────────────
func NewID_MACDCross12_26_9_5m()   *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Cross12_26_9_5m",   "cross",        12, 26,  9, 22, 44, 72, 0.26, 0.64) }
func NewID_MACDZero12_26_5m()      *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Zero12_26_5m",      "zero_cross",   12, 26,  9, 22, 44, 72, 0.26, 0.64) }
func NewID_MACDHistMom12_26_5m()   *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Hist12_26_5m",      "hist_momentum",12, 26,  9, 22, 46, 70, 0.24, 0.58) }
func NewID_MACDCross26_52_9_5m()   *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Cross26_52_9_5m",   "cross",        26, 52,  9, 25, 44, 72, 0.30, 0.72) }
func NewID_MACDCross5_35_5_5m()    *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Cross5_35_5_5m",    "cross",         5, 35,  5, 20, 44, 72, 0.24, 0.60) }
func NewID_MACDCross12_26_9_15m()  *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Cross12_26_9_15m",  "cross",        12, 26,  9, 22, 44, 72, 0.32, 0.78) }
func NewID_MACDZero12_26_15m()     *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Zero12_26_15m",     "zero_cross",   12, 26,  9, 22, 44, 72, 0.32, 0.78) }
func NewID_MACDHistMom5_13_5m()    *MACDSignalScalperV2 { return newMACDScalperV2("ID_MACD_Hist5_13_5m",       "hist_momentum", 5, 13,  3, 18, 46, 70, 0.22, 0.56) }

// ── Intraday VWAP (6) ─────────────────────────────────────────────────────────
func NewID_VWAPCross50_5m()     *VWAPSignalScalper { return newVWAPScalper("ID_VWAP_Cross50_5m",     "cross",     50, 0,    22, 44, 72, 0.24, 0.60) }
func NewID_VWAPCross100_5m()    *VWAPSignalScalper { return newVWAPScalper("ID_VWAP_Cross100_5m",    "cross",    100, 0,    25, 43, 73, 0.28, 0.68) }
func NewID_VWAPDev0p5_5m()      *VWAPSignalScalper { return newVWAPScalper("ID_VWAP_Dev0p5_5m",      "deviation", 50, 0.5,  18, 30, 54, 0.26, 0.64) }
func NewID_VWAPDev1p0_5m()      *VWAPSignalScalper { return newVWAPScalper("ID_VWAP_Dev1p0_5m",      "deviation", 50, 1.0,  20, 28, 52, 0.30, 0.74) }
func NewID_VWAPPullback50_5m()  *VWAPSignalScalper { return newVWAPScalper("ID_VWAP_Pullback50_5m",  "pullback",  50, 0,    22, 44, 66, 0.24, 0.60) }
func NewID_VWAPPullback100_5m() *VWAPSignalScalper { return newVWAPScalper("ID_VWAP_Pullback100_5m", "pullback", 100, 0,    25, 43, 66, 0.28, 0.68) }

// ── Intraday Bollinger Bands (8) ──────────────────────────────────────────────
func NewID_BBBounce20_2_5m()     *BBSignalScalper { return newBBScalper("ID_BB_Bounce20_2_5m",     "bounce_lower", 20, 2.0, 0, 18, 28, 50, 0.24, 0.58) }
func NewID_BBBounce50_2_5m()     *BBSignalScalper { return newBBScalper("ID_BB_Bounce50_2_5m",     "bounce_lower", 50, 2.0, 0, 22, 26, 50, 0.28, 0.68) }
func NewID_BBMidCross20_5m()     *BBSignalScalper { return newBBScalper("ID_BB_MidCross20_5m",     "mid_cross",    20, 2.0,20, 0, 44, 72, 0.24, 0.58) }
func NewID_BBMidCross50_5m()     *BBSignalScalper { return newBBScalper("ID_BB_MidCross50_5m",     "mid_cross",    50, 2.0,22, 0, 44, 72, 0.28, 0.68) }
func NewID_BBBreakout20_2_5m()   *BBSignalScalper { return newBBScalper("ID_BB_Break20_2_5m",      "breakout",     20, 2.0,22, 0, 52, 75, 0.26, 0.64) }
func NewID_BBBreakout50_2_5m()   *BBSignalScalper { return newBBScalper("ID_BB_Break50_2_5m",      "breakout",     50, 2.0,25, 0, 52, 75, 0.30, 0.74) }
func NewID_BBWidth20_2_5m()      *BBWidthScalper  { return newBBWidth("ID_BB_Width20_2_5m",         20, 2.0, 20, 43, 73, 0.24, 0.58) }
func NewID_BBWidth50_2_5m()      *BBWidthScalper  { return newBBWidth("ID_BB_Width50_2_5m",         50, 2.0, 22, 43, 73, 0.28, 0.68) }

// ── Intraday RSI (5) ──────────────────────────────────────────────────────────
func NewID_RSIOversold30_5m() *RSIThresholdScalper { return newRSIThreshold("ID_RSI_Oversold30_5m", 14, 32, 38, 18, 0.24, 0.58) }
func NewID_RSICross50_5m()    *RSIThresholdScalper { return newRSIThreshold("ID_RSI_Cross50_5m",    14, 48, 52, 20, 0.24, 0.58) }
func NewID_RSICross55_5m()    *RSIThresholdScalper { return newRSIThreshold("ID_RSI_Cross55_5m",    14, 53, 57, 22, 0.26, 0.62) }
func NewID_RSI21_5m()         *RSIThresholdScalper { return newRSIThreshold("ID_RSI21_5m",          21, 45, 52, 20, 0.28, 0.66) }
func NewID_RSIOversold20_5m() *RSIThresholdScalper { return newRSIThreshold("ID_RSI_Oversold20_5m", 14, 22, 28, 15, 0.26, 0.64) }

// ── Intraday Keltner (5) ──────────────────────────────────────────────────────
func NewID_KeltBreak20_2_5m()    *KeltnerScalperV2 { return newKeltnerV2("ID_Kelt_Break20_2_5m",    "break",   20, 14, 2.0, 22, 52, 74, 0.26, 0.64) }
func NewID_KeltBreak50_2_5m()    *KeltnerScalperV2 { return newKeltnerV2("ID_Kelt_Break50_2_5m",    "break",   50, 14, 2.0, 25, 52, 74, 0.30, 0.74) }
func NewID_KeltBounce20_2_5m()   *KeltnerScalperV2 { return newKeltnerV2("ID_Kelt_Bounce20_2_5m",   "bounce",  20, 14, 2.0,  0, 26, 74, 0.26, 0.64) }
func NewID_KeltMid20_5m()        *KeltnerScalperV2 { return newKeltnerV2("ID_Kelt_Mid20_5m",        "midline", 20, 14, 2.0, 20, 44, 72, 0.24, 0.58) }
func NewID_KeltMid50_5m()        *KeltnerScalperV2 { return newKeltnerV2("ID_Kelt_Mid50_5m",        "midline", 50, 14, 2.0, 22, 44, 72, 0.28, 0.68) }

// ── Intraday Stochastic (5) ───────────────────────────────────────────────────
func NewID_StochCross14_3_5m()  *StochSignalScalper { return newStochScalper("ID_Stoch_Cross14_3_5m",   "cross",    14,  3, 20, 44, 68, 0.24, 0.58) }
func NewID_StochCross14_5_5m()  *StochSignalScalper { return newStochScalper("ID_Stoch_Cross14_5_5m",   "cross",    14,  5, 22, 44, 68, 0.24, 0.58) }
func NewID_StochOversold14_5m() *StochSignalScalper { return newStochScalper("ID_Stoch_Oversold14_5m",  "oversold", 14,  3,  0, 28, 74, 0.26, 0.64) }
func NewID_StochTrend14_5m()    *StochSignalScalper { return newStochScalper("ID_Stoch_Trend14_5m",     "trend",    14,  3, 22, 47, 73, 0.24, 0.58) }
func NewID_StochCross21_5_5m()  *StochSignalScalper { return newStochScalper("ID_Stoch_Cross21_5_5m",   "cross",    21,  5, 22, 44, 68, 0.26, 0.62) }

// ── Intraday Hull MA (5) ──────────────────────────────────────────────────────
func NewID_HullMA20_5m()  *HullMAScalperV2 { return newHullMAV2("ID_HullMA20_5m",  20, 22, 44, 72, 0.24, 0.58) }
func NewID_HullMA30_5m()  *HullMAScalperV2 { return newHullMAV2("ID_HullMA30_5m",  30, 22, 44, 72, 0.26, 0.62) }
func NewID_HullMA50_5m()  *HullMAScalperV2 { return newHullMAV2("ID_HullMA50_5m",  50, 25, 44, 72, 0.28, 0.68) }
func NewID_HullMA100_5m() *HullMAScalperV2 { return newHullMAV2("ID_HullMA100_5m",100, 28, 43, 73, 0.32, 0.78) }
func NewID_HullMA14_5m()  *HullMAScalperV2 { return newHullMAV2("ID_HullMA14_5m",  14, 20, 44, 72, 0.22, 0.56) }

// ── Intraday CCI (5) ──────────────────────────────────────────────────────────
func NewID_CCIZeroCross20_5m()  *CCISignalScalper { return newCCIScalper("ID_CCI_Zero20_5m",  "zero_cross",     20, 20, 44, 72, 0.24, 0.58) }
func NewID_CCIExtreme20_5m()    *CCISignalScalper { return newCCIScalper("ID_CCI_Extreme20_5m","extreme_bounce", 20, 18, 28, 74, 0.26, 0.64) }
func NewID_CCITrend20_5m()      *CCISignalScalper { return newCCIScalper("ID_CCI_Trend20_5m",  "trend",          20, 25, 50, 75, 0.24, 0.60) }
func NewID_CCIZeroCross50_5m()  *CCISignalScalper { return newCCIScalper("ID_CCI_Zero50_5m",  "zero_cross",     50, 22, 44, 72, 0.28, 0.68) }
func NewID_CCITrend50_5m()      *CCISignalScalper { return newCCIScalper("ID_CCI_Trend50_5m",  "trend",          50, 25, 50, 75, 0.28, 0.68) }

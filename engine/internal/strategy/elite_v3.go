package strategy

// =============================================================================
// ELITE STRATEGIES V3 — 105 additional strategies.
// Uses generic structs defined in elite_v2.go.
// All strategies carry ADX + RSI guards and R:R ≥ 1.5 geometry.
// =============================================================================

// ── Stochastic family (12) ────────────────────────────────────────────────────
func NewStochCross5_3_Scalp()   *StochSignalScalper { return newStochScalper("Stoch_Cross5_3_Scalp",    "cross",    5,  3, 18, 45, 68, 0.16, 0.38) }
func NewStochCross9_3_Scalp()   *StochSignalScalper { return newStochScalper("Stoch_Cross9_3_Scalp",    "cross",    9,  3, 18, 45, 68, 0.17, 0.40) }
func NewStochCross14_3_Scalp()  *StochSignalScalper { return newStochScalper("Stoch_Cross14_3_Scalp",   "cross",   14,  3, 18, 45, 68, 0.18, 0.42) }
func NewStochCross14_5_Scalp()  *StochSignalScalper { return newStochScalper("Stoch_Cross14_5_Scalp",   "cross",   14,  5, 20, 45, 68, 0.18, 0.42) }
func NewStochCross21_3_Scalp()  *StochSignalScalper { return newStochScalper("Stoch_Cross21_3_Scalp",   "cross",   21,  3, 20, 45, 68, 0.19, 0.44) }
func NewStochOversold5_Scalp()  *StochSignalScalper { return newStochScalper("Stoch_Oversold5_Scalp",   "oversold", 5,  3,  0, 32, 72, 0.16, 0.38) }
func NewStochOversold9_Scalp()  *StochSignalScalper { return newStochScalper("Stoch_Oversold9_Scalp",   "oversold", 9,  3,  0, 30, 72, 0.17, 0.40) }
func NewStochOversold14_Scalp() *StochSignalScalper { return newStochScalper("Stoch_Oversold14_Scalp",  "oversold",14,  3,  0, 30, 72, 0.18, 0.42) }
func NewStochOversold21_Scalp() *StochSignalScalper { return newStochScalper("Stoch_Oversold21_Scalp",  "oversold",21,  3,  0, 30, 72, 0.19, 0.44) }
func NewStochTrend9_Scalp()     *StochSignalScalper { return newStochScalper("Stoch_Trend9_Scalp",      "trend",    9,  3, 20, 48, 72, 0.17, 0.40) }
func NewStochTrend14_Scalp()    *StochSignalScalper { return newStochScalper("Stoch_Trend14_Scalp",     "trend",   14,  3, 20, 48, 72, 0.18, 0.42) }
func NewStochTrend21_Scalp()    *StochSignalScalper { return newStochScalper("Stoch_Trend21_Scalp",     "trend",   21,  3, 22, 48, 72, 0.19, 0.44) }

// ── ATR signal family (10) ────────────────────────────────────────────────────
func NewATRMom7_14_Scalp()    *ATRSignalScalper { return newATRScalper("ATR_Mom7_14_Scalp",    "momentum",    7, 14, 20, 48, 70, 0.17, 0.40) }
func NewATRMom10_20_Scalp()   *ATRSignalScalper { return newATRScalper("ATR_Mom10_20_Scalp",   "momentum",   10, 20, 20, 48, 70, 0.18, 0.42) }
func NewATRMom14_20_Scalp()   *ATRSignalScalper { return newATRScalper("ATR_Mom14_20_Scalp",   "momentum",   14, 20, 22, 48, 70, 0.18, 0.42) }
func NewATRMom14_50_Scalp()   *ATRSignalScalper { return newATRScalper("ATR_Mom14_50_Scalp",   "momentum",   14, 50, 22, 48, 70, 0.20, 0.46) }
func NewATRMom21_50_Scalp()   *ATRSignalScalper { return newATRScalper("ATR_Mom21_50_Scalp",   "momentum",   21, 50, 25, 47, 70, 0.20, 0.48) }
func NewATRChan14_20_Scalp()  *ATRSignalScalper { return newATRScalper("ATR_Chan14_20_Scalp",  "channel_break", 14, 20, 22, 52, 74, 0.18, 0.44) }
func NewATRChan14_50_Scalp()  *ATRSignalScalper { return newATRScalper("ATR_Chan14_50_Scalp",  "channel_break", 14, 50, 22, 52, 74, 0.20, 0.48) }
func NewATRChan10_20_Scalp()  *ATRSignalScalper { return newATRScalper("ATR_Chan10_20_Scalp",  "channel_break", 10, 20, 20, 52, 74, 0.18, 0.44) }
func NewATRContr14_20_Scalp() *ATRSignalScalper { return newATRScalper("ATR_Contr14_20_Scalp", "contraction",   14, 20, 20, 48, 70, 0.18, 0.44) }
func NewATRContr10_20_Scalp() *ATRSignalScalper { return newATRScalper("ATR_Contr10_20_Scalp", "contraction",   10, 20, 18, 48, 70, 0.17, 0.42) }

// ── ROC signal family (8) ─────────────────────────────────────────────────────
func NewROC3_0p3_Scalp()  *ROCSignalScalper { return newROCScalper("ROC3_0p3_Scalp",   3, 0.30, 15, 47, 70, 0.16, 0.38) }
func NewROC5_0p5_Scalp()  *ROCSignalScalper { return newROCScalper("ROC5_0p5_Scalp",   5, 0.50, 18, 47, 70, 0.17, 0.40) }
func NewROC5_1p0_Scalp()  *ROCSignalScalper { return newROCScalper("ROC5_1p0_Scalp",   5, 1.00, 20, 47, 70, 0.17, 0.40) }
func NewROC9_0p5_Scalp()  *ROCSignalScalper { return newROCScalper("ROC9_0p5_Scalp",   9, 0.50, 18, 46, 70, 0.18, 0.42) }
func NewROC9_1p0_Scalp()  *ROCSignalScalper { return newROCScalper("ROC9_1p0_Scalp",   9, 1.00, 20, 46, 70, 0.18, 0.42) }
func NewROC12_0p5_Scalp() *ROCSignalScalper { return newROCScalper("ROC12_0p5_Scalp", 12, 0.50, 20, 46, 70, 0.19, 0.44) }
func NewROC12_1p0_Scalp() *ROCSignalScalper { return newROCScalper("ROC12_1p0_Scalp", 12, 1.00, 22, 46, 70, 0.19, 0.44) }
func NewROC21_1p5_Scalp() *ROCSignalScalper { return newROCScalper("ROC21_1p5_Scalp", 21, 1.50, 22, 45, 70, 0.20, 0.46) }

// ── Williams %R family (8) ────────────────────────────────────────────────────
func NewWRBounce7_Scalp()  *WilliamsRScalperV2 { return newWilliamsRV2("WR_Bounce7_Scalp",   "bounce",  7, 15, 30, 72, 0.16, 0.38) }
func NewWRBounce10_Scalp() *WilliamsRScalperV2 { return newWilliamsRV2("WR_Bounce10_Scalp",  "bounce", 10, 15, 30, 72, 0.17, 0.40) }
func NewWRBounce14_Scalp() *WilliamsRScalperV2 { return newWilliamsRV2("WR_Bounce14_Scalp",  "bounce", 14, 18, 30, 72, 0.18, 0.42) }
func NewWRBounce21_Scalp() *WilliamsRScalperV2 { return newWilliamsRV2("WR_Bounce21_Scalp",  "bounce", 21, 18, 30, 72, 0.19, 0.44) }
func NewWRTrend7_Scalp()   *WilliamsRScalperV2 { return newWilliamsRV2("WR_Trend7_Scalp",    "trend",   7, 18, 46, 70, 0.16, 0.38) }
func NewWRTrend10_Scalp()  *WilliamsRScalperV2 { return newWilliamsRV2("WR_Trend10_Scalp",   "trend",  10, 18, 46, 70, 0.17, 0.40) }
func NewWRTrend14_Scalp()  *WilliamsRScalperV2 { return newWilliamsRV2("WR_Trend14_Scalp",   "trend",  14, 20, 46, 70, 0.18, 0.42) }
func NewWRTrend21_Scalp()  *WilliamsRScalperV2 { return newWilliamsRV2("WR_Trend21_Scalp",   "trend",  21, 20, 46, 70, 0.19, 0.44) }

// ── Parabolic SAR + EMA family (8) ───────────────────────────────────────────
func NewPsarEMA9_0p02_Scalp()  *PsarEMAScalper { return newPsarEMA("PSAR_EMA9_0p02_Scalp",   9, 0.02, 0.20, 18, 46, 70, 0.17, 0.40) }
func NewPsarEMA14_0p02_Scalp() *PsarEMAScalper { return newPsarEMA("PSAR_EMA14_0p02_Scalp", 14, 0.02, 0.20, 20, 46, 70, 0.18, 0.42) }
func NewPsarEMA20_0p02_Scalp() *PsarEMAScalper { return newPsarEMA("PSAR_EMA20_0p02_Scalp", 20, 0.02, 0.20, 20, 46, 70, 0.18, 0.42) }
func NewPsarEMA20_0p03_Scalp() *PsarEMAScalper { return newPsarEMA("PSAR_EMA20_0p03_Scalp", 20, 0.03, 0.20, 22, 46, 70, 0.18, 0.42) }
func NewPsarEMA50_0p02_Scalp() *PsarEMAScalper { return newPsarEMA("PSAR_EMA50_0p02_Scalp", 50, 0.02, 0.20, 22, 45, 70, 0.20, 0.48) }
func NewPsarEMA9_0p01_Scalp()  *PsarEMAScalper { return newPsarEMA("PSAR_EMA9_0p01_Scalp",   9, 0.01, 0.20, 16, 47, 70, 0.16, 0.38) }
func NewPsarEMA20_0p01_Scalp() *PsarEMAScalper { return newPsarEMA("PSAR_EMA20_0p01_Scalp", 20, 0.01, 0.20, 18, 47, 70, 0.17, 0.40) }
func NewPsarEMA14_0p03_Scalp() *PsarEMAScalper { return newPsarEMA("PSAR_EMA14_0p03_Scalp", 14, 0.03, 0.20, 20, 46, 70, 0.18, 0.42) }

// ── Hull MA family (8) ────────────────────────────────────────────────────────
func NewHullMA7_Scalp()  *HullMAScalperV2 { return newHullMAV2("HullMA7_Scalp",   7, 15, 46, 70, 0.16, 0.38) }
func NewHullMA9_Scalp()  *HullMAScalperV2 { return newHullMAV2("HullMA9_Scalp",   9, 18, 46, 70, 0.17, 0.40) }
func NewHullMA14_Scalp() *HullMAScalperV2 { return newHullMAV2("HullMA14_Scalp", 14, 20, 46, 70, 0.18, 0.42) }
func NewHullMA20_Scalp() *HullMAScalperV2 { return newHullMAV2("HullMA20_Scalp", 20, 20, 45, 70, 0.18, 0.44) }
func NewHullMA25_Scalp() *HullMAScalperV2 { return newHullMAV2("HullMA25_Scalp", 25, 22, 45, 70, 0.19, 0.44) }
func NewHullMA30_Scalp() *HullMAScalperV2 { return newHullMAV2("HullMA30_Scalp", 30, 22, 45, 70, 0.19, 0.46) }
func NewHullMA40_Scalp() *HullMAScalperV2 { return newHullMAV2("HullMA40_Scalp", 40, 22, 45, 70, 0.20, 0.46) }
func NewHullMA50_Scalp() *HullMAScalperV2 { return newHullMAV2("HullMA50_Scalp", 50, 25, 44, 70, 0.20, 0.48) }

// ── Keltner Channel family (12) ───────────────────────────────────────────────
func NewKeltBreak20_14_1p5_Scalp()  *KeltnerScalperV2 { return newKeltnerV2("Kelt_Break20_1p5_Scalp",  "break",   20, 14, 1.5, 20, 52, 74, 0.18, 0.44) }
func NewKeltBreak20_14_2_Scalp()    *KeltnerScalperV2 { return newKeltnerV2("Kelt_Break20_2_Scalp",    "break",   20, 14, 2.0, 22, 52, 74, 0.19, 0.46) }
func NewKeltBreak20_14_2p5_Scalp()  *KeltnerScalperV2 { return newKeltnerV2("Kelt_Break20_2p5_Scalp",  "break",   20, 14, 2.5, 22, 52, 76, 0.20, 0.48) }
func NewKeltBreak10_14_1p5_Scalp()  *KeltnerScalperV2 { return newKeltnerV2("Kelt_Break10_1p5_Scalp",  "break",   10, 14, 1.5, 18, 52, 74, 0.17, 0.42) }
func NewKeltBreak50_14_2_Scalp()    *KeltnerScalperV2 { return newKeltnerV2("Kelt_Break50_2_Scalp",    "break",   50, 14, 2.0, 22, 52, 74, 0.20, 0.48) }
func NewKeltBreak20_10_1p5_Scalp()  *KeltnerScalperV2 { return newKeltnerV2("Kelt_Break20_10_1p5_Scalp","break",  20, 10, 1.5, 20, 52, 74, 0.18, 0.44) }
func NewKeltBounce20_14_1p5_Scalp() *KeltnerScalperV2 { return newKeltnerV2("Kelt_Bounce20_1p5_Scalp", "bounce",  20, 14, 1.5,  0, 30, 72, 0.18, 0.44) }
func NewKeltBounce20_14_2_Scalp()   *KeltnerScalperV2 { return newKeltnerV2("Kelt_Bounce20_2_Scalp",   "bounce",  20, 14, 2.0,  0, 28, 72, 0.19, 0.46) }
func NewKeltBounce10_14_1p5_Scalp() *KeltnerScalperV2 { return newKeltnerV2("Kelt_Bounce10_1p5_Scalp", "bounce",  10, 14, 1.5,  0, 30, 72, 0.17, 0.42) }
func NewKeltMid20_14_Scalp()        *KeltnerScalperV2 { return newKeltnerV2("Kelt_Mid20_14_Scalp",     "midline", 20, 14, 1.5, 18, 45, 70, 0.18, 0.42) }
func NewKeltMid10_14_Scalp()        *KeltnerScalperV2 { return newKeltnerV2("Kelt_Mid10_14_Scalp",     "midline", 10, 14, 1.5, 16, 45, 70, 0.17, 0.40) }
func NewKeltMid50_14_Scalp()        *KeltnerScalperV2 { return newKeltnerV2("Kelt_Mid50_14_Scalp",     "midline", 50, 14, 2.0, 22, 45, 70, 0.20, 0.46) }

// ── Momentum Divergence family (6) ────────────────────────────────────────────
func NewMomDiv14_5_Scalp()  *MomentumDivScalper { return newMomDiv("MomDiv_14_5_Scalp",   14,  5, 28, 0.17, 0.40) }
func NewMomDiv14_8_Scalp()  *MomentumDivScalper { return newMomDiv("MomDiv_14_8_Scalp",   14,  8, 28, 0.18, 0.42) }
func NewMomDiv14_10_Scalp() *MomentumDivScalper { return newMomDiv("MomDiv_14_10_Scalp",  14, 10, 30, 0.18, 0.44) }
func NewMomDiv9_5_Scalp()   *MomentumDivScalper { return newMomDiv("MomDiv_9_5_Scalp",     9,  5, 25, 0.16, 0.38) }
func NewMomDiv9_8_Scalp()   *MomentumDivScalper { return newMomDiv("MomDiv_9_8_Scalp",     9,  8, 25, 0.17, 0.40) }
func NewMomDiv21_10_Scalp() *MomentumDivScalper { return newMomDiv("MomDiv_21_10_Scalp",  21, 10, 32, 0.19, 0.44) }

// ── Consecutive Candles family (8) ────────────────────────────────────────────
func NewConsec2_ADX18_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec2_ADX18_Scalp", 2, 18, 48, 70, 0.16, 0.38) }
func NewConsec3_ADX20_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec3_ADX20_Scalp", 3, 20, 48, 70, 0.17, 0.40) }
func NewConsec3_ADX22_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec3_ADX22_Scalp", 3, 22, 48, 70, 0.17, 0.40) }
func NewConsec4_ADX20_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec4_ADX20_Scalp", 4, 20, 48, 70, 0.18, 0.42) }
func NewConsec4_ADX25_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec4_ADX25_Scalp", 4, 25, 48, 70, 0.18, 0.42) }
func NewConsec5_ADX22_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec5_ADX22_Scalp", 5, 22, 50, 72, 0.19, 0.44) }
func NewConsec5_ADX28_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec5_ADX28_Scalp", 5, 28, 50, 72, 0.19, 0.44) }
func NewConsec3_Tight_Scalp() *ConsecCandlesScalper { return newConsecCandles("Consec3_Tight_Scalp", 3, 25, 48, 70, 0.16, 0.38) }

// ── Additional EMA Cross variants (5) ─────────────────────────────────────────
func NewEMA2_5CrossScalp()   *EMACrossV2 { return newEMACrossV2("EMA_2_5_Cross_Scalp",    2,  5, 16, 48, 68, 0.15, 0.36) }
func NewEMA3_21CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_3_21_Cross_Scalp",   3, 21, 18, 47, 68, 0.17, 0.40) }
func NewEMA5_50CrossScalp()  *EMACrossV2 { return newEMACrossV2("EMA_5_50_Cross_Scalp",   5, 50, 22, 45, 70, 0.20, 0.46) }
func NewEMA10_50CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_10_50_Cross_Scalp", 10, 50, 22, 45, 70, 0.20, 0.46) }
func NewEMA21_89CrossScalp() *EMACrossV2 { return newEMACrossV2("EMA_21_89_Cross_Scalp", 21, 89, 25, 44, 72, 0.22, 0.50) }

// ── Additional RSI variants (5) ───────────────────────────────────────────────
func NewRSIOversold20Scalp()  *RSIThresholdScalper { return newRSIThreshold("RSI_Oversold20_Scalp",  14, 22, 28, 12, 0.18, 0.42) }
func NewRSIOversold28Scalp()  *RSIThresholdScalper { return newRSIThreshold("RSI_Oversold28_Scalp",  14, 28, 34, 14, 0.17, 0.40) }
func NewRSICross45Scalp()     *RSIThresholdScalper { return newRSIThreshold("RSI_Cross45_Scalp",     14, 43, 47, 18, 0.18, 0.42) }
func NewRSICross60Scalp()     *RSIThresholdScalper { return newRSIThreshold("RSI_Cross60_Scalp",     14, 58, 62, 22, 0.18, 0.44) }
func NewRSI7Cross50Scalp()    *RSIThresholdScalper { return newRSIThreshold("RSI7_Cross50_Scalp",     7, 48, 52, 16, 0.16, 0.38) }

// ── Additional Bollinger Band variants (5) ────────────────────────────────────
func NewBBBounce40_2Scalp()     *BBSignalScalper { return newBBScalper("BB_Bounce40_2_Scalp",    "bounce_lower", 40, 2.0, 0, 22, 28, 48, 0.20, 0.46) }
func NewBBBounce14_1p8Scalp()   *BBSignalScalper { return newBBScalper("BB_Bounce14_1p8_Scalp",  "bounce_lower", 14, 1.8, 0, 18, 30, 50, 0.17, 0.40) }
func NewBBMidCross30Scalp()     *BBSignalScalper { return newBBScalper("BB_MidCross30_Scalp",    "mid_cross",    30, 2.0, 20, 0, 45, 70, 0.19, 0.44) }
func NewBBBreakout30_2Scalp()   *BBSignalScalper { return newBBScalper("BB_Breakout30_2_Scalp",  "breakout",     30, 2.0, 22, 0, 52, 75, 0.20, 0.48) }
func NewBBWidth20_2p5Scalp()    *BBWidthScalper  { return newBBWidth("BB_Width20_2p5_Scalp",     20, 2.5, 18, 44, 72, 0.18, 0.44) }

// ── Additional VWAP variants (5) ──────────────────────────────────────────────
func NewVWAPCross40Scalp()    *VWAPSignalScalper { return newVWAPScalper("VWAP_Cross40_Scalp",    "cross",     40, 0,    20, 45, 70, 0.18, 0.42) }
func NewVWAPDev0p6Scalp()     *VWAPSignalScalper { return newVWAPScalper("VWAP_Dev0p6_Scalp",     "deviation", 40, 0.6,  18, 28, 50, 0.20, 0.48) }
func NewVWAPPullback40Scalp() *VWAPSignalScalper { return newVWAPScalper("VWAP_Pullback40_Scalp",  "pullback",  40, 0,    22, 45, 65, 0.18, 0.42) }
func NewVWAPCross60Scalp()    *VWAPSignalScalper { return newVWAPScalper("VWAP_Cross60_Scalp",    "cross",     60, 0,    22, 45, 70, 0.20, 0.46) }
func NewVWAPDev0p15Scalp()    *VWAPSignalScalper { return newVWAPScalper("VWAP_Dev0p15_Scalp",    "deviation", 20, 0.15, 12, 36, 56, 0.16, 0.38) }

// ── Additional N-bar breakout variants (5) ────────────────────────────────────
func NewNBar4Break()  *NBarBreakoutScalper { return newNBarBreakout("NBar4_Break_Scalp",   4, 16, 52, 74, 0.16, 0.38) }
func NewNBar6Break()  *NBarBreakoutScalper { return newNBarBreakout("NBar6_Break_Scalp",   6, 18, 52, 74, 0.17, 0.40) }
func NewNBar14Break() *NBarBreakoutScalper { return newNBarBreakout("NBar14_Break_Scalp", 14, 22, 52, 74, 0.18, 0.44) }
func NewNBar18Break() *NBarBreakoutScalper { return newNBarBreakout("NBar18_Break_Scalp", 18, 22, 52, 74, 0.19, 0.46) }
func NewNBar40Break() *NBarBreakoutScalper { return newNBarBreakout("NBar40_Break_Scalp", 40, 25, 52, 74, 0.22, 0.50) }

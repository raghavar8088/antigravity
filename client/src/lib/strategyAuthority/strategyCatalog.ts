import type { ISPAPCatalogEntry } from "./types";

/** Complete catalog of all 305 strategies from engine/internal/strategy/curated_registry.go */
export const STRATEGY_CATALOG: ISPAPCatalogEntry[] = [
  // ── ORIGINAL PROVEN (24 surviving) ─────────────────────────────────────
  { id: "EMACrossScalper_8_21", name: "EMA Cross Scalper (8/21)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VolumeWeightedTrendScalper", name: "Volume Weighted Trend", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPRSIReversionScalper", name: "VWAP RSI Reversion", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "BollingerRSIFadeScalper", name: "Bollinger RSI Fade", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "StochasticRangeScalper", name: "Stochastic Range", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "VolatilitySqueeze", name: "Volatility Squeeze", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "RangeCompressionScalper_20_0p3", name: "Range Compression (20/0.3)", family: "Volatility", category: "Volatility", timeframe: "5m" },
  { id: "OpeningRangeBreakoutScalper", name: "Opening Range Breakout", family: "Breakout", category: "Time-of-Day", timeframe: "1m" },
  { id: "OrderFlowPressureProScalper_80", name: "Order Flow Pressure Pro", family: "Microstructure", category: "Microstructure", timeframe: "tick" },
  { id: "LinRegScalper_30", name: "Linear Regression (30)", family: "Mean Reversion", category: "Statistical", timeframe: "1m" },
  { id: "ZScoreBandScalper_30", name: "Z-Score Band (30)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "RSIBBScalper", name: "RSI + Bollinger Band", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "TripleFilterScalper", name: "Triple Filter", family: "Multi-Signal", category: "Multi-Signal", timeframe: "1m" },
  { id: "SentimentConfluenceProScalper", name: "Sentiment Confluence Pro", family: "Multi-Signal", category: "Multi-Signal", timeframe: "1m" },
  { id: "ExhaustionScalper_10", name: "Exhaustion (10)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "ChartDoubleTapReversalScalper", name: "Double Tap Reversal", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "ChartWedgeBreakoutScalper", name: "Wedge Breakout", family: "Breakout", category: "Price Action Elite", timeframe: "5m" },
  { id: "AdaptiveRSIScalper_14", name: "Adaptive RSI (14)", family: "Trend", category: "Adaptive Elite", timeframe: "1m" },
  { id: "TrendMomentumScoreScalper", name: "Trend Momentum Score", family: "Multi-Signal", category: "Multi-Signal", timeframe: "1m" },
  { id: "VWAPBounceScalper", name: "VWAP Bounce", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "SessionOpenMomentumScalper", name: "Session Open Momentum", family: "Trend", category: "Time-of-Day", timeframe: "1m" },
  { id: "RSIMACDDivergenceScalper", name: "RSI MACD Divergence", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "TripleTrendConfluenceScalper", name: "Triple Trend Confluence", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "BollingerWalkScalper", name: "Bollinger Walk", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V2 — EMA Cross (15) ───────────────────────────────────────────
  { id: "EMA3_8CrossScalp", name: "EMA Cross (3/8)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA5_13CrossScalp", name: "EMA Cross (5/13)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA10_30CrossScalp", name: "EMA Cross (10/30)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA13_34CrossScalp", name: "EMA Cross (13/34)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA21_55CrossScalp", name: "EMA Cross (21/55)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA5_20CrossScalp", name: "EMA Cross (5/20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA8_34CrossScalp", name: "EMA Cross (8/34)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA3_15CrossScalp", name: "EMA Cross (3/15)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA7_21CrossScalp", name: "EMA Cross (7/21)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA12_26CrossScalp", name: "EMA Cross (12/26)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA20_50CrossScalp", name: "EMA Cross (20/50)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA4_12CrossScalp", name: "EMA Cross (4/12)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA6_18CrossScalp", name: "EMA Cross (6/18)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA15_45CrossScalp", name: "EMA Cross (15/45)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA9_26CrossScalp", name: "EMA Cross (9/26)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V2 — RSI Threshold (8) ───────────────────────────────────────
  { id: "RSIOversold30Scalp", name: "RSI Oversold (30)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSIOversold35Scalp", name: "RSI Oversold (35)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSICross50Scalp", name: "RSI Cross (50)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSICross55Scalp", name: "RSI Cross (55)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSIZoneBull60Scalp", name: "RSI Zone Bull (60)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSI9Cross50Scalp", name: "RSI9 Cross (50)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSI7Oversold28Scalp", name: "RSI7 Oversold (28)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSI21BullScalp", name: "RSI21 Bull", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },

  // ── ELITE V2 — RSI Slope (5) ────────────────────────────────────────────
  { id: "RSI14Slope5Scalp", name: "RSI14 Slope (5)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "RSI14Slope8Scalp", name: "RSI14 Slope (8)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "RSISlope3_3Scalp", name: "RSI Slope (3/3)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "RSISlope5_10Scalp", name: "RSI Slope (5/10)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "RSI9Slope3_5Scalp", name: "RSI9 Slope (3/5)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },

  // ── ELITE V2 — Bollinger Band (12) ─────────────────────────────────────
  { id: "BBBounce20_2Scalp", name: "BB Bounce (20/2)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "BBBounce14_2Scalp", name: "BB Bounce (14/2)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "BBBounce20_1p5Scalp", name: "BB Bounce (20/1.5)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "BBBounce30_2Scalp", name: "BB Bounce (30/2)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "BBMidCross20Scalp", name: "BB Mid Cross (20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "BBMidCross14Scalp", name: "BB Mid Cross (14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "BBBreakout20_2Scalp", name: "BB Breakout (20/2)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "BBBreakout20_2p5Scalp", name: "BB Breakout (20/2.5)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "BBBreakout14_2Scalp", name: "BB Breakout (14/2)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "BBWidth20_2Scalp", name: "BB Width (20/2)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "BBWidth14_2Scalp", name: "BB Width (14/2)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "BBWidth30_2Scalp", name: "BB Width (30/2)", family: "Volatility", category: "Volatility", timeframe: "1m" },

  // ── ELITE V2 — VWAP (10) ───────────────────────────────────────────────
  { id: "VWAPCross30Scalp", name: "VWAP Cross (30)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPCross50Scalp", name: "VWAP Cross (50)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPDev0p3Scalp", name: "VWAP Dev (0.3)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VWAPDev0p5Scalp", name: "VWAP Dev (0.5)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VWAPDev0p4Scalp", name: "VWAP Dev (0.4)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VWAPPullback30Scalp", name: "VWAP Pullback (30)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPPullback50Scalp", name: "VWAP Pullback (50)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPCross20Scalp", name: "VWAP Cross (20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPDev0p2Scalp", name: "VWAP Dev (0.2)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VWAPPullback20Scalp", name: "VWAP Pullback (20)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V2 — MACD (10) ───────────────────────────────────────────────
  { id: "MACDCross5_13_3Scalp", name: "MACD Cross (5/13/3)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDCross8_17_9Scalp", name: "MACD Cross (8/17/9)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDCross12_26_9Scalp", name: "MACD Cross (12/26/9)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDZero5_13Scalp", name: "MACD Zero (5/13)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDZero12_26Scalp", name: "MACD Zero (12/26)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDHistMom5_13Scalp", name: "MACD Hist (5/13)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDHistMom8_17Scalp", name: "MACD Hist (8/17)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDHistMom12_26Scalp", name: "MACD Hist (12/26)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDCross3_10_3Scalp", name: "MACD Cross (3/10/3)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "MACDZero3_10Scalp", name: "MACD Zero (3/10)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },

  // ── ELITE V2 — Volume + Price (8) ──────────────────────────────────────
  { id: "VolBreak1p5xScalp", name: "Vol Break (1.5x)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "VolBreak2xScalp", name: "Vol Break (2x)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "VolBreak2p5xScalp", name: "Vol Break (2.5x)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "VolBreak3xScalp", name: "Vol Break (3x)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "VolClimaxRev3xScalp", name: "Vol Climax Rev (3x)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VolClimaxRev4xScalp", name: "Vol Climax Rev (4x)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VolBreak10_1p5Scalp", name: "Vol Break (10/1.5x)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "VolBreak30_2Scalp", name: "Vol Break (30/2x)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },

  // ── ELITE V2 — N-Bar Breakout (10) ─────────────────────────────────────
  { id: "NBar3Break", name: "N-Bar Breakout (3)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar5Break", name: "N-Bar Breakout (5)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar7Break", name: "N-Bar Breakout (7)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar8Break", name: "N-Bar Breakout (8)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar10Break", name: "N-Bar Breakout (10)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar12Break", name: "N-Bar Breakout (12)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar15Break", name: "N-Bar Breakout (15)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "NBar20Break", name: "N-Bar Breakout (20)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "NBar25Break", name: "N-Bar Breakout (25)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "NBar30Break", name: "N-Bar Breakout (30)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },

  // ── ELITE V2 — Triple EMA (8) ───────────────────────────────────────────
  { id: "Triple3_8_21Scalp", name: "Triple EMA (3/8/21)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Triple5_13_34Scalp", name: "Triple EMA (5/13/34)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Triple8_21_55Scalp", name: "Triple EMA (8/21/55)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Triple10_30_60Scalp", name: "Triple EMA (10/30/60)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "Triple4_9_18Scalp", name: "Triple EMA (4/9/18)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Triple5_10_20Scalp", name: "Triple EMA (5/10/20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Triple6_14_30Scalp", name: "Triple EMA (6/14/30)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Triple7_21_50Scalp", name: "Triple EMA (7/21/50)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V2 — CCI (8) ─────────────────────────────────────────────────
  { id: "CCIZeroCross14Scalp", name: "CCI Zero Cross (14)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "CCIZeroCross20Scalp", name: "CCI Zero Cross (20)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "CCIZeroCross10Scalp", name: "CCI Zero Cross (10)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "CCIExtreme14Scalp", name: "CCI Extreme (14)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "CCIExtreme20Scalp", name: "CCI Extreme (20)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "CCIExtreme10Scalp", name: "CCI Extreme (10)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "CCITrend14Scalp", name: "CCI Trend (14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "CCITrend20Scalp", name: "CCI Trend (20)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V3 — Stochastic (12) ─────────────────────────────────────────
  { id: "StochCross5_3_Scalp", name: "Stoch Cross (5/3)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochCross9_3_Scalp", name: "Stoch Cross (9/3)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochCross14_3_Scalp", name: "Stoch Cross (14/3)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochCross14_5_Scalp", name: "Stoch Cross (14/5)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochCross21_3_Scalp", name: "Stoch Cross (21/3)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochOversold5_Scalp", name: "Stoch Oversold (5)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochOversold9_Scalp", name: "Stoch Oversold (9)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochOversold14_Scalp", name: "Stoch Oversold (14)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochOversold21_Scalp", name: "Stoch Oversold (21)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "StochTrend9_Scalp", name: "Stoch Trend (9)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "StochTrend14_Scalp", name: "Stoch Trend (14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "StochTrend21_Scalp", name: "Stoch Trend (21)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V3 — ATR Signal (10) ─────────────────────────────────────────
  { id: "ATRMom7_14_Scalp", name: "ATR Momentum (7/14)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "ATRMom10_20_Scalp", name: "ATR Momentum (10/20)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "ATRMom14_20_Scalp", name: "ATR Momentum (14/20)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "ATRMom14_50_Scalp", name: "ATR Momentum (14/50)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "ATRMom21_50_Scalp", name: "ATR Momentum (21/50)", family: "Volatility", category: "Volatility", timeframe: "5m" },
  { id: "ATRChan14_20_Scalp", name: "ATR Channel (14/20)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "ATRChan14_50_Scalp", name: "ATR Channel (14/50)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "ATRChan10_20_Scalp", name: "ATR Channel (10/20)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "ATRContr14_20_Scalp", name: "ATR Contraction (14/20)", family: "Volatility", category: "Volatility", timeframe: "1m" },
  { id: "ATRContr10_20_Scalp", name: "ATR Contraction (10/20)", family: "Volatility", category: "Volatility", timeframe: "1m" },

  // ── ELITE V3 — ROC (8) ────────────────────────────────────────────────
  { id: "ROC3_0p3_Scalp", name: "ROC (3/0.3)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC5_0p5_Scalp", name: "ROC (5/0.5)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC5_1p0_Scalp", name: "ROC (5/1.0)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC9_0p5_Scalp", name: "ROC (9/0.5)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC9_1p0_Scalp", name: "ROC (9/1.0)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC12_0p5_Scalp", name: "ROC (12/0.5)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC12_1p0_Scalp", name: "ROC (12/1.0)", family: "Momentum", category: "Momentum Elite", timeframe: "1m" },
  { id: "ROC21_1p5_Scalp", name: "ROC (21/1.5)", family: "Momentum", category: "Momentum Elite", timeframe: "5m" },

  // ── ELITE V3 — Williams %R (8) ─────────────────────────────────────────
  { id: "WRBounce7_Scalp", name: "Williams %R Bounce (7)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "WRBounce10_Scalp", name: "Williams %R Bounce (10)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "WRBounce14_Scalp", name: "Williams %R Bounce (14)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "WRBounce21_Scalp", name: "Williams %R Bounce (21)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "WRTrend7_Scalp", name: "Williams %R Trend (7)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "WRTrend10_Scalp", name: "Williams %R Trend (10)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "WRTrend14_Scalp", name: "Williams %R Trend (14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "WRTrend21_Scalp", name: "Williams %R Trend (21)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V3 — Parabolic SAR (8) ───────────────────────────────────────
  { id: "PsarEMA9_0p02_Scalp", name: "PSAR+EMA (9/0.02)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "PsarEMA14_0p02_Scalp", name: "PSAR+EMA (14/0.02)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "PsarEMA20_0p02_Scalp", name: "PSAR+EMA (20/0.02)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "PsarEMA20_0p03_Scalp", name: "PSAR+EMA (20/0.03)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "PsarEMA50_0p02_Scalp", name: "PSAR+EMA (50/0.02)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "PsarEMA9_0p01_Scalp", name: "PSAR+EMA (9/0.01)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "PsarEMA20_0p01_Scalp", name: "PSAR+EMA (20/0.01)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "PsarEMA14_0p03_Scalp", name: "PSAR+EMA (14/0.03)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V3 — Hull MA (8) ─────────────────────────────────────────────
  { id: "HullMA7_Scalp", name: "Hull MA (7)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "HullMA9_Scalp", name: "Hull MA (9)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "HullMA14_Scalp", name: "Hull MA (14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "HullMA20_Scalp", name: "Hull MA (20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "HullMA25_Scalp", name: "Hull MA (25)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "HullMA30_Scalp", name: "Hull MA (30)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "HullMA40_Scalp", name: "Hull MA (40)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "HullMA50_Scalp", name: "Hull MA (50)", family: "Trend", category: "Trend", timeframe: "5m" },

  // ── ELITE V3 — Keltner (12) ────────────────────────────────────────────
  { id: "KeltBreak20_14_1p5_Scalp", name: "Keltner Break (20/14/1.5)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "KeltBreak20_14_2_Scalp", name: "Keltner Break (20/14/2)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "KeltBreak20_14_2p5_Scalp", name: "Keltner Break (20/14/2.5)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "KeltBreak10_14_1p5_Scalp", name: "Keltner Break (10/14/1.5)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "KeltBreak50_14_2_Scalp", name: "Keltner Break (50/14/2)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "KeltBreak20_10_1p5_Scalp", name: "Keltner Break (20/10/1.5)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "KeltBounce20_14_1p5_Scalp", name: "Keltner Bounce (20/14/1.5)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "KeltBounce20_14_2_Scalp", name: "Keltner Bounce (20/14/2)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "KeltBounce10_14_1p5_Scalp", name: "Keltner Bounce (10/14/1.5)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "KeltMid20_14_Scalp", name: "Keltner Mid (20/14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "KeltMid10_14_Scalp", name: "Keltner Mid (10/14)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "KeltMid50_14_Scalp", name: "Keltner Mid (50/14)", family: "Trend", category: "Trend", timeframe: "5m" },

  // ── ELITE V3 — Momentum Divergence (6) ─────────────────────────────────
  { id: "MomDiv14_5_Scalp", name: "Momentum Div (14/5)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "MomDiv14_8_Scalp", name: "Momentum Div (14/8)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "MomDiv14_10_Scalp", name: "Momentum Div (14/10)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "MomDiv9_5_Scalp", name: "Momentum Div (9/5)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "MomDiv9_8_Scalp", name: "Momentum Div (9/8)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "1m" },
  { id: "MomDiv21_10_Scalp", name: "Momentum Div (21/10)", family: "Price Action Elite", category: "Price Action Elite", timeframe: "5m" },

  // ── ELITE V3 — Consecutive Candles (8) ─────────────────────────────────
  { id: "Consec2_ADX18_Scalp", name: "Consecutive (2/ADX18)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec3_ADX20_Scalp", name: "Consecutive (3/ADX20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec3_ADX22_Scalp", name: "Consecutive (3/ADX22)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec4_ADX20_Scalp", name: "Consecutive (4/ADX20)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec4_ADX25_Scalp", name: "Consecutive (4/ADX25)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec5_ADX22_Scalp", name: "Consecutive (5/ADX22)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec5_ADX28_Scalp", name: "Consecutive (5/ADX28)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "Consec3_Tight_Scalp", name: "Consecutive Tight (3)", family: "Trend", category: "Trend", timeframe: "1m" },

  // ── ELITE V3 — Additional EMA variants (5) ─────────────────────────────
  { id: "EMA2_5CrossScalp", name: "EMA Cross (2/5)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA3_21CrossScalp", name: "EMA Cross (3/21)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "EMA5_50CrossScalp", name: "EMA Cross (5/50)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "EMA10_50CrossScalp", name: "EMA Cross (10/50)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "EMA21_89CrossScalp", name: "EMA Cross (21/89)", family: "Trend", category: "Trend", timeframe: "5m" },

  // ── ELITE V3 — Additional RSI variants (5) ─────────────────────────────
  { id: "RSIOversold20Scalp", name: "RSI Oversold (20)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSIOversold28Scalp", name: "RSI Oversold (28)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSICross45Scalp", name: "RSI Cross (45)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSICross60Scalp", name: "RSI Cross (60)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "RSI7Cross50Scalp", name: "RSI7 Cross (50)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },

  // ── ELITE V3 — Additional BB variants (5) ──────────────────────────────
  { id: "BBBounce40_2Scalp", name: "BB Bounce (40/2)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "5m" },
  { id: "BBBounce14_1p8Scalp", name: "BB Bounce (14/1.8)", family: "Mean Reversion", category: "Mean Reversion", timeframe: "1m" },
  { id: "BBMidCross30Scalp", name: "BB Mid Cross (30)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "BBBreakout30_2Scalp", name: "BB Breakout (30/2)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "BBWidth20_2p5Scalp", name: "BB Width (20/2.5)", family: "Volatility", category: "Volatility", timeframe: "1m" },

  // ── ELITE V3 — Additional VWAP variants (5) ────────────────────────────
  { id: "VWAPCross40Scalp", name: "VWAP Cross (40)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPDev0p6Scalp", name: "VWAP Dev (0.6)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },
  { id: "VWAPPullback40Scalp", name: "VWAP Pullback (40)", family: "Trend", category: "Trend", timeframe: "1m" },
  { id: "VWAPCross60Scalp", name: "VWAP Cross (60)", family: "Trend", category: "Trend", timeframe: "5m" },
  { id: "VWAPDev0p15Scalp", name: "VWAP Dev (0.15)", family: "Mean Reversion", category: "Mean Rev Elite", timeframe: "1m" },

  // ── ELITE V3 — Additional N-bar variants (5) ───────────────────────────
  { id: "NBar4Break", name: "N-Bar Breakout (4)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar6Break", name: "N-Bar Breakout (6)", family: "Breakout", category: "Breakout Elite", timeframe: "1m" },
  { id: "NBar14Break", name: "N-Bar Breakout (14)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "NBar18Break", name: "N-Bar Breakout (18)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },
  { id: "NBar40Break", name: "N-Bar Breakout (40)", family: "Breakout", category: "Breakout Elite", timeframe: "5m" },

  // ── INTRADAY — EMA Cross 5m/15m (10) ───────────────────────────────────
  { id: "ID_EMA5_20_5m", name: "Intraday EMA (5/20) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_EMA8_21_5m", name: "Intraday EMA (8/21) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_EMA9_50_5m", name: "Intraday EMA (9/50) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_EMA12_26_5m", name: "Intraday EMA (12/26) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_EMA20_50_5m", name: "Intraday EMA (20/50) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_EMA21_55_5m", name: "Intraday EMA (21/55) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_EMA50_200_15m", name: "Intraday EMA (50/200) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_EMA20_100_15m", name: "Intraday EMA (20/100) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_EMA10_30_15m", name: "Intraday EMA (10/30) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_EMA13_34_15m", name: "Intraday EMA (13/34) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },

  // ── INTRADAY — Triple EMA (8) ───────────────────────────────────────────
  { id: "ID_Triple5_13_34_5m", name: "Intraday Triple EMA (5/13/34) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_Triple8_21_55_5m", name: "Intraday Triple EMA (8/21/55) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_Triple10_30_60_5m", name: "Intraday Triple EMA (10/30/60) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_Triple20_50_100_15m", name: "Intraday Triple EMA (20/50/100) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_Triple5_10_20_5m", name: "Intraday Triple EMA (5/10/20) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_Triple9_18_36_5m", name: "Intraday Triple EMA (9/18/36) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_Triple10_20_40_15m", name: "Intraday Triple EMA (10/20/40) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_Triple20_50_200_15m", name: "Intraday Triple EMA (20/50/200) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },

  // ── INTRADAY — MACD (8) ────────────────────────────────────────────────
  { id: "ID_MACDCross12_26_9_5m", name: "Intraday MACD Cross (12/26/9) 5m", family: "Momentum", category: "Intraday", timeframe: "5m" },
  { id: "ID_MACDZero12_26_5m", name: "Intraday MACD Zero (12/26) 5m", family: "Momentum", category: "Intraday", timeframe: "5m" },
  { id: "ID_MACDHistMom12_26_5m", name: "Intraday MACD Hist (12/26) 5m", family: "Momentum", category: "Intraday", timeframe: "5m" },
  { id: "ID_MACDCross26_52_9_15m", name: "Intraday MACD Cross (26/52/9) 15m", family: "Momentum", category: "Intraday", timeframe: "15m" },
  { id: "ID_MACDCross5_35_5_5m", name: "Intraday MACD Cross (5/35/5) 5m", family: "Momentum", category: "Intraday", timeframe: "5m" },
  { id: "ID_MACDCross12_26_9_15m", name: "Intraday MACD Cross (12/26/9) 15m", family: "Momentum", category: "Intraday", timeframe: "15m" },
  { id: "ID_MACDZero12_26_15m", name: "Intraday MACD Zero (12/26) 15m", family: "Momentum", category: "Intraday", timeframe: "15m" },
  { id: "ID_MACDHistMom5_13_5m", name: "Intraday MACD Hist (5/13) 5m", family: "Momentum", category: "Intraday", timeframe: "5m" },

  // ── INTRADAY — VWAP (6) ────────────────────────────────────────────────
  { id: "ID_VWAPCross50_5m", name: "Intraday VWAP Cross (50) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_VWAPCross100_15m", name: "Intraday VWAP Cross (100) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_VWAPDev0p5_5m", name: "Intraday VWAP Dev (0.5) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_VWAPDev1p0_15m", name: "Intraday VWAP Dev (1.0) 15m", family: "Mean Reversion", category: "Intraday", timeframe: "15m" },
  { id: "ID_VWAPPullback50_5m", name: "Intraday VWAP Pullback (50) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_VWAPPullback100_15m", name: "Intraday VWAP Pullback (100) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },

  // ── INTRADAY — Bollinger (8) ───────────────────────────────────────────
  { id: "ID_BBBounce20_2_5m", name: "Intraday BB Bounce (20/2) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_BBBounce50_2_15m", name: "Intraday BB Bounce (50/2) 15m", family: "Mean Reversion", category: "Intraday", timeframe: "15m" },
  { id: "ID_BBMidCross20_5m", name: "Intraday BB Mid Cross (20) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_BBMidCross50_15m", name: "Intraday BB Mid Cross (50) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_BBBreakout20_2_5m", name: "Intraday BB Breakout (20/2) 5m", family: "Breakout", category: "Intraday", timeframe: "5m" },
  { id: "ID_BBBreakout50_2_15m", name: "Intraday BB Breakout (50/2) 15m", family: "Breakout", category: "Intraday", timeframe: "15m" },
  { id: "ID_BBWidth20_2_5m", name: "Intraday BB Width (20/2) 5m", family: "Volatility", category: "Intraday", timeframe: "5m" },
  { id: "ID_BBWidth50_2_15m", name: "Intraday BB Width (50/2) 15m", family: "Volatility", category: "Intraday", timeframe: "15m" },

  // ── INTRADAY — RSI (5) ─────────────────────────────────────────────────
  { id: "ID_RSIOversold30_5m", name: "Intraday RSI Oversold (30) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_RSICross50_5m", name: "Intraday RSI Cross (50) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_RSICross55_5m", name: "Intraday RSI Cross (55) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_RSI21_15m", name: "Intraday RSI21 15m", family: "Mean Reversion", category: "Intraday", timeframe: "15m" },
  { id: "ID_RSIOversold20_5m", name: "Intraday RSI Oversold (20) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },

  // ── INTRADAY — Keltner (5) ─────────────────────────────────────────────
  { id: "ID_KeltBreak20_2_5m", name: "Intraday Keltner Break (20/2) 5m", family: "Breakout", category: "Intraday", timeframe: "5m" },
  { id: "ID_KeltBreak50_2_15m", name: "Intraday Keltner Break (50/2) 15m", family: "Breakout", category: "Intraday", timeframe: "15m" },
  { id: "ID_KeltBounce20_2_5m", name: "Intraday Keltner Bounce (20/2) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_KeltMid20_5m", name: "Intraday Keltner Mid (20) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_KeltMid50_15m", name: "Intraday Keltner Mid (50) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },

  // ── INTRADAY — Stochastic (5) ──────────────────────────────────────────
  { id: "ID_StochCross14_3_5m", name: "Intraday Stoch Cross (14/3) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_StochCross14_5_5m", name: "Intraday Stoch Cross (14/5) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_StochOversold14_5m", name: "Intraday Stoch Oversold (14) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_StochTrend14_5m", name: "Intraday Stoch Trend (14) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_StochCross21_5_15m", name: "Intraday Stoch Cross (21/5) 15m", family: "Mean Reversion", category: "Intraday", timeframe: "15m" },

  // ── INTRADAY — Hull MA (5) ─────────────────────────────────────────────
  { id: "ID_HullMA20_5m", name: "Intraday Hull MA (20) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_HullMA30_5m", name: "Intraday Hull MA (30) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_HullMA50_15m", name: "Intraday Hull MA (50) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_HullMA100_15m", name: "Intraday Hull MA (100) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },
  { id: "ID_HullMA14_5m", name: "Intraday Hull MA (14) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },

  // ── INTRADAY — CCI (5) ─────────────────────────────────────────────────
  { id: "ID_CCIZeroCross20_5m", name: "Intraday CCI Zero Cross (20) 5m", family: "Momentum", category: "Intraday", timeframe: "5m" },
  { id: "ID_CCIExtreme20_5m", name: "Intraday CCI Extreme (20) 5m", family: "Mean Reversion", category: "Intraday", timeframe: "5m" },
  { id: "ID_CCITrend20_5m", name: "Intraday CCI Trend (20) 5m", family: "Trend", category: "Intraday", timeframe: "5m" },
  { id: "ID_CCIZeroCross50_15m", name: "Intraday CCI Zero Cross (50) 15m", family: "Momentum", category: "Intraday", timeframe: "15m" },
  { id: "ID_CCITrend50_15m", name: "Intraday CCI Trend (50) 15m", family: "Trend", category: "Intraday", timeframe: "15m" },

  // ── INSTITUTIONAL ALPHA ENGINE (10) ────────────────────────────────────
  { id: "FundingMeanReversionAlpha", name: "Funding Mean Reversion Alpha", family: "Funding", category: "Funding", timeframe: "1m" },
  { id: "CVDDivergenceAlpha", name: "CVD Divergence Alpha", family: "Microstructure", category: "Microstructure", timeframe: "tick" },
  { id: "DeltaAbsorptionAlpha", name: "Delta Absorption Alpha", family: "Microstructure", category: "Microstructure", timeframe: "tick" },
  { id: "LiquiditySweepReversalAlpha", name: "Liquidity Sweep Reversal Alpha", family: "Liquidity", category: "Liquidity", timeframe: "1m" },
  { id: "FVGRetestAlpha", name: "FVG Retest Alpha", family: "Structure", category: "Structure", timeframe: "1m" },
  { id: "OrderBlockRetestAlpha", name: "Order Block Retest Alpha", family: "Smart Money", category: "Smart Money", timeframe: "1m" },
  { id: "MSSContinuationAlpha", name: "MSS Continuation Alpha", family: "Structure", category: "Structure", timeframe: "1m" },
  { id: "POCBounceAlpha", name: "POC Bounce Alpha", family: "Market Profile", category: "Market Profile", timeframe: "1m" },
  { id: "SessionExpansionAlpha", name: "Session Expansion Alpha", family: "Session", category: "Session", timeframe: "1m" },
  { id: "LiquidationCascadeAlpha", name: "Liquidation Cascade Alpha", family: "Liquidations", category: "Liquidations", timeframe: "1m" },

  // ── PHASE 11 MICROSTRUCTURE ALPHA ENGINE (7) ───────────────────────────
  { id: "Phase11LiquiditySweepAlpha", name: "Phase 11 Liquidity Sweep Alpha", family: "Phase 11 Liquidity", category: "Phase 11 Liquidity", timeframe: "1m" },
  { id: "Phase11FundingMeanReversionAlpha", name: "Phase 11 Funding MR Alpha", family: "Phase 11 Derivatives", category: "Phase 11 Derivatives", timeframe: "1m" },
  { id: "Phase11CVDDivergenceAlpha", name: "Phase 11 CVD Divergence Alpha", family: "Phase 11 Order Flow", category: "Phase 11 Order Flow", timeframe: "tick" },
  { id: "Phase11LiquidationCascadeAlpha", name: "Phase 11 Liquidation Cascade Alpha", family: "Phase 11 Liquidations", category: "Phase 11 Liquidations", timeframe: "1m" },
  { id: "Phase11FVGAlpha", name: "Phase 11 FVG Alpha", family: "Phase 11 Structure", category: "Phase 11 Structure", timeframe: "1m" },
  { id: "Phase11OrderBlockAlpha", name: "Phase 11 Order Block Alpha", family: "Phase 11 Smart Money", category: "Phase 11 Smart Money", timeframe: "1m" },
  { id: "Phase11MSSAlpha", name: "Phase 11 MSS Alpha", family: "Phase 11 Structure", category: "Phase 11 Structure", timeframe: "1m" },
];

export const CATALOG_BY_ID = new Map(STRATEGY_CATALOG.map((s) => [s.id, s]));

export const TOTAL_STRATEGIES = STRATEGY_CATALOG.length;

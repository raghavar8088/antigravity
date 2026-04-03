package strategy

// BuildCuratedScalpers returns all 300 live strategies across scalping and intraday.
// The signal aggregator's selective filter and priority scoring determine which
// signals are ultimately forwarded for execution each cycle.
func BuildCuratedScalpers() []RegistryEntry {
	return []RegistryEntry{
		// ── ORIGINAL PROVEN STRATEGIES (35) ────────────────────────────────────
		{NewEMACrossScalper(8, 21), "Trend", "1m"},
		{NewADXTrendScalper(), "Trend", "1m"},
		{NewVolumeWeightedTrendScalper(), "Trend", "1m"},
		{NewPullbackContinuationProScalper(), "Trend", "1m"},
		{NewVWAPRSI2ReversionScalper(), "Mean Rev Elite", "1m"},
		{NewBollingerRSIFadeScalper(), "Mean Rev Elite", "1m"},
		{NewMACDVWAPFlipScalper(), "Momentum Elite", "1m"},
		{NewStochasticRangeScalper(), "Mean Reversion", "1m"},
		{NewDonchianScalper(20), "Breakout", "5m"},
		{NewATRBreakoutScalper(14, 1.5), "Breakout Elite", "1m"},
		{NewATRVolumeImpulseScalper(), "Breakout Elite", "1m"},
		{NewVolatilitySqueeze(), "Volatility", "1m"},
		{NewRangeCompressionScalper(20, 0.3), "Volatility", "5m"},
		{NewPriceChannelScalper(20), "Breakout Elite", "5m"},
		{NewOpeningRangeBreakoutScalper(16, 0, 15), "Time-of-Day", "1m"},
		{NewVolumeBreakoutImpulseScalper(20), "Breakout Elite", "5m"},
		{NewOrderFlowPressureProScalper(80), "Microstructure", "tick"},
		{NewLinRegScalper(30, 2.0), "Statistical", "1m"},
		{NewZScoreBandScalper(30, 2.0), "Mean Rev Elite", "1m"},
		{NewRSIBBScalper(), "Mean Rev Elite", "1m"},
		{NewTripleFilterScalper(), "Multi-Signal", "1m"},
		{NewSentimentConfluenceProScalper(), "Multi-Signal", "1m"},
		{NewExhaustionScalper(10), "Price Action Elite", "1m"},
		{NewChartDoubleTapReversalScalper(), "Price Action Elite", "1m"},
		{NewChartWedgeBreakoutScalper(), "Price Action Elite", "5m"},
		{NewAdaptiveRSIScalper(14), "Adaptive Elite", "1m"},
		{NewKAMAScalper(10), "Adaptive", "1m"},
		{NewTrendMomentumScoreScalper(), "Multi-Signal", "1m"},
		{NewVWAPBounceScalper(), "Trend", "1m"},
		{NewSessionOpenMomentumScalper(), "Time-of-Day", "1m"},
		{NewRSIMACDDivergenceScalper(), "Price Action Elite", "1m"},
		{NewTripleTrendConfluenceScalper(), "Trend", "1m"},
		{NewVolumeDeltaSpikeScalper(), "Microstructure", "1m"},
		{NewMACDZeroCrossConfluenceScalper(), "Momentum Elite", "1m"},
		{NewBollingerWalkScalper(), "Trend", "1m"},

		// ── ELITE V2 — EMA Cross family (15) ───────────────────────────────────
		{NewEMA3_8CrossScalp(), "Trend", "1m"},
		{NewEMA5_13CrossScalp(), "Trend", "1m"},
		{NewEMA10_30CrossScalp(), "Trend", "1m"},
		{NewEMA13_34CrossScalp(), "Trend", "1m"},
		{NewEMA21_55CrossScalp(), "Trend", "1m"},
		{NewEMA5_20CrossScalp(), "Trend", "1m"},
		{NewEMA8_34CrossScalp(), "Trend", "1m"},
		{NewEMA3_15CrossScalp(), "Trend", "1m"},
		{NewEMA7_21CrossScalp(), "Trend", "1m"},
		{NewEMA12_26CrossScalp(), "Trend", "1m"},
		{NewEMA20_50CrossScalp(), "Trend", "1m"},
		{NewEMA4_12CrossScalp(), "Trend", "1m"},
		{NewEMA6_18CrossScalp(), "Trend", "1m"},
		{NewEMA15_45CrossScalp(), "Trend", "1m"},
		{NewEMA9_26CrossScalp(), "Trend", "1m"},

		// ── ELITE V2 — RSI threshold family (8) ────────────────────────────────
		{NewRSIOversold30Scalp(), "Mean Reversion", "1m"},
		{NewRSIOversold35Scalp(), "Mean Reversion", "1m"},
		{NewRSICross50Scalp(), "Mean Reversion", "1m"},
		{NewRSICross55Scalp(), "Mean Reversion", "1m"},
		{NewRSIZoneBull60Scalp(), "Mean Reversion", "1m"},
		{NewRSI9Cross50Scalp(), "Mean Reversion", "1m"},
		{NewRSI7Oversold28Scalp(), "Mean Reversion", "1m"},
		{NewRSI21BullScalp(), "Mean Reversion", "1m"},

		// ── ELITE V2 — RSI slope family (5) ────────────────────────────────────
		{NewRSI14Slope5Scalp(), "Mean Rev Elite", "1m"},
		{NewRSI14Slope8Scalp(), "Mean Rev Elite", "1m"},
		{NewRSISlope3_3Scalp(), "Mean Rev Elite", "1m"},
		{NewRSISlope5_10Scalp(), "Mean Rev Elite", "1m"},
		{NewRSI9Slope3_5Scalp(), "Mean Rev Elite", "1m"},

		// ── ELITE V2 — Bollinger Band family (12) ──────────────────────────────
		{NewBBBounce20_2Scalp(), "Mean Reversion", "1m"},
		{NewBBBounce14_2Scalp(), "Mean Reversion", "1m"},
		{NewBBBounce20_1p5Scalp(), "Mean Reversion", "1m"},
		{NewBBBounce30_2Scalp(), "Mean Reversion", "1m"},
		{NewBBMidCross20Scalp(), "Trend", "1m"},
		{NewBBMidCross14Scalp(), "Trend", "1m"},
		{NewBBBreakout20_2Scalp(), "Breakout Elite", "1m"},
		{NewBBBreakout20_2p5Scalp(), "Breakout Elite", "1m"},
		{NewBBBreakout14_2Scalp(), "Breakout Elite", "1m"},
		{NewBBWidth20_2Scalp(), "Volatility", "1m"},
		{NewBBWidth14_2Scalp(), "Volatility", "1m"},
		{NewBBWidth30_2Scalp(), "Volatility", "1m"},

		// ── ELITE V2 — VWAP family (10) ────────────────────────────────────────
		{NewVWAPCross30Scalp(), "Trend", "1m"},
		{NewVWAPCross50Scalp(), "Trend", "1m"},
		{NewVWAPDev0p3Scalp(), "Mean Rev Elite", "1m"},
		{NewVWAPDev0p5Scalp(), "Mean Rev Elite", "1m"},
		{NewVWAPDev0p4Scalp(), "Mean Rev Elite", "1m"},
		{NewVWAPPullback30Scalp(), "Trend", "1m"},
		{NewVWAPPullback50Scalp(), "Trend", "1m"},
		{NewVWAPCross20Scalp(), "Trend", "1m"},
		{NewVWAPDev0p2Scalp(), "Mean Rev Elite", "1m"},
		{NewVWAPPullback20Scalp(), "Trend", "1m"},

		// ── ELITE V2 — MACD family (10) ────────────────────────────────────────
		{NewMACDCross5_13_3Scalp(), "Momentum Elite", "1m"},
		{NewMACDCross8_17_9Scalp(), "Momentum Elite", "1m"},
		{NewMACDCross12_26_9Scalp(), "Momentum Elite", "1m"},
		{NewMACDZero5_13Scalp(), "Momentum Elite", "1m"},
		{NewMACDZero12_26Scalp(), "Momentum Elite", "1m"},
		{NewMACDHistMom5_13Scalp(), "Momentum Elite", "1m"},
		{NewMACDHistMom8_17Scalp(), "Momentum Elite", "1m"},
		{NewMACDHistMom12_26Scalp(), "Momentum Elite", "1m"},
		{NewMACDCross3_10_3Scalp(), "Momentum Elite", "1m"},
		{NewMACDZero3_10Scalp(), "Momentum Elite", "1m"},

		// ── ELITE V2 — Volume + Price family (8) ───────────────────────────────
		{NewVolBreak1p5xScalp(), "Breakout Elite", "1m"},
		{NewVolBreak2xScalp(), "Breakout Elite", "1m"},
		{NewVolBreak2p5xScalp(), "Breakout Elite", "1m"},
		{NewVolBreak3xScalp(), "Breakout Elite", "1m"},
		{NewVolClimaxRev3xScalp(), "Mean Rev Elite", "1m"},
		{NewVolClimaxRev4xScalp(), "Mean Rev Elite", "1m"},
		{NewVolBreak10_1p5Scalp(), "Breakout Elite", "1m"},
		{NewVolBreak30_2Scalp(), "Breakout Elite", "1m"},

		// ── ELITE V2 — N-bar breakout family (10) ──────────────────────────────
		{NewNBar3Break(), "Breakout Elite", "1m"},
		{NewNBar5Break(), "Breakout Elite", "1m"},
		{NewNBar7Break(), "Breakout Elite", "1m"},
		{NewNBar8Break(), "Breakout Elite", "1m"},
		{NewNBar10Break(), "Breakout Elite", "1m"},
		{NewNBar12Break(), "Breakout Elite", "1m"},
		{NewNBar15Break(), "Breakout Elite", "5m"},
		{NewNBar20Break(), "Breakout Elite", "5m"},
		{NewNBar25Break(), "Breakout Elite", "5m"},
		{NewNBar30Break(), "Breakout Elite", "5m"},

		// ── ELITE V2 — Triple EMA family (8) ───────────────────────────────────
		{NewTriple3_8_21Scalp(), "Trend", "1m"},
		{NewTriple5_13_34Scalp(), "Trend", "1m"},
		{NewTriple8_21_55Scalp(), "Trend", "1m"},
		{NewTriple10_30_60Scalp(), "Trend", "5m"},
		{NewTriple4_9_18Scalp(), "Trend", "1m"},
		{NewTriple5_10_20Scalp(), "Trend", "1m"},
		{NewTriple6_14_30Scalp(), "Trend", "1m"},
		{NewTriple7_21_50Scalp(), "Trend", "1m"},

		// ── ELITE V2 — CCI family (8) ──────────────────────────────────────────
		{NewCCIZeroCross14Scalp(), "Momentum Elite", "1m"},
		{NewCCIZeroCross20Scalp(), "Momentum Elite", "1m"},
		{NewCCIZeroCross10Scalp(), "Momentum Elite", "1m"},
		{NewCCIExtreme14Scalp(), "Mean Reversion", "1m"},
		{NewCCIExtreme20Scalp(), "Mean Reversion", "1m"},
		{NewCCIExtreme10Scalp(), "Mean Reversion", "1m"},
		{NewCCITrend14Scalp(), "Trend", "1m"},
		{NewCCITrend20Scalp(), "Trend", "1m"},

		// ── ELITE V3 — Stochastic family (12) ──────────────────────────────────
		{NewStochCross5_3_Scalp(), "Mean Reversion", "1m"},
		{NewStochCross9_3_Scalp(), "Mean Reversion", "1m"},
		{NewStochCross14_3_Scalp(), "Mean Reversion", "1m"},
		{NewStochCross14_5_Scalp(), "Mean Reversion", "1m"},
		{NewStochCross21_3_Scalp(), "Mean Reversion", "1m"},
		{NewStochOversold5_Scalp(), "Mean Reversion", "1m"},
		{NewStochOversold9_Scalp(), "Mean Reversion", "1m"},
		{NewStochOversold14_Scalp(), "Mean Reversion", "1m"},
		{NewStochOversold21_Scalp(), "Mean Reversion", "1m"},
		{NewStochTrend9_Scalp(), "Trend", "1m"},
		{NewStochTrend14_Scalp(), "Trend", "1m"},
		{NewStochTrend21_Scalp(), "Trend", "1m"},

		// ── ELITE V3 — ATR signal family (10) ──────────────────────────────────
		{NewATRMom7_14_Scalp(), "Volatility", "1m"},
		{NewATRMom10_20_Scalp(), "Volatility", "1m"},
		{NewATRMom14_20_Scalp(), "Volatility", "1m"},
		{NewATRMom14_50_Scalp(), "Volatility", "1m"},
		{NewATRMom21_50_Scalp(), "Volatility", "5m"},
		{NewATRChan14_20_Scalp(), "Breakout Elite", "1m"},
		{NewATRChan14_50_Scalp(), "Breakout Elite", "1m"},
		{NewATRChan10_20_Scalp(), "Breakout Elite", "1m"},
		{NewATRContr14_20_Scalp(), "Volatility", "1m"},
		{NewATRContr10_20_Scalp(), "Volatility", "1m"},

		// ── ELITE V3 — ROC family (8) ───────────────────────────────────────────
		{NewROC3_0p3_Scalp(), "Momentum Elite", "1m"},
		{NewROC5_0p5_Scalp(), "Momentum Elite", "1m"},
		{NewROC5_1p0_Scalp(), "Momentum Elite", "1m"},
		{NewROC9_0p5_Scalp(), "Momentum Elite", "1m"},
		{NewROC9_1p0_Scalp(), "Momentum Elite", "1m"},
		{NewROC12_0p5_Scalp(), "Momentum Elite", "1m"},
		{NewROC12_1p0_Scalp(), "Momentum Elite", "1m"},
		{NewROC21_1p5_Scalp(), "Momentum Elite", "5m"},

		// ── ELITE V3 — Williams %R family (8) ──────────────────────────────────
		{NewWRBounce7_Scalp(), "Mean Reversion", "1m"},
		{NewWRBounce10_Scalp(), "Mean Reversion", "1m"},
		{NewWRBounce14_Scalp(), "Mean Reversion", "1m"},
		{NewWRBounce21_Scalp(), "Mean Reversion", "1m"},
		{NewWRTrend7_Scalp(), "Trend", "1m"},
		{NewWRTrend10_Scalp(), "Trend", "1m"},
		{NewWRTrend14_Scalp(), "Trend", "1m"},
		{NewWRTrend21_Scalp(), "Trend", "1m"},

		// ── ELITE V3 — Parabolic SAR + EMA family (8) ──────────────────────────
		{NewPsarEMA9_0p02_Scalp(), "Trend", "1m"},
		{NewPsarEMA14_0p02_Scalp(), "Trend", "1m"},
		{NewPsarEMA20_0p02_Scalp(), "Trend", "1m"},
		{NewPsarEMA20_0p03_Scalp(), "Trend", "1m"},
		{NewPsarEMA50_0p02_Scalp(), "Trend", "5m"},
		{NewPsarEMA9_0p01_Scalp(), "Trend", "1m"},
		{NewPsarEMA20_0p01_Scalp(), "Trend", "1m"},
		{NewPsarEMA14_0p03_Scalp(), "Trend", "1m"},

		// ── ELITE V3 — Hull MA family (8) ──────────────────────────────────────
		{NewHullMA7_Scalp(), "Trend", "1m"},
		{NewHullMA9_Scalp(), "Trend", "1m"},
		{NewHullMA14_Scalp(), "Trend", "1m"},
		{NewHullMA20_Scalp(), "Trend", "1m"},
		{NewHullMA25_Scalp(), "Trend", "1m"},
		{NewHullMA30_Scalp(), "Trend", "5m"},
		{NewHullMA40_Scalp(), "Trend", "5m"},
		{NewHullMA50_Scalp(), "Trend", "5m"},

		// ── ELITE V3 — Keltner family (12) ─────────────────────────────────────
		{NewKeltBreak20_14_1p5_Scalp(), "Breakout Elite", "1m"},
		{NewKeltBreak20_14_2_Scalp(), "Breakout Elite", "1m"},
		{NewKeltBreak20_14_2p5_Scalp(), "Breakout Elite", "1m"},
		{NewKeltBreak10_14_1p5_Scalp(), "Breakout Elite", "1m"},
		{NewKeltBreak50_14_2_Scalp(), "Breakout Elite", "5m"},
		{NewKeltBreak20_10_1p5_Scalp(), "Breakout Elite", "1m"},
		{NewKeltBounce20_14_1p5_Scalp(), "Mean Reversion", "1m"},
		{NewKeltBounce20_14_2_Scalp(), "Mean Reversion", "1m"},
		{NewKeltBounce10_14_1p5_Scalp(), "Mean Reversion", "1m"},
		{NewKeltMid20_14_Scalp(), "Trend", "1m"},
		{NewKeltMid10_14_Scalp(), "Trend", "1m"},
		{NewKeltMid50_14_Scalp(), "Trend", "5m"},

		// ── ELITE V3 — Momentum Divergence family (6) ──────────────────────────
		{NewMomDiv14_5_Scalp(), "Price Action Elite", "1m"},
		{NewMomDiv14_8_Scalp(), "Price Action Elite", "1m"},
		{NewMomDiv14_10_Scalp(), "Price Action Elite", "1m"},
		{NewMomDiv9_5_Scalp(), "Price Action Elite", "1m"},
		{NewMomDiv9_8_Scalp(), "Price Action Elite", "1m"},
		{NewMomDiv21_10_Scalp(), "Price Action Elite", "5m"},

		// ── ELITE V3 — Consecutive Candles family (8) ──────────────────────────
		{NewConsec2_ADX18_Scalp(), "Trend", "1m"},
		{NewConsec3_ADX20_Scalp(), "Trend", "1m"},
		{NewConsec3_ADX22_Scalp(), "Trend", "1m"},
		{NewConsec4_ADX20_Scalp(), "Trend", "1m"},
		{NewConsec4_ADX25_Scalp(), "Trend", "1m"},
		{NewConsec5_ADX22_Scalp(), "Trend", "1m"},
		{NewConsec5_ADX28_Scalp(), "Trend", "1m"},
		{NewConsec3_Tight_Scalp(), "Trend", "1m"},

		// ── ELITE V3 — Additional EMA Cross variants (5) ───────────────────────
		{NewEMA2_5CrossScalp(), "Trend", "1m"},
		{NewEMA3_21CrossScalp(), "Trend", "1m"},
		{NewEMA5_50CrossScalp(), "Trend", "5m"},
		{NewEMA10_50CrossScalp(), "Trend", "5m"},
		{NewEMA21_89CrossScalp(), "Trend", "5m"},

		// ── ELITE V3 — Additional RSI variants (5) ─────────────────────────────
		{NewRSIOversold20Scalp(), "Mean Reversion", "1m"},
		{NewRSIOversold28Scalp(), "Mean Reversion", "1m"},
		{NewRSICross45Scalp(), "Mean Reversion", "1m"},
		{NewRSICross60Scalp(), "Mean Reversion", "1m"},
		{NewRSI7Cross50Scalp(), "Mean Reversion", "1m"},

		// ── ELITE V3 — Additional BB variants (5) ──────────────────────────────
		{NewBBBounce40_2Scalp(), "Mean Reversion", "5m"},
		{NewBBBounce14_1p8Scalp(), "Mean Reversion", "1m"},
		{NewBBMidCross30Scalp(), "Trend", "5m"},
		{NewBBBreakout30_2Scalp(), "Breakout Elite", "5m"},
		{NewBBWidth20_2p5Scalp(), "Volatility", "1m"},

		// ── ELITE V3 — Additional VWAP variants (5) ────────────────────────────
		{NewVWAPCross40Scalp(), "Trend", "1m"},
		{NewVWAPDev0p6Scalp(), "Mean Rev Elite", "1m"},
		{NewVWAPPullback40Scalp(), "Trend", "1m"},
		{NewVWAPCross60Scalp(), "Trend", "5m"},
		{NewVWAPDev0p15Scalp(), "Mean Rev Elite", "1m"},

		// ── ELITE V3 — Additional N-bar variants (5) ───────────────────────────
		{NewNBar4Break(), "Breakout Elite", "1m"},
		{NewNBar6Break(), "Breakout Elite", "1m"},
		{NewNBar14Break(), "Breakout Elite", "5m"},
		{NewNBar18Break(), "Breakout Elite", "5m"},
		{NewNBar40Break(), "Breakout Elite", "5m"},

		// ── INTRADAY — EMA Cross 5m/15m (10) ───────────────────────────────────
		{NewID_EMA5_20_5m(), "Intraday", "5m"},
		{NewID_EMA8_21_5m(), "Intraday", "5m"},
		{NewID_EMA9_50_5m(), "Intraday", "5m"},
		{NewID_EMA12_26_5m(), "Intraday", "5m"},
		{NewID_EMA20_50_5m(), "Intraday", "5m"},
		{NewID_EMA21_55_5m(), "Intraday", "5m"},
		{NewID_EMA50_200_5m(), "Intraday", "15m"},
		{NewID_EMA20_100_5m(), "Intraday", "15m"},
		{NewID_EMA10_30_15m(), "Intraday", "15m"},
		{NewID_EMA13_34_15m(), "Intraday", "15m"},

		// ── INTRADAY — Triple EMA (8) ───────────────────────────────────────────
		{NewID_Triple5_13_34_5m(), "Intraday", "5m"},
		{NewID_Triple8_21_55_5m(), "Intraday", "5m"},
		{NewID_Triple10_30_60_5m(), "Intraday", "5m"},
		{NewID_Triple20_50_100_5m(), "Intraday", "15m"},
		{NewID_Triple5_10_20_5m(), "Intraday", "5m"},
		{NewID_Triple9_18_36_5m(), "Intraday", "5m"},
		{NewID_Triple10_20_40_15m(), "Intraday", "15m"},
		{NewID_Triple20_50_200_15m(), "Intraday", "15m"},

		// ── INTRADAY — MACD (8) ────────────────────────────────────────────────
		{NewID_MACDCross12_26_9_5m(), "Intraday", "5m"},
		{NewID_MACDZero12_26_5m(), "Intraday", "5m"},
		{NewID_MACDHistMom12_26_5m(), "Intraday", "5m"},
		{NewID_MACDCross26_52_9_5m(), "Intraday", "15m"},
		{NewID_MACDCross5_35_5_5m(), "Intraday", "5m"},
		{NewID_MACDCross12_26_9_15m(), "Intraday", "15m"},
		{NewID_MACDZero12_26_15m(), "Intraday", "15m"},
		{NewID_MACDHistMom5_13_5m(), "Intraday", "5m"},

		// ── INTRADAY — VWAP (6) ────────────────────────────────────────────────
		{NewID_VWAPCross50_5m(), "Intraday", "5m"},
		{NewID_VWAPCross100_5m(), "Intraday", "15m"},
		{NewID_VWAPDev0p5_5m(), "Intraday", "5m"},
		{NewID_VWAPDev1p0_5m(), "Intraday", "15m"},
		{NewID_VWAPPullback50_5m(), "Intraday", "5m"},
		{NewID_VWAPPullback100_5m(), "Intraday", "15m"},

		// ── INTRADAY — Bollinger Bands (8) ─────────────────────────────────────
		{NewID_BBBounce20_2_5m(), "Intraday", "5m"},
		{NewID_BBBounce50_2_5m(), "Intraday", "15m"},
		{NewID_BBMidCross20_5m(), "Intraday", "5m"},
		{NewID_BBMidCross50_5m(), "Intraday", "15m"},
		{NewID_BBBreakout20_2_5m(), "Intraday", "5m"},
		{NewID_BBBreakout50_2_5m(), "Intraday", "15m"},
		{NewID_BBWidth20_2_5m(), "Intraday", "5m"},
		{NewID_BBWidth50_2_5m(), "Intraday", "15m"},

		// ── INTRADAY — RSI (5) ─────────────────────────────────────────────────
		{NewID_RSIOversold30_5m(), "Intraday", "5m"},
		{NewID_RSICross50_5m(), "Intraday", "5m"},
		{NewID_RSICross55_5m(), "Intraday", "5m"},
		{NewID_RSI21_5m(), "Intraday", "15m"},
		{NewID_RSIOversold20_5m(), "Intraday", "5m"},

		// ── INTRADAY — Keltner (5) ─────────────────────────────────────────────
		{NewID_KeltBreak20_2_5m(), "Intraday", "5m"},
		{NewID_KeltBreak50_2_5m(), "Intraday", "15m"},
		{NewID_KeltBounce20_2_5m(), "Intraday", "5m"},
		{NewID_KeltMid20_5m(), "Intraday", "5m"},
		{NewID_KeltMid50_5m(), "Intraday", "15m"},

		// ── INTRADAY — Stochastic (5) ──────────────────────────────────────────
		{NewID_StochCross14_3_5m(), "Intraday", "5m"},
		{NewID_StochCross14_5_5m(), "Intraday", "5m"},
		{NewID_StochOversold14_5m(), "Intraday", "5m"},
		{NewID_StochTrend14_5m(), "Intraday", "5m"},
		{NewID_StochCross21_5_5m(), "Intraday", "15m"},

		// ── INTRADAY — Hull MA (5) ─────────────────────────────────────────────
		{NewID_HullMA20_5m(), "Intraday", "5m"},
		{NewID_HullMA30_5m(), "Intraday", "5m"},
		{NewID_HullMA50_5m(), "Intraday", "15m"},
		{NewID_HullMA100_5m(), "Intraday", "15m"},
		{NewID_HullMA14_5m(), "Intraday", "5m"},

		// ── INTRADAY — CCI (5) ─────────────────────────────────────────────────
		{NewID_CCIZeroCross20_5m(), "Intraday", "5m"},
		{NewID_CCIExtreme20_5m(), "Intraday", "5m"},
		{NewID_CCITrend20_5m(), "Intraday", "5m"},
		{NewID_CCIZeroCross50_5m(), "Intraday", "15m"},
		{NewID_CCITrend50_5m(), "Intraday", "15m"},
	}
}

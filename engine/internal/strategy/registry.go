package strategy

// =============================================================================
// STRATEGY REGISTRY — ALL 40 SCALPING STRATEGIES
// =============================================================================

type RegistryEntry struct {
	Strategy  Strategy
	Category  string
	Timeframe string
}

func BuildAllScalpers() []RegistryEntry {
	return []RegistryEntry{
		// ══════════ TEST ══════════
		{NewTestScalper(), "Test", "tick"},

		// ══════════ ORIGINAL 20 ══════════
		{NewEMACrossScalper(8, 21), "Trend", "1m"},
		{NewTripleEMAScalper(5, 13, 34), "Trend", "1m"},
		{NewHullMAScalper(16), "Trend", "1m"},
		{NewADXTrendScalper(), "Trend", "1m"},
		{NewIchimokuScalper(), "Trend", "5m"},
		{NewParabolicSARScalper(), "Trend", "1m"},
		{NewRSIReversalScalper(14), "Mean Reversion", "1m"},
		{NewBollingerScalper(20, 2.0), "Mean Reversion", "1m"},
		{NewVWAPScalper(50, 0.15), "Mean Reversion", "1m"},
		{NewMeanReversionScalper(30, 2.0), "Mean Reversion", "1m"},
		{NewStochRSIScalper(), "Mean Reversion", "1m"},
		{NewWilliamsRScalper(14), "Mean Reversion", "1m"},
		{NewCCIScalper(20), "Mean Reversion", "1m"},
		{NewMomentumScalper(10, 0.3), "Breakout", "1m"},
		{NewDonchianScalper(20), "Breakout", "5m"},
		{NewKeltnerScalper(), "Breakout", "1m"},
		{NewPivotScalper(60), "Breakout", "1h"},
		{NewMACDScalper(), "Momentum", "1m"},
		{NewROCReversalScalper(12, 0.5), "Momentum", "1m"},
		{NewOrderFlowScalper(100, 0.65), "Microstructure", "tick"},

		// ══════════ ADVANCED 20 ══════════
		{NewTickVelocityScalper(20, 5.0), "Velocity", "tick"},
		{NewVolumeSpikeScalper(50), "Velocity", "tick"},
		{NewGapFillScalper(0.2), "Velocity", "tick"},
		{NewFibonacciScalper(50), "Statistical", "1m"},
		{NewLinRegScalper(30, 2.0), "Statistical", "1m"},
		{NewEMASpreadScalper(8, 21, 50), "Statistical", "1m"},
		{NewConsensusScalper(), "Statistical", "1m"},
		{NewVolatilitySqueeze(), "Volatility", "1m"},
		{NewRangeCompressionScalper(20, 0.3), "Volatility", "5m"},
		{NewVWAPRSI2ReversionScalper(), "Mean Rev Elite", "1m"},
		{NewBollingerRSIFadeScalper(), "Mean Rev Elite", "1m"},
		{NewMACDVWAPFlipScalper(), "Momentum Elite", "1m"},
		{NewStochasticRangeScalper(), "Mean Reversion", "1m"},
		{NewATRVolumeImpulseScalper(), "Breakout Elite", "1m"},
		{NewOpeningRangeBreakoutScalper(16, 0, 15), "Time-of-Day", "1m"},
		{NewOBVScalper(20), "Smart Money", "1m"},
		{NewChaikinMFScalper(20), "Smart Money", "1m"},
		{NewADLineScalper(20), "Smart Money", "1m"},
		{NewAroonScalper(14), "Smart Money", "1m"},
		{NewEngulfingScalper(), "Price Action", "1m"},
		{NewHeikinAshiScalper(), "Price Action", "1m"},
		{NewZigZagScalper(0.5), "Price Action", "1m"},
		{NewMicroPullbackScalper(), "Price Action", "1m"},
		{NewSupertrendScalper(10, 3.0), "Adaptive", "1m"},
		{NewKAMAScalper(10), "Adaptive", "1m"},
		{NewMTFRSIScalper(), "Adaptive", "1m"},

		// ══════════ ELITE 50 (41-90) ══════════
		// Trend Elite
		{NewDEMACrossScalper(8, 21), "Trend Elite", "1m"},
		{NewVortexScalper(14), "Trend Elite", "1m"},
		{NewDMIScalper(14), "Trend Elite", "1m"},
		{NewMARibbonScalper(), "Trend Elite", "1m"},
		{NewTIIScalper(30), "Trend Elite", "1m"},
		// Mean Reversion Elite
		{NewRSIDivergenceScalper(14), "Mean Rev Elite", "1m"},
		{NewZScoreBandScalper(30, 2.0), "Mean Rev Elite", "1m"},
		{NewRSIBBScalper(), "Mean Rev Elite", "1m"},
		{NewAnchoredVWAPScalper(50), "Mean Rev Elite", "1m"},
		{NewDoublePatternScalper(40, 0.01), "Mean Rev Elite", "1m"},
		// Breakout Elite
		{NewATRBreakoutScalper(14, 1.5), "Breakout Elite", "1m"},
		{NewSqueezeMomentumScalper(20), "Breakout Elite", "1m"},
		{NewPriceChannelScalper(20), "Breakout Elite", "5m"},
		{NewFractalScalper(), "Breakout Elite", "1m"},
		{NewAccelBandScalper(20), "Breakout Elite", "1m"},
		{NewVolRatioScalper(5, 20), "Breakout Elite", "1m"},
		{NewVCPScalper(), "Breakout Elite", "1m"},
		// Momentum Elite
		{NewTRIXScalper(15), "Momentum Elite", "1m"},
		{NewCoppockScalper(), "Momentum Elite", "1m"},
		{NewKSTScalper(), "Momentum Elite", "1m"},
		{NewPPOScalper(12, 26), "Momentum Elite", "1m"},
		{NewROCAccelScalper(10), "Momentum Elite", "1m"},
		{NewMomDivScalper(10), "Momentum Elite", "1m"},
		{NewMomentumRSIScalper(10, 14), "Momentum Elite", "1m"},
		// Oscillator Elite
		{NewFisherTransformScalper(10), "Oscillator Elite", "1m"},
		{NewConnorsRSIScalper(), "Oscillator Elite", "1m"},
		{NewChandeMOScalper(14), "Oscillator Elite", "1m"},
		{NewDPOScalper(20), "Oscillator Elite", "1m"},
		{NewUltimateOscScalper(), "Oscillator Elite", "1m"},
		{NewStochasticCrossScalper(14), "Oscillator Elite", "1m"},
		{NewSchaffScalper(), "Oscillator Elite", "1m"},
		{NewSmoothedRSIScalper(14), "Oscillator Elite", "1m"},
		{NewTripleRSIScalper(), "Oscillator Elite", "1m"},
		{NewRVIScalper(14), "Oscillator Elite", "1m"},
		// Volume Elite
		{NewKlingerScalper(), "Volume Elite", "1m"},
		{NewMassIndexScalper(25), "Volume Elite", "1m"},
		// Multi-Signal Elite
		{NewTripleFilterScalper(), "Multi-Signal", "1m"},
		{NewSentimentConfluenceProScalper(), "Multi-Signal", "1m"},
		{NewElderRayScalper(13), "Multi-Signal", "1m"},
		{NewMultiMAScalper(), "Multi-Signal", "1m"},
		{NewQuadSniperScalper(), "Multi-Signal", "1m"},
		// Volatility Elite
		{NewMomSqueezeScalper(), "Volatility Elite", "1m"},
		{NewNATRScalper(14, 0.5), "Volatility Elite", "1m"},
		// Price Action Elite
		{NewPinbarScalper(), "Price Action Elite", "1m"},
		{NewExhaustionScalper(10), "Price Action Elite", "1m"},
		{NewRoundNumberScalper(0.1), "Price Action Elite", "1m"},
		{NewPivotFibScalper(60), "Price Action Elite", "1h"},
		{NewChandelierScalper(22, 3.0), "Price Action Elite", "1m"},
		{NewWCOScalper(10, 30), "Price Action Elite", "1m"},
		{NewAdaptiveRSIScalper(14), "Adaptive Elite", "1m"},
		{NewIMIScalper(14), "Adaptive Elite", "1m"},
	}
}

// StrategyGroups organizes strategies by their intended processing timeframe.
type StrategyGroups struct {
	Tick []RegistryEntry // Processed on every raw tick
	M1   []RegistryEntry // Processed on 1-minute candle close
	M5   []RegistryEntry // Processed on 5-minute candle close
	H1   []RegistryEntry // Processed on simulated hourly (every 12th 5m candle)
}

// GroupByTimeframe separates strategies into processing groups.
// This is critical: candle-based strategies MUST only receive candle data,
// not raw ticks, or their indicators will be meaningless.
func GroupByTimeframe(entries []RegistryEntry) StrategyGroups {
	var groups StrategyGroups
	for _, e := range entries {
		switch e.Timeframe {
		case "tick":
			groups.Tick = append(groups.Tick, e)
		case "5m":
			groups.M5 = append(groups.M5, e)
		case "1h":
			groups.H1 = append(groups.H1, e)
		default: // "1m" and anything else
			groups.M1 = append(groups.M1, e)
		}
	}
	return groups
}

func GetStrategyNames() []string {
	entries := BuildAllScalpers()
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Strategy.Name()
	}
	return names
}

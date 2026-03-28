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
		// Trend
		{NewEMACrossScalper(8, 21), "Trend", "1m"},
		{NewTripleEMAScalper(5, 13, 34), "Trend", "1m"},
		{NewHullMAScalper(16), "Trend", "1m"},
		{NewADXTrendScalper(), "Trend", "1m"},
		{NewIchimokuScalper(), "Trend", "5m"},
		{NewParabolicSARScalper(), "Trend", "1m"},
		// Mean Reversion
		{NewRSIReversalScalper(14), "Mean Reversion", "1m"},
		{NewBollingerScalper(20, 2.0), "Mean Reversion", "1m"},
		{NewVWAPScalper(50, 0.15), "Mean Reversion", "1m"},
		{NewMeanReversionScalper(30, 2.0), "Mean Reversion", "1m"},
		{NewStochRSIScalper(), "Mean Reversion", "1m"},
		{NewWilliamsRScalper(14), "Mean Reversion", "1m"},
		{NewCCIScalper(20), "Mean Reversion", "1m"},
		// Breakout
		{NewMomentumScalper(10, 0.3), "Breakout", "1m"},
		{NewDonchianScalper(20), "Breakout", "5m"},
		{NewKeltnerScalper(), "Breakout", "1m"},
		{NewPivotScalper(60), "Breakout", "1h"},
		// Momentum
		{NewMACDScalper(), "Momentum", "1m"},
		{NewROCReversalScalper(12, 0.5), "Momentum", "1m"},
		// Microstructure
		{NewOrderFlowScalper(100, 0.65), "Microstructure", "tick"},

		// ══════════ ADVANCED 20 (HIGH ALPHA) ══════════
		// Velocity & Microstructure
		{NewTickVelocityScalper(20, 5.0), "Velocity", "tick"},
		{NewVolumeSpikeScalper(50), "Velocity", "tick"},
		{NewGapFillScalper(0.2), "Velocity", "tick"},
		// Statistical Edge
		{NewFibonacciScalper(50), "Statistical", "1m"},
		{NewLinRegScalper(30, 2.0), "Statistical", "1m"},
		{NewEMASpreadScalper(8, 21, 50), "Statistical", "1m"},
		{NewConsensusScalper(), "Statistical", "1m"},
		// Volatility
		{NewVolatilitySqueeze(), "Volatility", "1m"},
		{NewRangeCompressionScalper(20, 0.3), "Volatility", "5m"},
		// Smart Money
		{NewOBVScalper(20), "Smart Money", "1m"},
		{NewChaikinMFScalper(20), "Smart Money", "1m"},
		{NewADLineScalper(20), "Smart Money", "1m"},
		{NewAroonScalper(14), "Smart Money", "1m"},
		// Price Action
		{NewEngulfingScalper(), "Price Action", "1m"},
		{NewHeikinAshiScalper(), "Price Action", "1m"},
		{NewZigZagScalper(0.5), "Price Action", "1m"},
		{NewMicroPullbackScalper(), "Price Action", "1m"},
		// Adaptive
		{NewSupertrendScalper(10, 3.0), "Adaptive", "1m"},
		{NewKAMAScalper(10), "Adaptive", "1m"},
		{NewMTFRSIScalper(), "Adaptive", "1m"},
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

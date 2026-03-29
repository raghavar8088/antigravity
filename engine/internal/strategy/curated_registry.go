package strategy

// BuildCuratedScalpers returns the smaller live strategy pack.
// The broader catalog remains available for experimentation and backtesting,
// but only this shortlist is considered stable enough for runtime execution.
func BuildCuratedScalpers() []RegistryEntry {
	return []RegistryEntry{
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
		{NewExhaustionScalper(10), "Price Action Elite", "1m"},
		{NewChartDoubleTapReversalScalper(), "Price Action Elite", "1m"},
		{NewChartWedgeBreakoutScalper(), "Price Action Elite", "5m"},
		{NewAdaptiveRSIScalper(14), "Adaptive Elite", "1m"},
		{NewKAMAScalper(10), "Adaptive", "1m"},
	}
}

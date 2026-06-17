package scalpers

// buildExpansionPack returns additional strategies for the curated registry,
// merged into BuildCuratedScalpers() alongside BuildAllScalpers().
func buildExpansionPack() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     &SMCOrderBlockFVG{},
			Name:         "SMC_OrderBlock_FVG",
			Description:  "SMC order-block + FVG retest with MSS confirmation",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"15m", "5m"},
			MaxPositions: 1,
		},
	}
}

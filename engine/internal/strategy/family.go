package strategy

import riskv2 "antigravity-engine/internal/risk/v2"

// CategoryFamily maps a strategy category string to the riskv2.StrategyFamily
// type used by the Risk V2 engine for family concentration checks.
//
// This is the authoritative mapping. Edit here to reclassify a category — do
// not duplicate the mapping elsewhere. FamilyReserve is only valid for
// strategies that genuinely have no family (administrative entries).
//
// Risk V2 uses this for:
//   - Family allocation caps (MaxFamilyAllocationPct: 30%)
//   - Family exposure tracking (EvaluateFamilyRisk)
//   - Correlation cluster analysis within families
var categoryFamilyMap = map[string]riskv2.StrategyFamily{
	"Trend":              riskv2.FamilyTrend,
	"Trend Elite":        riskv2.FamilyTrend,
	"Breakout":           riskv2.FamilyBreakout,
	"Breakout Elite":     riskv2.FamilyBreakout,
	"Mean Reversion":     riskv2.FamilyMeanReversion,
	"Mean Rev Elite":     riskv2.FamilyMeanReversion,
	"Momentum":           riskv2.FamilyTrend, // Momentum is directional — closest to Trend
	"Momentum Elite":     riskv2.FamilyTrend,
	"Oscillator Elite":   riskv2.FamilyMeanReversion,
	"Volume Elite":       riskv2.FamilyVolumeProfile,
	"Microstructure":     riskv2.FamilyOrderFlow,
	"Velocity":           riskv2.FamilyOrderFlow,
	"Statistical":        riskv2.FamilyMeanReversion,
	"Volatility":         riskv2.FamilyMeanReversion, // Volatility mean-reverts
	"Volatility Elite":   riskv2.FamilyMeanReversion,
	"Time-of-Day":        riskv2.FamilyTrend, // Session-based directional bias
	"Smart Money":        riskv2.FamilySmartMoney,
	"Price Action":       riskv2.FamilySmartMoney,
	"Price Action Elite": riskv2.FamilySmartMoney,
	"Adaptive":           riskv2.FamilyTrend,
	"Adaptive Elite":     riskv2.FamilyTrend,
	"Multi-Signal":       riskv2.FamilyMSS, // Multi-strategy synthesis
	"Intraday":           riskv2.FamilyTrend,
	"Test":               riskv2.FamilyReserve,
	"Unknown":            riskv2.FamilyReserve,
}

// CategoryToRiskFamily returns the riskv2.StrategyFamily for a given category.
// Unknown categories fall back to FamilyReserve (uncapped bucket).
func CategoryToRiskFamily(category string) riskv2.StrategyFamily {
	if f, ok := categoryFamilyMap[category]; ok {
		return f
	}
	return riskv2.FamilyReserve
}

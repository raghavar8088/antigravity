package strategy

// eliteCategories is the authoritative set of category strings that qualify
// as StrategyTierElite. These are the categories explicitly designated "Elite"
// in the curated strategy registry and have earned higher confidence weights in
// the SignalAggregator scoring system.
var eliteCategories = map[string]struct{}{
	"Mean Rev Elite":     {},
	"Trend Elite":        {},
	"Breakout Elite":     {},
	"Momentum Elite":     {},
	"Oscillator Elite":   {},
	"Volume Elite":       {},
	"Volatility Elite":   {},
	"Price Action Elite": {},
	"Adaptive Elite":     {},
}

// CategoryTier returns the StrategyTier for a given category string.
// This is the single authoritative source for tier classification — use
// this function everywhere rather than duplicating category checks.
func CategoryTier(category string) StrategyTier {
	if _, ok := eliteCategories[category]; ok {
		return StrategyTierElite
	}
	// Multi-Signal strategies aggregate multiple elite signals and are treated
	// as elite for drawdown-regime purposes.
	if category == "Multi-Signal" {
		return StrategyTierElite
	}
	if category == "Test" {
		return StrategyTierExperimental
	}
	return StrategyTierStandard
}

// IsElite is a convenience wrapper for the common drawdown-regime check.
func IsElite(category string) bool {
	return CategoryTier(category) == StrategyTierElite
}

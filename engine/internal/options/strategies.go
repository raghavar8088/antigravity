package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.020
)

var strategyIDs = map[string]int{}

// BuildStrategies returns no option strategies.
func BuildStrategies() []StrategyDef {
	return nil
}

// buildAllStrategies returns no base option strategies.
func buildAllStrategies() []StrategyDef {
	return nil
}

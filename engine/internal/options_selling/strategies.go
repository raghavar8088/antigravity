package options_selling

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.020
)

var strategyIDs = map[string]int{}

// BuildStrategies returns no option-selling strategies.
func BuildStrategies() []StrategyDef {
	return nil
}

// buildAllStrategies returns no base option-selling strategies.
func buildAllStrategies() []StrategyDef {
	return nil
}

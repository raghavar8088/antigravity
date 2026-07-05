package regime

// StrategyRegistry provides strategy names to the gate for allowlist construction.
type StrategyRegistry interface {
	Names() []string
}

// StrategyGate filters which strategies are eligible to trade given the current
// market regime, and provides position-size multipliers per regime.
//
// It is populated at engine startup from the curated strategy registry and
// updated whenever the regime classifier emits a new regime signal.
type StrategyGate struct {
	classifier     *Classifier
	registry       StrategyRegistry
	// perRegime maps a regime string to the set of allowed strategy names.
	perRegime map[string][]string
	// sizeMultiplier maps a regime string to a position-size scaling factor.
	sizeMultiplier map[string]float64
}

// NewStrategyGate constructs a gate wired to the regime classifier and strategy registry.
func NewStrategyGate(classifier *Classifier, registry StrategyRegistry) *StrategyGate {
	return &StrategyGate{
		classifier:     classifier,
		registry:       registry,
		perRegime:      make(map[string][]string),
		sizeMultiplier: make(map[string]float64),
	}
}

// SetRegimeStrategies replaces the allowed strategy list for the given regime.
func (g *StrategyGate) SetRegimeStrategies(regime string, names []string) {
	g.perRegime[regime] = names
}

// SetSizeMultiplier sets the position-size multiplier for the given regime.
func (g *StrategyGate) SetSizeMultiplier(regime string, mult float64) {
	g.sizeMultiplier[regime] = mult
}

// GetActiveStrategies returns the names of strategies allowed in the given regime.
// Returns nil (all strategies pass) if no explicit list is configured for the regime.
func (g *StrategyGate) GetActiveStrategies(rc RegimeClassification) []string {
	return g.perRegime[string(rc.Regime)]
}

// GetPositionSizeMultiplier returns the position-size scaling factor for the regime.
// Returns 1.0 if no multiplier is configured.
func (g *StrategyGate) GetPositionSizeMultiplier(rc RegimeClassification) float64 {
	if rc.PositionSizeMult > 0 {
		return rc.PositionSizeMult
	}
	if m, ok := g.sizeMultiplier[string(rc.Regime)]; ok {
		return m
	}
	return 1.0
}

// IsStrategyAllowed returns true if the strategy is permitted to trade in the given regime.
// A classification with AllowNewEntries=false suspends every strategy — the same
// rule the trading loop enforces by skipping the cycle; encoded here too so any
// caller of the gate gets consistent policy. When no explicit allowlist is set
// for the regime, all strategies are permitted.
func (g *StrategyGate) IsStrategyAllowed(strategyName string, rc RegimeClassification) bool {
	if !rc.AllowNewEntries {
		return false
	}
	allowed := g.perRegime[string(rc.Regime)]
	if len(allowed) == 0 {
		// No explicit list — allow all strategies in this regime.
		return true
	}
	for _, name := range allowed {
		if name == strategyName {
			return true
		}
	}
	return false
}

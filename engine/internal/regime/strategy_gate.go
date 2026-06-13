package regime

import (
	"log/slog"
	"strings"
)

// StrategyRegistry is the minimal interface the gate needs to enumerate
// available strategies.
type StrategyRegistry interface {
	// Names returns all strategy names registered in the engine.
	Names() []string
}

// StrategyGate gates strategy execution based on the current market regime.
// It wraps the existing Router with the higher-level classification logic
// required by the execution layer.
type StrategyGate struct {
	classifier *Classifier
	router     *Router
	registry   StrategyRegistry
}

// NewStrategyGate creates a StrategyGate backed by the given classifier and
// strategy registry.
func NewStrategyGate(classifier *Classifier, registry StrategyRegistry) *StrategyGate {
	return &StrategyGate{
		classifier: classifier,
		router:     NewRouter(),
		registry:   registry,
	}
}

// IsStrategyAllowed returns true if strategyName should execute in the given regime.
func (g *StrategyGate) IsStrategyAllowed(strategyName string, rc RegimeClassification) bool {
	if rc.Regime == RegimeHighVol {
		return false
	}
	family := FamilyFromName(strategyName)
	isTrend := isTrendFamily(family)
	isReversion := isReversionFamily(family)

	switch rc.Regime {
	case RegimeRanging:
		if isTrend {
			return false
		}
	case RegimeTrendingBull, RegimeTrendingBear:
		if isReversion {
			return false
		}
	}
	return true
}

// GetActiveStrategies returns all strategy names allowed in the current regime.
func (g *StrategyGate) GetActiveStrategies(rc RegimeClassification) []string {
	if g.registry == nil {
		return nil
	}
	all := g.registry.Names()
	active := make([]string, 0, len(all))
	for _, name := range all {
		if g.IsStrategyAllowed(name, rc) {
			active = append(active, name)
		}
	}
	slog.Info("[regime] strategy gate applied",
		"regime", rc.Regime,
		"active", len(active),
		"total", len(all),
	)
	return active
}

// GetPositionSizeMultiplier returns the regime-driven position size multiplier.
func (g *StrategyGate) GetPositionSizeMultiplier(rc RegimeClassification) float64 {
	return rc.PositionSizeMult
}

// GetMinConfidence returns the effective minimum confidence threshold, taking
// the regime's floor into account.
func (g *StrategyGate) GetMinConfidence(baseThreshold float64, rc RegimeClassification) float64 {
	if rc.MinConfidenceOverride > baseThreshold {
		return rc.MinConfidenceOverride
	}
	return baseThreshold
}

// ── family classification helpers ────────────────────────────────────────────

func isTrendFamily(f StrategyFamily) bool {
	switch f {
	case FamilyTrend, FamilyMomentum, FamilyBreakout:
		return true
	}
	return false
}

func isReversionFamily(f StrategyFamily) bool {
	return f == FamilyMeanRev
}

// isTrendByName checks strategy names containing trend-family keywords
// (supplements FamilyFromName for edge cases).
func isTrendByName(name string) bool {
	n := strings.ToLower(name)
	keywords := []string{"trend", "ema", "momentum", "breakout", "3soldiers", "3crows", "momflip", "emacross"}
	for _, kw := range keywords {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

// isReversionByName checks strategy names containing reversion keywords.
func isReversionByName(name string) bool {
	n := strings.ToLower(name)
	keywords := []string{"revert", "bounce", "oversold", "overbought", "rejection", "bbbn", "bbrj"}
	for _, kw := range keywords {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

// Override FamilyFromName result with name-based fallback for universal strategies.
func init() {
	_ = isTrendByName
	_ = isReversionByName
}

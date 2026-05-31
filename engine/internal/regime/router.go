// Package regime — RegimeStrategyRouter activates and deactivates strategy
// families based on the current market regime.
//
// This replaces the Macro Analyst AI agent's "should_trade" logic with a
// deterministic lookup table that runs in nanoseconds.
package regime

import "strings"

// StrategyFamily groups strategies by trading style.
type StrategyFamily string

const (
	FamilyTrend        StrategyFamily = "TREND"
	FamilyMomentum     StrategyFamily = "MOMENTUM"
	FamilyBreakout     StrategyFamily = "BREAKOUT"
	FamilyMeanRev      StrategyFamily = "MEAN_REVERSION"
	FamilyScalp        StrategyFamily = "SCALP"
	FamilyVolatility   StrategyFamily = "VOLATILITY"
	FamilyOrderFlow    StrategyFamily = "ORDER_FLOW"
	FamilyPriceAction  StrategyFamily = "PRICE_ACTION"
	FamilySmartMoney   StrategyFamily = "SMART_MONEY"
	FamilyComposite    StrategyFamily = "COMPOSITE"
)

// RoutingDecision carries the activation map for one regime.
type RoutingDecision struct {
	Regime       Regime
	Active       []StrategyFamily // families that should generate signals
	Inactive     []StrategyFamily // families that must be suppressed
	SizeMultiplier float64        // portfolio-wide position size modifier
}

// Router maps regimes to strategy family activation rules.
type Router struct {
	rules map[Regime]RoutingDecision
}

// NewRouter returns a Router pre-loaded with production routing rules.
func NewRouter() *Router {
	r := &Router{rules: make(map[Regime]RoutingDecision)}

	r.rules[RegimeTrendingBull] = RoutingDecision{
		Regime:         RegimeTrendingBull,
		Active:         []StrategyFamily{FamilyTrend, FamilyMomentum, FamilyBreakout, FamilyOrderFlow, FamilyPriceAction, FamilyComposite},
		Inactive:       []StrategyFamily{FamilyMeanRev},
		SizeMultiplier: 1.0,
	}

	r.rules[RegimeTrendingBear] = RoutingDecision{
		Regime:         RegimeTrendingBear,
		Active:         []StrategyFamily{FamilyTrend, FamilyMomentum, FamilyBreakout, FamilyOrderFlow, FamilyPriceAction, FamilyComposite},
		Inactive:       []StrategyFamily{FamilyMeanRev},
		SizeMultiplier: 1.0,
	}

	r.rules[RegimeRanging] = RoutingDecision{
		Regime:         RegimeRanging,
		Active:         []StrategyFamily{FamilyMeanRev, FamilyScalp, FamilyOrderFlow, FamilySmartMoney},
		Inactive:       []StrategyFamily{FamilyTrend, FamilyMomentum, FamilyBreakout},
		SizeMultiplier: 0.75, // reduce size in chop
	}

	r.rules[RegimeHighVol] = RoutingDecision{
		Regime:         RegimeHighVol,
		Active:         []StrategyFamily{FamilyScalp, FamilyVolatility, FamilyOrderFlow},
		Inactive:       []StrategyFamily{FamilyTrend, FamilyMeanRev, FamilyBreakout, FamilyMomentum},
		SizeMultiplier: 0.50, // halve size in high vol
	}

	r.rules[RegimeLowVol] = RoutingDecision{
		Regime:         RegimeLowVol,
		Active:         []StrategyFamily{FamilyMeanRev, FamilyScalp, FamilyOrderFlow},
		Inactive:       []StrategyFamily{FamilyBreakout, FamilyMomentum, FamilyVolatility},
		SizeMultiplier: 0.60,
	}

	r.rules[RegimeUnknown] = RoutingDecision{
		Regime:         RegimeUnknown,
		Active:         []StrategyFamily{},
		Inactive:       []StrategyFamily{FamilyTrend, FamilyMeanRev, FamilyBreakout, FamilyMomentum, FamilyScalp, FamilyVolatility, FamilyOrderFlow, FamilyPriceAction, FamilySmartMoney, FamilyComposite},
		SizeMultiplier: 0.0, // no trading in unknown regime
	}

	return r
}

// Decision returns the routing decision for the given regime.
func (r *Router) Decision(regime Regime) RoutingDecision {
	if d, ok := r.rules[regime]; ok {
		return d
	}
	return r.rules[RegimeUnknown]
}

// IsActive returns true if the given strategy family should generate signals
// in the current regime.
func (r *Router) IsActive(regime Regime, family StrategyFamily) bool {
	d := r.Decision(regime)
	for _, f := range d.Active {
		if f == family {
			return true
		}
	}
	return false
}

// SizeMultiplier returns the regime-based position size multiplier.
func (r *Router) SizeMultiplier(regime Regime) float64 {
	return r.Decision(regime).SizeMultiplier
}

// FamilyFromName infers the strategy family from the strategy name string.
// This is used when strategy struct does not carry an explicit Family field.
func FamilyFromName(name string) StrategyFamily {
	n := strings.ToUpper(name)
	switch {
	case contains(n, "EMA", "HULL", "TRIPLE", "DEMA", "KAMA", "SUPERTREND", "SAR", "ICHIMOKU", "MA_RIBBON", "MULTI_MA"):
		return FamilyTrend
	case contains(n, "VWAP", "BOLLINGER", "RSI_REVERSAL", "MEAN_REVERSION", "ZSCORE", "WILLIAMS", "CCI", "STOCH"):
		return FamilyMeanRev
	case contains(n, "DONCHIAN", "KELTNER", "PIVOT", "OPENING_RANGE", "ORB", "BREAKOUT", "ATR_BREAKOUT", "FRACTAL"):
		return FamilyBreakout
	case contains(n, "MACD", "MOMENTUM", "ROC", "PPO", "TRIX", "KST", "COPPOCK"):
		return FamilyMomentum
	case contains(n, "TICK", "ORDER_FLOW", "OBV", "CHAIKIN", "KLINGER", "VOLUME_SPIKE"):
		return FamilyOrderFlow
	case contains(n, "SQUEEZE", "VOLATILITY", "NATR", "ACCEL"):
		return FamilyVolatility
	case contains(n, "ENGULF", "HEIKIN", "ZIGZAG", "PULLBACK", "PINBAR", "EXHAUSTION"):
		return FamilyPriceAction
	case contains(n, "AD_LINE", "AROON", "ELDER_RAY", "VORTEX", "DMI"):
		return FamilySmartMoney
	case contains(n, "CONSENSUS", "TRIPLE_FILTER", "QUAD", "SENTIMENT", "COMPOSITE", "MULTI"):
		return FamilyComposite
	default:
		return FamilyScalp
	}
}

func contains(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

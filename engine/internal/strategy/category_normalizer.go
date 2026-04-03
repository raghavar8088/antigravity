package strategy

import "strings"

// NormalizeCategory maps broad registry buckets into execution-aware families.
// The "Intraday" pack includes multiple sub-families that should be routed and
// filtered like their real strategy style instead of as one coarse bucket.
func NormalizeCategory(category, strategyName string) string {
	if category != "Intraday" {
		return category
	}

	switch {
	case strings.HasPrefix(strategyName, "ID_EMA"),
		strings.HasPrefix(strategyName, "ID_Triple"),
		strings.HasPrefix(strategyName, "ID_Hull"),
		strings.HasPrefix(strategyName, "ID_VWAP_Cross"),
		strings.HasPrefix(strategyName, "ID_VWAP_Pullback"),
		strings.HasPrefix(strategyName, "ID_Kelt_Mid"),
		strings.HasPrefix(strategyName, "ID_Stoch_Trend"),
		strings.HasPrefix(strategyName, "ID_CCI_Trend"):
		return "Trend Elite"

	case strings.HasPrefix(strategyName, "ID_MACD"),
		strings.HasPrefix(strategyName, "ID_CCI_Zero"):
		return "Momentum Elite"

	case strings.HasPrefix(strategyName, "ID_VWAP_Dev"),
		strings.HasPrefix(strategyName, "ID_BB_Bounce"),
		strings.HasPrefix(strategyName, "ID_RSI"),
		strings.HasPrefix(strategyName, "ID_Kelt_Bounce"),
		strings.HasPrefix(strategyName, "ID_Stoch_Oversold"),
		strings.HasPrefix(strategyName, "ID_CCI_Extreme"):
		return "Mean Rev Elite"

	case strings.HasPrefix(strategyName, "ID_BB_Break"),
		strings.HasPrefix(strategyName, "ID_Kelt_Break"):
		return "Breakout Elite"

	case strings.HasPrefix(strategyName, "ID_BB_Width"):
		return "Volatility Elite"

	default:
		return "Intraday"
	}
}

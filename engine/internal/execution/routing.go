package execution

// OrderMode represents the intended execution style for a trade.
type OrderMode string

const (
	OrderModeMarket   OrderMode = "MARKET"
	OrderModePostOnly OrderMode = "POST_ONLY"
	OrderModeIOC      OrderMode = "IOC"
)

// FillResult records the simulated execution outcome.
type FillResult struct {
	ExecPrice float64
	OrderMode OrderMode
}

// RouteModeForCategory picks a low-latency execution intent from the current
// strategy family and regime. This keeps breakout logic aggressive while still
// letting mean-reversion and pullback setups behave more like maker entries.
func RouteModeForCategory(category, regime string) OrderMode {
	switch category {
	case "Mean Reversion", "Mean Rev Elite", "Statistical", "Adaptive", "Adaptive Elite":
		return OrderModePostOnly
	case "Breakout", "Breakout Elite", "Volatility", "Volatility Elite", "Time-of-Day", "Microstructure":
		return OrderModeIOC
	case "Momentum", "Momentum Elite":
		return OrderModeMarket
	case "Trend", "Trend Elite":
		if regime == "TREND" {
			return OrderModePostOnly
		}
		return OrderModeMarket
	case "Price Action", "Price Action Elite":
		if regime == "RANGE" {
			return OrderModePostOnly
		}
		return OrderModeMarket
	case "Multi-Signal":
		if regime == "TREND" {
			return OrderModeIOC
		}
		return OrderModeMarket
	default:
		return OrderModeMarket
	}
}

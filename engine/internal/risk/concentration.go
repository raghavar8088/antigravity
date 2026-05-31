package risk

func ConcentrationBySymbol(positions map[string]RiskPosition) map[string]float64 {
	return concentration(positions, func(p RiskPosition) string { return p.Symbol })
}

func ConcentrationByStrategy(positions map[string]RiskPosition) map[string]float64 {
	return concentration(positions, func(p RiskPosition) string { return p.Strategy })
}

func ConcentrationByFamily(positions map[string]RiskPosition) map[string]float64 {
	return concentration(positions, func(p RiskPosition) string {
		if p.StrategyFamily == "" {
			return "Unknown"
		}
		return p.StrategyFamily
	})
}

func ConcentrationByExchange(positions map[string]RiskPosition) map[string]float64 {
	return concentration(positions, func(p RiskPosition) string {
		if p.Exchange == "" {
			return "Unknown"
		}
		return p.Exchange
	})
}

func concentration(positions map[string]RiskPosition, keyFn func(RiskPosition) string) map[string]float64 {
	out := make(map[string]float64)
	total := 0.0
	for _, p := range positions {
		total += p.NotionalUSD
		out[keyFn(p)] += p.NotionalUSD
	}
	if total <= 0 {
		return out
	}
	for k, v := range out {
		out[k] = v / total * 100
	}
	return out
}

func CheckConcentration(metrics PortfolioMetrics, limits RiskLimits) string {
	if metrics.GrossExposureUSD <= 0 || metrics.EquityUSD <= 0 {
		return ""
	}
	// Concentration is meaningful once the portfolio has enough gross exposure
	// for diversification to exist. A first small BTC trade should not be
	// rejected just because it is temporarily 100% of open risk.
	if metrics.GrossExposureUSD/metrics.EquityUSD*100 < 20 {
		return ""
	}
	for _, pct := range metrics.SymbolConcentration {
		if pct > limits.MaxSymbolConcentrationPct {
			return "symbol concentration limit breached"
		}
	}
	for _, pct := range metrics.StrategyConcentration {
		if pct > limits.MaxStrategyAllocationPct {
			return "strategy concentration limit breached"
		}
	}
	for _, pct := range metrics.ExchangeConcentration {
		if pct > limits.MaxExchangeExposurePct {
			return "exchange concentration limit breached"
		}
	}
	return ""
}

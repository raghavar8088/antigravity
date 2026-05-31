package risk

func StrategyReturnSeries(stats map[string]TradeStats) map[string][]float64 {
	out := make(map[string][]float64, len(stats))
	for name, s := range stats {
		if len(s.Returns) > 0 {
			out[name] = append([]float64(nil), s.Returns...)
		}
	}
	return out
}

func FamilyReturnSeries(stats map[string]TradeStats) map[string][]float64 {
	out := make(map[string][]float64)
	for _, s := range stats {
		if s.Family == "" || len(s.Returns) == 0 {
			continue
		}
		out[s.Family] = append(out[s.Family], s.Returns...)
	}
	return out
}

func CorrelationGuardReason(order RiskOrder, positions map[string]RiskPosition, stats map[string]TradeStats, limit float64) string {
	sameDirection := 0
	for _, p := range positions {
		if p.Symbol == order.Symbol && p.Side == order.Side {
			sameDirection++
		}
	}
	if sameDirection >= 4 {
		return "correlation guard: too many same-symbol same-direction positions"
	}

	matrix := BuildCorrelationMatrix(StrategyReturnSeries(stats))
	if MaxAbsCorrelation(matrix) > limit {
		return "correlation guard: strategy correlation exceeds limit"
	}
	return ""
}

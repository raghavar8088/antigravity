package v2

type DynamicSizingDecision struct {
	Scale              SizingScale
	Multiplier         float64
	RecommendedSizeBTC float64
	Disabled           bool
	Reason             string
	Logs               []SizingDecisionLog
}

func DynamicSize(req TradeRequest, metrics StrategyMetrics, market MarketState, heat HeatSnapshot, drawdown DrawdownDecision, corr CorrelationReport, tail TailRiskDecision) DynamicSizingDecision {
	size := req.RequestedSizeBTC
	if size <= 0 && req.EntryPrice > 0 {
		size = 1000 / req.EntryPrice
	}
	out := DynamicSizingDecision{Multiplier: 1, RecommendedSizeBTC: size}
	apply := func(layer, reason string, mult float64) {
		before := out.RecommendedSizeBTC
		out.Multiplier *= mult
		out.RecommendedSizeBTC *= mult
		out.Logs = append(out.Logs, SizingDecisionLog{Layer: layer, BeforeBTC: before, AfterBTC: out.RecommendedSizeBTC, Multiplier: mult, Reason: reason})
	}
	if metrics.HealthScore > 0 && metrics.HealthScore < 50 {
		apply("strategy_health", "strategy health below institutional threshold", 0.50)
	}
	if metrics.Sharpe > 0 && metrics.Sharpe < 1.0 {
		apply("sharpe", "Sharpe below 1.0", 0.75)
	}
	if metrics.ProfitFactor > 0 && metrics.ProfitFactor < 1.2 {
		apply("profit_factor", "profit factor below 1.2", 0.50)
	}
	if drawdown.SizeMultiplier < 1 {
		apply("drawdown", drawdown.Action, drawdown.SizeMultiplier)
	}
	if heat.SizeMultiplier < 1 {
		apply("portfolio_heat", heat.Action, heat.SizeMultiplier)
	}
	if market.Regime == RegimeHighVol || market.VolatilityPct >= 3 {
		apply("volatility", "high volatility risk reduction", 0.50)
	}
	if market.Regime == RegimeFundingExtreme || abs(market.FundingRate) > 0.0008 {
		apply("funding", "funding environment is stressed", 0.75)
	}
	if corr.MaxCorrelation >= 0.70 {
		apply("correlation", "correlation cluster detected", 0.50)
	}
	if tail.Action == TailActionReduceRisk {
		apply("tail_risk", tail.Reason, 0.25)
	}
	if tail.Action == TailActionHalt || tail.Action == TailActionClosePositions {
		apply("tail_risk", tail.Reason, 0)
	}
	out.Scale = scaleFromMultiplier(out.Multiplier)
	out.Disabled = out.Scale == SizeDisable
	if out.Disabled {
		out.Reason = "dynamic sizing disabled the trade"
	} else {
		out.Reason = "dynamic sizing applied"
	}
	return out
}

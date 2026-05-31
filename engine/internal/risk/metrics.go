package risk

func (e *PortfolioRiskEngine) metricsLocked() PortfolioMetrics {
	return e.metricsForPositionsLocked(e.positions)
}

func (e *PortfolioRiskEngine) metricsForPositionsLocked(positions map[string]RiskPosition) PortfolioMetrics {
	equity := e.equitySafeLocked()
	exposure := PortfolioExposure(positions, equity)
	varMetrics := PortfolioVaR(e.returns, equity)
	cvarMetrics := PortfolioCVaR(e.returns, equity)
	strategyCorr := BuildCorrelationMatrix(StrategyReturnSeries(e.stats))
	familyCorr := BuildCorrelationMatrix(FamilyReturnSeries(e.stats))
	maxCorr := MaxAbsCorrelation(strategyCorr)
	if f := MaxAbsCorrelation(familyCorr); f > maxCorr {
		maxCorr = f
	}
	drawdown := DrawdownPct(e.equityUSD, e.highWatermarkUSD)
	dailyLossPct := lossPct(e.dailyPnLUSD, equity)
	weeklyLossPct := lossPct(e.weeklyPnLUSD, equity)
	monthlyLossPct := lossPct(e.monthlyPnLUSD, equity)
	heat := PortfolioHeatPct(positions, equity)
	riskOfRuin := RiskOfRuinPct(0.5, 1.5, maxFloat(heat, 1), drawdown)
	return PortfolioMetrics{
		EquityUSD:             equity,
		PortfolioHeatPct:      heat,
		HeatLevel:             HeatRiskLevel(heat),
		LongExposureUSD:       exposure.LongExposureUSD,
		ShortExposureUSD:      exposure.ShortExposureUSD,
		NetExposureUSD:        exposure.NetExposureUSD,
		GrossExposureUSD:      exposure.GrossExposureUSD,
		Leverage:              exposure.Leverage,
		VaR95USD:              varMetrics.Daily95USD,
		VaR99USD:              varMetrics.Daily99USD,
		CVaR95USD:             cvarMetrics.CVaR95USD,
		CVaR99USD:             cvarMetrics.CVaR99USD,
		MaxCorrelation:        maxCorr,
		DrawdownPct:           drawdown,
		DailyLossPct:          dailyLossPct,
		WeeklyLossPct:         weeklyLossPct,
		MonthlyLossPct:        monthlyLossPct,
		RiskOfRuinPct:         riskOfRuin,
		SymbolConcentration:   ConcentrationBySymbol(positions),
		StrategyConcentration: ConcentrationByStrategy(positions),
		FamilyConcentration:   ConcentrationByFamily(positions),
		ExchangeConcentration: ConcentrationByExchange(positions),
		RiskBudgetUsage:       RiskBudgetUsage(positions, equity),
	}
}

func (e *PortfolioRiskEngine) lossLimitReasonLocked() string {
	metrics := e.metricsLocked()
	if DailyLossLevel(metrics.DailyLossPct) == RiskLevelBlocked {
		return "daily hard loss limit breached"
	}
	if WeeklyLossLevel(metrics.WeeklyLossPct) == RiskLevelBlocked {
		return "weekly hard loss limit breached"
	}
	if MonthlyLossLevel(metrics.MonthlyLossPct) == RiskLevelBlocked {
		return "monthly hard loss limit breached"
	}
	if DrawdownTradingHalt(metrics.DrawdownPct) {
		return "drawdown trading halt"
	}
	return ""
}

func lossPct(pnlUSD, equityUSD float64) float64 {
	if equityUSD <= 0 || pnlUSD >= 0 {
		return 0
	}
	return -pnlUSD / equityUSD * 100
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

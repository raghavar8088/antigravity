package v2

type InstitutionalBacktestDashboard struct {
	BacktestSummary        Metrics
	ExecutionQuality       ExecutionQualityMetrics
	SpreadAnalysis         float64
	FundingAttribution     float64
	MonteCarloDistribution MonteCarloReport
	RegimePerformance      map[Regime]Metrics
	BenchmarkComparison    BenchmarkReport
	OOSPerformance         WalkForwardReport
	Robustness             RobustnessReport
	Portfolio              PortfolioBacktestResult
	VaR                    float64
	CVaR                   float64
	RiskOfRuin             float64
	TradeDistribution      []Trade
	Drawdown               float64
	LatencyImpactMs        float64
	MarketImpact           float64
}

func BuildDashboard(result Result, mc MonteCarloReport, wf WalkForwardReport, robustness RobustnessReport, benchmarks BenchmarkReport, portfolio PortfolioBacktestResult) InstitutionalBacktestDashboard {
	return InstitutionalBacktestDashboard{
		BacktestSummary:        result.Metrics,
		ExecutionQuality:       result.ExecutionQuality,
		SpreadAnalysis:         result.ExecutionQuality.AverageSpreadCost,
		FundingAttribution:     result.ExecutionQuality.FundingPnL,
		MonteCarloDistribution: mc,
		RegimePerformance:      result.RegimeStats,
		BenchmarkComparison:    benchmarks,
		OOSPerformance:         wf,
		Robustness:             robustness,
		Portfolio:              portfolio,
		VaR:                    result.Metrics.VaR95,
		CVaR:                   result.Metrics.CVaR95,
		RiskOfRuin:             result.Metrics.RiskOfRuin,
		TradeDistribution:      result.Trades,
		Drawdown:               result.Metrics.MaxDrawdown,
		LatencyImpactMs:        result.ExecutionQuality.AverageLatencyMs,
		MarketImpact:           result.ExecutionQuality.AverageImpactCost,
	}
}

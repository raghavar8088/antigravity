package v2

type KellyMode string

const (
	KellyFull    KellyMode = "FULL"
	KellyHalf    KellyMode = "HALF"
	KellyQuarter KellyMode = "QUARTER"
)

type KellyDecision struct {
	FullKellyFraction    float64
	HalfKellyFraction    float64
	QuarterKellyFraction float64
	SelectedMode         KellyMode
	SelectedFraction     float64
	KellyConfidence      float64
	KellyStability       float64
	KellyDrawdownRisk    float64
	RecommendedSizeBTC   float64
	RecommendedRiskUSD   float64
}

func KellySize(req TradeRequest, metrics StrategyMetrics, equityUSD, maxRiskPct float64) KellyDecision {
	if equityUSD <= 0 {
		equityUSD = 100_000
	}
	if maxRiskPct <= 0 {
		maxRiskPct = 2
	}
	avgLoss := abs(metrics.AverageLossUSD)
	avgWin := metrics.AverageWinUSD
	if avgLoss <= 0 {
		avgLoss = max(avgWin, 1)
	}
	if avgWin <= 0 {
		avgWin = avgLoss
	}
	b := avgWin / avgLoss
	p := clamp(metrics.WinRate, 0, 1)
	if p == 0 && metrics.TotalTrades == 0 {
		p = 0.5
	}
	q := 1 - p
	full := 0.0
	if b > 0 {
		full = (b*p - q) / b
	}
	full = clamp(full, 0, maxRiskPct/100)
	half := full * 0.5
	quarter := full * 0.25
	stability := clamp(float64(metrics.TotalTrades)/100, 0, 1) * clamp(metrics.ProfitFactor/1.5, 0, 1)
	confidence := clamp((metrics.Sharpe/2+metrics.OOSProfitFactor/2)/2, 0, 1)
	ddRisk := clamp(metrics.MaxDrawdownPct/10, 0, 1)
	selectedMode := KellyHalf
	selected := half
	if stability < 0.45 || ddRisk > 0.6 {
		selectedMode = KellyQuarter
		selected = quarter
	}
	riskUSD := min(equityUSD*selected, equityUSD*maxRiskPct/100)
	perBTC := PositionRiskUSD(req, 1)
	size := 0.0
	if perBTC > 0 {
		size = riskUSD / perBTC
	}
	if req.RequestedSizeBTC > 0 {
		size = min(size, req.RequestedSizeBTC)
	}
	return KellyDecision{
		FullKellyFraction:    full,
		HalfKellyFraction:    half,
		QuarterKellyFraction: quarter,
		SelectedMode:         selectedMode,
		SelectedFraction:     selected,
		KellyConfidence:      confidence,
		KellyStability:       stability,
		KellyDrawdownRisk:    ddRisk,
		RecommendedSizeBTC:   size,
		RecommendedRiskUSD:   riskUSD,
	}
}

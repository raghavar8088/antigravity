package v2

import (
	"math"
	"sort"

	"antigravity-engine/internal/risk"
)

type Metrics struct {
	TotalTrades     int
	WinRate         float64
	GrossPnL        float64
	NetPnL          float64
	ProfitFactor    float64
	Expectancy      float64
	Sharpe          float64
	Sortino         float64
	Calmar          float64
	Omega           float64
	UlcerIndex      float64
	VaR95           float64
	CVaR95          float64
	MaxDrawdown     float64
	RecoveryFactor  float64
	RiskOfRuin      float64
	TailRatio       float64
	GainToPainRatio float64
	KellyFraction   float64
}

type ExecutionQualityMetrics struct {
	AverageSpreadCost   float64
	AverageSlippageCost float64
	AverageImpactCost   float64
	AverageFillRatio    float64
	AverageLatencyMs    float64
	FundingPnL          float64
}

func CalculateMetrics(trades []Trade, initialCapital float64) Metrics {
	if len(trades) == 0 {
		return Metrics{}
	}
	returns := make([]float64, 0, len(trades))
	pnls := make([]float64, 0, len(trades))
	grossProfit, grossLoss, wins, losses := 0.0, 0.0, 0, 0
	equity, peak, maxDD, ulcerSum := initialCapital, initialCapital, 0.0, 0.0
	for _, tr := range trades {
		pnls = append(pnls, tr.NetPnL)
		if initialCapital > 0 {
			returns = append(returns, tr.NetPnL/initialCapital)
		}
		if tr.NetPnL >= 0 {
			grossProfit += tr.NetPnL
			wins++
		} else {
			grossLoss += -tr.NetPnL
			losses++
		}
		equity += tr.NetPnL
		if equity > peak {
			peak = equity
		}
		dd := peak - equity
		if dd > maxDD {
			maxDD = dd
		}
		ddPct := 0.0
		if peak > 0 {
			ddPct = dd / peak * 100
		}
		ulcerSum += ddPct * ddPct
	}
	net := sum(pnls)
	pf := 0.0
	if grossLoss > 0 {
		pf = grossProfit / grossLoss
	}
	winRate := float64(wins) / float64(len(trades))
	avgWin, avgLoss := 0.0, 0.0
	if wins > 0 {
		avgWin = grossProfit / float64(wins)
	}
	if losses > 0 {
		avgLoss = grossLoss / float64(losses)
	}
	kelly := risk.KellySizing(risk.TradeStats{WinRate: winRate, ProfitFactor: pf, AverageWin: avgWin, AverageLoss: -avgLoss})
	return Metrics{
		TotalTrades:     len(trades),
		WinRate:         winRate,
		GrossPnL:        grossProfit - grossLoss,
		NetPnL:          net,
		ProfitFactor:    pf,
		Expectancy:      net / float64(len(trades)),
		Sharpe:          sharpe(returns),
		Sortino:         sortino(returns),
		Calmar:          ratio(net, maxDD),
		Omega:           ratio(grossProfit, grossLoss),
		UlcerIndex:      math.Sqrt(ulcerSum / float64(len(trades))),
		VaR95:           risk.HistoricalVaR(returns, initialCapital, 0.95),
		CVaR95:          risk.HistoricalCVaR(returns, initialCapital, 0.95),
		MaxDrawdown:     maxDD,
		RecoveryFactor:  ratio(net, maxDD),
		RiskOfRuin:      risk.RiskOfRuinPct(winRate, ratio(avgWin, avgLoss), 1, ratio(maxDD, initialCapital)*100),
		TailRatio:       tailRatio(pnls),
		GainToPainRatio: ratio(grossProfit, grossLoss),
		KellyFraction:   kelly.CappedKelly,
	}
}

func CalculateExecutionQuality(trades []Trade) ExecutionQualityMetrics {
	if len(trades) == 0 {
		return ExecutionQualityMetrics{}
	}
	var out ExecutionQualityMetrics
	for _, tr := range trades {
		out.AverageSpreadCost += tr.SpreadCost
		out.AverageSlippageCost += tr.SlippageCost
		out.AverageImpactCost += tr.ImpactCost
		out.AverageFillRatio += tr.ExecutionQuality.FillRatio
		out.AverageLatencyMs += float64(tr.ExecutionQuality.Latency.EntryLatency.Milliseconds())
		out.FundingPnL += tr.FundingPnL
	}
	n := float64(len(trades))
	out.AverageSpreadCost /= n
	out.AverageSlippageCost /= n
	out.AverageImpactCost /= n
	out.AverageFillRatio /= n
	out.AverageLatencyMs /= n
	return out
}

func sharpe(returns []float64) float64 {
	mean, std := meanStd(returns, false)
	if std == 0 {
		return 0
	}
	return mean / std * math.Sqrt(252)
}

func sortino(returns []float64) float64 {
	mean, down := meanStd(returns, true)
	if down == 0 {
		return 0
	}
	return mean / down * math.Sqrt(252)
}

func meanStd(values []float64, downside bool) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	mean := sum(values) / float64(len(values))
	var sq float64
	count := 0
	for _, v := range values {
		if downside && v >= 0 {
			continue
		}
		d := v - mean
		sq += d * d
		count++
	}
	if count <= 1 {
		return mean, 0
	}
	return mean, math.Sqrt(sq / float64(count-1))
}

func tailRatio(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	p95 := cp[int(0.95*float64(len(cp)-1))]
	p05 := cp[int(0.05*float64(len(cp)-1))]
	if p05 == 0 {
		return 0
	}
	return math.Abs(p95 / p05)
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

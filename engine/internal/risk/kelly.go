package risk

import "math"

type KellyMetrics struct {
	WinRate       float64
	ProfitFactor  float64
	AverageWin    float64
	AverageLoss   float64
	KellyFraction float64
	HalfKelly     float64
	CappedKelly   float64
}

func KellySizing(stats TradeStats) KellyMetrics {
	winRate := stats.WinRate
	if winRate <= 0 && len(stats.Returns) > 0 {
		wins := 0
		for _, r := range stats.Returns {
			if r > 0 {
				wins++
			}
		}
		winRate = float64(wins) / float64(len(stats.Returns))
	}
	avgWin := math.Abs(stats.AverageWin)
	avgLoss := math.Abs(stats.AverageLoss)
	if avgLoss == 0 {
		avgLoss = 1
	}
	b := avgWin / avgLoss
	if b <= 0 {
		b = math.Max(stats.ProfitFactor, 0.01)
	}
	kelly := winRate - ((1 - winRate) / b)
	if kelly < 0 {
		kelly = 0
	}
	half := kelly * 0.5
	capped := math.Min(half, 0.25)
	return KellyMetrics{
		WinRate:       winRate,
		ProfitFactor:  stats.ProfitFactor,
		AverageWin:    avgWin,
		AverageLoss:   avgLoss,
		KellyFraction: kelly,
		HalfKelly:     half,
		CappedKelly:   capped,
	}
}

func HalfKellySize(order RiskOrder, stats TradeStats, equityUSD float64) float64 {
	if equityUSD <= 0 || order.EntryPrice <= 0 {
		return order.SizeBTC
	}
	k := KellySizing(stats)
	if k.CappedKelly <= 0 {
		return order.SizeBTC
	}
	notional := equityUSD * k.CappedKelly
	return notional / order.EntryPrice
}

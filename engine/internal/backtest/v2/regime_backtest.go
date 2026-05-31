package v2

import (
	"math"

	"antigravity-engine/internal/marketdata"
)

type Regime string

const (
	RegimeTrendingBull   Regime = "TRENDING_BULL"
	RegimeTrendingBear   Regime = "TRENDING_BEAR"
	RegimeRange          Regime = "RANGE"
	RegimeHighVol        Regime = "HIGH_VOLATILITY"
	RegimeLowVol         Regime = "LOW_VOLATILITY"
	RegimeNewsEvent      Regime = "NEWS_EVENT"
	RegimeFundingExtreme Regime = "FUNDING_EXTREME"
)

type RegimeClassifier struct {
	window []float64
}

func (r *RegimeClassifier) Classify(t marketdata.Tick) Regime {
	r.window = append(r.window, t.Price)
	if len(r.window) > 30 {
		r.window = r.window[1:]
	}
	if len(r.window) < 5 {
		return RegimeRange
	}
	start := r.window[0]
	end := r.window[len(r.window)-1]
	changePct := (end - start) / start * 100
	vol := realizedVol(r.window)
	switch {
	case math.Abs(changePct) > 4 && vol > 2.5:
		return RegimeNewsEvent
	case vol > 1.8:
		return RegimeHighVol
	case vol < 0.25:
		return RegimeLowVol
	case changePct > 0.75:
		return RegimeTrendingBull
	case changePct < -0.75:
		return RegimeTrendingBear
	default:
		return RegimeRange
	}
}

func RegimeStatistics(trades []Trade, initialCapital float64) map[Regime]Metrics {
	grouped := make(map[Regime][]Trade)
	for _, tr := range trades {
		grouped[tr.Regime] = append(grouped[tr.Regime], tr)
	}
	out := make(map[Regime]Metrics, len(grouped))
	for regime, trs := range grouped {
		out[regime] = CalculateMetrics(trs, initialCapital)
	}
	return out
}

func realizedVol(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var returns []float64
	for i := 1; i < len(values); i++ {
		if values[i-1] > 0 {
			returns = append(returns, (values[i]-values[i-1])/values[i-1]*100)
		}
	}
	_, std := meanStd(returns, false)
	return std
}

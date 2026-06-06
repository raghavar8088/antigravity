package phase28

import (
	"math"

	"antigravity-engine/internal/validation/phase25"
)

// BuildCorrelationMatrix computes pairwise Pearson r of daily strategy returns
// from actual LiveTrade records. Pairs with insufficient data are noted.
func BuildCorrelationMatrix(strategies []string, trades []phase25.LiveTrade) CorrelationMatrix {
	cm := CorrelationMatrix{
		Strategies: strategies,
		Matrix:     make(map[string]map[string]float64),
	}

	// Build daily PnL series per strategy
	dailyPnL := make(map[string]map[string]float64)
	for _, s := range strategies {
		dailyPnL[s] = make(map[string]float64)
	}
	for _, t := range trades {
		day := t.EntryTime.Format("2006-01-02")
		if _, ok := dailyPnL[t.StrategyName]; ok {
			dailyPnL[t.StrategyName][day] += t.NetPnLUSD
		}
	}

	// Compute pairwise correlations
	for i, sA := range strategies {
		if cm.Matrix[sA] == nil {
			cm.Matrix[sA] = make(map[string]float64)
		}
		cm.Matrix[sA][sA] = 1.0

		for j := i + 1; j < len(strategies); j++ {
			sB := strategies[j]
			if cm.Matrix[sB] == nil {
				cm.Matrix[sB] = make(map[string]float64)
			}

			// Build aligned series
			var xA, xB []float64
			tradingDaysA := len(dailyPnL[sA])
			tradingDaysB := len(dailyPnL[sB])

			// Note insufficient data for pairs with < 10 trading days
			if tradingDaysA < 10 || tradingDaysB < 10 {
				cm.Matrix[sA][sB] = math.NaN()
				cm.Matrix[sB][sA] = math.NaN()
				continue
			}

			for day, pnlA := range dailyPnL[sA] {
				if pnlB, ok := dailyPnL[sB][day]; ok {
					xA = append(xA, pnlA)
					xB = append(xB, pnlB)
				}
			}

			r := pearsonR28(xA, xB)
			cm.Matrix[sA][sB] = r
			cm.Matrix[sB][sA] = r
		}
	}

	return cm
}

func pearsonR28(x, y []float64) float64 {
	n := len(x)
	if n < 2 || n != len(y) {
		return 0
	}

	var mx, my float64
	for i := 0; i < n; i++ {
		mx += x[i]
		my += y[i]
	}
	mx /= float64(n)
	my /= float64(n)

	var num, sx, sy float64
	for i := 0; i < n; i++ {
		dx := x[i] - mx
		dy := y[i] - my
		num += dx * dy
		sx += dx * dx
		sy += dy * dy
	}

	denom := math.Sqrt(sx * sy)
	if denom == 0 {
		return 0
	}
	return num / denom
}

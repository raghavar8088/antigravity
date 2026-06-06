package phase26

import (
	"math"

	"antigravity-engine/internal/validation/phase25"
)

// computeMetricsFromTrades computes key metrics from a slice of LiveTrade records.
func computeMetricsFromTrades(trades []phase25.LiveTrade) (winRate, pf, sharpe, sortino, expectancy, maxDD, ror, netPnL, totalFee, totalFund, totalSlip float64) {
	if len(trades) == 0 {
		return
	}

	var wins, losses int
	var sumWin, sumLoss, totalNet float64
	var pnls []float64

	for _, t := range trades {
		netPnL += t.NetPnLUSD
		totalFee += t.FeeUSD
		totalFund += t.FundingUSD
		totalSlip += t.SlippageUSD
		pnls = append(pnls, t.NetPnLUSD)
		if t.IsWin {
			wins++
			sumWin += t.NetPnLUSD
		} else {
			losses++
			sumLoss += t.NetPnLUSD
		}
	}

	n := float64(len(trades))
	if wins+losses > 0 {
		winRate = float64(wins) / n
	}
	if sumLoss < 0 {
		pf = sumWin / math.Abs(sumLoss)
	} else if sumWin > 0 {
		pf = 999.0
	}

	totalNet = netPnL
	expectancy = totalNet / n

	// Sharpe: mean / stddev * sqrt(252)
	mean := totalNet / n
	var variance float64
	for _, p := range pnls {
		diff := p - mean
		variance += diff * diff
	}
	if n > 1 {
		variance /= (n - 1)
	}
	stdDev := math.Sqrt(variance)
	if stdDev > 0 {
		sharpe = (mean / stdDev) * math.Sqrt(252)
	}

	// Sortino: use only downside deviation
	var downsideVar float64
	for _, p := range pnls {
		if p < 0 {
			downsideVar += p * p
		}
	}
	if n > 1 {
		downsideVar /= (n - 1)
	}
	downsideStd := math.Sqrt(downsideVar)
	if downsideStd > 0 {
		sortino = (mean / downsideStd) * math.Sqrt(252)
	}

	// MaxDD: walk equity curve
	peak := 0.0
	equity := 0.0
	for _, p := range pnls {
		equity += p
		if equity > peak {
			peak = equity
		}
		dd := 0.0
		if peak > 0 {
			dd = (peak - equity) / peak * 100
		}
		if dd > maxDD {
			maxDD = dd
		}
	}

	// RoR: simplified Kelly approximation — only if >= 50 trades
	if len(trades) >= 50 && winRate > 0 && winRate < 1 {
		avgLoss := 0.0
		if losses > 0 {
			avgLoss = math.Abs(sumLoss) / float64(losses)
		}
		if avgLoss > 0 {
			ror = math.Pow(1-winRate, n/avgLoss)
		}
	}

	return
}

// computeMilestoneAt computes a MilestoneReport for the first N trades.
func computeMilestoneAt(stratName string, trades []phase25.LiveTrade, count int) MilestoneReport {
	subset := trades
	if len(subset) > count {
		subset = subset[:count]
	}
	wr, pf, sh, so, exp, mdd, ror, net, fee, fund, slip := computeMetricsFromTrades(subset)
	verdict := "PASS"
	if pf < LiveMinPF || sh < LiveMinSharpe || mdd > LiveMaxDD || exp <= LiveMinExpectancy {
		verdict = "FAIL"
	}
	return MilestoneReport{
		StrategyName: stratName,
		TradeCount:   len(subset),
		WinRate:      wr,
		ProfitFactor: pf,
		Sharpe:       sh,
		Sortino:      so,
		Expectancy:   exp,
		MaxDD:        mdd,
		RoR:          ror,
		NetPnLUSD:    net,
		TotalFeeUSD:  fee,
		TotalFundUSD: fund,
		TotalSlipUSD: slip,
		Verdict:      verdict,
	}
}

// filterTrades returns only the trades for the given strategy name.
func filterTrades(trades []phase25.LiveTrade, stratName string) []phase25.LiveTrade {
	var out []phase25.LiveTrade
	for _, t := range trades {
		if t.StrategyName == stratName {
			out = append(out, t)
		}
	}
	return out
}

// computeUlcerIndex computes the root mean square drawdown series.
func computeUlcerIndex(pnls []float64) float64 {
	if len(pnls) == 0 {
		return 0
	}
	peak := 0.0
	equity := 0.0
	var sumSq float64
	for _, p := range pnls {
		equity += p
		if equity > peak {
			peak = equity
		}
		dd := 0.0
		if peak > 0 {
			dd = (peak - equity) / peak * 100
		}
		sumSq += dd * dd
	}
	return math.Sqrt(sumSq / float64(len(pnls)))
}

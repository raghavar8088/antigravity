package phase27

import (
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase25"
)

// BuildFamilyPerf computes all metrics for one alpha family from attribution records.
// If Trades < 30, it returns the partial struct and the caller must track InsufficientEvidence.
func BuildFamilyPerf(family phase25.AlphaFamily, records []AlphaAttributionRecord, totalNetPnL float64) AlphaFamilyPerf {
	p := AlphaFamilyPerf{Family: family}

	var wins int
	var sumWin, sumLoss float64
	var pnls []float64

	for _, r := range records {
		if r.AlphaFamily != family {
			continue
		}
		p.Trades++
		p.NetPnLUSD += r.NetPnLUSD
		p.GrossPnLUSD += r.GrossPnLUSD
		p.TotalFeeUSD += r.FeeUSD
		p.TotalFundingUSD += r.FundingUSD
		p.TotalSlippageUSD += r.SlippageUSD
		pnls = append(pnls, r.NetPnLUSD)
		if r.IsWin {
			wins++
			sumWin += r.NetPnLUSD
		} else {
			sumLoss += r.NetPnLUSD
		}
	}

	if p.Trades == 0 {
		return p
	}

	n := float64(p.Trades)
	p.WinRate = float64(wins) / n

	if sumLoss < 0 {
		p.ProfitFactor = sumWin / math.Abs(sumLoss)
	} else if sumWin > 0 {
		p.ProfitFactor = 999.0
	}

	p.Expectancy = p.NetPnLUSD / n

	mean := p.NetPnLUSD / n
	var variance, downsideVar float64
	for _, v := range pnls {
		diff := v - mean
		variance += diff * diff
		if v < 0 {
			downsideVar += v * v
		}
	}
	if n > 1 {
		variance /= (n - 1)
		downsideVar /= (n - 1)
	}
	stdDev := math.Sqrt(variance)
	if stdDev > 0 {
		p.Sharpe = (mean / stdDev) * math.Sqrt(252)
	}
	downside := math.Sqrt(downsideVar)
	if downside > 0 {
		p.Sortino = (mean / downside) * math.Sqrt(252)
	}

	// MaxDD
	peak, equity := 0.0, 0.0
	var sumSq float64
	for _, v := range pnls {
		equity += v
		if equity > peak {
			peak = equity
		}
		dd := 0.0
		if peak > 0 {
			dd = (peak - equity) / peak * 100
		}
		if dd > p.MaxDrawdown {
			p.MaxDrawdown = dd
		}
		sumSq += dd * dd
	}
	p.UlcerIndex = math.Sqrt(sumSq / n)

	if p.MaxDrawdown > 0 {
		p.RecoveryFactor = p.NetPnLUSD / (p.MaxDrawdown / 100 * math.Abs(p.NetPnLUSD))
	}

	// RoR — only if >= 50 trades
	if p.Trades >= 50 && p.WinRate > 0 && p.WinRate < 1 {
		losses := p.Trades - wins
		if losses > 0 {
			avgLoss := math.Abs(sumLoss) / float64(losses)
			if avgLoss > 0 {
				p.RiskOfRuin = math.Pow(1-p.WinRate, n/avgLoss)
			}
		}
	}

	// Profit contribution
	if totalNetPnL > 0 {
		p.ProfitContributionPct = p.NetPnLUSD / totalNetPnL * 100
	}

	return p
}

// BuildAllFamilyPerfs computes performance for every alpha family present in records.
func BuildAllFamilyPerfs(records []AlphaAttributionRecord) ([]AlphaFamilyPerf, float64, []string) {
	// Collect families and total PnL
	familySet := make(map[phase25.AlphaFamily]bool)
	total := 0.0
	for _, r := range records {
		familySet[r.AlphaFamily] = true
		total += r.NetPnLUSD
	}

	var perfs []AlphaFamilyPerf
	var insufficient []string

	for fam := range familySet {
		p := BuildFamilyPerf(fam, records, total)
		if p.Trades < 30 {
			insufficient = append(insufficient, string(fam))
		}
		perfs = append(perfs, p)
	}

	// Sort by NetPnLUSD descending
	sort.Slice(perfs, func(i, j int) bool {
		return perfs[i].NetPnLUSD > perfs[j].NetPnLUSD
	})

	return perfs, total, insufficient
}

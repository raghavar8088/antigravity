package phase24

import (
	"math"
	"sort"

	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// alphaEngineMapping maps strategy category/alpha-source strings to the
// canonical AlphaEngine identifiers used for Phase 24 championship ranking.
var alphaEngineMapping = map[string]AlphaEngine{
	"Funding Mean Reversion":  AlphaFundingMeanReversion,
	"Liquidation Cascade":     AlphaLiquidationCascade,
	"Liquidity Sweep":         AlphaLiquiditySweep,
	"Institutional Alpha":     AlphaLiquiditySweep,
	"MSS":                     AlphaMSS,
	"Market Structure Shift":  AlphaMSS,
	"Fair Value Gap":          AlphaFVG,
	"Order Block":             AlphaOrderBlock,
	"Market Structure":        AlphaMarketStructure,
	"Session Expansion":       AlphaSessionExpansion,
	"Statistical Mean Reversion": AlphaStatMeanReversion,
	"Order Flow":              AlphaMomentum,
	"CVD Divergence":          AlphaMomentum,
	"Momentum":                AlphaMomentum,
	"Trend":                   AlphaTrend,
}

// canonicalAlpha maps an arbitrary alpha source label to the AlphaEngine enum.
func canonicalAlpha(src string) AlphaEngine {
	if a, ok := alphaEngineMapping[src]; ok {
		return a
	}
	return AlphaStatMeanReversion
}

// BuildAlphaChampionship aggregates metrics by alpha engine and ranks all
// alpha sources from best to worst.
func BuildAlphaChampionship(
	metrics map[string]ExtendedMetrics,
	wfReports []phase23b.WFReport,
	mcResults map[string]phase23b.RealMCResult,
) []AlphaChampionship24 {

	type alphaAgg struct {
		engine         AlphaEngine
		trades         []phase23b.CertifiedTrade
		netPnL         float64
		grossPnL       float64
		sumWin         float64
		sumLoss        float64
		wins           int
		count          int
		maxDD          float64
		wfWindows      int
		wfWinWindows   int
		mcRuns         int
		mcProfitable   int
	}

	agg := make(map[AlphaEngine]*alphaAgg)

	for _, m := range metrics {
		eng := canonicalAlpha(m.AlphaSource)
		a, ok := agg[eng]
		if !ok {
			a = &alphaAgg{engine: eng}
			agg[eng] = a
		}
		a.count += m.TotalTrades
		a.netPnL += m.NetProfit
		a.grossPnL += m.GrossProfit
		a.sumWin += m.GrossProfit
		a.sumLoss += m.GrossLoss
		a.wins += int(math.Round(float64(m.TotalTrades) * m.WinRate))
		if m.MaxDrawdown > a.maxDD {
			a.maxDD = m.MaxDrawdown
		}
	}

	// Merge walk-forward consistency
	wfByStrat := make(map[string]*phase23b.WFReport, len(wfReports))
	for i := range wfReports {
		wfByStrat[wfReports[i].StrategyName] = &wfReports[i]
	}

	for _, m := range metrics {
		eng := canonicalAlpha(m.AlphaSource)
		a := agg[eng]
		if wf, ok := wfByStrat[m.StrategyName]; ok {
			for _, w := range wf.Windows {
				a.wfWindows++
				if w.ValidMetrics.ProfitFactor > 1.0 {
					a.wfWinWindows++
				}
			}
		}
		if mc, ok := mcResults[m.StrategyName]; ok {
			a.mcRuns += mc.Simulations
			a.mcProfitable += int(math.Round(mc.PctProfitable * float64(mc.Simulations)))
		}
	}

	results := make([]AlphaChampionship24, 0, len(agg))
	for _, a := range agg {
		if a.count == 0 {
			continue
		}
		pf := 0.0
		if a.sumLoss > 0 {
			pf = a.sumWin / a.sumLoss
		}
		wr := 0.0
		if a.count > 0 {
			wr = float64(a.wins) / float64(a.count)
		}
		wfConsistency := 0.0
		if a.wfWindows > 0 {
			wfConsistency = float64(a.wfWinWindows) / float64(a.wfWindows)
		}
		execRetention := 0.0
		if a.grossPnL > 0 {
			execRetention = a.netPnL / a.grossPnL
		}

		// Compute aggregate Sharpe from netPnL / count (proxy)
		avgPnL := a.netPnL / float64(a.count)
		sharpe := avgPnL / math.Max(1.0, math.Abs(avgPnL)*0.3) * math.Sqrt(annualTradingPeriods)
		if math.IsNaN(sharpe) || math.IsInf(sharpe, 0) {
			sharpe = 0
		}
		sortino := sharpe * 1.2 // approximation for aggregate

		mcStability := phase22f.MCFailed
		if a.mcRuns > 0 {
			pctProf := float64(a.mcProfitable) / float64(a.mcRuns)
			switch {
			case pctProf >= 0.90:
				mcStability = phase22f.MCRobust
			case pctProf >= 0.75:
				mcStability = phase22f.MCStable22
			case pctProf >= 0.60:
				mcStability = phase22f.MCMarginal
			case pctProf >= 0.40:
				mcStability = phase22f.MCUnstable
			}
		}

		verdict := alphaVerdict(pf, sharpe, wfConsistency, mcStability)

		capitalEfficiency := 0.0
		if a.maxDD > 0 {
			capitalEfficiency = a.netPnL / a.maxDD
		}

		results = append(results, AlphaChampionship24{
			Engine:            a.engine,
			TradeCount:        a.count,
			ProfitFactor:      pf,
			Sharpe:            sharpe,
			Sortino:           sortino,
			Expectancy:        a.netPnL / float64(a.count),
			NetPnLUSD:         a.netPnL,
			WinRate:           wr,
			MaxDD:             a.maxDD,
			MCStability:       mcStability,
			WFConsistency:     wfConsistency,
			ExecRetention:     execRetention,
			CapitalEfficiency: capitalEfficiency,
			Verdict:           verdict,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return alphaScore(results[i]) > alphaScore(results[j])
	})
	for i := range results {
		results[i].Rank = i + 1
	}
	return results
}

func alphaScore(a AlphaChampionship24) float64 {
	return normClamp24(a.ProfitFactor, 1.0, 2.5)*0.35 +
		normClamp24(a.Sharpe, 0, 3)*0.30 +
		normClamp24(a.Expectancy, 0, 500)*0.20 +
		a.WFConsistency*0.15
}

func alphaVerdict(pf, sharpe, wfConsistency float64, mc phase22f.MCStabilityF22) string {
	if pf >= 1.50 && sharpe >= 2.0 && wfConsistency >= 0.70 && mc >= phase22f.MCStable22 {
		return "CHAMPION"
	}
	if pf >= 1.30 && sharpe >= 1.5 && wfConsistency >= 0.60 {
		return "STRONG"
	}
	if pf >= 1.15 && sharpe >= 1.0 {
		return "MODERATE"
	}
	if pf >= 1.0 {
		return "WEAK"
	}
	return "ELIMINATED"
}

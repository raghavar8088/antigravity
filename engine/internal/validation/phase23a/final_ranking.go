package phase23a

import (
	"fmt"
	"sort"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

// BuildFinalRanking constructs the Phase 12 ranked strategy table.
// Uses composite evidence from backtest, walk-forward, MC, and edge certification.
func BuildFinalRanking(
	reports []WalkForwardReport,
	certs []EdgeCertification,
	mcResults map[string]phase22f.MonteCarloF22,
	tiers []phase22f.TierClassification,
	extStats []phase22f.ExtendedStats,
	capAlloc []phase22f.CapitalAllocationEntry,
	totalCapital float64,
) []RankedStrategy {
	// Build lookup maps
	certMap := make(map[string]EdgeCertification, len(certs))
	for _, c := range certs {
		certMap[c.StrategyID] = c
	}
	tierMap := make(map[string]phase22f.TierClassification, len(tiers))
	for _, tc := range tiers {
		tierMap[tc.StrategyID] = tc
	}
	mcMap := mcResults
	statsMap := make(map[string]phase22f.ExtendedStats, len(extStats))
	for _, s := range extStats {
		statsMap[s.Base.StrategyID] = s
	}
	allocMap := make(map[string]phase22f.CapitalAllocationEntry, len(capAlloc))
	for _, ca := range capAlloc {
		allocMap[ca.StrategyID] = ca
	}

	type entry struct {
		rs    RankedStrategy
		score float64
	}
	entries := make([]entry, 0, len(reports))

	for _, rpt := range reports {
		// gather all validation trades for this strategy
		var trades []phase22e.TradeRecord
		for _, w := range rpt.Windows {
			trades = append(trades, w.ValidResult.Trades...)
		}
		if len(trades) == 0 {
			continue
		}

		pf, sharpe, exp, wr := sampleMetrics23(trades)
		pnls := make([]float64, len(trades))
		for i, t := range trades {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDD23(pnls, totalCapital/20)
		ror := estimateRoR23(pnls, totalCapital/20)

		// CAGR from first window train result
		cagr := 0.0
		if len(rpt.Windows) > 0 {
			allTrades := StrategyBacktestTrades(reports, rpt.StrategyID)
			if len(allTrades) > 0 {
				days := 0
				if t0, t1 := allTrades[0].EntryTime, allTrades[len(allTrades)-1].ExitTime; !t1.IsZero() {
					days = int(t1.Sub(t0).Hours() / 24)
				}
				if days < 1 {
					days = 365
				}
				initialNAV := totalCapital / 20
				finalNAV := initialNAV
				for _, t := range allTrades {
					finalNAV += t.NetPnLUSD
				}
				cagr = computeCAGR(initialNAV, finalNAV, days)
			}
		}

		// Sortino from extended stats
		sortino := 0.0
		if s, ok := statsMap[rpt.StrategyID]; ok {
			sortino = s.SortinoRatio
		}

		// MC
		mc := mcMap[rpt.StrategyID]

		// regime strength
		rs := regimeStrengthLabel(trades)

		// tier
		tc := tierMap[rpt.StrategyID]

		// capital recommendation
		ca := allocMap[rpt.StrategyID]
		capRec := "0% (not approved)"
		if ca.AllocationPct > 0 {
			capRec = fmt.Sprintf("%.0f%% ($%.0f)", ca.AllocationPct, ca.AllocationUSD)
		}

		// Composite ranking score: same weights as Phase 22F top-20
		score := normScore(pf, 1.0, 2.0)*0.25 +
			normScore(sharpe, 0, 3.0)*0.20 +
			normScore(exp, -100, 500)*0.20 +
			normScore(100-dd, 50, 100)*0.15 +
			normScore(mc.ProbabilityGrow*100, 50, 100)*0.10 +
			normScore(rpt.Consistency, 0, 100)*0.10

		entries = append(entries, entry{
			score: score,
			rs: RankedStrategy{
				StrategyID:        rpt.StrategyID,
				StrategyName:      rpt.StrategyName,
				TradeCount:        len(trades),
				WinRate:           wr,
				ProfitFactor:      pf,
				Sharpe:            sharpe,
				Sortino:           sortino,
				CAGR:              cagr,
				Expectancy:        exp,
				MaxDD:             dd,
				RiskOfRuin:        ror,
				MCTier:            mc.Stability,
				RegimeStrength:    rs,
				WFConsistency:     rpt.Consistency,
				CapitalAllocation: capRec,
				CertificationTier: tc.Tier,
			},
		})
	}

	// sort by composite score
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].score > entries[j].score
	})

	ranked := make([]RankedStrategy, len(entries))
	for i, e := range entries {
		ranked[i] = e.rs
		ranked[i].Rank = i + 1
	}
	return ranked
}

func regimeStrengthLabel(trades []phase22e.TradeRecord) string {
	n := regimeRobustness23(trades)
	switch {
	case n >= 4:
		return "STRONG"
	case n >= 2:
		return "MODERATE"
	default:
		return "WEAK"
	}
}

func regimeRobustness23(trades []phase22e.TradeRecord) int {
	type rdata struct{ gw, gl float64 }
	regimes := make(map[phase22e.Regime]*rdata)
	for _, t := range trades {
		d, ok := regimes[t.Regime]
		if !ok {
			d = &rdata{}
			regimes[t.Regime] = d
		}
		if t.NetPnLUSD >= 0 {
			d.gw += t.NetPnLUSD
		} else {
			d.gl += -t.NetPnLUSD
		}
	}
	n := 0
	for _, d := range regimes {
		if d.gl > 0 && d.gw/d.gl >= 1.0 {
			n++
		}
	}
	return n
}

// normScore normalises v to [0,1] clipped to [min,max].
func normScore(v, min, max float64) float64 {
	if max == min {
		return 0.5
	}
	n := (v - min) / (max - min)
	if n < 0 {
		n = 0
	}
	if n > 1 {
		n = 1
	}
	return n
}

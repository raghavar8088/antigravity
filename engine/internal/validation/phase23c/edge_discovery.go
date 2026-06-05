package phase23c

import (
	"fmt"
	"sort"

	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// RankAllStrategies builds the full evidence-based ranking of every strategy.
// Uses only certified trade data from Phase 23B.
func RankAllStrategies(source *phase23b.Phase23BResult) []RankedStrategy23C {
	ranked := make([]RankedStrategy23C, 0, len(source.Metrics))

	// Build WF lookup
	wfByName := map[string]phase23b.WFReport{}
	for _, wf := range source.WalkForward {
		wfByName[wf.StrategyName] = wf
	}

	// Build replay lookup for category/alpha
	replayByName := map[string]phase23b.StrategyReplayResult{}
	for _, r := range source.StrategyReplays {
		replayByName[r.StrategyName] = r
	}

	// Build cert lookup
	certByName := map[string]phase23b.CapCertResult{}
	for _, c := range source.CapCertifications {
		certByName[c.StrategyName] = c
	}

	for name, m := range source.Metrics {
		if m.TotalTrades == 0 {
			continue
		}
		mc := source.MonteCarlo[name]
		cert := certByName[name]
		replay := replayByName[name]
		wf := wfByName[name]

		score := CompositeScore(m, mc)

		// Regime robust
		regimeRobust := false
		for _, rp := range source.RegimeProfiles {
			if rp.StrategyName == name {
				regimeRobust = rp.RegimeRobust
				break
			}
		}

		deploy := deploymentStatus(cert.Tier, m)
		cagr := 0.0
		if replay.CAGR != 0 {
			cagr = replay.CAGR
		}

		ranked = append(ranked, RankedStrategy23C{
			StrategyName:     name,
			AlphaSource:      replay.AlphaSource,
			Family:           replay.Category,
			TradeCount:       m.TotalTrades,
			WinRate:          m.WinRate,
			ProfitFactor:     m.ProfitFactor,
			Sharpe:           m.Sharpe,
			Sortino:          m.Sortino,
			CAGR:             cagr,
			Expectancy:       m.Expectancy,
			MaxDD:            m.MaxDrawdown,
			RiskOfRuin:       m.RiskOfRuin,
			MCTier:           mc.Stability,
			CapTier:          cert.Tier,
			RegimeRobust:     regimeRobust,
			WFConsistency:    wf.Consistency,
			CompositeScore:   score,
			AllocationRec:    fmt.Sprintf("%.1f%% ($%.0f)", cert.AllocationPct, cert.AllocationUSD),
			DeploymentStatus: deploy,
		})
	}

	// Sort by composite score descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].CompositeScore > ranked[j].CompositeScore
	})

	// Assign ranks
	for i := range ranked {
		ranked[i].Rank = i + 1
	}

	return ranked
}

// SelectTop returns the top N strategies from the full ranked list.
func SelectTop(ranked []RankedStrategy23C, n int) []RankedStrategy23C {
	if len(ranked) <= n {
		return ranked
	}
	return ranked[:n]
}

// BuildAlphaChampionship ranks alpha engines by aggregated real-trade performance.
func BuildAlphaChampionship(source *phase23b.Phase23BResult) []AlphaChampionResult {
	type alphaAgg struct {
		trades     []phase23b.CertifiedTrade
		strategies []string
	}
	agg := make(map[string]*alphaAgg)

	for _, r := range source.StrategyReplays {
		alpha := r.AlphaSource
		if _, ok := agg[alpha]; !ok {
			agg[alpha] = &alphaAgg{}
		}
		agg[alpha].trades = append(agg[alpha].trades, r.Trades...)
		agg[alpha].strategies = append(agg[alpha].strategies, r.StrategyName)
	}

	// Cost lookup for edge retention
	costByStrat := map[string]phase23b.CostBreakdown{}
	for _, cb := range source.CostBreakdowns {
		costByStrat[cb.StrategyName] = cb
	}

	results := make([]AlphaChampionResult, 0, len(agg))
	for alphaName, data := range agg {
		if len(data.trades) == 0 {
			continue
		}

		m := phase23b.ComputeMetrics(alphaName, data.trades)
		mc := source.MonteCarlo[data.strategies[0]] // use first strat's MC as proxy
		if len(data.strategies) > 1 {
			// Use worst MC stability across the strategies as conservative estimate
			for _, sn := range data.strategies[1:] {
				s2 := source.MonteCarlo[sn]
				if mcStabilityScore(s2.Stability) < mcStabilityScore(mc.Stability) {
					mc = s2
				}
			}
		}

		// Edge retention: average across strategies in this alpha
		totalRetention, retCount := 0.0, 0
		for _, sn := range data.strategies {
			if cb, ok := costByStrat[sn]; ok && cb.EdgeRetention > 0 {
				totalRetention += cb.EdgeRetention
				retCount++
			}
		}
		retention := 0.0
		if retCount > 0 {
			retention = totalRetention / float64(retCount)
		}

		// Regime robust
		robustCount := 0
		for _, sn := range data.strategies {
			for _, rp := range source.RegimeProfiles {
				if rp.StrategyName == sn && rp.RegimeRobust {
					robustCount++
					break
				}
			}
		}

		// Net PnL
		netPnL := 0.0
		for _, t := range data.trades {
			netPnL += t.NetPnLUSD
		}

		results = append(results, AlphaChampionResult{
			AlphaEngine:   alphaName,
			TradeCount:    len(data.trades),
			ProfitFactor:  m.ProfitFactor,
			Sharpe:        m.Sharpe,
			Expectancy:    m.Expectancy,
			NetPnLUSD:     netPnL,
			WinRate:       m.WinRate,
			Stability:     mc.Stability,
			RegimeRobust:  robustCount >= len(data.strategies)/2,
			ExecRetention: retention,
			Verdict:       alphaVerdict(m, mc),
		})
	}

	// Sort by PF descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].ProfitFactor > results[j].ProfitFactor
	})
	for i := range results {
		results[i].Rank = i + 1
	}

	return results
}

// BuildTop5Portfolio constructs the optimal 5-strategy portfolio from the top-ranked.
func BuildTop5Portfolio(ranked []RankedStrategy23C, metrics map[string]phase23b.Metrics23B, totalCapital float64) PortfolioProfile {
	// Filter for deployable strategies
	var deployable []RankedStrategy23C
	for _, r := range ranked {
		if r.DeploymentStatus == "DEPLOY_NOW" || r.DeploymentStatus == "PILOT" {
			deployable = append(deployable, r)
		}
	}
	if len(deployable) == 0 {
		deployable = ranked // fallback
	}

	top5 := deployable
	if len(top5) > 5 {
		top5 = top5[:5]
	}

	// Score-proportional weight allocation
	totalScore := 0.0
	for _, r := range top5 {
		totalScore += r.CompositeScore
	}

	entries := make([]PortfolioEntry, len(top5))
	for i, r := range top5 {
		weight := 0.0
		if totalScore > 0 {
			weight = r.CompositeScore / totalScore * 100
		}
		entries[i] = PortfolioEntry{
			Rank:           i + 1,
			StrategyName:   r.StrategyName,
			Weight:         weight,
			AllocationUSD:  totalCapital * weight / 100,
			ExpectedSharpe: r.Sharpe,
			RiskContrib:    weight, // simplified: equal to weight (true risk parity needs covariance matrix)
		}
	}

	// Portfolio-level metrics (weighted average)
	portPF, portSharpe, portCAGR, portDD := 0.0, 0.0, 0.0, 0.0
	for _, e := range entries {
		r2 := top5[e.Rank-1]
		w := e.Weight / 100
		portPF += r2.ProfitFactor * w
		portSharpe += r2.Sharpe * w
		portCAGR += r2.CAGR * w
		portDD += r2.MaxDD * w
	}

	return PortfolioProfile{
		Name:                 "TOP5 INSTITUTIONAL PORTFOLIO",
		Entries:              entries,
		ExpectedCAGR:         portCAGR,
		ExpectedPF:           portPF,
		ExpectedSharpe:       portSharpe,
		ExpectedMaxDD:        portDD,
		CorrelationNote:      "Full covariance matrix requires live co-trade data; weights approximate diversification",
		DiversificationScore: diversificationScore(top5),
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func deploymentStatus(tier phase23b.CapCertTier, m phase23b.Metrics23B) string {
	switch tier {
	case phase23b.CapTierInstitutional, phase23b.CapTierFull:
		return "DEPLOY_NOW"
	case phase23b.CapTierLimited:
		return "PILOT"
	case phase23b.CapTierPilot:
		return "PAPER"
	default:
		return "RETIRED"
	}
}

func alphaVerdict(m phase23b.Metrics23B, mc phase23b.RealMCResult) string {
	switch {
	case m.ProfitFactor >= 1.5 && m.Sharpe >= 2.0 && mc.Stability == phase22f.MCRobust:
		return "CHAMPION"
	case m.ProfitFactor >= 1.3 && m.Sharpe >= 1.5:
		return "STRONG"
	case m.ProfitFactor >= 1.1:
		return "MODERATE"
	case m.ProfitFactor >= 1.0:
		return "WEAK"
	default:
		return "ELIMINATED"
	}
}

func diversificationScore(strategies []RankedStrategy23C) float64 {
	if len(strategies) <= 1 {
		return 0
	}
	// Count unique alpha sources as proxy for diversification
	alphas := map[string]bool{}
	families := map[string]bool{}
	for _, s := range strategies {
		alphas[s.AlphaSource] = true
		families[s.Family] = true
	}
	alphaDiv := float64(len(alphas)) / float64(len(strategies))
	familyDiv := float64(len(families)) / float64(len(strategies))
	return (alphaDiv + familyDiv) / 2 * 100
}

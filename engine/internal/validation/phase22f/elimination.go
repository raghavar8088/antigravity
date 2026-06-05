package phase22f

import (
	"fmt"

	"antigravity-engine/internal/validation/phase22e"
)

// IdentifyEliminationCandidates implements the Phase 12 strategy elimination engine.
// A strategy is flagged when it meets any of the hard elimination criteria.
func IdentifyEliminationCandidates(
	trades []phase22e.TradeRecord,
	mcResults map[string]MonteCarloF22,
) []EliminationCandidate {
	byStrat := GroupTradesByStrategy(trades)
	var candidates []EliminationCandidate

	for id, ts := range byStrat {
		if len(ts) < 10 {
			continue
		}
		pf, sharpe, exp, _ := sampleMetrics(ts)
		pnls := make([]float64, len(ts))
		for i, t := range ts {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDrawdownPctLocal(pnls, InitialNAV/float64(len(byStrat)))

		mc := mcResults[id]
		stability := mc.Stability
		if mc.Simulations == 0 {
			stability = MCMarginal // default when MC not run
		}

		name, family := "", ""
		if len(ts) > 0 {
			name = ts[0].StrategyName
			family = ts[0].Family
		}

		reasons, severity := evaluateElimination(pf, sharpe, exp, dd, mc.ProbabilityRuin, stability, len(ts))
		if len(reasons) == 0 {
			continue
		}

		candidates = append(candidates, EliminationCandidate{
			StrategyID:   id,
			StrategyName: name,
			Family:       family,
			Severity:     severity,
			Reasons:      reasons,
			ProfitFactor: pf,
			Expectancy:   exp,
			Sharpe:       sharpe,
			MaxDrawdown:  dd,
			MCStability:  stability,
			TotalTrades:  len(ts),
		})
	}
	return candidates
}

func evaluateElimination(
	pf, sharpe, exp, maxDD, ror float64,
	stability MCStabilityF22,
	tradeCount int,
) (reasons []string, severity EliminationSeverity) {
	severity = EliminateConditional

	// ── Hard IMMEDIATE criteria ───────────────────────────────────────────────
	if pf < EliminationMinPF {
		reasons = append(reasons, fmt.Sprintf("PF=%.3f < %.2f (edge absent)", pf, EliminationMinPF))
		severity = EliminateImmediate
	}
	if exp < 0 {
		reasons = append(reasons, fmt.Sprintf("Expectancy=$%.2f (negative — lose money per trade)", exp))
		severity = EliminateImmediate
	}
	if stability == MCFailed {
		reasons = append(reasons, "Monte Carlo: FAILED — catastrophic ruin risk in simulation")
		severity = EliminateImmediate
	}
	if ror > EliminationMaxRoR {
		reasons = append(reasons, fmt.Sprintf("Risk of Ruin=%.1f%% > %.0f%%", ror*100, EliminationMaxRoR*100))
		if severity != EliminateImmediate {
			severity = EliminateImmediate
		}
	}

	// ── RECOMMENDED criteria ──────────────────────────────────────────────────
	if sharpe < EliminationMinSharpe && severity != EliminateImmediate {
		reasons = append(reasons, fmt.Sprintf("Sharpe=%.2f < %.2f (risk-adjusted return poor)", sharpe, EliminationMinSharpe))
		if severity == EliminateConditional {
			severity = EliminateRecommended
		}
	}
	if maxDD > EliminationMaxDD && severity != EliminateImmediate {
		reasons = append(reasons, fmt.Sprintf("MaxDD=%.1f%% > %.0f%% (unacceptable capital erosion)", maxDD, EliminationMaxDD))
		if severity == EliminateConditional {
			severity = EliminateRecommended
		}
	}
	if stability == MCUnstable && severity == EliminateConditional {
		reasons = append(reasons, "Monte Carlo: UNSTABLE — significant ruin risk under adverse paths")
		severity = EliminateRecommended
	}

	// ── CONDITIONAL criteria ──────────────────────────────────────────────────
	if tradeCount < 30 && len(reasons) == 0 {
		reasons = append(reasons, fmt.Sprintf("insufficient trades: %d < 30 (cannot certify)", tradeCount))
		// severity stays CONDITIONAL
	}
	if stability == MCMarginal && len(reasons) == 0 {
		reasons = append(reasons, "Monte Carlo: MARGINAL — borderline stability, monitor closely")
		// severity stays CONDITIONAL
	}

	return reasons, severity
}

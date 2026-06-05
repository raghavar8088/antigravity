package phase22e

import "fmt"

// RetirementSeverity ranks how urgently a strategy should be retired.
type RetirementSeverity string

const (
	SeverityImmediate   RetirementSeverity = "IMMEDIATE"   // live capital must be removed now
	SeverityRecommended RetirementSeverity = "RECOMMENDED" // retire at next review cycle
	SeverityConditional RetirementSeverity = "CONDITIONAL" // retire unless improved in 30 days
)

// RetirementCandidate represents a strategy flagged for retirement.
type RetirementCandidate struct {
	StrategyID   string
	StrategyName string
	Family       string
	Severity     RetirementSeverity
	Reasons      []string
	ProfitFactor float64
	Expectancy   float64
	Sharpe       float64
	MaxDrawdown  float64
	MCStability  MCStability
	TotalTrades  int
}

// IdentifyRetirementCandidates evaluates each strategy against retirement criteria
// and returns candidates ordered by severity (immediate first).
func IdentifyRetirementCandidates(strategies []StrategyMetrics, mcResults map[string]MonteCarloResult) []RetirementCandidate {
	var candidates []RetirementCandidate

	for _, s := range strategies {
		var reasons []string
		severity := SeverityConditional

		// Criterion 1: Profit factor below 1.00 (losing money absolutely)
		if s.ProfitFactor < 1.00 && s.TotalTrades >= MinStrategyTrades {
			reasons = append(reasons, fmt.Sprintf("Profit Factor %.2f < 1.00 — strategy loses money", s.ProfitFactor))
			severity = SeverityImmediate
		}

		// Criterion 2: Negative expectancy
		if s.Expectancy < 0 {
			reasons = append(reasons, fmt.Sprintf("Negative expectancy $%.2f per trade", s.Expectancy))
			if severity != SeverityImmediate {
				severity = SeverityImmediate
			}
		}

		// Criterion 3: Sharpe below 0.50
		if s.Sharpe < 0.50 && s.TotalTrades >= MinStrategyTrades {
			reasons = append(reasons, fmt.Sprintf("Sharpe Ratio %.2f < 0.50 — insufficient risk-adjusted return", s.Sharpe))
			if severity == SeverityConditional {
				severity = SeverityRecommended
			}
		}

		// Criterion 4: Persistent drawdown > 15%
		if s.MaxDrawdown > 15.0 {
			reasons = append(reasons, fmt.Sprintf("Max Drawdown %.1f%% exceeds 15%% persistent drawdown limit", s.MaxDrawdown))
			if severity == SeverityConditional {
				severity = SeverityRecommended
			}
		}

		// Criterion 5: Monte Carlo unstable or untradable
		mcStab := MCStable
		if mc, ok := mcResults[s.StrategyID]; ok {
			mcStab = mc.Stability
			if mcStab == MCUntradable {
				reasons = append(reasons, fmt.Sprintf("Monte Carlo: UNTRADABLE — ruin probability %.0f%%", mc.ProbabilityRuin*100))
				severity = SeverityImmediate
			} else if mcStab == MCUnstable {
				reasons = append(reasons, fmt.Sprintf("Monte Carlo: UNSTABLE — ruin probability %.0f%%, worst DD %.1f%%", mc.ProbabilityRuin*100, mc.WorstDD))
				if severity == SeverityConditional {
					severity = SeverityRecommended
				}
			}
		}

		// Criterion 6: No statistical edge (enough trades but still failing significance)
		if !s.StatSig && s.TotalTrades >= 50 {
			reasons = append(reasons, fmt.Sprintf("No statistical edge confirmed — %d trades, significance test failed", s.TotalTrades))
			if severity == SeverityConditional {
				severity = SeverityRecommended
			}
		}

		if len(reasons) == 0 {
			continue
		}

		candidates = append(candidates, RetirementCandidate{
			StrategyID:   s.StrategyID,
			StrategyName: s.StrategyName,
			Family:       s.Family,
			Severity:     severity,
			Reasons:      reasons,
			ProfitFactor: s.ProfitFactor,
			Expectancy:   s.Expectancy,
			Sharpe:       s.Sharpe,
			MaxDrawdown:  s.MaxDrawdown,
			MCStability:  mcStab,
			TotalTrades:  s.TotalTrades,
		})
	}

	// sort: IMMEDIATE first, then RECOMMENDED, then CONDITIONAL
	severityOrder := map[RetirementSeverity]int{
		SeverityImmediate:   0,
		SeverityRecommended: 1,
		SeverityConditional: 2,
	}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if severityOrder[candidates[i].Severity] > severityOrder[candidates[j].Severity] {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
	return candidates
}

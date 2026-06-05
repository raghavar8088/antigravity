package phase23c

import (
	"fmt"

	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// EliminateStrategies permanently eliminates strategies that fail the hard gates.
// Every elimination includes exact evidence traceable to real trade data.
func EliminateStrategies(source *phase23b.Phase23BResult) []EliminationRecord {
	var eliminated []EliminationRecord

	certByName := map[string]phase23b.CapCertResult{}
	for _, c := range source.CapCertifications {
		certByName[c.StrategyName] = c
	}

	for name, m := range source.Metrics {
		mc := source.MonteCarlo[name]
		cert := certByName[name]

		var reasons []string

		if m.ProfitFactor < phase23b.ElimMinPF {
			reasons = append(reasons, fmt.Sprintf("PF=%.3f < %.2f", m.ProfitFactor, float64(phase23b.ElimMinPF)))
		}
		if m.Expectancy < 0 {
			reasons = append(reasons, fmt.Sprintf("Expectancy=$%.2f (negative)", m.Expectancy))
		}
		if mc.Stability == phase22f.MCFailed {
			reasons = append(reasons, "Monte Carlo=FAILED")
		}
		if m.RiskOfRuin > phase23b.ElimMaxRoR {
			reasons = append(reasons, fmt.Sprintf("RoR=%.1f%% > %.0f%%", m.RiskOfRuin*100, phase23b.ElimMaxRoR*100))
		}
		if m.MaxDrawdown > phase23b.ElimMaxDD {
			reasons = append(reasons, fmt.Sprintf("MaxDD=%.1f%% > %.0f%%", m.MaxDrawdown, float64(phase23b.ElimMaxDD)))
		}
		if m.TotalTrades < 30 {
			reasons = append(reasons, fmt.Sprintf("Insufficient sample: %d trades (minimum 30)", m.TotalTrades))
		}

		if len(reasons) == 0 {
			continue
		}

		evidence := fmt.Sprintf("Trades=%d PF=%.3f Sharpe=%.2f Expectancy=$%.2f MaxDD=%.1f%% RoR=%.1f%% MC=%s Cert=%s",
			m.TotalTrades, m.ProfitFactor, m.Sharpe, m.Expectancy,
			m.MaxDrawdown, m.RiskOfRuin*100, mc.Stability, cert.Tier)

		eliminated = append(eliminated, EliminationRecord{
			StrategyName: name,
			Reason:       joinReasons(reasons),
			ProfitFactor: m.ProfitFactor,
			Expectancy:   m.Expectancy,
			RiskOfRuin:   m.RiskOfRuin,
			MaxDD:        m.MaxDrawdown,
			MCTier:       mc.Stability,
			TradeCount:   m.TotalTrades,
			Evidence:     evidence,
		})
	}

	return eliminated
}

func joinReasons(reasons []string) string {
	result := ""
	for i, r := range reasons {
		if i > 0 {
			result += "; "
		}
		result += r
	}
	return result
}

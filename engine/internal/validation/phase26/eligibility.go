package phase26

import (
	"sort"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
)

// BuildEligibility filters Phase24 CapCerts using Phase25 escalation outcomes.
// Strategies that are retired/demoted in Phase 25, or that have a failed edge,
// are excluded. The result is sorted by Phase24 CompositeScore descending.
func BuildEligibility(p24 *phase24.Phase24Result, p25 *phase25.Phase25Result) []EligibleStrategy {
	// Build lookup maps
	escalationMap := make(map[string]phase25.CapEscalationRecord)
	for _, e := range p25.CapEscalation {
		escalationMap[e.StrategyName] = e
	}

	driftMap := make(map[string]phase25.EdgeStatus)
	for _, d := range p25.EdgeDrift {
		driftMap[d.StrategyName] = d.EdgeStatus
	}

	liveMetricsMap := p25.LiveMetrics // map[string]LiveMetrics

	var result []EligibleStrategy

	for _, cert := range p24.CapCerts {
		name := cert.StrategyName

		// Skip retired/paper-only in Phase 24
		if cert.Tier == phase24.CapTier24Retired || cert.Tier == phase24.CapTier24PaperOnly {
			result = append(result, EligibleStrategy{
				StrategyName:    name,
				Phase24Tier:     cert.Tier,
				EligibleTier:    "EXCLUDED",
				ExclusionReason: "Phase24 tier: " + string(cert.Tier),
			})
			continue
		}

		// Check Phase 25 escalation decision
		esc, hasEsc := escalationMap[name]
		if hasEsc {
			if esc.PromotionStatus == "RETIRE" || esc.PromotionStatus == "DEMOTE" {
				result = append(result, EligibleStrategy{
					StrategyName:    name,
					Phase24Tier:     cert.Tier,
					EligibleTier:    "EXCLUDED",
					ExclusionReason: "Phase25 escalation: " + esc.PromotionStatus,
				})
				continue
			}
		}

		// Check edge drift
		edgeStatus, hasEdge := driftMap[name]
		if hasEdge && edgeStatus == phase25.EdgeFailed {
			result = append(result, EligibleStrategy{
				StrategyName:    name,
				Phase24Tier:     cert.Tier,
				EligibleTier:    "EXCLUDED",
				ExclusionReason: "Phase25 EdgeStatus: EDGE_FAILED",
			})
			continue
		}

		// Determine eligible tier
		eligibleTier := "TIER1"
		if cert.Tier == phase24.CapTier24Institutional || cert.Tier == phase24.CapTier24Full {
			eligibleTier = "TIER1"
		} else {
			eligibleTier = "TIER2"
		}

		// Fetch Phase24 metrics
		p24m := p24.Metrics[name]
		// Fetch Phase25 live metrics
		lm := liveMetricsMap[name]

		// Determine escalation tier
		var p25Tier phase25.CapLadderTier
		if hasEsc {
			p25Tier = esc.RecommendedTier
		}

		var edgeSt phase25.EdgeStatus
		if hasEdge {
			edgeSt = edgeStatus
		}

		result = append(result, EligibleStrategy{
			StrategyName:      name,
			Phase24Tier:       cert.Tier,
			Phase25Tier:       p25Tier,
			Phase24Score:      p24m.CompositeScore,
			Phase25Drift:      edgeSt,
			Phase24PF:         p24m.ProfitFactor,
			Phase24Sharpe:     p24m.Sharpe,
			Phase24DD:         p24m.MaxDrawdown,
			Phase24Expectancy: p24m.Expectancy,
			Phase25PF:         lm.ProfitFactor,
			Phase25Sharpe:     lm.Sharpe,
			EligibleTier:      eligibleTier,
		})
	}

	// Sort by Phase24 CompositeScore descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Phase24Score > result[j].Phase24Score
	})

	return result
}

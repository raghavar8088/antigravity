package phase25

import (
	"antigravity-engine/internal/validation/phase24"
)

// BuildEligibility imports Phase 24 capital certification output and classifies
// each strategy as approved or excluded for live-forward validation.
//
// Approved tiers: INSTITUTIONAL, FULL_CAPITAL, LIMITED_CAPITAL.
// Excluded tiers: RETIRE, PAPER_ONLY, FAILED_CERTIFICATION (Phase 24 gate).
func BuildEligibility(certs []phase24.CapCertification24, metrics map[string]phase24.ExtendedMetrics) []EligibilityRecord {
	records := make([]EligibilityRecord, 0, len(certs))

	for _, c := range certs {
		m := metrics[c.StrategyName]
		approved, reason := isApproved(c.Tier)
		records = append(records, EligibilityRecord{
			StrategyName:         c.StrategyName,
			Phase24Tier:          c.Tier,
			HistoricalPF:         m.ProfitFactor,
			HistoricalSharpe:     m.Sharpe,
			HistoricalDD:         m.MaxDrawdown,
			HistoricalExpectancy: m.Expectancy,
			ApprovedForLive:      approved,
			ExclusionReason:      reason,
		})
	}
	return records
}

func isApproved(tier phase24.CapTier24) (bool, string) {
	switch tier {
	case phase24.CapTier24Institutional:
		return true, ""
	case phase24.CapTier24Full:
		return true, ""
	case phase24.CapTier24Limited:
		return true, ""
	case phase24.CapTier24PaperOnly:
		return false, "Phase 24 tier PAPER_ONLY — excluded from live capital"
	case phase24.CapTier24Retired:
		return false, "Phase 24 tier RETIRE — permanently excluded"
	default:
		return false, "Failed Phase 24 certification"
	}
}

// ApprovedStrategies returns the subset of eligibility records approved for live.
func ApprovedStrategies(records []EligibilityRecord) []EligibilityRecord {
	out := make([]EligibilityRecord, 0, len(records))
	for _, r := range records {
		if r.ApprovedForLive {
			out = append(out, r)
		}
	}
	return out
}

// EligibilityMap returns a map of strategyName → EligibilityRecord.
func EligibilityMap(records []EligibilityRecord) map[string]EligibilityRecord {
	m := make(map[string]EligibilityRecord, len(records))
	for _, r := range records {
		m[r.StrategyName] = r
	}
	return m
}

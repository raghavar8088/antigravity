package phase26

import (
	"fmt"
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
)

// Run orchestrates all Phase 26 sub-phases and returns the master result.
func Run(p24 *phase24.Phase24Result, p25 *phase25.Phase25Result, cfg Phase26Config) (*Phase26Result, error) {
	if p24 == nil {
		return nil, fmt.Errorf("phase26: p24 result is nil")
	}
	if p25 == nil {
		return nil, fmt.Errorf("phase26: p25 result is nil")
	}

	result := &Phase26Result{
		GeneratedAt:     time.Now().UTC(),
		Config:          cfg,
		Tier2Milestones: make(map[string][]MilestoneReport),
	}

	// 26.1: Eligibility
	result.EligibleStrategies = BuildEligibility(p24, p25)
	result.TotalEligible = 0
	for _, e := range result.EligibleStrategies {
		if e.EligibleTier != "EXCLUDED" {
			result.TotalEligible++
		}
	}

	// 26.2: Edge retention and drift for all eligible strategies
	for _, e := range result.EligibleStrategies {
		if e.EligibleTier == "EXCLUDED" {
			continue
		}
		p24m := p24.Metrics[e.StrategyName]
		lm := p25.LiveMetrics[e.StrategyName]
		result.EdgeRetention = append(result.EdgeRetention, BuildEdgeRetention(e.StrategyName, p24m, lm))
		result.DriftRecords = append(result.DriftRecords, BuildDriftRecord(e.StrategyName, p25.LiveTrades))
	}

	// 26.3: Tier 1 certification (top strategies, min Tier1MinTrades)
	tier1Count := 0
	for _, e := range result.EligibleStrategies {
		if e.EligibleTier == "EXCLUDED" {
			continue
		}
		if tier1Count >= cfg.Tier1Count {
			break
		}

		cert := BuildTierCertification(e, p24, p25)

		// Check if insufficient evidence
		if cert.TotalTrades < cfg.Tier1MinTrades {
			result.InsufficientEvidenceStrategies = append(result.InsufficientEvidenceStrategies, e.StrategyName)
		}

		ApplyCertificationGates(&cert, cfg.Tier1MinTrades)
		result.Tier1Strategies = append(result.Tier1Strategies, cert)
		tier1Count++
	}

	// 26.4: Tier 2 certification (top scorers, min Tier2MinTrades)
	tier2Count := 0
	for _, e := range result.EligibleStrategies {
		if e.EligibleTier == "EXCLUDED" {
			continue
		}
		if tier2Count >= cfg.Tier2Count {
			break
		}

		cert := BuildTierCertification(e, p24, p25)

		if cert.TotalTrades < cfg.Tier2MinTrades {
			result.InsufficientEvidenceStrategies = append(result.InsufficientEvidenceStrategies, e.StrategyName)
		}

		ApplyCertificationGates(&cert, cfg.Tier2MinTrades)
		result.Tier2Strategies = append(result.Tier2Strategies, cert)

		// Milestones for Tier 2
		trades := filterTrades(p25.LiveTrades, e.StrategyName)
		result.Tier2Milestones[e.StrategyName] = BuildMilestones(e.StrategyName, trades)
		tier2Count++
	}

	// 26.5: Classify decisions
	allCerts := append(result.Tier1Strategies, result.Tier2Strategies...)
	seen := make(map[string]bool)
	for _, c := range allCerts {
		if seen[c.StrategyName] {
			continue
		}
		seen[c.StrategyName] = true
		switch c.Decision {
		case DecisionPromote:
			result.InstitutionalCertified = append(result.InstitutionalCertified, c.StrategyName)
			result.TotalCertified++
		case DecisionMaintain:
			result.Maintained = append(result.Maintained, c.StrategyName)
		case DecisionReduce:
			result.Reduced = append(result.Reduced, c.StrategyName)
		case DecisionDemote:
			result.Demoted = append(result.Demoted, c.StrategyName)
		case DecisionRetire:
			result.Retired = append(result.Retired, c.StrategyName)
		}
	}

	// 26.6: Build final verdict
	result.FinalVerdict = buildVerdict(result, p25)

	return result, nil
}

func buildVerdict(r *Phase26Result, p25 *phase25.Phase25Result) Phase26Verdict {
	v := Phase26Verdict{
		GeneratedAt:          time.Now().UTC(),
		HasVerifiedEdge:      r.TotalCertified > 0,
		CertifiedStrategies:  r.InstitutionalCertified,
		CapitalReadyStrategies: r.InstitutionalCertified,
		RetirementList:       r.Retired,
		PlatformPF:           p25.PlatformLivePF,
		PlatformSharpe:       p25.PlatformLiveSharpe,
		PlatformNetPnL:       p25.PlatformLiveNetPnLUSD,
	}

	if v.HasVerifiedEdge {
		v.VerifiedEdgeEvidence = fmt.Sprintf("%d strategies passed all institutional gates", r.TotalCertified)
		v.OverallDecision = "DEPLOY"
		v.Justification = fmt.Sprintf("Platform has %d certified strategies with PF=%.3f, Sharpe=%.3f", r.TotalCertified, v.PlatformPF, v.PlatformSharpe)
	} else {
		v.VerifiedEdgeEvidence = "INSUFFICIENT_EVIDENCE: no strategies passed all institutional gates"
		v.OverallDecision = "PAPER_ONLY"
		v.Justification = "No strategies met Tier 1/Tier 2 certification requirements"
	}

	return v
}

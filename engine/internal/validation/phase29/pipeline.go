package phase29

import (
	"fmt"
	"time"

	"antigravity-engine/internal/validation/phase23b"
	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
	"antigravity-engine/internal/validation/phase28"
)

// Run executes the complete Phase 29 evidence extraction pipeline.
//
// It accepts pre-computed Phase 24–28 results and produces a single master
// Phase29Result with all 10 institutional reports populated from actual
// evidence. It never fabricates data — any section lacking evidence is
// populated with INSUFFICIENT_EVIDENCE.
func Run(bundle EvidenceBundle) (*Phase29Result, error) {
	if err := validateBundle(bundle); err != nil {
		return nil, err
	}

	result := &Phase29Result{
		GeneratedAt: time.Now().UTC(),
	}

	// Section 1 — Strategy Championship
	result.Championship = extractStrategyChampionship(bundle.P24, bundle.P26)

	// Section 2 — Alpha Championship
	result.AlphaChamp = extractAlphaChampionship(bundle.P27)
	result.AlphaChamp.GeneratedAt = result.GeneratedAt

	// Section 3 — Retirement Report
	result.Retirement = extractRetirementReport(bundle.P24, bundle.P26)
	result.Retirement.GeneratedAt = result.GeneratedAt

	// Section 4 — Portfolio Championship
	result.Portfolios = extractPortfolioChampionship(bundle.P28)
	result.Portfolios.GeneratedAt = result.GeneratedAt

	// Section 5 — Capital Deployment Plan
	result.CapitalDeployment = extractCapitalDeployment(bundle.P28)
	result.CapitalDeployment.GeneratedAt = result.GeneratedAt

	// Section 6 — Edge Retention
	result.EdgeRetention = extractEdgeRetention(bundle.P25, bundle.P26)
	result.EdgeRetention.GeneratedAt = result.GeneratedAt

	// Section 7 — Execution Quality
	result.ExecutionQuality = extractExecutionQuality(bundle.P25)
	result.ExecutionQuality.GeneratedAt = result.GeneratedAt

	// Section 8 — Institutional Certification
	result.InstitutionalCert = extractInstitutionalCert(bundle.P26, bundle.P28)
	result.InstitutionalCert.GeneratedAt = result.GeneratedAt

	// Section 9 — Final Verdict (20 questions)
	result.FinalVerdict = extractFinalVerdict(bundle, result.InstitutionalCert, bundle.P27)
	result.FinalVerdict.GeneratedAt = result.GeneratedAt

	// Collect INSUFFICIENT_EVIDENCE sections
	result.InsufficientEvidence = collectInsufficientEvidence(result)

	return result, nil
}

// RunFromPhases is a convenience wrapper that runs all upstream phases
// (Phase 24 → 25 → 26 → 27 → 28) and then executes Phase 29.
func RunFromPhases(replayCfg phase23b.ReplayConfig) (*Phase29Result, error) {
	// Phase 24
	p24Pipeline := phase24.NewPipeline24(replayCfg)
	p24Result, err := p24Pipeline.Run()
	if err != nil {
		return nil, fmt.Errorf("phase24: %w", err)
	}

	// Phase 25
	p25Config := phase25.DefaultConfig()
	p25Pipeline := phase25.NewPipeline25(p25Config)
	p25Result, err := p25Pipeline.Run(p24Result)
	if err != nil {
		return nil, fmt.Errorf("phase25: %w", err)
	}

	// Phase 26
	p26Config := phase26.DefaultConfig()
	p26Result, err := phase26.Run(&p24Result, &p25Result, p26Config)
	if err != nil {
		return nil, fmt.Errorf("phase26: %w", err)
	}

	// Phase 27
	p27Result, err := phase27.Run(&p25Result, p26Result)
	if err != nil {
		return nil, fmt.Errorf("phase27: %w", err)
	}

	// Phase 28
	p28Result, err := phase28.Run(&p24Result, &p25Result, p26Result, p27Result)
	if err != nil {
		return nil, fmt.Errorf("phase28: %w", err)
	}

	return Run(EvidenceBundle{
		P24: &p24Result,
		P25: &p25Result,
		P26: p26Result,
		P27: p27Result,
		P28: p28Result,
	})
}

// ── validation ────────────────────────────────────────────────────────────────

func validateBundle(b EvidenceBundle) error {
	if b.P24 == nil {
		return EvidenceError{Field: "P24", Message: "Phase 24 result is nil — run Phase 24 first"}
	}
	if b.P25 == nil {
		return EvidenceError{Field: "P25", Message: "Phase 25 result is nil — run Phase 25 first"}
	}
	if b.P26 == nil {
		return EvidenceError{Field: "P26", Message: "Phase 26 result is nil — run Phase 26 first"}
	}
	if b.P27 == nil {
		return EvidenceError{Field: "P27", Message: "Phase 27 result is nil — run Phase 27 first"}
	}
	if b.P28 == nil {
		return EvidenceError{Field: "P28", Message: "Phase 28 result is nil — run Phase 28 first"}
	}
	if b.P24.TotalStrategiesEvaluated == 0 {
		return EvidenceError{Field: "P24.TotalStrategiesEvaluated", Message: "no strategies evaluated in Phase 24 — data load may have failed"}
	}
	return nil
}

func collectInsufficientEvidence(r *Phase29Result) []string {
	var issues []string
	if len(r.Championship.All) == 0 {
		issues = append(issues, "STRATEGY_CHAMPIONSHIP: no strategy metrics available from Phase 24")
	}
	if len(r.AlphaChamp.Rankings) == 0 {
		issues = append(issues, "ALPHA_CHAMPIONSHIP: no alpha attribution from Phase 27")
	}
	if len(r.InstitutionalCert.Institutional) == 0 && len(r.InstitutionalCert.FullCapital) == 0 {
		issues = append(issues, "INSTITUTIONAL_CERTIFICATION: no strategies cleared full capital or institutional gates")
	}
	if len(r.EdgeRetention.Entries) == 0 {
		issues = append(issues, "EDGE_RETENTION: no Phase 26 edge retention records available")
	}
	if r.ExecutionQuality.TotalCertifiedTrades == 0 {
		issues = append(issues, "EXECUTION_QUALITY: no live trade latency records from Phase 25")
	}
	return issues
}


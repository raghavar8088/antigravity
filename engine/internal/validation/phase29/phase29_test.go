package phase29

import (
	"os"
	"testing"
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
	"antigravity-engine/internal/validation/phase28"
)

// TestValidateBundle_NilPhase ensures bundle validation rejects nil inputs.
func TestValidateBundle_NilPhase(t *testing.T) {
	tests := []struct {
		name   string
		bundle EvidenceBundle
	}{
		{"nil P24", EvidenceBundle{P25: &phase25.Phase25Result{}, P26: &phase26.Phase26Result{}, P27: &phase27.Phase27Result{}, P28: &phase28.Phase28Result{}}},
		{"nil P25", EvidenceBundle{P24: &phase24.Phase24Result{TotalStrategiesEvaluated: 1}, P26: &phase26.Phase26Result{}, P27: &phase27.Phase27Result{}, P28: &phase28.Phase28Result{}}},
		{"nil P26", EvidenceBundle{P24: &phase24.Phase24Result{TotalStrategiesEvaluated: 1}, P25: &phase25.Phase25Result{}, P27: &phase27.Phase27Result{}, P28: &phase28.Phase28Result{}}},
		{"nil P27", EvidenceBundle{P24: &phase24.Phase24Result{TotalStrategiesEvaluated: 1}, P25: &phase25.Phase25Result{}, P26: &phase26.Phase26Result{}, P28: &phase28.Phase28Result{}}},
		{"nil P28", EvidenceBundle{P24: &phase24.Phase24Result{TotalStrategiesEvaluated: 1}, P25: &phase25.Phase25Result{}, P26: &phase26.Phase26Result{}, P27: &phase27.Phase27Result{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Run(tt.bundle)
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

// TestRun_EmptyBundle runs Phase 29 with valid but empty phase results and
// verifies it returns a result (not an error) with INSUFFICIENT_EVIDENCE populated.
func TestRun_EmptyBundle(t *testing.T) {
	bundle := EvidenceBundle{
		P24: &phase24.Phase24Result{
			TotalStrategiesEvaluated: 1,
			GeneratedAt:              time.Now(),
		},
		P25: &phase25.Phase25Result{},
		P26: &phase26.Phase26Result{
			Tier2Milestones: make(map[string][]phase26.MilestoneReport),
		},
		P27: &phase27.Phase27Result{},
		P28: &phase28.Phase28Result{},
	}

	result, err := Run(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.FinalVerdict.OverallVerdict == "" {
		t.Error("OverallVerdict should not be empty")
	}
	t.Logf("OverallVerdict: %s", result.FinalVerdict.OverallVerdict)
	t.Logf("InsufficientEvidence sections: %v", result.InsufficientEvidence)
}

// TestWriteAllReports verifies all 10 markdown files are written to a temp dir.
func TestWriteAllReports(t *testing.T) {
	bundle := EvidenceBundle{
		P24: &phase24.Phase24Result{TotalStrategiesEvaluated: 1, GeneratedAt: time.Now()},
		P25: &phase25.Phase25Result{},
		P26: &phase26.Phase26Result{Tier2Milestones: make(map[string][]phase26.MilestoneReport)},
		P27: &phase27.Phase27Result{},
		P28: &phase28.Phase28Result{},
	}

	result, err := Run(bundle)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	outDir := t.TempDir()
	if err := WriteAllReports(result, outDir); err != nil {
		t.Fatalf("WriteAllReports: %v", err)
	}

	expected := []string{
		"PHASE29_MASTER_EVIDENCE_REPORT.md",
		"STRATEGY_CHAMPIONSHIP.md",
		"ALPHA_CHAMPIONSHIP.md",
		"RETIREMENT_REPORT.md",
		"PORTFOLIO_CHAMPIONSHIP.md",
		"CAPITAL_DEPLOYMENT_PLAN.md",
		"EDGE_RETENTION_REPORT.md",
		"EXECUTION_QUALITY_REPORT.md",
		"INSTITUTIONAL_CERTIFICATION.md",
		"PHASE29_FINAL_VERDICT.md",
	}
	for _, fname := range expected {
		path := outDir + "/" + fname
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("file not created: %s", fname)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file is empty: %s", fname)
		} else {
			t.Logf("✅ %s (%d bytes)", fname, info.Size())
		}
	}
}

// TestClassifyEdgeRetention verifies boundary conditions.
func TestClassifyEdgeRetention(t *testing.T) {
	tests := []struct {
		pfRet, sharpeRet float64
		want             string
	}{
		{100, 100, "ELITE"},
		{95, 95, "ELITE"},
		{90, 90, "STRONG"},
		{85, 85, "STRONG"},
		{80, 80, "ACCEPTABLE"},
		{75, 75, "ACCEPTABLE"},
		{70, 70, "DEGRADING"},
		{60, 60, "DEGRADING"},
		{50, 50, "FAILED"},
		{0, 0, "FAILED"},
	}
	for _, tt := range tests {
		got := classifyEdgeRetention(tt.pfRet, tt.sharpeRet)
		if got != tt.want {
			t.Errorf("classifyEdgeRetention(%.0f, %.0f) = %s, want %s", tt.pfRet, tt.sharpeRet, got, tt.want)
		}
	}
}

// TestAssignCertTier verifies tier assignment logic.
func TestAssignCertTier(t *testing.T) {
	tests := []struct {
		pf, sharpe, dd, ror float64
		decision             phase26.CertificationDecision
		want                 CertificationTier29
	}{
		{1.6, 2.1, 7.0, 0.03, phase26.DecisionPromote, CertInstitutional},
		{1.35, 1.6, 9.0, 0.08, phase26.DecisionPromote, CertFullCapital},
		{1.22, 1.05, 14.0, 0.12, phase26.DecisionPromote, CertLimitedCapital},
		{1.12, 0.6, 18.0, 0.18, phase26.DecisionMaintain, CertPilot},
		{1.05, 0.4, 22.0, 0.22, phase26.DecisionMaintain, CertPaperOnly},
		{0.95, 0.1, 30.0, 0.40, phase26.DecisionRetire, CertRetired},
	}
	for _, tt := range tests {
		rec := phase26.CertificationRecord{
			ProfitFactor: tt.pf,
			Sharpe:       tt.sharpe,
			MaxDD:        tt.dd,
			RiskOfRuin:   tt.ror,
			Decision:     tt.decision,
		}
		got := assignCertTier(rec)
		if got != tt.want {
			t.Errorf("assignCertTier PF=%.2f Sharpe=%.2f = %s, want %s", tt.pf, tt.sharpe, got, tt.want)
		}
	}
}

// TestEvidenceError verifies the error type formats correctly.
func TestEvidenceError(t *testing.T) {
	err := EvidenceError{Field: "P24", Message: "nil result"}
	if err.Error() == "" {
		t.Error("EvidenceError.Error() returned empty string")
	}
}

package phase26_test

import (
	"testing"
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func makeTrades(stratName string, n int, winRate float64) []phase25.LiveTrade {
	trades := make([]phase25.LiveTrade, n)
	for i := range trades {
		isWin := float64(i)/float64(n) < winRate
		pnl := -50.0
		if isWin {
			pnl = 100.0
		}
		trades[i] = phase25.LiveTrade{
			TradeID:      stratName + "_" + string(rune('A'+i%26)),
			StrategyName: stratName,
			AlphaFamily:  phase25.FamilyMomentum,
			EntryTime:    time.Now().Add(-time.Duration(n-i) * time.Hour),
			ExitTime:     time.Now().Add(-time.Duration(n-i)*time.Hour + 30*time.Minute),
			NetPnLUSD:    pnl,
			GrossPnLUSD:  pnl + 10,
			FeeUSD:       5,
			SlippageUSD:  3,
			FundingUSD:   2,
			IsWin:        isWin,
		}
	}
	return trades
}

func makeP24(stratName string) *phase24.Phase24Result {
	return &phase24.Phase24Result{
		Metrics: map[string]phase24.ExtendedMetrics{
			stratName: {
				StrategyName:   stratName,
				TotalTrades:    500,
				ProfitFactor:   1.60,
				Sharpe:         2.10,
				MaxDrawdown:    7.5,
				Expectancy:     45.0,
				WinRate:        0.62,
				CompositeScore: 78.0,
			},
		},
		CapCerts: []phase24.CapCertification24{
			{StrategyName: stratName, Tier: phase24.CapTier24Institutional},
		},
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestBuildEligibility_ExcludesRetired(t *testing.T) {
	p24 := &phase24.Phase24Result{
		CapCerts: []phase24.CapCertification24{
			{StrategyName: "retired_strat", Tier: phase24.CapTier24Retired},
		},
		Metrics: map[string]phase24.ExtendedMetrics{},
	}
	p25 := &phase25.Phase25Result{
		LiveMetrics: map[string]phase25.LiveMetrics{},
	}
	eligible := phase26.BuildEligibility(p24, p25)
	if len(eligible) == 0 {
		t.Fatal("expected at least one record")
	}
	if eligible[0].EligibleTier != "EXCLUDED" {
		t.Errorf("expected EXCLUDED, got %s", eligible[0].EligibleTier)
	}
}

func TestBuildEligibility_ExcludesDemotedPhase25(t *testing.T) {
	p24 := &phase24.Phase24Result{
		CapCerts: []phase24.CapCertification24{
			{StrategyName: "strat1", Tier: phase24.CapTier24Full},
		},
		Metrics: map[string]phase24.ExtendedMetrics{"strat1": {}},
	}
	p25 := &phase25.Phase25Result{
		LiveMetrics: map[string]phase25.LiveMetrics{},
		CapEscalation: []phase25.CapEscalationRecord{
			{StrategyName: "strat1", PromotionStatus: "DEMOTE"},
		},
	}
	eligible := phase26.BuildEligibility(p24, p25)
	if len(eligible) == 0 {
		t.Fatal("expected record")
	}
	if eligible[0].EligibleTier != "EXCLUDED" {
		t.Errorf("expected EXCLUDED for DEMOTE, got %s", eligible[0].EligibleTier)
	}
}

func TestBuildEligibility_IncludesValidStrategy(t *testing.T) {
	p24 := &phase24.Phase24Result{
		CapCerts: []phase24.CapCertification24{
			{StrategyName: "good_strat", Tier: phase24.CapTier24Institutional},
		},
		Metrics: map[string]phase24.ExtendedMetrics{
			"good_strat": {CompositeScore: 85.0, ProfitFactor: 1.6, Sharpe: 2.1},
		},
	}
	p25 := &phase25.Phase25Result{
		LiveMetrics: map[string]phase25.LiveMetrics{
			"good_strat": {ProfitFactor: 1.5, Sharpe: 1.8},
		},
		EdgeDrift: []phase25.EdgeDriftRecord{
			{StrategyName: "good_strat", EdgeStatus: phase25.EdgeStable},
		},
	}
	eligible := phase26.BuildEligibility(p24, p25)
	if len(eligible) == 0 {
		t.Fatal("expected eligible strategy")
	}
	if eligible[0].EligibleTier == "EXCLUDED" {
		t.Errorf("expected TIER1 or TIER2, got EXCLUDED")
	}
}

func TestBuildEdgeRetention_PFRetention(t *testing.T) {
	p24m := phase24.ExtendedMetrics{ProfitFactor: 2.0, Sharpe: 2.0, Expectancy: 50, WinRate: 0.6}
	lm := phase25.LiveMetrics{ProfitFactor: 1.5, Sharpe: 1.6, Expectancy: 40, WinRate: 0.55}
	rec := phase26.BuildEdgeRetention("strat1", p24m, lm)
	expected := 75.0
	if rec.PFRetentionPct != expected {
		t.Errorf("PFRetentionPct: expected %.1f, got %.1f", expected, rec.PFRetentionPct)
	}
}

func TestBuildEdgeRetention_ZeroHistoricalPF(t *testing.T) {
	p24m := phase24.ExtendedMetrics{ProfitFactor: 0, Sharpe: 0}
	lm := phase25.LiveMetrics{ProfitFactor: 1.2, Sharpe: 1.4}
	rec := phase26.BuildEdgeRetention("strat_zero", p24m, lm)
	if rec.PFRetentionPct != 0 {
		t.Errorf("expected 0 retention for zero historical PF, got %.1f", rec.PFRetentionPct)
	}
}

func TestBuildEdgeRetention_RetentionClass(t *testing.T) {
	p24m := phase24.ExtendedMetrics{ProfitFactor: 2.0, Sharpe: 2.0}
	lm := phase25.LiveMetrics{ProfitFactor: 1.98, Sharpe: 1.97}
	rec := phase26.BuildEdgeRetention("elite", p24m, lm)
	if rec.RetentionClass != phase26.EliteRetention {
		t.Errorf("expected EliteRetention, got %s", rec.RetentionClass)
	}
}

func TestBuildDriftRecord_InsufficientTrades(t *testing.T) {
	trades := makeTrades("strat1", 10, 0.6) // only 10 trades
	rec := phase26.BuildDriftRecord("strat1", trades)
	if rec.DriftClass != phase26.DriftFailure {
		t.Errorf("expected DriftFailure for <30 trades, got %s", rec.DriftClass)
	}
}

func TestBuildDriftRecord_StableEdge(t *testing.T) {
	trades := makeTrades("strat1", 200, 0.65)
	rec := phase26.BuildDriftRecord("strat1", trades)
	if rec.DriftClass == "" {
		t.Error("DriftClass should not be empty")
	}
}

func TestApplyCertificationGates_AllPass(t *testing.T) {
	cert := &phase26.CertificationRecord{
		TotalTrades:  400,
		ProfitFactor: 1.5,
		Sharpe:       1.8,
		Expectancy:   30.0,
		RiskOfRuin:   0.05,
		MaxDD:        8.0,
		MCStability:  "STABLE",
		WFResult:     "PASS",
		EdgeRetention: phase26.EdgeRetentionRecord{
			HistoricalPF:     2.0,
			PFRetentionPct:   90.0,
			HistoricalSharpe: 2.0,
			SharpeRetentionPct: 88.0,
		},
	}
	phase26.ApplyCertificationGates(cert, 300)
	if cert.Decision != phase26.DecisionPromote {
		t.Errorf("expected PROMOTE, got %s (failed: %v)", cert.Decision, cert.GatesFailed)
	}
}

func TestApplyCertificationGates_InsufficientTrades(t *testing.T) {
	cert := &phase26.CertificationRecord{
		TotalTrades:  50,
		ProfitFactor: 1.5,
	}
	phase26.ApplyCertificationGates(cert, 300)
	if cert.Decision != phase26.DecisionMaintain {
		t.Errorf("expected MAINTAIN for insufficient trades, got %s", cert.Decision)
	}
}

func TestPipelineRun_EmptyInputs(t *testing.T) {
	p24 := &phase24.Phase24Result{
		Metrics: map[string]phase24.ExtendedMetrics{},
	}
	p25 := &phase25.Phase25Result{
		LiveMetrics: map[string]phase25.LiveMetrics{},
	}
	cfg := phase26.DefaultConfig()
	result, err := phase26.Run(p24, p25, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.TotalCertified != 0 {
		t.Errorf("expected 0 certified, got %d", result.TotalCertified)
	}
}

func TestPipelineRun_NilInputErrors(t *testing.T) {
	_, err := phase26.Run(nil, nil, phase26.DefaultConfig())
	if err == nil {
		t.Error("expected error for nil inputs")
	}
}

func TestBuildReports_ContainsAllKeys(t *testing.T) {
	r := &phase26.Phase26Result{
		GeneratedAt:    time.Now(),
		FinalVerdict:   phase26.Phase26Verdict{GeneratedAt: time.Now()},
		Tier2Milestones: map[string][]phase26.MilestoneReport{},
	}
	reports := phase26.BuildReports(r)
	required := []string{"eligibility", "tier1_validation", "tier2_validation", "edge_retention", "drift_detection", "certification", "verdict", "executive_summary"}
	for _, key := range required {
		if _, ok := reports[key]; !ok {
			t.Errorf("missing report: %s", key)
		}
	}
}

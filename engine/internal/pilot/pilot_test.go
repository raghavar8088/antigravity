package pilot_test

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/pilot"
)

// ── CertificationEngine ──────────────────────────────────────────────────────

func TestCertify_NotReady_InsufficientTrades(t *testing.T) {
	ce := pilot.NewCertificationEngine()
	status, reasons := ce.Certify(pilot.LiveMetricsSummary{
		TotalTrades:    100,
		ProfitFactor:   1.50,
		WinRate:        0.55,
		SharpeRatio:    2.0,
		MaxDrawdown:    0.05,
		PositiveMonths: 4,
	})
	if status != pilot.CertNotReady {
		t.Errorf("expected NOT_READY, got %s", status)
	}
	if len(reasons) == 0 {
		t.Error("expected failing reasons")
	}
}

func TestCertify_ReadyForPilot(t *testing.T) {
	ce := pilot.NewCertificationEngine()
	status, reasons := ce.Certify(pilot.LiveMetricsSummary{
		TotalTrades:    1100,
		ProfitFactor:   1.35,
		WinRate:        0.48,
		SharpeRatio:    1.60,
		MaxDrawdown:    0.08,
		PositiveMonths: 3,
		RiskViolations: 0,
	})
	if status != pilot.CertReadyForPilot {
		t.Errorf("expected READY_FOR_PILOT, got %s (reasons: %v)", status, reasons)
	}
}

func TestCertify_RiskViolationBlocksCert(t *testing.T) {
	ce := pilot.NewCertificationEngine()
	status, _ := ce.Certify(pilot.LiveMetricsSummary{
		TotalTrades:    1200,
		ProfitFactor:   1.40,
		WinRate:        0.52,
		SharpeRatio:    1.80,
		MaxDrawdown:    0.06,
		PositiveMonths: 4,
		RiskViolations: 1,
	})
	if status != pilot.CertNotReady {
		t.Errorf("risk violation must block certification; got %s", status)
	}
}

func TestCertify_ReadyForInstitutionalScaling(t *testing.T) {
	ce := pilot.NewCertificationEngine()
	status, _ := ce.Certify(pilot.LiveMetricsSummary{
		TotalTrades:    2100,
		ProfitFactor:   1.60,
		WinRate:        0.55,
		SharpeRatio:    2.10,
		MaxDrawdown:    0.05,
		PositiveMonths: 7,
		RiskViolations: 0,
	})
	if status != pilot.CertReadyForInstitutionalScaling {
		t.Errorf("expected INSTITUTIONAL_SCALING, got %s", status)
	}
}

// ── CapitalScaler ─────────────────────────────────────────────────────────────

func TestCapitalScaler_DownOnPFBreach(t *testing.T) {
	cs := pilot.NewCapitalScaler()
	dec := cs.Evaluate(pilot.LiveMetricsSummary{
		TotalTrades:  600,
		ProfitFactor: 1.05, // below 1.10 → downscale
		WinRate:      0.50,
		SharpeRatio:  1.50,
		MaxDrawdown:  0.05,
	})
	if dec.Direction != "DOWN" {
		t.Errorf("expected DOWN, got %s", dec.Direction)
	}
	if dec.Trigger != "PF_BREACH" {
		t.Errorf("expected PF_BREACH, got %s", dec.Trigger)
	}
}

func TestCapitalScaler_DownOnConsecutiveLosses(t *testing.T) {
	cs := pilot.NewCapitalScaler()
	dec := cs.Evaluate(pilot.LiveMetricsSummary{
		TotalTrades:       600,
		ProfitFactor:      1.20,
		WinRate:           0.48,
		SharpeRatio:       1.20,
		MaxDrawdown:       0.07,
		ConsecutiveLosses: 3,
	})
	if dec.Direction != "DOWN" {
		t.Errorf("expected DOWN, got %s", dec.Direction)
	}
}

func TestCapitalScaler_DownOnRiskViolation(t *testing.T) {
	cs := pilot.NewCapitalScaler()
	dec := cs.Evaluate(pilot.LiveMetricsSummary{
		TotalTrades:    600,
		ProfitFactor:   1.40,
		WinRate:        0.55,
		SharpeRatio:    2.0,
		MaxDrawdown:    0.04,
		RiskViolations: 1,
	})
	if dec.Direction != "DOWN" || dec.Trigger != "RISK_VIOLATION" {
		t.Errorf("expected DOWN/RISK_VIOLATION, got %s/%s", dec.Direction, dec.Trigger)
	}
}

func TestCapitalScaler_ScaleUp(t *testing.T) {
	cs := pilot.NewCapitalScaler()
	dec := cs.Evaluate(pilot.LiveMetricsSummary{
		TotalTrades:    600,
		ProfitFactor:   1.40,
		WinRate:        0.52,
		SharpeRatio:    1.80,
		MaxDrawdown:    0.06,
		RiskViolations: 0,
	})
	if dec.Direction != "UP" {
		t.Errorf("expected UP, got %s (reason: %s)", dec.Direction, dec.Reason)
	}
}

func TestCapitalScaler_HoldInsufficientTrades(t *testing.T) {
	cs := pilot.NewCapitalScaler()
	dec := cs.Evaluate(pilot.LiveMetricsSummary{
		TotalTrades:  300, // below 500 minimum
		ProfitFactor: 1.40,
		WinRate:      0.52,
		SharpeRatio:  1.80,
		MaxDrawdown:  0.06,
	})
	if dec.Direction != "HOLD" {
		t.Errorf("expected HOLD, got %s", dec.Direction)
	}
}

// ── StrategyRanker ────────────────────────────────────────────────────────────

func TestStrategyRanker_OrdersCorrectly(t *testing.T) {
	r := pilot.NewStrategyRanker()
	strategies := []pilot.StrategyLiveMetrics{
		{StrategyID: "s1", Trades: 100, Sharpe: 1.5, ProfitFactor: 1.3, WinRate: 0.50, MaxDrawdown: 0.08},
		{StrategyID: "s2", Trades: 150, Sharpe: 2.5, ProfitFactor: 1.7, WinRate: 0.60, MaxDrawdown: 0.04},
		{StrategyID: "s3", Trades: 80, Sharpe: 1.2, ProfitFactor: 1.2, WinRate: 0.45, MaxDrawdown: 0.12},
	}
	ranked := r.Rank(strategies)
	if ranked[0].StrategyID != "s2" {
		t.Errorf("expected s2 ranked first, got %s", ranked[0].StrategyID)
	}
	if ranked[len(ranked)-1].StrategyID != "s3" {
		t.Errorf("expected s3 ranked last, got %s", ranked[len(ranked)-1].StrategyID)
	}
}

func TestStrategyRanker_BelowMinTradesScoresZero(t *testing.T) {
	r := pilot.NewStrategyRanker()
	strategies := []pilot.StrategyLiveMetrics{
		{StrategyID: "s1", Trades: 10, Sharpe: 5.0, ProfitFactor: 3.0, WinRate: 0.70, MaxDrawdown: 0.01},
		{StrategyID: "s2", Trades: 100, Sharpe: 1.5, ProfitFactor: 1.3, WinRate: 0.50, MaxDrawdown: 0.08},
	}
	ranked := r.Rank(strategies)
	// s1 has > MinTrades requirement so should be ranked last despite great paper metrics
	if ranked[0].StrategyID != "s2" {
		t.Errorf("expected s2 (sufficient trades) ranked first, got %s", ranked[0].StrategyID)
	}
}

func TestStrategyRanker_TopN(t *testing.T) {
	r := pilot.NewStrategyRanker()
	strategies := make([]pilot.StrategyLiveMetrics, 10)
	for i := range strategies {
		strategies[i] = pilot.StrategyLiveMetrics{
			StrategyID: "s" + string(rune('0'+i)),
			Trades:     100 + i*10,
			Sharpe:     float64(i) * 0.3,
			ProfitFactor: 1.1 + float64(i)*0.1,
			WinRate:    0.40 + float64(i)*0.02,
			MaxDrawdown: 0.15 - float64(i)*0.01,
		}
	}
	top3 := r.TopN(strategies, 3)
	if len(top3) != 3 {
		t.Errorf("expected 3 strategies, got %d", len(top3))
	}
}

// ── PilotAggregate ────────────────────────────────────────────────────────────

func TestPilotAggregate_DeployStrategy_ValidatesStageRange(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("pilot-1", "acc-1", 1_000_000, store)
	// Stage1 max alloc = 0.50%; 1.00% should fail
	err := agg.AdvanceStage(context.Background(), pilot.LiveMetricsSummary{
		TotalTrades: 600, ProfitFactor: 1.40, WinRate: 0.52, SharpeRatio: 1.80, MaxDrawdown: 0.06,
	})
	if err != nil {
		t.Fatalf("AdvanceStage: %v", err)
	}
	err = agg.DeployStrategy(context.Background(), "s1", "ema_fast", 1, 0.01) // 1% > Stage1 max 0.50%
	if err == nil {
		t.Error("expected error for alloc above Stage1 ceiling")
	}
}

func TestPilotAggregate_DeployStrategy_ValidatesNotStarted(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("pilot-2", "acc-2", 1_000_000, store)
	// Stage is NOT_STARTED; deploy should fail
	err := agg.DeployStrategy(context.Background(), "s1", "ema_fast", 1, 0.0025)
	if err == nil {
		t.Error("expected error when pilot not started")
	}
}

func TestPilotAggregate_AdvanceStage(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("pilot-3", "acc-3", 1_000_000, store)
	metrics := pilot.LiveMetricsSummary{
		TotalTrades: 600, ProfitFactor: 1.40, WinRate: 0.52, SharpeRatio: 1.80, MaxDrawdown: 0.06,
	}
	if err := agg.AdvanceStage(context.Background(), metrics); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agg.Stage != pilot.Stage1 {
		t.Errorf("expected Stage1, got %s", agg.Stage)
	}
	if err := agg.AdvanceStage(context.Background(), metrics); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agg.Stage != pilot.Stage2 {
		t.Errorf("expected Stage2, got %s", agg.Stage)
	}
}

func TestPilotAggregate_HaltPreventsDeployment(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("pilot-4", "acc-4", 1_000_000, store)
	_ = agg.AdvanceStage(context.Background(), pilot.LiveMetricsSummary{
		TotalTrades: 600, ProfitFactor: 1.40, WinRate: 0.52, SharpeRatio: 1.80, MaxDrawdown: 0.06,
	})
	agg.Halt(context.Background(), "circuit breaker triggered", "risk_engine")
	err := agg.DeployStrategy(context.Background(), "s1", "ema_fast", 1, 0.0025)
	if err == nil {
		t.Error("expected error on deploy when halted")
	}
}

func TestPilotAggregate_ScaleCapital(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("pilot-5", "acc-5", 1_000_000, store)
	_ = agg.AdvanceStage(context.Background(), pilot.LiveMetricsSummary{
		TotalTrades: 600, ProfitFactor: 1.40, WinRate: 0.52, SharpeRatio: 1.80, MaxDrawdown: 0.06,
	})
	_ = agg.DeployStrategy(context.Background(), "s1", "ema_fast", 1, 0.0025)

	err := agg.ScaleCapital(context.Background(), "s1", "UP", "ALL_THRESHOLDS_MET", 0.0040, 1.40)
	if err != nil {
		t.Fatalf("ScaleCapital: %v", err)
	}
	if agg.DeployedStrategies["s1"].AllocPct != 0.0040 {
		t.Errorf("expected alloc 0.0040, got %f", agg.DeployedStrategies["s1"].AllocPct)
	}
}

func TestPilotAggregate_RetractStrategy(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("pilot-6", "acc-6", 1_000_000, store)
	_ = agg.AdvanceStage(context.Background(), pilot.LiveMetricsSummary{
		TotalTrades: 600, ProfitFactor: 1.40, WinRate: 0.52, SharpeRatio: 1.80, MaxDrawdown: 0.06,
	})
	_ = agg.DeployStrategy(context.Background(), "s1", "ema_fast", 1, 0.0025)
	agg.RetractStrategy(context.Background(), "s1", "underperformance")
	if _, ok := agg.DeployedStrategies["s1"]; ok {
		t.Error("strategy should have been removed")
	}
}

// ── LiveMetrics ───────────────────────────────────────────────────────────────

func TestLiveMetrics_RecordFill_UpdatesDerivedMetrics(t *testing.T) {
	m := &pilot.LiveMetrics{
		StrategyMetrics: make(map[string]*pilot.StrategyLiveMetrics),
		PeakNAV:         1_000_000,
		CurrentNAV:      1_000_000,
	}
	m.RecordFill(pilot.FillRecord{PnL: 200, FeesUSD: 5, SlippageBps: 2})
	m.RecordFill(pilot.FillRecord{PnL: 100, FeesUSD: 5, SlippageBps: 1})
	m.RecordFill(pilot.FillRecord{PnL: -100, FeesUSD: 5, SlippageBps: 3})

	if m.TotalTrades != 3 {
		t.Errorf("expected 3 trades, got %d", m.TotalTrades)
	}
	if m.WinRate < 0.66 || m.WinRate > 0.68 {
		t.Errorf("expected ~0.667 win rate, got %.3f", m.WinRate)
	}
	if m.ProfitFactor < 2.9 {
		t.Errorf("expected PF ≥ 3.0, got %.2f", m.ProfitFactor)
	}
}

func TestLiveMetrics_ConsecutiveLosses(t *testing.T) {
	m := &pilot.LiveMetrics{}
	m.AddWeeklyReturn(-0.02)
	m.AddWeeklyReturn(-0.01)
	m.AddWeeklyReturn(-0.03)
	if m.ConsecutiveLosses != 3 {
		t.Errorf("expected 3 consecutive losses, got %d", m.ConsecutiveLosses)
	}
	m.AddWeeklyReturn(0.05)
	if m.ConsecutiveLosses != 0 {
		t.Error("consecutive losses should reset on positive week")
	}
}

func TestLiveMetrics_PositiveMonths(t *testing.T) {
	m := &pilot.LiveMetrics{}
	m.AddMonthlyReturn(0.05)
	m.AddMonthlyReturn(-0.02)
	m.AddMonthlyReturn(0.03)
	if m.PositiveMonths != 2 {
		t.Errorf("expected 2 positive months, got %d", m.PositiveMonths)
	}
}

// ── Stage ─────────────────────────────────────────────────────────────────────

func TestStage_CapitalRange(t *testing.T) {
	tests := []struct {
		stage    pilot.Stage
		wantMin  float64
		wantMax  float64
	}{
		{pilot.Stage1, 0.0025, 0.0050},
		{pilot.Stage2, 0.0050, 0.0100},
		{pilot.Stage3, 0.0100, 0.0200},
		{pilot.Stage4, 0.0200, 0.0500},
	}
	for _, tt := range tests {
		got_min, got_max := tt.stage.CapitalRangePct()
		if got_min != tt.wantMin || got_max != tt.wantMax {
			t.Errorf("stage %s: got [%.4f, %.4f], want [%.4f, %.4f]",
				tt.stage, got_min, got_max, tt.wantMin, tt.wantMax)
		}
	}
}

func TestStage_MaxStrategies(t *testing.T) {
	if pilot.Stage1.MaxStrategies() != 3 {
		t.Error("Stage1 should allow max 3 strategies")
	}
	if pilot.Stage4.MaxStrategies() != 0 {
		t.Error("Stage4 should have no limit (0)")
	}
}

// ── Projections ───────────────────────────────────────────────────────────────

func TestBuildPilotProjection(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("p1", "a1", 500_000, store)
	_ = agg.AdvanceStage(context.Background(), pilot.LiveMetricsSummary{
		TotalTrades: 600, ProfitFactor: 1.40, WinRate: 0.52, SharpeRatio: 1.80, MaxDrawdown: 0.06,
	})
	_ = agg.DeployStrategy(context.Background(), "s1", "ema_fast", 1, 0.0025)
	_ = agg.DeployStrategy(context.Background(), "s2", "rsi_slope", 2, 0.0025)

	proj := pilot.BuildPilotProjection(agg)
	if proj.StrategyCount != 2 {
		t.Errorf("expected 2 strategies, got %d", proj.StrategyCount)
	}
	if proj.TotalCapital != 500_000 {
		t.Errorf("expected 500000 capital, got %f", proj.TotalCapital)
	}
	if proj.DeployedCapPct < 0.0049 || proj.DeployedCapPct > 0.0051 {
		t.Errorf("expected 0.005 deployed cap pct, got %f", proj.DeployedCapPct)
	}
}

// ── CertificationReport generation ───────────────────────────────────────────

func TestGenerateCertificationReport_ContainsVerdict(t *testing.T) {
	store := ledger.NewMemoryStore()
	agg := pilot.NewPilotAggregate("p-rpt", "a-rpt", 1_000_000, store)
	m := &pilot.LiveMetrics{
		StrategyMetrics: make(map[string]*pilot.StrategyLiveMetrics),
		PeakNAV:         1_000_000,
		CurrentNAV:      1_000_000,
		TotalTrades:     50,
	}
	ce := pilot.NewCertificationEngine()
	report := ce.GenerateCertificationReport(agg, m, nil)
	if len(report) == 0 {
		t.Error("report must not be empty")
	}
	if !contains(report, "Certification Verdict") {
		t.Error("report must contain 'Certification Verdict'")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

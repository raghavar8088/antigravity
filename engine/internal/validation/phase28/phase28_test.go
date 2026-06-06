package phase28_test

import (
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
	"antigravity-engine/internal/validation/phase28"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func makeCert(name string, pf, sharpe, maxDD, ror float64, trades int) phase26.CertificationRecord {
	return phase26.CertificationRecord{
		StrategyName: name,
		TotalTrades:  trades,
		ProfitFactor: pf,
		Sharpe:       sharpe,
		MaxDD:        maxDD,
		RiskOfRuin:   ror,
		NetPnLUSD:    1000 * pf,
		Expectancy:   20.0,
		MCStability:  "STABLE",
		WFResult:     "PASS",
		Decision:     phase26.DecisionPromote,
		EdgeRetention: phase26.EdgeRetentionRecord{
			HistoricalPF:       2.0,
			PFRetentionPct:     92.0,
			HistoricalSharpe:   2.0,
			SharpeRetentionPct: 88.0,
		},
	}
}

func makeLiveTrades28(stratName string, n int, winRate float64) []phase25.LiveTrade {
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
			EntryTime:    time.Now().Add(-time.Duration(n-i) * 24 * time.Hour),
			NetPnLUSD:    pnl,
			IsWin:        isWin,
		}
	}
	return trades
}

// ── Correlation matrix tests ──────────────────────────────────────────────────

func TestBuildCorrelationMatrix_SelfCorrelation(t *testing.T) {
	trades := makeLiveTrades28("strat1", 30, 0.6)
	cm := phase28.BuildCorrelationMatrix([]string{"strat1"}, trades)
	if cm.Matrix["strat1"]["strat1"] != 1.0 {
		t.Errorf("self-correlation should be 1.0, got %.4f", cm.Matrix["strat1"]["strat1"])
	}
}

func TestBuildCorrelationMatrix_PairBounds(t *testing.T) {
	tradesA := makeLiveTrades28("stratA", 30, 0.6)
	tradesB := makeLiveTrades28("stratB", 30, 0.4)
	allTrades := append(tradesA, tradesB...)
	cm := phase28.BuildCorrelationMatrix([]string{"stratA", "stratB"}, allTrades)
	r := cm.Matrix["stratA"]["stratB"]
	if !math.IsNaN(r) && (r < -1.01 || r > 1.01) {
		t.Errorf("Pearson r must be in [-1,1], got %.4f", r)
	}
}

// ── Portfolio optimizer tests ─────────────────────────────────────────────────

func TestBuildPortfolioMaxSharpe_Weights(t *testing.T) {
	certs := []phase26.CertificationRecord{
		makeCert("stratA", 1.8, 2.2, 6.0, 0.02, 500),
		makeCert("stratB", 1.6, 1.9, 8.0, 0.03, 400),
	}
	cm := phase28.CorrelationMatrix{Matrix: make(map[string]map[string]float64)}
	p := phase28.BuildPortfolioMaxSharpe(certs, cm)
	totalWeight := 0.0
	for _, pw := range p.Weights {
		totalWeight += pw.Weight
	}
	if math.Abs(totalWeight-1.0) > 0.001 {
		t.Errorf("weights should sum to 1.0, got %.4f", totalWeight)
	}
}

func TestBuildPortfolioMinDD_PrefersLowerDD(t *testing.T) {
	certs := []phase26.CertificationRecord{
		makeCert("lowDD", 1.5, 1.8, 3.0, 0.03, 400), // lower DD
		makeCert("highDD", 1.5, 1.8, 9.0, 0.03, 400), // higher DD
	}
	cm := phase28.CorrelationMatrix{Matrix: make(map[string]map[string]float64)}
	p := phase28.BuildPortfolioMinDD(certs, cm)
	lowDDWeight := 0.0
	highDDWeight := 0.0
	for _, pw := range p.Weights {
		if pw.StrategyName == "lowDD" {
			lowDDWeight = pw.Weight
		} else {
			highDDWeight = pw.Weight
		}
	}
	if lowDDWeight <= highDDWeight {
		t.Errorf("low-DD strategy should have higher weight: lowDD=%.4f highDD=%.4f", lowDDWeight, highDDWeight)
	}
}

func TestBuildPortfolioEmptyInput(t *testing.T) {
	cm := phase28.CorrelationMatrix{Matrix: make(map[string]map[string]float64)}
	p := phase28.BuildPortfolioMaxSharpe(nil, cm)
	if p.Evidence == "" {
		t.Error("expected evidence string for empty input")
	}
}

// ── Allocation engine tests ───────────────────────────────────────────────────

func TestScoreStrategies_Sorted(t *testing.T) {
	certs := []phase26.CertificationRecord{
		makeCert("low", 1.3, 1.5, 8.0, 0.05, 300),
		makeCert("high", 2.0, 2.5, 5.0, 0.02, 600),
	}
	p25 := &phase25.Phase25Result{LiveMetrics: map[string]phase25.LiveMetrics{}}
	allocs := phase28.ScoreStrategies(certs, p25)
	if len(allocs) < 2 {
		t.Fatal("expected 2 allocations")
	}
	if allocs[0].CompositeScore < allocs[1].CompositeScore {
		t.Error("allocations should be sorted by CompositeScore descending")
	}
}

func TestScoreStrategies_TotalPct100(t *testing.T) {
	certs := []phase26.CertificationRecord{
		makeCert("A", 1.5, 2.0, 6.0, 0.03, 400),
		makeCert("B", 1.7, 2.1, 7.0, 0.04, 350),
		makeCert("C", 1.4, 1.6, 9.0, 0.06, 300),
	}
	p25 := &phase25.Phase25Result{LiveMetrics: map[string]phase25.LiveMetrics{}}
	allocs := phase28.ScoreStrategies(certs, p25)
	total := 0.0
	for _, a := range allocs {
		total += a.AllocationPct
	}
	if math.Abs(total-100.0) > 0.01 {
		t.Errorf("total allocation should be 100%%, got %.2f", total)
	}
}

// ── Capital deployment tests ──────────────────────────────────────────────────

func TestBuildCapitalPlans_MinSlot(t *testing.T) {
	allocs := []phase28.StrategyAllocation{
		{StrategyName: "A", AllocationPct: 60},
		{StrategyName: "B", AllocationPct: 40},
	}
	plans := phase28.BuildCapitalPlans(allocs, []float64{1000})
	if len(plans) == 0 {
		t.Fatal("expected at least one plan")
	}
	for _, slot := range plans[0].Allocations {
		if slot.CapitalUSD < 100 {
			t.Errorf("slot below min $100 for $1k plan: %s = $%.2f", slot.StrategyName, slot.CapitalUSD)
		}
	}
}

// ── Failure analysis tests ────────────────────────────────────────────────────

func TestAnalyzeFailures_ExcludesPortfolioStrategies(t *testing.T) {
	certInPortfolio := makeCert("in_port", 1.8, 2.2, 5.0, 0.02, 500)
	certOutside := makeCert("outside", 1.2, 1.2, 12.0, 0.08, 200)
	portfolio := phase28.Portfolio28{
		Weights:        []phase28.PortfolioWeight{{StrategyName: "in_port"}},
		ExpectedSharpe: 2.0,
		ExpectedMaxDD:  5.0,
	}
	failures := phase28.AnalyzeFailures([]phase26.CertificationRecord{certInPortfolio, certOutside}, portfolio)
	for _, f := range failures {
		if f.StrategyName == "in_port" {
			t.Error("portfolio strategy should not appear in failure analysis")
		}
	}
}

// ── Verdict tests ─────────────────────────────────────────────────────────────

func TestBuildFinalVerdict_NoEdge(t *testing.T) {
	p26r := &phase26.Phase26Result{
		TotalCertified:         0,
		InstitutionalCertified: nil,
		Retired:                []string{"strat1"},
		FinalVerdict:           phase26.Phase26Verdict{GeneratedAt: time.Now()},
		Tier2Milestones:        map[string][]phase26.MilestoneReport{},
	}
	p27r := &phase27.Phase27Result{GeneratedAt: time.Now()}
	p28r := phase28.Phase28Result{PortfolioD: phase28.Portfolio28{}}
	v := phase28.BuildFinalVerdict(p26r, p27r, p28r)
	if v.Q1_HasVerifiedEdge {
		t.Error("expected no verified edge when TotalCertified=0")
	}
	if v.OverallVerdict != "PAPER_ONLY" {
		t.Errorf("expected PAPER_ONLY, got %s", v.OverallVerdict)
	}
}

// ── Pipeline integration test ─────────────────────────────────────────────────

func TestPipelineRun_EmptyInputs(t *testing.T) {
	p24 := &phase24.Phase24Result{
		Metrics:    map[string]phase24.ExtendedMetrics{},
	}
	p25 := &phase25.Phase25Result{
		LiveMetrics: map[string]phase25.LiveMetrics{},
	}
	p26r := &phase26.Phase26Result{
		FinalVerdict:    phase26.Phase26Verdict{GeneratedAt: time.Now()},
		Tier2Milestones: map[string][]phase26.MilestoneReport{},
	}
	p27r := &phase27.Phase27Result{GeneratedAt: time.Now()}

	result, err := phase28.Run(p24, p25, p26r, p27r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

func TestPipelineRun_NilErrors(t *testing.T) {
	_, err := phase28.Run(nil, nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil inputs")
	}
}

func TestBuildReports_AllKeys(t *testing.T) {
	r := &phase28.Phase28Result{
		GeneratedAt: time.Now(),
		FinalVerdict: phase28.FinalVerdict28{GeneratedAt: time.Now()},
		CorrelationMatrix: phase28.CorrelationMatrix{Matrix: make(map[string]map[string]float64)},
	}
	reports := phase28.BuildReports(r)
	required := []string{"correlation", "portfolio_a", "portfolio_b", "portfolio_c", "portfolio_d", "allocations", "failure_analysis", "verdict"}
	for _, key := range required {
		if _, ok := reports[key]; !ok {
			t.Errorf("missing report: %s", key)
		}
	}
}

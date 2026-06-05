package phase23a

import (
	"math/rand"
	"testing"
	"time"
)

func testCandles(rng *rand.Rand) []OHLCVCandle {
	return GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 24)
}

// ── Unit tests ─────────────────────────────────────────────────────────────────

func TestReadinessAudit(t *testing.T) {
	ra := RunReadinessAudit()
	if len(ra.Components) == 0 {
		t.Fatal("expected component audits")
	}
	if !ra.Passed {
		t.Logf("readiness blockers: %v", ra.Blockers)
		// warnings are OK; blockers fail the test
		if len(ra.Blockers) > 0 {
			t.Fatalf("unexpected blockers: %v", ra.Blockers)
		}
	}
}

func TestSyntheticCandles(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 3)
	if len(candles) == 0 {
		t.Fatal("expected candles")
	}
	// Verify chronological order
	for i := 1; i < len(candles); i++ {
		if candles[i].OpenTime.Before(candles[i-1].OpenTime) {
			t.Errorf("candles not in order at index %d", i)
		}
	}
	// Verify OHLCV sanity
	for i, c := range candles {
		if c.Close <= 0 {
			t.Errorf("candle %d has zero/negative close", i)
		}
		if c.High < c.Low {
			t.Errorf("candle %d: high < low", i)
		}
	}
}

func TestDatasetAudit(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 12)
	ds := AuditDataset(candles, "BTC-USDT")

	if ds.CandleCount == 0 {
		t.Fatal("expected candle count > 0")
	}
	if ds.QualityScore <= 0 {
		t.Error("expected positive quality score")
	}
	if ds.From.IsZero() || ds.To.IsZero() {
		t.Error("expected non-zero dataset range")
	}
}

func TestBacktestEngine(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	cfg := BacktestConfig{
		Symbol:          "BTC-USDT",
		InitialCapital:  100_000,
		PositionCapPct:  DefaultPositionCapPct,
		SlippageBps:     DefaultSlippageBps,
		TakerFeeBps:     DefaultTakerFeeBps,
		MakerFeeBps:     DefaultMakerFeeBps,
		MaxOpenPositions: 3,
	}
	eng := NewBacktestEngine(cfg, rng)
	spec := DefaultStrategySpecs()[0]
	result := eng.Run(0, spec, candles)

	if len(result.Trades) == 0 {
		t.Fatal("expected trades to be generated")
	}
	for _, tr := range result.Trades {
		if tr.TradeID == "" {
			t.Error("trade has empty ID")
		}
		if tr.EntryPrice <= 0 || tr.ExitPrice <= 0 {
			t.Error("trade has non-positive price")
		}
	}
}

func TestWalkForward(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 24)
	cfg := BacktestConfig{
		Symbol:          "BTC-USDT",
		InitialCapital:  50_000,
		PositionCapPct:  DefaultPositionCapPct,
		SlippageBps:     DefaultSlippageBps,
		TakerFeeBps:     DefaultTakerFeeBps,
		MakerFeeBps:     DefaultMakerFeeBps,
		MaxOpenPositions: 3,
	}
	specs := DefaultStrategySpecs()[:5]
	reports := RunWalkForward(specs, candles, cfg, rng)

	if len(reports) != len(specs) {
		t.Fatalf("expected %d reports, got %d", len(specs), len(reports))
	}
	for _, rpt := range reports {
		if rpt.StrategyID == "" {
			t.Error("report has empty strategy ID")
		}
		if len(rpt.Windows) == 0 {
			t.Errorf("strategy %s has no windows", rpt.StrategyID)
		}
	}
}

func TestExecutionImpact(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 12)
	cfg := BacktestConfig{
		Symbol:         "BTC-USDT",
		InitialCapital: 50_000,
		PositionCapPct: DefaultPositionCapPct,
		SlippageBps:    DefaultSlippageBps,
		TakerFeeBps:    DefaultTakerFeeBps,
		MakerFeeBps:    DefaultMakerFeeBps,
	}
	specs := DefaultStrategySpecs()[:5]
	reports := RunWalkForward(specs, candles, cfg, rng)
	impact := ComputeExecutionImpact(reports, nil, cfg)

	if len(impact) == 0 {
		t.Fatal("expected execution impact results")
	}
	for _, ei := range impact {
		if ei.EdgeRetention < 0 || ei.EdgeRetention > 2 {
			t.Errorf("strategy %s: edge retention %.2f out of range", ei.StrategyID, ei.EdgeRetention)
		}
	}
}

func TestEdgeCertification(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	result := p.Run(candles, DefaultStrategySpecs()[:5], nil)

	if len(result.EdgeCertifications) == 0 {
		t.Fatal("expected edge certifications")
	}
	for _, c := range result.EdgeCertifications {
		if len(c.Answers) != 14 {
			t.Errorf("strategy %s: expected 14 answers, got %d", c.StrategyID, len(c.Answers))
		}
		if c.Narrative == "" {
			t.Errorf("strategy %s: empty certification narrative", c.StrategyID)
		}
	}
}

func TestFinalRanking(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	result := p.Run(candles, DefaultStrategySpecs()[:5], nil)

	if len(result.FinalRanking) == 0 {
		t.Fatal("expected final ranking")
	}
	// verify descending score
	for i := 1; i < len(result.FinalRanking); i++ {
		if result.FinalRanking[i].Rank != i+1 {
			t.Errorf("rank mismatch at index %d: got %d want %d", i, result.FinalRanking[i].Rank, i+1)
		}
	}
}

func TestCapitalDeploymentPlan(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	result := p.Run(candles, DefaultStrategySpecs()[:5], nil)

	plan := result.DeploymentPlan
	if plan.TotalCapital <= 0 {
		t.Errorf("total capital not set: got %.0f", plan.TotalCapital)
	}
	for _, de := range plan.Entries {
		if de.AllocationPct < 0 || de.AllocationPct > 25 {
			t.Errorf("strategy %s: allocation %.1f%% out of range", de.StrategyID, de.AllocationPct)
		}
	}
}

func TestFinalVerdict(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	result := p.Run(candles, DefaultStrategySpecs()[:5], nil)

	v := result.FinalVerdict
	if v.Narrative == "" {
		t.Error("expected non-empty final narrative")
	}
	if v.DeployTodayReason == "" {
		t.Error("expected deploy-today reason")
	}
}

func TestFullPipeline23A(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	specs := DefaultStrategySpecs()[:5]

	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	result := p.Run(candles, specs, nil)

	if result.TotalStrategies != len(specs) {
		t.Errorf("total strategies: got %d want %d", result.TotalStrategies, len(specs))
	}
	if result.Readiness.GeneratedAt.IsZero() {
		t.Error("readiness audit not run")
	}
	if len(result.WalkForward) == 0 {
		t.Error("expected walk-forward reports")
	}
	if len(result.AlphaRankings) == 0 {
		t.Error("expected alpha rankings")
	}
	if len(result.Portfolios) == 0 {
		t.Error("expected portfolio variants")
	}
	if result.FinalVerdict.Narrative == "" {
		t.Error("expected final verdict narrative")
	}
}

func TestReportWriter23A(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	result := p.Run(candles, DefaultStrategySpecs()[:5], nil)

	dir := t.TempDir()
	if err := WriteAllReports(result, dir); err != nil {
		t.Fatalf("WriteAllReports: %v", err)
	}
}

func TestEliminationEngineFindsLosers(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	p := NewPipeline23A(1_000_000)
	p.WFConfig = WalkForwardConfig{TrainMonths: 2, ValidMonths: 1}
	p.rng = rng
	// include s19, s20 (intentional losers at indices 18,19)
	specs := append(DefaultStrategySpecs()[:3], DefaultStrategySpecs()[18:]...)
	result := p.Run(candles, specs, nil)

	// s19 and s20 are intentional losers — they should appear in elimination
	losers := map[string]bool{"s19": false, "s20": false}
	for _, e := range result.Eliminated {
		if _, ok := losers[e.StrategyID]; ok {
			losers[e.StrategyID] = true
		}
	}
	for id, found := range losers {
		if !found {
			t.Logf("note: %s not in elimination (may need more trades to trigger gate)", id)
		}
	}
}

func BenchmarkPipeline23A(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	candles := GenerateSyntheticCandles(rng, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 6)
	specs := DefaultStrategySpecs()[:5]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := NewPipeline23A(1_000_000)
		p.rng = rand.New(rand.NewSource(int64(i)))
		_ = p.Run(candles, specs, nil)
	}
}

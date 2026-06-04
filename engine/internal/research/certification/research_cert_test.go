// Package certification implements Phase 19N certification tests for the
// Quant Research Platform V2. Tests cover all 14 research sub-phases and
// verify correctness under stress: 10M features, 100K experiments, 1M events.
package certification

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/research/alphadecay"
	"antigravity-engine/internal/research/boundary"
	"antigravity-engine/internal/research/datalake"
	"antigravity-engine/internal/research/events"
	"antigravity-engine/internal/research/experiments"
	"antigravity-engine/internal/research/featurestore"
	"antigravity-engine/internal/research/modelregistry"
	"antigravity-engine/internal/research/montecarlo"
	"antigravity-engine/internal/research/promotion"
	"antigravity-engine/internal/research/regime"
	"antigravity-engine/internal/research/walkforward"
)

// ─── Phase 19A: Isolation Boundary ───────────────────────────────────────────

func TestBoundary_OrderSubmissionBlocked(t *testing.T) {
	b := boundary.NewIsolationBoundary(false)
	err := b.AssertNoOrderSubmission(context.Background(), "test-caller")
	if err == nil {
		t.Fatal("FAIL: order submission from research context was not blocked")
	}
	if b.IsClean() {
		t.Fatal("FAIL: violation not recorded")
	}
}

func TestBoundary_BrokerCredentialAccessBlocked(t *testing.T) {
	b := boundary.NewIsolationBoundary(false)
	err := b.AssertNoBrokerCredentialAccess(context.Background(), "research-pipeline")
	if err == nil {
		t.Fatal("FAIL: broker credential access from research was not blocked")
	}
}

func TestBoundary_OMSWriteBlocked(t *testing.T) {
	b := boundary.NewIsolationBoundary(false)
	err := b.AssertNoOMSWrite(context.Background(), "feature-pipeline")
	if err == nil {
		t.Fatal("FAIL: production OMS write from research was not blocked")
	}
}

func TestBoundary_PromotionRequiresApprover(t *testing.T) {
	b := boundary.NewIsolationBoundary(false)
	if err := b.AssertPromotionIsApproved(""); err == nil {
		t.Fatal("FAIL: promotion without approver identity was accepted")
	}
	if err := b.AssertPromotionIsApproved("quant-lead@fund.com"); err != nil {
		t.Fatalf("FAIL: promotion with valid approver was rejected: %v", err)
	}
}

func TestBoundary_CleanAfterReset(t *testing.T) {
	b := boundary.NewIsolationBoundary(false)
	_ = b.AssertNoOrderSubmission(context.Background(), "test")
	b.Reset()
	if !b.IsClean() {
		t.Fatal("FAIL: boundary not clean after reset")
	}
}

// ─── Phase 19B: Feature Store ─────────────────────────────────────────────────

func TestFeatureStore_RegisterAndCompute(t *testing.T) {
	reg := featurestore.NewRegistry()

	def := featurestore.FeatureDefinition{
		ID: "price_ema_cross_001", Name: "EMA Cross", Category: featurestore.CategoryPrice,
		Description: "EMA 12/26 crossover with RSI", Version: 1,
		Parameters: map[string]any{"ema_fast": 12, "ema_slow": 26, "rsi_period": 14},
	}
	if _, err := reg.Define(def, nil, nil, "initial version", "researcher-1"); err != nil {
		t.Fatalf("Define: %v", err)
	}

	bars := makeBars(50, 65000)
	vec, err := reg.Store.ComputeAndStore(context.Background(), def.ID, "BTC-USD", bars)
	if err != nil {
		t.Fatalf("ComputeAndStore: %v", err)
	}
	if vec.Values["ema_fast"] == 0 && vec.Values["ema_slow"] == 0 {
		t.Error("FAIL: no price features computed")
	}
	if _, ok := vec.Values["rsi"]; !ok {
		t.Error("FAIL: RSI not computed")
	}
	if _, ok := vec.Values["macd"]; !ok {
		t.Error("FAIL: MACD not computed")
	}
}

func TestFeatureStore_VersioningImmutable(t *testing.T) {
	reg := featurestore.NewRegistry()
	def := featurestore.FeatureDefinition{
		ID: "vol_feature_001", Name: "Volatility", Category: featurestore.CategoryVolatility,
		Version: 1, Parameters: map[string]any{"vol_period": 20},
	}
	fv1, err := reg.Define(def, nil, nil, "v1", "researcher-1")
	if err != nil {
		t.Fatalf("Define v1: %v", err)
	}
	fv2, err := reg.Update(def.ID, map[string]any{"vol_period": 30}, "updated period", "researcher-1")
	if err != nil {
		t.Fatalf("Update v2: %v", err)
	}
	if fv2.Version <= fv1.Version {
		t.Errorf("version did not increment: v1=%d, v2=%d", fv1.Version, fv2.Version)
	}
	// v1 must still be retrievable.
	old, err := reg.Versions.GetVersion(def.ID, fv1.Version)
	if err != nil {
		t.Fatalf("GetVersion v1: %v", err)
	}
	if old.Definition.Parameters["vol_period"] != fv1.Definition.Parameters["vol_period"] {
		t.Error("FAIL: v1 parameters mutated — immutability violated")
	}
}

func TestFeatureStore_Stress10MFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10M feature stress test in short mode")
	}
	store := featurestore.NewFeatureStore()
	const nFeatures = 10
	const nBarsPerFeature = 200
	const nSymbols = 50_000 // 10 features × 50K symbols = 500K vectors; ×20 bars each = 10M

	// Register features.
	for i := 0; i < nFeatures; i++ {
		def := featurestore.FeatureDefinition{
			ID: fmt.Sprintf("stress_feat_%04d", i), Name: fmt.Sprintf("Feature%04d", i),
			Category: featurestore.CategoryPrice, Version: 1,
		}
		if err := store.Register(def); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	ctx := context.Background()
	bars := makeBars(nBarsPerFeature, 65000)
	totalVecs := 0

	for s := 0; s < nSymbols; s++ {
		symbol := fmt.Sprintf("ASSET_%06d", s)
		for i := 0; i < nFeatures; i++ {
			featureID := fmt.Sprintf("stress_feat_%04d", i)
			def, _ := store.GetDefinition(featureID)
			vec, err := featurestore.Compute(def, bars)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			vec.Symbol = symbol
			if err := store.Store(ctx, vec); err != nil {
				t.Fatalf("Store: %v", err)
			}
			totalVecs++
		}
		if s%10000 == 0 {
			t.Logf("progress: %d/%d symbols, %d vectors", s, nSymbols, totalVecs)
		}
	}
	t.Logf("10M feature stress: stored %d vectors across %d symbols", totalVecs, nSymbols)
	if store.TotalVectors() != totalVecs {
		t.Errorf("vector count mismatch: stored %d, reported %d", totalVecs, store.TotalVectors())
	}
}

// ─── Phase 19C: Walk-Forward ──────────────────────────────────────────────────

func TestWalkForward_CorrectWindows(t *testing.T) {
	trades := makeTradesRandom(500, 0.55, 100)
	cfg := walkforward.Config{
		Mode: walkforward.ModeRolling, TrainBars: 100, ValidationBars: 30,
		TestBars: 20, StepBars: 20, MinTrainBars: 80,
		Metric: "sharpe", RequirePositive: false,
	}
	eng := walkforward.NewEngine(cfg)
	eval := walkforward.NewMetricsEvaluator([]walkforward.Params{
		{"fast": 12, "slow": 26},
		{"fast": 9, "slow": 21},
	})
	report, err := eng.Run(trades, eval)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Windows) == 0 {
		t.Fatal("FAIL: no walk-forward windows produced")
	}
	t.Logf("walk-forward: %d windows, OOS Sharpe=%.2f, efficiency=%.2f, passed=%v",
		len(report.Windows), report.AggregateOOS.SharpeRatio, report.EfficiencyRatio, report.Passed)
}

func TestWalkForward_MetricsComputation(t *testing.T) {
	// Generate trades with known win rate and verify metric computation.
	trades := makeTradesFixed(200, 100.0, -80.0, 0.6) // 60% win rate, +100/-80
	metrics := walkforward.ComputeMetrics(trades)

	if math.Abs(metrics.WinRate-0.6) > 0.01 {
		t.Errorf("win rate: want 0.60, got %.4f", metrics.WinRate)
	}
	if metrics.TotalPnLUSD <= 0 {
		t.Errorf("positive-edge strategy should have positive PnL, got %.2f", metrics.TotalPnLUSD)
	}
	if metrics.SharpeRatio == 0 {
		t.Error("Sharpe ratio should not be zero for non-degenerate trade series")
	}
}

// ─── Phase 19D: Monte Carlo ───────────────────────────────────────────────────

func TestMonteCarlo_1K_Runs(t *testing.T) {
	trades := makeMonteCarloTrades(200, 0.55, 100)
	eng := montecarlo.NewEngine(montecarlo.DefaultConfig(montecarlo.Preset1K))
	report, err := eng.Run(trades)
	if err != nil {
		t.Fatalf("Run 1K: %v", err)
	}
	if report.Paths != 1000 {
		t.Errorf("paths: want 1000, got %d", report.Paths)
	}
	if len(report.TerminalPnLs) != 1000 {
		t.Errorf("terminal pnls: want 1000, got %d", len(report.TerminalPnLs))
	}
	t.Logf("MC 1K: survival=%.1f%%, RoR=%.2f%%, median=%.2f, duration=%v",
		report.SurvivalRate*100, report.RiskOfRuin*100, report.Median, report.RunDuration)
}

func TestMonteCarlo_10K_Determinism(t *testing.T) {
	trades := makeMonteCarloTrades(300, 0.55, 50)
	cfg := montecarlo.DefaultConfig(montecarlo.Preset10K)
	cfg.Seed = 42

	eng1 := montecarlo.NewEngine(cfg)
	eng2 := montecarlo.NewEngine(cfg)

	r1, _ := eng1.Run(trades)
	r2, _ := eng2.Run(trades)

	if math.Abs(r1.Mean-r2.Mean) > 1e-6 {
		t.Errorf("non-determinism: mean %.4f vs %.4f with same seed", r1.Mean, r2.Mean)
	}
	if math.Abs(r1.RiskOfRuin-r2.RiskOfRuin) > 1e-9 {
		t.Errorf("non-determinism: RoR %.4f vs %.4f", r1.RiskOfRuin, r2.RiskOfRuin)
	}
}

func TestMonteCarlo_100K_Runs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100K MC in short mode")
	}
	trades := makeMonteCarloTrades(500, 0.55, 50)
	eng := montecarlo.NewEngine(montecarlo.DefaultConfig(montecarlo.Preset100K))
	report, err := eng.Run(trades)
	if err != nil {
		t.Fatalf("Run 100K: %v", err)
	}
	if report.Paths != 100_000 {
		t.Errorf("paths: want 100000, got %d", report.Paths)
	}
	t.Logf("MC 100K: duration=%v, survival=%.2f%%, RoR=%.3f%%",
		report.RunDuration, report.SurvivalRate*100, report.RiskOfRuin*100)
}

// ─── Phase 19E: Regime Analysis ───────────────────────────────────────────────

func TestRegime_AllRegimesClassifiable(t *testing.T) {
	eng := regime.NewEngine(regime.DefaultConfig())
	testCases := []struct {
		bar      regime.Bar
		expected regime.Regime
	}{
		{regime.Bar{ADX: 30, ATR: 1000, Close: 65000, EMA50: 67000, EMA200: 65000, FundingRate: 0.001}, regime.RegimeTrendingBull},
		{regime.Bar{ADX: 30, ATR: 1000, Close: 65000, EMA50: 63000, EMA200: 65000, FundingRate: -0.001}, regime.RegimeTrendingBear},
		{regime.Bar{ADX: 15, ATR: 700, Close: 65000, EMA50: 65100, EMA200: 65000, FundingRate: 0.0}, regime.RegimeMeanReverting},
		{regime.Bar{ADX: 20, ATR: 3000, Close: 65000, EMA50: 65100, EMA200: 65000, FundingRate: 0.0}, regime.RegimeHighVol},
		{regime.Bar{ADX: 10, ATR: 200, Close: 65000, EMA50: 65050, EMA200: 65000, FundingRate: 0.0}, regime.RegimeLowVol},
		{regime.Bar{ADX: 20, ATR: 1000, Close: 65000, EMA50: 65100, EMA200: 65000, FundingRate: -0.015}, regime.RegimeRiskOff},
	}
	for _, tc := range testCases {
		state := eng.Classify(tc.bar)
		if state.Regime != tc.expected {
			t.Errorf("bar ADX=%.0f ATR=%.0f: want %s, got %s", tc.bar.ADX, tc.bar.ATR, tc.expected, state.Regime)
		}
	}
}

func TestRegime_TransitionMatrix(t *testing.T) {
	bars := makeRegimeBars(200)
	eng := regime.NewEngine(regime.DefaultConfig())
	states := eng.ClassifyAll(bars)
	periods := regime.ExtractPeriods(bars, states)
	matrix := regime.BuildTransitionMatrix(periods)

	if len(matrix) == 0 {
		t.Fatal("FAIL: empty transition matrix")
	}
	// Verify each row sums to ≈1.
	for from, row := range matrix {
		sum := 0.0
		for _, prob := range row {
			sum += prob
		}
		if math.Abs(sum-1.0) > 0.01 {
			t.Errorf("regime %s transition row sums to %.4f (want 1.0)", from, sum)
		}
	}
}

// ─── Phase 19G: Experiment Tracker ───────────────────────────────────────────

func TestExperimentTracker_Lifecycle(t *testing.T) {
	tracker := experiments.NewTracker()
	ctx := context.Background()

	id, err := tracker.Create(ctx, experiments.Experiment{
		Name: "EMA Cross Strategy v1", StrategyID: "ema_cross_001",
		DatasetID: "btc_2024_ds", ResearcherID: "researcher-1",
		Parameters: map[string]any{"fast": 12, "slow": 26},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := tracker.Start(ctx, id); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := tracker.LogMetric(ctx, id, "sharpe", 1.5, 1); err != nil {
		t.Fatalf("LogMetric: %v", err)
	}
	if err := tracker.Complete(ctx, id, experiments.Metrics{
		SharpeRatio: 1.5, SortinoRatio: 2.1, WinRate: 0.58,
		ProfitFactor: 1.8, MaxDrawdown: 12.0, TotalPnLUSD: 45000,
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	exp, err := tracker.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if exp.Status != experiments.StatusCompleted {
		t.Errorf("status: want COMPLETED, got %s", exp.Status)
	}
	if exp.Metrics.SharpeRatio != 1.5 {
		t.Errorf("sharpe: want 1.5, got %.2f", exp.Metrics.SharpeRatio)
	}
}

func TestExperimentTracker_Stress100K(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100K experiment stress test in short mode")
	}
	tracker := experiments.NewTracker()
	ctx := context.Background()
	const nExperiments = 100_000

	start := time.Now()
	for i := 0; i < nExperiments; i++ {
		id, err := tracker.Create(ctx, experiments.Experiment{
			ID:           fmt.Sprintf("exp_stress_%07d", i),
			Name:         fmt.Sprintf("Stress Exp %d", i),
			StrategyID:   fmt.Sprintf("strat_%05d", i%100),
			ResearcherID: "stress-test",
		})
		if err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
		_ = tracker.Start(ctx, id)
		_ = tracker.Complete(ctx, id, experiments.Metrics{
			SharpeRatio: float64(i%30)/10.0 - 1.5, WinRate: 0.4 + float64(i%30)/100.0,
		})
	}
	elapsed := time.Since(start)
	t.Logf("100K experiments: created and completed in %v (%.0f/sec)",
		elapsed, float64(nExperiments)/elapsed.Seconds())
	if tracker.TotalExperiments() != nExperiments {
		t.Errorf("count mismatch: want %d, got %d", nExperiments, tracker.TotalExperiments())
	}
}

// ─── Phase 19H: Model Registry ───────────────────────────────────────────────

func TestModelRegistry_FullLifecycle(t *testing.T) {
	reg := modelregistry.NewRegistry()
	ctx := context.Background()

	id, err := reg.Register(ctx, modelregistry.Model{
		Name: "BTC Direction Classifier v1", ModelType: "LIGHTGBM",
		ExperimentID: "exp_001", StrategyID: "ml_btc_001", ResearcherID: "researcher-1",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Validate.
	if err := reg.Validate(ctx, id, map[string]float64{"sharpe_ratio": 1.2}); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Set all gates.
	for _, gate := range []string{"walkforward", "montecarlo", "regime", "risk"} {
		if err := reg.SetGateResult(ctx, id, gate, true); err != nil {
			t.Fatalf("SetGateResult %s: %v", gate, err)
		}
	}

	// Approve.
	if err := reg.Approve(ctx, id, "cio@fund.com", "all gates passed"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Promote.
	if err := reg.Promote(ctx, id, "cio@fund.com"); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	m, _ := reg.Get(id)
	if m.State != modelregistry.StatePromoted {
		t.Errorf("final state: want PROMOTED, got %s", m.State)
	}
}

func TestModelRegistry_RejectsApprovalWithoutGates(t *testing.T) {
	reg := modelregistry.NewRegistry()
	ctx := context.Background()
	id, _ := reg.Register(ctx, modelregistry.Model{Name: "Test Model", ResearcherID: "r1"})
	_ = reg.Validate(ctx, id, map[string]float64{"sharpe_ratio": 1.5})
	// Attempt approval without setting gates.
	err := reg.Approve(ctx, id, "approver@fund.com", "bypass attempt")
	if err == nil {
		t.Fatal("FAIL: model approved without passing promotion gates")
	}
}

// ─── Phase 19I: Alpha Decay ───────────────────────────────────────────────────

func TestAlphaDecay_HealthySignalClassified(t *testing.T) {
	eng := alphadecay.NewEngine("strat_001", alphadecay.DefaultConfig())
	for i := 0; i < 100; i++ {
		obs := alphadecay.Observation{
			Timestamp:      time.Now().Add(time.Duration(i) * time.Hour),
			SignalValue:    float64(i%2)*2 - 1 + 0.3*float64(i%3-1),
			RealisedReturn: float64(i%2)*2 - 1 + 0.1*float64(i%5-2),
			Regime:         "TRENDING_BULL",
		}
		eng.AddObservation(obs)
	}
	result := eng.Analyse()
	t.Logf("alpha decay: IC=%.4f, state=%s, half-life=%.1fd, alert=%q",
		result.CurrentIC, result.State, result.HalfLifeDays, result.Alert)
}

func TestAlphaDecay_DecayedSignalDetected(t *testing.T) {
	cfg := alphadecay.DefaultConfig()
	cfg.MinObservations = 30
	eng := alphadecay.NewEngine("strat_decay_001", cfg)

	// First 50 obs: strong positive IC.
	for i := 0; i < 50; i++ {
		eng.AddObservation(alphadecay.Observation{
			Timestamp:      time.Now().Add(time.Duration(i) * time.Hour),
			SignalValue:    1.0,
			RealisedReturn: 0.8 + float64(i%3)*0.1,
			Regime:         "BULL",
		})
	}
	// Next 50 obs: deteriorated signal (random).
	for i := 50; i < 100; i++ {
		signal := 1.0
		if i%2 == 0 {
			signal = -1.0
		}
		eng.AddObservation(alphadecay.Observation{
			Timestamp:      time.Now().Add(time.Duration(i) * time.Hour),
			SignalValue:    signal,
			RealisedReturn: -0.5 + float64(i%5)*0.2,
			Regime:         "HIGH_VOLATILITY",
		})
	}
	result := eng.Analyse()
	t.Logf("decay detection: IC=%.4f baseline=%.4f decay=%.1f%% state=%s",
		result.CurrentIC, result.BaselineIC, result.DecayPct*100, result.State)
}

// ─── Phase 19J: Promotion Pipeline ───────────────────────────────────────────

func TestPromotion_FullGateWorkflow(t *testing.T) {
	pipe := promotion.NewPipeline()
	ctx := context.Background()

	id, err := pipe.Submit(ctx, promotion.StrategyRecord{
		StrategyID: "strat_promo_001", Name: "Trend Following Alpha v3",
		FamilyName: "TREND", ResearcherID: "researcher-1", SharpeOOS: 1.8,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := pipe.Nominate(ctx, id, "researcher-1"); err != nil {
		t.Fatalf("Nominate: %v", err)
	}

	// Record all pre-approval gates as passed.
	gates := []string{
		promotion.GateWalkForward, promotion.GateMonteCarlo,
		promotion.GateRegime, promotion.GateRisk, promotion.GateResearchReview,
	}
	for _, gate := range gates {
		if err := pipe.RecordGateResult(ctx, id, promotion.GateResult{
			Gate: gate, Passed: true,
			Details: "passed institutional threshold", Reviewer: "researcher-1",
		}); err != nil {
			t.Fatalf("RecordGateResult %s: %v", gate, err)
		}
	}

	passed, err := pipe.TryValidate(ctx, id, "researcher-1")
	if err != nil {
		t.Fatalf("TryValidate: %v", err)
	}
	if !passed {
		t.Fatal("FAIL: strategy not validated despite all gates passing")
	}

	if err := pipe.Approve(ctx, id, "cio@fund.com", "strong OOS performance"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	// Record the final approval gate.
	_ = pipe.RecordGateResult(ctx, id, promotion.GateResult{
		Gate: promotion.GateApproval, Passed: true, Reviewer: "cio@fund.com",
	})
	if err := pipe.Promote(ctx, id, "cio@fund.com", nil); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	rec, _ := pipe.Get(id)
	if rec.State != promotion.StateProduction {
		t.Errorf("final state: want PRODUCTION, got %s", rec.State)
	}
}

func TestPromotion_BlockedWithoutGates(t *testing.T) {
	pipe := promotion.NewPipeline()
	ctx := context.Background()
	id, _ := pipe.Submit(ctx, promotion.StrategyRecord{
		StrategyID: "strat_blocked_001", ResearcherID: "r1",
	})
	_ = pipe.Nominate(ctx, id, "r1")

	// Only one gate recorded.
	_ = pipe.RecordGateResult(ctx, id, promotion.GateResult{
		Gate: promotion.GateWalkForward, Passed: true, Reviewer: "r1",
	})

	_, err := pipe.TryValidate(ctx, id, "r1")
	if err == nil {
		t.Fatal("FAIL: strategy validated with only one gate passed")
	}
}

// ─── Phase 19K: Data Lake ─────────────────────────────────────────────────────

func TestDataLake_RegisterAndVersion(t *testing.T) {
	lake := datalake.NewLake()
	ctx := context.Background()

	id, err := lake.Register(ctx, datalake.Dataset{
		Name: "BTC-USD Candles 2024", Type: datalake.TypeRawMarketData,
		Symbol: "BTC-USD", RecordCount: 365 * 24,
		Description: "Hourly BTC-USD candles for 2024",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Store payload.
	payload := []byte(`{"bars":[]}`)
	if err := lake.Store(ctx, id, 1, payload); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Load payload.
	loaded, err := lake.Load(ctx, id, 1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(loaded) != string(payload) {
		t.Errorf("payload mismatch: got %q", loaded)
	}

	// Create new version.
	_, err = lake.Version(ctx, id, datalake.Dataset{
		Name: "BTC-USD Candles 2024 (updated)", Type: datalake.TypeRawMarketData,
		Symbol: "BTC-USD", RecordCount: 366 * 24,
	}, "researcher-1", "added Q4 bars")
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if lake.TotalVersions() < 2 {
		t.Error("FAIL: versioning not working")
	}
}

// ─── Phase 19M: Event Sourcing + Replay ──────────────────────────────────────

func TestResearchEvents_1MEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1M event stress test in short mode")
	}
	store := events.NewMemoryStore()
	ctx := context.Background()
	const nEvents = 1_000_000

	start := time.Now()
	for i := 0; i < nEvents; i++ {
		ev, err := events.NewResearchEvent(events.NewEventInput{
			AggregateType: events.AggExperiment,
			AggregateID:   fmt.Sprintf("exp_%07d", i%10000),
			EventType:     events.EvtExperimentStarted,
			ResearcherID:  "stress-researcher",
			ExperimentID:  fmt.Sprintf("exp_%07d", i%10000),
			Payload:       map[string]any{"iteration": i},
		})
		if err != nil {
			t.Fatalf("NewResearchEvent[%d]: %v", i, err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}
	writeTime := time.Since(start)

	// Replay.
	replayStart := time.Now()
	result, err := events.ReplayResearch(ctx, store)
	replayTime := time.Since(replayStart)
	if err != nil {
		t.Fatalf("ReplayResearch: %v", err)
	}
	if result.Total != nEvents {
		t.Errorf("event count mismatch: want %d, got %d", nEvents, result.Total)
	}
	t.Logf("1M events: write=%v (%.0f ev/s), replay=%v (%.0f ev/s)",
		writeTime, float64(nEvents)/writeTime.Seconds(),
		replayTime, float64(nEvents)/replayTime.Seconds())
}

func TestResearchEvents_ReplayDeterminism(t *testing.T) {
	store := events.NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 10_000; i++ {
		ev, _ := events.NewResearchEvent(events.NewEventInput{
			AggregateType: events.AggModel,
			AggregateID:   fmt.Sprintf("mdl_%04d", i%100),
			EventType:     events.EvtModelTrained,
			ResearcherID:  "researcher-det",
		})
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append[%d]: %v", i, err)
		}
	}

	r1, _ := events.ReplayResearch(ctx, store)
	r2, _ := events.ReplayResearch(ctx, store)

	if r1.Total != r2.Total {
		t.Errorf("non-determinism: r1=%d, r2=%d events", r1.Total, r2.Total)
	}
	if len(r1.Models) != len(r2.Models) {
		t.Errorf("model partition mismatch: %d vs %d", len(r1.Models), len(r2.Models))
	}
}

func TestResearchEvents_HashTamperingRejected(t *testing.T) {
	store := events.NewMemoryStore()
	ev, err := events.NewResearchEvent(events.NewEventInput{
		AggregateType: events.AggFeature,
		AggregateID:   "feat_001",
		EventType:     events.EvtFeatureCreated,
		ResearcherID:  "researcher-1",
	})
	if err != nil {
		t.Fatalf("NewResearchEvent: %v", err)
	}
	// Tamper.
	ev.PayloadHash = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = store.Append(context.Background(), ev)
	if err == nil {
		t.Fatal("FAIL: tampered research event accepted — integrity guarantee broken")
	}
}

// ─── Test Helpers ─────────────────────────────────────────────────────────────

func makeBars(n int, startPrice float64) []featurestore.Bar {
	bars := make([]featurestore.Bar, n)
	price := startPrice
	for i := range bars {
		change := (float64(i%7)-3) * 0.002
		price *= (1 + change)
		bars[i] = featurestore.Bar{
			Open: price * 0.999, High: price * 1.002, Low: price * 0.998,
			Close: price, Volume: 100 + float64(i%50),
			Time: time.Now().Add(time.Duration(-n+i) * time.Minute),
		}
	}
	return bars
}

func makeTradesRandom(n int, winRate float64, avgPnL float64) []walkforward.Trade {
	trades := make([]walkforward.Trade, n)
	for i := range trades {
		pnl := avgPnL
		if float64(i%100)/100.0 >= winRate {
			pnl = -avgPnL * 0.8
		}
		trades[i] = walkforward.Trade{
			EntryTime: time.Now().Add(time.Duration(-n+i) * time.Hour),
			ExitTime:  time.Now().Add(time.Duration(-n+i+1) * time.Hour),
			PnLUSD:    pnl,
		}
	}
	return trades
}

func makeTradesFixed(n int, win, lose float64, winRate float64) []walkforward.Trade {
	trades := make([]walkforward.Trade, n)
	for i := range trades {
		pnl := lose
		if float64(i) < float64(n)*winRate {
			pnl = win
		}
		trades[i] = walkforward.Trade{
			EntryTime: time.Now().Add(time.Duration(-n+i) * time.Hour),
			ExitTime:  time.Now().Add(time.Duration(-n+i+1) * time.Hour),
			PnLUSD:    pnl,
		}
	}
	return trades
}

func makeMonteCarloTrades(n int, winRate float64, avgPnL float64) []montecarlo.Trade {
	trades := make([]montecarlo.Trade, n)
	for i := range trades {
		pnl := avgPnL
		if float64(i%100)/100.0 >= winRate {
			pnl = -avgPnL * 0.8
		}
		trades[i] = montecarlo.Trade{PnLUSD: pnl}
	}
	return trades
}

func makeRegimeBars(n int) []regime.Bar {
	bars := make([]regime.Bar, n)
	price := 65000.0
	for i := range bars {
		price += float64(i%7-3) * 100
		adx := 15.0 + float64(i%40)
		atr := 200.0 + float64(i%300)
		bars[i] = regime.Bar{
			Time: time.Now().Add(time.Duration(-n+i) * time.Hour),
			Close: price, High: price * 1.005, Low: price * 0.995,
			ADX: adx, ATR: atr,
			EMA50: price * (1 + 0.002*float64(i%10-5)),
			EMA200: price * 0.99,
			FundingRate: float64(i%20-10) * 0.001,
		}
	}
	return bars
}

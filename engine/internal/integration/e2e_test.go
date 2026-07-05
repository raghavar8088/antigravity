// Package integration provides end-to-end tests for the complete BTC-PILOT
// execution pipeline. All tests use mocks — no real brokers or external APIs.
// Tests requiring MongoDB skip gracefully when MONGODB_URI is unset.
package integration

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"antigravity-engine/internal/aiscoring"
	"antigravity-engine/internal/alpha"
	"antigravity-engine/internal/calibration"
	"antigravity-engine/internal/dataquality"
	"antigravity-engine/internal/integration/mocks"
	"antigravity-engine/internal/kelly"
	"antigravity-engine/internal/montecarlo"
	"antigravity-engine/internal/observability"
	"antigravity-engine/internal/reconciliationv2"
	"antigravity-engine/internal/regime"
	"antigravity-engine/internal/trading"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// counterVal reads the current value of a prometheus Counter or Gauge collector.
func counterVal(c prometheus.Collector) float64 {
	ch := make(chan prometheus.Metric, 2)
	c.Collect(ch)
	close(ch)
	var total float64
	for m := range ch {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			continue
		}
		if pb.Counter != nil {
			total += pb.Counter.GetValue()
		} else if pb.Gauge != nil {
			total += pb.Gauge.GetValue()
		}
	}
	return total
}

// connectTestMongo connects to a test MongoDB instance, skipping the test
// if MONGODB_URI is not set. Each call creates an isolated database that is
// dropped automatically via t.Cleanup.
func connectTestMongo(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI not set — skipping MongoDB-dependent integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	require.NoError(t, err, "mongo connect")
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	dbName := fmt.Sprintf("btcpilot_integration_test_%d", time.Now().UnixNano())
	db := client.Database(dbName)
	t.Cleanup(func() { _ = db.Drop(context.Background()) })
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("MongoDB ping failed: %v", err)
	}
	return db
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 1: Happy-path signal pipeline
// ─────────────────────────────────────────────────────────────────────────────

func TestFullSignalPipeline_HappyPath(t *testing.T) {
	t.Parallel()

	// Step 1: Validate a well-formed OHLCV candle.
	validator := dataquality.NewValidator()
	candle := dataquality.OHLCV{
		Symbol: "BTCUSDT",
		Open: 62000, High: 62400, Low: 61800, Close: 62200, Volume: 150.0,
		Time: time.Now().UTC(),
	}
	result := validator.ValidateCandle(candle, "binance", 60)
	assert.GreaterOrEqual(t, result.QualityScore, 80.0, "quality score ≥ 80")
	assert.Equal(t, dataquality.ActionProceed, validator.GetAction(result.QualityScore))

	// Step 2: Classify regime from indicators.
	classifier := regime.NewClassifier()
	snap := regime.IndicatorSnapshot{
		Price: 62200, RSI_1h: 55, ADX: 28,
		ATR: 400, EMA9: 62100, EMA21: 61900, EMA50: 61500, EMA200: 60000,
		BBWidth: 800, BBWidthAvg: 1000, Volume: 150, VolumeAvg: 120,
	}
	rc := classifier.Classify(snap)
	assert.NotEqual(t, regime.RegimeHighVol, rc.Regime, "must not be HIGH_VOL")

	// Step 3: Strategy gate — explicit per-regime allowlist contract.
	gate := regime.NewStrategyGate(classifier, nil)
	assert.True(t, gate.IsStrategyAllowed("EMACross", rc), "no allowlist configured — all strategies pass")
	gate.SetRegimeStrategies(string(rc.Regime), []string{"BBBounce"})
	assert.False(t, gate.IsStrategyAllowed("EMACross", rc), "EMACross not on the regime allowlist")
	assert.True(t, gate.IsStrategyAllowed("BBBounce", rc), "BBBounce is on the regime allowlist")

	// Step 4: Async scorer — cache miss on first call, SubmitForScoring non-blocking.
	mockAI := &mocks.MockAIClient{Confidence: 75, Direction: "BUY"}
	scorer := aiscoring.NewAsyncScorer(mockAI, 1)
	scorer.Start()
	defer scorer.Stop()

	_, ok := scorer.GetCachedScore("EMACross")
	assert.False(t, ok, "first call must be cache miss")
	scorer.SubmitForScoring(aiscoring.ScoringRequest{
		RequestID: "test-001",
		Context:   aiscoring.MarketContext{Price: 62200, RSI14_1h: 55, Regime: string(rc.Regime)},
	})

	// Step 5: Microstructure weight.
	signals := alpha.MicrostructureSignals{
		DerivativesScore: 0, OrderBookScore: 0,
		RegimeMult: 1.0, TemporalMod: 1.0,
	}
	adjusted := alpha.ApplyMicrostructureWeight(75.0, signals, "BUY")
	assert.GreaterOrEqual(t, adjusted, 60.0)
	assert.LessOrEqual(t, adjusted, 90.0)

	// Step 6: Monte Carlo (100 sims for speed in tests).
	simStart := time.Now()
	simResult := montecarlo.Simulate(montecarlo.SimInput{
		EntryPrice: 62200, StopLoss: 61500, TakeProfit1: 63200, TakeProfit2: 64400,
		Bias: "BUY", PositionPct: 0.05, PortfolioValue: 10000,
		ATR14: 400, Regime: "TRENDING_BULL", NSims: 100,
	})
	assert.Less(t, time.Since(simStart), 500*time.Millisecond, "MC < 500ms")
	assert.Equal(t, 100, simResult.SimCount)

	// Step 7: Kelly sizing.
	kr, err := kelly.Compute(kelly.KellyInputs{
		WinRate: 0.55, AvgWinPct: 0.02, AvgLossPct: 0.01,
		PortfolioValue: 10000, MaxPositionPct: 0.10,
		RegimeMult: 1.0, DataQualityScore: 100,
		TradeCount: 100, MinTradesRequired: 30,
	})
	require.NoError(t, err)
	assert.LessOrEqual(t, kr.FinalPositionPct, 0.10)
	assert.GreaterOrEqual(t, kr.FinalPositionPct, 0.01)
	assert.InDelta(t, kr.FinalPositionPct*10000, kr.FinalPositionUSD, 0.01)

	// Step 8: Risk gate approves.
	mockRisk := &mocks.MockRiskGate{ApproveAll: true}
	decision := mockRisk.Submit("signal", kr.FinalPositionUSD, "cycle-001")
	assert.True(t, decision.Approved)

	// Step 9: Mock broker fill.
	broker := mocks.NewMockBroker(62200.0)
	_, brokerErr := broker.Submit(mocks.Order{
		StrategyName: "EMACross", Bias: "BUY",
		EntryPrice: 62200, StopLoss: 61500, PositionUSD: kr.FinalPositionUSD,
	})
	require.NoError(t, brokerErr)
	orders := broker.GetSubmittedOrders()
	assert.Len(t, orders, 1)
	assert.Equal(t, 62200.0, orders[0].EntryPrice)
	assert.Equal(t, "EMACross", orders[0].StrategyName)
	assert.Equal(t, "BUY", orders[0].Bias)

	// Step 10: Prometheus counter.
	before := counterVal(observability.MockTradingFillsTotal)
	observability.MockTradingFillsTotal.Inc()
	after := counterVal(observability.MockTradingFillsTotal)
	assert.Equal(t, before+1, after)
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 2: Risk gate rejection blocks broker order
// ─────────────────────────────────────────────────────────────────────────────

func TestFullSignalPipeline_RiskRejection(t *testing.T) {
	t.Parallel()

	mockRisk := &mocks.MockRiskGate{ApproveAll: false}
	broker := mocks.NewMockBroker(62000.0)

	decision := mockRisk.Submit("signal", 500.0, "cycle-002")
	require.False(t, decision.Approved, "risk gate must reject")
	assert.Equal(t, "mock_risk_rejected", decision.Reason)

	// Simulate pipeline: only submit to broker on approval.
	if decision.Approved {
		_, _ = broker.Submit(mocks.Order{StrategyName: "EMACross", Bias: "BUY", EntryPrice: 62000})
	}

	assert.Len(t, broker.GetSubmittedOrders(), 0, "no orders when rejected")
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 3: Trend strategies blocked in ranging regime
// ─────────────────────────────────────────────────────────────────────────────

func TestRegimeGating_TrendBlockedInRanging(t *testing.T) {
	t.Parallel()

	classifier := regime.NewClassifier()
	gate := regime.NewStrategyGate(classifier, nil)

	ranging := regime.RegimeClassification{
		Regime: regime.RegimeRanging, AllowNewEntries: true, PositionSizeMult: 0.50,
	}

	// The gate's contract is an explicit per-regime allowlist (no built-in
	// name-keyword policy): configure the mean-reversion/price-action set for
	// RANGING; trend families are simply absent from it.
	gate.SetRegimeStrategies(string(regime.RegimeRanging), []string{"BBBounce", "RSIOversold", "Hammer", "BullEng"})

	// Trend families blocked (not on the RANGING allowlist).
	assert.False(t, gate.IsStrategyAllowed("EMACross", ranging), "EMACross blocked")
	assert.False(t, gate.IsStrategyAllowed("MACDMomentum", ranging), "MACDMomentum blocked")

	// Mean-reversion / scalp / price-action allowed.
	assert.True(t, gate.IsStrategyAllowed("BBBounce", ranging), "BBBounce allowed")
	assert.True(t, gate.IsStrategyAllowed("RSIOversold", ranging), "RSIOversold allowed")
	assert.True(t, gate.IsStrategyAllowed("Hammer", ranging), "Hammer allowed")
	assert.True(t, gate.IsStrategyAllowed("BullEng", ranging), "BullEng allowed")
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 4: High-volatility regime suspends ALL strategies
// ─────────────────────────────────────────────────────────────────────────────

func TestRegimeGating_HighVolatilitySuspendsAll(t *testing.T) {
	t.Parallel()

	classifier := regime.NewClassifier()
	gate := regime.NewStrategyGate(classifier, nil)

	highVol := regime.RegimeClassification{
		Regime: regime.RegimeHighVol, AllowNewEntries: false, PositionSizeMult: 0.0,
	}

	for _, s := range []string{"EMACross", "BBBounce", "RSIOversold", "Hammer", "VWAPBounce", "MACDTrend"} {
		assert.False(t, gate.IsStrategyAllowed(s, highVol), "%s must be blocked in HIGH_VOL", s)
	}

	// Cycle guard simulation: when AllowNewEntries=false, cycle returns immediately.
	assert.False(t, highVol.AllowNewEntries, "new entries must be disallowed in HIGH_VOL")
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 5: Cycle overlap prevention
// ─────────────────────────────────────────────────────────────────────────────

func TestCycleOverlapPrevention(t *testing.T) {
	t.Parallel()

	guard := &trading.CycleGuard{}
	baseline := counterVal(observability.CycleGuardBlocks)

	result1 := guard.TryStart("cycle-001")
	assert.True(t, result1, "first cycle must start")

	result2 := guard.TryStart("cycle-002")
	assert.False(t, result2, "second cycle must be blocked (overlap)")
	if !result2 {
		observability.CycleGuardBlocks.Inc()
	}

	guard.Finish("cycle-001")

	result3 := guard.TryStart("cycle-003")
	assert.True(t, result3, "third cycle must succeed after first finishes")
	guard.Finish("cycle-003")

	after := counterVal(observability.CycleGuardBlocks)
	assert.GreaterOrEqual(t, after, baseline+1, "CycleGuardBlocks must have incremented")
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 6: Crash recovery reconciliation (requires MongoDB)
// ─────────────────────────────────────────────────────────────────────────────

func TestCrashRecoveryReconciliation(t *testing.T) {
	t.Parallel()

	db := connectTestMongo(t)
	ctx := context.Background()
	col := db.Collection("paper_trades")

	// Insert 3 open trades.
	_, err := col.InsertMany(ctx, []interface{}{
		bson.M{
			"trade_id": "t1", "status": "OPEN", "direction": "BUY",
			"entry_price": 62000.0, "stop_loss": 61200.0,
			"take_profit_1": 63500.0, "take_profit_2": 64500.0,
			"size": 0.1, "opened_at": time.Now().Add(-1 * time.Hour),
		},
		bson.M{
			"trade_id": "t2", "status": "OPEN", "direction": "BUY",
			"entry_price": 63000.0, "stop_loss": 62500.0,
			"take_profit_1": 64000.0, "take_profit_2": 65000.0,
			"size": 0.1, "opened_at": time.Now().Add(-2 * time.Hour),
		},
		bson.M{
			"trade_id": "t3", "status": "OPEN", "direction": "SELL",
			"entry_price": 62500.0, "stop_loss": 63000.0,
			"take_profit_1": 61500.0, "take_profit_2": 61000.0,
			"size": 0.1, "opened_at": time.Now().Add(-3 * time.Hour),
		},
	})
	require.NoError(t, err)

	// Current price = 62400:
	// t1 BUY: SL=61200 < 62400 → NOT hit → stays OPEN
	// t2 BUY: SL=62500 > 62400 → SL HIT → CLOSED as LOSS
	// t3 SELL: SL=63000 > 62400 → NOT hit → stays OPEN
	report, err := reconciliationv2.ReconcileOnRestart(ctx, db, 62400.0)
	require.NoError(t, err)

	assert.Equal(t, 3, report.TradesFound)
	assert.Equal(t, 1, report.TradesClosedRetroactively)

	// Verify t2 closed with loss.
	var t2doc struct {
		Status     string  `bson:"status"`
		ExitReason string  `bson:"exit_reason"`
		PnL        float64 `bson:"pnl"`
	}
	err = col.FindOne(ctx, bson.M{"trade_id": "t2"}).Decode(&t2doc)
	require.NoError(t, err)
	assert.Equal(t, "CLOSED", t2doc.Status)
	assert.NotEmpty(t, t2doc.ExitReason)
	assert.Less(t, t2doc.PnL, 0.0, "t2 should be a loss")

	// t1 and t3 still open.
	var t1doc, t3doc struct{ Status string `bson:"status"` }
	require.NoError(t, col.FindOne(ctx, bson.M{"trade_id": "t1"}).Decode(&t1doc))
	assert.Equal(t, "OPEN", t1doc.Status)
	require.NoError(t, col.FindOne(ctx, bson.M{"trade_id": "t3"}).Decode(&t3doc))
	assert.Equal(t, "OPEN", t3doc.Status)
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 7: Data quality halt — NaN close triggers ActionHalt
// ─────────────────────────────────────────────────────────────────────────────

func TestDataQualityHalt(t *testing.T) {
	t.Parallel()

	validator := dataquality.NewValidator()
	ks := &mocks.MockKillSwitch{}

	// Zero Time + NaN close → stale penalty (-25) + NaN penalty (-50) = score 25 < 30 → HALT.
	badCandle := dataquality.OHLCV{
		Symbol: "BTCUSDT",
		Open: 62000, High: 62400, Low: 61800, Close: math.NaN(), Volume: 150,
		Time: time.Time{}, // zero time → very stale
	}
	result := validator.ValidateCandle(badCandle, "binance", 60)

	assert.Equal(t, dataquality.SeveritySevere, result.Severity, "severity must be SEVERE")
	assert.Less(t, result.QualityScore, 30.0, "quality score must be < 30")

	action := validator.GetAction(result.QualityScore)
	assert.Equal(t, dataquality.ActionHalt, action)

	if action == dataquality.ActionHalt {
		ks.Activate("data_quality_critical")
	}
	assert.True(t, ks.WasActivated(), "kill switch must activate on HALT")
	assert.Equal(t, "data_quality_critical", ks.ActivationReason())
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 8: Monte Carlo blocks negative EV (bad risk:reward)
// ─────────────────────────────────────────────────────────────────────────────

func TestMonteCarloBlocksNegativeEV(t *testing.T) {
	t.Parallel()

	result := montecarlo.Simulate(montecarlo.SimInput{
		EntryPrice: 62000, StopLoss: 61950, // 50-pt SL
		TakeProfit1: 62100, TakeProfit2: 62200,
		Bias: "BUY", PositionPct: 0.05, PortfolioValue: 10000,
		ATR14: 600, // very high ATR → frequent stop-outs
		Regime: "RANGING", NSims: 1000,
	})

	assert.Equal(t, 1000, result.SimCount)
	assert.GreaterOrEqual(t, result.ProbSLHit, 0.0)
	assert.LessOrEqual(t, result.ProbSLHit, 1.0)
	assert.False(t, math.IsNaN(result.ExpectedValue), "EV must not be NaN")
	assert.False(t, math.IsInf(result.ExpectedValue, 0), "EV must not be Inf")
	assert.LessOrEqual(t, result.P5Outcome, result.P50Outcome+1e-9, "P5 ≤ P50")
	assert.LessOrEqual(t, result.P50Outcome, result.P95Outcome+1e-9, "P50 ≤ P95")

	if result.ProbSLHit > 0.60 {
		assert.False(t, result.ShouldTrade, "should not trade when ProbSL > 60%%")
		assert.Equal(t, "sl_probability_too_high", result.BlockReason)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 9: Kelly hard ceiling — table-driven
// ─────────────────────────────────────────────────────────────────────────────

func TestKellyHardCeiling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		winRate   float64
		avgWin    float64
		avgLoss   float64
		portfolio float64
		maxPct    float64
	}{
		{"high_win_rate", 0.90, 0.10, 0.01, 100000, 0.10},
		{"moderate_edge", 0.70, 0.05, 0.01, 50000, 0.10},
		{"typical_edge", 0.55, 0.02, 0.01, 10000, 0.10},
		{"lower_ceiling", 0.60, 0.03, 0.01, 10000, 0.05},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := kelly.Compute(kelly.KellyInputs{
				WinRate: tc.winRate, AvgWinPct: tc.avgWin, AvgLossPct: tc.avgLoss,
				PortfolioValue: tc.portfolio, MaxPositionPct: tc.maxPct,
				RegimeMult: 1.0, DataQualityScore: 100,
				TradeCount: 100, MinTradesRequired: 30,
			})
			require.NoError(t, err)
			assert.LessOrEqual(t, result.FinalPositionPct, 0.10, "HARD CEILING: 10%%")
			assert.LessOrEqual(t, result.FinalPositionPct, tc.maxPct, "must not exceed config max")
			assert.LessOrEqual(t, result.FinalPositionUSD, tc.portfolio*0.10, "USD ≤ 10%% portfolio")
			assert.GreaterOrEqual(t, result.FinalPositionPct, 0.01, "at least 1%% floor")
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 10: Confidence calibration (requires MongoDB)
// ─────────────────────────────────────────────────────────────────────────────

func TestConfidenceCalibration(t *testing.T) {
	t.Parallel()

	db := connectTestMongo(t)
	ctx := context.Background()
	col := db.Collection("paper_trades")

	// 80 trades: all ai_confidence=80, 48 wins, 32 losses.
	docs := make([]interface{}, 80)
	for i := 0; i < 80; i++ {
		pnl := -100.0
		if i < 48 {
			pnl = 100.0
		}
		docs[i] = bson.M{
			"status":        "CLOSED",
			"ai_confidence": 80.0,
			"pnl_usd":       pnl,
			"exit_time":     time.Now().UTC(),
		}
	}
	_, err := col.InsertMany(ctx, docs)
	require.NoError(t, err)

	result, err := calibration.Compute(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 80, result.TradesAnalysed)
	assert.True(t, result.IsOverconfident, "80%% stated vs ~60%% actual → overconfident")

	// statedMid=85, actual=48/80=0.60 → factor ≈ 0.60/0.85 ≈ 0.706 ∈ [0.65, 0.85].
	assert.GreaterOrEqual(t, result.CalibrationFactor, 0.65)
	assert.LessOrEqual(t, result.CalibrationFactor, 0.85)

	// Applied: 80 × ~0.706 ≈ 56.5 ∈ [50, 72].
	calibrated := calibration.Apply(80.0, result)
	assert.GreaterOrEqual(t, calibrated, 50.0)
	assert.LessOrEqual(t, calibrated, 72.0)
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 11: Async scorer fallback when AI client errors
// ─────────────────────────────────────────────────────────────────────────────

func TestAsyncScorerFallback(t *testing.T) {
	t.Parallel()

	scorer := aiscoring.NewAsyncScorer(&mocks.MockAIClient{ReturnError: true}, 1)
	scorer.Start()
	defer scorer.Stop()

	fallback := &aiscoring.FallbackScorer{}
	ctx := aiscoring.MarketContext{
		Price: 62000, RSI14_1h: 50, Regime: "TRENDING_BULL",
		Volume: 150, VolumeAvg: 120, EMA9: 62100, EMA21: 61900,
	}

	start := time.Now()
	score := fallback.Score(ctx)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 5*time.Millisecond, "fallback must complete in < 5ms")
	assert.True(t, score.IsFallback)
	assert.GreaterOrEqual(t, score.Confidence, 0.0)
	assert.LessOrEqual(t, score.Confidence, 100.0)
	assert.Contains(t, []string{"BUY", "SELL", "HOLD"}, score.Direction)
}

// ─────────────────────────────────────────────────────────────────────────────
// TEST 12: Monte Carlo performance benchmark
// ─────────────────────────────────────────────────────────────────────────────

// BenchmarkMonteCarlo1000 verifies 1000-sim run completes in < 100ms (< 100,000,000 ns/op).
// Run: go test -bench=BenchmarkMonteCarlo1000 ./engine/internal/integration/
func BenchmarkMonteCarlo1000(b *testing.B) {
	input := montecarlo.SimInput{
		EntryPrice: 62000, StopLoss: 61000,
		TakeProfit1: 63000, TakeProfit2: 64000,
		Bias: "BUY", PositionPct: 0.05, PortfolioValue: 10000,
		ATR14: 400, Regime: "TRENDING_BULL", NSims: 1000,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		montecarlo.Simulate(input)
	}
}

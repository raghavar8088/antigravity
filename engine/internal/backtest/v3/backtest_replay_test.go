package v3

import (
	"fmt"
	"math"
	"testing"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

// deterministicStrategy is a simple buy-at-tick strategy for replay determinism tests.
type deterministicStrategy struct {
	name    string
	counter int
	modulo  int // buy every N ticks
}

func (s *deterministicStrategy) Name() string { return s.name }
func (s *deterministicStrategy) OnTick(tick marketdata.Tick) []strategy.Signal {
	s.counter++
	if s.counter%s.modulo == 0 {
		return []strategy.Signal{{
			Symbol:        tick.Symbol,
			Action:        strategy.ActionBuy,
			TargetSize:    0.01,
			StopLossPct:   0.35,
			TakeProfitPct: 0.70,
			Confidence:    0.90,
		}}
	}
	return nil
}
func (s *deterministicStrategy) OnCandle(tick marketdata.Tick) []strategy.Signal { return nil }

// generateTicks produces N synthetic price ticks.
func generateTicks(n int, startPrice float64) []marketdata.Tick {
	ticks := make([]marketdata.Tick, n)
	price := startPrice
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	rng := newSimpleRNG(12345)
	for i := range ticks {
		delta := rng.NormFloat64() * 50
		price += delta
		if price < 1000 {
			price = 1000
		}
		ticks[i] = marketdata.Tick{
			Symbol:   "BTCUSD",
			Price:    price,
			Quantity: 0.5 + rng.Float64()*2,
			TimeMs:   base.Add(time.Duration(i)*time.Minute).UnixMilli(),
		}
	}
	return ticks
}

func TestReplay_100K_Orders_Deterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100k replay test in short mode")
	}
	ticks := generateTicks(100_000, 50000)
	runAndVerifyDeterminism(t, ticks, "100k")
}

func TestReplay_1M_Orders_Deterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1M replay test in short mode")
	}
	ticks := generateTicks(1_000_000, 50000)
	runAndVerifyDeterminism(t, ticks, "1M")
}

func runAndVerifyDeterminism(t *testing.T, ticks []marketdata.Tick, label string) {
	t.Helper()
	cfg := Config{
		Mode:              SimModeInstitutional,
		Exchange:          ExchangeBinance,
		InitialCapitalUSD: 1_000_000,
		RiskPerTradePct:   0.1,
		LatencyTier:       btExecution.Latency50ms,
		LatencyJitter:     false, // disable jitter for determinism
		WarmupBars:        10,
		AllowShorts:       true,
		MaxOpenPositions:  20,
		CommissionTier:    TierStandard,
		LiquidityScenario: ScenarioNormal,
		MonteCarloConfig:  MonteCarloV3Config{Paths: 100, Seed: 42},
		ValidationConfig:  ValidationConfig{MaxSimErrorPct: 10},
	}

	strat := &deterministicStrategy{name: "DET", modulo: 50}
	input := V3Input{Config: cfg, Ticks: ticks, Strategy: strat}

	engine1 := NewEngine(cfg, nil, nil)
	result1, err := engine1.Run(input)
	if err != nil {
		t.Fatalf("[%s] first run error: %v", label, err)
	}

	// Reset strategy counter for second run.
	strat2 := &deterministicStrategy{name: "DET", modulo: 50}
	input2 := V3Input{Config: cfg, Ticks: ticks, Strategy: strat2}
	engine2 := NewEngine(cfg, nil, nil)
	result2, err := engine2.Run(input2)
	if err != nil {
		t.Fatalf("[%s] second run error: %v", label, err)
	}

	// Verify determinism.
	if len(result1.Trades) != len(result2.Trades) {
		t.Fatalf("[%s] trade count differs: %d vs %d", label, len(result1.Trades), len(result2.Trades))
	}

	for i := range result1.Trades {
		t1 := result1.Trades[i]
		t2 := result2.Trades[i]
		if math.Abs(t1.NetPnL-t2.NetPnL) > 0.001 {
			t.Fatalf("[%s] trade[%d] NetPnL differs: %f vs %f", label, i, t1.NetPnL, t2.NetPnL)
		}
		if math.Abs(t1.EntryPrice-t2.EntryPrice) > 0.001 {
			t.Fatalf("[%s] trade[%d] EntryPrice differs: %f vs %f", label, i, t1.EntryPrice, t2.EntryPrice)
		}
	}

	// Final capital deterministic.
	if math.Abs(result1.FinalCapitalUSD-result2.FinalCapitalUSD) > 0.01 {
		t.Fatalf("[%s] FinalCapital differs: %f vs %f", label, result1.FinalCapitalUSD, result2.FinalCapitalUSD)
	}

	t.Logf("[%s] %d ticks → %d trades, deterministic replay verified", label, len(ticks), len(result1.Trades))
}

func TestReplay_Small_Sanity(t *testing.T) {
	ticks := generateTicks(5000, 50000)
	cfg := DefaultConfig()
	cfg.LatencyJitter = false
	cfg.WarmupBars = 20
	cfg.MonteCarloConfig = MonteCarloV3Config{Paths: 50, Seed: 7}
	strat := &deterministicStrategy{name: "S1", modulo: 100}
	engine := NewEngine(cfg, nil, nil)
	result, err := engine.Run(V3Input{Config: cfg, Ticks: ticks, Strategy: strat})
	if err != nil {
		t.Fatalf("engine error: %v", err)
	}
	t.Logf("trades=%d finalCapital=%.2f analytics.EQS=%.1f MC.Median=%.2f",
		len(result.Trades), result.FinalCapitalUSD,
		result.Analytics.ExecutionQualityScore,
		result.MonteCarloReport.Median)
}

func TestReplay_StressMode_DoesNotPanic(t *testing.T) {
	ticks := generateTicks(2000, 50000)
	cfg := DefaultConfig()
	cfg.Mode = SimModeStress
	cfg.LiquidityScenario = ScenarioFTX
	cfg.StressScenarios = []StressScenarioType{StressFlashCrash, StressFundingShock}
	cfg.LatencyJitter = false
	cfg.WarmupBars = 20
	cfg.MonteCarloConfig = MonteCarloV3Config{Paths: 50, Seed: 1}
	strat := &deterministicStrategy{name: "S2", modulo: 50}
	engine := NewEngine(cfg, nil, nil)
	_, err := engine.Run(V3Input{Config: cfg, Ticks: ticks, Strategy: strat})
	if err != nil {
		t.Fatalf("stress mode run error: %v", err)
	}
}

func TestMonteCarlo_V3_FatTails(t *testing.T) {
	trades := make([]V3Trade, 200)
	rng := newSimpleRNG(55)
	for i := range trades {
		pnl := rng.NormFloat64() * 1000
		trades[i] = V3Trade{NetPnL: pnl, EntryPrice: 50000, Size: 0.1}
	}
	report := RunMonteCarloV3(trades, MonteCarloV3Config{
		Paths:                    1000,
		Seed:                     42,
		UseStudentT:              true,
		DegreesOfFreedom:         4,
		BootstrapWithReplacement: true,
	})
	if report.Percentile5 >= report.Percentile95 {
		t.Fatal("5th pct must be less than 95th pct")
	}
	if report.SurvivalRate < 0 || report.SurvivalRate > 100 {
		t.Fatalf("survival rate out of range: %f", report.SurvivalRate)
	}
	if report.RiskOfRuin < 0 || report.RiskOfRuin > 100 {
		t.Fatalf("risk of ruin out of range: %f", report.RiskOfRuin)
	}
	t.Logf("MC1000: p5=%.0f median=%.0f p95=%.0f survival=%.1f%% ruin=%.1f%%",
		report.Percentile5, report.Median, report.Percentile95,
		report.SurvivalRate, report.RiskOfRuin)
}

func TestValidation_PassesWithZeroPairs(t *testing.T) {
	report, err := Validate(nil, ValidationConfig{MaxSimErrorPct: 10})
	if err != nil {
		t.Fatalf("empty validation should pass: %v", err)
	}
	if !report.PassesThreshold {
		t.Fatal("empty validation should pass threshold")
	}
}

func TestValidation_FailsWithHighError(t *testing.T) {
	pairs := []ValidationPair{
		{
			Historical: HistoricalFill{OrderID: "H1", FillPrice: 50000, SlippageBps: 5, SpreadBps: 3},
			Simulated:  SimulatedFill{OrderID: "H1", FillPrice: 60000, SlippageBps: 5, SpreadBps: 3}, // 20% error
		},
	}
	_, err := Validate(pairs, ValidationConfig{MaxSimErrorPct: 5})
	if err == nil {
		t.Fatal("validation with 20% fill price error should fail 5% threshold")
	}
}

func TestExecutionAnalytics_EQS_Range(t *testing.T) {
	trades := make([]V3Trade, 50)
	for i := range trades {
		trades[i] = V3Trade{
			GrossPnL:        100,
			SpreadCostUSD:   2,
			SlippageCostUSD: 1,
			ImpactCostUSD:   0.5,
			CommissionUSD:   4,
			FundingCostUSD:  0.5,
			LatencyCostUSD:  0.1,
			NetPnL:          92,
			FillRatio:       0.95,
			EntryPrice:      50000,
			Size:            0.01,
			SimulatedLatencyMs: 55,
			EntryFill: btExecution.Fill{ExpectedFill: 0.01, ActualFill: 0.0095},
		}
	}
	analytics := BuildExecutionAnalytics(trades)
	if analytics.ExecutionQualityScore < 0 || analytics.ExecutionQualityScore > 100 {
		t.Fatalf("EQS out of range: %f", analytics.ExecutionQualityScore)
	}
	t.Logf("EQS=%.1f fillRatio=%.3f spreadCost=%.2f slippageCost=%.2f",
		analytics.ExecutionQualityScore,
		analytics.AvgFillRatio,
		analytics.AvgSpreadBps,
		analytics.AvgSlippageBps)
}

// BenchmarkEngine measures throughput of the V3 engine on 10,000 ticks.
func BenchmarkEngine_10K(b *testing.B) {
	ticks := generateTicks(10_000, 50000)
	cfg := DefaultConfig()
	cfg.LatencyJitter = false
	cfg.WarmupBars = 10
	cfg.MonteCarloConfig = MonteCarloV3Config{Paths: 0}
	for n := 0; n < b.N; n++ {
		strat := &deterministicStrategy{name: fmt.Sprintf("BT%d", n), modulo: 50}
		engine := NewEngine(cfg, nil, nil)
		_, _ = engine.Run(V3Input{Config: cfg, Ticks: ticks, Strategy: strat})
	}
}

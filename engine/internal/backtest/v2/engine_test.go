package v2

import (
	"testing"
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type alwaysBuyStrategy struct{}

func (alwaysBuyStrategy) Name() string { return "test-always-buy" }
func (alwaysBuyStrategy) OnTick(t marketdata.Tick) []strategy.Signal {
	return []strategy.Signal{{Symbol: t.Symbol, Action: strategy.ActionBuy, TargetSize: 1, Confidence: 0.8, StopLossPct: 1, TakeProfitPct: 0.05}}
}
func (s alwaysBuyStrategy) OnCandle(t marketdata.Tick) []strategy.Signal { return s.OnTick(t) }

func TestBiasDetectorRejectsNonMonotonicData(t *testing.T) {
	report := BiasDetector{}.ValidateTicks([]marketdata.Tick{
		{Symbol: "BTCUSD", Price: 30_000, TimeMs: 2},
		{Symbol: "BTCUSD", Price: 30_100, TimeMs: 1},
	})
	if report.Passed {
		t.Fatal("expected bias detector to reject non-monotonic data")
	}
}

func TestEngineRunsRealisticBacktestAndWarmup(t *testing.T) {
	start := time.Unix(1_700_000_000, 0).UTC()
	ticks := make([]marketdata.Tick, 0, 12)
	for i := 0; i < 12; i++ {
		ticks = append(ticks, marketdata.Tick{
			Symbol:   "BTCUSD",
			Price:    30_000 + float64(i*50),
			Quantity: 20,
			TimeMs:   start.Add(time.Duration(i) * time.Minute).UnixMilli(),
		})
	}
	engine := NewEngine(nil)
	result, err := engine.Run(Input{
		Ticks:    ticks,
		Strategy: alwaysBuyStrategy{},
		Config: Config{
			Mode:              ModeFast,
			Kind:              SimulationTick,
			InitialCapitalUSD: 100_000,
			WarmupBars:        2,
			LatencyTier:       50_000_000,
		},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !result.BiasReport.Passed || !result.WarmupReport.Completed {
		t.Fatalf("expected clean bias and completed warmup, got %+v %+v", result.BiasReport, result.WarmupReport)
	}
	if len(result.Trades) == 0 {
		t.Fatal("expected at least one completed trade")
	}
	if result.ExecutionQuality.AverageFillRatio <= 0 {
		t.Fatalf("expected execution quality metrics, got %+v", result.ExecutionQuality)
	}
}

func TestPromotionRequiresInstitutionalEvidence(t *testing.T) {
	res := Result{Metrics: Metrics{TotalTrades: 10}}
	decision := DecidePromotion(res, WalkForwardReport{}, MonteCarloReport{}, RobustnessReport{}, BenchmarkReport{})
	if decision.Approved {
		t.Fatal("expected promotion rejection without minimum evidence")
	}
}

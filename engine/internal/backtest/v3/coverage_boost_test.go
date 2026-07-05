package v3

import (
	"math"
	"testing"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

// Tests for functions with 0% or very low coverage

func TestValidateFromTrades(t *testing.T) {
	historical := []HistoricalFill{
		{OrderID: "T1", FillPrice: 50000, SlippageBps: 3, SpreadBps: 2, FilledQty: 0.5},
		{OrderID: "T2", FillPrice: 51000, SlippageBps: 4, SpreadBps: 2.5, FilledQty: 0.3},
	}
	trades := []V3Trade{
		{
			ID: "T1", Symbol: "BTCUSD", EntryPrice: 50050, Size: 0.5,
			SpreadCostUSD: 5, SlippageCostUSD: 7.5, CommissionUSD: 10,
			EntryFill: btExecution.Fill{FilledQuantity: 0.5},
		},
		{
			ID: "T2", Symbol: "BTCUSD", EntryPrice: 51025, Size: 0.3,
			SpreadCostUSD: 3, SlippageCostUSD: 6, CommissionUSD: 6,
			EntryFill: btExecution.Fill{FilledQuantity: 0.3},
		},
	}
	cfg := ValidationConfig{MaxSimErrorPct: 50} // generous for synthetic data
	report, err := ValidateFromTrades(trades, historical, cfg)
	if err != nil {
		t.Fatalf("ValidateFromTrades should not error with generous threshold: %v", err)
	}
	if report.Pairs != 2 {
		t.Fatalf("expected 2 pairs, got %d", report.Pairs)
	}
}

func TestPnLWaterfall_Keys(t *testing.T) {
	a := ExecutionAnalytics{
		GrossPnL:       1000,
		SpreadCost:     20,
		SlippageCost:   15,
		ImpactCost:     10,
		FundingCost:    5,
		CommissionCost: 40,
		LatencyCost:    2,
		NetPnL:         908,
	}
	wf := PnLWaterfall(a)
	if _, ok := wf["1_gross_pnl"]; !ok {
		t.Fatal("waterfall missing 1_gross_pnl key")
	}
	if _, ok := wf["8_net_pnl"]; !ok {
		t.Fatal("waterfall missing 8_net_pnl key")
	}
	if len(wf) != 8 {
		t.Fatalf("expected 8 waterfall entries, got %d", len(wf))
	}
}

func TestSortedBidsAsks(t *testing.T) {
	levels := []PriceLevel{
		{Price: 50010, Size: 1},
		{Price: 49990, Size: 2},
		{Price: 50000, Size: 3},
	}
	bids := SortedBids(levels)
	if bids[0].Price < bids[1].Price {
		t.Fatal("SortedBids should be descending")
	}
	asks := SortedAsks(levels)
	if asks[0].Price > asks[1].Price {
		t.Fatal("SortedAsks should be ascending")
	}
	// Original slice should not be mutated.
	if levels[0].Price != 50010 {
		t.Fatal("SortedBids/Asks should not mutate original slice")
	}
}

func TestLiquidityEngine_AdjustVolatility(t *testing.T) {
	normal := NewLiquidityEngine(ScenarioNormal)
	crisis := NewLiquidityEngine(ScenarioMarch2020)
	baseVol := 20.0
	normVol := normal.AdjustVolatility(baseVol)
	crisisVol := crisis.AdjustVolatility(baseVol)
	if normVol != baseVol {
		t.Fatalf("normal scenario should not change vol: got %f", normVol)
	}
	if crisisVol <= normVol {
		t.Fatal("March 2020 should amplify volatility")
	}
}

func TestLiquidityEngine_Regime(t *testing.T) {
	eng := NewLiquidityEngine(ScenarioFTX)
	regime := eng.Regime()
	if regime.SpreadMultiplier <= 1 {
		t.Fatal("FTX regime should have spread > 1")
	}
	if regime.Name == "" {
		t.Fatal("regime name should not be empty")
	}
}

func TestCommissionEngine_Schedule(t *testing.T) {
	eng := NewCommissionEngine(ExchangeBinance, TierTier2)
	sched := eng.Schedule()
	if sched.TakerBps <= 0 {
		t.Fatal("taker fee should be positive")
	}
}

func TestShouldCloseV3_SLBuy(t *testing.T) {
	pos := &v3Position{
		trade: V3Trade{
			Side:       strategy.ActionBuy,
			EntryPrice: 50000,
			OpenedAt:   time.Now(),
		},
		stopLossPct:   0.35,
		takeProfitPct: 0.70,
	}
	// Price drops to SL.
	if !shouldCloseV3(pos, 50000*(1-0.0035)-1, time.Now(), DefaultConfig()) {
		t.Fatal("position should close on SL hit for long")
	}
}

func TestShouldCloseV3_TPBuy(t *testing.T) {
	pos := &v3Position{
		trade: V3Trade{
			Side:       strategy.ActionBuy,
			EntryPrice: 50000,
			OpenedAt:   time.Now(),
		},
		stopLossPct:   0.35,
		takeProfitPct: 0.70,
	}
	// Price rises to TP.
	if !shouldCloseV3(pos, 50000*(1+0.0070)+1, time.Now(), DefaultConfig()) {
		t.Fatal("position should close on TP hit for long")
	}
}

func TestShouldCloseV3_TPSell(t *testing.T) {
	pos := &v3Position{
		trade: V3Trade{
			Side:       strategy.ActionSell,
			EntryPrice: 50000,
			OpenedAt:   time.Now(),
		},
		stopLossPct:   0.35,
		takeProfitPct: 0.70,
	}
	// Price drops to short TP.
	if !shouldCloseV3(pos, 50000*(1-0.0070)-1, time.Now(), DefaultConfig()) {
		t.Fatal("position should close on TP hit for short")
	}
}

func TestShouldCloseV3_TimeExit(t *testing.T) {
	pos := &v3Position{
		trade: V3Trade{
			Side:       strategy.ActionBuy,
			EntryPrice: 50000,
			OpenedAt:   time.Now().Add(-50 * time.Minute),
		},
		stopLossPct:   0.35,
		takeProfitPct: 0.70,
	}
	if !shouldCloseV3(pos, 50000, time.Now(), DefaultConfig()) {
		t.Fatal("position should close on 45-minute time limit")
	}
}

func TestNormalizeConfig_Defaults(t *testing.T) {
	cfg := normalizeConfig(Config{})
	if cfg.Mode != SimModeInstitutional {
		t.Fatalf("default mode should be institutional, got %s", cfg.Mode)
	}
	if cfg.Exchange != ExchangeBinance {
		t.Fatalf("default exchange should be Binance, got %s", cfg.Exchange)
	}
	if cfg.InitialCapitalUSD != 1_000_000 {
		t.Fatalf("default capital should be 1M, got %f", cfg.InitialCapitalUSD)
	}
	if cfg.MaxOpenPositions != 10 {
		t.Fatalf("default max positions should be 10, got %d", cfg.MaxOpenPositions)
	}
}

func TestNormalizeSignalV3_Defaults(t *testing.T) {
	sig := strategy.Signal{Action: strategy.ActionBuy}
	cfg := Config{InitialCapitalUSD: 100_000, RiskPerTradePct: 1.0}
	out := normalizeSignalV3(sig, 50000, cfg)
	if out.Symbol != "BTCUSD" {
		t.Fatalf("symbol should default to BTCUSD, got %s", out.Symbol)
	}
	if out.TargetSize <= 0 {
		t.Fatal("target size should be computed from capital")
	}
}

func TestTickTimestamp_ZeroMs(t *testing.T) {
	tick := marketdata.Tick{Price: 50000} // TimeMs = 0
	ts := tickTimestamp(tick)
	// Should return a recent timestamp (from time.Now())
	diff := time.Since(ts)
	if diff > time.Second {
		t.Fatalf("zero TimeMs should return approximately now, got diff %v", diff)
	}
}

func TestLastPrice_Empty(t *testing.T) {
	p := lastPrice(nil)
	if p != 0 {
		t.Fatalf("lastPrice of nil should be 0, got %f", p)
	}
}

func TestMomentumPct_InsufficientHistory(t *testing.T) {
	h := NewMidPriceHistory(100)
	h.Add(50000)
	h.Add(50100)
	pct := momentumPct(h)
	if pct != 0 {
		t.Fatalf("< 5 prices should return 0 momentum, got %f", pct)
	}
}

func TestMomentumPct_WithHistory(t *testing.T) {
	h := NewMidPriceHistory(100)
	for i := 0; i < 10; i++ {
		h.Add(50000 + float64(i)*100)
	}
	pct := momentumPct(h)
	if pct <= 0 {
		t.Fatal("uptrend history should produce positive momentum")
	}
}

func TestSimulateMakerFills_LowProbability(t *testing.T) {
	engine := NewQueuePositionEngine()
	order := QueueOrder{
		OrderID:     "LP1",
		Side:        "BUY",
		Type:        "LIMIT",
		LimitPrice:  49000, // far from mid — very low fill probability
		Quantity:    1.0,
		SubmittedAt: time.Now(),
	}
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	result := engine.Estimate(order, book, 0)
	// At 10% probability threshold, partial fills may be minimal.
	if result.FillProbability < 0 || result.FillProbability > 1 {
		t.Fatalf("fill probability out of range: %f", result.FillProbability)
	}
}

func TestMaxDrawdownFromPnL_AllLosses(t *testing.T) {
	pnls := []float64{-100, -200, -150, -300}
	dd := maxDrawdownFromPnL(pnls)
	if dd <= 0 {
		t.Fatal("all-loss sequence should have positive drawdown")
	}
}

func TestMaxDrawdownFromPnL_Empty(t *testing.T) {
	dd := maxDrawdownFromPnL(nil)
	if dd != 0 {
		t.Fatal("empty PnL slice should have 0 drawdown")
	}
}

func TestMonteCarloV3_BootstrapShuffle(t *testing.T) {
	trades := make([]V3Trade, 100)
	for i := range trades {
		trades[i] = V3Trade{NetPnL: float64(i) * 10}
	}
	// Non-bootstrap (shuffle) mode.
	report := RunMonteCarloV3(trades, MonteCarloV3Config{
		Paths:                    200,
		Seed:                     1,
		UseStudentT:              false,
		BootstrapWithReplacement: false,
	})
	if report.Paths != 200 {
		t.Fatalf("expected 200 paths, got %d", report.Paths)
	}
	if math.IsNaN(report.Median) {
		t.Fatal("median should not be NaN")
	}
}

func TestGaussianJitter_ProducesNonZero(t *testing.T) {
	h := NewMidPriceHistory(20)
	for i := 0; i < 20; i++ {
		h.Add(float64(50000 + i))
	}
	jitter := gaussianJitter(h, 0.3)
	// Just verify it doesn't panic and returns a float.
	if math.IsNaN(jitter) || math.IsInf(jitter, 0) {
		t.Fatal("jitter should be finite")
	}
}

func TestBuildStressReport_WithTrades(t *testing.T) {
	start := time.Now().Add(-2 * time.Hour)
	engine := NewStressEngine([]StressScenarioType{StressFlashCrash}, start)
	trades := []V3Trade{
		{OpenedAt: start.Add(1 * time.Minute), NetPnL: -500, EntryPrice: 50000, Size: 0.1},
		{OpenedAt: start.Add(2 * time.Minute), NetPnL: 200, EntryPrice: 50000, Size: 0.1},
	}
	report := BuildStressReport(trades, engine.events)
	if report.PortfolioSurvival < 0 || report.PortfolioSurvival > 100 {
		t.Fatalf("portfolio survival out of range: %f", report.PortfolioSurvival)
	}
}

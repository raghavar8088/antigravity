package phase22e

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// ── Test data builders ────────────────────────────────────────────────────────

func makeTradeRecord(id, stratID, stratName, family string, pnl float64, entryT, exitT time.Time) TradeRecord {
	entryPrice := 65000.0
	side := "LONG"
	if pnl < 0 {
		side = "SHORT"
	}
	exitPrice := entryPrice + pnl/0.01
	return TradeRecord{
		TradeID:      id,
		StrategyID:   stratID,
		StrategyName: stratName,
		Family:       family,
		Symbol:       "BTC-USD",
		Side:         side,
		EntryPrice:   entryPrice,
		ExitPrice:    exitPrice,
		Quantity:     0.01,
		GrossPnLUSD:  pnl + pnl*0.001,
		NetPnLUSD:    pnl,
		FeesUSD:      math.Abs(pnl) * 0.001,
		HoldMinutes:  30,
		EntryTime:    entryT,
		ExitTime:     exitT,
		IsLive:       false,
	}
}

func syntheticTrades(n int, winRate float64, avgWin, avgLoss float64, stratID, stratName, family string) []TradeRecord {
	trades := make([]TradeRecord, n)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		pnl := -avgLoss
		if float64(i)/float64(n) < winRate {
			pnl = avgWin
		}
		entry := base.Add(time.Duration(i) * 4 * time.Hour)
		exit := entry.Add(30 * time.Minute)
		trades[i] = makeTradeRecord(
			fmt.Sprintf("%s-%d", stratID, i),
			stratID, stratName, family,
			pnl, entry, exit,
		)
	}
	return trades
}

// ── Unit tests ────────────────────────────────────────────────────────────────

func TestMean(t *testing.T) {
	got := mean([]float64{1, 2, 3, 4, 5})
	if got != 3.0 {
		t.Errorf("mean = %.2f, want 3.0", got)
	}
	if mean(nil) != 0 {
		t.Error("mean(nil) should be 0")
	}
}

func TestStddev(t *testing.T) {
	xs := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	got := stddev(xs)
	// sample std (n-1): sqrt(32/7) ≈ 2.138
	if math.Abs(got-2.138) > 0.01 {
		t.Errorf("stddev = %.4f, want ~2.138", got)
	}
}

func TestMaxDrawdown_flat(t *testing.T) {
	dd := maxDrawdownPct([]float64{1, 1, 1, 1})
	if dd != 0 {
		t.Errorf("flat series drawdown should be 0, got %.4f", dd)
	}
}

func TestMaxDrawdown_decline(t *testing.T) {
	pnls := []float64{100, -50}
	dd := maxDrawdownPct(pnls)
	if math.Abs(dd-50.0) > 0.01 {
		t.Errorf("drawdown = %.2f, want 50.0", dd)
	}
}

func TestLongestConsecPositive(t *testing.T) {
	months := []MonthlyReturn{
		{Positive: true}, {Positive: true}, {Positive: false},
		{Positive: true}, {Positive: true}, {Positive: true},
	}
	got := longestConsecPositive(months)
	if got != 3 {
		t.Errorf("consec = %d, want 3", got)
	}
}

func TestVarCVar(t *testing.T) {
	// Mix of losses and profits: losses are negative values.
	xs := make([]float64, 100)
	for i := range xs {
		xs[i] = float64(i+1) - 50 // range -49..+50, so bottom 5% are losses
	}
	varVal, cvarVal := varCVar(xs, 0.05)
	// worst 5% of xs are the smallest values (-49..-47); VaR = -(-49) = 49
	if varVal <= 0 {
		t.Errorf("VaR should be positive loss (bottom 5%% are losses), got %.2f", varVal)
	}
	if cvarVal <= 0 {
		t.Errorf("CVaR should be positive, got %.2f", cvarVal)
	}
}

func TestComputePortfolioMetrics_empty(t *testing.T) {
	pm := computePortfolioMetrics(nil)
	if pm.TotalTrades != 0 {
		t.Error("empty trades should give TotalTrades=0")
	}
}

func TestComputePortfolioMetrics_basic(t *testing.T) {
	trades := []TradeRecord{
		{NetPnLUSD: 100, ExitTime: time.Now(), Regime: RegimeBull},
		{NetPnLUSD: 50, ExitTime: time.Now(), Regime: RegimeBull},
		{NetPnLUSD: -30, ExitTime: time.Now(), Regime: RegimeBear},
		{NetPnLUSD: -20, ExitTime: time.Now(), Regime: RegimeRange},
	}
	pm := computePortfolioMetrics(trades)
	if pm.TotalTrades != 4 {
		t.Errorf("TotalTrades = %d, want 4", pm.TotalTrades)
	}
	if math.Abs(pm.WinRate-0.5) > 0.01 {
		t.Errorf("WinRate = %.2f, want 0.50", pm.WinRate)
	}
	// GrossWin=150, GrossLoss=50, PF=3.0
	if math.Abs(pm.ProfitFactor-3.0) > 0.01 {
		t.Errorf("ProfitFactor = %.2f, want 3.0", pm.ProfitFactor)
	}
}

func TestKellyFraction(t *testing.T) {
	// 60% WR, avg win $200, avg loss $100 → Kelly = 0.6 - 0.4*(100/200) = 0.4
	winRate := 0.60
	avgWin := 200.0
	avgLoss := 100.0
	kelly := winRate - (1-winRate)*(avgLoss/avgWin)
	if math.Abs(kelly-0.4) > 0.001 {
		t.Errorf("kelly = %.4f, want 0.4", kelly)
	}
}

func TestRegimeClassify(t *testing.T) {
	reg := ClassifyRegimeSimple([]float64{0.001, -0.001, 0.002, 0.0, -0.001})
	if reg != RegimeRange {
		t.Errorf("expected RANGE, got %s", reg)
	}
	reg = ClassifyRegimeSimple([]float64{0.02, 0.02, 0.02, 0.01, 0.01})
	if reg != RegimeBull {
		t.Errorf("expected BULL, got %s", reg)
	}
	reg = ClassifyRegimeSimple([]float64{0.05, -0.05, 0.05, -0.05, 0.05})
	if reg != RegimeVolatile {
		t.Errorf("expected VOLATILE, got %s", reg)
	}
}

func TestCertifyPassAll(t *testing.T) {
	result := buildPassingResult(1200)
	Certify(&result)
	if result.Status == StatusNotApproved {
		t.Errorf("should not be NOT_APPROVED with strong metrics; got %s, failed: %v", result.Status, result.Failed)
	}
}

func TestCertifyFailTrades(t *testing.T) {
	result := buildPassingResult(200)
	Certify(&result)
	if result.Status != StatusConditional && result.Status != StatusNotApproved {
		t.Errorf("expected conditional/not-approved with only 200 trades, got %s", result.Status)
	}
}

func TestRankStrategies(t *testing.T) {
	strategies := []StrategyMetrics{
		{
			StrategyID: "A", StrategyName: "Alpha", Family: "EMA",
			TotalTrades: 100, WinRate: 0.55, ProfitFactor: 1.50, Sharpe: 2.0,
			MaxDrawdown: 5.0, KellyFraction: 0.15, HalfKelly: 0.075,
			Expectancy: 25, LiveVsBacktestOK: true, StatSig: true,
		},
		{
			StrategyID: "B", StrategyName: "Beta", Family: "RSI",
			TotalTrades: 15, WinRate: 0.60, ProfitFactor: 1.80, Sharpe: 2.5,
			MaxDrawdown: 4.0, KellyFraction: 0.20, HalfKelly: 0.10,
			Expectancy: 30, LiveVsBacktestOK: true, StatSig: false,
		},
	}
	ranked := RankStrategies(strategies, 100_000)
	if ranked[0].Rank != 1 {
		t.Error("first element should have rank 1")
	}
	for _, s := range ranked {
		if s.StrategyID == "B" && s.Approved {
			t.Error("Beta should be rejected due to insufficient trades")
		}
	}
}

func TestValidatorRun_empty(t *testing.T) {
	v := NewValidator(100_000)
	result := v.Run(nil)
	if result.Status != StatusNotApproved {
		t.Errorf("empty trades should give NOT_APPROVED, got %s", result.Status)
	}
}

func TestValidatorRun_syntheticTrades(t *testing.T) {
	v := NewValidator(100_000)
	trades := syntheticTrades(100, 0.55, 120, 80, "strat-A", "EMA Cross 21/50", "EMA Cross")
	result := v.Run(trades)
	if result.Portfolio.TotalTrades != 100 {
		t.Errorf("expected 100 trades, got %d", result.Portfolio.TotalTrades)
	}
	if result.Portfolio.WinRate <= 0 {
		t.Error("expected non-zero win rate")
	}
}

// buildPassingResult synthesises a ValidationResult that should certify.
func buildPassingResult(tradeCount int) ValidationResult {
	pm := PortfolioMetrics{
		TotalTrades:          tradeCount,
		Checkpoint500:        tradeCount >= 500,
		Checkpoint1000:       tradeCount >= 1000,
		StatSig:              tradeCount >= 30,
		ProfitFactor:         1.45,
		WinRate:              0.52,
		Sharpe:               1.85,
		Expectancy:           18.5,
		MaxDrawdown:          6.2,
		ConsecPositiveMonths: 4,
		GrossWinUSD:          float64(tradeCount) * 60,
		GrossLossUSD:         float64(tradeCount) * 41,
		NetPnLUSD:            float64(tradeCount) * 18.5,
		AvgWinUSD:            115,
		AvgLossUSD:           79,
		OutlierWinPct:        32,
		OutlierLossPct:       28,
		MonthlyReturns: []MonthlyReturn{
			{Positive: true}, {Positive: true}, {Positive: true}, {Positive: true},
		},
		RegimePerf: map[Regime]*RegimePerformance{
			RegimeBull:     {Regime: RegimeBull, Trades: 300, WinRate: 0.56, ProfitFactor: 1.55},
			RegimeBear:     {Regime: RegimeBear, Trades: 200, WinRate: 0.48, ProfitFactor: 1.25},
			RegimeRange:    {Regime: RegimeRange, Trades: 400, WinRate: 0.50, ProfitFactor: 1.40},
			RegimeVolatile: {Regime: RegimeVolatile, Trades: 300, WinRate: 0.53, ProfitFactor: 1.35},
		},
	}
	return ValidationResult{
		GeneratedAt: time.Now().UTC(),
		Portfolio:   pm,
		Strategies:  []StrategyMetrics{},
	}
}

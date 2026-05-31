package risk

import (
	"strings"
	"testing"
)

func TestPortfolioRiskEngineBlocksPortfolioHeat(t *testing.T) {
	engine := NewPortfolioRiskEngine(DefaultRiskLimits(100000))
	engine.AddPosition("p1", RiskPosition{
		Symbol:         "BTC-USD",
		Strategy:       "EMA",
		StrategyFamily: "Trend",
		Exchange:       "paper",
		Side:           SideLong,
		EntryPrice:     100000,
		SizeBTC:        1,
		NotionalUSD:    100000,
		StopLossPrice:  94000,
		Leverage:       1,
	})
	engine.AddPosition("p2", RiskPosition{
		Symbol:         "BTC-USD",
		Strategy:       "MACD",
		StrategyFamily: "Trend",
		Exchange:       "paper",
		Side:           SideLong,
		EntryPrice:     100000,
		SizeBTC:        1,
		NotionalUSD:    100000,
		StopLossPrice:  94000,
		Leverage:       1,
	})

	decision := engine.ValidateTrade(RiskOrder{
		Symbol:         "BTC-USD",
		Strategy:       "ADX",
		StrategyFamily: "Trend",
		Exchange:       "paper",
		Side:           SideLong,
		EntryPrice:     100000,
		SizeBTC:        0.1,
		NotionalUSD:    10000,
		StopLossPrice:  99000,
		Leverage:       1,
		Regime:         RegimeTrendingBull,
	})
	if decision.Approved {
		t.Fatal("expected portfolio heat to block the trade")
	}
	if !strings.Contains(decision.Reason, "portfolio heat") {
		t.Fatalf("expected heat rejection, got %q", decision.Reason)
	}
}

func TestPortfolioRiskEngineApprovesControlledTrade(t *testing.T) {
	limits := DefaultRiskLimits(100000)
	limits.MaxFamilyAllocationPct = 100
	engine := NewPortfolioRiskEngine(limits)

	decision := engine.ValidateTrade(RiskOrder{
		Symbol:         "BTC-USD",
		Strategy:       "BreakoutA",
		StrategyFamily: "Breakout",
		Exchange:       "paper",
		Side:           SideLong,
		EntryPrice:     100000,
		SizeBTC:        0.05,
		NotionalUSD:    5000,
		StopLossPrice:  99000,
		Leverage:       1,
		Regime:         RegimeTrendingBull,
	})
	if !decision.Approved {
		t.Fatalf("expected controlled trade approval, got %q", decision.Reason)
	}
	if decision.RecommendedLeverage <= 0 {
		t.Fatal("expected leverage recommendation")
	}
}

func TestCorrelationGuardBlocksSameDirectionBTCStack(t *testing.T) {
	positions := map[string]RiskPosition{
		"a": {Symbol: "BTC-USD", Side: SideLong},
		"b": {Symbol: "BTC-USD", Side: SideLong},
		"c": {Symbol: "BTC-USD", Side: SideLong},
		"d": {Symbol: "BTC-USD", Side: SideLong},
	}
	reason := CorrelationGuardReason(RiskOrder{Symbol: "BTC-USD", Side: SideLong}, positions, nil, 0.85)
	if reason == "" {
		t.Fatal("expected same-direction BTC stack to be blocked")
	}
}

func TestVaRCVaRKellyAndDrawdownModels(t *testing.T) {
	returns := []float64{0.01, -0.02, 0.015, -0.05, 0.005, -0.01}
	if got := HistoricalVaR(returns, 100000, 0.95); got <= 0 {
		t.Fatalf("expected positive VaR, got %.2f", got)
	}
	if got := HistoricalCVaR(returns, 100000, 0.95); got <= 0 {
		t.Fatalf("expected positive CVaR, got %.2f", got)
	}
	k := KellySizing(TradeStats{WinRate: 0.55, ProfitFactor: 1.4, AverageWin: 120, AverageLoss: -80})
	if k.CappedKelly > 0.25 {
		t.Fatalf("expected capped Kelly <= 25%%, got %.2f", k.CappedKelly)
	}
	if got := DrawdownSizingMultiplier(6); got != 0.50 {
		t.Fatalf("expected 50%% sizing at 6%% drawdown, got %.2f", got)
	}
}

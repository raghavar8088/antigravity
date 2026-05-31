package v2

import "testing"

func TestKellySizeCapsAtTwoPercentAndAvoidsFullKelly(t *testing.T) {
	req := TradeRequest{EntryPrice: 100_000, StopLossPrice: 99_000, RequestedSizeBTC: 10, Side: SideLong}
	metrics := StrategyMetrics{WinRate: 0.58, AverageWinUSD: 150, AverageLossUSD: -100, ProfitFactor: 1.5, Sharpe: 1.5, OOSProfitFactor: 1.3, TotalTrades: 80}
	decision := KellySize(req, metrics, 100_000, 2)
	if decision.FullKellyFraction > 0.02 {
		t.Fatalf("full Kelly exceeded 2%% cap: %.4f", decision.FullKellyFraction)
	}
	if decision.SelectedFraction >= decision.FullKellyFraction && decision.FullKellyFraction > 0 {
		t.Fatalf("selected Kelly should be fractional, got selected %.4f full %.4f", decision.SelectedFraction, decision.FullKellyFraction)
	}
	if decision.RecommendedRiskUSD > 2_000 {
		t.Fatalf("risk exceeded 2%% account cap: %.2f", decision.RecommendedRiskUSD)
	}
}

func TestRiskV2BlocksPortfolioHeatBreach(t *testing.T) {
	engine := NewEngine(100_000)
	engine.AddPosition(Position{ID: "p1", Symbol: "BTCUSD", Strategy: "trend-a", Family: FamilyTrend, Side: SideLong, EntryPrice: 100_000, MarkPrice: 100_000, SizeBTC: 1, StopLossPrice: 94_000})
	decision := engine.ValidateTrade(
		TradeRequest{Symbol: "BTCUSD", Strategy: "trend-b", Family: FamilyTrend, Side: SideLong, EntryPrice: 100_000, StopLossPrice: 98_000, RequestedSizeBTC: 1, RequestedLeverage: 1},
		MarketState{Regime: RegimeTrendingBull, LiquidityScore: 0.8},
		StrategyMetrics{Strategy: "trend-b", Family: FamilyTrend, WinRate: 0.55, ProfitFactor: 1.4, Sharpe: 1.4, OOSProfitFactor: 1.2, OOSExpectancyUSD: 10, HealthScore: 80, TotalTrades: 80, AverageWinUSD: 200, AverageLossUSD: -100},
	)
	if decision.Approved {
		t.Fatalf("expected heat breach to block trade, got %+v", decision.Heat)
	}
}

func TestTailRiskHaltBlocksApproval(t *testing.T) {
	engine := NewEngine(100_000)
	decision := engine.ValidateTrade(
		TradeRequest{Symbol: "BTCUSD", Strategy: "orderflow", Family: FamilyOrderFlow, Side: SideLong, EntryPrice: 100_000, StopLossPrice: 99_000, RequestedSizeBTC: 0.1},
		MarketState{Regime: RegimeHighVol, VolatilityPct: 10, LiquidityScore: 0.05, BTCMovePct1m: -5, LiquidationImbalance: 4, FundingRate: 0.002},
		StrategyMetrics{Strategy: "orderflow", Family: FamilyOrderFlow, WinRate: 0.6, ProfitFactor: 1.8, Sharpe: 2, OOSProfitFactor: 1.5, OOSExpectancyUSD: 20, HealthScore: 90, TotalTrades: 100, AverageWinUSD: 200, AverageLossUSD: -100},
	)
	if decision.Approved || decision.TailRisk.Action != TailActionHalt {
		t.Fatalf("expected tail-risk halt, got approved=%v action=%s", decision.Approved, decision.TailRisk.Action)
	}
}

func TestRiskPromotionRequiresOOSAndRiskScore(t *testing.T) {
	decision := RiskDecision{RiskScore: 80}
	metrics := StrategyMetrics{OOSProfitFactor: 1.3, OOSExpectancyUSD: 10, Sharpe: 1.4, MaxDrawdownPct: 5}
	promo := DecideRiskPromotion(metrics, decision)
	if !promo.Approved {
		t.Fatalf("expected promotion approval, got %+v", promo.Reasons)
	}
	metrics.Sharpe = 0.8
	promo = DecideRiskPromotion(metrics, decision)
	if promo.Approved {
		t.Fatal("expected low Sharpe to reject promotion")
	}
}

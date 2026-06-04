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

// ── DynamicSize bug-fix tests ────────────────────────────────────────────────

func TestDynamicSize_ZeroPFGetsReduction(t *testing.T) {
	// Bug fix: PF=0 (all losses) previously escaped the < 1.2 check because of
	// the "> 0" guard. Now uses TotalTrades > 0 guard.
	req := TradeRequest{EntryPrice: 100_000, StopLossPrice: 99_820, RequestedSizeBTC: 0.1, Side: SideLong}
	metrics := StrategyMetrics{
		TotalTrades:  10,
		WinRate:      0,
		ProfitFactor: 0, // all losses
		HealthScore:  0, // catastrophic — previously escaped reduction too
		Sharpe:       -2.0,
	}
	market := MarketState{Regime: RegimeUnknown, LiquidityScore: 0.65}
	heat := HeatSnapshot{SizeMultiplier: 1}
	dd := DrawdownDecision{SizeMultiplier: 1}
	corr := CorrelationReport{}
	tail := TailRiskDecision{Action: TailActionNormal}

	result := DynamicSize(req, metrics, market, heat, dd, corr, tail)

	if result.RecommendedSizeBTC >= 0.1 {
		t.Fatalf("expected reduction for PF=0/Health=0 strategy, got %.4f BTC (no reduction)", result.RecommendedSizeBTC)
	}
	if result.Multiplier >= 1.0 {
		t.Fatalf("expected combined multiplier < 1.0 for all-losses strategy, got %.3f", result.Multiplier)
	}
}

func TestDynamicSize_NegativeSharpeGetsReduction(t *testing.T) {
	// Bug fix: negative Sharpe previously escaped the reduction because of "> 0" guard.
	req := TradeRequest{EntryPrice: 100_000, StopLossPrice: 99_820, RequestedSizeBTC: 0.1, Side: SideLong}
	metrics := StrategyMetrics{
		TotalTrades:  20,
		ProfitFactor: 1.5, // good PF but consistent loser
		HealthScore:  60,
		Sharpe:       -1.5, // negative — persistent losing pattern
	}
	market := MarketState{Regime: RegimeUnknown, LiquidityScore: 0.65}
	heat := HeatSnapshot{SizeMultiplier: 1}
	dd := DrawdownDecision{SizeMultiplier: 1}
	corr := CorrelationReport{}
	tail := TailRiskDecision{Action: TailActionNormal}

	result := DynamicSize(req, metrics, market, heat, dd, corr, tail)

	if result.RecommendedSizeBTC >= 0.1 {
		t.Fatalf("expected Sharpe reduction for Sharpe=-1.5, got no reduction (%.4f BTC)", result.RecommendedSizeBTC)
	}
}

func TestDynamicSize_ColdStartNoReductionForHealth(t *testing.T) {
	// Cold-start strategies (TotalTrades=0) must NOT receive health/PF/Sharpe reductions.
	req := TradeRequest{EntryPrice: 100_000, StopLossPrice: 99_820, RequestedSizeBTC: 0.1, Side: SideLong}
	metrics := StrategyMetrics{
		TotalTrades:  0,
		ProfitFactor: 1.0,
		HealthScore:  50,
		Sharpe:       0,
	}
	market := MarketState{Regime: RegimeUnknown, LiquidityScore: 0.65}
	heat := HeatSnapshot{SizeMultiplier: 1}
	dd := DrawdownDecision{SizeMultiplier: 1}
	corr := CorrelationReport{}
	tail := TailRiskDecision{Action: TailActionNormal}

	result := DynamicSize(req, metrics, market, heat, dd, corr, tail)

	if result.RecommendedSizeBTC != 0.1 {
		t.Fatalf("cold-start strategy should not be penalised by DynamicSize, got %.4f BTC", result.RecommendedSizeBTC)
	}
}

func TestKellySize_SelectedModeMatchesFraction(t *testing.T) {
	// Bug fix: SelectedMode was always KellyHalf even when quarter was selected.
	req := TradeRequest{EntryPrice: 100_000, StopLossPrice: 99_820, RequestedSizeBTC: 10, Side: SideLong}

	// Low TotalTrades → stability < 0.45 → should select quarter
	lowStability := StrategyMetrics{
		WinRate: 0.55, AverageWinUSD: 150, AverageLossUSD: -100,
		ProfitFactor: 1.4, Sharpe: 1.2, OOSProfitFactor: 1.2,
		TotalTrades: 20, // 20/100=0.20; 0.20×(1.4/1.5)=0.187 < 0.45
	}
	d := KellySize(req, lowStability, 1_000_000, 2)
	if d.SelectedMode != KellyQuarter {
		t.Fatalf("expected KellyQuarter for low-stability strategy, got %s (stability check failed)", d.SelectedMode)
	}
	if d.SelectedFraction != d.QuarterKellyFraction {
		t.Fatalf("SelectedFraction %.4f != QuarterKellyFraction %.4f", d.SelectedFraction, d.QuarterKellyFraction)
	}

	// High TotalTrades, good PF → stability ≥ 0.45 → should select half
	highStability := StrategyMetrics{
		WinRate: 0.55, AverageWinUSD: 150, AverageLossUSD: -100,
		ProfitFactor: 1.8, Sharpe: 1.5, OOSProfitFactor: 1.5,
		TotalTrades: 100, // 1.0 × (1.8/1.5)=1.0 → 1.0 ≥ 0.45
	}
	d2 := KellySize(req, highStability, 1_000_000, 2)
	if d2.SelectedMode != KellyHalf {
		t.Fatalf("expected KellyHalf for high-stability strategy, got %s", d2.SelectedMode)
	}
	if d2.SelectedFraction != d2.HalfKellyFraction {
		t.Fatalf("SelectedFraction %.4f != HalfKellyFraction %.4f", d2.SelectedFraction, d2.HalfKellyFraction)
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

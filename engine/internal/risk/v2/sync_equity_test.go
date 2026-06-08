package v2

import "testing"

func TestSyncEquity_UpdatesKellyInput(t *testing.T) {
	engine := NewEngine(10_000)
	req := TradeRequest{
		Symbol:           "BTC-USD",
		Strategy:         "trend-a",
		Family:           FamilyTrend,
		Side:             SideLong,
		EntryPrice:       50_000,
		StopLossPrice:    49_500,
		RequestedSizeBTC: 0.05,
	}
	metrics := StrategyMetrics{
		Strategy:         "trend-a",
		Family:           FamilyTrend,
		WinRate:          0.58,
		ProfitFactor:     1.5,
		Sharpe:           1.4,
		OOSProfitFactor:  1.3,
		AverageWinUSD:    150,
		AverageLossUSD:   -100,
		TotalTrades:      80,
	}

	before := engine.ValidateTrade(req, MarketState{Regime: RegimeTrendingBull, LiquidityScore: 0.8}, metrics)
	engine.SyncEquity(250_000)
	after := engine.ValidateTrade(req, MarketState{Regime: RegimeTrendingBull, LiquidityScore: 0.8}, metrics)

	if before.Kelly.RecommendedRiskUSD >= after.Kelly.RecommendedRiskUSD {
		t.Fatalf("expected higher Kelly risk after equity sync: before=%.2f after=%.2f",
			before.Kelly.RecommendedRiskUSD, after.Kelly.RecommendedRiskUSD)
	}
	if after.Kelly.RecommendedRiskUSD <= 10_000*before.Kelly.SelectedFraction*0.9 {
		t.Fatalf("Kelly risk should scale with live equity, got %.2f", after.Kelly.RecommendedRiskUSD)
	}
}

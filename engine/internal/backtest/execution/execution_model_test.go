package execution

import (
	"testing"
	"time"
)

func TestFillExecutesAgainstBidAskWithCosts(t *testing.T) {
	model := NewFillModel()
	now := time.Unix(1_700_000_000, 0).UTC()
	fill := model.Execute(Order{
		ID:          "o1",
		Symbol:      "BTCUSD",
		Side:        SideBuy,
		Type:        OrderMarket,
		Quantity:    1,
		SignalPrice: 30_000,
		GeneratedAt: now,
	}, MarketContext{
		MidPrice:       30_000,
		VolatilityPct:  1,
		LiquidityScore: 0.8,
		ADVNotionalUSD: 30_000_000,
		BookDepthUSD:   10_000_000,
		MomentumPct:    0.1,
		Timestamp:      now,
		Latency:        LatencyInput{Tier: Latency50ms, SignalEdgeBps: 5},
	})
	if fill.FillPrice <= 30_000 {
		t.Fatalf("expected buy fill above midpoint, got %.2f", fill.FillPrice)
	}
	if fill.SlippageCostUSD <= 0 || fill.ImpactCostUSD <= 0 || fill.SpreadCostUSD <= 0 {
		t.Fatalf("expected positive execution costs, got spread %.4f slip %.4f impact %.4f", fill.SpreadCostUSD, fill.SlippageCostUSD, fill.ImpactCostUSD)
	}
	if fill.FillRatio <= 0 || fill.FillRatio > 1 {
		t.Fatalf("invalid fill ratio %.2f", fill.FillRatio)
	}
}

func TestFundingModelLongPaysPositiveFunding(t *testing.T) {
	start := time.Unix(1_700_000_000, 0).UTC()
	funding := NewFundingModel([]FundingRate{{Rate: 0.0001, Timestamp: start.Add(8 * time.Hour)}})
	attr := funding.Apply(SideBuy, 100_000, start, start.Add(9*time.Hour))
	if attr.FundingPaidUSD <= 0 || attr.FundingPnLUSD >= 0 {
		t.Fatalf("expected long to pay positive funding, got %+v", attr)
	}
}

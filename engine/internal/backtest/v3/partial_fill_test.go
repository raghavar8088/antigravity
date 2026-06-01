package v3

import (
	"testing"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
)

func TestFillRatio_MarketOrder_HighLiquidity(t *testing.T) {
	order := btExecution.Order{Type: btExecution.OrderMarket, Side: btExecution.SideBuy, Quantity: 1.0}
	ctx := btExecution.MarketContext{LiquidityScore: 0.9}
	ratio := btExecution.FillRatio(order, ctx)
	if ratio < 0.9 {
		t.Fatalf("high-liquidity market order should fill > 90%%, got %f", ratio)
	}
}

func TestFillRatio_MarketOrder_LowLiquidity(t *testing.T) {
	order := btExecution.Order{Type: btExecution.OrderMarket, Side: btExecution.SideBuy, Quantity: 1.0}
	ctx := btExecution.MarketContext{LiquidityScore: 0.1}
	ratio := btExecution.FillRatio(order, ctx)
	if ratio >= 0.5 {
		t.Fatalf("low-liquidity market order should fill < 50%%, got %f", ratio)
	}
}

func TestFillRatio_PostOnly_AdverseMomentum(t *testing.T) {
	order := btExecution.Order{Type: btExecution.OrderPostOnly, Side: btExecution.SideBuy, Quantity: 1.0, PostOnly: true}
	ctx := btExecution.MarketContext{MomentumPct: 0.05, LiquidityScore: 0.8}
	ratio := btExecution.FillRatio(order, ctx)
	if ratio > 0.6 {
		t.Fatalf("post-only into adverse momentum should fill ≤ 50%%, got %f", ratio)
	}
}

func TestFillRatio_Limit_InTheMoney(t *testing.T) {
	order := btExecution.Order{
		Type:       btExecution.OrderLimit,
		Side:       btExecution.SideBuy,
		Quantity:   1.0,
		LimitPrice: 50100, // above mid → ITM
	}
	ctx := btExecution.MarketContext{MidPrice: 50000, LiquidityScore: 0.8}
	ratio := btExecution.FillRatio(order, ctx)
	if ratio != 1.0 {
		t.Fatalf("in-the-money limit buy should fill 100%%, got %f", ratio)
	}
}

func TestPartialFillEvents_RemainingDecreases(t *testing.T) {
	engine := NewQueuePositionEngine()
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	order := QueueOrder{
		OrderID:     "PF1",
		Side:        "BUY",
		Type:        "LIMIT",
		LimitPrice:  book.BestBid().Price,
		Quantity:    3.0,
		SubmittedAt: time.Now(),
	}
	result := engine.Estimate(order, book, 0)
	remaining := order.Quantity
	for _, ev := range result.PartialFills {
		if ev.RemainingQuantity > remaining {
			t.Fatal("remaining quantity should be non-increasing across partial fills")
		}
		remaining = ev.RemainingQuantity
	}
}

func TestFillModel_PartialFill_ReducesFilledQty(t *testing.T) {
	model := btExecution.NewFillModel()
	order := btExecution.Order{
		ID:          "PF2",
		Symbol:      "BTCUSD",
		Side:        btExecution.SideBuy,
		Type:        btExecution.OrderMarket,
		Quantity:    10.0,
		SignalPrice: 50000,
		GeneratedAt: time.Now(),
	}
	// Very illiquid — forces partial fill.
	ctx := btExecution.MarketContext{
		MidPrice:       50000,
		LiquidityScore: 0.1,
		ADVNotionalUSD: 100_000,
		BookDepthUSD:   50_000,
		Timestamp:      time.Now(),
		Latency:        btExecution.LatencyInput{Tier: btExecution.Latency50ms},
	}
	fill := model.Execute(order, ctx)
	if fill.FilledQuantity >= order.Quantity {
		t.Fatalf("illiquid fill should be partial: requested %f filled %f", order.Quantity, fill.FilledQuantity)
	}
	if fill.FillRatio >= 1.0 {
		t.Fatalf("fill ratio should be < 1.0 for partial fill, got %f", fill.FillRatio)
	}
}

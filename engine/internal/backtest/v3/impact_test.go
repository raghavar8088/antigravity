package v3

import (
	"testing"
	"time"
)

func TestBookWalk_ImpactScalesWithOrderSize(t *testing.T) {
	// Use low liquidity so each level holds ~20 BTC, forcing large orders to walk multiple levels.
	book := NewOrderBook(50000, 0.20, 0.10, time.Now())
	bestAskSize := book.BestAsk().Size // ~20 BTC at low liquidity

	// Small order fills inside level 0.
	small := book.WalkAsks(bestAskSize * 0.01)
	// Large order walks level 0 + multiple additional levels.
	book2 := NewOrderBook(50000, 0.20, 0.10, time.Now())
	large := book2.WalkAsks(bestAskSize * 3)

	if large.LevelsConsumed <= small.LevelsConsumed {
		t.Fatalf("large order should consume more levels (%d) than small (%d)",
			large.LevelsConsumed, small.LevelsConsumed)
	}
	if large.AverageFillPrice <= small.AverageFillPrice {
		t.Fatalf("large order avg fill (%f) should exceed small (%f) due to walking the book",
			large.AverageFillPrice, small.AverageFillPrice)
	}
}

func TestBookWalk_ImpactPositiveForBuy(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.70, time.Now())
	result := book.WalkAsks(0.5)
	if result.PriceImpactBps < 0 {
		t.Fatalf("buy impact should be non-negative, got %f", result.PriceImpactBps)
	}
}

func TestBookWalk_ImpactPositiveForSell(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.70, time.Now())
	result := book.WalkBids(0.5)
	if result.PriceImpactBps < 0 {
		t.Fatalf("sell impact should be non-negative, got %f", result.PriceImpactBps)
	}
}

func TestBookWalk_LargeOrder_WalksMultipleLevels(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	// Buy more than best ask level can provide.
	bestAskSize := book.BestAsk().Size
	result := book.WalkAsks(bestAskSize * 3)
	if result.LevelsConsumed < 2 {
		t.Fatalf("large order should consume >= 2 levels, consumed %d", result.LevelsConsumed)
	}
}

func TestBookWalk_FilledCappedByAvailableDepth(t *testing.T) {
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	// Try to buy 10,000 BTC (way more than book can provide).
	result := book.WalkAsks(10_000)
	if result.FilledQuantity >= 10_000 {
		t.Fatal("can't fill more BTC than the book has")
	}
	if result.UnfilledQuantity <= 0 {
		t.Fatal("should have unfilled quantity when order exceeds book depth")
	}
}

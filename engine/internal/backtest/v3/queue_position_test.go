package v3

import (
	"testing"
	"time"
)

func TestQueueEngine_TakerOrderFillsImmediately(t *testing.T) {
	engine := NewQueuePositionEngine()
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	order := QueueOrder{
		OrderID:     "O1",
		Side:        "BUY",
		Type:        "MARKET",
		LimitPrice:  50100, // above best ask → taker
		Quantity:    0.1,
		SubmittedAt: time.Now(),
	}
	result := engine.Estimate(order, book, 0)
	if result.Class != ClassTaker {
		t.Fatalf("market buy above ask should be ClassTaker, got %s", result.Class)
	}
	if result.FillProbability != 1.0 {
		t.Fatalf("taker should have 100%% fill probability, got %f", result.FillProbability)
	}
	if result.ExpectedFillTime > 10*1e6 { // 10ms in nanoseconds
		t.Fatalf("taker should fill in < 10ms, got %v", result.ExpectedFillTime)
	}
}

func TestQueueEngine_MakerOrderHasQueueDepth(t *testing.T) {
	engine := NewQueuePositionEngine()
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	bestBid := book.BestBid().Price
	order := QueueOrder{
		OrderID:     "O2",
		Side:        "BUY",
		Type:        "LIMIT",
		LimitPrice:  bestBid - 10, // below best bid → deep maker
		Quantity:    0.5,
		SubmittedAt: time.Now(),
	}
	result := engine.Estimate(order, book, 0)
	if result.Class != ClassMaker {
		t.Fatalf("limit order below bid should be ClassMaker, got %s", result.Class)
	}
	if result.FillProbability <= 0 || result.FillProbability > 1 {
		t.Fatalf("fill probability out of range: %f", result.FillProbability)
	}
}

func TestQueueEngine_FillProbabilityInRange(t *testing.T) {
	engine := NewQueuePositionEngine()
	book := NewOrderBook(50000, 0.20, 0.65, time.Now())
	for _, liqScore := range []float64{0.1, 0.5, 0.9} {
		book2 := NewOrderBook(50000, 0.20, liqScore, time.Now())
		order := QueueOrder{
			OrderID:     "O3",
			Side:        "BUY",
			Type:        "LIMIT",
			LimitPrice:  book2.BestBid().Price,
			Quantity:    1.0,
			SubmittedAt: time.Now(),
		}
		result := engine.Estimate(order, book2, 0)
		if result.FillProbability < 0 || result.FillProbability > 1 {
			t.Fatalf("liqScore=%f: fill probability %f out of [0,1]", liqScore, result.FillProbability)
		}
		_ = book // suppress unused warning
	}
}

func TestQueueEngine_PartialFillEventsOrdered(t *testing.T) {
	engine := NewQueuePositionEngine()
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	order := QueueOrder{
		OrderID:     "O4",
		Side:        "BUY",
		Type:        "LIMIT",
		LimitPrice:  book.BestBid().Price,
		Quantity:    2.0,
		SubmittedAt: time.Now(),
	}
	result := engine.Estimate(order, book, 0)
	for i := 1; i < len(result.PartialFills); i++ {
		if result.PartialFills[i].Timestamp.Before(result.PartialFills[i-1].Timestamp) {
			t.Fatal("partial fill timestamps must be non-decreasing")
		}
		if result.PartialFills[i].FillNumber != i+1 {
			t.Fatalf("fill number should be %d, got %d", i+1, result.PartialFills[i].FillNumber)
		}
	}
}

func TestQueueEngine_ClassifyOrder(t *testing.T) {
	engine := NewQueuePositionEngine()
	book := NewOrderBook(50000, 0.20, 0.80, time.Now())
	bestAsk := book.BestAsk().Price
	bestBid := book.BestBid().Price

	cases := []struct {
		side       string
		limit      float64
		wantClass  MakerTakerClass
	}{
		{"BUY", bestAsk + 1, ClassTaker},
		{"BUY", bestBid - 1, ClassMaker},
		{"SELL", bestBid - 1, ClassTaker},
		{"SELL", bestAsk + 1, ClassMaker},
	}
	for _, c := range cases {
		order := QueueOrder{Side: c.side, Type: "LIMIT", LimitPrice: c.limit}
		got := engine.ClassifyOrder(order, book)
		if got != c.wantClass {
			t.Errorf("side=%s limit=%f: want %s got %s", c.side, c.limit, c.wantClass, got)
		}
	}
}

package marketdata

import (
	"testing"
	"time"
)

func TestCandleAggregatorUsesTickTimestamp(t *testing.T) {
	agg := NewCandleAggregator()
	base := time.Date(2026, time.April, 1, 9, 30, 0, 0, time.UTC)

	agg.Feed(Tick{
		Symbol:   "BTC-USD",
		Price:    100,
		Quantity: 1,
		TimeMs:   base.Add(10 * time.Second).UnixMilli(),
	})
	agg.Feed(Tick{
		Symbol:   "BTC-USD",
		Price:    101,
		Quantity: 2,
		TimeMs:   base.Add(50 * time.Second).UnixMilli(),
	})
	agg.Feed(Tick{
		Symbol:   "BTC-USD",
		Price:    102,
		Quantity: 3,
		TimeMs:   base.Add(65 * time.Second).UnixMilli(),
	})

	select {
	case candle := <-agg.Candles1m:
		if !candle.OpenTime.Equal(base) {
			t.Fatalf("expected candle open time %s, got %s", base, candle.OpenTime)
		}
		if candle.CloseTime != base.Add(time.Minute) {
			t.Fatalf("expected candle close time %s, got %s", base.Add(time.Minute), candle.CloseTime)
		}
		if candle.Open != 100 || candle.Close != 101 {
			t.Fatalf("expected first candle O/C 100/101, got %.2f/%.2f", candle.Open, candle.Close)
		}
	default:
		t.Fatal("expected first 1m candle to be emitted")
	}
}

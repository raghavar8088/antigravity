package strategy

import (
	"testing"
	"time"

	"antigravity-engine/internal/marketdata"
)

func feedStrategy(strat Strategy, start time.Time, prices, volumes []float64) []Signal {
	signals := make([]Signal, 0, len(prices))
	for i, price := range prices {
		volume := 1.0
		if i < len(volumes) {
			volume = volumes[i]
		}
		signals = append(signals, strat.OnCandle(marketdata.Tick{
			Symbol:   "BTC-USD",
			Price:    price,
			Quantity: volume,
			TimeMs:   start.Add(time.Duration(i+1) * time.Minute).UnixMilli(),
		})...)
	}
	return signals
}

func hasAction(signals []Signal, action Action) bool {
	for _, signal := range signals {
		if signal.Action == action {
			return true
		}
	}
	return false
}

func TestVWAPRSI2ReversionScalperSignalsLong(t *testing.T) {
	strat := NewVWAPRSI2ReversionScalper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	for i := 0; i < 42; i++ {
		prices = append(prices, 100.0+float64(i%3)*0.02)
		volumes = append(volumes, 1.0)
	}
	prices = append(prices, 99.4, 98.9, 98.2)
	volumes = append(volumes, 1.1, 1.2, 1.3)

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected VWAP RSI2 strategy to emit a buy signal")
	}
}

func TestATRVolumeImpulseScalperRejectsOverextendedBreakout(t *testing.T) {
	strat := NewATRVolumeImpulseScalper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	price := 100.0
	for i := 0; i < 30; i++ {
		if i%5 == 4 {
			price -= 0.10
		} else {
			price += 0.06
		}
		prices = append(prices, price)
		volumes = append(volumes, 1.0)
	}
	prices = append(prices, price+0.95)
	volumes = append(volumes, 3.5)

	signals := feedStrategy(strat, start, prices, volumes)
	if hasAction(signals, ActionBuy) {
		t.Fatalf("expected ATR volume impulse strategy to reject the overextended breakout fixture")
	}
}

func TestMACDVWAPFlipScalperWaitsForFreshHistogramFlip(t *testing.T) {
	strat := NewMACDVWAPFlipScalper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	price := 100.0
	for i := 0; i < 26; i++ {
		prices = append(prices, price)
		volumes = append(volumes, 1.0)
		price -= 0.18
	}
	for i := 0; i < 24; i++ {
		price += 0.28
		prices = append(prices, price)
		volumes = append(volumes, 1.4)
	}

	signals := feedStrategy(strat, start, prices, volumes)
	if hasAction(signals, ActionBuy) {
		t.Fatalf("expected MACD VWAP flip strategy to wait for a fresh histogram flip")
	}
}

func TestOpeningRangeBreakoutScalperSignalsLong(t *testing.T) {
	strat := NewOpeningRangeBreakoutScalper(16, 0, 15)
	base := time.Date(2026, 1, 1, 16, 0, 0, 0, time.UTC)

	var signals []Signal
	rangePrices := []float64{
		100.10, 100.20, 100.05, 100.30, 100.15,
		100.25, 100.12, 100.28, 100.18, 100.35,
		100.16, 100.22, 100.14, 100.26, 100.19,
	}
	for i, price := range rangePrices {
		signals = append(signals, strat.OnCandle(marketdata.Tick{
			Symbol:   "BTC-USD",
			Price:    price,
			Quantity: 1.0,
			TimeMs:   base.Add(time.Duration(i+1) * time.Minute).UnixMilli(),
		})...)
	}

	signals = append(signals, strat.OnCandle(marketdata.Tick{
		Symbol:   "BTC-USD",
		Price:    101.20,
		Quantity: 2.5,
		TimeMs:   base.Add(16 * time.Minute).UnixMilli(),
	})...)

	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected opening range breakout strategy to emit a buy signal")
	}
}

func TestChartWedgeBreakoutScalperSignalsLong(t *testing.T) {
	strat := NewChartWedgeBreakoutScalper()
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64

	// Wide range first.
	for i := 0; i < 24; i++ {
		if i%2 == 0 {
			prices = append(prices, 101.10)
		} else {
			prices = append(prices, 99.20)
		}
		volumes = append(volumes, 1.0)
	}

	// Compressed range next.
	for i := 0; i < 24; i++ {
		if i%2 == 0 {
			prices = append(prices, 100.18)
		} else {
			prices = append(prices, 99.94)
		}
		volumes = append(volumes, 1.05)
	}

	// Breakout candle.
	prices = append(prices, 101.05)
	volumes = append(volumes, 2.2)

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected chart wedge breakout strategy to emit a buy signal")
	}
}

func TestChartDoubleTapReversalScalperSignalsLong(t *testing.T) {
	strat := NewChartDoubleTapReversalScalper()
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	prices := []float64{
		100.20, 100.05, 100.25, 100.10, 100.30, 100.15, 100.28, 100.12,
		100.26, 100.08, 100.22, 100.02, 100.18, 99.95, 100.12, 99.90,
		100.08, 99.86, 100.05, 99.82, 99.45, 99.20, 99.05, 99.55,
		100.05, 100.30, 99.95, 99.70, 99.45, 99.25, 99.12, 99.04, 99.10,
	}
	volumes := make([]float64, len(prices))
	for i := range volumes {
		volumes[i] = 1.0
	}
	volumes[len(volumes)-1] = 1.5

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected chart double tap strategy to emit a buy signal")
	}
}

func TestSentimentConfluenceProScalperSignalsLong(t *testing.T) {
	strat := NewSentimentConfluenceProScalper()
	start := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	price := 100.0

	for i := 0; i < 70; i++ {
		step := 0.08
		if i >= 45 {
			step = 0.14
		}
		price += step
		prices = append(prices, price)
		volumes = append(volumes, 1.0)
	}
	for i := 0; i < 8; i++ {
		price += 0.22
		prices = append(prices, price)
		if i == 7 {
			volumes = append(volumes, 2.4)
		} else {
			volumes = append(volumes, 1.2)
		}
	}

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected sentiment confluence strategy to emit a buy signal")
	}
}

func TestSentimentConfluenceProScalperSignalsShort(t *testing.T) {
	strat := NewSentimentConfluenceProScalper()
	start := time.Date(2026, 1, 3, 2, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	price := 100.0

	for i := 0; i < 70; i++ {
		step := 0.08
		if i >= 45 {
			step = 0.14
		}
		price -= step
		prices = append(prices, price)
		volumes = append(volumes, 1.0)
	}
	for i := 0; i < 8; i++ {
		price -= 0.22
		prices = append(prices, price)
		if i == 7 {
			volumes = append(volumes, 2.4)
		} else {
			volumes = append(volumes, 1.2)
		}
	}

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionSell) {
		t.Fatalf("expected sentiment confluence strategy to emit a sell signal")
	}
}

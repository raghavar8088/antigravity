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

func TestATRVolumeImpulseScalperSignalsLong(t *testing.T) {
	strat := NewATRVolumeImpulseScalper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	for i := 0; i < 24; i++ {
		offset := 0.02
		if i%2 == 0 {
			offset = -0.02
		}
		prices = append(prices, 100.0+offset)
		volumes = append(volumes, 1.0)
	}
	prices = append(prices, 101.5)
	volumes = append(volumes, 4.5)

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected ATR volume impulse strategy to emit a buy signal")
	}
}

func TestMACDVWAPFlipScalperSignalsLong(t *testing.T) {
	strat := NewMACDVWAPFlipScalper()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var prices []float64
	var volumes []float64
	price := 100.0
	for i := 0; i < 20; i++ {
		prices = append(prices, price)
		volumes = append(volumes, 1.0)
		price -= 0.15
	}
	for i := 0; i < 22; i++ {
		price += 0.22
		prices = append(prices, price)
		volumes = append(volumes, 1.3)
	}

	signals := feedStrategy(strat, start, prices, volumes)
	if !hasAction(signals, ActionBuy) {
		t.Fatalf("expected MACD VWAP flip strategy to emit a buy signal")
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

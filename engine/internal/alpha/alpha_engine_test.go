package alpha_test

import (
	"testing"
	"time"

	"antigravity-engine/internal/alpha"
	"antigravity-engine/internal/alpha/cvd"
	"antigravity-engine/internal/alpha/fvg"
	"antigravity-engine/internal/alpha/liquidity"
	"antigravity-engine/internal/alpha/quality"
)

func TestCVDBullishDivergence(t *testing.T) {
	prices := []float64{100, 99, 98, 99, 97, 98, 99, 100}
	cvdSeries := []float64{-10, -9, -8, -7, -5, -4, -3, -2}
	div := cvd.DetectDivergence(prices, cvdSeries)
	if div.Direction != alpha.ActionBuy {
		t.Fatalf("expected bullish CVD divergence, got %s", div.Direction)
	}
	if div.Confidence < 0.75 {
		t.Fatalf("expected institutional confidence, got %.2f", div.Confidence)
	}
}

func TestLiquiditySweepDetection(t *testing.T) {
	now := time.Now().UTC()
	candles := []alpha.Candle{
		{Symbol: "BTC-USD", High: 101, Low: 99, Close: 100, Volume: 10, Timestamp: now.Add(-4 * time.Minute)},
		{Symbol: "BTC-USD", High: 100.8, Low: 99.01, Close: 100, Volume: 10, Timestamp: now.Add(-3 * time.Minute)},
		{Symbol: "BTC-USD", High: 100.5, Low: 99.02, Close: 99.5, Volume: 10, Timestamp: now.Add(-2 * time.Minute)},
		{Symbol: "BTC-USD", High: 100.2, Low: 98.7, Close: 99.3, Volume: 18, Timestamp: now.Add(-time.Minute)},
		{Symbol: "BTC-USD", High: 100.4, Low: 98.5, Close: 99.4, Volume: 20, Timestamp: now},
	}
	levels := liquidity.DetectLevels("BTC-USD", []float64{101, 100.8, 100.5, 100.2}, []float64{99, 99.01, 99.02, 98.7}, 0.06)
	event := liquidity.DetectSweep(candles, levels)
	if event.Direction != alpha.ActionBuy {
		t.Fatalf("expected bullish liquidity sweep, got %s", event.Direction)
	}
}

func TestFVGDetectionAndQualityGate(t *testing.T) {
	candles := []alpha.Candle{
		{Symbol: "BTC-USD", High: 100, Low: 98, Close: 99, Timestamp: time.Now().UTC()},
		{Symbol: "BTC-USD", High: 102, Low: 99, Close: 101, Timestamp: time.Now().UTC()},
		{Symbol: "BTC-USD", High: 104, Low: 101, Close: 103, Timestamp: time.Now().UTC()},
	}
	gaps := fvg.Detect(candles)
	if len(gaps) == 0 || gaps[0].Direction != alpha.ActionBuy {
		t.Fatal("expected bullish FVG")
	}
	score := quality.Score(quality.Inputs{FVG: 1, MSS: 1, CVD: 1, Liquidity: 1, Delta: 1, OrderBlock: 0.5, Funding: 0.5, VolumeProfile: 0.5, Session: 0.5})
	if !quality.MandatoryPass(score.Score) {
		t.Fatalf("expected quality gate pass, got %d", score.Score)
	}
}

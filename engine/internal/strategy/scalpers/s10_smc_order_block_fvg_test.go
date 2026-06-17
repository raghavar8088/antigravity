package scalpers

import (
	"math"
	"testing"
	"time"
)

// flatCandle returns a low-volatility candle centered on price with the
// given half-range, used to pad out warmup history that shouldn't influence
// the structure under test.
func flatCandle(t time.Time, price, halfRange float64) Candle {
	return Candle{
		OpenTime: t,
		Open:     price,
		Close:    price + halfRange*0.1,
		High:     price + halfRange,
		Low:      price - halfRange,
		Volume:   10,
	}
}

// buildSMCBullishCandles constructs a 26-bar 15m sequence with: a calmer
// baseline (indices 0-14), a few wider-range bars (indices 11-14 overlap is
// fine — they still only feed ATR, not the swing-high window) to give ATR
// enough magnitude, a flat pre-impulse zone (15-20), then a tight bearish OB
// candle (21) immediately followed by two bullish confirmation candles
// (22, 23) that leave an unfilled bullish FVG, a continuation candle (24)
// that sets the swing high, and a still-forming candle (25).
func buildSMCBullishCandles() []Candle {
	base := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	c := make([]Candle, 0, 26)
	t := func(i int) time.Time { return base.Add(time.Duration(i) * 15 * time.Minute) }

	// 0-10: calm baseline, far from the structure under test.
	for i := 0; i <= 10; i++ {
		c = append(c, flatCandle(t(i), 100, 0.3))
	}
	// 11-14: wider-range bars purely to push ATR up (outside the 10-bar
	// swing-high window, which only looks at the last 10 of the prior slice).
	wide := []struct{ open, close, low, high float64 }{
		{99.0, 101.5, 98.0, 102.0},
		{101.5, 99.5, 99.0, 102.5},
		{99.5, 101.8, 98.5, 102.2},
		{101.8, 100.0, 99.0, 102.4},
	}
	for i, w := range wide {
		c = append(c, Candle{OpenTime: t(11 + i), Open: w.open, Close: w.close, Low: w.low, High: w.high, Volume: 20})
	}
	// 15-20: calmer pre-impulse bars, capped well below the eventual breakout.
	for i := 15; i <= 20; i++ {
		c = append(c, flatCandle(t(i), 100.5, 0.4))
	}
	// 21: bearish OB candle, tight range.
	c = append(c, Candle{OpenTime: t(21), Open: 102.55, Close: 102.52, Low: 102.50, High: 102.60, Volume: 15})
	// 22: bullish confirmation 1.
	c = append(c, Candle{OpenTime: t(22), Open: 102.52, Close: 102.65, Low: 102.51, High: 102.66, Volume: 15})
	// 23: bullish confirmation 2 — close exceeds OB candle's open (102.55), and
	// candle21.High (102.60) < candle23.Low (102.66) leaves an unfilled FVG.
	c = append(c, Candle{OpenTime: t(23), Open: 102.65, Close: 102.70, Low: 102.66, High: 102.72, Volume: 15})
	// 24: continuation bar that becomes the swing high; close kept outside
	// the FVG zone [102.60, 102.66] so the gap stays unfilled.
	c = append(c, Candle{OpenTime: t(24), Open: 102.70, Close: 102.71, Low: 102.69, High: 102.73, Volume: 15})
	// 25: still-forming current bar; close also kept outside the FVG zone.
	c = append(c, Candle{OpenTime: t(25), Open: 102.71, Close: 102.78, Low: 102.70, High: 102.80, Volume: 15})

	return c
}

// buildSMCBearishCandles mirrors buildSMCBullishCandles for a short setup.
func buildSMCBearishCandles() []Candle {
	base := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	c := make([]Candle, 0, 26)
	t := func(i int) time.Time { return base.Add(time.Duration(i) * 15 * time.Minute) }

	for i := 0; i <= 10; i++ {
		c = append(c, flatCandle(t(i), 100, 0.3))
	}
	wide := []struct{ open, close, low, high float64 }{
		{101.0, 98.5, 97.5, 102.0},
		{98.5, 100.5, 97.5, 101.5},
		{100.5, 98.2, 97.5, 101.5},
		{98.2, 100.0, 97.5, 101.0},
	}
	for i, w := range wide {
		c = append(c, Candle{OpenTime: t(11 + i), Open: w.open, Close: w.close, Low: w.low, High: w.high, Volume: 20})
	}
	for i := 15; i <= 20; i++ {
		c = append(c, flatCandle(t(i), 99.5, 0.4))
	}
	// 21: bullish OB candle, tight range.
	c = append(c, Candle{OpenTime: t(21), Open: 97.45, Close: 97.48, Low: 97.40, High: 97.50, Volume: 15})
	// 22: bearish confirmation 1.
	c = append(c, Candle{OpenTime: t(22), Open: 97.48, Close: 97.35, Low: 97.34, High: 97.49, Volume: 15})
	// 23: bearish confirmation 2 — close goes below OB candle's open (97.45);
	// candle21.Low (97.40) > candle23.High (97.34) leaves an unfilled bearish FVG.
	c = append(c, Candle{OpenTime: t(23), Open: 97.35, Close: 97.30, Low: 97.28, High: 97.34, Volume: 15})
	// 24: continuation bar that becomes the swing low.
	c = append(c, Candle{OpenTime: t(24), Open: 97.30, Close: 97.29, Low: 97.27, High: 97.31, Volume: 15})
	// 25: still-forming current bar.
	c = append(c, Candle{OpenTime: t(25), Open: 97.29, Close: 97.22, Low: 97.20, High: 97.30, Volume: 15})

	return c
}

func TestSMCOrderBlockFVG_NoSignalWhenVolatileRegime(t *testing.T) {
	s := &SMCOrderBlockFVG{}
	c15 := buildSMCBullishCandles()
	atr := ATR(c15, 14)
	fvg := detectBullishFVG(c15)
	swingHigh := SwingHigh(c15[:len(c15)-1], 10)
	price := math.Max(swingHigh+0.01, fvg.High-0.1*atr)

	ctx := MarketContext{
		Regime:     RegimeVolatile,
		Price:      price,
		Candles15m: c15,
		CVD:        100,
		CVDHistory: []float64{10, 20, 30},
	}
	sig := s.Evaluate(ctx)
	if sig.Direction != DirectionNone {
		t.Fatalf("expected NoSignal in VOLATILE regime, got %s", sig.Direction)
	}
}

func TestSMCOrderBlockFVG_NoSignalWhenInsufficientCandles(t *testing.T) {
	s := &SMCOrderBlockFVG{}
	c15 := buildSMCBullishCandles()[:24] // < 25 candles
	ctx := MarketContext{
		Regime:     RegimeTrending,
		Price:      102.78,
		Candles15m: c15,
		CVD:        100,
		CVDHistory: []float64{10, 20, 30},
	}
	sig := s.Evaluate(ctx)
	if sig.Direction != DirectionNone {
		t.Fatalf("expected NoSignal with <25 candles, got %s", sig.Direction)
	}
}

func TestSMCOrderBlockFVG_NoSignalWhenNoFVG(t *testing.T) {
	s := &SMCOrderBlockFVG{}
	base := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	c15 := make([]Candle, 0, 30)
	// Perfectly flat, overlapping candles never leave a 3-bar gap.
	for i := 0; i < 30; i++ {
		c15 = append(c15, flatCandle(base.Add(time.Duration(i)*15*time.Minute), 100, 0.2))
	}
	ctx := MarketContext{
		Regime:     RegimeTrending,
		Price:      100.5,
		Candles15m: c15,
		CVD:        100,
		CVDHistory: []float64{10, 20, 30},
	}
	sig := s.Evaluate(ctx)
	if sig.Direction != DirectionNone {
		t.Fatalf("expected NoSignal when no FVG present, got %s", sig.Direction)
	}
}

func TestSMCOrderBlockFVG_LongSignalFiresOnFullConfluence(t *testing.T) {
	s := &SMCOrderBlockFVG{}
	c15 := buildSMCBullishCandles()

	fvg := detectBullishFVG(c15)
	if !fvg.Found {
		t.Fatalf("test fixture invalid: expected an unfilled bullish FVG")
	}
	obLow, obHigh, obFound := detectBullishOB(c15)
	if !obFound {
		t.Fatalf("test fixture invalid: expected a bullish OB")
	}
	atr := ATR(c15, 14)
	swingHigh := SwingHigh(c15[:len(c15)-1], 10)

	price := swingHigh + 0.01
	if price > fvg.High+0.2*atr {
		t.Fatalf("test fixture invalid: retest price %.4f outside FVG+0.2*ATR (%.4f)", price, fvg.High+0.2*atr)
	}

	ctx := MarketContext{
		Regime:     RegimeTrending,
		Price:      price,
		Candles15m: c15,
		CVD:        50,
		CVDHistory: []float64{10, 20, 30}, // rising 3-bar
		OrderBook:  OrderBookSnapshot{BidWallSize: 10, AskWallSize: 5, Imbalance: 0.2},
	}

	sig := s.Evaluate(ctx)
	if sig.Direction != DirectionLong {
		t.Fatalf("expected LONG signal, got %s (reason=%s)", sig.Direction, sig.Reason)
	}

	wantSL := obLow - 0.5*atr
	wantTP := fvg.High + 1.5*atr
	wantTP2 := fvg.High + 3.0*atr
	if math.Abs(sig.StopLoss-wantSL) > 1e-9 {
		t.Errorf("StopLoss = %.4f, want %.4f", sig.StopLoss, wantSL)
	}
	if math.Abs(sig.TakeProfit-wantTP) > 1e-9 {
		t.Errorf("TakeProfit = %.4f, want %.4f", sig.TakeProfit, wantTP)
	}
	if math.Abs(sig.TakeProfit2-wantTP2) > 1e-9 {
		t.Errorf("TakeProfit2 = %.4f, want %.4f", sig.TakeProfit2, wantTP2)
	}
	if sig.StopLoss >= obHigh {
		t.Errorf("StopLoss %.4f should sit below the OB zone (obHigh=%.4f)", sig.StopLoss, obHigh)
	}

	// Both CVDHistory confluence and OB imbalance confluence apply: 0.72+0.05+0.05=0.82.
	wantConf := 0.82
	if math.Abs(sig.Confidence-wantConf) > 1e-9 {
		t.Errorf("Confidence = %.4f, want %.4f", sig.Confidence, wantConf)
	}
}

func TestSMCOrderBlockFVG_ShortSignalFiresOnFullConfluence(t *testing.T) {
	s := &SMCOrderBlockFVG{}
	c15 := buildSMCBearishCandles()

	fvg := detectBearishFVG(c15)
	if !fvg.Found {
		t.Fatalf("test fixture invalid: expected an unfilled bearish FVG")
	}
	obLow, obHigh, obFound := detectBearishOB(c15)
	if !obFound {
		t.Fatalf("test fixture invalid: expected a bearish OB")
	}
	atr := ATR(c15, 14)
	swingLow := SwingLow(c15[:len(c15)-1], 10)

	price := swingLow - 0.01
	if price < fvg.Low-0.2*atr {
		t.Fatalf("test fixture invalid: retest price %.4f outside FVG-0.2*ATR (%.4f)", price, fvg.Low-0.2*atr)
	}

	ctx := MarketContext{
		Regime:     RegimeRanging,
		Price:      price,
		Candles15m: c15,
		CVD:        -50,
		CVDHistory: []float64{30, 20, 10}, // falling 3-bar
		OrderBook:  OrderBookSnapshot{BidWallSize: 5, AskWallSize: 10, Imbalance: -0.2},
	}

	sig := s.Evaluate(ctx)
	if sig.Direction != DirectionShort {
		t.Fatalf("expected SHORT signal, got %s (reason=%s)", sig.Direction, sig.Reason)
	}

	wantSL := obHigh + 0.5*atr
	wantTP := fvg.Low - 1.5*atr
	wantTP2 := fvg.Low - 3.0*atr
	if math.Abs(sig.StopLoss-wantSL) > 1e-9 {
		t.Errorf("StopLoss = %.4f, want %.4f", sig.StopLoss, wantSL)
	}
	if math.Abs(sig.TakeProfit-wantTP) > 1e-9 {
		t.Errorf("TakeProfit = %.4f, want %.4f", sig.TakeProfit, wantTP)
	}
	if math.Abs(sig.TakeProfit2-wantTP2) > 1e-9 {
		t.Errorf("TakeProfit2 = %.4f, want %.4f", sig.TakeProfit2, wantTP2)
	}
	if sig.StopLoss <= obLow {
		t.Errorf("StopLoss %.4f should sit above the OB zone (obLow=%.4f)", sig.StopLoss, obLow)
	}

	wantConf := 0.82
	if math.Abs(sig.Confidence-wantConf) > 1e-9 {
		t.Errorf("Confidence = %.4f, want %.4f", sig.Confidence, wantConf)
	}
}

func TestSMCOrderBlockFVG_ConfidenceNotBoostedWithoutConfirmation(t *testing.T) {
	s := &SMCOrderBlockFVG{}
	c15 := buildSMCBullishCandles()
	fvg := detectBullishFVG(c15)
	atr := ATR(c15, 14)
	swingHigh := SwingHigh(c15[:len(c15)-1], 10)
	price := swingHigh + 0.01
	if price > fvg.High+0.2*atr {
		t.Fatalf("test fixture invalid: retest price outside FVG+0.2*ATR")
	}

	ctx := MarketContext{
		Regime:     RegimeTrending,
		Price:      price,
		Candles15m: c15,
		CVD:        5, // still > 0 so the base CVD gate passes
		CVDHistory: []float64{30, 20, 10}, // NOT rising 3-bar → no CVD boost
		OrderBook:  OrderBookSnapshot{}, // unpopulated → no imbalance boost
	}

	sig := s.Evaluate(ctx)
	if sig.Direction != DirectionLong {
		t.Fatalf("expected LONG signal (CVD>0 still satisfies the base gate), got %s (reason=%s)", sig.Direction, sig.Reason)
	}
	if math.Abs(sig.Confidence-0.72) > 1e-9 {
		t.Errorf("Confidence = %.4f, want base 0.72 (no boosts)", sig.Confidence)
	}
}

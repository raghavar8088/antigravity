package scalpers

import "testing"

// A swing needs candles on BOTH sides to confirm it, so the most recent bars
// can never be swings.
//
// This is the lookahead that would be invisible in results: recognising a swing
// high on the bar that is still forming means a "double top" gets drawn on a
// market still making highs, and the backtest would look excellent while the
// live version entered into strength.
func TestSwingPoints_CannotSeeTheFormingBars(t *testing.T) {
	c := mtfCandles(60, 100, 0.001, 0.004)
	w := 3
	highs, lows := swingPoints(c, w)
	limit := len(c) - w
	for _, i := range highs {
		if i >= limit {
			t.Errorf("swing high at index %d is within %d bars of the end — it cannot be confirmed yet", i, w)
		}
	}
	for _, i := range lows {
		if i >= limit {
			t.Errorf("swing low at index %d is within %d bars of the end", i, w)
		}
	}
}

// A swing high must genuinely be the highest of its window, or the structure
// families are reading noise as structure.
func TestSwingPoints_HighsAreLocalMaxima(t *testing.T) {
	c := mtfCandles(80, 100, 0.0005, 0.006)
	w := 3
	highs, _ := swingPoints(c, w)
	if len(highs) == 0 {
		t.Skip("no swings in this series")
	}
	for _, i := range highs {
		for j := i - w; j <= i+w; j++ {
			if j != i && c[j].High >= c[i].High {
				t.Fatalf("index %d marked a swing high but bar %d is at least as high", i, j)
			}
		}
	}
}

// bodyFrac must not divide by zero on a flat candle — those occur constantly on
// thin symbols, where a whole minute can pass with no trade.
func TestBodyFrac_FlatCandleIsNotADivideByZero(t *testing.T) {
	flat := Candle{Open: 10, High: 10, Low: 10, Close: 10}
	if got := bodyFrac(flat); got != 0 {
		t.Errorf("flat candle gave bodyFrac %v, want 0", got)
	}
}

// Every pattern family must refuse to signal on insufficient history, exactly
// like the indicator families. A pattern needs its neighbours to mean anything.
func TestPatternFamilies_RefuseShortHistory(t *testing.T) {
	short := mtfCandles(20, 100, 0.001, 0.003)
	fams := map[string]func(bool) func(string, []Candle, float64) Signal{
		"Engulfing":        patEngulfing,
		"PinBar":           patPinBar,
		"InsideBarBreak":   patInsideBarBreak,
		"ThreeBarReversal": patThreeBarReversal,
		"Star":             patStar,
		"DojiBreak":        patDojiBreak,
		"DoubleTopBottom":  patDoubleTopBottom,
		"StructureBreak":   patStructureBreak,
		"TriangleBreak":    patTriangleBreak,
		"LevelRetest":      patLevelRetest,
	}
	for name, mk := range fams {
		for _, long := range []bool{true, false} {
			if s := mk(long)("T", short, 100); s.Direction != DirectionNone {
				t.Errorf("%s (long=%v) signalled on 20 candles", name, long)
			}
		}
	}
}

// A clean bullish engulfing above the trend must be recognised — otherwise the
// refusal tests above would pass on a family that never fires at all.
func TestPatEngulfing_RecognisesTheRealThing(t *testing.T) {
	// 130 candles: EMA55 needs 110 to compute at all, which is what the
	// MinCandles fix encodes.
	c := mtfCandles(130, 100, 0.002, 0.001) // steady uptrend, low noise
	n := len(c)

	// Levels are derived from where the series ACTUALLY is, not hardcoded.
	// Fixed numbers put the pattern below a drifting EMA55 and the family
	// correctly refused it — the test was wrong, not the strategy.
	base := c[n-3].Close
	prev := &c[n-2]
	prev.Open, prev.Close = base*1.010, base*1.002
	prev.High, prev.Low = base*1.012, base*1.000

	last := &c[n-1]
	last.Open, last.Close = base*1.001, base*1.020
	last.High, last.Low = base*1.021, base*1.000
	last.Volume = c[n-3].Volume * 3

	price := last.Close
	s := patEngulfing(true)("T", c, price)
	if s.Direction != DirectionLong {
		ema, _ := mtfEMA(c, 55)
		t.Fatalf("clean bullish engulfing not recognised (price %.4f, EMA55 %.4f): %q", price, ema, s.Reason)
	}
	if s.StopLoss >= price || s.TakeProfit <= price {
		t.Errorf("levels inverted: stop %.4f target %.4f", s.StopLoss, s.TakeProfit)
	}
}

// The doji family must trade the RESOLUTION, never the doji. Trading the doji
// is trading the absence of a decision.
func TestPatDojiBreak_DoesNotTradeTheDojiItself(t *testing.T) {
	c := mtfCandles(130, 100, 0.0, 0.002)
	n := len(c)
	// Make the LAST candle a doji: there is no resolution yet.
	last := &c[n-1]
	mid := last.Open
	last.Close = mid
	last.High, last.Low = mid*1.002, mid*0.998
	if s := patDojiBreak(true)("T", c, mid); s.Direction != DirectionNone {
		t.Errorf("signalled on the doji itself: %s", s.Reason)
	}
}

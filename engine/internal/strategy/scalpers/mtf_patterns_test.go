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
	// The series must RISE, PULL BACK, then engulf.
	//
	// A steadily rising series has no swing high above the last bar, and the
	// family now correctly declines: with structural targets there is nothing
	// for the trade to reach. That is a real behaviour change — the family
	// trades pullbacks within a trend rather than continuation at new highs —
	// and this test has to reflect it rather than assert the old behaviour.
	c := mtfCandles(130, 100, 0.002, 0.001)
	n := len(c)

	// Carve a pullback into the last 12 bars so a confirmed swing high sits
	// above the entry.
	peak := c[n-13].Close
	for i := n - 12; i < n; i++ {
		drop := peak * (1 - 0.02*float64(i-(n-13))/12.0)
		c[i].Open, c[i].Close = drop, drop
		c[i].High, c[i].Low = drop*1.001, drop*0.999
	}

	base := c[n-3].Close
	prev := &c[n-2]
	prev.Open, prev.Close = base*1.004, base*0.998
	prev.High, prev.Low = base*1.005, base*0.997

	last := &c[n-1]
	last.Open, last.Close = base*0.997, base*1.008
	last.High, last.Low = base*1.009, base*0.996
	last.Volume = c[n-20].Volume * 3

	price := last.Close
	s := patEngulfing(true)("T", c, price)
	if s.Direction != DirectionLong {
		ema, _ := mtfEMA(c, 55)
		sw, ok := priorSwing(c, true)
		t.Fatalf("clean bullish engulfing not recognised (price %.4f, EMA55 %.4f, swing %.4f ok=%v): %q",
			price, ema, sw, ok, s.Reason)
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

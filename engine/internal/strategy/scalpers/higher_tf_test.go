package scalpers

import "testing"

// Every timeframe must allow enough candles for the SLOWEST indicator any pack
// uses, or those families silently never fire.
//
// mtfEMA seeds with an SMA and needs 2n candles, so EMA55 needs 110. The
// original per-timeframe minimums (1h 100, 4h 80, 1d 60) all sat below it: the
// indicator returned ok=false forever and the strategy returned no signal,
// which looks exactly like a strategy that found no setups.
func TestMinCandles_CoversTheSlowestIndicator(t *testing.T) {
	const slowestEMA = 55
	need := slowestEMA * 2
	for _, tf := range []HigherTF{TF15m, TF30m, TF1h, TF4h, TF1d} {
		if got := tf.MinCandles(); got < need {
			t.Errorf("%s allows %d candles but EMA%d needs %d — those families would never signal",
				tf, got, slowestEMA, need)
		}
	}
}

// CandlesFor must report insufficiency rather than hand back a short slice.
func TestCandlesFor_ReportsInsufficientHistory(t *testing.T) {
	ctx := MarketContext{Candles4h: mtfCandles(30, 100, 0.001, 0.002)}
	if _, ok := TF4h.CandlesFor(ctx); ok {
		t.Error("30 candles reported as sufficient for 4h")
	}
	ctx.Candles4h = mtfCandles(130, 100, 0.001, 0.002)
	if _, ok := TF4h.CandlesFor(ctx); !ok {
		t.Error("130 candles reported as insufficient for 4h")
	}
}

// An unpopulated series must be insufficient, never "sufficient and empty".
func TestCandlesFor_NilSeriesIsInsufficient(t *testing.T) {
	for _, tf := range []HigherTF{TF15m, TF30m, TF1h, TF4h, TF1d} {
		if c, ok := tf.CandlesFor(MarketContext{}); ok || len(c) != 0 {
			t.Errorf("%s reported nil candles as usable", tf)
		}
	}
}

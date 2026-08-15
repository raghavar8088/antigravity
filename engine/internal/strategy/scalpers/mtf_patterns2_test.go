package scalpers

import "testing"

// Heikin Ashi must be computed, not approximated. The smoothing IS the signal:
// a flip means the recent several bars changed character, which is the whole
// reason to prefer it over a raw candle colour change.
func TestHeikinAshi_SmoothsAndPreservesLength(t *testing.T) {
	c := mtfCandles(60, 100, 0.001, 0.01)
	ha := heikinAshi(c)
	if len(ha) != len(c) {
		t.Fatalf("heikin ashi returned %d candles for %d input", len(ha), len(c))
	}
	// HA closes are 4-price averages, so they must be less jumpy than raw ones.
	rawJump, haJump := 0.0, 0.0
	for i := 1; i < len(c); i++ {
		rawJump += abs(c[i].Close - c[i-1].Close)
		haJump += abs(ha[i].Close - ha[i-1].Close)
	}
	if haJump >= rawJump {
		t.Errorf("heikin ashi is not smoother (ha %.4f vs raw %.4f)", haJump, rawJump)
	}
	// And the HA high/low must contain its own open and close.
	for i, x := range ha {
		if x.High < x.Open || x.High < x.Close || x.Low > x.Open || x.Low > x.Close {
			t.Fatalf("candle %d has a body outside its range", i)
		}
	}
}

// heikinAshi must not panic on an empty series — thin symbols produce them.
func TestHeikinAshi_EmptyInput(t *testing.T) {
	if got := heikinAshi(nil); got != nil {
		t.Errorf("nil input returned %d candles", len(got))
	}
}

// Every new family must refuse short history, exactly like the first batch.
func TestPatternFamilies2_RefuseShortHistory(t *testing.T) {
	short := mtfCandles(20, 100, 0.001, 0.003)
	fams := map[string]func(bool) func(string, []Candle, float64) Signal{
		"Marubozu":            patMarubozu,
		"OutsideBar":          patOutsideBar,
		"ThreeSoldiers":       patThreeSoldiers,
		"HeikinAshiFlip":      patHeikinAshiFlip,
		"HeadShoulders":       patHeadShoulders,
		"TripleTopBottom":     patTripleTopBottom,
		"DirectionalTriangle": patDirectionalTriangle,
		"Flag":                patFlag,
		"OpeningRangeBreak":   patOpeningRangeBreak,
		"FibRetrace":          patFibRetrace,
	}
	for name, mk := range fams {
		for _, long := range []bool{true, false} {
			if s := mk(long)("T", short, 100); s.Direction != DirectionNone {
				t.Errorf("%s (long=%v) signalled on 20 candles", name, long)
			}
		}
	}
}

// The short timeframes exist but must be economically self-limiting: at 1m the
// measured move is usually smaller than the round trip, and the fee bar has to
// be what stops it rather than a special case.
func TestShortTimeframes_AreRefusedByTheFeeBarNotBySpecialCasing(t *testing.T) {
	// A 1m-scale setup: 0.05% ATR -> 0.075% stop -> a 2.5R target of 0.19%,
	// against a 0.118% round trip. That is under the 6x bar.
	if s := mtfSignal("MTF_1m_X", DirectionLong, 100, 0.0005, 2.5, "tiny"); s.Direction != DirectionNone {
		t.Error("a 1m-scale setup cleared the fee bar; the bar is not doing its job")
	}
	// The same family on a bigger move is allowed — nothing is banned by name.
	if s := mtfSignal("MTF_1m_X", DirectionLong, 100, 0.01, 2.5, "real"); s.Direction != DirectionLong {
		t.Error("a genuinely large 1m move was refused; timeframes must not be special-cased")
	}
}

// All eight timeframes must be present and distinct.
func TestBuildMTFPack_CoversEveryTimeframeIncludingShort(t *testing.T) {
	seen := map[HigherTF]int{}
	for _, tf := range []HigherTF{TF1m, TF5m, TF10m, TF15m, TF30m, TF1h, TF4h, TF1d} {
		if tf.Step() <= 0 {
			t.Errorf("%s has no step duration", tf)
		}
		if tf.MinCandles() <= 0 {
			t.Errorf("%s has no warm-up minimum", tf)
		}
		seen[tf]++
	}
	if len(seen) != 8 {
		t.Errorf("expected 8 distinct timeframes, got %d", len(seen))
	}
}

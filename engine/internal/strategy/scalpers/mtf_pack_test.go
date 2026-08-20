package scalpers

import (
	"math"
	"testing"
	"time"
)

// Build a synthetic series with a controllable trend and range.
func mtfCandles(n int, start, driftPct, noisePct float64) []Candle {
	out := make([]Candle, 0, n)
	px := start
	t := time.Now().UTC().Add(-time.Duration(n) * time.Hour)
	for i := 0; i < n; i++ {
		px *= 1 + driftPct
		wob := px * noisePct
		// Deterministic alternation: no RNG, so a failure is reproducible.
		if i%2 == 0 {
			wob = -wob
		}
		o := px
		cl := px + wob
		hi := math.Max(o, cl) + px*noisePct*0.5
		lo := math.Min(o, cl) - px*noisePct*0.5
		out = append(out, Candle{
			Open: o, High: hi, Low: lo, Close: cl,
			Volume: 1000, OpenTime: t.Add(time.Duration(i) * time.Hour),
		})
	}
	return out
}

// The pack must cover every timeframe and family, in both directions.
func TestBuildMTFPack_CoversEveryTimeframe(t *testing.T) {
	p := BuildMTFPack()
	// 42 families on 9 timeframes, both sides.
	//
	// Was 26 families x 8 timeframes. 2026-08-17 added 45m and fifteen families
	// completing the chart / candlestick / price-structure catalogue: Wedge,
	// Pennant, CupHandle, Rounding, Broadening, Diamond, Hammer, Keltner,
	// PriorSessionBreak, RoundNumber, TTMSqueeze, EMARibbon, ATRThrust,
	// PivotBreak and GapFade.
	if len(p) != 9*42*2 {
		t.Fatalf("pack has %d strategies, want %d (9 timeframes x 42 families x 2 sides)", len(p), 9*42*2)
	}
	seen := map[string]bool{}
	for _, e := range p {
		n := e.Strategy.Name()
		if seen[n] {
			t.Errorf("duplicate strategy name %s", n)
		}
		seen[n] = true
	}
	for _, want := range []string{
		"MTF_15m_TrendPullback_Long", "MTF_1d_Breakout55_Short", "MTF_4h_BollingerFade_Long",
		"MTF_1h_Engulfing_Long", "MTF_4h_PinBar_Short", "MTF_1d_DoubleTopBottom_Long",
		"MTF_30m_StructureBreak_Short", "MTF_15m_LevelRetest_Long", "MTF_1h_TriangleBreak_Short",
		"MTF_5m_HeadShoulders_Short", "MTF_1m_Marubozu_Long", "MTF_10m_Flag_Long",
		"MTF_4h_HeikinAshiFlip_Short", "MTF_1d_TripleTopBottom_Long", "MTF_30m_FibRetrace_Long",
		// The 2026-08-17 additions, including the new 45m timeframe.
		"MTF_45m_TrendPullback_Long", "MTF_45m_Wedge_Short", "MTF_1h_CupHandle_Long",
		"MTF_4h_Diamond_Short", "MTF_1d_Broadening_Long", "MTF_15m_Hammer_Short",
		"MTF_30m_Keltner_Long", "MTF_1h_PriorSessionBreak_Short", "MTF_5m_RoundNumber_Long",
		"MTF_4h_TTMSqueeze_Short", "MTF_1h_EMARibbon_Long", "MTF_15m_ATRThrust_Short",
		"MTF_1d_PivotBreak_Long", "MTF_5m_GapFade_Short", "MTF_45m_Pennant_Long",
		"MTF_45m_Rounding_Short",
	} {
		if !seen[want] {
			t.Errorf("missing %s", want)
		}
	}
}

// A strategy with too little history must return NO signal, not a signal built
// from a short window. Indicators warm up; a 55-period EMA over 30 candles is a
// fast average wearing a slow name.
func TestMTFPack_RefusesShortHistory(t *testing.T) {
	p := BuildMTFPack()
	ctx := MarketContext{Price: 100, Candles1h: mtfCandles(10, 100, 0.001, 0.002)}
	for _, e := range p {
		if s := e.Strategy.Evaluate(ctx); s.Direction != DirectionNone {
			t.Fatalf("%s signalled on 10 candles: %s", e.Strategy.Name(), s.Reason)
		}
	}
}

// The fee bar must actually refuse trades. A setup reaching for less than 6
// round trips is a commission with a coin flip attached, and the 1m roster
// proved that at scale.
func TestMTFSignal_RefusesTargetsThatCannotClearFees(t *testing.T) {
	// ATR of 0.02% -> stop 0.03%, target at 2.5R is 0.075% — under the
	// 0.118% x 6 bar.
	if s := mtfSignal("X", DirectionLong, 100, 0.0002, 2.5, "tiny"); s.Direction != DirectionNone {
		t.Errorf("accepted a target of %.4f%% against a %.3f%% fee", (s.TakeProfit/100-1)*100, roundTripFeePct)
	}
	// A real move: 1% ATR -> 1.5% stop -> 3.75% target, comfortably clear.
	s := mtfSignal("X", DirectionLong, 100, 0.01, 2.5, "real")
	if s.Direction != DirectionLong {
		t.Fatal("refused a setup reaching 3.75%")
	}
	if s.StopLoss >= 100 || s.TakeProfit <= 100 {
		t.Errorf("long levels inverted: stop %.4f target %.4f", s.StopLoss, s.TakeProfit)
	}
}

// Shorts must mirror: stop above, target below.
func TestMTFSignal_ShortLevels(t *testing.T) {
	s := mtfSignal("X", DirectionShort, 100, 0.01, 2.5, "real")
	if s.Direction != DirectionShort {
		t.Fatal("short refused")
	}
	if s.StopLoss <= 100 || s.TakeProfit >= 100 {
		t.Errorf("short levels inverted: stop %.4f target %.4f", s.StopLoss, s.TakeProfit)
	}
}

// The stop must sit outside ordinary candle noise — that was the single
// clearest cause of the 1m roster's 55% SL rate at a 130-second median hold.
func TestMTFSignal_StopIsOutsideOneCandleOfNoise(t *testing.T) {
	atr := 0.01 // 1% average true range
	s := mtfSignal("X", DirectionLong, 100, atr, 2.5, "x")
	stopDist := (100 - s.StopLoss) / 100
	if stopDist <= atr {
		t.Errorf("stop %.4f%% is inside a single ATR of %.4f%% — noise will resolve it", stopDist*100, atr*100)
	}
}

// No mirrors. The ANTI_ inversion is arithmetically broken under fees, and
// reintroducing it would repeat the roster that produced 25-29% win rates.
func TestBuildMTFPack_ContainsNoMirrors(t *testing.T) {
	for _, e := range BuildMTFPack() {
		n := e.Strategy.Name()
		if len(n) >= 5 && n[:5] == "ANTI_" {
			t.Errorf("%s is a mirror; the inversion premise does not survive fees", n)
		}
	}
}

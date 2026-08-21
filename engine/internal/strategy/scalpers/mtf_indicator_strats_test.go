package scalpers

import "testing"

func indicatorFamilies() map[string]func(bool) func(string, []Candle, float64) Signal {
	return map[string]func(bool) func(string, []Candle, float64) Signal{
		"Supertrend":      stratSupertrend,
		"Ichimoku":        stratIchimoku,
		"ParabolicSAR":    stratPSAR,
		"Aroon":           stratAroon,
		"Vortex":          stratVortex,
		"HMAFlip":         stratHMAFlip,
		"TEMACross":       stratTEMACross,
		"KAMATrend":       stratKAMATrend,
		"GoldenCross":     stratGoldenCross,
		"CCIExtreme":      stratCCI(),
		"WilliamsR":       stratWilliamsR(),
		"CMOExtreme":      stratCMO(),
		"MFIExtreme":      stratMFI(),
		"StochCross":      stratStochCross,
		"FisherReversal":  stratFisher,
		"TSICross":        stratTSICross,
		"TRIXCross":       stratTRIXCross,
		"MomentumBurst":   stratMomentumBurst,
		"OBVBreak":        stratOBVBreak,
		"CMFConfirm":      stratCMFConfirm,
		"ZScoreReversion": stratZScore,
		"LinRegBreak":     stratLinRegBreak,
		"LinRegFade":      stratLinRegFade,
	}
}

func TestIndicatorRefuseShortHistory(t *testing.T) {
	for name, mk := range indicatorFamilies() {
		for _, n := range []int{0, 1, 15, 40, 55} {
			c := randomWalk(n, 100, 0.01, 6)
			px := 100.0
			if n > 0 {
				px = c[len(c)-1].Close
			}
			for _, long := range []bool{true, false} {
				if sig := mk(long)(name, c, px); sig.Direction != DirectionNone {
					t.Errorf("%s(long=%v) signalled on %d candles — must refuse", name, long, n)
				}
			}
		}
	}
}

func TestIndicatorCoherentSignals(t *testing.T) {
	for name, mk := range indicatorFamilies() {
		for seed := int64(1); seed <= 50; seed++ {
			for _, vol := range []float64{0.004, 0.015, 0.035} {
				c := randomWalk(260, 100, vol, seed)
				px := c[len(c)-1].Close
				for _, long := range []bool{true, false} {
					sig := mk(long)(name, c, px)
					if sig.Direction == DirectionNone {
						continue
					}
					if sig.StopLoss <= 0 || sig.TakeProfit <= 0 {
						t.Fatalf("%s: non-positive levels sl=%f tp=%f", name, sig.StopLoss, sig.TakeProfit)
					}
					if sig.Direction == DirectionLong && (sig.StopLoss >= px || sig.TakeProfit <= px) {
						t.Fatalf("%s LONG: incoherent sl=%f tp=%f entry=%f", name, sig.StopLoss, sig.TakeProfit, px)
					}
					if sig.Direction == DirectionShort && (sig.StopLoss <= px || sig.TakeProfit >= px) {
						t.Fatalf("%s SHORT: incoherent sl=%f tp=%f entry=%f", name, sig.StopLoss, sig.TakeProfit, px)
					}
				}
			}
		}
	}
}

// REGIME GATING is the property that makes these families worth running
// together rather than against each other. A mean-reversion family must not
// fire in a strong trend — that is the behaviour that fades a breakout the whole
// way up.
func TestMeanReversionRefusesInATrend(t *testing.T) {
	// A clean, persistent uptrend: choppiness low, oscillators pinned high.
	c := flat(60, 100, 0.2, 1000)
	c = ramp(c, 120, 100, 40, 1000)

	if rangeRegime(c) {
		t.Fatal("a monotonic 40% advance was classified as a RANGE — the gate is inverted or broken")
	}
	if !trendRegime(c) {
		t.Fatal("a monotonic 40% advance was not classified as a TREND")
	}

	meanReversion := map[string]func(bool) func(string, []Candle, float64) Signal{
		"CCIExtreme":      stratCCI(),
		"WilliamsR":       stratWilliamsR(),
		"CMOExtreme":      stratCMO(),
		"MFIExtreme":      stratMFI(),
		"StochCross":      stratStochCross,
		"FisherReversal":  stratFisher,
		"ZScoreReversion": stratZScore,
	}
	px := lastPx(c)
	for name, mk := range meanReversion {
		for _, long := range []bool{true, false} {
			if sig := mk(long)(name, c, px); sig.Direction != DirectionNone {
				t.Errorf("%s fired in a trending regime — mean reversion must be gated off there", name)
			}
		}
	}
}

// And the converse: a trend family must not fire in a chop. Vortex and
// MomentumBurst carry explicit trend gates.
func TestTrendFamiliesRefuseInAChop(t *testing.T) {
	// A tight oscillation: choppiness high.
	c := flat(200, 100, 0.6, 1000)
	if !rangeRegime(c) {
		t.Skip("fixture did not produce a ranging regime; choppiness is data-dependent")
	}
	px := lastPx(c)
	for name, mk := range map[string]func(bool) func(string, []Candle, float64) Signal{
		"Vortex":        stratVortex,
		"MomentumBurst": stratMomentumBurst,
	} {
		for _, long := range []bool{true, false} {
			if sig := mk(long)(name, c, px); sig.Direction != DirectionNone {
				t.Errorf("%s fired in a ranging regime — it carries a trend gate", name)
			}
		}
	}
}

// An UNKNOWN regime is not permission. If choppiness cannot be computed, the
// gated families must refuse rather than default to firing.
func TestUnknownRegimeIsNotPermission(t *testing.T) {
	short := randomWalk(10, 100, 0.01, 2)
	if rangeRegime(short) || trendRegime(short) {
		t.Fatal("a regime was reported from 10 candles — neither gate may pass on an unknown regime")
	}
}

// The indicator maths must refuse a short window rather than return a confident
// number computed over half its period.
func TestIndicatorsRefuseShortWindows(t *testing.T) {
	c := randomWalk(8, 100, 0.01, 1)
	checks := map[string]bool{}
	_, checks["WMA"] = mtfWMA(c, 20)
	_, checks["HMA"] = mtfHMA(c, 21)
	_, checks["DEMA"] = mtfDEMA(c, 14)
	_, checks["TEMA"] = mtfTEMA(c, 14)
	_, checks["KAMA"] = mtfKAMA(c, 20)
	_, _, checks["Supertrend"] = mtfSupertrend(c, 10, 3)
	_, _, checks["PSAR"] = mtfPSAR(c, 0.02, 0.2)
	_, _, _, _, checks["Ichimoku"] = mtfIchimoku(c)
	_, _, checks["Aroon"] = mtfAroon(c, 25)
	_, _, checks["Vortex"] = mtfVortex(c, 14)
	_, checks["CCI"] = mtfCCI(c, 20)
	_, checks["WilliamsR"] = mtfWilliamsR(c, 14)
	_, _, checks["Stochastic"] = mtfStochastic(c, 14, 3)
	_, checks["CMO"] = mtfCMO(c, 14)
	_, checks["ROC"] = mtfROC(c, 10)
	_, checks["TSI"] = mtfTSI(c, 25, 13)
	_, checks["TRIX"] = mtfTRIX(c, 15)
	_, _, checks["Fisher"] = mtfFisher(c, 10)
	_, _, checks["OBV"] = mtfOBV(c, 20)
	_, checks["MFI"] = mtfMFI(c, 14)
	_, checks["CMF"] = mtfCMF(c, 20)
	_, checks["ZScore"] = mtfZScore(c, 20)
	_, _, _, checks["LinReg"] = mtfLinReg(c, 40)
	_, checks["Choppiness"] = mtfChoppiness(c, 14)
	for name, ok := range checks {
		if ok {
			t.Errorf("%s returned a value from 8 candles — it must refuse instead", name)
		}
	}
}

// Spot-check the maths against values that can be reasoned about by hand, so a
// silently wrong formula does not hide behind "it compiles and returns a float".
func TestIndicatorSanity(t *testing.T) {
	// A perfectly monotonic rise.
	up := ramp(flat(60, 100, 0.05, 1000), 60, 100, 30, 1000)

	if r, ok := mtfWilliamsR(up, 14); !ok || r > -20 == false {
		t.Errorf("Williams %%R in a strong uptrend = %.2f, expected near 0 (overbought)", r)
	}
	if cmo, ok := mtfCMO(up, 14); !ok || cmo < 90 {
		t.Errorf("CMO in a monotonic rise = %.2f, expected near +100", cmo)
	}
	if _, up2, ok := mtfPSAR(up, 0.02, 0.2); !ok || !up2 {
		t.Error("PSAR did not report a rising regime in a monotonic advance")
	}
	if aUp, aDown, ok := mtfAroon(up, 25); !ok || aUp < 90 || aDown > 20 {
		t.Errorf("Aroon in a monotonic rise = up %.0f down %.0f, expected ~100/~0", aUp, aDown)
	}
	if _, slope, _, ok := mtfLinReg(up, 40); !ok || slope <= 0 {
		t.Errorf("regression slope in a rise = %.6f, expected positive", slope)
	}
	if ch, ok := mtfChoppiness(up, 14); !ok || ch > chopTrendMax {
		t.Errorf("choppiness in a monotonic rise = %.1f, expected trending (<= %.1f)", ch, chopTrendMax)
	}
	if z, ok := mtfZScore(up, 20); !ok || z <= 0 {
		t.Errorf("z-score at the top of a rise = %.2f, expected positive", z)
	}
}

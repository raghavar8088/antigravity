package scalpers

import "fmt"

// S10 — SMC Order Block + Fair Value Gap
//
// Regime:     TRENDING, RANGING only (VOLATILE excluded — SMC structure
//             needs clean, non-noisy price action)
// Timeframes: 15m (structure: order blocks, FVGs, market structure shift)
// Logic:      A market structure shift (MSS) confirms a directional break.
//             Price retests an unfilled fair value gap (FVG) left by the
//             impulse leg, stacked against a nearby order block (OB) on the
//             same side, with CVD confirming participation.
// Edge:       Trades with institutional order flow (OB) at a high-probability
//             FVG retest rather than chasing the breakout candle itself.

type SMCOrderBlockFVG struct{}

func (s *SMCOrderBlockFVG) Name() string { return "SMC_OrderBlock_FVG" }

func (s *SMCOrderBlockFVG) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

// fvgZone is a fair value gap price range.
type fvgZone struct {
	Low, High float64
	Found     bool
}

// detectBullishFVG scans the last 20 15m candles for the most recent unfilled
// bullish FVG: candle[i].High < candle[i+2].Low. The zone is
// [candle[i].High, candle[i+2].Low]. A gap is "filled" once any later candle
// has closed inside the zone.
func detectBullishFVG(c15 []Candle) fvgZone {
	n := len(c15)
	lookback := 20
	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := n - 3; i >= start; i-- {
		a := c15[i]
		c := c15[i+2]
		if a.High >= c.Low {
			continue
		}
		zoneLow, zoneHigh := a.High, c.Low
		filled := false
		for j := i + 3; j < n; j++ {
			if c15[j].Close >= zoneLow && c15[j].Close <= zoneHigh {
				filled = true
				break
			}
		}
		if !filled {
			return fvgZone{Low: zoneLow, High: zoneHigh, Found: true}
		}
	}
	return fvgZone{}
}

// detectBearishFVG scans the last 20 15m candles for the most recent unfilled
// bearish FVG: candle[i].Low > candle[i+2].High. The zone is
// [candle[i+2].High, candle[i].Low].
func detectBearishFVG(c15 []Candle) fvgZone {
	n := len(c15)
	lookback := 20
	start := n - lookback
	if start < 0 {
		start = 0
	}
	for i := n - 3; i >= start; i-- {
		a := c15[i]
		c := c15[i+2]
		if a.Low <= c.High {
			continue
		}
		zoneLow, zoneHigh := c.High, a.Low
		filled := false
		for j := i + 3; j < n; j++ {
			if c15[j].Close >= zoneLow && c15[j].Close <= zoneHigh {
				filled = true
				break
			}
		}
		if !filled {
			return fvgZone{Low: zoneLow, High: zoneHigh, Found: true}
		}
	}
	return fvgZone{}
}

// detectBullishOB scans the 3 most recent candidate windows for a bullish
// order block: a bearish candle (close<open) immediately followed by 2
// consecutive bullish candles where the second's close exceeds the bearish
// candle's open. Returns the most recent qualifying zone [Low, High].
func detectBullishOB(c15 []Candle) (low, high float64, found bool) {
	n := len(c15)
	if n < 3 {
		return 0, 0, false
	}
	for w := 0; w < 3; w++ {
		i := n - 3 - w
		if i < 0 {
			break
		}
		ob := c15[i]
		c1 := c15[i+1]
		c2 := c15[i+2]
		if ob.Close >= ob.Open {
			continue // candidate must be bearish
		}
		if c1.Close > c1.Open && c2.Close > c2.Open && c2.Close > ob.Open {
			return ob.Low, ob.High, true
		}
	}
	return 0, 0, false
}

// detectBearishOB mirrors detectBullishOB: a bullish candle followed by 2
// consecutive bearish candles where the second's close goes below the
// bullish candle's open.
func detectBearishOB(c15 []Candle) (low, high float64, found bool) {
	n := len(c15)
	if n < 3 {
		return 0, 0, false
	}
	for w := 0; w < 3; w++ {
		i := n - 3 - w
		if i < 0 {
			break
		}
		ob := c15[i]
		c1 := c15[i+1]
		c2 := c15[i+2]
		if ob.Close <= ob.Open {
			continue // candidate must be bullish
		}
		if c1.Close < c1.Open && c2.Close < c2.Open && c2.Close < ob.Open {
			return ob.Low, ob.High, true
		}
	}
	return 0, 0, false
}

// obStackedBelowFVG returns true when a bullish OB zone overlaps the bullish
// FVG zone, or sits within ATR*0.5 directly below it (stacked confluence).
func obStackedBelowFVG(obLow, obHigh, fvgLow, fvgHigh, atr float64) bool {
	overlap := obLow <= fvgHigh && obHigh >= fvgLow
	gap := fvgLow - obHigh
	withinBelow := gap >= 0 && gap <= 0.5*atr
	return overlap || withinBelow
}

// obStackedAboveFVG mirrors obStackedBelowFVG for bearish setups: the
// bearish OB overlaps the bearish FVG, or sits within ATR*0.5 directly above it.
func obStackedAboveFVG(obLow, obHigh, fvgLow, fvgHigh, atr float64) bool {
	overlap := obLow <= fvgHigh && obHigh >= fvgLow
	gap := obLow - fvgHigh
	withinAbove := gap >= 0 && gap <= 0.5*atr
	return overlap || withinAbove
}

func (s *SMCOrderBlockFVG) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	// Market structure shift: break of the most recent swing (excluding the
	// still-forming current bar) confirms directional intent.
	prior := c15[:len(c15)-1]
	swingHigh := SwingHigh(prior, 10)
	swingLow := SwingLow(prior, 10)
	bullishMSS := swingHigh > 0 && price > swingHigh
	bearishMSS := swingLow > 0 && price < swingLow

	cvdHist := ctx.CVDHistory
	cvdRising3 := len(cvdHist) >= 3 &&
		cvdHist[len(cvdHist)-1] > cvdHist[len(cvdHist)-2] &&
		cvdHist[len(cvdHist)-2] > cvdHist[len(cvdHist)-3]
	cvdFalling3 := len(cvdHist) >= 3 &&
		cvdHist[len(cvdHist)-1] < cvdHist[len(cvdHist)-2] &&
		cvdHist[len(cvdHist)-2] < cvdHist[len(cvdHist)-3]

	if bullishMSS {
		fvg := detectBullishFVG(c15)
		if fvg.Found {
			retest := price >= fvg.Low-0.2*atr && price <= fvg.High+0.2*atr
			obLow, obHigh, obFound := detectBullishOB(c15)
			stacked := obFound && obStackedBelowFVG(obLow, obHigh, fvg.Low, fvg.High, atr)
			cvdOK := ctx.CVD > 0 || cvdRising3

			if retest && stacked && cvdOK {
				sl := obLow - 0.5*atr
				tp := fvg.High + 1.5*atr
				tp2 := fvg.High + 3.0*atr
				riskDist := price - sl
				rewardDist := tp - price
				if riskDist < 0.0025*price || riskDist <= 0 || rewardDist/riskDist < 2.0 {
					return NoSignal(name)
				}
				conf := 0.72
				if cvdRising3 {
					conf += 0.05
				}
				if ctx.OrderBook.Imbalance > 0.1 {
					conf += 0.05
				}
				if conf > 0.90 {
					conf = 0.90
				}
				return Signal{
					Strategy:    name,
					Direction:   DirectionLong,
					Confidence:  conf,
					StopLoss:    sl,
					TakeProfit:  tp,
					TakeProfit2: tp2,
					Reason: fmt.Sprintf(
						"%s: bullish MSS above %.0f, FVG retest [%.0f,%.0f], stacked bullish OB [%.0f,%.0f], CVD=%.0f",
						ctx.Regime, swingHigh, fvg.Low, fvg.High, obLow, obHigh, ctx.CVD,
					),
				}
			}
		}
	}

	if bearishMSS {
		fvg := detectBearishFVG(c15)
		if fvg.Found {
			retest := price >= fvg.Low-0.2*atr && price <= fvg.High+0.2*atr
			obLow, obHigh, obFound := detectBearishOB(c15)
			stacked := obFound && obStackedAboveFVG(obLow, obHigh, fvg.Low, fvg.High, atr)
			cvdOK := ctx.CVD < 0 || cvdFalling3

			if retest && stacked && cvdOK {
				sl := obHigh + 0.5*atr
				tp := fvg.Low - 1.5*atr
				tp2 := fvg.Low - 3.0*atr
				riskDist := sl - price
				rewardDist := price - tp
				if riskDist < 0.0025*price || riskDist <= 0 || rewardDist/riskDist < 2.0 {
					return NoSignal(name)
				}
				conf := 0.72
				if cvdFalling3 {
					conf += 0.05
				}
				if ctx.OrderBook.Imbalance < -0.1 {
					conf += 0.05
				}
				if conf > 0.90 {
					conf = 0.90
				}
				return Signal{
					Strategy:    name,
					Direction:   DirectionShort,
					Confidence:  conf,
					StopLoss:    sl,
					TakeProfit:  tp,
					TakeProfit2: tp2,
					Reason: fmt.Sprintf(
						"%s: bearish MSS below %.0f, FVG retest [%.0f,%.0f], stacked bearish OB [%.0f,%.0f], CVD=%.0f",
						ctx.Regime, swingLow, fvg.Low, fvg.High, obLow, obHigh, ctx.CVD,
					),
				}
			}
		}
	}

	return NoSignal(name)
}

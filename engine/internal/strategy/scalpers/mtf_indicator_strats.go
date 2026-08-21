package scalpers

import (
	"fmt"
	"math"
)

// mtf_indicator_strats.go — the indicator families: trend followers, oscillator
// reversals, volume confirmation, and the statistical mean-reversion set.
//
// REGIME GATING is applied here rather than left to the operator. The
// Choppiness Index is the gate: mean-reversion families are allowed to fire
// only when it is HIGH (a range), breakout and trend families only when it is
// LOW (a trend). Without it, a mean-reversion strategy fades every breakout and
// a trend strategy buys every range high — the two lose money in precisely the
// conditions the other one wants, and running both ungated produces a desk
// whose net result is the fee bill.
//
// UNITS: mtfATR returns ATR as a FRACTION OF PRICE.

// Choppiness thresholds. 61.8/38.2 are the conventional Fibonacci-derived
// boundaries; they are stated once here so every family in the file agrees on
// what "a range" means.
const (
	chopRangeMin = 61.8 // at or above this, the market is ranging
	chopTrendMax = 38.2 // at or below this, the market is trending
)

// rangeRegime and trendRegime are the gates. A family that cannot determine the
// regime does not fire: an unknown regime is not permission.
func rangeRegime(c []Candle) bool {
	ch, ok := mtfChoppiness(c, 14)
	return ok && ch >= chopRangeMin
}

func trendRegime(c []Candle) bool {
	ch, ok := mtfChoppiness(c, 14)
	return ok && ch <= chopTrendMax
}

// ── trend followers ──────────────────────────────────────────────────────────

// stratSupertrend: enter on the FLIP, not while the regime persists.
//
// The flip is a discrete event; "price is above the Supertrend" is true for most
// of a trend and would have the desk entering at every bar of it.
func stratSupertrend(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		_, upNow, ok1 := mtfSupertrend(c, 10, 3.0)
		_, upPrev, ok2 := mtfSupertrend(c[:len(c)-1], 10, 3.0)
		if !ok1 || !ok2 || upNow == upPrev {
			return NoSignal(name)
		}
		if long != upNow {
			return NoSignal(name)
		}
		if long {
			return mtfSignal(name, DirectionLong, price, atr, 2.5, "supertrend flipped up")
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, "supertrend flipped down")
	}
}

// stratIchimoku: a close through the cloud with the conversion/base lines
// agreeing. Both conditions matter — a cloud break against the TK cross is the
// classic false signal.
func stratIchimoku(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		tenkan, kijun, spanA, spanB, ok2 := mtfIchimoku(c)
		if !ok || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		top, bot := math.Max(spanA, spanB), math.Min(spanA, spanB)
		prev := c[len(c)-2].Close
		if long {
			if !(prev <= top && price > top) || tenkan <= kijun {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+(top-bot)+2*atr*price,
				"closed above the cloud with tenkan over kijun")
		}
		if !(prev >= bot && price < bot) || tenkan >= kijun {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-(top-bot)-2*atr*price,
			"closed below the cloud with tenkan under kijun")
	}
}

// stratPSAR: the SAR flipping sides.
func stratPSAR(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		_, nowUp, ok1 := mtfPSAR(c, 0.02, 0.2)
		_, prevUp, ok2 := mtfPSAR(c[:len(c)-1], 0.02, 0.2)
		if !ok1 || !ok2 || nowUp == prevUp || long != nowUp {
			return NoSignal(name)
		}
		if long {
			return mtfSignal(name, DirectionLong, price, atr, 2.0, "parabolic SAR flipped below price")
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.0, "parabolic SAR flipped above price")
	}
}

// stratAroon: Aroon Up crossing Aroon Down, with both extended enough that the
// cross means a new extreme rather than a drift.
func stratAroon(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		up, down, ok2 := mtfAroon(c, 25)
		pUp, pDown, ok3 := mtfAroon(c[:len(c)-1], 25)
		if !ok || !ok2 || !ok3 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(pUp <= pDown && up > down) || up < 70 {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, fmt.Sprintf("aroon up %.0f crossed down %.0f", up, down))
		}
		if !(pDown <= pUp && down > up) || down < 70 {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, fmt.Sprintf("aroon down %.0f crossed up %.0f", down, up))
	}
}

// stratVortex: VI+ crossing VI-, gated to a trending regime because the vortex
// whipsaws continuously in a range.
func stratVortex(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 || !trendRegime(c) {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		p, m, ok2 := mtfVortex(c, 14)
		pp, pm, ok3 := mtfVortex(c[:len(c)-1], 14)
		if !ok || !ok2 || !ok3 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(pp <= pm && p > m) {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, fmt.Sprintf("VI+ %.3f crossed VI- %.3f in a trend", p, m))
		}
		if !(pm <= pp && m > p) {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, fmt.Sprintf("VI- %.3f crossed VI+ %.3f in a trend", m, p))
	}
}

// stratHMAFlip: the Hull turning, which it does earlier than an EMA of the same
// length — the reason it is a separate family and not a parameter.
func stratHMAFlip(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		h0, ok1 := mtfHMA(c, 21)
		h1, ok2 := mtfHMA(c[:len(c)-1], 21)
		h2, ok3 := mtfHMA(c[:len(c)-2], 21)
		if !ok || !ok1 || !ok2 || !ok3 || atr <= 0 {
			return NoSignal(name)
		}
		fallingThenRising := h1 <= h2 && h0 > h1
		risingThenFalling := h1 >= h2 && h0 < h1
		if long {
			if !fallingThenRising || price < h0 {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, "hull moving average turned up")
		}
		if !risingThenFalling || price > h0 {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, "hull moving average turned down")
	}
}

// stratTEMACross: TEMA crossing DEMA. Both reduce lag, so their cross fires
// earlier than an EMA pair and disagrees with it often enough to be its own bet.
func stratTEMACross(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 120 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		t0, ok1 := mtfTEMA(c, 14)
		d0, ok2 := mtfDEMA(c, 14)
		t1, ok3 := mtfTEMA(c[:len(c)-1], 14)
		d1, ok4 := mtfDEMA(c[:len(c)-1], 14)
		if !ok || !ok1 || !ok2 || !ok3 || !ok4 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(t1 <= d1 && t0 > d0) {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, "TEMA crossed above DEMA")
		}
		if !(d1 <= t1 && d0 > t0) {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, "TEMA crossed below DEMA")
	}
}

// stratKAMATrend: price separating from the adaptive average, which only widens
// when the efficiency ratio is high — i.e. when the move is directional rather
// than noisy. That built-in filter is why it needs no separate regime gate.
func stratKAMATrend(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		k0, ok1 := mtfKAMA(c, 20)
		k1, ok2 := mtfKAMA(c[:len(c)-1], 20)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if k0 <= k1 || price <= k0 || price-k0 < 0.5*atr*price {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, "price extending above a rising KAMA")
		}
		if k0 >= k1 || price >= k0 || k0-price < 0.5*atr*price {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, "price extending below a falling KAMA")
	}
}

// stratGoldenCross: the 50/200 crossover, the most famous signal in the
// business and one that fires a handful of times per instrument per year — so
// on the fast timeframes here it is effectively a long-horizon regime marker.
func stratGoldenCross(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 220 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		f0, ok1 := mtfSMA(c, 50)
		s0, ok2 := mtfSMA(c, 200)
		f1, ok3 := mtfSMA(c[:len(c)-1], 50)
		s1, ok4 := mtfSMA(c[:len(c)-1], 200)
		if !ok || !ok1 || !ok2 || !ok3 || !ok4 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(f1 <= s1 && f0 > s0) {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 3.0, "golden cross: SMA50 crossed above SMA200")
		}
		if !(s1 <= f1 && s0 > f0) {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 3.0, "death cross: SMA50 crossed below SMA200")
	}
}

// ── oscillator reversals, gated to RANGE regimes ─────────────────────────────

// oscExtreme builds a mean-reversion family from any bounded oscillator.
//
// All of them share the same failure: in a trend, an oscillator sits at its
// extreme for the whole move and fades it the entire way. So every family built
// here is gated to a ranging regime, and the gate is the reason these are worth
// running at all.
func oscExtreme(
	label string,
	read func([]Candle) (float64, bool),
	lowThresh, highThresh float64,
) func(bool) func(string, []Candle, float64) Signal {
	return func(long bool) func(string, []Candle, float64) Signal {
		return func(name string, c []Candle, price float64) Signal {
			if len(c) < 80 || !rangeRegime(c) {
				return NoSignal(name)
			}
			atr, ok := mtfATR(c, 14)
			v, ok1 := read(c)
			pv, ok2 := read(c[:len(c)-1])
			if !ok || !ok1 || !ok2 || atr <= 0 {
				return NoSignal(name)
			}
			if long {
				// Turning UP out of oversold, not merely sitting there.
				if pv > lowThresh || v <= pv {
					return NoSignal(name)
				}
				return mtfSignal(name, DirectionLong, price, atr, 2.0,
					fmt.Sprintf("%s turning up from %.1f in a range", label, pv))
			}
			if pv < highThresh || v >= pv {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionShort, price, atr, 2.0,
				fmt.Sprintf("%s turning down from %.1f in a range", label, pv))
		}
	}
}

func stratCCI() func(bool) func(string, []Candle, float64) Signal {
	return oscExtreme("CCI", func(c []Candle) (float64, bool) { return mtfCCI(c, 20) }, -100, 100)
}

func stratWilliamsR() func(bool) func(string, []Candle, float64) Signal {
	return oscExtreme("Williams %R", func(c []Candle) (float64, bool) { return mtfWilliamsR(c, 14) }, -80, -20)
}

func stratCMO() func(bool) func(string, []Candle, float64) Signal {
	return oscExtreme("CMO", func(c []Candle) (float64, bool) { return mtfCMO(c, 14) }, -50, 50)
}

func stratMFI() func(bool) func(string, []Candle, float64) Signal {
	return oscExtreme("MFI", func(c []Candle) (float64, bool) { return mtfMFI(c, 14) }, 20, 80)
}

// stratStochCross: %K crossing %D inside the extreme zones. The cross is the
// trigger and the zone is the filter; a cross in mid-range is noise.
func stratStochCross(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 || !rangeRegime(c) {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		k0, d0, ok1 := mtfStochastic(c, 14, 3)
		k1, d1, ok2 := mtfStochastic(c[:len(c)-1], 14, 3)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(k1 <= d1 && k0 > d0) || k1 > 25 {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.0, fmt.Sprintf("stochastic %%K crossed %%D at %.0f", k1))
		}
		if !(d1 <= k1 && d0 > k0) || k1 < 75 {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.0, fmt.Sprintf("stochastic %%K crossed %%D at %.0f", k1))
	}
}

// stratFisher: the Fisher Transform reversing at an extreme. Its sharp turns are
// the point — it is designed to make reversals discrete rather than gradual.
func stratFisher(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 || !rangeRegime(c) {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		f, prev, ok1 := mtfFisher(c, 10)
		if !ok || !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if prev > -1.5 || f <= prev {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.0, fmt.Sprintf("fisher turned up from %.2f", prev))
		}
		if prev < 1.5 || f >= prev {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.0, fmt.Sprintf("fisher turned down from %.2f", prev))
	}
}

// ── momentum, gated to TREND regimes ─────────────────────────────────────────

// stratTSICross: TSI crossing its zero line — a momentum regime change rather
// than an overbought reading.
func stratTSICross(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 120 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		t0, ok1 := mtfTSI(c, 25, 13)
		t1, ok2 := mtfTSI(c[:len(c)-1], 25, 13)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(t1 <= 0 && t0 > 0) {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, fmt.Sprintf("TSI crossed zero to %.1f", t0))
		}
		if !(t1 >= 0 && t0 < 0) {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, fmt.Sprintf("TSI crossed zero to %.1f", t0))
	}
}

// stratTRIXCross: TRIX crossing zero. Triple smoothing makes it slow and
// relatively few-signal, which is the trade being made.
func stratTRIXCross(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 120 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		t0, ok1 := mtfTRIX(c, 15)
		t1, ok2 := mtfTRIX(c[:len(c)-1], 15)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if !(t1 <= 0 && t0 > 0) {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5, "TRIX crossed above zero")
		}
		if !(t1 >= 0 && t0 < 0) {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5, "TRIX crossed below zero")
	}
}

// stratMomentumBurst: rate of change AND volume together.
//
// Either alone is a poor signal — price can jump on nothing, and volume can
// spike without direction. The conjunction is the strategy, which is why this
// is one family rather than a ROC family and a volume family.
func stratMomentumBurst(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 || !trendRegime(c) {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		roc, ok1 := mtfROC(c, 10)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		thresh := atr * 100 * 2 // two ATR of movement, in percent
		if vr < 1.8 {
			return NoSignal(name)
		}
		if long {
			if roc < thresh {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				fmt.Sprintf("momentum burst %.2f%% over 10 bars on %.1fx volume", roc, vr))
		}
		if roc > -thresh {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			fmt.Sprintf("momentum burst %.2f%% over 10 bars on %.1fx volume", roc, vr))
	}
}

// ── volume ───────────────────────────────────────────────────────────────────

// stratOBVBreak: OBV making a new extreme while price has not — accumulation
// showing up before it reaches the tape.
func stratOBVBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		now, then, ok1 := mtfOBV(c, 20)
		if !ok || !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		hi, lo, ok2 := mtfDonchian(c[:len(c)-1], 20)
		if !ok2 {
			return NoSignal(name)
		}
		if long {
			// OBV up strongly while price has NOT yet cleared its range high.
			if now <= then || price >= hi || price < lo+(hi-lo)*0.5 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, hi+(hi-lo)*0.5,
				"OBV rising into the top of the range before price breaks it")
		}
		if now >= then || price <= lo || price > hi-(hi-lo)*0.5 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, lo-(hi-lo)*0.5,
			"OBV falling into the bottom of the range before price breaks it")
	}
}

// stratCMFConfirm: Chaikin Money Flow agreeing with a breakout. A confirmation
// family: it does not find setups, it refuses the ones the tape disagrees with.
func stratCMFConfirm(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		cmf, ok1 := mtfCMF(c, 20)
		hi, lo, ok2 := mtfDonchian(c[:len(c)-1], 20)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if price <= hi || cmf < 0.10 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+(hi-lo)*0.6,
				fmt.Sprintf("range high broken with CMF %.2f confirming", cmf))
		}
		if price >= lo || cmf > -0.10 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-(hi-lo)*0.6,
			fmt.Sprintf("range low broken with CMF %.2f confirming", cmf))
	}
}

// ── statistical ──────────────────────────────────────────────────────────────

// stratZScore: fade a statistically extreme deviation from the rolling mean.
//
// Gated to a range, because in a trend the z-score is extreme for the whole
// move and fading it is how a mean-reversion book gives back a year.
func stratZScore(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 || !rangeRegime(c) {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		z, ok1 := mtfZScore(c, 20)
		mean, ok2 := mtfSMA(c, 20)
		if !ok || !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		if long {
			if z > -2.0 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, mean,
				fmt.Sprintf("z-score %.2f below the 20-bar mean, in a range", z))
		}
		if z < 2.0 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, mean,
			fmt.Sprintf("z-score %.2f above the 20-bar mean, in a range", z))
	}
}

// stratLinRegBreak: price leaving the regression channel in the direction the
// channel is already sloping — an acceleration, not a reversal.
func stratLinRegBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		val, slope, se, ok1 := mtfLinReg(c, 40)
		if !ok || !ok1 || atr <= 0 || se <= 0 {
			return NoSignal(name)
		}
		if long {
			if slope <= 0 || price < val+2*se {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				fmt.Sprintf("broke 2 SE above a rising regression channel (slope %.5f)", slope))
		}
		if slope >= 0 || price > val-2*se {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			fmt.Sprintf("broke 2 SE below a falling regression channel (slope %.5f)", slope))
	}
}

// stratLinRegFade: the opposite trade — price at the channel edge AGAINST a flat
// channel, faded back toward the fit.
//
// The slope condition is what separates it from the break family above: a flat
// channel is a range, a sloping one is a trend, and the same price location
// means opposite things in each.
func stratLinRegFade(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		val, slope, se, ok1 := mtfLinReg(c, 40)
		if !ok || !ok1 || atr <= 0 || se <= 0 || val <= 0 {
			return NoSignal(name)
		}
		// Flat: the channel drifts less than half an ATR across its whole span.
		if math.Abs(slope)*40 > 0.5*atr*price {
			return NoSignal(name)
		}
		if long {
			if price > val-2*se {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, val,
				"at the lower edge of a flat regression channel")
		}
		if price < val+2*se {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, val,
			"at the upper edge of a flat regression channel")
	}
}

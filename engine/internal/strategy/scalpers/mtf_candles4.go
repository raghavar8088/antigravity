package scalpers

import (
	"fmt"
	"math"
)

// mtf_candles4.go — the named candlestick reversal and continuation patterns the
// catalogue was still missing: tweezers, harami, piercing/dark cloud, three
// inside/outside, three methods, kicker, abandoned baby, spinning top, belt hold
// and the one-sided dojis.
//
// UNITS: mtfATR returns ATR as a FRACTION OF PRICE. Anything compared against a
// price gap must use atr*price; only mtfSignalToTarget takes the fraction. An
// earlier pack in this directory got that wrong across 26 sites, and every
// affected family passed its own conditions and was then refused downstream by
// the fee bar — which on a live desk is indistinguishable from "no setups found".
//
// Every pattern is paired with a CONFIRMATION: location against a moving
// average, a volume check, or a trend condition. These shapes are common enough
// that alone they fire constantly and win rarely, which is what the retired 1m
// roster demonstrated.

// extremeDistATR measures the pattern's OWN EXTREME against the 21 EMA, in ATR.
//
// Not its close, and the difference is the whole point. A bullish reversal
// candle closes UP by construction — frequently back above the mean — so testing
// the close asks "did this candle end strong" when the question is "did it
// reject a low". Measuring the close rejected every correct tweezer, harami,
// piercing line and dragonfly the fixtures could produce, while each pattern's
// own shape rules passed: a filter that looks like a location check and is
// actually a momentum one.
//
// bars is how much of the pattern to include, so a two-candle shape is measured
// across both of its candles.
func extremeDistATR(c []Candle, price float64, bars int, low bool) (float64, bool) {
	ema, ok1 := mtfEMA(c, 21)
	atr, ok2 := mtfATR(c, 14)
	if !ok1 || !ok2 || atr <= 0 || price <= 0 || len(c) == 0 {
		return 0, false
	}
	if bars < 1 {
		bars = 1
	}
	start := len(c) - bars
	if start < 0 {
		start = 0
	}
	ext := c[start].Low
	if !low {
		ext = c[start].High
	}
	for _, k := range c[start:] {
		if low {
			ext = math.Min(ext, k.Low)
		} else {
			ext = math.Max(ext, k.High)
		}
	}
	return (ext - ema) / (atr * price), true
}

// patTweezer: two adjacent candles rejecting from almost the same extreme.
//
// The equality of those extremes IS the pattern, so it is measured in ATR rather
// than eyeballed. A fixed tick tolerance would mean something completely
// different on BTC at $64,000 than on a token at $0.005.
func patTweezer(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		tol := 0.15 * atr * price
		a, b := c[len(c)-2], c[len(c)-1]
		if long {
			dist, ok2 := extremeDistATR(c, price, 2, true)
			if !ok2 {
				return NoSignal(name)
			}
			// Tweezer BOTTOM: matched lows, second candle closes up, into weakness.
			if math.Abs(a.Low-b.Low) > tol || !pBear(a) || !pBull(b) || dist > -0.5 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("tweezer bottom, lows within %.0f%% of an ATR", math.Abs(a.Low-b.Low)/(atr*price)*100))
		}
		dist, ok2 := extremeDistATR(c, price, 2, false)
		if !ok2 {
			return NoSignal(name)
		}
		if math.Abs(a.High-b.High) > tol || !pBull(a) || !pBear(b) || dist < 0.5 {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("tweezer top, highs within %.0f%% of an ATR", math.Abs(a.High-b.High)/(atr*price)*100))
	}
}

// haramiFamily builds both the standard harami and the cross variant.
//
// The cross demands the inside candle be a doji — indecision INSIDE the prior
// range, a stronger statement than a merely small body and rare enough to be
// worth its own family rather than a parameter.
func haramiFamily(cross bool) func(bool) func(string, []Candle, float64) Signal {
	return func(long bool) func(string, []Candle, float64) Signal {
		return func(name string, c []Candle, price float64) Signal {
			if len(c) < 60 {
				return NoSignal(name)
			}
			atr, ok := mtfATR(c, 14)
			if !ok || atr <= 0 {
				return NoSignal(name)
			}
			mother, baby := c[len(c)-2], c[len(c)-1]
			mBody, bBody := pBody(mother), pBody(baby)
			// The mother must be a real candle, not noise the baby happens to fit inside.
			if mBody < 0.8*atr*price || pRange(baby) <= 0 {
				return NoSignal(name)
			}
			mTop, mBot := math.Max(mother.Open, mother.Close), math.Min(mother.Open, mother.Close)
			if !(math.Max(baby.Open, baby.Close) < mTop && math.Min(baby.Open, baby.Close) > mBot) {
				return NoSignal(name)
			}
			if cross {
				if bodyFrac(baby) > 0.10 {
					return NoSignal(name)
				}
			} else if bBody > mBody*0.6 {
				return NoSignal(name)
			}
			label := "harami"
			if cross {
				label = "harami cross"
			}
			if long {
				dist, ok2 := extremeDistATR(c, price, 2, true)
				if !ok2 || !pBear(mother) || dist > -0.5 {
					return NoSignal(name)
				}
				tgt, okT := priorSwing(c, true)
				if !okT {
					return NoSignal(name)
				}
				return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
					fmt.Sprintf("bullish %s inside a %.1f ATR body", label, mBody/(atr*price)))
			}
			dist, ok2 := extremeDistATR(c, price, 2, false)
			if !ok2 || !pBull(mother) || dist < 0.5 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, false)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
				fmt.Sprintf("bearish %s inside a %.1f ATR body", label, mBody/(atr*price)))
		}
	}
}

// patPiercing: piercing line (long) and dark cloud cover (short).
//
// The DEPTH of the close into the prior body is the whole signal, so it is
// enforced rather than assumed. Past the midpoint is the classical definition; a
// close that stops short is a weaker candle wearing the name.
func patPiercing(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		a, b := c[len(c)-2], c[len(c)-1]
		if pBody(a) < 0.8*atr*price || vr < 1.1 {
			return NoSignal(name)
		}
		mid := (a.Open + a.Close) / 2
		if long {
			dist, ok3 := extremeDistATR(c, price, 2, true)
			if !ok3 || !pBear(a) || !pBull(b) || b.Open >= a.Low || b.Close <= mid || b.Close >= a.Open || dist > -0.5 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("piercing line, closed %.0f%% back into the prior body", (b.Close-a.Close)/pBody(a)*100))
		}
		dist, ok3 := extremeDistATR(c, price, 2, false)
		if !ok3 || !pBull(a) || !pBear(b) || b.Open <= a.High || b.Close >= mid || b.Close <= a.Open || dist < 0.5 {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("dark cloud cover, closed %.0f%% back into the prior body", (a.Close-b.Close)/pBody(a)*100))
	}
}

// patThreeInside: a harami plus CONFIRMATION — the third candle closing past the
// mother's body. The confirmation is what separates this from a harami that
// never went anywhere, which is most of them.
func patThreeInside(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		m, b, k := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		if pBody(m) < 0.8*atr*price {
			return NoSignal(name)
		}
		mTop, mBot := math.Max(m.Open, m.Close), math.Min(m.Open, m.Close)
		if !(math.Max(b.Open, b.Close) < mTop && math.Min(b.Open, b.Close) > mBot) {
			return NoSignal(name)
		}
		if long {
			if !pBear(m) || !pBull(b) || !pBull(k) || k.Close <= mTop {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+pRange(m)*1.5,
				"three inside up: harami confirmed past the mother body")
		}
		if !pBull(m) || !pBear(b) || !pBear(k) || k.Close >= mBot {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-pRange(m)*1.5,
			"three inside down: harami confirmed past the mother body")
	}
}

// patThreeOutside: an engulfing candle followed by continuation the same way.
func patThreeOutside(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		a, e, k := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		aTop, aBot := math.Max(a.Open, a.Close), math.Min(a.Open, a.Close)
		eTop, eBot := math.Max(e.Open, e.Close), math.Min(e.Open, e.Close)
		if !(eTop > aTop && eBot < aBot) || pBody(e) < 0.8*atr*price {
			return NoSignal(name)
		}
		if long {
			if !pBear(a) || !pBull(e) || !pBull(k) || k.Close <= e.Close {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+pRange(e)*1.5,
				"three outside up: engulfing then continuation")
		}
		if !pBull(a) || !pBear(e) || !pBear(k) || k.Close >= e.Close {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-pRange(e)*1.5,
			"three outside down: engulfing then continuation")
	}
}

// patThreeMethods: rising/falling three methods — a long candle, a shallow
// counter-trend pause held INSIDE its range, then a resumption closing beyond it.
//
// A continuation pattern, so the pause must not break the range. That break is
// exactly what turns this shape into a reversal, and treating the two the same
// is how a continuation strategy ends up buying tops.
func patThreeMethods(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		n := len(c)
		first := c[n-5]
		mid := c[n-4 : n-1]
		last := c[n-1]
		if pBody(first) < 1.0*atr*price {
			return NoSignal(name)
		}
		for _, k := range mid {
			if k.High > first.High || k.Low < first.Low {
				return NoSignal(name)
			}
		}
		if long {
			if !pBull(first) || !pBull(last) || last.Close <= first.High {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+pRange(first),
				"rising three methods: pause held inside the range, then resumed")
		}
		if !pBear(first) || !pBear(last) || last.Close >= first.Low {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-pRange(first),
			"falling three methods: pause held inside the range, then resumed")
	}
}

// patKicker: two strong candles of OPPOSITE colour separated by a gap that is
// never filled within the pair.
//
// The gap is the pattern. On a perpetual that trades continuously a true gap is
// rare and usually a liquidation cascade, so the size band is bounded at both
// ends: below it there is nothing to trade, above it the move is news and the
// entry is already late.
func patKicker(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		atrAbs := atr * price
		a, b := c[len(c)-2], c[len(c)-1]
		if bodyFrac(a) < 0.6 || bodyFrac(b) < 0.6 || vr < 1.3 {
			return NoSignal(name)
		}
		if long {
			gap := b.Open - a.Open
			if !pBear(a) || !pBull(b) || gap < 0.5*atrAbs || gap > 4*atrAbs || b.Low < a.Open {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+2*pRange(b),
				fmt.Sprintf("bullish kicker, %.1f ATR gap on %.1fx volume", gap/atrAbs, vr))
		}
		gap := a.Open - b.Open
		if !pBull(a) || !pBear(b) || gap < 0.5*atrAbs || gap > 4*atrAbs || b.High > a.Open {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-2*pRange(b),
			fmt.Sprintf("bearish kicker, %.1f ATR gap on %.1fx volume", gap/atrAbs, vr))
	}
}

// patAbandonedBaby: a doji ISLANDED by gaps on both sides at a trend extreme.
// The rarest of these shapes and the strictest test — both gaps must exist, so
// the middle candle's entire range sits clear of both neighbours.
func patAbandonedBaby(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		a, d, b := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		if bodyFrac(d) > 0.10 {
			return NoSignal(name)
		}
		if long {
			dist, ok2 := extremeDistATR(c, price, 3, true)
			if !ok2 || !pBear(a) || !pBull(b) || d.High >= a.Low || d.High >= b.Low || dist > -0.5 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				"bullish abandoned baby: doji islanded by gaps at a low")
		}
		dist, ok2 := extremeDistATR(c, price, 3, false)
		if !ok2 || !pBull(a) || !pBear(b) || d.Low <= a.High || d.Low <= b.High || dist < 0.5 {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			"bearish abandoned baby: doji islanded by gaps at a high")
	}
}

// patSpinningTop: a small body with wicks on BOTH sides, at a trend extreme.
//
// Exhaustion, not reversal — it says the move has stopped, which is a weaker
// claim than saying it will turn. So the extreme has to be real: two ATR from
// the mean, not merely a quiet candle somewhere in a range.
func patSpinningTop(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		k := c[len(c)-1]
		if pRange(k) <= 0 || bodyFrac(k) > 0.30 {
			return NoSignal(name)
		}
		if pUpWick(k) < pRange(k)*0.25 || pLoWick(k) < pRange(k)*0.25 {
			return NoSignal(name)
		}
		if long {
			dist, ok2 := extremeDistATR(c, price, 1, true)
			if !ok2 || dist > -2.0 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("spinning top %.1f ATR below the mean: downside exhaustion", -dist))
		}
		dist, ok2 := extremeDistATR(c, price, 1, false)
		if !ok2 || dist < 2.0 {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("spinning top %.1f ATR above the mean: upside exhaustion", dist))
	}
}

// patBeltHold: a candle opening AT its extreme and running the other way, with
// no wick on the opening side. One side had the bar from the first tick.
func patBeltHold(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		k := c[len(c)-1]
		if pRange(k) < 1.0*atr*price || bodyFrac(k) < 0.7 || vr < 1.2 {
			return NoSignal(name)
		}
		// Judged by where it OPENED. A bullish belt hold closes at its high by
		// definition, so testing the close asks whether the candle was strong —
		// which it always is — instead of whether it began from weakness.
		ema, okE := mtfEMA(c, 21)
		if !okE {
			return NoSignal(name)
		}
		if long {
			if !pBull(k) || pLoWick(k) > pRange(k)*0.05 || k.Open > ema {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+1.5*pRange(k),
				fmt.Sprintf("bullish belt hold, opened at the low on %.1fx volume", vr))
		}
		if !pBear(k) || pUpWick(k) > pRange(k)*0.05 || k.Open < ema {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-1.5*pRange(k),
			fmt.Sprintf("bearish belt hold, opened at the high on %.1fx volume", vr))
	}
}

// patLongLeggedDoji: a doji with LARGE wicks both sides — a wide fight that
// resolved nowhere. Distinct from the small spinning top and from the one-sided
// dragonfly/gravestone shapes, which are different statements about who lost.
func patLongLeggedDoji(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		k := c[len(c)-1]
		if pRange(k) < 1.5*atr*price || bodyFrac(k) > 0.10 {
			return NoSignal(name)
		}
		if pUpWick(k) < pRange(k)*0.35 || pLoWick(k) < pRange(k)*0.35 {
			return NoSignal(name)
		}
		if long {
			dist, ok2 := extremeDistATR(c, price, 1, true)
			if !ok2 || dist > -1.5 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("long-legged doji %.1f ATR wide at a low", pRange(k)/(atr*price)))
		}
		dist, ok2 := extremeDistATR(c, price, 1, false)
		if !ok2 || dist < 1.5 {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("long-legged doji %.1f ATR wide at a high", pRange(k)/(atr*price)))
	}
}

// patDragonflyGravestone: the one-sided dojis. Dragonfly (all lower wick) is
// bullish rejection, gravestone (all upper wick) bearish.
//
// Kept apart from the generic doji family because here the WICK SIDE is the
// direction — pairing a dragonfly with a short would be reading the candle
// backwards.
func patDragonflyGravestone(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		k := c[len(c)-1]
		if pRange(k) < 0.8*atr*price || bodyFrac(k) > 0.10 {
			return NoSignal(name)
		}
		if long {
			dist, ok2 := extremeDistATR(c, price, 1, true)
			if !ok2 || pUpWick(k) > pRange(k)*0.10 || pLoWick(k) < pRange(k)*0.70 || dist > -0.5 {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt, "dragonfly doji: full rejection of the low")
		}
		dist, ok2 := extremeDistATR(c, price, 1, false)
		if !ok2 || pLoWick(k) > pRange(k)*0.10 || pUpWick(k) < pRange(k)*0.70 || dist < 0.5 {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt, "gravestone doji: full rejection of the high")
	}
}

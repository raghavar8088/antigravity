package scalpers

import (
	"fmt"
	"math"
)

// mtf_patterns3.go — the remaining chart, candlestick and price-structure
// templates, completing the catalogue.
//
// Same contract as the first two packs: a shape, a CONFIRMATION, an ATR-derived
// stop, and a target read off structure so the reward:risk falls out of the
// setup instead of being imposed on it. A pattern without confirmation fires
// constantly and wins rarely — that is what the retired 1m roster demonstrated,
// and none of these repeat it.
//
// The multi-swing shapes here (wedge, diamond, cup, rounding, broadening) need
// a long lookback to recognise anything real, so they self-reject on short
// histories rather than "detecting" a shape in eight candles of noise. On the
// fast timeframes most of them will simply never fire, and on the ones that do
// the fee bar refuses the majority. That is intended: the pattern is allowed to
// exist everywhere and the economics decide where it is worth trading.

// UNITS, because getting this wrong is silent and this pack got it wrong once:
// mtfATR returns ATR as a FRACTION OF PRICE, not a price distance. Anything
// compared against a price gap must use atr*price; only mtfSignalToTarget takes
// the fraction. The first draft of this file added `2*atr` to an EMA — adding
// 0.005 to a price of 100 instead of two ATR — and every affected family passed
// its own conditions and was then refused by the fee bar, which looks exactly
// like "no setups found".

// ── small helpers, local to this pack ────────────────────────────────────────

// lineFit returns slope and intercept of a least-squares line through pts,
// where x is the index and y the price. Slope is per-bar.
func lineFit(idx []int, y []float64) (slope, intercept float64, ok bool) {
	n := float64(len(idx))
	if len(idx) < 2 || len(idx) != len(y) {
		return 0, 0, false
	}
	var sx, sy, sxx, sxy float64
	for i, x := range idx {
		fx := float64(x)
		sx += fx
		sy += y[i]
		sxx += fx * fx
		sxy += fx * y[i]
	}
	den := n*sxx - sx*sx
	if den == 0 {
		return 0, 0, false
	}
	slope = (n*sxy - sx*sy) / den
	intercept = (sy - slope*sx) / n
	return slope, intercept, true
}

// swingPrices lifts the high/low values at the given swing indices.
func swingPrices(c []Candle, idx []int, high bool) []float64 {
	out := make([]float64, 0, len(idx))
	for _, i := range idx {
		if high {
			out = append(out, c[i].High)
		} else {
			out = append(out, c[i].Low)
		}
	}
	return out
}

// lastN returns the final n elements, or everything when shorter.
func lastNInts(v []int, n int) []int {
	if len(v) <= n {
		return v
	}
	return v[len(v)-n:]
}

// ── chart patterns ───────────────────────────────────────────────────────────

// patWedge: rising or falling wedge — both boundaries sloping the SAME way and
// converging.
//
// The direction is counter to the slope, which is the whole point of the shape:
// a rising wedge makes higher highs on shrinking momentum, so it breaks down.
// Trading it in the direction of the slope is just buying a trend, which the
// pullback and breakout families already cover.
func patWedge(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		hi, lo := swingPoints(c, 3)
		hi, lo = lastNInts(hi, 4), lastNInts(lo, 4)
		if len(hi) < 3 || len(lo) < 3 {
			return NoSignal(name)
		}
		sh, _, okH := lineFit(hi, swingPrices(c, hi, true))
		sl, _, okL := lineFit(lo, swingPrices(c, lo, false))
		if !okH || !okL {
			return NoSignal(name)
		}
		// Converging: the gap between the boundaries at the last swing is
		// materially smaller than at the first.
		wideStart := c[hi[0]].High - c[lo[0]].Low
		wideEnd := c[hi[len(hi)-1]].High - c[lo[len(lo)-1]].Low
		if wideStart <= 0 || wideEnd >= wideStart*0.75 {
			return NoSignal(name)
		}
		if long {
			// FALLING wedge: both boundaries down, breaks UP.
			if sh >= 0 || sl >= 0 || price <= c[hi[len(hi)-1]].High || vr < 1.2 {
				return NoSignal(name)
			}
			// Target: the wedge's own height projected from the break.
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+wideStart,
				fmt.Sprintf("falling wedge break up, %.0f%% narrowed, %.1fx volume", (1-wideEnd/wideStart)*100, vr))
		}
		// RISING wedge: both boundaries up, breaks DOWN.
		if sh <= 0 || sl <= 0 || price >= c[lo[len(lo)-1]].Low || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-wideStart,
			fmt.Sprintf("rising wedge break down, %.0f%% narrowed, %.1fx volume", (1-wideEnd/wideStart)*100, vr))
	}
}

// patPennant: a sharp move (the flagpole) followed by a SYMMETRICAL
// consolidation, then continuation.
//
// The distinction from a flag is the shape of the pause: a flag drifts against
// the move in a channel, a pennant converges from both sides. Both continue, so
// the flagpole is what has to be real — without it this is just a triangle.
func patPennant(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		if !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		n := len(c)
		// Flagpole: a decisive run over the 10 bars ending 12 bars ago.
		poleStart, poleEnd := n-22, n-12
		atrAbs := atr * price
		pole := c[poleEnd].Close - c[poleStart].Close
		if math.Abs(pole) < 3*atrAbs {
			return NoSignal(name)
		}
		// Consolidation: the last 12 bars, converging and tighter than the pole.
		seg := c[n-12:]
		var hiA, loA, hiB, loB float64
		hiA, loA = seg[0].High, seg[0].Low
		for _, k := range seg[:6] {
			hiA = math.Max(hiA, k.High)
			loA = math.Min(loA, k.Low)
		}
		hiB, loB = seg[6].High, seg[6].Low
		for _, k := range seg[6:] {
			hiB = math.Max(hiB, k.High)
			loB = math.Min(loB, k.Low)
		}
		if hiA-loA <= 0 || hiB-loB >= (hiA-loA)*0.7 {
			return NoSignal(name)
		}
		if long {
			if pole <= 0 || price <= hiB {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+pole,
				fmt.Sprintf("bull pennant, pole %.2f ATR, break up", math.Abs(pole)/atr))
		}
		if pole >= 0 || price >= loB {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price+pole,
			fmt.Sprintf("bear pennant, pole %.2f ATR, break down", math.Abs(pole)/atr))
	}
}

// patCupHandle: a rounded base, a shallow pullback, then a break of the rim.
//
// The handle is the part that matters and the part usually skipped. A cup with
// no handle is a rounding bottom, which is a different template with a
// different failure mode — so this requires the pullback to exist and to be
// shallow relative to the cup.
func patCupHandle(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 120 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		n := len(c)
		cup := c[n-60 : n-10]  // the bowl
		handle := c[n-10:]     // the pause at the rim
		mid := cup[len(cup)/2] // the extreme should sit near the middle

		if long {
			rim := math.Max(cup[0].High, cup[len(cup)-1].High)
			var deep float64 = cup[0].Low
			deepIdx := 0
			for i, k := range cup {
				if k.Low < deep {
					deep, deepIdx = k.Low, i
				}
			}
			depth := rim - deep
			// Bowl, not a V: the low belongs in the middle third.
			if depth <= 2*atr*price || deepIdx < len(cup)/3 || deepIdx > 2*len(cup)/3 || mid.Low > deep+depth*0.5 {
				return NoSignal(name)
			}
			// Handle: a shallow drift, no more than a third of the cup deep.
			hLo := handle[0].Low
			for _, k := range handle {
				hLo = math.Min(hLo, k.Low)
			}
			if rim-hLo > depth*0.35 || price <= rim || vr < 1.2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, rim+depth,
				fmt.Sprintf("cup & handle break, depth %.2f ATR, %.1fx volume", depth/atr, vr))
		}

		// Inverted: a dome with a shallow rally as the handle.
		rim := math.Min(cup[0].Low, cup[len(cup)-1].Low)
		peak := cup[0].High
		peakIdx := 0
		for i, k := range cup {
			if k.High > peak {
				peak, peakIdx = k.High, i
			}
		}
		depth := peak - rim
		if depth <= 2*atr*price || peakIdx < len(cup)/3 || peakIdx > 2*len(cup)/3 || mid.High < peak-depth*0.5 {
			return NoSignal(name)
		}
		hHi := handle[0].High
		for _, k := range handle {
			hHi = math.Max(hHi, k.High)
		}
		if hHi-rim > depth*0.35 || price >= rim || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, rim-depth,
			fmt.Sprintf("inverted cup & handle break, depth %.2f ATR, %.1fx volume", depth/atr, vr))
	}
}

// patRounding: a saucer — a gradual curve with no sharp reversal point.
//
// Distinguished from a V-reversal by requiring the extreme to be flat: the
// deepest quarter of the base has to be shallow relative to the whole. A V that
// happens to end lower is a different event and usually a violent one.
func patRounding(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 100 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		if !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		seg := c[len(c)-50:]
		q := len(seg) / 4
		first, mid, last := seg[:q], seg[q:3*q], seg[3*q:]

		agg := func(s []Candle, high bool) float64 {
			v := s[0].High
			if !high {
				v = s[0].Low
			}
			for _, k := range s {
				if high {
					v = math.Max(v, k.High)
				} else {
					v = math.Min(v, k.Low)
				}
			}
			return v
		}
		if long {
			// Saucer bottom: edges high, middle low, and rising back out.
			fHi, lHi, mLo := agg(first, true), agg(last, true), agg(mid, false)
			depth := math.Max(fHi, lHi) - mLo
			if depth <= 3*atr*price {
				return NoSignal(name)
			}
			// The base must be FLAT: the middle's own range is a small part of
			// the depth, which is what separates a saucer from a V.
			if agg(mid, true)-mLo > depth*0.45 || price <= lHi || lHi < fHi-2*atr*price {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+depth*0.8,
				fmt.Sprintf("rounding bottom completing, depth %.2f ATR", depth/atr))
		}
		fLo, lLo, mHi := agg(first, false), agg(last, false), agg(mid, true)
		depth := mHi - math.Min(fLo, lLo)
		if depth <= 3*atr*price {
			return NoSignal(name)
		}
		if mHi-agg(mid, false) > depth*0.45 || price >= lLo || lLo > fLo+2*atr*price {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-depth*0.8,
			fmt.Sprintf("rounding top completing, depth %.2f ATR", depth/atr))
	}
}

// patBroadening: a megaphone — successively higher highs AND lower lows.
//
// Faded rather than followed. A broadening formation is disagreement widening,
// and the edges are where the disagreement has just been resolved against
// whoever pushed last, so the trade is back toward the middle.
func patBroadening(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		if !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		hi, lo := swingPoints(c, 3)
		hi, lo = lastNInts(hi, 3), lastNInts(lo, 3)
		if len(hi) < 3 || len(lo) < 3 {
			return NoSignal(name)
		}
		// Diverging on both sides.
		hiRising := c[hi[2]].High > c[hi[1]].High && c[hi[1]].High > c[hi[0]].High
		loFalling := c[lo[2]].Low < c[lo[1]].Low && c[lo[1]].Low < c[lo[0]].Low
		if !hiRising || !loFalling {
			return NoSignal(name)
		}
		top, bot := c[hi[2]].High, c[lo[2]].Low
		midPx := (top + bot) / 2
		if top-bot <= 3*atr*price {
			return NoSignal(name)
		}
		if long {
			// At the lower edge, fade back toward the middle.
			if price > bot+atr*price {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, midPx,
				fmt.Sprintf("broadening formation, long from the lower rail (%.2f ATR wide)", (top-bot)/atr))
		}
		if price < top-atr*price {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, midPx,
			fmt.Sprintf("broadening formation, short from the upper rail (%.2f ATR wide)", (top-bot)/atr))
	}
}

// patDiamond: a broadening phase followed by a narrowing one.
//
// Rare and frequently imagined. Requiring BOTH halves — widening then
// converging, each measured against the other — is what stops this matching any
// noisy range, which is what a looser reading of "diamond" would do.
func patDiamond(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 120 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		seg := c[len(c)-60:]
		third := len(seg) / 3
		span := func(s []Candle) float64 {
			hi, lo := s[0].High, s[0].Low
			for _, k := range s {
				hi = math.Max(hi, k.High)
				lo = math.Min(lo, k.Low)
			}
			return hi - lo
		}
		a, b, d := span(seg[:third]), span(seg[third:2*third]), span(seg[2*third:])
		// Widened, then narrowed — the middle is the widest part.
		if !(b > a*1.3 && d < b*0.7) {
			return NoSignal(name)
		}
		height := b
		if height <= 3*atr*price {
			return NoSignal(name)
		}
		lastHi, lastLo := seg[2*third].High, seg[2*third].Low
		for _, k := range seg[2*third:] {
			lastHi = math.Max(lastHi, k.High)
			lastLo = math.Min(lastLo, k.Low)
		}
		if long {
			if price <= lastHi || vr < 1.2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+height,
				fmt.Sprintf("diamond break up, %.2f ATR tall, %.1fx volume", height/atr, vr))
		}
		if price >= lastLo || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-height,
			fmt.Sprintf("diamond break down, %.2f ATR tall, %.1fx volume", height/atr, vr))
	}
}

// ── candlestick ──────────────────────────────────────────────────────────────

// patHammer: hammer / shooting star — a small body with one long wick.
//
// Separate from PinBar, which additionally demands the bar sit at a swing
// extreme. This is the plain single-bar shape with a trend filter, so the two
// disagree often enough to be worth running as different streams: the pin bar
// is a location statement, this is a rejection statement.
func patHammer(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		ema, ok2 := mtfEMA(c, 21)
		if !ok1 || !ok2 || pRange(last) <= 0 {
			return NoSignal(name)
		}
		body := pBody(last)
		if body <= 0 || body > pRange(last)*0.35 {
			return NoSignal(name)
		}
		up, dn := pUpWick(last), pLoWick(last)
		if long {
			// Hammer: long lower wick, into weakness.
			if dn < 2*body || dn < 2*up || price > ema {
				return NoSignal(name)
			}
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("hammer, lower wick %.1fx body", dn/body))
		}
		// Shooting star: long upper wick, into strength.
		if up < 2*body || up < 2*dn || price < ema {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("shooting star, upper wick %.1fx body", up/body))
	}
}

// ── price structure ──────────────────────────────────────────────────────────

// patKeltner: a close outside the Keltner channel — EMA ± 2 ATR.
//
// Treated as a breakout, not a fade, which is the opposite of the Bollinger %B
// family. The difference is deliberate: Bollinger bands widen with volatility so
// an extreme reading mean-reverts, while a Keltner break is a move beyond what
// recent RANGE explains and tends to continue.
func patKeltner(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		ema, ok1 := mtfEMA(c, 20)
		atr, ok2 := mtfATR(c, 14)
		vr, ok3 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || !ok3 || atr <= 0 {
			return NoSignal(name)
		}
		atrAbs := atr * price
		upper, lower := ema+2*atrAbs, ema-2*atrAbs
		prev := c[len(c)-2].Close
		if long {
			// Must be the bar that CROSSES out, not any bar already outside.
			if !(prev <= upper && price > upper) || vr < 1.2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+(price-ema),
				fmt.Sprintf("keltner break up, %.1fx volume", vr))
		}
		if !(prev >= lower && price < lower) || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-(ema-price),
			fmt.Sprintf("keltner break down, %.1fx volume", vr))
	}
}

// patPriorSessionBreak: a break of the prior 24-hour high or low.
//
// Crypto has no session close, so "prior session" is a rolling window rather
// than an exchange calendar — stated plainly because an equity trader reading
// this name would assume an opening bell that does not exist here.
func patPriorSessionBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 100 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		// The window ending 10 bars ago, so the level is established rather
		// than being set by the same move that is breaking it.
		w := c[len(c)-100 : len(c)-10]
		hi, lo := w[0].High, w[0].Low
		for _, k := range w {
			hi = math.Max(hi, k.High)
			lo = math.Min(lo, k.Low)
		}
		if hi-lo <= 2*atr*price {
			return NoSignal(name)
		}
		if long {
			if price <= hi || vr < 1.2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+(hi-lo)*0.5,
				fmt.Sprintf("prior-window high break, %.1fx volume", vr))
		}
		if price >= lo || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-(hi-lo)*0.5,
			fmt.Sprintf("prior-window low break, %.1fx volume", vr))
	}
}

// patRoundNumber: a break through a psychologically round level.
//
// The level is derived from the price's own magnitude, so it is $1,000 on BTC
// and $0.001 on a sub-cent token — a fixed step would be meaningless across a
// book whose prices span six orders of magnitude.
func patRoundNumber(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 || price <= 0 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		// Round step: start one order of magnitude below the price, then step
		// DOWN until the level is reachable relative to volatility.
		//
		// Magnitude alone is not enough. At price 100 with an ATR of 0.15 the
		// naive step is 10 — sixty-six ATR away — so a "round-number break"
		// would fire roughly never, and the family would look implemented while
		// contributing nothing. Descending by powers of ten keeps the level
		// genuinely round (10, 1, 0.1, 0.01 …) while making it a level price
		// actually visits.
		step := math.Pow(10, math.Floor(math.Log10(price))-1)
		for i := 0; i < 6 && step > 6*atr*price; i++ {
			step /= 10
		}
		if step <= 0 || math.IsInf(step, 0) {
			return NoSignal(name)
		}
		prev := c[len(c)-2].Close
		lvl := math.Round(price/step) * step
		// The level has to be genuinely nearby relative to noise, or every bar
		// "breaks" one.
		// Near enough to the level that this is a break rather than a coincidence,
		// and the step must still be wide enough that the level means something.
		if math.Abs(price-lvl) > atr*price || step < atr*price*0.5 {
			return NoSignal(name)
		}
		if long {
			if !(prev < lvl && price >= lvl) || vr < 1.2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, lvl+step,
				fmt.Sprintf("round-number break above %.6g, %.1fx volume", lvl, vr))
		}
		if !(prev > lvl && price <= lvl) || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, lvl-step,
			fmt.Sprintf("round-number break below %.6g, %.1fx volume", lvl, vr))
	}
}

// patTTMSqueeze: Bollinger bands inside the Keltner channel, then released.
//
// The classic TTM construction. It differs from the SqueezeExpansion family,
// which measures bandwidth against its own history; this one is a relationship
// between two envelopes, so the two fire at different moments often enough to
// be worth separating.
func patTTMSqueeze(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		bbU, bbM, bbL, ok1 := mtfBollinger(c, 20, 2.0)
		ema, ok2 := mtfEMA(c, 20)
		atr, ok3 := mtfATR(c, 14)
		if !ok1 || !ok2 || !ok3 || atr <= 0 {
			return NoSignal(name)
		}
		atrAbs := atr * price
		kU, kL := ema+1.5*atrAbs, ema-1.5*atrAbs
		inSqueeze := bbU < kU && bbL > kL
		// Released THIS bar: the squeeze must have been on, and price now out.
		if inSqueeze {
			return NoSignal(name)
		}
		prevIn := false
		if len(c) > 21 {
			pU, _, pL, okp := mtfBollinger(c[:len(c)-1], 20, 2.0)
			pe, oke := mtfEMA(c[:len(c)-1], 20)
			pa, oka := mtfATR(c[:len(c)-1], 14)
			if okp && oke && oka {
				paAbs := pa * pe
				prevIn = pU < pe+1.5*paAbs && pL > pe-1.5*paAbs
			}
		}
		if !prevIn {
			return NoSignal(name)
		}
		if long {
			if price <= bbM {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+(bbU-bbL),
				"TTM squeeze released upward")
		}
		if price >= bbM {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-(bbU-bbL),
			"TTM squeeze released downward")
	}
}

// patEMARibbon: the 8/13/21/34 EMAs compressed, then fanning out.
//
// Compression means every horizon agrees on price, which is the condition
// immediately before a trend rather than during one. The trade is the fan, not
// the bunch — entering inside the compression is entering a range.
func patEMARibbon(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, okA := mtfATR(c, 14)
		e8, ok1 := mtfEMA(c, 8)
		e13, ok2 := mtfEMA(c, 13)
		e21, ok3 := mtfEMA(c, 21)
		e34, ok4 := mtfEMA(c, 34)
		if !okA || !ok1 || !ok2 || !ok3 || !ok4 || atr <= 0 {
			return NoSignal(name)
		}
		spread := math.Max(math.Max(e8, e13), math.Max(e21, e34)) - math.Min(math.Min(e8, e13), math.Min(e21, e34))
		// Was compressed one bar ago, is expanding now.
		prev := c[:len(c)-1]
		p8, _ := mtfEMA(prev, 8)
		p13, _ := mtfEMA(prev, 13)
		p21, _ := mtfEMA(prev, 21)
		p34, _ := mtfEMA(prev, 34)
		pSpread := math.Max(math.Max(p8, p13), math.Max(p21, p34)) - math.Min(math.Min(p8, p13), math.Min(p21, p34))
		if pSpread > atr*price*0.6 || spread <= pSpread {
			return NoSignal(name)
		}
		if long {
			if !(e8 > e13 && e13 > e21 && e21 > e34) || price <= e8 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price*(1+4*atr),
				"EMA ribbon fanning up out of compression")
		}
		if !(e8 < e13 && e13 < e21 && e21 < e34) || price >= e8 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price*(1-4*atr),
			"EMA ribbon fanning down out of compression")
	}
}

// patATRThrust: a single bar whose range dwarfs recent ATR, closing at its
// extreme.
//
// The close is the filter. A huge range that closes mid-bar is a fight; a huge
// range that closes on its high is one side finishing unopposed, and only the
// second continues with any regularity.
func patATRThrust(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 || pRange(last) <= 0 {
			return NoSignal(name)
		}
		if pRange(last) < 2.0*atr*price || vr < 1.5 {
			return NoSignal(name)
		}
		// Closing position within its own bar.
		pos := (last.Close - last.Low) / pRange(last)
		if long {
			if pos < 0.75 || !pBull(last) {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, price+pRange(last),
				fmt.Sprintf("ATR thrust up %.1fx ATR, closed at %.0f%% of bar", pRange(last)/(atr*price), pos*100))
		}
		if pos > 0.25 || !pBear(last) {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, price-pRange(last),
			fmt.Sprintf("ATR thrust down %.1fx ATR, closed at %.0f%% of bar", pRange(last)/(atr*price), (1-pos)*100))
	}
}

// patPivotBreak: a break of the classic floor-trader pivot levels R1/S1.
//
// Pivots are computed from the PRIOR window's high, low and close, so the level
// is fixed before the bar that tests it — which is the only reason a level like
// this carries information at all.
func patPivotBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		// Prior "session": the 24 bars ending 24 bars ago.
		w := c[len(c)-48 : len(c)-24]
		hi, lo := w[0].High, w[0].Low
		for _, k := range w {
			hi = math.Max(hi, k.High)
			lo = math.Min(lo, k.Low)
		}
		cl := w[len(w)-1].Close
		p := (hi + lo + cl) / 3
		r1 := 2*p - lo
		s1 := 2*p - hi
		prev := c[len(c)-2].Close
		if long {
			if !(prev <= r1 && price > r1) || vr < 1.2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, r1+(r1-p),
				fmt.Sprintf("R1 break at %.6g, %.1fx volume", r1, vr))
		}
		if !(prev >= s1 && price < s1) || vr < 1.2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, s1-(p-s1),
			fmt.Sprintf("S1 break at %.6g, %.1fx volume", s1, vr))
	}
}

// patGapFade: a gap between bars that closes back toward the prior close.
//
// Perpetuals trade continuously, so a true gap is rare and usually a liquidation
// cascade or a thin book — which is exactly when a fade works and exactly when
// it is most dangerous. The size band is bounded at BOTH ends for that reason:
// below it there is nothing to fade, above it the gap is news and does not come
// back.
func patGapFade(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		if !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		prev, last := c[len(c)-2], c[len(c)-1]
		gap := last.Open - prev.Close
		mag := math.Abs(gap)
		atrAbs := atr * price
		if mag < 1.0*atrAbs || mag > 4.0*atrAbs {
			return NoSignal(name)
		}
		if long {
			// Gapped DOWN, fade up toward the prior close.
			if gap >= 0 || price >= prev.Close {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, prev.Close,
				fmt.Sprintf("gap down %.1f ATR, fading up to prior close", mag/atrAbs))
		}
		if gap <= 0 || price <= prev.Close {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, prev.Close,
			fmt.Sprintf("gap up %.1f ATR, fading down to prior close", mag/atrAbs))
	}
}

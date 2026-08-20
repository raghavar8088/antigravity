package scalpers

import "fmt"

// Volatility Contraction Pattern — Minervini.
//
// A base that TIGHTENS. Price pulls back, recovers, pulls back less, recovers,
// pulls back less again; volume dries up through the sequence; then it breaks
// out of the final, shallowest contraction. The claim is about supply: each
// shallower pullback is sellers being exhausted at progressively higher prices,
// and the volume dry-up is the evidence there is little left to sell.
//
// The detector below is built from that claim rather than from the picture. A
// chart with three dips in it is not a VCP; three dips that get SHALLOWER, on
// FALLING volume, inside an uptrend, is. Each of those is checked separately
// and any one of them failing refuses the setup — which is why this family
// fires rarely and should.
//
// What is deliberately NOT copied from the TradingView indicator this was
// specified from: its fixed -10% stop and +8% first target. That is a reward
// smaller than its risk (the panel reads "Risk : Reward 1 : 0.8"), and a
// strategy whose target is nearer than its stop needs to be right more than
// half the time before costs to break even. The stop here is structural —
// under the low of the final contraction, which is the price that falsifies the
// pattern — and the target is the measured move, so the ratio is whatever the
// base actually offers. mtfSignalToTarget then refuses anything under 1:1 or
// through the fee bar.

// vcpMinContractions is how many successive pullbacks must be present.
//
// Three, matching the "3 shrinking" the specification showed. Two is a pattern
// that has not established a sequence — any base has two dips — and four is
// rare enough on intraday timeframes to make the family never fire.
const vcpMinContractions = 3

// vcpTightening is how much shallower each contraction must be than the last.
//
// A contraction must be at most 80% of its predecessor. Requiring merely
// "smaller" would accept 9.9% after 10.0%, which is measurement noise rather
// than a supply signal; requiring half would reject most real bases.
const vcpTightening = 0.80

// vcpMaxFirstPullback caps how deep the FIRST contraction may be.
//
// Beyond 35% the structure is not a base, it is a downtrend with rallies in it,
// and "each pullback is shallower" describes a fall decelerating rather than
// accumulation.
const vcpMaxFirstPullback = 0.35

// vcpVolumeDryUp is the volume bar for the final contraction, as a share of the
// base's own average.
//
// 0.65 rather than the 0.38 the specification's panel happened to read: that
// figure was one instance, not a threshold, and demanding it would fit the rule
// to a single chart. Two thirds of average is the loosest reading that still
// means "quieter than the base it sits in".
const vcpVolumeDryUp = 0.65

// patVCP detects a volatility contraction and trades its breakout.
//
// long=false is the mirror: a series of shrinking RALLIES into a breakdown,
// which is the same supply argument with the sides reversed. Included for
// symmetry with every other family in the pack, and worth reading with more
// suspicion than the long — Minervini's version is a bull-market pattern about
// accumulation, and the short is an analogy rather than the same claim.
func patVCP(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		// A base needs room to have formed. This is the longest lookback in the
		// pack for a reason: three contractions cannot be measured in twenty
		// bars without calling noise a structure.
		if len(c) < 120 || price <= 0 {
			return NoSignal(name)
		}

		// 1. TREND. A VCP is a continuation pattern; a contraction in a
		//    downtrend is just a downtrend getting quieter.
		ema, ok := mtfEMA(c, 50)
		if !ok {
			return NoSignal(name)
		}
		if long && price < ema {
			return NoSignal(name)
		}
		if !long && price > ema {
			return NoSignal(name)
		}

		// 2. CONTRACTIONS. Swings confirmed on both sides, so nothing here can
		//    be read off a bar that is still forming.
		highs, lows := swingPoints(c, 3)
		if len(highs) < vcpMinContractions || len(lows) < vcpMinContractions {
			return NoSignal(name)
		}

		depths, tops, bottoms, ok := vcpDepths(c, highs, lows, long)
		if !ok || len(depths) < vcpMinContractions {
			return NoSignal(name)
		}
		// Only the most recent contractions matter; an old wide one before the
		// base began is not part of it.
		k := len(depths) - vcpMinContractions
		depths, tops, bottoms = depths[k:], tops[k:], bottoms[k:]

		// THE ASYMMETRY, and the whole reason a VCP is worth trading.
		//
		//	pivot     — the resistance being broken: the highest high in the base
		//	floor     — the LAST, shallowest contraction's low: the tight stop
		//	baseDepth — the WHOLE base's depth, projected from the pivot
		//
		// Measuring the target from the last contraction instead makes reward
		// equal risk by construction: the projection and the stop would both be
		// that contraction's depth, and every VCP would price at 1:1 no matter
		// how much structure preceded it. The point of the pattern is that the
		// stop shrinks with each contraction while the objective stays the size
		// of the base.
		pivot, floor := tops[0], bottoms[len(bottoms)-1]
		baseExtreme := bottoms[0]
		for _, t := range tops {
			if (long && t > pivot) || (!long && t < pivot) {
				pivot = t
			}
		}
		for _, b := range bottoms {
			if (long && b < baseExtreme) || (!long && b > baseExtreme) {
				baseExtreme = b
			}
		}

		if depths[0] > vcpMaxFirstPullback {
			return NoSignal(name)
		}
		for i := 1; i < len(depths); i++ {
			if depths[i] > depths[i-1]*vcpTightening {
				return NoSignal(name)
			}
		}

		// 3. VOLUME DRY-UP. Measured over the last few bars against the base's
		//    own average, so a symbol's absolute volume never enters into it.
		vr, ok := mtfVolumeRatio(c, 50)
		if !ok || vr > vcpVolumeDryUp {
			return NoSignal(name)
		}

		// 4. BREAKOUT. Price must be through the pivot — the high of the final
		//    contraction — on the CLOSED bar. Buying inside the base is buying
		//    a pattern that has not yet done the one thing it predicts.
		last := c[len(c)-1]
		if long && !(last.Close > pivot && price > pivot) {
			return NoSignal(name)
		}
		if !long && !(last.Close < pivot && price < pivot) {
			return NoSignal(name)
		}

		// 5. LEVELS. Stop under the final contraction's own low: that price is
		//    what says the contraction was not a contraction. Target is the
		//    measured move — the base's depth projected from the pivot — which
		//    is the conventional VCP objective and, unlike a fixed +8%, scales
		//    with how much structure was actually built.
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		baseDepth := pivot - baseExtreme
		if !long {
			baseDepth = baseExtreme - pivot
		}
		if baseDepth <= 0 {
			return NoSignal(name)
		}
		target := pivot + baseDepth
		if !long {
			target = pivot - baseDepth
		}

		dir := DirectionLong
		if !long {
			dir = DirectionShort
		}
		// The structural stop, expressed as the ATR fraction the pack's signal
		// builder expects. Widened slightly past the contraction low so a
		// one-tick undercut does not close a valid base.
		stopDist := price - floor
		if !long {
			stopDist = floor - price
		}
		if stopDist <= 0 {
			return NoSignal(name)
		}
		stopFrac := (stopDist / price) * 1.05

		return mtfSignalToTarget(name, dir, price, stopFrac/mtfStopATRMultiple, target,
			fmt.Sprintf("VCP %d contractions %.1f%%→%.1f%% tightening, vol %.0f%% of avg, pivot %.4f",
				len(depths), depths[0]*100, depths[len(depths)-1]*100, vr*100, pivot))
	}
}

// vcpDepths measures each contraction as a fraction of the high it fell from,
// and returns the top and bottom of each so the caller can pick a pivot, a
// stop and a projection from them independently.
//
// Depth is measured high-to-low as a FRACTION rather than in price, so a base
// that forms while the symbol doubles is comparable with one that forms flat —
// "9% then 6% then 4%" is the pattern, and the same sequence in rupees is not.
func vcpDepths(c []Candle, highs, lows []int, long bool) (depths, tops, bottoms []float64, ok bool) {
	// Pair each swing high with the swing low that FOLLOWS it. A low before its
	// high belongs to the previous contraction, and pairing them the other way
	// measures a rally rather than a pullback.
	for _, hi := range highs {
		var lo int = -1
		for _, l := range lows {
			if l > hi {
				lo = l
				break
			}
		}
		if lo < 0 {
			continue
		}
		var top, bottom float64
		if long {
			top, bottom = c[hi].High, c[lo].Low
		} else {
			// Mirrored: the "contraction" is a rally off a low, so the roles of
			// the two swing series swap.
			top, bottom = c[lo].Low, c[hi].High
		}
		if top <= 0 {
			continue
		}
		d := (top - bottom) / top
		if !long {
			d = (bottom - top) / bottom
		}
		if d <= 0 {
			continue
		}
		depths = append(depths, d)
		tops = append(tops, top)
		bottoms = append(bottoms, bottom)
	}
	return depths, tops, bottoms, len(depths) > 0
}

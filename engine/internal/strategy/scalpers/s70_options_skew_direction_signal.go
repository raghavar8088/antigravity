package scalpers

import (
	"fmt"
	"math"
)

// S70 — Options_Skew_Direction_Signal
//
// Research:   Bates (1991) "The Crash of '87: Was It Expected? The Evidence
//
//	from Options Markets" — documents the persistent post-crash
//	negative skew in equity-index options (downside puts bid up
//	relative to calls) as a fear/crash-premium signature; Deribit
//	market reports on crypto options skew document the analogous
//	BTC 25-delta risk-reversal behaviour (skew flips negative in
//	fear regimes, positive in euphoric up-moves).
//
// Regime:     TRENDING, RANGING (not VOLATILE, not UNKNOWN)
// Timeframes: 1h
// Logic:      This engine has no options chain feed (no true 25-delta risk
//
//	reversal data), so we build a SYNTHETIC skew proxy from the
//	asymmetry between Deribit DVOL (implied vol) and realized vol
//	on Candles1h. When DVOL runs significantly hot vs realized vol
//	WHILE price has been falling over the recent window, that is
//	read as "downside fear over-priced" (analogous to a richly bid
//	put skew) — a contrarian LONG signal. The symmetric case
//	(DVOL hot + price rising) reads as euphoric upside skew richly
//	priced — contrarian SHORT.
//
// Edge:       Skew/IV-RV asymmetry that is unconfirmed by sustained price
//
//	continuation tends to mean-revert as the priced-in fear/greed
//	fails to materialize — classic vol risk-premium harvesting,
//	applied directionally via the price-direction confirmation
//	rather than traded as a pure vol play (this engine cannot
//	trade vega directly).
//
// Data dependency: PROXY — ctx.DVOL/DVOLHistory (DVOLPopulated/DVOLHealthy)
//
//	is a live, wired feed (Deribit DVOL). True option-chain 25-delta
//	risk reversal skew is NOT available in this engine (no options
//	chain feed wired) so the synthetic DVOL-vs-RealizedVol asymmetry
//	above stands in for true skew. If DVOLPopulated/DVOLHealthy is
//	false, this strategy returns NoSignal cleanly.
type OptionsSkewDirectionSignal struct{}

func (s *OptionsSkewDirectionSignal) Name() string { return "Options_Skew_Direction_Signal" }

func (s *OptionsSkewDirectionSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	skewDVOLRVGapThreshold = 10.0  // DVOL - RV (vol points) considered a meaningfully "hot" skew proxy
	skewPriceMoveMinPct    = 0.004 // min 0.4% net move over the confirmation window to call it a directional fall/rise
)

func (s *OptionsSkewDirectionSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.DVOLPopulated || !ctx.DVOLHealthy || ctx.DVOL <= 0 {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 25 {
		return NoSignal(name)
	}

	rv := RealizedVol(ctx.Candles1h, 20)
	if rv == 0 {
		return NoSignal(name)
	}

	gap := ctx.DVOL - rv
	if gap < skewDVOLRVGapThreshold {
		return NoSignal(name)
	}

	// 3-bar confirmation: require the price direction over the last 3 closed
	// 1h bars to be consistent (not just a 1-bar blip) before reading the
	// "fear priced in while falling" / "greed priced in while rising" setup.
	n := len(ctx.Candles1h)
	c0 := ctx.Candles1h[n-4].Close
	c1 := ctx.Candles1h[n-3].Close
	c2 := ctx.Candles1h[n-2].Close
	c3 := ctx.Candles1h[n-1].Close
	if c0 <= 0 {
		return NoSignal(name)
	}

	fallingConfirmed := c1 < c0 && c2 < c1 && c3 < c2
	risingConfirmed := c1 > c0 && c2 > c1 && c3 > c2
	netMovePct := (c3 - c0) / c0

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := math.Max(1.0*atr1h, 0.003*price)

	// ── Downside fear over-priced (DVOL hot + sustained fall) → contrarian LONG ──
	if fallingConfirmed && netMovePct <= -skewPriceMoveMinPct {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.62,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"synthetic skew LONG: DVOL=%.1f RV=%.1f gap=%.1f (downside-fear richly priced), "+
					"3-bar confirmed fall %.2f%%",
				ctx.DVOL, rv, gap, netMovePct*100,
			),
		}
	}

	// ── Upside euphoria over-priced (DVOL hot + sustained rise) → contrarian SHORT ──
	if risingConfirmed && netMovePct >= skewPriceMoveMinPct {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.62,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"synthetic skew SHORT: DVOL=%.1f RV=%.1f gap=%.1f (upside-euphoria richly priced), "+
					"3-bar confirmed rise %.2f%%",
				ctx.DVOL, rv, gap, netMovePct*100,
			),
		}
	}

	return NoSignal(name)
}

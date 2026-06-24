package scalpers

import (
	"fmt"
	"math"
)

// S71 — Gamma_Exposure_Magnet
//
// Research:   SpotGamma / Tier1Alpha public research on dealer gamma
//
//	exposure (GEX) — the observation that price tends to "pin"
//	near large positive-gamma strikes (dealer hedging dampens
//	moves) and accelerates away from large negative-gamma strikes
//	(dealer hedging amplifies moves).
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:      True GEX requires a Deribit (or other) options-chain
//
//	OI-by-strike feed, which this engine does NOT have wired.
//	The full intended logic structure is implemented below so the
//	strategy activates with zero rewrites the moment a real
//	strike-level OI feed is wired: it computes a notional "GEX
//	proxy" from ctx.OpenInterest changes clustered near
//	round-number price levels (nearest $1000 increment, standing
//	in for likely options strikes, since BTC options strikes are
//	conventionally listed at $1000/$2000/$5000 increments) and
//	would bias price to "magnet" toward the nearest such level
//	when OI concentration there is high and rising.
//
// Edge:       (Once live) dealer hedging flow creates a real, well-documented
//
//	mean-reverting pull toward high-OI strikes intraday.
//
// Data dependency: PENDING — gated behind gexFeedAvailable = false below.
//
//	This engine has no Deribit OI-by-strike feed, so the proxy
//	computed here (OpenInterest concentration near round-number
//	levels) is illustrative scaffolding only, NOT a substitute
//	good enough to trade on (unlike S70's DVOL proxy, there is no
//	reasonable approximation for per-strike OI from aggregate OI
//	alone — that's why this strategy always returns NoSignal until
//	a real strike-level feed exists). Flip gexFeedAvailable to true
//	once such a feed is wired; no other code changes needed.
type GammaExposureMagnet struct{}

func (s *GammaExposureMagnet) Name() string { return "Gamma_Exposure_Magnet" }

func (s *GammaExposureMagnet) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

// gexFeedAvailable gates the entire strategy. Set to true once a real
// Deribit (or equivalent) OI-by-strike feed is wired into MarketContext —
// at that point the proxy computation below should be replaced with real
// per-strike gamma exposure, and this gate flipped on.
const gexFeedAvailable = false

// gexRoundLevelIncrement mirrors conventional BTC options strike spacing
// ($1000 increments are the most liquid/common Deribit BTC strike spacing).
const gexRoundLevelIncrement = 1000.0

// gexProxyNearestLevel returns the nearest round-number "strike-like" level
// to price, standing in for a real options strike until a strike-level OI
// feed is wired.
func gexProxyNearestLevel(price float64) float64 {
	return math.Round(price/gexRoundLevelIncrement) * gexRoundLevelIncrement
}

// gexProxyScore computes a notional "GEX proxy" magnitude from aggregate OI
// change and distance-to-level — NOT real per-strike gamma exposure, just
// the scaffolding shape the real computation will eventually fill in.
func gexProxyScore(oi, oiPrev, price, level float64) float64 {
	if oiPrev == 0 {
		return 0
	}
	oiChangePct := (oi - oiPrev) / oiPrev
	distPct := math.Abs(price-level) / price
	// Closer to the level + larger OI build = larger notional proxy score.
	proximityWeight := math.Max(0, 1-distPct/0.01) // decays to 0 beyond 1% away
	return oiChangePct * proximityWeight
}

func (s *GammaExposureMagnet) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}

	if !gexFeedAvailable {
		// No strike-level OI feed wired — cannot compute real GEX. Always
		// NoSignal until an operator wires the fetcher and flips the gate.
		return NoSignal(name)
	}

	// ── Intended logic structure (dormant until gexFeedAvailable=true) ──────
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}
	price := ctx.Price
	oi := ctx.OpenInterest
	oiPrev := ctx.OpenInterestPrev
	if oi == 0 || oiPrev == 0 || price <= 0 {
		return NoSignal(name)
	}

	level := gexProxyNearestLevel(price)
	score := gexProxyScore(oi, oiPrev, price, level)

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	slDist := math.Max(1.0*atr1h, 0.003*price)

	const gexScoreThreshold = 0.03 // placeholder threshold pending real calibration

	if score >= gexScoreThreshold && price < level {
		// Positive-gamma-like magnet above price → expect pin/pull upward.
		sl := price - slDist
		tp1 := math.Min(level, price+2.0*slDist)
		if tp1 <= price {
			return NoSignal(name)
		}
		rr := (tp1 - price) / (price - sl)
		if rr < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.55,
			StopLoss:   sl,
			TakeProfit: tp1,
			Reason: fmt.Sprintf(
				"GEX proxy magnet LONG: nearest level=%.0f, proxy score=%.4f, price=%.0f below level",
				level, score, price,
			),
		}
	}

	if score >= gexScoreThreshold && price > level {
		sl := price + slDist
		tp1 := math.Max(level, price-2.0*slDist)
		if tp1 >= price {
			return NoSignal(name)
		}
		rr := (price - tp1) / (sl - price)
		if rr < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.55,
			StopLoss:   sl,
			TakeProfit: tp1,
			Reason: fmt.Sprintf(
				"GEX proxy magnet SHORT: nearest level=%.0f, proxy score=%.4f, price=%.0f above level",
				level, score, price,
			),
		}
	}

	return NoSignal(name)
}

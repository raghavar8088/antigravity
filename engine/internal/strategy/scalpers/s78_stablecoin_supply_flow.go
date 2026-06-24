package scalpers

import (
	"fmt"
	"math"
)

// S78 — Stablecoin_Supply_Flow
//
// Research:   Ante (2021) "Bitcoin and cryptocurrency altcoin bubbles";
//
//	stablecoin-supply-flow research — growing stablecoin supply
//	represents fresh "dry powder" sidelined capital that
//	correlates with subsequent buying pressure/bullish bias.
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:      Per the spec, this is implemented as a CONVICTION MODIFIER,
//
//	NOT a standalone direction generator: it computes a base BTC
//	trend signal from EMA9 vs EMA21 on Candles1h, and when
//	ctx.StablecoinSupplyChange7dPct is populated, positive, and
//	above its own trailing-average behaviour, it BOOSTS the
//	Confidence of that base trend signal rather than producing an
//	independent directional call. With supply growth flat/negative
//	or the feed unpopulated, the base trend signal still fires
//	but at unboosted (lower) confidence.
//
// Edge:       Stablecoin supply growth as a corroborating "dry powder"
//
//	signal historically strengthens conviction in trend
//	continuation rather than acting as its own timing trigger —
//	modifier framing avoids over-trading off a slow 7d-cadence
//	feed.
//
// Data dependency: PENDING — ctx.StablecoinSupplyChange7dPct is defined in
//
//	MarketContext but the live fetcher is NOT yet wired
//	(ctx.StablecoinSupplyPopulated reads false in production
//	today). Until populated, this strategy still evaluates the
//	base EMA9/EMA21 trend signal but at the lower, unboosted
//	confidence (no modifier applied) — it does NOT NoSignal
//	outright, since the base trend trigger is independently
//	valid; only the conviction-boost feature is gated. Intended
//	source for whoever wires this next: CoinGecko or DefiLlama
//	stablecoin supply API, e.g.
//	GET https://stablecoins.llama.fi/stablecoins (DefiLlama, free,
//	public), polled daily (7d-change is a slow-moving series).
type StablecoinSupplyFlow struct{}

func (s *StablecoinSupplyFlow) Name() string { return "Stablecoin_Supply_Flow" }

func (s *StablecoinSupplyFlow) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	ssfBaseConfidence    = 0.55
	ssfBoostedConfidence = 0.70
	ssfGrowthThreshold   = 0.01 // +1% 7d stablecoin supply growth, above trailing avg
)

func (s *StablecoinSupplyFlow) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := math.Max(1.0*atr1h, 0.003*price)

	longBias := ema9 > ema21
	shortBias := ema9 < ema21
	if !longBias && !shortBias {
		return NoSignal(name)
	}

	// Conviction modifier: only applies when the stablecoin supply feed is
	// populated, positive, and above the growth threshold AND the base trend
	// direction is LONG (a stablecoin-supply boost only makes sense
	// corroborating the long/dry-powder thesis — it does not boost shorts,
	// per the spec's framing of supply growth as bullish dry powder).
	confidence := ssfBaseConfidence
	modifierApplied := false
	if ctx.StablecoinSupplyPopulated && ctx.StablecoinSupplyChange7dPct >= ssfGrowthThreshold && longBias {
		confidence = ssfBoostedConfidence
		modifierApplied = true
	}

	if longBias {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  confidence,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"base trend LONG (EMA9=%.0f>EMA21=%.0f), stablecoin supply modifier applied=%v (7dPct=%.2f%%, populated=%v)",
				ema9, ema21, modifierApplied, ctx.StablecoinSupplyChange7dPct*100, ctx.StablecoinSupplyPopulated,
			),
		}
	}

	// shortBias — supply-flow modifier intentionally not applied (per spec,
	// the conviction boost is for the bullish dry-powder corroboration case).
	sl := price + slDist
	tp1 := price - 2.0*slDist
	tp2 := price - 3.0*slDist
	if sl <= price || tp1 >= price {
		return NoSignal(name)
	}
	return Signal{
		Strategy:    name,
		Direction:   DirectionShort,
		Confidence:  ssfBaseConfidence,
		StopLoss:    sl,
		TakeProfit:  tp1,
		TakeProfit2: tp2,
		Reason: fmt.Sprintf(
			"base trend SHORT (EMA9=%.0f<EMA21=%.0f), stablecoin supply modifier not applicable to shorts",
			ema9, ema21,
		),
	}
}

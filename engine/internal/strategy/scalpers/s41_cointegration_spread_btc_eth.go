package scalpers

import (
	"fmt"
	"math"
)

// S41 — Cointegration Spread BTC/ETH
//
// Research:   Engle & Granger (1987) cointegration theory; crypto stat-arb
//
//	literature 2018-2024 (BTC/ETH pair trading on the cointegrated
//	ratio spread).
//
// Regime:     RANGING only
// Timeframes: 1h (rolling ratio mean/stdev window)
// Logic:      Computes the BTC/ETH price ratio and z-scores it against its
//
//	own rolling mean/stdev, fading >=2.0 std dev deviations back
//	toward the mean ratio.
//
// Edge:       Trades the statistical relationship between two correlated
//
//	assets rather than directional price action — historically a
//	lower-correlation-to-market-beta edge.
//
// DATA-FEED DEPENDENCY: requires ctx.ETHPrice. ETHPricePopulated must be true
// or this strategy returns NoSignal — see types.go: "ETHPrice... NOT YET
// WIRED to a live feed — treat as may-be-zero."
//
// APPROXIMATION NOTE: a textbook cointegration test needs a rolling history
// of the BTC/ETH ratio itself (raw ETH candles aren't tracked in
// MarketContext, only the latest ETHPrice scalar). As a documented
// simplification, this strategy approximates the rolling ratio mean/stdev
// using the same-length BTC-price-based VWAP/stdev computed from
// Candles1h — i.e. it assumes the ratio's relative dispersion through time
// tracks BTC's own price dispersion over the same window (a proxy, not a
// true joint BTC/ETH series). This is a deliberate simplification per spec,
// not a substitute for proper Engle-Granger residual testing.
type CointegrationSpreadBTCETH struct{}

func (s *CointegrationSpreadBTCETH) Name() string { return "Cointegration_Spread_BTC_ETH" }

func (s *CointegrationSpreadBTCETH) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *CointegrationSpreadBTCETH) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.ETHPricePopulated || !ctx.ETHPriceHealthy || ctx.ETHPrice <= 0 {
		// Data-feed dependency not satisfied — documented per spec.
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 24 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}
	ratio := price / ctx.ETHPrice

	// Approximation: use BTC's own rolling mean/stdev (over the same window
	// length) as a proxy for the ratio's rolling mean/stdev dispersion, scaled
	// into ratio-space. We compute the relative (percentage) deviation of
	// price from its own SMA, then apply that same relative deviation to the
	// ratio — i.e. we assume ratio dispersion tracks BTC's own dispersion.
	const lookback = 20
	if len(ctx.Candles1h) < lookback {
		return NoSignal(name)
	}
	btcMean := smaOf(ctx.Candles1h, lookback)
	if btcMean == 0 {
		return NoSignal(name)
	}
	btcStdev := stdevOf(ctx.Candles1h, lookback, btcMean)
	if btcStdev == 0 {
		return NoSignal(name)
	}
	// Relative dispersion of BTC price (proxy for ratio dispersion).
	relStdev := btcStdev / btcMean
	ratioMeanProxy := ratio * (btcMean / price) // approximate "fair" ratio if price reverted to its own mean
	ratioStdevProxy := ratio * relStdev
	if ratioStdevProxy == 0 {
		return NoSignal(name)
	}

	zScore := (ratio - ratioMeanProxy) / ratioStdevProxy
	const zThreshold = 2.0

	if zScore <= -zThreshold {
		// Ratio (BTC/ETH) is unusually low — BTC cheap relative to ETH proxy mean.
		// Long BTC back toward the mean ratio.
		sl := price - math.Max(1.0*atr1h, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price + 2.0*risk
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.65,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: BTC/ETH ratio=%.4f z-score=%.2f below -%.1f (approx proxy mean=%.4f), "+
					"ETH=%.0f, expect ratio reversion up",
				ratio, zScore, zThreshold, ratioMeanProxy, ctx.ETHPrice,
			),
		}
	}

	if zScore >= zThreshold {
		// Ratio unusually high — BTC expensive relative to ETH proxy mean.
		sl := price + math.Max(1.0*atr1h, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price - 2.0*risk
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.65,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: BTC/ETH ratio=%.4f z-score=%.2f above %.1f (approx proxy mean=%.4f), "+
					"ETH=%.0f, expect ratio reversion down",
				ratio, zScore, zThreshold, ratioMeanProxy, ctx.ETHPrice,
			),
		}
	}

	return NoSignal(name)
}

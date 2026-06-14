package strategy

import "fmt"

// S6 — CVD Divergence Sniper
//
// Regime:     ALL (Trending, Ranging, Volatile) — skips UNKNOWN only
// Timeframes: 5m (CVD divergence) + 15m (price structure)
// Logic:      Price makes new high/low on 15m but CVD fails to confirm.
//             MACD histogram flips direction simultaneously. Order book
//             imbalance tilted opposite to price move.
// Edge:       CVD divergence is one of the purest order flow signals —
//             it reveals when price movement is not backed by real volume.
//             Works in all market conditions.

type CVDDivergenceSniper struct{}

func (s *CVDDivergenceSniper) Name() string { return "CVD_Divergence_Sniper" }

func (s *CVDDivergenceSniper) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *CVDDivergenceSniper) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	// Fires in all regimes except UNKNOWN
	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 25 || len(ctx.Candles5m) < 35 {
		return NoSignal(name)
	}

	// ── 15m price structure: detect new high or new low ───────────────────────
	// Compare last candle vs prior 10 candles
	recent15m := ctx.Candles15m[len(ctx.Candles15m)-11 : len(ctx.Candles15m)-1]
	last15m := ctx.Candles15m[len(ctx.Candles15m)-1]
	priorHigh := SwingHigh(recent15m, 10)
	priorLow := SwingLow(recent15m, 10)

	newHigh := last15m.High > priorHigh
	newLow := last15m.Low < priorLow

	if !newHigh && !newLow {
		return NoSignal(name) // no structure extreme to diverge from
	}

	// ── MACD histogram: must be flipping direction ────────────────────────────
	macd5m := MACD(ctx.Candles5m)
	macd5mPrev := MACD(ctx.Candles5m[:len(ctx.Candles5m)-1])
	// Flipping = histogram crossing zero or changing sign
	macdFlippingBearish := macd5m.Histogram < 0 && macd5mPrev.Histogram >= 0
	macdFlippingBullish := macd5m.Histogram > 0 && macd5mPrev.Histogram <= 0

	// ── Order book: imbalance opposite to price move ──────────────────────────
	ob := ctx.OrderBook
	// Imbalance: positive = more bids, negative = more asks
	obFavorsBuyers := ob.Imbalance > 0.15  // meaningful bid dominance
	obFavorsSellers := ob.Imbalance < -0.15 // meaningful ask dominance

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	// ── BEARISH DIVERGENCE: new price high, CVD fails to follow ──────────────
	// → Hidden selling — expect reversal down
	cvdBearDiv := CVDDivergesBearish(last15m.High, priorHigh, ctx.CVD, ctx.CVDPrev)
	if newHigh && cvdBearDiv && macdFlippingBearish && obFavorsSellers {
		sl := last15m.High + 0.25*atr15m
		tp := priorLow // revert to prior swing low
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward < 2*risk {
			return NoSignal(name)
		}
		// Confidence boost if funding rate also diverges
		confidence := 0.74
		if ctx.FundingRate > 0.005 { // longs over-extended
			confidence = 0.82
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: confidence,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CVD bearish divergence: price new high=%.0f (prev=%.0f) "+
					"but CVD=%.0f<prev=%.0f, MACD hist flipping bearish (%.4f→%.4f), "+
					"OB imbalance=%.2f (sellers), regime=%s",
				last15m.High, priorHigh, ctx.CVD, ctx.CVDPrev,
				macd5mPrev.Histogram, macd5m.Histogram, ob.Imbalance, ctx.Regime,
			),
		}
	}

	// ── BULLISH DIVERGENCE: new price low, CVD holds higher ──────────────────
	// → Hidden buying — expect reversal up
	cvdBullDiv := CVDDivergesBullish(last15m.Low, priorLow, ctx.CVD, ctx.CVDPrev)
	if newLow && cvdBullDiv && macdFlippingBullish && obFavorsBuyers {
		sl := last15m.Low - 0.25*atr15m
		tp := priorHigh
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward < 2*risk {
			return NoSignal(name)
		}
		confidence := 0.74
		if ctx.FundingRate < -0.005 { // shorts over-extended
			confidence = 0.82
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: confidence,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CVD bullish divergence: price new low=%.0f (prev=%.0f) "+
					"but CVD=%.0f>prev=%.0f, MACD hist flipping bullish (%.4f→%.4f), "+
					"OB imbalance=%.2f (buyers), regime=%s",
				last15m.Low, priorLow, ctx.CVD, ctx.CVDPrev,
				macd5mPrev.Histogram, macd5m.Histogram, ob.Imbalance, ctx.Regime,
			),
		}
	}

	return NoSignal(name)
}

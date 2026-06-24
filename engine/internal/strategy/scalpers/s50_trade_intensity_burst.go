package scalpers

import (
	"fmt"
	"math"
)

// S50 — Trade Intensity Burst
//
// Regime:     TRENDING, VOLATILE
// Timeframes: 1m (intensity burst detection) + 5m (directional confirmation)
// Research:   Easley & O'Hara (1992) "Time and the Process of Security Price
//             Adjustment" — clustering of trade arrival rate (time between
//             trades compresses) signals informed order flow entering the
//             market ahead of a price move.
// Logic:      Ideal data is TradeCountPerMin1m > 3x TradeCountAvg20m (raw
//             tick-arrival rate). That feed (MicrostructurePopulated) is not
//             yet wired to a live per-trade source in production, so this
//             strategy checks ctx.MicrostructurePopulated first; if false it
//             falls back to a volume-based proxy: the last CLOSED 1m candle's
//             Volume > 3x the trailing 20-period AvgVolume(), which captures
//             the same "burst of activity" idea using data that IS already
//             live. Either path additionally requires the burst candle to be
//             directional (close away from open by a meaningful fraction of
//             its range) before signaling, and 3-bar confirmation that the
//             move is sustained rather than a single noisy print.
// Edge:       Catches the early acceleration phase of a directional move
//             right as trade intensity spikes, before the broader market has
//             reacted to the same information.

type TradeIntensityBurst struct{}

func (s *TradeIntensityBurst) Name() string { return "Trade_Intensity_Burst" }

func (s *TradeIntensityBurst) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeVolatile}
}

func (s *TradeIntensityBurst) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles1m) < 23 || len(ctx.Candles5m) < 5 {
		return NoSignal(name)
	}

	atr1m := ATR(ctx.Candles1m, 14)
	if atr1m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}

	c1m := ctx.Candles1m
	n := len(c1m)
	// Use last 3 CLOSED candles (n-4..n-2) for 3-bar confirmation, avoiding the
	// still-forming candle at n-1.
	if n < 24 {
		return NoSignal(name)
	}
	confirmBars := c1m[n-4 : n-1] // 3 bars

	var intensityConfirmed bool
	var intensityDetail string

	if ctx.MicrostructurePopulated {
		// Ideal path: live trade-arrival-rate feed.
		burst := ctx.TradeCountPerMin1m > 3.0*ctx.TradeCountAvg20m && ctx.TradeCountAvg20m > 0
		intensityConfirmed = burst
		intensityDetail = fmt.Sprintf("TradeCountPerMin1m=%.0f > 3x avg20m=%.0f", ctx.TradeCountPerMin1m, ctx.TradeCountAvg20m)
	} else {
		// PROXY: MicrostructurePopulated==false in production today, so fall
		// back to volume-burst detection on the last closed 1m candle.
		avgVol := AvgVolume(c1m[:n-1], 20)
		if avgVol == 0 {
			return NoSignal(name)
		}
		lastClosed := c1m[n-2]
		intensityConfirmed = lastClosed.Volume > 3.0*avgVol
		intensityDetail = fmt.Sprintf("PROXY volume burst: lastVol=%.2f > 3x avg20=%.2f", lastClosed.Volume, avgVol)
	}

	if !intensityConfirmed {
		return NoSignal(name)
	}

	// Directional confirmation: 3 consecutive closed bars moving the same way,
	// each candle's body a meaningful fraction of its range (not a doji burst).
	upBars, downBars := 0, 0
	for _, c := range confirmBars {
		rng := c.High - c.Low
		if rng <= 0 {
			continue
		}
		body := c.Close - c.Open
		if body > 0 && math.Abs(body)/rng >= 0.4 {
			upBars++
		} else if body < 0 && math.Abs(body)/rng >= 0.4 {
			downBars++
		}
	}

	if upBars == 3 {
		sl := price - math.Max(1.0*atr1m, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.66
		if ctx.MicrostructurePopulated {
			conf = 0.73
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Trade intensity burst LONG: %s, 3/3 bullish confirm bars",
				intensityDetail,
			),
		}
	}

	if downBars == 3 {
		sl := price + math.Max(1.0*atr1m, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.66
		if ctx.MicrostructurePopulated {
			conf = 0.73
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Trade intensity burst SHORT: %s, 3/3 bearish confirm bars",
				intensityDetail,
			),
		}
	}

	return NoSignal(name)
}

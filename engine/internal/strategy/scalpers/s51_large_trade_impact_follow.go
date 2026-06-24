package scalpers

import (
	"fmt"
	"math"
)

// S51 — Large Trade Impact Follow
//
// Regime:     TRENDING, VOLATILE
// Timeframes: 1m (large-trade detection) + 5m (trend filter)
// Research:   Kyle (1985) "Continuous Auctions and Insider Trading"; Makarov
//             & Schoar (2020) — large individual trades carry disproportionate
//             price impact and often signal informed flow; following the
//             direction of a large trade in line with the prevailing
//             short-term trend captures continuation of that impact.
// Logic:      Ideal data is LastTradeSizeRatio > 5 (single trade vs trailing
//             100-trade average). That feed requires MicrostructurePopulated;
//             since it is not yet wired live, this strategy falls back to a
//             proxy: a single CLOSED 1m candle with Volume > 5x its trailing
//             20-bar average AND a strong directional close (body >= 50% of
//             range), only trading WITH the short-term trend defined by
//             EMA9 vs EMA21 on Candles5m. Tight SL per spec (large-trade
//             impact strategies need fast invalidation since the edge decays
//             quickly once the order is absorbed).
// Edge:       Rides the momentum created by a large aggressive order while the
//             broader trend agrees, exiting quickly if wrong (tight stop).

type LargeTradeImpactFollow struct{}

func (s *LargeTradeImpactFollow) Name() string { return "Large_Trade_Impact_Follow" }

func (s *LargeTradeImpactFollow) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeVolatile}
}

func (s *LargeTradeImpactFollow) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles1m) < 22 || len(ctx.Candles5m) < 22 {
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

	ema9_5m := EMA(ctx.Candles5m, 9)
	ema21_5m := EMA(ctx.Candles5m, 21)
	if ema9_5m == 0 || ema21_5m == 0 {
		return NoSignal(name)
	}
	trendUp := ema9_5m > ema21_5m
	trendDown := ema9_5m < ema21_5m

	c1m := ctx.Candles1m
	n := len(c1m)

	var largeTradeUp, largeTradeDown bool
	var detail string

	if ctx.MicrostructurePopulated {
		// Ideal path: live last-trade-size-ratio feed. Direction inferred from
		// the most recent closed candle's body since the feed itself is scalar.
		if ctx.LastTradeSizeRatio > 5 {
			lastClosed := c1m[n-2]
			body := lastClosed.Close - lastClosed.Open
			rng := lastClosed.High - lastClosed.Low
			if rng > 0 && math.Abs(body)/rng >= 0.5 {
				largeTradeUp = body > 0
				largeTradeDown = body < 0
			}
		}
		detail = fmt.Sprintf("LastTradeSizeRatio=%.1f > 5", ctx.LastTradeSizeRatio)
	} else {
		// PROXY: MicrostructurePopulated==false in production today.
		avgVol := AvgVolume(c1m[:n-1], 20)
		if avgVol == 0 {
			return NoSignal(name)
		}
		lastClosed := c1m[n-2]
		volSpike := lastClosed.Volume > 5.0*avgVol
		body := lastClosed.Close - lastClosed.Open
		rng := lastClosed.High - lastClosed.Low
		strongClose := rng > 0 && math.Abs(body)/rng >= 0.5
		if volSpike && strongClose {
			largeTradeUp = body > 0
			largeTradeDown = body < 0
		}
		detail = fmt.Sprintf("PROXY large-trade: vol=%.2f > 5x avg20=%.2f, bodyRatio strong close", lastClosed.Volume, avgVol)
	}

	// Tight SL per spec: still respects the mandatory minimum floor
	// (max(1.0xATR, 0.3% price)) but stays at the floor rather than wider.
	slDistMin := math.Max(1.0*atr1m, 0.003*price)

	if largeTradeUp && trendUp {
		sl := price - slDistMin
		tp := price + 2.0*slDistMin
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.65
		if ctx.MicrostructurePopulated {
			conf = 0.72
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Large trade impact follow LONG: %s, trend up (EMA9=%.0f>EMA21=%.0f 5m)",
				detail, ema9_5m, ema21_5m,
			),
		}
	}

	if largeTradeDown && trendDown {
		sl := price + slDistMin
		tp := price - 2.0*slDistMin
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.65
		if ctx.MicrostructurePopulated {
			conf = 0.72
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Large trade impact follow SHORT: %s, trend down (EMA9=%.0f<EMA21=%.0f 5m)",
				detail, ema9_5m, ema21_5m,
			),
		}
	}

	return NoSignal(name)
}

package scalpers

import (
	"fmt"
	"math"
)

// S59 — Hidden Liquidity Detection
//
// Regime:     RANGING, VOLATILE
// Timeframes: 5m (touch/bounce detection over ~1hr lookback)
// Research:   Bessembinder, Panayides & Venkataraman (2009) "Hidden
//             Liquidity: An Analysis of Order Exposure Strategies in
//             Electronic Stock Markets" — repeated price rejection at the
//             same level without that level ever printing as visible resting
//             size implies hidden/iceberg liquidity is defending it; once a
//             level has proven itself across multiple touches, subsequent
//             touches are higher-probability bounce trades.
// Logic:      Price touches a similar level (within 0.2%) 3+ times within the
//             last 12 Candles5m bars (~1hr) and bounces (closes back away
//             from the level) each time -> trade the bounce on the next
//             touch. Implemented using SwingLow/SwingHigh-style extremes:
//             scans the trailing 12 5m candles for lows (support case) /
//             highs (resistance case) that cluster within 0.2% of each other,
//             requires each touch's candle to close back away from the
//             cluster level (bounce confirmation), and the CURRENT bar must
//             be touching that same level again to trigger the entry.
// Edge:       Trades proven support/resistance levels that have already
//             demonstrated repeated defense, rather than guessing untested
//             levels.

type HiddenLiquidityDetection struct{}

func (s *HiddenLiquidityDetection) Name() string { return "Hidden_Liquidity_Detection" }

func (s *HiddenLiquidityDetection) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

const hiddenLiqClusterPct = 0.002 // 0.2%

// clusterTouches counts how many of the given candle extremes (lows or
// highs) fall within hiddenLiqClusterPct of the reference level AND closed
// back away from that level (bounce confirmation). Returns the touch count.
func clusterTouchesLow(candles []Candle, level float64) int {
	count := 0
	for _, c := range candles {
		if level <= 0 {
			continue
		}
		if math.Abs(c.Low-level)/level <= hiddenLiqClusterPct {
			// Bounce confirmation: closed back above the touched low.
			if c.Close > c.Low+(c.High-c.Low)*0.3 {
				count++
			}
		}
	}
	return count
}

func clusterTouchesHigh(candles []Candle, level float64) int {
	count := 0
	for _, c := range candles {
		if level <= 0 {
			continue
		}
		if math.Abs(c.High-level)/level <= hiddenLiqClusterPct {
			// Bounce confirmation: closed back below the touched high.
			if c.Close < c.High-(c.High-c.Low)*0.3 {
				count++
			}
		}
	}
	return count
}

func (s *HiddenLiquidityDetection) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	c5 := ctx.Candles5m
	n := len(c5)
	if n < 13 {
		return NoSignal(name)
	}

	atr5m := ATR(c5, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}

	lookback := c5[n-13 : n-1] // last 12 closed candles (~1hr), excludes still-forming bar
	current := c5[n-1]

	slDistMin := math.Max(1.0*atr5m, 0.003*price)

	// ── Support cluster: use the current bar's low as the candidate level and
	// count how many of the prior 12 bars touched-and-bounced near it.
	supportLevel := current.Low
	supportTouches := clusterTouchesLow(lookback, supportLevel)
	if supportTouches >= 3 && math.Abs(price-supportLevel)/supportLevel <= hiddenLiqClusterPct {
		sl := supportLevel - slDistMin
		// Ensure SL is genuinely below price with the mandatory minimum floor.
		if price-sl < slDistMin {
			sl = price - slDistMin
		}
		tp := price + 2.0*(price-sl)
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Hidden liquidity LONG: support level=%.0f touched+bounced %d times in last 12 5m bars (~1hr), current price=%.0f retesting",
				supportLevel, supportTouches, price,
			),
		}
	}

	// ── Resistance cluster: use the current bar's high as the candidate level.
	resistanceLevel := current.High
	resistanceTouches := clusterTouchesHigh(lookback, resistanceLevel)
	if resistanceTouches >= 3 && math.Abs(price-resistanceLevel)/resistanceLevel <= hiddenLiqClusterPct {
		sl := resistanceLevel + slDistMin
		if sl-price < slDistMin {
			sl = price + slDistMin
		}
		tp := price - 2.0*(sl-price)
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Hidden liquidity SHORT: resistance level=%.0f touched+bounced %d times in last 12 5m bars (~1hr), current price=%.0f retesting",
				resistanceLevel, resistanceTouches, price,
			),
		}
	}

	return NoSignal(name)
}

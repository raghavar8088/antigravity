package scalpers

import (
	"fmt"
	"math"
)

// S96 — On Balance Volume Divergence
//
// Citation:   Joseph Granville, "New Key to Stock Market Profits" (1963).
//             OBV is one of the most validated volume indicators in academic
//             literature. OBV-price divergence is a leading indicator of trend
//             change, extensively documented across asset classes.
// Regime:     ALL regimes
// Timeframes: 15m
// Logic:      Price makes new 20-bar high BUT OBV does not confirm (bearish
//             divergence) over 3+ bars = SHORT. Price makes new 20-bar low
//             but OBV does NOT decline (bullish divergence) = LONG.

type OBVDivergence struct{}

func (s *OBVDivergence) Name() string { return "OBV_Divergence" }

func (s *OBVDivergence) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *OBVDivergence) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	// Compute OBV at current bar and 3 bars ago (for divergence check)
	obvNow := OBV(candles)
	obvMinus1 := OBV(candles[:n-1])
	obvMinus2 := OBV(candles[:n-2])
	obvMinus3 := OBV(candles[:n-3])

	// Price highs and lows at current and -3 bars
	priceNow := candles[n-1].Close
	priceMinus3 := candles[n-4].Close

	swing20HighNow := SwingHigh(candles, 20)
	swing20HighPrev := SwingHigh(candles[:n-3], 20)
	swing20LowNow := SwingLow(candles, 20)
	swing20LowPrev := SwingLow(candles[:n-3], 20)

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// Bearish OBV divergence: price makes new 20-bar high but OBV is declining
	// Check 3-bar divergence: price trend up, OBV trend down
	priceTrendUp := priceNow > priceMinus3 && swing20HighNow >= swing20HighPrev
	obvTrendDown := obvNow < obvMinus1 && obvMinus1 < obvMinus2 && obvMinus2 < obvMinus3

	if priceTrendUp && obvTrendDown {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := candles[n-1].High + 0.2*atr
		if sl-price < minSL {
			sl = price + minSL
		}
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		if (price-tp)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"OBV bearish divergence: price %.0f→%.0f (new high), OBV %.0f→%.0f (declining) 3-bar",
				priceMinus3, priceNow, obvMinus3, obvNow,
			),
		}
	}

	// Bullish OBV divergence: price makes new 20-bar low but OBV is rising
	priceTrendDown := priceNow < priceMinus3 && swing20LowNow <= swing20LowPrev
	obvTrendUp := obvNow > obvMinus1 && obvMinus1 > obvMinus2 && obvMinus2 > obvMinus3

	if priceTrendDown && obvTrendUp {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := candles[n-1].Low - 0.2*atr
		if price-sl < minSL {
			sl = price - minSL
		}
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		if (tp-price)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"OBV bullish divergence: price %.0f→%.0f (new low), OBV %.0f→%.0f (rising) 3-bar",
				priceMinus3, priceNow, obvMinus3, obvNow,
			),
		}
	}

	return NoSignal(name)
}

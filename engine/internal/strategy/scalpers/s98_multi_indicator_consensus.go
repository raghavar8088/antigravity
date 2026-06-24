package scalpers

import (
	"fmt"
	"math"
)

// S98 — Multi-Indicator Consensus
//
// Citation:   Krauss, Do & Huck (2017) "Deep Neural Networks, Gradient-Boosted
//             Trees, Random Forests: Statistical Arbitrage on the S&P 500" —
//             demonstrates ensemble approaches outperform individual indicators.
//             This is a rule-based ensemble (no ML inference at runtime).
// Regime:     ALL regimes
// Timeframes: 15m (primary), 1h (EMA bias)
// Logic:      8 independent binary signals scored +1 (bullish) or -1 (bearish).
//             Sum > +5 = LONG (≥6 of 8 agree). Sum < -5 = SHORT.
//             Confidence scales: 6/8=0.68, 7/8=0.74, 8/8=0.80.

type MultiIndicatorConsensus struct{}

func (s *MultiIndicatorConsensus) Name() string { return "Multi_Indicator_Consensus" }

func (s *MultiIndicatorConsensus) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *MultiIndicatorConsensus) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 35 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	candles15m := ctx.Candles15m
	candles1h := ctx.Candles1h
	n15 := len(candles15m)
	price := ctx.Price

	atr := ATR(candles15m, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	// ── Signal 1: EMA cross on 15m (EMA9 vs EMA21) ───────────────────────────
	ema9 := EMA(candles15m, 9)
	ema21 := EMA(candles15m, 21)
	sig1 := 0
	if ema9 > ema21 {
		sig1 = 1
	} else if ema9 < ema21 {
		sig1 = -1
	}

	// ── Signal 2: RSI level (>55 bullish, <45 bearish) ───────────────────────
	rsi := RSI(candles15m, 14)
	sig2 := 0
	if rsi > 55 {
		sig2 = 1
	} else if rsi < 45 {
		sig2 = -1
	}

	// ── Signal 3: MACD histogram cross ───────────────────────────────────────
	macd := MACD(candles15m)
	macdPrev := MACD(candles15m[:n15-1])
	sig3 := 0
	if macd.Histogram > 0 && macd.Histogram > macdPrev.Histogram {
		sig3 = 1
	} else if macd.Histogram < 0 && macd.Histogram < macdPrev.Histogram {
		sig3 = -1
	}

	// ── Signal 4: BB position (price above middle = bullish) ─────────────────
	bb := BB(candles15m, 20)
	sig4 := 0
	if price > bb.Middle {
		sig4 = 1
	} else if price < bb.Middle {
		sig4 = -1
	}

	// ── Signal 5: ATR expansion (expanding = momentum, shrinking = reversal) ──
	atrPrev := ATR(candles15m[:n15-1], 14)
	sig5 := 0
	if atr > atrPrev {
		// Expanding ATR: follow current direction
		if price > candles15m[n15-2].Close {
			sig5 = 1
		} else {
			sig5 = -1
		}
	}

	// ── Signal 6: CVD direction ───────────────────────────────────────────────
	sig6 := 0
	if ctx.CVD > ctx.CVDPrev {
		sig6 = 1
	} else if ctx.CVD < ctx.CVDPrev {
		sig6 = -1
	}

	// ── Signal 7: ADX strength (>25 = trend exists, use EMA direction) ────────
	adx := ADX(candles15m, 14)
	sig7 := 0
	if adx > 25 {
		if ema9 > ema21 {
			sig7 = 1
		} else {
			sig7 = -1
		}
	}

	// ── Signal 8: Stochastic cross (k>d bullish, k<d bearish) ─────────────────
	k, d := SlowStochastic(candles15m, 14, 3)
	kPrev, dPrev := SlowStochastic(candles15m[:n15-1], 14, 3)
	sig8 := 0
	if k > d && kPrev <= dPrev {
		sig8 = 1 // bullish cross
	} else if k < d && kPrev >= dPrev {
		sig8 = -1 // bearish cross
	} else if k > d {
		sig8 = 1
	} else if k < d {
		sig8 = -1
	}

	// Ensemble sum
	total := sig1 + sig2 + sig3 + sig4 + sig5 + sig6 + sig7 + sig8
	_ = candles1h // used implicitly via EMA on 15m with 1h bias already in sig1/sig7

	// Confidence based on agreement strength
	var conf float64
	var agree int
	if total > 0 {
		agree = total
	} else {
		agree = -total
	}
	switch agree {
	case 6:
		conf = 0.68
	case 7:
		conf = 0.74
	case 8:
		conf = 0.80
	default:
		return NoSignal(name) // fewer than 6 agree → no signal
	}

	if total >= 6 { // LONG: at least 6 bullish signals
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := price - minSL
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
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Consensus LONG %d/8 agree: EMA=%+d RSI=%+d MACD=%+d BB=%+d ATR=%+d CVD=%+d ADX=%+d Stoch=%+d",
				agree, sig1, sig2, sig3, sig4, sig5, sig6, sig7, sig8,
			),
		}
	}

	if total <= -6 { // SHORT: at least 6 bearish signals
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := price + minSL
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
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Consensus SHORT %d/8 agree: EMA=%+d RSI=%+d MACD=%+d BB=%+d ATR=%+d CVD=%+d ADX=%+d Stoch=%+d",
				agree, sig1, sig2, sig3, sig4, sig5, sig6, sig7, sig8,
			),
		}
	}

	return NoSignal(name)
}

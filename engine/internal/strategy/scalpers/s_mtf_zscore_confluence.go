package scalpers

import (
	"fmt"
	"sync"
)

// S23 — MTF_ZScore_Confluence
//
// Regime:     RANGING only
// Timeframes: 5m + 15m + 1h (simultaneous z-score confluence)
// Logic:      Multi-timeframe z-score confluence is a standard quant factor
//             technique: requiring a statistical extreme ((close-SMA)/stdev)
//             to align across multiple independent timeframes simultaneously
//             is a well-documented way to raise conviction and filter out
//             single-timeframe noise (the same idea underlies multi-factor
//             confirmation models in equities/FX quant desks). Here:
//               |z5m| > 2.0 AND |z15m| > 2.0 AND |z1h| > 2.0, all same sign
//               z > +2.0 on all three -> extreme overbought -> fade SHORT
//               z < -2.0 on all three -> extreme oversold  -> fade LONG
//             A 3-bar confirmation window requires the alignment to PERSIST
//             across the last 3 evaluation cycles (not a single snapshot)
//             before firing, reducing whipsaw on transient spikes.

type MTFZScoreConfluence struct {
	// confirmHistory tracks the last few aligned-direction readings so the
	// strategy can require persistence rather than a single-bar snapshot.
	// Direction: +1 = aligned overbought (short setup), -1 = aligned
	// oversold (long setup), 0 = not aligned.
	mu             sync.Mutex
	confirmHistory []int
}

func (s *MTFZScoreConfluence) Name() string { return "MTF_ZScore_Confluence" }

func (s *MTFZScoreConfluence) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

const mtfZScoreThreshold = 2.0
const mtfZScoreConfirmBars = 3

func (s *MTFZScoreConfluence) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 21 || len(ctx.Candles15m) < 21 || len(ctx.Candles1h) < 21 {
		return NoSignal(name)
	}

	z5m, z15m, z1h := ZScoreMultiTimeframe(ctx.Candles5m, ctx.Candles15m, ctx.Candles1h)
	if z5m == 0 || z15m == 0 || z1h == 0 {
		return NoSignal(name)
	}

	aligned := 0
	if z5m > mtfZScoreThreshold && z15m > mtfZScoreThreshold && z1h > mtfZScoreThreshold {
		aligned = 1 // overbought confluence -> fade short
	} else if z5m < -mtfZScoreThreshold && z15m < -mtfZScoreThreshold && z1h < -mtfZScoreThreshold {
		aligned = -1 // oversold confluence -> fade long
	}

	// Track persistence across recent evaluation cycles (3-bar confirmation).
	s.mu.Lock()
	s.confirmHistory = append(s.confirmHistory, aligned)
	if len(s.confirmHistory) > mtfZScoreConfirmBars {
		s.confirmHistory = s.confirmHistory[len(s.confirmHistory)-mtfZScoreConfirmBars:]
	}
	historySnapshot := make([]int, len(s.confirmHistory))
	copy(historySnapshot, s.confirmHistory)
	s.mu.Unlock()

	if len(historySnapshot) < mtfZScoreConfirmBars {
		return NoSignal(name)
	}
	persistent := historySnapshot[0]
	for _, v := range historySnapshot {
		if v != persistent || v == 0 {
			persistent = 0
			break
		}
	}
	if persistent == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	if persistent == -1 {
		// Oversold confluence persisted 3 bars -> fade LONG.
		sl := price - maxF(1.0*atr5m, 0.003*price)
		risk := price - sl
		tp := price + 2.0*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"MTF z-score confluence oversold persisted %d bars: z5m=%.2f z15m=%.2f z1h=%.2f (all<-2.0) -> fade LONG",
				mtfZScoreConfirmBars, z5m, z15m, z1h,
			),
		}
	}

	// persistent == 1: overbought confluence persisted 3 bars -> fade SHORT.
	sl := price + maxF(1.0*atr5m, 0.003*price)
	risk := sl - price
	tp := price - 2.0*risk
	if risk <= 0 {
		return NoSignal(name)
	}
	return Signal{
		Strategy:   name,
		Direction:  DirectionShort,
		Confidence: 0.74,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason: fmt.Sprintf(
			"MTF z-score confluence overbought persisted %d bars: z5m=%.2f z15m=%.2f z1h=%.2f (all>2.0) -> fade SHORT",
			mtfZScoreConfirmBars, z5m, z15m, z1h,
		),
	}
}

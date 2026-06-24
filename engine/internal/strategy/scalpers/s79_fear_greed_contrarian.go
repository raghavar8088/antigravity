package scalpers

import (
	"fmt"
	"math"
)

// S79 — Fear_Greed_Contrarian
//
// Research:   alternative.me Bitcoin Fear & Greed Index methodology; Da,
//
//	Engelberg & Gao (2015) "In Search of Attention" investor-
//	attention/sentiment research — extreme retail sentiment
//	readings (fear or greed) have historically been contrarian
//	indicators at market turning points.
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h (sentiment is daily-cadence; this is a bias filter, not a
//
//	scalping trigger by itself)
//
// Logic:      Uses ctx.FearGreedIndex / ctx.FearGreedHistory (check
//
//	FearGreedPopulated first). FearGreedIndex < 20 sustained for
//	3+ consecutive FearGreedHistory readings ("Extreme Fear") →
//	LONG contrarian. FearGreedIndex > 80 sustained for 3+
//	consecutive readings ("Extreme Greed") → SHORT contrarian.
//
// Edge:       Requiring 3 consecutive extreme daily readings (not a single
//
//	day's spike) filters out noisy single-day sentiment blips and
//	targets genuinely persistent extreme-sentiment regimes, which
//	is when the contrarian edge has historically been strongest.
//
// Data dependency: PENDING — ctx.FearGreedIndex / ctx.FearGreedHistory are
//
//	defined in MarketContext but the live fetcher is NOT yet
//	wired (ctx.FearGreedPopulated reads false in production
//	today). This strategy checks FearGreedPopulated first and
//	returns NoSignal cleanly when false. Intended source for
//	whoever wires this next: GET https://api.alternative.me/fng/
//	(alternative.me Fear & Greed Index, free public daily
//	endpoint, fetch once/day — matches FearGreedHistory's daily
//	cadence per types.go).
type FearGreedContrarian struct{}

func (s *FearGreedContrarian) Name() string { return "Fear_Greed_Contrarian" }

func (s *FearGreedContrarian) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	fgcExtremeFearThreshold  = 20.0
	fgcExtremeGreedThreshold = 80.0
)

func (s *FearGreedContrarian) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.FearGreedPopulated {
		// Feed not wired yet — see header "Data dependency: PENDING".
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}
	// 3-bar (3-day) confirmation: require at least 3 history readings.
	if len(ctx.FearGreedHistory) < 3 {
		return NoSignal(name)
	}

	hist := ctx.FearGreedHistory
	last3 := hist[len(hist)-3:]
	current := ctx.FearGreedIndex

	allExtremeFear := last3[0] < fgcExtremeFearThreshold && last3[1] < fgcExtremeFearThreshold && last3[2] < fgcExtremeFearThreshold
	allExtremeGreed := last3[0] > fgcExtremeGreedThreshold && last3[1] > fgcExtremeGreedThreshold && last3[2] > fgcExtremeGreedThreshold

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := math.Max(1.0*atr1h, 0.003*price)

	if current < fgcExtremeFearThreshold && allExtremeFear {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.63,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"fear/greed contrarian LONG: index=%.0f, 3-day sustained Extreme Fear (<%.0f): %.0f,%.0f,%.0f",
				current, fgcExtremeFearThreshold, last3[0], last3[1], last3[2],
			),
		}
	}

	if current > fgcExtremeGreedThreshold && allExtremeGreed {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.63,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"fear/greed contrarian SHORT: index=%.0f, 3-day sustained Extreme Greed (>%.0f): %.0f,%.0f,%.0f",
				current, fgcExtremeGreedThreshold, last3[0], last3[1], last3[2],
			),
		}
	}

	return NoSignal(name)
}

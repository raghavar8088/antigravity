package scalpers

import (
	"fmt"
	"math"
)

// S64 — Shannon Entropy Regime
//
// Research: Lo, A.W. & MacKinlay, A.C., "A Non-Random Walk Down Wall Street"
// (1999) — return-series predictability is regime-dependent; low-entropy
// (more ordered/predictable) windows are more tradeable than high-entropy
// (near-random) windows.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: uses the deterministic ShannonEntropy()
// indicator directly as a fixed-threshold gate, no learned model.
//
// Regime:     TRENDING, RANGING
// Timeframes: 15m
// Logic:      ShannonEntropy(period=30, bins=8) on Candles15m. Only trade
//             (EMA9 vs EMA21 trend direction as the underlying signal) when
//             entropy < 0.6 (fixed-threshold proxy for "bottom tercile of its
//             own trailing distribution" — see SIMPLIFICATION note below).
// Edge:       Lower-entropy windows indicate a more ordered/structured price
//             process (less random-walk-like), where trend-following signals
//             are more reliable.
//
// SIMPLIFICATION NOTE: True "bottom tercile vs trailing distribution" needs
// persisted state (the last ~20 entropy readings) that this strategy does
// not retain between Evaluate() calls. As a documented approximation, we
// instead gate on a fixed absolute threshold (entropy < 0.6 of the
// max-possible 1.0) as a proxy for "low entropy relative to typical
// readings." This is a simplification of the literature's relative-ranking
// approach, not a true tercile calculation.

type ShannonEntropyRegime struct{}

func (s *ShannonEntropyRegime) Name() string { return "Shannon_Entropy_Regime" }

func (s *ShannonEntropyRegime) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *ShannonEntropyRegime) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 31 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	entropy := ShannonEntropy(c15, 30, 8)

	// Fixed-threshold proxy for "bottom tercile" — see header simplification note.
	const entropyThreshold = 0.6
	if entropy >= entropyThreshold {
		return NoSignal(name)
	}

	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	ema9 := EMA(c15, 9)
	ema21 := EMA(c15, 21)
	if ema9 == 0 || ema21 == 0 {
		return NoSignal(name)
	}

	// 3-bar confirmation of trend direction
	n := len(c15)
	last3Up := c15[n-1].Close > c15[n-1].Open && c15[n-2].Close > c15[n-2].Open && c15[n-3].Close > c15[n-3].Open
	last3Down := c15[n-1].Close < c15[n-1].Open && c15[n-2].Close < c15[n-2].Open && c15[n-3].Close < c15[n-3].Open

	if ema9 > ema21 && last3Up {
		sl := price - math.Max(1.0*atr, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Shannon entropy regime LONG: entropy=%.3f <%.2f (low-entropy/ordered window), EMA9>EMA21 3-bar confirmed uptrend",
				entropy, entropyThreshold,
			),
		}
	}

	if ema9 < ema21 && last3Down {
		sl := price + math.Max(1.0*atr, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Shannon entropy regime SHORT: entropy=%.3f <%.2f (low-entropy/ordered window), EMA9<EMA21 3-bar confirmed downtrend",
				entropy, entropyThreshold,
			),
		}
	}

	return NoSignal(name)
}

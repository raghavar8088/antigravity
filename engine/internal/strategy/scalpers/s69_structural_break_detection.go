package scalpers

import (
	"fmt"
	"math"
)

// S69 — Structural Break Detection
//
// Research: Page, E.S. (1954) "Continuous Inspection Schemes", Biometrika
// (origin of the CUSUM statistic); Lopez de Prado, M. (2018) "Advances in
// Financial Machine Learning" (CUSUM filters for financial structural-break/
// regime-change detection).
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: uses the deterministic CUSUM() indicator
// (explicit cumulative-sum statistic vs trailing mean/stdev) directly, no
// learned changepoint model.
//
// Regime:     TRENDING, RANGING, VOLATILE
// Timeframes: 15m
// Logic:      CUSUM(period=20, threshold=2.0 std devs) on Candles15m. When a
//             break is detected, trade in the break direction. SL is
//             tightened to exactly 1.0xATR (the regulatory SL-floor minimum)
//             rather than wider, since an active CUSUM regime-change signal
//             argues for a tighter stop (fast invalidation if the break
//             reading was a false positive).
// Edge:       CUSUM structural breaks flag a statistically significant shift
//             in the underlying return-generating process — Lopez de Prado's
//             standard financial-ML application of Page's classic
//             changepoint statistic.

type StructuralBreakDetection struct{}

func (s *StructuralBreakDetection) Name() string { return "Structural_Break_Detection" }

func (s *StructuralBreakDetection) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *StructuralBreakDetection) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 21 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	broke, dir := CUSUM(c15, 20, 2.0)
	if !broke || dir == 0 {
		return NoSignal(name)
	}

	// 3-bar confirmation: require the last 3 bars to be consistent with the
	// break direction (avoid single-bar spike false positives).
	n := len(c15)
	last3Up := c15[n-1].Close > c15[n-1].Open && c15[n-2].Close > c15[n-2].Open && c15[n-3].Close > c15[n-3].Open
	last3Down := c15[n-1].Close < c15[n-1].Open && c15[n-2].Close < c15[n-2].Open && c15[n-3].Close < c15[n-3].Open

	// SL tightened to exactly 1.0xATR per spec (still respects the
	// max(1.0xATR, 0.3%) regulatory floor since we take the max anyway).
	slDist := math.Max(1.0*atr, 0.003*price)

	if dir == 1 && last3Up {
		sl := price - slDist
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CUSUM structural break UP detected (20-bar, 2.0 std dev threshold), 3-bar confirmed — tight 1.0xATR SL for active regime change",
			),
		}
	}

	if dir == -1 && last3Down {
		sl := price + slDist
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CUSUM structural break DOWN detected (20-bar, 2.0 std dev threshold), 3-bar confirmed — tight 1.0xATR SL for active regime change",
			),
		}
	}

	return NoSignal(name)
}

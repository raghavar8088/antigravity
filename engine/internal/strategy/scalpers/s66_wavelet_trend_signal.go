package scalpers

import (
	"fmt"
	"math"
)

// S66 — Wavelet Trend Signal
//
// Research: Gallegati, M. & Semmler, W. (eds.), "Wavelet Applications in
// Economics and Finance" (2014) — multi-resolution wavelet decomposition
// separates a price series into a long-horizon trend estimate and
// high-frequency noise/detail components.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: uses the deterministic HaarWavelet() indicator
// (explicit Haar transform math) directly, no learned/fitted wavelet model.
//
// Regime:     TRENDING
// Timeframes: 15m
// Logic:      HaarWavelet(levels=3) on Candles15m gives (trendEstimate,
//             noiseLevel). Trend entry when trendEstimate is clearly above/
//             below a longer EMA reference AND noiseLevel is low relative to
//             ATR (good signal-to-noise ratio).
// Edge:       Trading only when the low-frequency wavelet trend component is
//             both directional and clean (low high-frequency noise relative
//             to volatility) filters out choppy/noisy regimes where a naive
//             trend-follow would whipsaw.

type WaveletTrendSignal struct{}

func (s *WaveletTrendSignal) Name() string { return "Wavelet_Trend_Signal" }

func (s *WaveletTrendSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *WaveletTrendSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 32 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	atr := ATR(c15, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	trendEstimate, noiseLevel := HaarWavelet(c15, 3)
	if trendEstimate == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	emaRef := EMA(c15, 50)
	if emaRef == 0 {
		emaRef = EMA(c15, 21)
	}
	if emaRef == 0 {
		return NoSignal(name)
	}

	// Good signal-to-noise: noise small relative to ATR
	goodSNR := noiseLevel < 0.5*atr

	// 3-bar confirmation of direction
	n := len(c15)
	last3Up := c15[n-1].Close > c15[n-1].Open && c15[n-2].Close > c15[n-2].Open && c15[n-3].Close > c15[n-3].Open
	last3Down := c15[n-1].Close < c15[n-1].Open && c15[n-2].Close < c15[n-2].Open && c15[n-3].Close < c15[n-3].Open

	trendUp := trendEstimate > emaRef
	trendDown := trendEstimate < emaRef

	if trendUp && goodSNR && last3Up {
		sl := price - math.Max(1.0*atr, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Wavelet trend LONG: trendEstimate=%.0f > EMA ref=%.0f, noise=%.2f <0.5xATR(%.2f), 3-bar confirmed",
				trendEstimate, emaRef, noiseLevel, atr,
			),
		}
	}

	if trendDown && goodSNR && last3Down {
		sl := price + math.Max(1.0*atr, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Wavelet trend SHORT: trendEstimate=%.0f < EMA ref=%.0f, noise=%.2f <0.5xATR(%.2f), 3-bar confirmed",
				trendEstimate, emaRef, noiseLevel, atr,
			),
		}
	}

	return NoSignal(name)
}

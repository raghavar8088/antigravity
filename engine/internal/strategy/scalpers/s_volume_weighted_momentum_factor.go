package scalpers

import "fmt"

// S24 — Volume_Weighted_Momentum_Factor
//
// Regime:     TRENDING only
// Timeframes: 15m
// Logic:      Volume-weighted momentum factor models are a standard
//             factor-investing construct (combining a price-momentum signal
//             with a volume/participation confirmation multiplier is the
//             same idea behind "momentum quality" factors used in equities
//             factor investing, adapted here to crypto perpetuals):
//                 factor = pct_change(price, N) * (current_volume / avg_volume_N)
//             A factor is only tradeable when it represents a genuine
//             outlier vs its own recent history, not just any positive
//             reading.
//
// SIMPLIFICATION (documented): true percentile tracking requires maintaining
// a long rolling factor-history buffer with state across evaluation cycles.
// To keep this strategy self-contained and stateless (consistent with how
// other scalpers in this registry are pure functions of MarketContext), we
// approximate "top decile" with a simpler, well-understood proxy: compute
// the factor at each of the last ~20 bars directly from the candle slice
// (no persisted state needed) and require the CURRENT factor reading to
// exceed 1.5x the mean of the trailing 20 factor readings. This is a
// standard z-score-lite outlier proxy and avoids the statistical fragility
// of true percentile ranking on a short trailing window (20 samples gives
// a noisy top-decile cutoff of just ~2 points).

type VolumeWeightedMomentumFactor struct{}

func (s *VolumeWeightedMomentumFactor) Name() string { return "Volume_Weighted_Momentum_Factor" }

func (s *VolumeWeightedMomentumFactor) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

const (
	vwmfMomentumPeriod    = 10  // N periods for pct-change momentum leg
	vwmfVolumeAvgPeriod   = 20  // N periods for avg-volume normalisation
	vwmfFactorHistoryBars = 20  // trailing bars used to gauge "outlier" factor
	vwmfOutlierMultiple   = 1.5 // current factor must exceed 1.5x trailing mean
)

// computeFactor returns the volume-weighted momentum factor at candle index i
// (using candles[i] as "current"). Requires i >= max(vwmfMomentumPeriod, vwmfVolumeAvgPeriod).
func computeVWMFactor(candles []Candle, i int) (float64, bool) {
	if i < vwmfMomentumPeriod || i < vwmfVolumeAvgPeriod {
		return 0, false
	}
	closeNow := candles[i].Close
	closePast := candles[i-vwmfMomentumPeriod].Close
	if closePast <= 0 {
		return 0, false
	}
	pctChange := (closeNow - closePast) / closePast

	volStart := i - vwmfVolumeAvgPeriod + 1
	if volStart < 0 {
		return 0, false
	}
	var volSum float64
	for j := volStart; j <= i; j++ {
		volSum += candles[j].Volume
	}
	avgVol := volSum / float64(vwmfVolumeAvgPeriod)
	if avgVol == 0 {
		return 0, false
	}
	volRatio := candles[i].Volume / avgVol

	return pctChange * volRatio, true
}

func (s *VolumeWeightedMomentumFactor) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	candles := ctx.Candles15m
	minNeeded := vwmfVolumeAvgPeriod + vwmfFactorHistoryBars + 1
	if len(candles) < minNeeded {
		return NoSignal(name)
	}

	lastIdx := len(candles) - 1
	currentFactor, ok := computeVWMFactor(candles, lastIdx)
	if !ok {
		return NoSignal(name)
	}

	// Build trailing factor history (last vwmfFactorHistoryBars readings,
	// excluding the current bar) to gauge whether the current reading is an
	// outlier relative to its own recent distribution.
	var histSum float64
	var histCount int
	for i := lastIdx - vwmfFactorHistoryBars; i < lastIdx; i++ {
		if i < 0 {
			continue
		}
		f, ok := computeVWMFactor(candles, i)
		if !ok {
			continue
		}
		histSum += f
		histCount++
	}
	if histCount < vwmfFactorHistoryBars/2 {
		return NoSignal(name)
	}
	histMean := histSum / float64(histCount)

	// Only trade strong, clearly positive-direction outlier readings — skip
	// weak/negative readings entirely per spec.
	isOutlierLong := currentFactor > 0 && histMean > 0 && currentFactor > vwmfOutlierMultiple*histMean
	isOutlierLongFromFlat := currentFactor > 0 && histMean <= 0 && currentFactor > 0.02 // absolute floor when trailing mean is non-positive
	isOutlierShort := currentFactor < 0 && histMean < 0 && currentFactor < vwmfOutlierMultiple*histMean
	isOutlierShortFromFlat := currentFactor < 0 && histMean >= 0 && currentFactor < -0.02

	price := ctx.Price
	atr15m := ATR(candles, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	if isOutlierLong || isOutlierLongFromFlat {
		sl := price - maxF(1.0*atr15m, 0.003*price)
		risk := price - sl
		tp := price + 2.0*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VW momentum factor=%.4f outlier vs trailing mean=%.4f (>%.1fx) -> top-decile-proxy LONG",
				currentFactor, histMean, vwmfOutlierMultiple,
			),
		}
	}

	if isOutlierShort || isOutlierShortFromFlat {
		sl := price + maxF(1.0*atr15m, 0.003*price)
		risk := sl - price
		tp := price - 2.0*risk
		if risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VW momentum factor=%.4f outlier vs trailing mean=%.4f (<%.1fx) -> top-decile-proxy SHORT",
				currentFactor, histMean, vwmfOutlierMultiple,
			),
		}
	}

	return NoSignal(name)
}

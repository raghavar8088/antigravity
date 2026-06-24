package scalpers

import (
	"fmt"
	"math"
)

// S53 — VPIN Toxicity Signal
//
// Regime:     TRENDING, VOLATILE
// Timeframes: 1m (CVD bucket delta) + 15m (ATR/SL reference)
// Research:   Easley, Lopez de Prado & O'Hara (2012) "Flow Toxicity and
//             Liquidity in a High Frequency World" — VPIN (Volume-
//             Synchronized Probability of Informed Trading) estimates order
//             flow toxicity from volume-bucketed buy/sell imbalance; high
//             VPIN precedes liquidity withdrawal and volatile repricing.
// Logic:      True VPIN requires fixed-size VOLUME buckets classified by
//             trade aggressor side — this engine does not yet track raw tick
//             data. SIMPLIFIED PROXY implemented here per spec:
//             toxicity = |CVD - CVDPrev| / (|CVD| + |CVDPrev| + epsilon),
//             a 0..1 estimate of how lopsided the most recent CVD bucket
//             delta is relative to its own scale. This is a CVD-bucket proxy
//             for true volume-bucketed VPIN, NOT the textbook calculation —
//             documented explicitly. toxicity > 0.7 combined with a clear
//             directional CVD bias (CVD > CVDPrev = buy-side toxic flow, or
//             CVD < CVDPrev = sell-side toxic flow) trades that direction,
//             on the reasoning that toxic/informed flow tends to continue
//             pushing price until the informed trader's position is filled.
// Edge:       Flags abnormally one-sided order flow bursts that historically
//             precede continued directional pressure rather than mean
//             reversion.

type VPINToxicitySignal struct{}

func (s *VPINToxicitySignal) Name() string { return "VPIN_Toxicity_Signal" }

func (s *VPINToxicitySignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeVolatile}
}

func (s *VPINToxicitySignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 15 || len(ctx.CVDHistory) < 3 {
		return NoSignal(name)
	}

	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	const epsilon = 1e-9
	cvd := ctx.CVD
	cvdPrev := ctx.CVDPrev
	denom := math.Abs(cvd) + math.Abs(cvdPrev) + epsilon
	toxicity := math.Abs(cvd-cvdPrev) / denom

	if toxicity <= 0.7 {
		return NoSignal(name)
	}

	// 3-bar confirmation: require the CVDHistory trend to agree directionally
	// for at least the last 3 readings (not just a single noisy delta) before
	// treating this as a real toxic-flow regime rather than a 1-bar blip.
	nh := len(ctx.CVDHistory)
	hist := ctx.CVDHistory
	risingConfirmed := hist[nh-2] > hist[nh-3] && hist[nh-1] > hist[nh-2]
	fallingConfirmed := hist[nh-2] < hist[nh-3] && hist[nh-1] < hist[nh-2]

	slDistMin := math.Max(1.0*atr15m, 0.003*price)

	if cvd > cvdPrev && risingConfirmed {
		sl := price - slDistMin
		tp := price + 2.0*slDistMin
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
				"VPIN toxicity LONG: proxy toxicity=%.2f (CVD %.0f->%.0f), 3-bar rising CVD confirmed",
				toxicity, cvdPrev, cvd,
			),
		}
	}

	if cvd < cvdPrev && fallingConfirmed {
		sl := price + slDistMin
		tp := price - 2.0*slDistMin
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
				"VPIN toxicity SHORT: proxy toxicity=%.2f (CVD %.0f->%.0f), 3-bar falling CVD confirmed",
				toxicity, cvdPrev, cvd,
			),
		}
	}

	return NoSignal(name)
}

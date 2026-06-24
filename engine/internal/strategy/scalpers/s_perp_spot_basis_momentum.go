package scalpers

import (
	"fmt"
	"math"
)

// S16 — Perp/Spot Basis Momentum
//
// Regime:     TRENDING only
// Timeframes: 1h (ATR/SL reference), uses ctx.Price (Coinbase spot) vs
//
//	ctx.PerpPrice (Binance BTCUSDT perpetual mark price)
//
// Basis calc (documented per task spec):
//
//	basis_pct = (perp_price - spot_price) / spot_price
//	expected_basis_from_funding ≈ FundingRate (raw decimal per 8h interval) —
//	  funding is the mechanism that's SUPPOSED to keep perp tethered to spot,
//	  so a funding rate of e.g. 0.0003 (0.03%/8h) "explains" a basis of roughly
//	  that same order of magnitude on a per-8h-cycle basis. When basis runs
//	  WIDER than what the current funding rate would explain, that gap is
//	  unexplained aggressive flow — leveraged buyers (or sellers) pushing the
//	  perp away from spot faster than funding-rate arbitrage can correct it.
//	divergence = basis_pct - FundingRate (raw decimal terms; both are
//	  "per cycle" decimals, e.g. 0.001 = 0.1%, comparable order of magnitude).
//
// This is a CONTINUATION strategy (trade WITH momentum), not a fade: widening
// unexplained perp premium + rising OI = aggressive leveraged buying that
// tends to persist short-term (basis-momentum / funding-arbitrage-lag
// dynamics are a well-documented perp microstructure feature — basis traders
// only step in once the spread is wide enough to clear costs, so it can run
// further than fair value before being arbitraged back).
type PerpSpotBasisMomentum struct{}

func (s *PerpSpotBasisMomentum) Name() string { return "Perp_Spot_Basis_Momentum" }

func (s *PerpSpotBasisMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *PerpSpotBasisMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.PerpPricePopulated || !ctx.PerpPriceHealthy {
		// Graceful degradation: no reliable perp price feed this cycle.
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}
	spot := ctx.Price
	perp := ctx.PerpPrice
	if spot <= 0 || perp <= 0 {
		return NoSignal(name)
	}

	oi := ctx.OpenInterest
	oiPrev := ctx.OpenInterestPrev
	if oi == 0 || oiPrev == 0 {
		return NoSignal(name)
	}
	// 3-bar confirmation via funding history, consistent with other strategies'
	// minimum 3-bar confirmation rule for divergence/pattern signals.
	if len(ctx.FundingHistory) < 3 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	basisPct := (perp - spot) / spot
	fundingNow := ctx.FundingRate // raw decimal, e.g. 0.0001 = 0.01% per 8h — do NOT scale
	unexplainedDivergence := basisPct - fundingNow

	oiRising := oi > oiPrev*1.02 // OI up >2%, same threshold convention as S9

	const basisDivergenceThreshold = 0.0015 // 0.15% unexplained premium — meaningfully > typical funding noise

	// ── Bullish continuation: perp premium running hot above funding + OI rising ──
	if unexplainedDivergence >= basisDivergenceThreshold && oiRising {
		sl := spot - math.Max(1.0*atr1h, 0.003*spot)
		slDist := spot - sl
		tp := spot + 2.0*slDist
		risk := spot - sl
		reward := tp - spot
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Perp basis momentum LONG: perp=%.0f spot=%.0f basis=%.3f%% funding=%.4f%% unexplained=%.3f%% OI +%.1f%%",
				perp, spot, basisPct*100, fundingNow*100, unexplainedDivergence*100, (oi/oiPrev-1)*100,
			),
		}
	}

	// ── Bearish continuation: perp running BELOW spot beyond funding + OI rising
	//    (aggressive leveraged selling pressure on the perp) ──
	oiRisingShort := oi > oiPrev*1.02
	if unexplainedDivergence <= -basisDivergenceThreshold && oiRisingShort {
		sl := spot + math.Max(1.0*atr1h, 0.003*spot)
		slDist := sl - spot
		tp := spot - 2.0*slDist
		risk := sl - spot
		reward := spot - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Perp basis momentum SHORT: perp=%.0f spot=%.0f basis=%.3f%% funding=%.4f%% unexplained=%.3f%% OI +%.1f%%",
				perp, spot, basisPct*100, fundingNow*100, unexplainedDivergence*100, (oi/oiPrev-1)*100,
			),
		}
	}

	return NoSignal(name)
}

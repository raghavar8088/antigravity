package scalpers

import (
	"fmt"
	"math"
)

// S15 — OI + Funding Crowding Composite
//
// Regime:     TRENDING, RANGING (not VOLATILE, not UNKNOWN)
// Timeframes: 1h
//
// Differentiation from S9 (OI_Divergence) and S8 (funding fade):
//
//	S9 looks at OI direction DIVERGING from price direction (e.g. OI rising
//	while price falls = trapped longs). S8 fades funding alone when it's
//	extreme. S15 is NOT another flavor of either — it requires BOTH OI
//	extremity AND funding extremity to be present SIMULTANEOUSLY, combined
//	into a single normalized "crowding score". This is a strictly higher
//	conviction / lower frequency signal than either input alone: one-sided
//	funding without OI confirmation can be noise (e.g. temporary funding
//	skew from a single large taker order), and OI growth without funding
//	confirmation can be neutral hedging flow rather than directional
//	conviction. Requiring both gates out a large share of false positives
//	that S8 or S9 alone would catch individually, at the cost of frequency —
//	intentional, since this is meant to size up on rarer, higher-quality
//	crowded-positioning setups, not duplicate S9/S8 trade frequency.
//
// Crowding score = normalize(OI change %) + normalize(funding extremity).
// Extreme one-directional composite score (both legs agree on direction and
// both exceed their threshold) = contrarian fade signal.
type OIFundingCrowdingComposite struct{}

func (s *OIFundingCrowdingComposite) Name() string { return "OI_Funding_Crowding_Composite" }

func (s *OIFundingCrowdingComposite) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

// normalizeOIChange maps OI % change to roughly [-1,1], saturating at +/-10%.
func normalizeOIChange(pctChange float64) float64 {
	n := pctChange / 0.10
	if n > 1 {
		n = 1
	}
	if n < -1 {
		n = -1
	}
	return n
}

// normalizeFunding maps raw funding decimal to roughly [-1,1], saturating at
// +/-0.001 (0.1% per 8h — an extreme reading for BTC perps).
func normalizeFunding(funding float64) float64 {
	n := funding / 0.001
	if n > 1 {
		n = 1
	}
	if n < -1 {
		n = -1
	}
	return n
}

func (s *OIFundingCrowdingComposite) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}

	oi := ctx.OpenInterest
	oiPrev := ctx.OpenInterestPrev
	if oi == 0 || oiPrev == 0 {
		// Graceful degradation: OI feed unavailable this cycle.
		return NoSignal(name)
	}
	// 3-bar confirmation: require funding history (at least 3 readings) to
	// confirm the funding extremity isn't a single-reading blip.
	if len(ctx.FundingHistory) < 3 {
		return NoSignal(name)
	}

	price := ctx.Price
	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 || price <= 0 {
		return NoSignal(name)
	}

	oiPctChange := (oi - oiPrev) / oiPrev
	oiScore := normalizeOIChange(oiPctChange)

	fh := ctx.FundingHistory
	fundingNow := fh[len(fh)-1]
	fundingScore := normalizeFunding(fundingNow)

	// 3-bar funding confirmation: all 3 most recent readings must agree in sign
	// and be non-trivial, so a single spike reading can't trigger this alone.
	last3 := fh[len(fh)-3:]
	fundingConsistentPositive := last3[0] > 0.00005 && last3[1] > 0.00005 && last3[2] > 0.00005
	fundingConsistentNegative := last3[0] < -0.00005 && last3[1] < -0.00005 && last3[2] < -0.00005

	// Composite score = sum of the two normalized legs.
	compositeScore := oiScore + fundingScore

	const oiExtremeThreshold = 0.5      // |oiScore| >= 0.5 → OI changed >= 5%
	const fundingExtremeThreshold = 0.3 // |fundingScore| >= 0.3 → funding >= 0.03% per 8h
	const compositeThreshold = 0.9      // both legs roughly extreme and agreeing

	// ── Crowded LONGS (OI growing + funding positive/extreme) → contrarian SHORT ──
	if oiScore >= oiExtremeThreshold && fundingScore >= fundingExtremeThreshold &&
		fundingConsistentPositive && compositeScore >= compositeThreshold {
		sl := price + math.Max(1.0*atr1h, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.75 + 0.05*math.Min(1.0, compositeScore-compositeThreshold)
		if conf > 0.85 {
			conf = 0.85
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Crowded LONG composite fade: OI +%.1f%% (score=%.2f), funding=%.4f%% 3-bar consistent (score=%.2f), composite=%.2f",
				oiPctChange*100, oiScore, fundingNow*100, fundingScore, compositeScore,
			),
		}
	}

	// ── Crowded SHORTS (OI growing + funding negative/extreme) → contrarian LONG ──
	// Note: composite score here is OI-positive + funding-negative, so it won't
	// necessarily clear +compositeThreshold the way the long-crowding branch does —
	// the directional gate is the two independent extremity checks below, each
	// required simultaneously (the "both conditions, not either" differentiation
	// from S9/S8 described above), rather than the signed sum threshold.
	if oiScore >= oiExtremeThreshold && fundingScore <= -fundingExtremeThreshold && fundingConsistentNegative {
		sl := price - math.Max(1.0*atr1h, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		magnitude := oiScore + math.Abs(fundingScore)
		conf := 0.75 + 0.05*math.Min(1.0, magnitude-(oiExtremeThreshold+fundingExtremeThreshold))
		if conf > 0.85 {
			conf = 0.85
		}
		if conf < 0.75 {
			conf = 0.75
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Crowded SHORT composite fade: OI +%.1f%% (score=%.2f), funding=%.4f%% 3-bar consistent (score=%.2f)",
				oiPctChange*100, oiScore, fundingNow*100, fundingScore,
			),
		}
	}

	return NoSignal(name)
}

package trading

import (
	"log"

	tconfig "antigravity-engine/internal/config"
	"antigravity-engine/internal/observability"
	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// TODO: BTC/ETH stat-arb pairs — needs ETH feed wired into MarketContext (OI + price).

// defaultMetaLabelMinFraction is the minimum FRACTION of *evaluable* confluence
// axes a signal must satisfy to survive MetaLabelFilter.Filter.
//
// FLOOD MODE: this is a fraction (0.0-1.0) of the axes that actually had data
// to score this cycle — NOT a fixed absolute out of 5.0. The previous design
// used a hard 3.0/5.0 cut, which was systemically unreachable: axes 3/4/5
// (funding / OI / order book) are SKIPPED when those feeds are thin or zero,
// so the maximum attainable score collapsed to 2.0 and every scalers signal
// was suppressed regardless of quality. Scaling the bar to the number of
// evaluable axes fixes that. Default 0.0 = pass everything (flood); raise
// META_LABEL_MIN_FRACTION in the ThresholdRegistry to re-introduce selectivity.
const defaultMetaLabelMinFraction = 0.0

// MetaLabelFilter scores each raw scaler signal against independent confluence
// axes — CVD alignment, funding alignment, OI confirmation, and order-book
// pressure — and suppresses signals that satisfy fewer than MinFraction of the
// axes that were actually evaluable this cycle. The old cross-strategy
// direction-vote axis was removed: it required ≥2 strategies firing the SAME
// direction in the SAME cycle, an unsatisfiable chicken-and-egg for sparse
// signal batches. Surviving signals keep their Confidence (scaled only gently
// by confluence) so downstream quality floors are not re-tripped.
type MetaLabelFilter struct {
	// MinFraction is the minimum fraction (0.0-1.0) of evaluable axes a signal
	// must satisfy to pass. Zero = pass everything (flood mode).
	MinFraction float64
}

// NewMetaLabelFilter returns a MetaLabelFilter using the flood-mode default
// minimum-fraction threshold (overridable via META_LABEL_MIN_FRACTION).
func NewMetaLabelFilter() *MetaLabelFilter {
	frac := tconfig.Default().GetWithDefault("META_LABEL_MIN_FRACTION", defaultMetaLabelMinFraction)
	return &MetaLabelFilter{MinFraction: frac}
}

// signalDirection maps an AggregatedSignal's legacy BUY/SELL action back to
// the scalers.Direction enum used by MarketContext-based confluence checks.
func signalDirection(sig AggregatedSignal) scalers.Direction {
	if sig.Signal.Action == strategy.ActionBuy {
		return scalers.DirectionLong
	}
	return scalers.DirectionShort
}

// Score evaluates sig against the confluence axes and returns the number of
// axes satisfied (score), the number of axes that were actually evaluable this
// cycle (maxScore — axes whose feeds had no data are not counted), and the
// reason codes for every evaluable axis that did NOT contribute a point.
//
// Counting maxScore as the number of EVALUABLE axes (rather than a fixed 5) is
// the core fix: a signal is judged only against the confluence checks for which
// data actually exists, so a missing funding/OI/order-book feed can never push
// the attainable score below the pass bar.
func (f *MetaLabelFilter) Score(sig AggregatedSignal, ctx scalers.MarketContext, regime scalers.Regime, summary map[string]scalers.WalkForwardSummary) (score, maxScore float64, reasons []string) {
	dir := signalDirection(sig)

	// Axis — CVD alignment with signal direction. Always evaluable (CVD is
	// derived from the candle/aggTrade feed which is always present).
	maxScore++
	switch dir {
	case scalers.DirectionLong:
		if ctx.CVD > ctx.CVDPrev {
			score++
		} else {
			reasons = append(reasons, "cvd_misaligned")
		}
	default: // SHORT
		if ctx.CVD < ctx.CVDPrev {
			score++
		} else {
			reasons = append(reasons, "cvd_misaligned")
		}
	}

	// Axis — funding rate alignment. Only evaluable when funding history is
	// thick enough to be meaningful.
	if len(ctx.FundingHistory) >= 2 {
		maxScore++
		switch dir {
		case scalers.DirectionLong:
			if ctx.FundingRate < 0.0001 {
				score++
			} else {
				reasons = append(reasons, "funding_misaligned")
			}
		default: // SHORT
			if ctx.FundingRate > -0.0001 {
				score++
			} else {
				reasons = append(reasons, "funding_misaligned")
			}
		}
	}

	// Axis — open interest confirmation. Rising OI confirms participation in
	// either direction. Only evaluable when both OI readings are present.
	if ctx.OpenInterest != 0 && ctx.OpenInterestPrev != 0 {
		maxScore++
		if ctx.OpenInterest > ctx.OpenInterestPrev {
			score++
		} else {
			reasons = append(reasons, "oi_not_confirming")
		}
	}

	// Axis — order book pressure. Only evaluable when the snapshot is populated.
	if ctx.OrderBook.IsPopulated() {
		maxScore++
		switch dir {
		case scalers.DirectionLong:
			if ctx.OrderBook.Imbalance > 0.15 {
				score++
			} else {
				reasons = append(reasons, "ob_misaligned")
			}
		default: // SHORT
			if ctx.OrderBook.Imbalance < -0.15 {
				score++
			} else {
				reasons = append(reasons, "ob_misaligned")
			}
		}
	}

	return score, maxScore, reasons
}

// Filter scores every signal in sigs and drops those whose composite score
// is below Threshold. Surviving signals have Confidence scaled by
// score/5.0. Emits a Prometheus counter and a log line per suppressed
// signal/reason.
func (f *MetaLabelFilter) Filter(sigs []AggregatedSignal, ctx scalers.MarketContext, regime scalers.Regime, summary map[string]scalers.WalkForwardSummary) []AggregatedSignal {
	if len(sigs) == 0 {
		return sigs
	}

	minFraction := f.MinFraction
	if minFraction < 0 {
		minFraction = 0
	}

	out := make([]AggregatedSignal, 0, len(sigs))
	for _, s := range sigs {
		score, maxScore, reasons := f.Score(s, ctx, regime, summary)
		// Pass bar scales with the number of evaluable axes, so missing feeds
		// can never make the bar unreachable. With minFraction 0 (flood) every
		// signal passes; raise META_LABEL_MIN_FRACTION to re-introduce a cut.
		required := minFraction * maxScore
		if score < required {
			if len(reasons) == 0 {
				reasons = []string{"low_confluence"}
			}
			for _, reason := range reasons {
				observability.ScalersMetaLabelSuppressed.WithLabelValues(s.StrategyName, reason).Inc()
			}
			log.Printf("[META_LABEL] suppressed strategy=%s score=%.0f/%.0f need=%.1f reasons=%v", s.StrategyName, score, maxScore, required, reasons)
			continue
		}
		// Gentle confidence scaling by confluence ratio — bounded to [0.90,1.0]
		// so a low-confluence-but-passing signal is not crushed below the
		// downstream execution floors. maxScore is always ≥1 (CVD axis).
		ratio := 1.0
		if maxScore > 0 {
			ratio = score / maxScore
		}
		s.Signal.Confidence *= 0.90 + 0.10*ratio
		out = append(out, s)
	}
	return out
}

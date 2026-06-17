package trading

import (
	"log"

	"antigravity-engine/internal/observability"
	"antigravity-engine/internal/strategy"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// TODO: BTC/ETH stat-arb pairs — needs ETH feed wired into MarketContext (OI + price).

// defaultMetaLabelThreshold is the minimum composite confluence score (out of
// 5.0) a signal must reach to survive MetaLabelFilter.Filter.
const defaultMetaLabelThreshold = 3.0

// MetaLabelFilter scores each raw scaler signal against five independent
// confluence axes — cross-strategy direction vote, CVD alignment, funding
// alignment, OI confirmation, and order-book pressure — and suppresses
// signals whose composite score falls below Threshold. Surviving signals
// have Confidence scaled by score/5.0 so downstream Kelly sizing naturally
// de-risks low-confluence entries instead of treating every approved signal
// as equally confident.
type MetaLabelFilter struct {
	// Threshold is the minimum composite score (0.0-5.0) required to pass.
	// Zero/unset falls back to defaultMetaLabelThreshold.
	Threshold float64

	// directionVotes is transient per-Filter-call state: direction -> count
	// of signals in the current batch carrying that direction. Filter
	// populates it before scoring so Score can read cross-strategy
	// confluence (axis 1) without widening its fixed signature. Filter is
	// invoked once per 15m evaluation cycle from a single goroutine
	// (evalAndExecuteScalers) — this struct is not safe for concurrent
	// Filter calls.
	directionVotes map[scalers.Direction]int
}

// NewMetaLabelFilter returns a MetaLabelFilter using the default 3.0/5.0
// suppression threshold.
func NewMetaLabelFilter() *MetaLabelFilter {
	return &MetaLabelFilter{Threshold: defaultMetaLabelThreshold}
}

// signalDirection maps an AggregatedSignal's legacy BUY/SELL action back to
// the scalers.Direction enum used by MarketContext-based confluence checks.
func signalDirection(sig AggregatedSignal) scalers.Direction {
	if sig.Signal.Action == strategy.ActionBuy {
		return scalers.DirectionLong
	}
	return scalers.DirectionShort
}

// Score evaluates sig against the five confluence axes and returns the
// composite score (0.0-5.0) plus the reason codes for every axis that did
// NOT contribute a point (used for suppression logging/metrics).
func (f *MetaLabelFilter) Score(sig AggregatedSignal, ctx scalers.MarketContext, regime scalers.Regime, summary map[string]scalers.WalkForwardSummary) (float64, []string) {
	dir := signalDirection(sig)
	score := 0.0
	var reasons []string

	// Axis 1 — cross-strategy direction vote: at least one OTHER strategy in
	// the same batch signalling the same direction.
	others := 0
	if f.directionVotes != nil {
		others = f.directionVotes[dir] - 1 // exclude this signal's own vote
	}
	if others >= 1 {
		score++
	} else {
		reasons = append(reasons, "axis1_no_confluence")
	}

	// Axis 2 — CVD alignment with signal direction.
	switch dir {
	case scalers.DirectionLong:
		if ctx.CVD > ctx.CVDPrev {
			score++
		} else {
			reasons = append(reasons, "axis2_cvd_misaligned")
		}
	default: // SHORT
		if ctx.CVD < ctx.CVDPrev {
			score++
		} else {
			reasons = append(reasons, "axis2_cvd_misaligned")
		}
	}

	// Axis 3 — funding rate alignment. Skipped (no penalty) when funding
	// history is too thin to be meaningful.
	if len(ctx.FundingHistory) >= 2 {
		switch dir {
		case scalers.DirectionLong:
			if ctx.FundingRate < 0.0001 {
				score++
			} else {
				reasons = append(reasons, "axis3_funding_misaligned")
			}
		default: // SHORT
			if ctx.FundingRate > -0.0001 {
				score++
			} else {
				reasons = append(reasons, "axis3_funding_misaligned")
			}
		}
	}

	// Axis 4 — open interest confirmation. Rising OI confirms trend
	// participation in either direction. Skipped (no penalty) when either OI
	// reading is unavailable (zero).
	if ctx.OpenInterest != 0 && ctx.OpenInterestPrev != 0 {
		if ctx.OpenInterest > ctx.OpenInterestPrev {
			score++
		} else {
			reasons = append(reasons, "axis4_oi_not_confirming")
		}
	}

	// Axis 5 — order book pressure. Skipped (no penalty) when the order book
	// snapshot isn't populated yet.
	if ctx.OrderBook.IsPopulated() {
		switch dir {
		case scalers.DirectionLong:
			if ctx.OrderBook.Imbalance > 0.15 {
				score++
			} else {
				reasons = append(reasons, "axis5_ob_misaligned")
			}
		default: // SHORT
			if ctx.OrderBook.Imbalance < -0.15 {
				score++
			} else {
				reasons = append(reasons, "axis5_ob_misaligned")
			}
		}
	}

	return score, reasons
}

// Filter scores every signal in sigs and drops those whose composite score
// is below Threshold. Surviving signals have Confidence scaled by
// score/5.0. Emits a Prometheus counter and a log line per suppressed
// signal/reason.
func (f *MetaLabelFilter) Filter(sigs []AggregatedSignal, ctx scalers.MarketContext, regime scalers.Regime, summary map[string]scalers.WalkForwardSummary) []AggregatedSignal {
	if len(sigs) == 0 {
		return sigs
	}

	threshold := f.Threshold
	if threshold <= 0 {
		threshold = defaultMetaLabelThreshold
	}

	// Build the cross-strategy direction vote map once per cycle so axis 1
	// can be computed inside Score without changing its signature.
	votes := make(map[scalers.Direction]int, 2)
	for _, s := range sigs {
		votes[signalDirection(s)]++
	}
	f.directionVotes = votes
	defer func() { f.directionVotes = nil }()

	out := make([]AggregatedSignal, 0, len(sigs))
	for _, s := range sigs {
		score, reasons := f.Score(s, ctx, regime, summary)
		if score < threshold {
			if len(reasons) == 0 {
				reasons = []string{"low_confluence"}
			}
			for _, reason := range reasons {
				observability.ScalersMetaLabelSuppressed.WithLabelValues(s.StrategyName, reason).Inc()
			}
			log.Printf("[META_LABEL] suppressed strategy=%s score=%.1f/5 reasons=%v", s.StrategyName, score, reasons)
			continue
		}
		s.Signal.Confidence *= score / 5.0
		out = append(out, s)
	}
	return out
}

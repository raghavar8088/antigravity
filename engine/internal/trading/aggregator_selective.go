package trading

import (
	"log"
	"sort"
	"time"

	"antigravity-engine/internal/strategy"
)

const (
	minSelectiveScore  = 2.0
	minDominanceRatio  = 1.15
	minDominanceLead   = 0.35
	maxApprovedSignals = 1
)

// FilterSignalsSelective chooses the dominant side for the current batch and
// only forwards a small, high-conviction subset of stronger strategies.
func (a *SignalAggregator) FilterSignalsSelective(rawSignals []AggregatedSignal) []AggregatedSignal {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	eligible := make([]AggregatedSignal, 0, len(rawSignals))
	sideScore := map[strategy.Action]float64{
		strategy.ActionBuy:  0,
		strategy.ActionSell: 0,
	}

	for _, sig := range rawSignals {
		a.totalSignals++
		if sig.Signal.Action == strategy.ActionHold {
			continue
		}

		if lastFired, ok := a.lastSignal[sig.StrategyName]; ok {
			elapsed := now.Sub(lastFired)
			if elapsed < time.Duration(a.cooldownSec)*time.Second {
				a.filteredSignals++
				continue
			}
		}

		score := strategyPriority(sig)
		sideScore[sig.Signal.Action] += score
		eligible = append(eligible, sig)
	}

	if len(eligible) == 0 {
		return nil
	}

	dominantAction := strategy.ActionBuy
	dominantScore := sideScore[strategy.ActionBuy]
	opposingScore := sideScore[strategy.ActionSell]
	if sideScore[strategy.ActionSell] > sideScore[strategy.ActionBuy] {
		dominantAction = strategy.ActionSell
		dominantScore = sideScore[strategy.ActionSell]
		opposingScore = sideScore[strategy.ActionBuy]
	}

	if opposingScore > 0 && (dominantScore < opposingScore*minDominanceRatio || dominantScore-opposingScore < minDominanceLead) {
		a.filteredSignals += int64(len(eligible))
		log.Printf("[AGGREGATOR] SKIPPED batch: weak consensus | buyScore=%.2f sellScore=%.2f", sideScore[strategy.ActionBuy], sideScore[strategy.ActionSell])
		return nil
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		return strategyPriority(eligible[i]) > strategyPriority(eligible[j])
	})

	var approved []AggregatedSignal
	usedCategories := make(map[string]struct{})
	for _, sig := range eligible {
		score := strategyPriority(sig)
		if sig.Signal.Action != dominantAction {
			a.filteredSignals++
			continue
		}
		if score < minSelectiveScore {
			a.filteredSignals++
			continue
		}
		if _, exists := usedCategories[sig.Category]; exists {
			a.filteredSignals++
			continue
		}
		if len(approved) >= maxApprovedSignals {
			a.filteredSignals++
			continue
		}

		sig.FiredAt = now
		a.lastSignal[sig.StrategyName] = now
		usedCategories[sig.Category] = struct{}{}
		approved = append(approved, sig)

		log.Printf("[AGGREGATOR] APPROVED: %s -> %s %.4f %s | score=%.2f",
			sig.StrategyName, sig.Signal.Action, sig.Signal.TargetSize, sig.Signal.Symbol, score)
	}

	return approved
}

func strategyPriority(sig AggregatedSignal) float64 {
	score := sig.Signal.Confidence
	if score == 0 {
		score = 1.0
	}

	switch sig.StrategyName {
	case "TripleFilter_Alpha_Scalp":
		score += 1.4
	case "OrderFlow_Pressure_Pro_Scalp":
		score += 1.35
	case "ATR_Breakout_Scalp":
		score += 1.3
	case "OpeningRange_Breakout_Scalp":
		score += 1.25
	case "VolSqueeze_Explosion_Scalp":
		score += 1.2
	case "VolumeBreakout_Impulse_Scalp":
		score += 1.2
	case "ATR_Volume_Impulse_Scalp":
		score += 1.15
	case "Donchian_Breakout_Scalp":
		score += 1.1
	case "Pullback_Continuation_Pro_Scalp":
		score += 1.1
	case "VolumeWeighted_Trend_Scalp":
		score += 1.1
	case "VWAP_RSI2_Reversion_Scalp":
		score += 1.0
	case "MACD_VWAP_Flip_Scalp":
		score += 0.95
	case "PriceChannel_Breakout_Scalp":
		score += 1.0
	case "Chart_Wedge_Breakout_Scalp":
		score += 1.05
	case "Chart_DoubleTap_Reversal_Scalp":
		score += 0.95
	case "ADX_Trend_Scalp":
		score += 1.0
	case "EMA_Cross_Scalp":
		score += 0.9
	case "Bollinger_RSI_Fade_Scalp":
		score += 0.8
	case "KAMA_Adaptive_Scalp":
		score += 0.8
	case "AdaptiveRSI_Dynamic_Scalp":
		score += 0.7
	case "ZScoreBand_MeanRev_Scalp":
		score += 0.7
	case "RSI_BB_Confluence_Scalp":
		score += 0.7
	case "Stochastic_Range_Scalp":
		score += 0.65
	case "LinReg_Statistical_Scalp":
		score += 0.6
	case "RangeCompress_Breakout_Scalp":
		score += 0.6
	case "Exhaustion_Reversal_Scalp":
		score += 0.4
	}

	switch sig.Category {
	case "Multi-Signal", "Breakout Elite", "Volatility", "Trend", "Time-of-Day":
		score += 0.2
	}

	return score
}

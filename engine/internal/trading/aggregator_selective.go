package trading

import (
	"log"
	"sort"
	"time"

	"antigravity-engine/internal/strategy"
)

const (
	minSelectiveScore  = 1.65 // Tightened: require stronger edge before any signal passes
	minDominanceRatio  = 1.15 // Tightened: dominant side must clearly beat the opposing side
	minDominanceLead   = 0.30 // Tightened: avoid trading nearly tied buy/sell batches
	maxApprovedSignals = 2    // Keep capital concentrated in the best setups only
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

	if sig.ExecutionWeight > 0 {
		score += (sig.ExecutionWeight - 1.0) * 0.70
	}
	if sig.TotalTrades >= 8 {
		switch {
		case sig.TotalPnL > 0 && sig.WinRate >= 0.58:
			score += 0.25
		case sig.TotalPnL > 0 && sig.WinRate >= 0.50:
			score += 0.10
		case sig.TotalPnL < 0 && sig.WinRate < 0.40:
			score -= 0.25
		case sig.TotalPnL < 0 && sig.WinRate < 0.46:
			score -= 0.10
		}
	}

	// Priorities calibrated against live performance data. Strong winners get a
	// clear boost, while repeat losers stay below the selective threshold unless
	// raw confidence improves materially.
	switch sig.StrategyName {
	// ── PROVEN WINNERS — boost ──────────────────────────────────────
	case "TripleFilter_Alpha_Scalp": // +$20 live
		score += 1.6
	case "VolumeWeighted_Trend_Scalp": // +$16 live
		score += 1.55
	case "EMA_Cross_Scalp": // +$4.51 live
		score += 1.4
	case "ZScoreBand_MeanRev_Scalp": // +$4.32 live
		score += 1.35
	case "OrderFlow_Pressure_Pro_Scalp": // +$2 live (low win rate but profitable)
		score += 1.3
	case "BollingerWalk_Trend_Scalp": // small positive
		score += 1.25
	case "Stochastic_Range_Scalp": // +$1.77 live
		score += 1.2
	case "RSI_BB_Confluence_Scalp": // +$3 live
		score += 1.2
	case "LinReg_Statistical_Scalp": // +$0.56 live
		score += 1.15
	case "Chart_DoubleTap_Reversal_Scalp": // +$1.63 live
		score += 1.15
	case "OpeningRange_Breakout_Scalp":
		score += 1.1
	case "VolSqueeze_Explosion_Scalp":
		score += 1.1
	case "Bollinger_RSI_Fade_Scalp":
		score += 1.05
	case "AdaptiveRSI_Dynamic_Scalp": // small negative — borderline
		score += 0.9
	// Pro2 strategies
	case "TrendMomentum_Score_Scalp":
		score += 1.2
	case "VWAP_RSI2_Reversion_Scalp": // -$1.42 live — reduced
		score += 0.7
	case "VWAP_Bounce_Pro_Scalp": // -$1.07 live — reduced
		score += 0.75
	case "TripleTrend_Confluence_Scalp": // -$1.43 live — reduced
		score += 0.65
	case "RSI_MACD_Divergence_Scalp": // -$2.06 live — reduced
		score += 0.55
	case "SessionOpen_Momentum_Scalp": // -$1.40 live — reduced
		score += 0.65
	// ── PROVEN LOSERS — heavily demoted ────────────────────────────
	case "ATR_Volume_Impulse_Scalp": // -$19.65 live — WORST loser
		score += 0.05
	case "ATR_Breakout_Scalp": // -$15.43 live
		score += 0.10
	case "KAMA_Adaptive_Scalp": // -$14.36 live
		score += 0.10
	case "PriceChannel_Breakout_Scalp": // -$11.29 live
		score += 0.12
	case "MACD_VWAP_Flip_Scalp": // -$10.90 live
		score += 0.15
	case "Donchian_Breakout_Scalp": // -$7.84 live
		score += 0.18
	case "ADX_Trend_Scalp": // -$7.86 live (fixed entry logic now)
		score += 0.25
	case "Chart_Wedge_Breakout_Scalp": // -$6.41 live
		score += 0.18
	case "VolumeBreakout_Impulse_Scalp": // -$5.34 live
		score += 0.20
	case "Pullback_Continuation_Pro_Scalp": // -$4.27 live
		score += 0.22
	case "MACD_ZeroCross_Confluence_Scalp": // -$3.71 live
		score += 0.20
	case "VolumeDelta_Spike_Scalp": // -$3.44 live
		score += 0.22
	case "RangeCompress_Breakout_Scalp":
		score += 0.25
	case "Exhaustion_Reversal_Scalp":
		score += 0.20
	}

	switch sig.Category {
	case "Multi-Signal", "Breakout Elite", "Volatility", "Trend", "Time-of-Day":
		score += 0.2
	case "Trend Elite", "Momentum Elite", "Mean Rev Elite", "Volatility Elite":
		score += 0.15
	}

	return score
}

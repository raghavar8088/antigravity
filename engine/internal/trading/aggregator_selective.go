package trading

import (
	"log"
	"sort"
	"time"

	"antigravity-engine/internal/strategy"
)

const (
	// Phase 22A signal-unlock pass: starvation constants replaced with
	// evidence-based values derived from observed confidence distributions.
	//
	// minSelectiveScore 1.10 → 0.80: strategies with confidence 0.74–0.95 and
	// no hardcoded name boost scored 0.74–1.02, making them unreachable.
	// 0.80 allows all cold-start curated/expansion strategies with confidence
	// ≥ 0.74 in bonus categories, and ≥ 0.80 in non-bonus categories.
	//
	// minDominanceLead 0.18 → 0.10: with 677 strategies the aggregate score
	// differential can be small even when directional consensus is genuine.
	// 0.10 still blocks true 50/50 splits but unlocks near-consensus batches.
	//
	// maxApprovedSignals 8 → 25: <1.2% signal throughput with 677 strategies.
	// 25 allows meaningful participation while keeping batch size manageable.
	//
	// maxApprovedPerCategory 2 → 5: allows genuine category breadth without
	// letting a single well-boosted category monopolise the approved batch.
	minSelectiveScore      = 0.80
	minDominanceRatio      = 1.10
	minDominanceLead       = 0.10
	maxApprovedSignals     = 25
	maxApprovedPerCategory = 5
)

// FilterSignalsSelective chooses the dominant side for the current batch and
// only forwards a small, high-conviction subset of stronger strategies.
func (a *SignalAggregator) FilterSignalsSelective(rawSignals []AggregatedSignal) []AggregatedSignal {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	a.RecordSignalFlowStage(SignalStageGenerated, len(rawSignals), len(rawSignals))

	eligible := make([]AggregatedSignal, 0, len(rawSignals))
	sideScore := map[strategy.Action]float64{
		strategy.ActionBuy:  0,
		strategy.ActionSell: 0,
	}

	for _, sig := range rawSignals {
		a.totalSignals++
		if sig.Signal.Action == strategy.ActionHold {
			a.RecordSignalFlowRejection(SignalStageGenerated, "hold_signal", sig.Category)
			continue
		}

		if lastFired, ok := a.lastSignal[sig.StrategyName]; ok {
			cd := a.defaultCooldown
			if override, ok2 := a.strategyCooldowns[sig.StrategyName]; ok2 {
				cd = override
			}
			if now.Sub(lastFired) < cd {
				a.filteredSignals++
				a.RecordSignalFlowRejection(SignalStageCooldownFilter, "strategy_cooldown", sig.Category)
				continue
			}
		}

		score := strategyPriority(sig)
		sideScore[sig.Signal.Action] += score
		eligible = append(eligible, sig)
	}
	a.RecordSignalFlowStage(SignalStageCooldownFilter, len(rawSignals), len(eligible))

	if len(eligible) == 0 {
		a.RecordSignalFlowStage(SignalStageAggregator, len(rawSignals), 0)
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
		a.RecordSignalFlowStage(SignalStageDominanceFilter, len(eligible), 0)
		for _, sig := range eligible {
			a.RecordSignalFlowRejection(SignalStageDominanceFilter, "weak_directional_consensus", sig.Category)
		}
		a.RecordSignalFlowStage(SignalStageAggregator, len(rawSignals), 0)
		log.Printf("[AGGREGATOR] SKIPPED batch: weak consensus | buyScore=%.2f sellScore=%.2f", sideScore[strategy.ActionBuy], sideScore[strategy.ActionSell])
		return nil
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		return strategyPriority(eligible[i]) > strategyPriority(eligible[j])
	})

	var approved []AggregatedSignal
	categoryCounts := make(map[string]int)
	dominanceOut := 0
	scoreIn := 0
	scoreOut := 0
	categoryIn := 0
	categoryOut := 0
	throughputIn := 0
	for _, sig := range eligible {
		score := strategyPriority(sig)
		if sig.Signal.Action != dominantAction {
			a.filteredSignals++
			a.RecordSignalFlowRejection(SignalStageDominanceFilter, "non_dominant_side", sig.Category)
			continue
		}
		dominanceOut++
		scoreIn++
		if score < minSelectiveScore {
			a.filteredSignals++
			a.RecordSignalFlowRejection(SignalStageScoreFilter, "score_below_selective_floor", sig.Category)
			continue
		}
		scoreOut++
		categoryIn++
		if categoryCounts[sig.Category] >= maxApprovedPerCategory {
			a.filteredSignals++
			a.RecordSignalFlowRejection(SignalStageCategoryDeduplication, "category_batch_cap", sig.Category)
			continue
		}
		categoryOut++
		throughputIn++
		if len(approved) >= maxApprovedSignals {
			a.filteredSignals++
			a.RecordSignalFlowRejection(SignalStageThroughputCap, "batch_approval_cap", sig.Category)
			continue
		}

		sig.FiredAt = now
		a.lastSignal[sig.StrategyName] = now
		categoryCounts[sig.Category]++
		approved = append(approved, sig)
		a.flowMetrics.RecordStrategyApproval(sig.StrategyName, sig.Category)

		log.Printf("[AGGREGATOR] APPROVED: %s -> %s %.4f %s | score=%.2f",
			sig.StrategyName, sig.Signal.Action, sig.Signal.TargetSize, sig.Signal.Symbol, score)
	}

	a.RecordSignalFlowStage(SignalStageDominanceFilter, len(eligible), dominanceOut)
	a.RecordSignalFlowStage(SignalStageScoreFilter, scoreIn, scoreOut)
	a.RecordSignalFlowStage(SignalStageCategoryDeduplication, categoryIn, categoryOut)
	a.RecordSignalFlowStage(SignalStageThroughputCap, throughputIn, len(approved))
	a.RecordSignalFlowStage(SignalStageAggregator, len(rawSignals), len(approved))

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
	// ── PROVEN WINNERS — boosted aggressively ──────────────────────
	case "TripleFilter_Alpha_Scalp": // +$20 live — #1 winner
		score += 2.00
	case "VolumeWeighted_Trend_Scalp": // +$16 live — #2 winner
		score += 1.90
	case "EMA_Cross_Scalp": // +$4.51 live
		score += 1.70
	case "ZScoreBand_MeanRev_Scalp": // +$4.32 live
		score += 1.65
	case "RSI_BB_Confluence_Scalp": // +$3 live
		score += 1.55
	case "OrderFlow_Pressure_Pro_Scalp": // +$2 live
		score += 1.50
	case "Chart_DoubleTap_Reversal_Scalp": // +$1.63 live
		score += 1.45
	case "Stochastic_Range_Scalp": // +$1.77 live
		score += 1.45
	case "BollingerWalk_Trend_Scalp": // positive
		score += 1.40
	case "LinReg_Statistical_Scalp": // +$0.56 live
		score += 1.35
	case "OpeningRange_Breakout_Scalp":
		score += 1.35
	case "VolSqueeze_Explosion_Scalp":
		score += 1.35
	case "TrendMomentum_Score_Scalp":
		score += 1.40
	case "Bollinger_RSI_Fade_Scalp":
		score += 1.30
	// ── INSTITUTIONAL ALPHA — uncorrelated edge sources ─────────────
	case "CVDDivergence_Alpha",
		"DeltaAbsorption_Alpha",
		"LiquiditySweepReversal_Alpha",
		"FVGRetest_Alpha",
		"OrderBlockRetest_Alpha",
		"MSSContinuation_Alpha",
		"POCBounce_Alpha",
		"SessionExpansion_Alpha",
		"FundingMeanReversion_Alpha",
		"LiquidationCascade_Alpha":
		score += 1.45
	// ── PHASE 11 MICROSTRUCTURE ALPHA — multi-dimensional feature engine ──
	case "Phase11LiquiditySweepReversal_Alpha",
		"Phase11FundingMeanReversion_Alpha",
		"Phase11CVDDivergence_Alpha",
		"Phase11LiquidationCascadeReversal_Alpha",
		"Phase11FairValueGap_Alpha",
		"Phase11OrderBlock_Alpha",
		"Phase11MSSCHOCH_Alpha":
		score += 1.45
	// ── BORDERLINE — small negatives, below threshold ──────────────
	case "AdaptiveRSI_Dynamic_Scalp":
		score += 0.80
	case "VWAP_Bounce_Pro_Scalp": // -$1.07 live
		score += 0.60
	case "VWAP_RSI2_Reversion_Scalp": // -$1.42 live
		score += 0.55
	case "SessionOpen_Momentum_Scalp": // -$1.40 live
		score += 0.55
	case "TripleTrend_Confluence_Scalp": // -$1.43 live
		score += 0.50
	case "RSI_MACD_Divergence_Scalp": // -$2.06 live
		score += 0.40
	// Historical worst — keep below selective floor in tests and prod.
	case "ATR_Volume_Impulse_Scalp":
		score -= 0.70
	// ── PROVEN LOSERS — below threshold floor, will not pass ───────
	case "RangeCompress_Breakout_Scalp", "Exhaustion_Reversal_Scalp":
		score += 0.20
	}

	// Category bonuses: tier-1 categories are well-represented in live winners;
	// tier-2 covers expansion-pack families (Momentum, Breakout, Intraday, etc.)
	// that previously had no bonus and scored below the 0.80 floor with typical
	// confidence values of 0.74–0.85.
	switch sig.Category {
	case "Multi-Signal", "Breakout Elite", "Volatility", "Trend", "Time-of-Day",
		"Statistical", "Microstructure", "Mean Reversion":
		score += 0.20
	case "Trend Elite", "Momentum Elite", "Mean Rev Elite", "Volatility Elite":
		score += 0.15
	case "Momentum", "Breakout", "Order Flow", "Alpha", "Intraday",
		"Liquidity", "Funding", "Session", "Price Action", "Structure",
		"Smart Money", "Adaptive", "Market Profile":
		score += 0.10
	}

	return score
}

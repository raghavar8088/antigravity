package scalpers

import (
	"log"
	"sync"
)

// performanceRegistry holds live performance stats for each scalper strategy.
// Updated by UpdatePerformance() after each trade closes.
var (
	perfMu              sync.RWMutex
	performanceRegistry = map[string]Performance{}
)

// UpdatePerformance upserts live performance data for a strategy.
// Called by the trading loop after every TP/SL/TIME exit.
func UpdatePerformance(p Performance) {
	perfMu.Lock()
	performanceRegistry[p.StrategyName] = p
	perfMu.Unlock()
}

// GetPerformance returns the last recorded performance for a strategy.
func GetPerformance(strategyName string) (Performance, bool) {
	perfMu.RLock()
	defer perfMu.RUnlock()
	p, ok := performanceRegistry[strategyName]
	return p, ok
}

// AllPerformance returns a snapshot of all performance records.
func AllPerformance() []Performance {
	perfMu.RLock()
	defer perfMu.RUnlock()
	out := make([]Performance, 0, len(performanceRegistry))
	for _, p := range performanceRegistry {
		out = append(out, p)
	}
	return out
}

// tradeEngineEnabled is the LIVE whitelist for the main Trade Engine.
// Rebuilt 2026-07-05 from the honest backtest pipeline (taker-fee cost model +
// trade-frequency Sharpe annualisation): a strategy trades live only if it
// fully met promotion criteria in the train window (2021-06-26 → 2024-06-30)
// AND confirmed out-of-sample in the validation window (2024-07-01 → 2026-06-26).
// Of 303 backtested strategies, exactly these survived. See BACKTEST.md §5b.
//
// The previous 21-name list (picked from the short-window live Leadership
// Board on 2026-07-03) was removed: its 6 backtestable members were among the
// worst strategies in the 5-year backtest (combined −32%), and the other 15
// had no offline evidence at all — they now live in tradeEngineShadow below.
var tradeEngineEnabled = map[string]bool{
	"BB_Squeeze_EFI_ADX_Short": true, // train: sh 1.18 pf 1.46 n=276 | val: sh 1.00 pf 1.35 n=210
	"WMA_Bear_Cross_Short":     true, // train: sh 1.24 pf 2.39 n=59  | val: sh 1.52 pf 2.88 n=30
}

// tradeEngineShadow is the SHADOW tier: these strategies evaluate every cycle
// and record to the shadow ledger (engine/internal/shadow) but never reach the
// live OMS — withForcedShadow() pins IsShadow=true. Promotion path: accumulate
// a shadow track record that clears ShadowPromoter.CanPromote, then move the
// name to tradeEngineEnabled (or set STRATEGY_LIVE_OVERRIDE for a no-deploy
// promotion). Two groups:
//
//	(a) Feed-dependent strategies (orderbook/options/funding/liquidity) that
//	    cannot be backtested offline — the shadow ledger is their only way to
//	    build evidence. These were on the old live whitelist with zero
//	    validation; they keep evaluating here instead.
//	(b) Train-window qualifiers that FAILED out-of-sample confirmation —
//	    live shadow data will show whether the failure was regime-specific
//	    or terminal.
var tradeEngineShadow = map[string]bool{
	// (a) feed-dependent, no offline backtest possible
	"Sweep_And_Reload_Pattern":           true,
	"Volume_Profile_POC_Magnet":          true,
	"Coppock_Curve_Momentum":             true,
	"Historical_Vol_Percentile_Breakout": true,
	"Ornstein_Uhlenbeck_Reversion":       true,
	"Structural_Break_Detection":         true,
	"Hidden_Liquidity_Detection":         true,
	"Recurrence_Quantification_Signal":   true,
	"Absorption_Pattern_Signal":          true,
	"Bid_Ask_Spread_Regime":              true,
	"Orderbook_Imbalance_Persistence":    true,
	"Hull_MA_Scalp":                      true,
	"Options_Skew_Direction_Signal":      true,
	"Morning_Evening_Star":               true,
	"Funding_OI_Confirm":                 true,
	// (b) qualified in train window, failed OOS confirmation
	"CMF_OBV_Confluence_Bear_Short":  true,
	"WMA5_Cross_WMA21_EFI_ADX_Short": true,
	"WR_Mid_EFI_ADX_Short":           true,
	"WR_Mid_Cross_Bear_Short":        true,
	"Fisher_Zero_Cross_Short":        true,
	"WR_RSI_Bear_Confirm_Short":      true,
	"Big_Bear_Candle_Short":          true,
	"WMA_EFI_Bear_Short":             true,
	"BB_Width_Expand_Bear_Short":     true,
	"HTF_Aligned_Pullback_Short":     true,
	"WMA_CMF_Bear_Short":             true,
	"WMA5_Cross_EMA13_EFI_ADX_Short": true,
	"Big_Bear_Candle_ADX_Short":      true,
	"Donchian_Breakdown_Short":       true,
	"MACD_Double_Bearish_Short":      true,
	"MACD_Signal_Cross_Short":        true,
	"CMF_Zero_Bear_Short":            true,
}

// filterTradeEngineEnabled keeps live-whitelisted strategies as-is, keeps
// shadow-tier strategies wrapped in withForcedShadow (they evaluate and record
// to the shadow ledger but never trade live), drops everything else, and logs
// any configured names that matched nothing (catches renames/typos at startup).
func filterTradeEngineEnabled(entries []RegistryEntry) []RegistryEntry {
	seen := make(map[string]bool, len(tradeEngineEnabled)+len(tradeEngineShadow))
	out := make([]RegistryEntry, 0, len(tradeEngineEnabled)+len(tradeEngineShadow))
	for _, e := range entries {
		switch {
		case tradeEngineEnabled[e.Name]:
			out = append(out, e)
			seen[e.Name] = true
		case tradeEngineShadow[e.Name]:
			e.Strategy = withForcedShadow(e.Strategy)
			out = append(out, e)
			seen[e.Name] = true
		}
	}
	for name := range tradeEngineEnabled {
		if !seen[name] {
			log.Printf("[REGISTRY] WARNING: trade-engine whitelist name %q matched no registered strategy", name)
		}
	}
	for name := range tradeEngineShadow {
		if !seen[name] {
			log.Printf("[REGISTRY] WARNING: trade-engine shadow-tier name %q matched no registered strategy", name)
		}
	}
	return out
}

// BuildCuratedScalpers returns the active set of scalpers, filtered by
// FilterWinnersOnly. New strategies (no performance record yet) are always
// included so they can accumulate their first 30 trades.
func BuildCuratedScalpers() []RegistryEntry {
	all := BuildAllScalpers()
	expansion := buildExpansionPack()
	all = append(all, expansion...)
	volatility := buildVolatilityFamily()
	all = append(all, volatility...)
	microstructure := buildMicrostructureFamily()
	all = append(all, microstructure...)
	macro := buildMacroFamily()
	all = append(all, macro...)
	statistical := buildStatisticalFamily()
	all = append(all, statistical...)
	event := buildEventFamily()
	all = append(all, event...)

	// S30-S79: 50 shadow-only strategy families (research-backed, phase-99
	// gated — see rollout_phase.go). These ALWAYS evaluate every cycle so
	// they accumulate a real walk-forward track record, but never trade live
	// until an operator explicitly promotes them via STRATEGY_LIVE_OVERRIDE
	// or ShadowPromoter.Promote().
	family1 := buildFamily1Momentum()
	all = append(all, family1...)
	family2 := buildFamily2MeanReversion()
	all = append(all, family2...)
	family3 := buildFamily3OrderFlow()
	all = append(all, family3...)
	family4 := buildFamily4MLProxy()
	all = append(all, family4...)
	family5 := buildFamily5DerivativesMacro()
	all = append(all, family5...)

	// S80-S99: IMMORTAL EDITION — 20 research-backed strategies, live immediately.
	// No withRolloutPhase() wrapper — these evaluate and trade on the live paper
	// OMS from the first cycle. withShadowOverride() is applied uniformly below.
	immortal := buildImmortalEditionPack()
	all = append(all, immortal...)

	// S101-S150: Ported open-source strategies, shadow-only until promoted.
	ported := BuildPortedStrategies()
	all = append(all, ported...)

	// Strategies registered without withRolloutPhase() (the original S1-S10
	// roster) never trade live unless SHADOW_STRATEGIES/STRATEGY_LIVE_OVERRIDE
	// applies. Wrap them so those two operator knobs work uniformly across
	// every strategy in the curated set, not just the phase-gated ones.
	for i, e := range all {
		if _, alreadyGated := e.Strategy.(*phaseGatedStrategy); alreadyGated {
			continue
		}
		all[i].Strategy = withShadowOverride(e.Strategy)
	}

	all = filterTradeEngineEnabled(all)

	filtered := FilterWinnersOnly(all)
	log.Printf("[REGISTRY] BuildCuratedScalpers returned %d strategies (pre-filter %d)", len(filtered), len(all))
	return filtered
}

// FilterWinnersOnly removes strategies that have >= 30 trades AND are marked
// inactive (Active=false) in their performance record.
// Strategies with < 30 trades are always kept (probationary period).
func FilterWinnersOnly(entries []RegistryEntry) []RegistryEntry {
	perfMu.RLock()
	defer perfMu.RUnlock()

	out := make([]RegistryEntry, 0, len(entries))
	for _, e := range entries {
		p, exists := performanceRegistry[e.Name]
		if !exists {
			// No history yet — include (probationary)
			out = append(out, e)
			continue
		}
		if p.TotalTrades < 30 {
			// Insufficient data — keep running
			out = append(out, e)
			continue
		}
		if p.Active {
			out = append(out, e)
		}
	}
	return out
}

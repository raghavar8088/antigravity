package backtest

import (
	"fmt"
	"log"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// PromotionCriteria defines the thresholds a strategy must meet to be promoted
// from shadow mode to live trading via UpdatePerformance + Active=true.
var PromotionCriteria = struct {
	MinSharpe       float64
	MinWinRate      float64
	MaxDrawdownPct  float64
	MinProfitFactor float64
	MinTrades       int
	MinStressPasses int
}{
	MinSharpe:       1.0,
	MinWinRate:      0.45,
	MaxDrawdownPct:  20.0,
	MinProfitFactor: 1.3,
	MinTrades:       50,
	MinStressPasses: 3,
}

// DemotionCriteria defines auto-demotion thresholds.
var DemotionCriteria = struct {
	MinTrades    int
	MaxWinRate   float64
}{
	MinTrades:  30,
	MaxWinRate: 0.35,
}

// EvaluatePromotion checks whether a StrategyResult meets promotion criteria
// and fills sr.MeetsPromotion and sr.DemoteReason.
func EvaluatePromotion(sr *StrategyResult, v3Result interface{ StressPassCount() int }) {
	if sr.TotalTrades < PromotionCriteria.MinTrades {
		sr.DemoteReason = fmt.Sprintf("insufficient trades (%d < %d)", sr.TotalTrades, PromotionCriteria.MinTrades)
		return
	}
	if sr.WinRate < PromotionCriteria.MinWinRate {
		sr.DemoteReason = fmt.Sprintf("win rate %.1f%% < %.0f%%", sr.WinRate*100, PromotionCriteria.MinWinRate*100)
		return
	}
	if sr.Sharpe < PromotionCriteria.MinSharpe {
		sr.DemoteReason = fmt.Sprintf("sharpe %.2f < %.1f", sr.Sharpe, PromotionCriteria.MinSharpe)
		return
	}
	if sr.MaxDrawdownPct > PromotionCriteria.MaxDrawdownPct {
		sr.DemoteReason = fmt.Sprintf("max drawdown %.1f%% > %.0f%%", sr.MaxDrawdownPct, PromotionCriteria.MaxDrawdownPct)
		return
	}
	if sr.ProfitFactor < PromotionCriteria.MinProfitFactor {
		sr.DemoteReason = fmt.Sprintf("profit factor %.2f < %.1f", sr.ProfitFactor, PromotionCriteria.MinProfitFactor)
		return
	}
	if v3Result != nil && v3Result.StressPassCount() < PromotionCriteria.MinStressPasses {
		sr.DemoteReason = fmt.Sprintf("stress passes %d < %d", v3Result.StressPassCount(), PromotionCriteria.MinStressPasses)
		return
	}
	sr.MeetsPromotion = true
}

// PromoteFromResult promotes a strategy by writing an active Performance record
// into the global performance registry (scalers.UpdatePerformance).
// This makes the strategy eligible for live trading in the next eval cycle.
// The caller must have verified sr.MeetsPromotion == true.
func PromoteFromResult(sr StrategyResult) {
	p := scalers.Performance{
		StrategyName: sr.StrategyName,
		TotalTrades:  sr.TotalTrades,
		WinRate:      sr.WinRate,
		SharpeRatio:  sr.Sharpe,
		MaxDrawdown:  sr.MaxDrawdownPct / 100,
		Active:       true,
	}
	scalers.UpdatePerformance(p)
	log.Printf("[PROMOTION] %s promoted to live — sharpe=%.2f win=%.0f%% dd=%.1f%% trades=%d",
		sr.StrategyName, sr.Sharpe, sr.WinRate*100, sr.MaxDrawdownPct, sr.TotalTrades)
}

// DemoteFromResult writes an inactive Performance record, removing the strategy
// from live trading after the next FilterWinnersOnly pass.
func DemoteFromResult(strategyName, reason string) {
	p, ok := scalers.GetPerformance(strategyName)
	if !ok {
		return
	}
	p.Active = false
	scalers.UpdatePerformance(p)
	log.Printf("[DEMOTION] %s demoted: %s", strategyName, reason)
}

// ApplyAutoDemotion iterates all live performance records and demotes any
// strategy that has ≥30 trades and WinRate < 0.35.
// This satisfies the Task 9 auto-demotion requirement.
func ApplyAutoDemotion() {
	for _, p := range scalers.AllPerformance() {
		if p.TotalTrades >= DemotionCriteria.MinTrades && p.WinRate < DemotionCriteria.MaxWinRate && p.Active {
			p.Active = false
			scalers.UpdatePerformance(p)
			log.Printf("[AUTO-DEMOTION] %s demoted: trades=%d winRate=%.1f%%",
				p.StrategyName, p.TotalTrades, p.WinRate*100)
		}
	}
}

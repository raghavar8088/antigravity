package risk

import (
	"fmt"
	"log"

	riskv2 "antigravity-engine/internal/risk/v2"
	"antigravity-engine/internal/strategy"
)

const eliteOnlyDrawdownPct = 8.0

// EvaluateDrawdownExecution blocks non-elite strategies when portfolio drawdown
// reaches the ELITE_ONLY regime (>= 8%). Elite strategies may continue trading.
func EvaluateDrawdownExecution(meta strategy.StrategyMetadata, dd riskv2.DrawdownDecision) error {
	if dd.DrawdownPct < eliteOnlyDrawdownPct && !dd.OnlyEliteStrategies {
		return nil
	}
	if meta.Tier == strategy.StrategyTierElite {
		return nil
	}
	msg := fmt.Sprintf(
		"drawdown regime ELITE_ONLY active (dd=%.2f%%) — strategy tier %q (category=%q) not permitted",
		dd.DrawdownPct, meta.Tier, meta.Category,
	)
	log.Printf("[RISK REJECTION] %s: %s", meta.Name, msg)
	return fmt.Errorf("RISK_REJECT: %s", msg)
}

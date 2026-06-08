package risk

import (
	"testing"

	riskv2 "antigravity-engine/internal/risk/v2"
	"antigravity-engine/internal/strategy"
)

func TestEvaluateDrawdownExecutionBlocksNonEliteAt8Pct(t *testing.T) {
	meta := strategy.StrategyMetadata{
		Name:     "Trend_Scalp",
		Category: "Trend",
		Tier:     strategy.StrategyTierStandard,
	}
	dd := riskv2.DrawdownDecision{DrawdownPct: 8.5, OnlyEliteStrategies: true}
	if err := EvaluateDrawdownExecution(meta, dd); err == nil {
		t.Fatal("expected non-elite strategy to be blocked at 8%+ drawdown")
	}
}

func TestEvaluateDrawdownExecutionAllowsEliteAt8Pct(t *testing.T) {
	meta := strategy.StrategyMetadata{
		Name:     "ZScoreBand_MeanRev_Scalp",
		Category: "Mean Rev Elite",
		Tier:     strategy.StrategyTierElite,
	}
	dd := riskv2.DrawdownDecision{DrawdownPct: 9.0, OnlyEliteStrategies: true}
	if err := EvaluateDrawdownExecution(meta, dd); err != nil {
		t.Fatalf("expected elite strategy to pass: %v", err)
	}
}

func TestEvaluateDrawdownExecutionAllowsStandardBelow8Pct(t *testing.T) {
	meta := strategy.StrategyMetadata{
		Name:     "Trend_Scalp",
		Category: "Trend",
		Tier:     strategy.StrategyTierStandard,
	}
	dd := riskv2.DrawdownDecision{DrawdownPct: 5.0}
	if err := EvaluateDrawdownExecution(meta, dd); err != nil {
		t.Fatalf("expected standard strategy below 8%% to pass: %v", err)
	}
}

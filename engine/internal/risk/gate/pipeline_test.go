package gate

import (
	"context"
	"testing"

	riskv2 "antigravity-engine/internal/risk/v2"
)

type staticKillSwitch struct {
	active bool
	reason string
}

func (s staticKillSwitch) IsActive() bool { return s.active }
func (s staticKillSwitch) Reason() string { return s.reason }

func TestPreTradeRiskPipelineBlocksWhenKillSwitchActive(t *testing.T) {
	pipeline := NewPreTradeRiskPipeline(riskv2.NewEngine(100_000), staticKillSwitch{active: true, reason: "reconciliation mismatch"})

	decision := pipeline.Check(context.Background(), validInput())
	if decision.Status != DecisionBlocked {
		t.Fatalf("expected blocked, got %s", decision.Status)
	}
	if decision.Error() == nil {
		t.Fatal("expected blocked decision to return an error")
	}
}

func TestPreTradeRiskPipelineBlocksMissingStop(t *testing.T) {
	input := validInput()
	input.Request.StopLossPrice = 0
	pipeline := NewPreTradeRiskPipeline(riskv2.NewEngine(100_000), nil)

	decision := pipeline.Check(context.Background(), input)
	if decision.Status != DecisionBlocked || decision.Reason != "missing stop loss" {
		t.Fatalf("expected missing stop block, got %s %q", decision.Status, decision.Reason)
	}
}

func TestPreTradeRiskPipelineApprovesValidTrade(t *testing.T) {
	pipeline := NewPreTradeRiskPipeline(riskv2.NewEngine(100_000), nil)

	decision := pipeline.Check(context.Background(), validInput())
	if decision.Status != DecisionApproved {
		t.Fatalf("expected approved, got %s %q", decision.Status, decision.Reason)
	}
}

func validInput() Input {
	return Input{
		Request: riskv2.TradeRequest{
			Symbol:            "BTCUSDT",
			Strategy:          "Phase14Test",
			Family:            riskv2.FamilyOrderFlow,
			Side:              riskv2.SideLong,
			EntryPrice:        100_000,
			StopLossPrice:     99_000,
			RequestedSizeBTC:  0.01,
			RequestedLeverage: 1,
			Confidence:        0.8,
			Exchange:          "paper",
		},
		Market: riskv2.MarketState{Regime: riskv2.RegimeRange, LiquidityScore: 0.8},
		Metrics: riskv2.StrategyMetrics{
			Strategy:         "Phase14Test",
			Family:           riskv2.FamilyOrderFlow,
			WinRate:          0.55,
			ProfitFactor:     1.5,
			Sharpe:           1.2,
			OOSProfitFactor:  1.2,
			OOSExpectancyUSD: 10,
			HealthScore:      80,
			TotalTrades:      50,
		},
	}
}

package gate

import (
	"context"
	"fmt"
	"time"

	riskv2 "antigravity-engine/internal/risk/v2"
)

type DecisionStatus string

const (
	DecisionApproved DecisionStatus = "APPROVED"
	DecisionBlocked  DecisionStatus = "BLOCKED"
)

type KillSwitch interface {
	IsActive() bool
	Reason() string
}

type PreTradeRiskPipeline struct {
	engine     *riskv2.Engine
	killSwitch KillSwitch
}

type Input struct {
	Request riskv2.TradeRequest
	Market  riskv2.MarketState
	Metrics riskv2.StrategyMetrics
}

type Decision struct {
	Status       DecisionStatus
	Reason       string
	RiskDecision riskv2.RiskDecision
	CheckedAt    time.Time
}

func NewPreTradeRiskPipeline(engine *riskv2.Engine, killSwitch KillSwitch) *PreTradeRiskPipeline {
	return &PreTradeRiskPipeline{engine: engine, killSwitch: killSwitch}
}

func (p *PreTradeRiskPipeline) Check(ctx context.Context, input Input) Decision {
	checkedAt := time.Now().UTC()
	if err := ctx.Err(); err != nil {
		return Decision{Status: DecisionBlocked, Reason: err.Error(), CheckedAt: checkedAt}
	}
	if p.killSwitch != nil && p.killSwitch.IsActive() {
		return Decision{Status: DecisionBlocked, Reason: "kill switch active: " + p.killSwitch.Reason(), CheckedAt: checkedAt}
	}
	if p.engine == nil {
		return Decision{Status: DecisionBlocked, Reason: "risk engine unavailable", CheckedAt: checkedAt}
	}
	if input.Request.Symbol == "" {
		return Decision{Status: DecisionBlocked, Reason: "missing symbol", CheckedAt: checkedAt}
	}
	if input.Request.EntryPrice <= 0 {
		return Decision{Status: DecisionBlocked, Reason: "missing entry price", CheckedAt: checkedAt}
	}
	if input.Request.StopLossPrice <= 0 {
		return Decision{Status: DecisionBlocked, Reason: "missing stop loss", CheckedAt: checkedAt}
	}
	if input.Request.RequestedSizeBTC <= 0 {
		return Decision{Status: DecisionBlocked, Reason: "missing requested size", CheckedAt: checkedAt}
	}

	decision := p.engine.ValidateTrade(input.Request, input.Market, input.Metrics)
	if !decision.Approved {
		return Decision{Status: DecisionBlocked, Reason: decision.Reason, RiskDecision: decision, CheckedAt: checkedAt}
	}
	if decision.RecommendedSizeBTC <= 0 {
		return Decision{Status: DecisionBlocked, Reason: "risk engine recommended zero size", RiskDecision: decision, CheckedAt: checkedAt}
	}
	return Decision{Status: DecisionApproved, Reason: "approved", RiskDecision: decision, CheckedAt: checkedAt}
}

func (d Decision) Error() error {
	if d.Status == DecisionApproved {
		return nil
	}
	return fmt.Errorf("PRE_TRADE_RISK_BLOCKED: %s", d.Reason)
}

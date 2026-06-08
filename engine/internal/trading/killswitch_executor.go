package trading

import (
	"context"
	"log"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/strategy"
)

// KillSwitchExecutor bridges the institutional killswitch.Executor interface to
// the live paper trading engine. It is passed to killswitch.NewService in main.go
// and is called automatically when kill switch actions fire.
type KillSwitchExecutor struct {
	paper  *execution.PaperClient
	posMgr *positions.Manager
}

// NewKillSwitchExecutor constructs an executor wired to the live paper engine.
func NewKillSwitchExecutor(paper *execution.PaperClient, posMgr *positions.Manager) *KillSwitchExecutor {
	return &KillSwitchExecutor{paper: paper, posMgr: posMgr}
}

// CancelOpenOrders closes all open paper positions at the current market price.
// This implements Mode B: close positions without process shutdown.
func (e *KillSwitchExecutor) CancelOpenOrders(ctx context.Context, reason string) error {
	price := e.paper.GetLastPrice()
	if price <= 0 {
		log.Printf("[KILL SWITCH] CancelOpenOrders: no live price available — skipping flatten (%s)", reason)
		return nil
	}
	open := e.posMgr.GetOpenPositions()
	if len(open) == 0 {
		log.Printf("[KILL SWITCH] CancelOpenOrders: no open positions to cancel (%s)", reason)
		return nil
	}
	e.posMgr.CloseAllPositions(price)
	log.Printf("[KILL SWITCH] CancelOpenOrders: closed %d positions at $%.2f — %s", len(open), price, reason)
	return nil
}

// FlattenPositions performs a hard flatten by submitting a market order to zero
// out net BTC exposure in the paper engine, then closing all tracked positions.
func (e *KillSwitchExecutor) FlattenPositions(ctx context.Context, reason string) error {
	price := e.paper.GetLastPrice()
	position := e.paper.GetPosition("BTCUSDT")
	if position == 0 {
		log.Printf("[KILL SWITCH] FlattenPositions: exposure already zero (%s)", reason)
		return nil
	}
	action := strategy.ActionSell
	size := position
	if position < 0 {
		action = strategy.ActionBuy
		size = -position
	}
	sig := strategy.Signal{
		Symbol:     "BTCUSDT",
		Action:     action,
		TargetSize: size,
	}
	if err := e.paper.PlaceMarketOrder(sig); err != nil {
		log.Printf("[KILL SWITCH] FlattenPositions: market order failed: %v (%s)", err, reason)
		return err
	}
	if price > 0 {
		e.posMgr.CloseAllPositions(price)
	}
	log.Printf("[KILL SWITCH] FlattenPositions: flattened %.4f BTC — %s", size, reason)
	return nil
}

// SendAlert logs the kill switch activation. Extend this to push to Slack/PagerDuty.
func (e *KillSwitchExecutor) SendAlert(_ context.Context, event killswitch.Activation) error {
	log.Printf("[KILL SWITCH ALERT] trigger=%s reason=%s activated_at=%s actions=%v",
		event.Trigger, event.Reason, event.ActivatedAt.Format("2006-01-02T15:04:05Z"), event.Actions)
	return nil
}

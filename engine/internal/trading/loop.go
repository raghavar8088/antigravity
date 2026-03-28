package trading

import (
	"context"
	"log"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
)

type Orchestrator struct {
	client marketdata.MarketDataClient
	algo   strategy.Strategy
	risk   *risk.RiskEngine
	exec   *execution.PaperClient
}

func NewOrchestrator(c marketdata.MarketDataClient, a strategy.Strategy, r *risk.RiskEngine, e *execution.PaperClient) *Orchestrator {
	return &Orchestrator{
		client: c,
		algo:   a,
		risk:   r,
		exec:   e,
	}
}

// Run is the infinite heartbeat of Antigravity Live Trading.
func (o *Orchestrator) Run(ctx context.Context) {
	log.Println("[MASTER LOOP] Booting Live Autonomous Orchestrator...")
	ticks := o.client.GetTickChannel()

	for {
		select {
		case <-ctx.Done():
			log.Println("[MASTER LOOP] Gracefully halting execution loop...")
			return
		case t := <-ticks:
			// 1. Inform the virtual matching engine of the exact current spread price
			o.exec.UpdateMarketState(t.Price)

			// 2. Query Strategy For Intent (assuming high-speed tick model)
			// Notice: This is entirely non-blocking memory operations.
			signals := o.algo.OnTick(t)

			// 3. Routing Engine
			for _, sig := range signals {
				if sig.Action == strategy.ActionHold {
					continue
				}

				log.Printf("[SIGNAL DETECTED] %s: Requests to %s %.4f %s", o.algo.Name(), sig.Action, sig.TargetSize, sig.Symbol)

				// 4. Middle-tier: Risk Circuit Breakers
				err := o.risk.Validate(sig, t.Price)
				if err != nil {
					log.Printf("[RISK DROPPED] %s", err.Error())
					continue // Discard the request forcefully.
				}

				// 5. Safe To Execute
				err = o.exec.PlaceMarketOrder(sig)
				if err != nil {
					log.Printf("[EXECUTION FAILED] %s", err.Error())
				} else {
					// Inform risk engine that capital is now mathematically locked up in position
					o.risk.NotifyFill(sig)
				}
			}
		}
	}
}

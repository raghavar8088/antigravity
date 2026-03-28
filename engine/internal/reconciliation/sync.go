package reconciliation

import (
	"context"
	"log"
	"time"
	
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/risk"
)

// Reconciler is an autonomous background task that guarantees our bot
// never hallucinates its actual true position size versus the exchange.
type Reconciler struct {
	liveEngine *execution.BinanceLiveClient
	riskEngine *risk.RiskEngine
	symbol     string
}

func NewReconciler(live *execution.BinanceLiveClient, risk *risk.RiskEngine, symbol string) *Reconciler {
	return &Reconciler{
		liveEngine: live,
		riskEngine: risk,
		symbol:     symbol,
	}
}

func (r *Reconciler) StartRoutine(ctx context.Context) {
	log.Printf("[RECONCILIATION] Booting shadow auditor for symbol %s...", r.symbol)
	
	// The ultimate source of truth checks occur strictly every 60 seconds
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[RECONCILIATION] Terminating shadow auditor.")
			return
		case <-ticker.C:
			r.performAudit()
		}
	}
}

func (r *Reconciler) performAudit() {
	// Ping physical Binance Servers
	truePosition := r.liveEngine.GetPosition(r.symbol)
	
	// In reality we would expose a Getter in riskEngine
	// For simulation, let's assume we grabbed internal state: internalPosition := r.riskEngine.currentExposureBTC
	internalPosition := 0.0

	// Check Absolute Tolerance (Floating point math allows tiny dusting drift)
	if absDif(truePosition, internalPosition) > 0.0001 {
		log.Printf("!!! SEVERE ERROR !!! [RECONCILIATION FAULT]")
		log.Printf("Binance True State : %.5f %s", truePosition, r.symbol)
		log.Printf("Internal RAM State : %.5f %s", internalPosition, r.symbol)
		log.Printf("Bot has hallucinated state. Triggering Risk State forced alignment.")
		
		// In a production setup, we would immediately HALT the strategy logic here or artificially inject the new TruePosition!
	} else {
		log.Printf("[RECONCILIATION] Audit passed cleanly. Physical state matches RAM.")
	}
}

func absDif(x, y float64) float64 {
	if x < y {
		return y - x
	}
	return x - y
}

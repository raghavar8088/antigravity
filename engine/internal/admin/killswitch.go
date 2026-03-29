package admin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/persistence"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
)

// KillSwitchController maps the API endpoints that control the bot forcibly.
type KillSwitchController struct {
	cancelFunc context.CancelFunc
	engine     execution.Engine
	journal    *execution.TradeJournal
	posMgr     *positions.Manager
	dbStore    *persistence.Store
	riskEngine *risk.RiskEngine
	tracker    *risk.StrategyTracker
	ctx        context.Context
}

func NewKillSwitch(
	ctx context.Context,
	cancel context.CancelFunc,
	eng execution.Engine,
	journal *execution.TradeJournal,
	posMgr *positions.Manager,
	dbStore *persistence.Store,
	riskEngine *risk.RiskEngine,
	tracker *risk.StrategyTracker,
) *KillSwitchController {
	return &KillSwitchController{
		ctx:        ctx,
		cancelFunc: cancel,
		engine:     eng,
		journal:    journal,
		posMgr:     posMgr,
		dbStore:    dbStore,
		riskEngine: riskEngine,
		tracker:    tracker,
	}
}

// HandleTrigger is a POST listener mapped to the Dashboard UI's big red alert button.
func (k *KillSwitchController) HandleTrigger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("!!! SEVERITY 1: EXTERNAL KILL SWITCH TRIGGERED VIA HTTP !!!")

	// 1. Immediately halt the main orchestrator loop from accepting any more WebSocket Ticks
	k.cancelFunc()
	log.Println("[SHUTDOWN] Primary Trading Context Severed.")

	// 2. Query physical active positions on Binance
	currentRisk := k.engine.GetPosition("BTCUSDT")

	// 3. Force completely flat! Dump any active inventory immediately at whatever Market Price is.
	if currentRisk > 0 {
		log.Printf("[SHUTDOWN] Abandoning Position! Dumping %.4f BTCUSDT to Market...", currentRisk)

		dumpOrder := strategy.Signal{
			Symbol:     "BTCUSDT",
			Action:     strategy.ActionSell,
			TargetSize: currentRisk, // Entire position
		}

		err := k.engine.PlaceMarketOrder(dumpOrder)
		if err != nil {
			log.Printf("CRITICAL SHUTDOWN ERROR: Failed to dump market! Manual intervention required ASAP. Err: %s", err)
		} else {
			log.Println("[SHUTDOWN] SUCCESS - Position flatlined to 0.0 BTC. Risk is neutralized.")
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "Systems Halted",
		"message": "Market execution loop forcibly terminated and positions closed.",
	})
}

// HandleReset performs a full account reset: wipes in-memory paper balance, positions,
// trade journal, risk engine counters, AND persists the clean state to the database
// so the engine starts fresh on the next restart as well.
func (k *KillSwitchController) HandleReset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("[ADMIN] 🔄 FULL ACCOUNT RESET requested — wiping all state...")

	// 1. Reset in-memory paper trading balance ($100k starting)
	if err := k.engine.ResetAccount(); err != nil {
		log.Printf("[ADMIN] ⚠️  Failed to reset paper engine: %v", err)
		http.Error(w, "Failed to reset paper engine", http.StatusInternalServerError)
		return
	}
	log.Println("[ADMIN] ✅ Paper balance reset to $100,000")

	// 2. Wipe all open positions from memory
	k.posMgr.Reset()
	log.Println("[ADMIN] ✅ Open positions cleared")

	// 3. Wipe the trade journal (history + counters)
	k.journal.Reset()
	log.Println("[ADMIN] ✅ Trade journal cleared")

	// 4. Reset risk counters used by the live stats API.
	if k.riskEngine != nil {
		k.riskEngine.Reset()
		log.Println("[ADMIN] Risk engine counters reset")
	}

	// 5. Reset strategy state so strategies return to a clean running state.
	if k.tracker != nil {
		k.tracker.Reset()
		log.Println("[ADMIN] Strategy tracker reset")
	}

	// 6. Reset database so the clean state persists across restarts
	if k.dbStore != nil {
		if err := k.dbStore.ResetState(k.ctx); err != nil {
			log.Printf("[ADMIN] ⚠️  Failed to reset database: %v", err)
			// Non-fatal: in-memory state is already clean; DB will be overwritten on next save tick
		} else {
			log.Println("[ADMIN] ✅ Database state reset to defaults")
		}
	}

	log.Println("[ADMIN] 🎉 FULL RESET COMPLETE — Engine running with $100,000 fresh account")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "Account Reset",
		"message":     "All balances, positions, and trade history have been wiped. Starting fresh with $100,000.",
		"newBalance":  100000.0,
		"openTrades":  0,
		"totalTrades": 0,
	})
}

package admin

import (
	"context"
	"encoding/json"
	"log"
	"math"
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
	paper      *execution.PaperClient
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
	paper *execution.PaperClient,
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
		paper:      paper,
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

	// 1. Immediately halt the main orchestrator loop from accepting any more WebSocket ticks.
	k.cancelFunc()
	log.Println("[SHUTDOWN] Primary Trading Context Severed.")

	// 2. Query the current signed net BTC position.
	currentRisk := k.engine.GetPosition("BTCUSDT")

	// 3. Force completely flat. Positive positions are sold, negative positions are bought back.
	if currentRisk != 0 {
		action := strategy.ActionSell
		targetSize := currentRisk
		if currentRisk < 0 {
			action = strategy.ActionBuy
			targetSize = math.Abs(currentRisk)
		}

		log.Printf("[SHUTDOWN] Flattening signed exposure %.4f BTC with %s %.4f BTC...", currentRisk, action, targetSize)
		dumpOrder := strategy.Signal{
			Symbol:     "BTCUSDT",
			Action:     action,
			TargetSize: targetSize,
		}

		err := k.engine.PlaceMarketOrder(dumpOrder)
		if err != nil {
			log.Printf("CRITICAL SHUTDOWN ERROR: Failed to flatten market exposure. Manual intervention required ASAP. Err: %s", err)
		} else {
			log.Println("[SHUTDOWN] SUCCESS - Net BTC exposure flattened to 0.0.")
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "Systems Halted",
		"message": "Market execution loop forcibly terminated and positions closed.",
	})
}

// HandleCloseAll closes all open paper positions at the current market price
// while keeping the engine running.
func (k *KillSwitchController) HandleCloseAll(w http.ResponseWriter, r *http.Request) {
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

	if k.paper == nil {
		http.Error(w, "Paper execution engine unavailable", http.StatusServiceUnavailable)
		return
	}

	exitPrice := k.paper.GetLastPrice()
	if exitPrice <= 0 {
		http.Error(w, "No live market price available yet", http.StatusConflict)
		return
	}

	openPositions := k.posMgr.GetOpenPositions()
	if len(openPositions) == 0 {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "No Positions",
			"message":         "There are no open positions to close.",
			"closedPositions": 0,
			"exitPrice":       exitPrice,
		})
		return
	}

	k.posMgr.CloseAllPositions(exitPrice)
	log.Printf("[ADMIN] Force-closed %d open positions at $%.2f", len(openPositions), exitPrice)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "Positions Closed",
		"message":         "All open paper positions were closed at the current market price.",
		"closedPositions": len(openPositions),
		"exitPrice":       exitPrice,
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

	log.Println("[ADMIN] FULL ACCOUNT RESET requested - wiping all state...")

	// 1. Reset in-memory paper trading balance ($1M starting)
	if err := k.engine.ResetAccount(); err != nil {
		log.Printf("[ADMIN] Failed to reset paper engine: %v", err)
		http.Error(w, "Failed to reset paper engine", http.StatusInternalServerError)
		return
	}
	log.Println("[ADMIN] Paper balance reset to $1,000,000")

	// 2. Wipe all open positions from memory
	k.posMgr.Reset()
	log.Println("[ADMIN] Open positions cleared")

	// 3. Wipe the trade journal (history + counters)
	k.journal.Reset()
	log.Println("[ADMIN] Trade journal cleared")

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
			log.Printf("[ADMIN] Failed to reset database: %v", err)
			// Non-fatal: in-memory state is already clean; DB will be overwritten on next save tick
		} else {
			log.Println("[ADMIN] Database state reset to defaults")
		}
	}

	log.Println("[ADMIN] FULL RESET COMPLETE - Engine running with $1,000,000 fresh account")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "Account Reset",
		"message":     "All balances, positions, and trade history have been wiped. Starting fresh with $1,000,000.",
		"newBalance":  1000000.0,
		"openTrades":  0,
		"totalTrades": 0,
	})
}

// HandleClearHistory clears completed trade history and performance counters
// while leaving balances and open positions intact.
func (k *KillSwitchController) HandleClearHistory(w http.ResponseWriter, r *http.Request) {
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

	openPositions := 0
	if k.posMgr != nil {
		openPositions = len(k.posMgr.GetOpenPositions())
	}

	log.Println("[ADMIN] TRADE HISTORY RESET requested - clearing journal and persisted trade records...")

	if k.journal != nil {
		k.journal.Reset()
		log.Println("[ADMIN] Trade journal cleared")
	}

	if k.tracker != nil {
		k.tracker.Reset()
		log.Println("[ADMIN] Strategy tracker reset")
	}

	if k.riskEngine != nil {
		k.riskEngine.ResetDaily()
		log.Println("[ADMIN] Risk daily counters reset")
	}

	if k.dbStore != nil {
		if err := k.dbStore.ClearTradeHistory(k.ctx); err != nil {
			log.Printf("[ADMIN] Failed to clear trade history in database: %v", err)
			http.Error(w, "Failed to clear trade history in database", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":               "Trade History Cleared",
		"message":              "Completed trade history and performance counters were cleared. Balances and open positions were preserved.",
		"openPositionsKept":    openPositions,
		"totalTrades":          0,
		"strategyStatsCleared": true,
	})
}

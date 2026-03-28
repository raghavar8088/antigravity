package admin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/strategy"
)

// KillSwitchController maps the API endpoint that stops the bot forcibly.
type KillSwitchController struct {
	cancelFunc context.CancelFunc
	engine     execution.Engine
}

func NewKillSwitch(cancel context.CancelFunc, eng execution.Engine) *KillSwitchController {
	return &KillSwitchController{
		cancelFunc: cancel,
		engine:     eng,
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
		"status": "Systems Halted",
		"message": "Market execution loop forcibly terminated and positions closed.",
	})
}

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

	log.Println("[ADMIN] Reset request received. Resetting paper trading account state.")

	err := k.engine.ResetAccount()
	if err != nil {
		http.Error(w, "Failed to reset account", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "Account Reset",
		"message": "Paper trading balances and positions have been reset to starting state.",
	})
}

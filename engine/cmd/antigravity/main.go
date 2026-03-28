package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"antigravity-engine/internal/admin"
	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/risk"
	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/trading"
)

func main() {
	fmt.Println("Starting Antigravity Trading Engine... [PROD HARDENED]")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. WebSocket Live Stream
	binanceClient := marketdata.NewBinanceClient()
	go func() {
		err := binanceClient.Connect(ctx, []string{"btcusdt"})
		if err != nil {
			log.Fatalf("Fatal error connecting to Binance: %v", err)
		}
	}()

	// 2. Logic Controller
	algo := strategy.NewMovingAverageCrossover(5, 10)
	
	// 3. Middle Risk Layer (Mandatory limit checking)
	riskProfile := risk.RiskProfile{
		MaxPositionBTC:  0.5,      
		MaxCapitalUSD:   20000.00, 
		MaxDailyLossPct: 0.10,     
	}
	riskEngine := risk.NewRiskEngine(riskProfile)
	
	// 4. Paper Environment for executing locally during Demo
	paperExecute := execution.NewPaperClient(100000.0) 

	// 5. Build and Fire Autonomous Brain Pipeline
	orchestrator := trading.NewOrchestrator(binanceClient, algo, riskEngine, paperExecute)
	go orchestrator.Run(ctx)


	// ============================================
	// Admin HTTP Interfaces (Metrics & Safety)
	// ============================================

	killswitch := admin.NewKillSwitch(cancel, paperExecute)

	// Inject the Phase 7 Prometheus metrics native publisher
	http.Handle("/metrics", promhttp.Handler())
	
	// Inject the Phase 7 UI Panic Red Button Endpoint
	http.HandleFunc("/api/admin/kill", killswitch.HandleTrigger)
	http.HandleFunc("/api/admin/reset", killswitch.HandleReset)
	
	// Native React Dashboard Poller Endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok", "service": "antigravity-engine"}`))
	})
	
	go func() {
		fmt.Println("Admin REST Engine listening on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Println("Admin Server error:", err)
		}
	}()

	// Hardware Fallback Hook
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Hardware Bash Kill Signal: Shutting down entire engine loop...")
	cancel() // Ensures WS + Tick processes end natively!
	binanceClient.Close()
	time.Sleep(1 * time.Second)
	log.Println("Systems offline.")
}

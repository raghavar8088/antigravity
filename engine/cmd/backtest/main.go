package main

import (
	"log"
	
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
	"antigravity-engine/internal/backtest"
)

func main() {
	log.Println("Initializing Antigravity Backtest Simulator...")

	// 1. Generate Mock Historic BTC Candles
	// In reality, Phase 3 pulls direct from our TimescaleDB instance.
	mockData := loadMockHistory()

	// 2. Wrap Algorithmic Strategy (e.g. 5m and 10m moving averages)
	algo := strategy.NewMovingAverageCrossover(5, 10)

	// 3. Initiate the Simulator with $100,000 USD Wallet 
	sim := backtest.NewSimulator(algo, 100000.0, mockData)

	// 4. Run Physics Loop!
	finalState := sim.Run()

	// 5. Output mathematical metrics
	backtest.CalculateMetrics(finalState)
}

// loadMockHistory gives us fake sine-wave price action for now
func loadMockHistory() []marketdata.Tick {
	var data []marketdata.Tick
	basePrice := 65000.0
	
	// Generate 100 fake consecutive minute candles
	for i := 0; i < 100; i++ {
		// A dummy algorithm creating artificial trends to test crossover
		shift := float64((i % 10) - 4) * 100.0 
		basePrice += shift
		
		data = append(data, marketdata.Tick{
			Symbol:  "BTCUSDT",
			Price:   basePrice,
			TimeMs:  int64(1700000000000 + (i * 60000)), // Incrementing 60 seconds
		})
	}
	return data
}

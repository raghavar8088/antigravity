package backtest

import (
	"log"
	
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

// PnLState tracks the entire virtual portfolio state throughout execution.
type PnLState struct {
	InitialBalance float64
	Balance        float64
	PositionSize   float64
	AverageEntry   float64
	
	Trades []TradeRecord
}

type TradeRecord struct {
	Side  strategy.Action
	Price float64
	Qty   float64
	PnL   float64 // Local Realized loop.
}

type Simulator struct {
	strat  strategy.Strategy
	state  *PnLState
	mockDB []marketdata.Tick // Representing our time-series payload
}

func NewSimulator(strat strategy.Strategy, initialCapital float64, data []marketdata.Tick) *Simulator {
	return &Simulator{
		strat: strat,
		state: &PnLState{
			InitialBalance: initialCapital,
			Balance:        initialCapital,
			PositionSize:   0,
		},
		mockDB: data,
	}
}

// Run executes the massive sequence of historical ticks over the Strategy securely.
func (s *Simulator) Run() *PnLState {
	log.Printf("Starting Backtest over [%d] Historical Ticks for Strategy: %s\n", len(s.mockDB), s.strat.Name())

	for _, tick := range s.mockDB {
		// Funnel the historical data into the Intelligence core
		signals := s.strat.OnCandle(tick)

		// Naive Paper Matching Loop
		for _, sig := range signals {
			if sig.Action == strategy.ActionHold {
				continue 
			}
			
			// Simulate a BUY execution (Going inherently Long)
			if sig.Action == strategy.ActionBuy && s.state.PositionSize <= 0 {
				tradeCost := tick.Price * sig.TargetSize
				if s.state.Balance >= tradeCost {
					// We can afford it!
					s.state.Balance -= tradeCost
					s.state.PositionSize += sig.TargetSize
					s.state.AverageEntry = tick.Price
					
					s.state.Trades = append(s.state.Trades, TradeRecord{Side: strategy.ActionBuy, Price: tick.Price, Qty: sig.TargetSize})
				}
			}

			// Simulate a SELL execution (Closing the Long)
			if sig.Action == strategy.ActionSell && s.state.PositionSize > 0 {
				revenue := tick.Price * s.state.PositionSize
				tradePnL := revenue - (s.state.AverageEntry * s.state.PositionSize) // Fixed Math!
				
				s.state.Balance += revenue
				
				s.state.Trades = append(s.state.Trades, TradeRecord{Side: strategy.ActionSell, Price: tick.Price, Qty: s.state.PositionSize, PnL: tradePnL})
				
				// Reset virtual exposure
				s.state.PositionSize = 0 
				s.state.AverageEntry = 0
			}
		}
	}
	
	log.Println("Backtest Complete!")
	return s.state
}

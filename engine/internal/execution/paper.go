package execution

import (
	"log"

	"antigravity-engine/internal/strategy"
)

// PaperClient fakes live executions by storing balances locally in RAM, 
// using whatever the most recent market price stream is.
type PaperClient struct {
	initialUSD   float64
	balanceUSD   float64
	positionBTC  float64
	
	lastKnownPrice float64 // Piped directly from WebSocket Tick feed
}

func NewPaperClient(startingUSD float64) *PaperClient {
	return &PaperClient{
		initialUSD:    startingUSD,
		balanceUSD:    startingUSD,
		positionBTC:   0,
		lastKnownPrice: 0,
	}
}

// UpdateMarketState allows the master loop to constantly feed the latest tick explicitly here.
func (p *PaperClient) UpdateMarketState(price float64) {
	p.lastKnownPrice = price
}

func (p *PaperClient) PlaceMarketOrder(sig strategy.Signal) error {
	log.Printf("[PAPER EXEC] Attempting to place %s order for %.4f BTC @ ~$%.2f", sig.Action, sig.TargetSize, p.lastKnownPrice)

	if sig.Action == strategy.ActionBuy {
		cost := sig.TargetSize * p.lastKnownPrice
		if cost > p.balanceUSD {
			log.Printf("[PAPER EXEC] INSUFFICIENT FUNDS! Wants $%.2f, has $%.2f", cost, p.balanceUSD)
			return nil
		}
		
		p.balanceUSD -= cost
		p.positionBTC += sig.TargetSize
		log.Printf("[PAPER EXEC] SUCCESS! Bought %.4f BTC. Wallet: $%.2f", sig.TargetSize, p.balanceUSD)
	
	} else if sig.Action == strategy.ActionSell {
		if p.positionBTC < sig.TargetSize {
			log.Printf("[PAPER EXEC] INSUFFICIENT BTC! Wants %.4f, has %.4f", sig.TargetSize, p.positionBTC)
			return nil
		}

		revenue := sig.TargetSize * p.lastKnownPrice
		p.positionBTC -= sig.TargetSize
		p.balanceUSD += revenue
		log.Printf("[PAPER EXEC] SUCCESS! Sold %.4f BTC. Wallet: $%.2f", sig.TargetSize, p.balanceUSD)
	}

	return nil
}

func (p *PaperClient) GetPosition(symbol string) float64 {
	if symbol == "BTCUSDT" {
		return p.positionBTC
	}
	return 0
}

func (p *PaperClient) GetBalanceUSD() float64 {
	return p.balanceUSD
}

func (p *PaperClient) ResetAccount() error {
	p.positionBTC = 0
	p.balanceUSD = p.initialUSD
	p.lastKnownPrice = 0
	return nil
}

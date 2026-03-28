package execution

import (
	"log"

	"antigravity-engine/internal/strategy"
)

// PaperClient fakes live executions by storing balances locally in RAM,
// using whatever the most recent market price stream is.
type PaperClient struct {
	initialUSD     float64
	balanceUSD     float64
	positionBTC    float64
	lastKnownPrice float64

	// Fee simulation
	feeRate float64 // Per-side fee (e.g. 0.001 = 0.1%)
	totalFeesPaid float64
}

func NewPaperClient(startingUSD float64) *PaperClient {
	return &PaperClient{
		initialUSD:     startingUSD,
		balanceUSD:     startingUSD,
		positionBTC:    0,
		lastKnownPrice: 0,
		feeRate:        0.001, // 0.1% Binance taker fee
	}
}

// UpdateMarketState allows the master loop to constantly feed the latest tick.
func (p *PaperClient) UpdateMarketState(price float64) {
	p.lastKnownPrice = price
}

func (p *PaperClient) PlaceMarketOrder(sig strategy.Signal) error {
	// Apply slippage (0.01% adverse)
	execPrice := p.lastKnownPrice
	if sig.Action == strategy.ActionBuy {
		execPrice = p.lastKnownPrice * 1.0001
	} else {
		execPrice = p.lastKnownPrice * 0.9999
	}

	if sig.Action == strategy.ActionBuy {
		cost := sig.TargetSize * execPrice
		fee := cost * p.feeRate
		totalCost := cost + fee

		if totalCost > p.balanceUSD {
			log.Printf("[PAPER EXEC] INSUFFICIENT FUNDS! Wants $%.2f, has $%.2f", totalCost, p.balanceUSD)
			return nil
		}

		p.balanceUSD -= totalCost
		p.positionBTC += sig.TargetSize
		p.totalFeesPaid += fee
		log.Printf("[PAPER EXEC] BUY %.4f BTC @ $%.2f (fee: $%.4f) | Balance: $%.2f",
			sig.TargetSize, execPrice, fee, p.balanceUSD)

	} else if sig.Action == strategy.ActionSell {
		if p.positionBTC < sig.TargetSize {
			// Allow selling even without position (simulated short)
			log.Printf("[PAPER EXEC] SHORT %.4f BTC @ $%.2f", sig.TargetSize, execPrice)
		}

		revenue := sig.TargetSize * execPrice
		fee := revenue * p.feeRate
		netRevenue := revenue - fee

		p.positionBTC -= sig.TargetSize
		if p.positionBTC < 0 {
			p.positionBTC = 0
		}
		p.balanceUSD += netRevenue
		p.totalFeesPaid += fee
		log.Printf("[PAPER EXEC] SELL %.4f BTC @ $%.2f (fee: $%.4f) | Balance: $%.2f",
			sig.TargetSize, execPrice, fee, p.balanceUSD)
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

func (p *PaperClient) GetTotalFees() float64 {
	return p.totalFeesPaid
}

func (p *PaperClient) GetLastPrice() float64 {
	return p.lastKnownPrice
}

func (p *PaperClient) ResetAccount() error {
	p.positionBTC = 0
	p.balanceUSD = p.initialUSD
	p.lastKnownPrice = 0
	p.totalFeesPaid = 0
	return nil
}

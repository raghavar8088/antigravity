package execution

import (
	"fmt"
	"log"
	"math"

	"antigravity-engine/internal/strategy"
)

// PaperClient fakes live executions by storing balances locally in RAM,
// using whatever the most recent market price stream is.
type PaperClient struct {
	initialUSD     float64
	balanceUSD     float64
	positionBTC    float64 // Signed net BTC position; negative values represent shorts.
	lastKnownPrice float64

	// Fee simulation
	feeRate       float64 // Per-side fee (e.g. 0.001 = 0.1%)
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

func isSupportedBTCSymbol(symbol string) bool {
	return symbol == "BTCUSDT" || symbol == "BTC-USD" || symbol == "BTC-USDT"
}

func clampNearZero(value float64) float64 {
	if math.Abs(value) < 1e-9 {
		return 0
	}
	return value
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
	} else if sig.Action == strategy.ActionSell {
		execPrice = p.lastKnownPrice * 0.9999
	}

	if sig.Action == strategy.ActionBuy {
		cost := sig.TargetSize * execPrice
		fee := cost * p.feeRate
		totalCost := cost + fee

		if totalCost > p.balanceUSD {
			log.Printf("[PAPER EXEC] INSUFFICIENT FUNDS! Wants $%.2f, has $%.2f", totalCost, p.balanceUSD)
			return fmt.Errorf("insufficient funds: wants %.2f, has %.2f", totalCost, p.balanceUSD)
		}

		p.balanceUSD -= totalCost
		p.positionBTC += sig.TargetSize
		p.positionBTC = clampNearZero(p.positionBTC)
		p.totalFeesPaid += fee
		log.Printf("[PAPER EXEC] BUY %.4f BTC @ $%.2f (fee: $%.4f) | Balance: $%.2f",
			sig.TargetSize, execPrice, fee, p.balanceUSD)

	} else if sig.Action == strategy.ActionSell {
		if p.positionBTC <= 0 {
			log.Printf("[PAPER EXEC] SHORT %.4f BTC @ $%.2f", sig.TargetSize, execPrice)
		}

		revenue := sig.TargetSize * execPrice
		fee := revenue * p.feeRate
		netRevenue := revenue - fee

		p.positionBTC -= sig.TargetSize
		p.positionBTC = clampNearZero(p.positionBTC)
		p.balanceUSD += netRevenue
		p.totalFeesPaid += fee
		log.Printf("[PAPER EXEC] SELL %.4f BTC @ $%.2f (fee: $%.4f) | Balance: $%.2f",
			sig.TargetSize, execPrice, fee, p.balanceUSD)
	}

	return nil
}

func (p *PaperClient) GetPosition(symbol string) float64 {
	if isSupportedBTCSymbol(symbol) {
		return p.positionBTC
	}
	return 0
}

func (p *PaperClient) GetBalanceUSD() float64 {
	return p.balanceUSD
}

// GetEquityUSD returns cash plus mark-to-market value of the signed BTC position.
func (p *PaperClient) GetEquityUSD() float64 {
	if p.lastKnownPrice <= 0 {
		return p.balanceUSD
	}
	return p.balanceUSD + (p.positionBTC * p.lastKnownPrice)
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

// SettlePosition updates the paper balance when a position is closed or
// partially reduced, crediting USD back after a long close or debiting for a
// short cover.
func (p *PaperClient) SettlePosition(side strategy.Action, size, exitPrice float64) {
	if side == strategy.ActionBuy {
		// Closing a LONG position: sell BTC back at exit price
		revenue := size * exitPrice
		fee := revenue * p.feeRate
		p.balanceUSD += revenue - fee
		p.positionBTC -= size
		p.positionBTC = clampNearZero(p.positionBTC)
		p.totalFeesPaid += fee
		log.Printf("[PAPER SETTLE] CLOSE LONG: SELL %.4f BTC @ $%.2f | Fee: $%.4f | Balance: $%.2f",
			size, exitPrice, fee, p.balanceUSD)
	} else {
		// Closing a SHORT position: buy BTC back at exit price
		cost := size * exitPrice
		fee := cost * p.feeRate
		p.balanceUSD -= cost + fee
		p.positionBTC += size
		p.positionBTC = clampNearZero(p.positionBTC)
		p.totalFeesPaid += fee
		log.Printf("[PAPER SETTLE] CLOSE SHORT: BUY %.4f BTC @ $%.2f | Fee: $%.4f | Balance: $%.2f",
			size, exitPrice, fee, p.balanceUSD)
	}
}

// RestoreBalance restores balance and fees from database on restart.
func (p *PaperClient) RestoreBalance(balance, fees float64) {
	p.balanceUSD = balance
	p.totalFeesPaid = fees
	log.Printf("[PAPER EXEC] Restored balance: $%.2f | Fees: $%.4f", balance, fees)
}

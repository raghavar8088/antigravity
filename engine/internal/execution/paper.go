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
}

func NewPaperClient(startingUSD float64) *PaperClient {
	return &PaperClient{
		initialUSD:     startingUSD,
		balanceUSD:     startingUSD,
		positionBTC:    0,
		lastKnownPrice: 0,
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

func (p *PaperClient) executionPrice(mode OrderMode, action strategy.Action) float64 {
	execPrice := p.lastKnownPrice
	switch mode {
	case OrderModePostOnly:
		if action == strategy.ActionBuy {
			return p.lastKnownPrice * 0.99995
		}
		if action == strategy.ActionSell {
			return p.lastKnownPrice * 1.00005
		}
	case OrderModeIOC:
		if action == strategy.ActionBuy {
			return p.lastKnownPrice * 1.00012
		}
		if action == strategy.ActionSell {
			return p.lastKnownPrice * 0.99988
		}
	default:
		if action == strategy.ActionBuy {
			return p.lastKnownPrice * 1.0001
		}
		if action == strategy.ActionSell {
			return p.lastKnownPrice * 0.9999
		}
	}
	return execPrice
}

func (p *PaperClient) applyFill(sig strategy.Signal, execPrice float64, mode OrderMode) error {
	if sig.Action == strategy.ActionBuy {
		cost := sig.TargetSize * execPrice

		if cost > p.balanceUSD {
			log.Printf("[PAPER EXEC] INSUFFICIENT FUNDS! Wants $%.2f, has $%.2f", cost, p.balanceUSD)
			return fmt.Errorf("insufficient funds: wants %.2f, has %.2f", cost, p.balanceUSD)
		}

		p.balanceUSD -= cost
		p.positionBTC += sig.TargetSize
		p.positionBTC = clampNearZero(p.positionBTC)
		log.Printf("[PAPER EXEC] %s BUY %.4f BTC @ $%.2f | Balance: $%.2f",
			mode, sig.TargetSize, execPrice, p.balanceUSD)

	} else if sig.Action == strategy.ActionSell {
		if p.positionBTC <= 0 {
			log.Printf("[PAPER EXEC] %s SHORT %.4f BTC @ $%.2f", mode, sig.TargetSize, execPrice)
		}

		revenue := sig.TargetSize * execPrice
		p.positionBTC -= sig.TargetSize
		p.positionBTC = clampNearZero(p.positionBTC)
		p.balanceUSD += revenue
		log.Printf("[PAPER EXEC] %s SELL %.4f BTC @ $%.2f | Balance: $%.2f",
			mode, sig.TargetSize, execPrice, p.balanceUSD)
	}

	return nil
}

func (p *PaperClient) ExecuteSignal(sig strategy.Signal, mode OrderMode) (FillResult, error) {
	execPrice := p.executionPrice(mode, sig.Action)
	if execPrice <= 0 {
		return FillResult{}, fmt.Errorf("no market price available for execution")
	}
	if err := p.applyFill(sig, execPrice, mode); err != nil {
		return FillResult{}, err
	}
	return FillResult{
		ExecPrice: execPrice,
		OrderMode: mode,
	}, nil
}

func (p *PaperClient) PlaceMarketOrder(sig strategy.Signal) error {
	_, err := p.ExecuteSignal(sig, OrderModeMarket)
	return err
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
	return 0
}

func (p *PaperClient) GetLastPrice() float64 {
	return p.lastKnownPrice
}

func (p *PaperClient) ResetAccount() error {
	p.positionBTC = 0
	p.balanceUSD = p.initialUSD
	p.lastKnownPrice = 0
	return nil
}

// SettlePosition updates the paper balance when a position is closed or
// partially reduced, crediting USD back after a long close or debiting for a
// short cover.
func (p *PaperClient) SettlePosition(side strategy.Action, size, exitPrice float64) {
	if side == strategy.ActionBuy {
		// Closing a LONG position: sell BTC back at exit price
		revenue := size * exitPrice
		p.balanceUSD += revenue
		p.positionBTC -= size
		p.positionBTC = clampNearZero(p.positionBTC)
		log.Printf("[PAPER SETTLE] CLOSE LONG: SELL %.4f BTC @ $%.2f | Balance: $%.2f",
			size, exitPrice, p.balanceUSD)
	} else {
		// Closing a SHORT position: buy BTC back at exit price
		cost := size * exitPrice
		p.balanceUSD -= cost
		p.positionBTC += size
		p.positionBTC = clampNearZero(p.positionBTC)
		log.Printf("[PAPER SETTLE] CLOSE SHORT: BUY %.4f BTC @ $%.2f | Balance: $%.2f",
			size, exitPrice, p.balanceUSD)
	}
}

// RestoreBalance restores balance and fees from database on restart.
func (p *PaperClient) RestoreBalance(balance, fees float64) {
	p.balanceUSD = balance
	log.Printf("[PAPER EXEC] Restored balance: $%.2f | Fees ignored in zero-fee mode (was: $%.4f)", balance, fees)
}

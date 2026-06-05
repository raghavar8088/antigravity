package execution

import (
	"fmt"
	"log"
	"math"

	"antigravity-engine/internal/strategy"
)

// Exchange fee constants (2026 base tier, Binance USD-M Futures).
// Taker orders remove liquidity (market orders); maker orders add it (limit/post-only).
const (
	BinanceFuturesTakerFeePct = 0.00050 // 0.05%
	BinanceFuturesMakerFeePct = 0.00020 // 0.02%
)

// PaperClient fakes live executions by storing balances locally in RAM,
// using whatever the most recent market price stream is.
// All fills deduct the Binance taker fee so paper PnL reflects real net returns.
type PaperClient struct {
	initialUSD     float64
	balanceUSD     float64
	positionBTC    float64 // Signed net BTC position; negative values represent shorts.
	lastKnownPrice float64
	totalFeesUSD   float64 // Cumulative taker fees deducted across all fills.
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

// takerFeePct returns 0 for post-only (maker) orders, BinanceFuturesTakerFeePct otherwise.
func takerFeePct(mode OrderMode) float64 {
	if mode == OrderModePostOnly {
		return BinanceFuturesMakerFeePct
	}
	return BinanceFuturesTakerFeePct
}

func (p *PaperClient) applyFill(sig strategy.Signal, execPrice float64, mode OrderMode) error {
	notional := sig.TargetSize * execPrice
	fee := notional * takerFeePct(mode)

	if sig.Action == strategy.ActionBuy {
		totalCost := notional + fee
		if totalCost > p.balanceUSD {
			log.Printf("[PAPER EXEC] INSUFFICIENT FUNDS! Wants $%.2f (incl fee $%.4f), has $%.2f",
				totalCost, fee, p.balanceUSD)
			return fmt.Errorf("insufficient funds: wants %.2f, has %.2f", totalCost, p.balanceUSD)
		}

		p.balanceUSD -= totalCost
		p.positionBTC += sig.TargetSize
		p.positionBTC = clampNearZero(p.positionBTC)
		p.totalFeesUSD += fee
		log.Printf("[PAPER EXEC] %s BUY %.4f BTC @ $%.2f | Fee: $%.4f | Balance: $%.2f",
			mode, sig.TargetSize, execPrice, fee, p.balanceUSD)

	} else if sig.Action == strategy.ActionSell {
		if p.positionBTC <= 0 {
			log.Printf("[PAPER EXEC] %s SHORT %.4f BTC @ $%.2f", mode, sig.TargetSize, execPrice)
		}

		// Revenue net of taker fee (fee paid on sell side too).
		netRevenue := notional - fee
		p.positionBTC -= sig.TargetSize
		p.positionBTC = clampNearZero(p.positionBTC)
		p.balanceUSD += netRevenue
		p.totalFeesUSD += fee
		log.Printf("[PAPER EXEC] %s SELL %.4f BTC @ $%.2f | Fee: $%.4f | Balance: $%.2f",
			mode, sig.TargetSize, execPrice, fee, p.balanceUSD)
	}

	return nil
}

func (p *PaperClient) ExecuteSignal(sig strategy.Signal, mode OrderMode) (FillResult, error) {
	requestedPrice := p.lastKnownPrice
	execPrice := p.executionPrice(mode, sig.Action)
	if execPrice <= 0 {
		return FillResult{}, fmt.Errorf("no market price available for execution")
	}
	if err := p.applyFill(sig, execPrice, mode); err != nil {
		return FillResult{}, err
	}
	return FillResult{
		ExecPrice:      execPrice,
		OrderMode:      mode,
		RequestedPrice: requestedPrice,
		SlippageBps:    signedSlippageBps(requestedPrice, execPrice, sig.Action),
	}, nil
}

// signedSlippageBps returns the directional slippage of a fill in basis points.
// Positive = adverse (worse price for the trade's direction).
func signedSlippageBps(requested, filled float64, action strategy.Action) float64 {
	if requested <= 0 || filled <= 0 {
		return 0
	}
	raw := (filled - requested) / requested * 10000
	if action == strategy.ActionSell {
		// A SELL filled below the reference is adverse → report adverse as positive.
		return -raw
	}
	return raw
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
	return p.totalFeesUSD
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

// RestoreBalance restores balance and accumulated fees from database on restart.
func (p *PaperClient) RestoreBalance(balance, fees float64) {
	p.balanceUSD = balance
	p.totalFeesUSD = fees
	log.Printf("[PAPER EXEC] Restored balance: $%.2f | Cumulative fees: $%.4f", balance, fees)
}

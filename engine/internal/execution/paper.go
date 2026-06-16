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
	longBTC        float64 // BUG 11: gross long BTC held
	shortBTC       float64 // BUG 11: gross short BTC sold
	lastKnownPrice float64
	totalFeesUSD   float64 // Cumulative taker fees deducted across all fills.
}

// NetPosition returns longBTC - shortBTC (signed net exposure). BUG 11.
func (p *PaperClient) NetPosition() float64 {
	return clampNearZero(p.longBTC - p.shortBTC)
}

// GrossExposure returns longBTC + shortBTC (total absolute exposure). BUG 11.
func (p *PaperClient) GrossExposure() float64 {
	return p.longBTC + p.shortBTC
}

func NewPaperClient(startingUSD float64) *PaperClient {
	return &PaperClient{
		initialUSD:     startingUSD,
		balanceUSD:     startingUSD,
		longBTC:        0, // BUG 11
		shortBTC:       0, // BUG 11
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

// clampBalance enforces the invariant: paper cash balance cannot go below zero.
func (p *PaperClient) clampBalance(context string) {
	if p.balanceUSD >= 0 {
		return
	}
	log.Printf("[PAPER BALANCE INVARIANT] %s: balance %.4f clamped to 0", context, p.balanceUSD)
	p.balanceUSD = 0
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
		p.longBTC += sig.TargetSize // BUG 11: add to long leg
		p.longBTC = math.Max(0, p.longBTC)
		p.totalFeesUSD += fee
		log.Printf("[PAPER EXEC] %s BUY %.4f BTC @ $%.2f | Fee: $%.4f | Balance: $%.2f | Net=%.4f",
			mode, sig.TargetSize, execPrice, fee, p.balanceUSD, p.NetPosition())

	} else if sig.Action == strategy.ActionSell {
		// Revenue net of taker fee (fee paid on sell side too).
		netRevenue := notional - fee
		p.shortBTC += sig.TargetSize // BUG 11: add to short leg
		p.shortBTC = math.Max(0, p.shortBTC)
		p.balanceUSD += netRevenue
		p.totalFeesUSD += fee
		log.Printf("[PAPER EXEC] %s SELL %.4f BTC @ $%.2f | Fee: $%.4f | Balance: $%.2f | Net=%.4f",
			mode, sig.TargetSize, execPrice, fee, p.balanceUSD, p.NetPosition())
	}

	p.clampBalance("applyFill")
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
		return p.NetPosition() // BUG 11: return net signed position
	}
	return 0
}

func (p *PaperClient) GetBalanceUSD() float64 {
	return p.balanceUSD
}

// GetEquityUSD returns cash plus mark-to-market value of the net BTC position.
func (p *PaperClient) GetEquityUSD() float64 {
	if p.lastKnownPrice <= 0 {
		return p.balanceUSD
	}
	return p.balanceUSD + (p.NetPosition() * p.lastKnownPrice) // BUG 11
}

func (p *PaperClient) GetTotalFees() float64 {
	return p.totalFeesUSD
}

func (p *PaperClient) GetLastPrice() float64 {
	return p.lastKnownPrice
}

func (p *PaperClient) ResetAccount() error {
	p.longBTC = 0  // BUG 11
	p.shortBTC = 0 // BUG 11
	p.balanceUSD = p.initialUSD
	p.lastKnownPrice = 0
	return nil
}

// exitAdjustedPrice applies adverse slippage to an exit fill price.
// Exits are always treated as IOC (taker) orders in paper mode: LONG exits
// (selling BTC) fill slightly below market, SHORT exits (buying back) slightly above.
// 1.2 bps adverse slippage mirrors the entry model in executionPrice(OrderModeIOC).
func exitAdjustedPrice(side strategy.Action, price float64) float64 {
	const slippageFactor = 1.2 / 10000.0 // 1.2 bps
	if side == strategy.ActionBuy {
		// Closing LONG = selling BTC → adverse = lower fill price
		return price * (1 - slippageFactor)
	}
	// Closing SHORT = buying BTC back → adverse = higher fill price
	return price * (1 + slippageFactor)
}

// SettlePosition updates the paper balance when a position is closed or
// partially reduced. Applies both adverse exit slippage (1.2 bps IOC) and
// taker exit fee (0.05%) — matching the full round-trip cost model.
func (p *PaperClient) SettlePosition(side strategy.Action, size, exitPrice float64) {
	adjPrice := exitAdjustedPrice(side, exitPrice)
	exitNotional := size * adjPrice
	exitFee := exitNotional * BinanceFuturesTakerFeePct
	p.totalFeesUSD += exitFee

	if side == strategy.ActionBuy {
		// Closing a LONG position: sell BTC back at adjusted exit price, net of fee
		netRevenue := exitNotional - exitFee
		p.balanceUSD += netRevenue
		p.longBTC -= size // BUG 11: decrement long leg
		p.longBTC = math.Max(0, clampNearZero(p.longBTC))
		log.Printf("[PAPER SETTLE] CLOSE LONG: SELL %.4f BTC @ $%.2f (slip adj=%.2f) | fee=$%.4f | net=$%.2f | Balance=$%.2f | Net=%.4f",
			size, exitPrice, adjPrice, exitFee, netRevenue, p.balanceUSD, p.NetPosition())
	} else {
		// Closing a SHORT position: buy BTC back at adjusted exit price, plus fee
		totalCost := exitNotional + exitFee
		if totalCost > p.balanceUSD {
			log.Printf("[PAPER SETTLE] INSUFFICIENT FUNDS for short cover: needs $%.2f (incl fee $%.4f), has $%.2f — clamping balance to 0",
				totalCost, exitFee, p.balanceUSD)
			p.balanceUSD = 0
		} else {
			p.balanceUSD -= totalCost
		}
		p.shortBTC -= size // BUG 11: decrement short leg
		p.shortBTC = math.Max(0, clampNearZero(p.shortBTC))
		log.Printf("[PAPER SETTLE] CLOSE SHORT: BUY %.4f BTC @ $%.2f (slip adj=%.2f) | fee=$%.4f | cost=$%.2f | Balance=$%.2f | Net=%.4f",
			size, exitPrice, adjPrice, exitFee, totalCost, p.balanceUSD, p.NetPosition())
	}
	p.clampBalance("SettlePosition")
}

// RestoreBalance restores balance and accumulated fees from database on restart.
func (p *PaperClient) RestoreBalance(balance, fees float64) {
	if balance < 0 {
		log.Printf("[PAPER EXEC] Restored balance %.4f is negative — clamping to 0", balance)
		balance = 0
	}
	p.balanceUSD = balance
	p.totalFeesUSD = fees
	log.Printf("[PAPER EXEC] Restored balance: $%.2f | Cumulative fees: $%.4f", balance, fees)
}

// RestoreOpenPosition adds a recovered open position's BTC size back into the
// paper client's signed position so that mark-to-market equity and SettlePosition
// are correct after a restart. Call once per recovered open position before Run().
func (p *PaperClient) RestoreOpenPosition(side strategy.Action, sizeBTC float64) {
	if side == strategy.ActionBuy {
		p.longBTC += sizeBTC  // BUG 11
	} else {
		p.shortBTC += sizeBTC // BUG 11
	}
	p.longBTC = math.Max(0, clampNearZero(p.longBTC))
	p.shortBTC = math.Max(0, clampNearZero(p.shortBTC))
	log.Printf("[PAPER EXEC] Restored open position: side=%s size=%.4f BTC | net=%.4f gross=%.4f",
		side, sizeBTC, p.NetPosition(), p.GrossExposure())
}

// PERF 7 — Partial Fill Simulation.
// estimateFillFraction returns the fraction of an order that can be filled given
// available market depth. Returns 1.0 if depthUSD <= 0 (assume full liquidity).
func estimateFillFraction(notionalUSD, depthUSD float64) float64 {
	if depthUSD <= 0 {
		return 1.0
	}
	if notionalUSD <= 0 {
		return 1.0
	}
	if notionalUSD <= depthUSD {
		return 1.0
	}
	return depthUSD / notionalUSD
}

// ApplyPartialFillIfLargeOrder adjusts sig.TargetSize for very large orders.
// Returns the adjusted signal and any error (too thin a market = reject).
func (p *PaperClient) ApplyPartialFillIfLargeOrder(sig strategy.Signal, currentPrice float64) (strategy.Signal, error) {
	const largeOrderThresholdUSD = 500_000.0
	const assumedDepthUSD = 500_000.0
	notional := sig.TargetSize * currentPrice
	if notional <= largeOrderThresholdUSD {
		return sig, nil
	}
	fraction := estimateFillFraction(notional, assumedDepthUSD)
	if fraction < 0.5 {
		return sig, fmt.Errorf("partial fill %.0f%% below 50%% threshold — order rejected (notional $%.0f > depth $%.0f)",
			fraction*100, notional, assumedDepthUSD)
	}
	sig.TargetSize = sig.TargetSize * fraction
	log.Printf("[PAPER EXEC] Partial fill applied: %.0f%% of %.4f BTC (notional $%.0f)",
		fraction*100, sig.TargetSize, notional)
	return sig, nil
}

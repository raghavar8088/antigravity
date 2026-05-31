package v2

import (
	"fmt"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/strategy"
)

type Portfolio struct {
	CashUSD   float64
	Positions map[string]Position
	Trades    []Trade
	seq       int
}

func NewPortfolio(initialUSD float64) *Portfolio {
	return &Portfolio{CashUSD: initialUSD, Positions: make(map[string]Position)}
}

func (p *Portfolio) Open(strategyName string, sig strategy.Signal, fill btExecution.Fill, regime Regime) Position {
	p.seq++
	side := sig.Action
	entry := fill.FillPrice
	stop := entry * (1 - sig.StopLossPct/100)
	tp := entry * (1 + sig.TakeProfitPct/100)
	if side == strategy.ActionSell {
		stop = entry * (1 + sig.StopLossPct/100)
		tp = entry * (1 - sig.TakeProfitPct/100)
	}
	pos := Position{
		ID:           fmt.Sprintf("BT2-POS-%d", p.seq),
		StrategyName: strategyName,
		Symbol:       sig.Symbol,
		Side:         side,
		EntryPrice:   entry,
		Size:         fill.FilledQuantity,
		StopLoss:     stop,
		TakeProfit:   tp,
		OpenedAt:     fill.Timestamp,
		Regime:       regime,
		EntryFill:    fill,
	}
	p.Positions[pos.ID] = pos
	return pos
}

func (p *Portfolio) Close(pos Position, exitFill btExecution.Fill, funding btExecution.FundingAttribution, feeBps float64, reason string) Trade {
	delete(p.Positions, pos.ID)
	gross := 0.0
	if pos.Side == strategy.ActionBuy {
		gross = (exitFill.FillPrice - pos.EntryPrice) * pos.Size
	} else {
		gross = (pos.EntryPrice - exitFill.FillPrice) * pos.Size
	}
	notionalIn := pos.EntryPrice * pos.Size
	notionalOut := exitFill.FillPrice * pos.Size
	fees := (notionalIn + notionalOut) * feeBps / 10_000
	net := gross - fees - pos.EntryFill.SpreadCostUSD - pos.EntryFill.SlippageCostUSD - pos.EntryFill.ImpactCostUSD - exitFill.SpreadCostUSD - exitFill.SlippageCostUSD - exitFill.ImpactCostUSD + funding.FundingPnLUSD
	p.CashUSD += net
	tr := Trade{
		ID:               pos.ID,
		StrategyName:     pos.StrategyName,
		Symbol:           pos.Symbol,
		Side:             pos.Side,
		EntryPrice:       pos.EntryPrice,
		ExitPrice:        exitFill.FillPrice,
		Size:             pos.Size,
		GrossPnL:         gross,
		Fees:             fees,
		SpreadCost:       pos.EntryFill.SpreadCostUSD + exitFill.SpreadCostUSD,
		SlippageCost:     pos.EntryFill.SlippageCostUSD + exitFill.SlippageCostUSD,
		ImpactCost:       pos.EntryFill.ImpactCostUSD + exitFill.ImpactCostUSD,
		FundingPnL:       funding.FundingPnLUSD,
		NetPnL:           net,
		EntryTime:        pos.OpenedAt,
		ExitTime:         exitFill.Timestamp,
		Regime:           pos.Regime,
		ExecutionQuality: exitFill,
	}
	p.Trades = append(p.Trades, tr)
	return tr
}

func (p *Portfolio) Equity(mark float64) float64 {
	equity := p.CashUSD
	for _, pos := range p.Positions {
		if pos.Side == strategy.ActionBuy {
			equity += (mark - pos.EntryPrice) * pos.Size
		} else {
			equity += (pos.EntryPrice - mark) * pos.Size
		}
	}
	return equity
}

func shouldClose(pos Position, price float64, now time.Time) bool {
	if pos.Side == strategy.ActionBuy {
		return price <= pos.StopLoss || price >= pos.TakeProfit || now.Sub(pos.OpenedAt) >= 45*time.Minute
	}
	return price >= pos.StopLoss || price <= pos.TakeProfit || now.Sub(pos.OpenedAt) >= 45*time.Minute
}

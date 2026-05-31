package v2

import (
	btExecution "antigravity-engine/internal/backtest/execution"
	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type ExecutionSimulator struct {
	model   btExecution.FillModel
	funding btExecution.FundingModel
}

func NewExecutionSimulator(funding btExecution.FundingModel) ExecutionSimulator {
	return ExecutionSimulator{model: btExecution.NewFillModel(), funding: funding}
}

func (s ExecutionSimulator) FillEntry(sig strategy.Signal, tick marketdata.Tick, cfg Config, id string) btExecution.Fill {
	side := btExecution.SideBuy
	if sig.Action == strategy.ActionSell {
		side = btExecution.SideSell
	}
	orderType := btExecution.OrderMarket
	if cfg.Mode == ModeInstitutional {
		orderType = btExecution.OrderLimit
	}
	order := btExecution.Order{
		ID:          id,
		Symbol:      sig.Symbol,
		Side:        side,
		Type:        orderType,
		Quantity:    sig.TargetSize,
		SignalPrice: tick.Price,
		LimitPrice:  tick.Price,
		GeneratedAt: tickTime(tick),
	}
	return s.model.Execute(order, marketContextFromTick(tick, cfg))
}

func (s ExecutionSimulator) FillExit(pos Position, tick marketdata.Tick, cfg Config) btExecution.Fill {
	side := btExecution.SideSell
	if pos.Side == strategy.ActionSell {
		side = btExecution.SideBuy
	}
	order := btExecution.Order{
		ID:          pos.ID + "-EXIT",
		Symbol:      pos.Symbol,
		Side:        side,
		Type:        btExecution.OrderMarket,
		Quantity:    pos.Size,
		SignalPrice: tick.Price,
		GeneratedAt: tickTime(tick),
	}
	return s.model.Execute(order, marketContextFromTick(tick, cfg))
}

func (s ExecutionSimulator) Funding(pos Position, exitTick marketdata.Tick) btExecution.FundingAttribution {
	side := btExecution.SideBuy
	if pos.Side == strategy.ActionSell {
		side = btExecution.SideSell
	}
	return s.funding.Apply(side, pos.EntryPrice*pos.Size, pos.OpenedAt, tickTime(exitTick))
}

func marketContextFromTick(tick marketdata.Tick, cfg Config) btExecution.MarketContext {
	liq := 0.65
	if tick.Quantity > 0 {
		liq = btExecutionLiquidityScore(tick.Quantity)
	}
	vol := 0.20
	if cfg.Mode == ModeFast {
		vol = 0.05
	}
	return btExecution.MarketContext{
		MidPrice:       tick.Price,
		VolatilityPct:  vol,
		LiquidityScore: liq,
		ADVNotionalUSD: tick.Price * 5000,
		BookDepthUSD:   tick.Price * 250,
		MomentumPct:    0.02,
		Timestamp:      tickTime(tick),
		Latency: btExecution.LatencyInput{
			Tier:                 cfg.LatencyTier,
			SignalEdgeBps:        8,
			MomentumPctPerSecond: 0.01,
		},
	}
}

func btExecutionLiquidityScore(qty float64) float64 {
	if qty <= 0 {
		return 0.5
	}
	if qty >= 10 {
		return 1
	}
	return 0.35 + qty/10*0.65
}

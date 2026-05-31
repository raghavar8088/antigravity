package v2

import "time"

type Position struct {
	ID            string
	Symbol        string
	Strategy      string
	Family        StrategyFamily
	Regime        MarketRegime
	SignalType    string
	Side          Side
	EntryPrice    float64
	MarkPrice     float64
	SizeBTC       float64
	StopLossPrice float64
	Leverage      float64
	OpenedAt      time.Time
}

type Portfolio struct {
	Account         AccountState
	Positions       map[string]Position
	Returns         []float64
	StrategyMetrics map[string]StrategyMetrics
	Allocations     map[string]float64
}

func NewPortfolio(equityUSD float64) Portfolio {
	account := normalizeAccount(AccountState{EquityUSD: equityUSD, HighWatermarkUSD: equityUSD})
	return Portfolio{
		Account:         account,
		Positions:       make(map[string]Position),
		StrategyMetrics: make(map[string]StrategyMetrics),
		Allocations:     make(map[string]float64),
	}
}

func (p *Portfolio) AddPosition(pos Position) {
	if pos.ID == "" {
		pos.ID = pos.Strategy + "-" + pos.Symbol + "-" + pos.OpenedAt.UTC().Format("20060102150405")
	}
	if pos.OpenedAt.IsZero() {
		pos.OpenedAt = time.Now().UTC()
	}
	p.Positions[pos.ID] = pos
}

func (p Portfolio) Clone() Portfolio {
	out := Portfolio{
		Account:         p.Account,
		Positions:       make(map[string]Position, len(p.Positions)),
		Returns:         append([]float64(nil), p.Returns...),
		StrategyMetrics: make(map[string]StrategyMetrics, len(p.StrategyMetrics)),
		Allocations:     make(map[string]float64, len(p.Allocations)),
	}
	for k, v := range p.Positions {
		out.Positions[k] = v
	}
	for k, v := range p.StrategyMetrics {
		v.Returns = append([]float64(nil), v.Returns...)
		out.StrategyMetrics[k] = v
	}
	for k, v := range p.Allocations {
		out.Allocations[k] = v
	}
	return out
}

func PositionNotionalUSD(pos Position) float64 {
	price := pos.MarkPrice
	if price <= 0 {
		price = pos.EntryPrice
	}
	return abs(price * pos.SizeBTC)
}

func PositionRiskUSD(req TradeRequest, sizeBTC float64) float64 {
	if req.EntryPrice <= 0 || sizeBTC <= 0 {
		return 0
	}
	stop := req.StopLossPrice
	if stop <= 0 {
		stop = req.EntryPrice * 0.99
		if req.Side == SideShort {
			stop = req.EntryPrice * 1.01
		}
	}
	return abs(req.EntryPrice-stop) * sizeBTC
}

func PositionRiskFromPosition(pos Position) float64 {
	req := TradeRequest{EntryPrice: pos.EntryPrice, StopLossPrice: pos.StopLossPrice, Side: pos.Side}
	return PositionRiskUSD(req, pos.SizeBTC)
}

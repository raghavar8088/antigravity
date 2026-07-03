package execution

import (
	"math"
	"time"
)

type OrderSide string
type OrderType string

const (
	SideBuy  OrderSide = "BUY"
	SideSell OrderSide = "SELL"

	OrderMarket   OrderType = "MARKET"
	OrderLimit    OrderType = "LIMIT"
	OrderPostOnly OrderType = "POST_ONLY"
)

type Order struct {
	ID          string
	Symbol      string
	Side        OrderSide
	Type        OrderType
	Quantity    float64
	SignalPrice float64
	LimitPrice  float64
	ReduceOnly  bool
	PostOnly    bool
	GeneratedAt time.Time
}

type Fill struct {
	OrderID           string
	Symbol            string
	Side              OrderSide
	RequestedQuantity float64
	FilledQuantity    float64
	CancelledQuantity float64
	FillRatio         float64
	ExpectedFill      float64
	ActualFill        float64
	FillEfficiency    float64
	FillPrice         float64
	SpreadCostUSD     float64
	SlippageCostUSD   float64
	ImpactCostUSD     float64
	Latency           LatencyMetrics
	Timestamp         time.Time
}

type FillModel struct {
	Spread   SpreadModel
	Slippage SlippageEngine
	Impact   ImpactModel
	Latency  LatencyEngine
}

func NewFillModel() FillModel {
	return NewFillModelWithSpread(1.5)
}

func NewFillModelWithSpread(baseBps float64) FillModel {
	if baseBps <= 0 {
		baseBps = 1.5
	}
	return FillModel{Spread: NewSpreadModel(baseBps), Slippage: NewSlippageEngine(), Impact: ImpactModel{}, Latency: LatencyEngine{}}
}

func (m FillModel) Execute(order Order, ctx MarketContext) Fill {
	spread := m.Spread.Snapshot(ctx.MidPrice, ctx.VolatilityPct, ctx.LiquidityScore, ctx.Timestamp)
	lat := m.Latency.Simulate(order.GeneratedAt, ctx.Latency)
	notional := order.Quantity * ctx.MidPrice
	slippage := m.Slippage.Estimate(SlippageInput{OrderNotionalUSD: notional, ADVNotionalUSD: ctx.ADVNotionalUSD, VolatilityPct: ctx.VolatilityPct, LiquidityScore: ctx.LiquidityScore, MomentumPct: ctx.MomentumPct})
	impact := m.Impact.Estimate(ImpactInput{OrderNotionalUSD: notional, BookDepthUSD: ctx.BookDepthUSD, LiquidityScore: ctx.LiquidityScore, VolatilityPct: ctx.VolatilityPct})
	fillRatio := FillRatio(order, ctx)
	filledQty := order.Quantity * fillRatio
	price := spread.Ask
	if order.Side == SideSell {
		price = spread.Bid
	}
	adverseBps := slippage.ExpectedBps + impact.TemporaryBps + impact.PermanentBps + lat.SignalDecayBps
	if order.Side == SideBuy {
		price *= 1 + adverseBps/10_000
	} else {
		price *= 1 - adverseBps/10_000
	}
	spreadCost := math.Abs(spread.Mid-price) * filledQty
	return Fill{
		OrderID:           order.ID,
		Symbol:            order.Symbol,
		Side:              order.Side,
		RequestedQuantity: order.Quantity,
		FilledQuantity:    filledQty,
		CancelledQuantity: order.Quantity - filledQty,
		FillRatio:         fillRatio,
		ExpectedFill:      order.Quantity,
		ActualFill:        filledQty,
		FillEfficiency:    fillRatio * impact.ExecutionEfficiency,
		FillPrice:         price,
		SpreadCostUSD:     spreadCost,
		SlippageCostUSD:   slippage.ExpectedUSD * fillRatio,
		ImpactCostUSD:     impact.ImpactCostUSD * fillRatio,
		Latency:           lat,
		Timestamp:         lat.FilledAt,
	}
}

type MarketContext struct {
	MidPrice       float64
	VolatilityPct  float64
	LiquidityScore float64
	ADVNotionalUSD float64
	BookDepthUSD   float64
	MomentumPct    float64
	Timestamp      time.Time
	Latency        LatencyInput
}

func FillRatio(order Order, ctx MarketContext) float64 {
	if order.Type == OrderMarket {
		return clamp(ctx.LiquidityScore+0.25, 0.25, 1)
	}
	if order.Type == OrderPostOnly || order.PostOnly {
		if ctx.MomentumPct > 0 && order.Side == SideBuy {
			return 0.50
		}
		if ctx.MomentumPct < 0 && order.Side == SideSell {
			return 0.50
		}
		return 0.75
	}
	if order.Type == OrderLimit {
		if order.Side == SideBuy && order.LimitPrice >= ctx.MidPrice {
			return 1
		}
		if order.Side == SideSell && order.LimitPrice <= ctx.MidPrice {
			return 1
		}
		return 0.25
	}
	return 1
}

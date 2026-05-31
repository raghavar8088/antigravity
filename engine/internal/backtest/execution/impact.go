package execution

import "math"

type ImpactInput struct {
	OrderNotionalUSD float64
	BookDepthUSD     float64
	LiquidityScore   float64
	VolatilityPct    float64
}

type ImpactResult struct {
	TemporaryBps        float64
	PermanentBps        float64
	ImpactCostUSD       float64
	ExecutionEfficiency float64
}

type ImpactModel struct{}

func (ImpactModel) Estimate(input ImpactInput) ImpactResult {
	depth := input.BookDepthUSD
	if depth <= 0 {
		depth = math.Max(input.OrderNotionalUSD*50, 1)
	}
	liq := clamp(input.LiquidityScore, 0.05, 1)
	participation := input.OrderNotionalUSD / depth
	temp := math.Sqrt(participation) * 25 / liq
	perm := participation * 8 * (1 + input.VolatilityPct/100)
	total := temp + perm
	eff := clamp(1-total/100, 0, 1)
	return ImpactResult{
		TemporaryBps:        temp,
		PermanentBps:        perm,
		ImpactCostUSD:       input.OrderNotionalUSD * total / 10_000,
		ExecutionEfficiency: eff,
	}
}

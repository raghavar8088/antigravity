package execution

import "math"

type SlippageInput struct {
	BaseBps          float64
	VolatilityPct    float64
	LiquidityScore   float64
	OrderNotionalUSD float64
	ADVNotionalUSD   float64
	MomentumPct      float64
}

type SlippageResult struct {
	ExpectedBps       float64
	ExpectedUSD       float64
	SlippageBudgetBps float64
	SlippageDriftBps  float64
}

type SlippageEngine struct {
	DefaultBaseBps float64
	MaxBudgetBps   float64
}

func NewSlippageEngine() SlippageEngine {
	return SlippageEngine{DefaultBaseBps: 1.0, MaxBudgetBps: 25}
}

func (e SlippageEngine) Estimate(input SlippageInput) SlippageResult {
	base := input.BaseBps
	if base <= 0 {
		base = e.DefaultBaseBps
	}
	liq := input.LiquidityScore
	if liq <= 0 {
		liq = 0.5
	}
	adv := input.ADVNotionalUSD
	if adv <= 0 {
		adv = math.Max(input.OrderNotionalUSD*100, 1)
	}
	volFactor := math.Max(0, input.VolatilityPct) * 2.2
	sizeFactor := math.Sqrt(math.Max(input.OrderNotionalUSD/adv, 0)) * 35
	liquidityFactor := (1 - clamp(liq, 0.05, 1)) * 10
	momentumFactor := math.Abs(input.MomentumPct) * 1.5
	bps := base + volFactor + sizeFactor + liquidityFactor + momentumFactor
	budget := e.MaxBudgetBps
	if budget <= 0 {
		budget = 25
	}
	return SlippageResult{
		ExpectedBps:       bps,
		ExpectedUSD:       input.OrderNotionalUSD * bps / 10_000,
		SlippageBudgetBps: budget,
		SlippageDriftBps:  bps - budget,
	}
}

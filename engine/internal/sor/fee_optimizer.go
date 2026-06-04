package sor

// CostBreakdown is the full cost decomposition for executing an order on a venue.
// All values are in USD unless suffixed Bps.
type CostBreakdown struct {
	VenueID VenueID

	NotionalUSD float64

	// Per-component costs (USD)
	ExecutionCostUSD float64 // exchange maker/taker fee
	SpreadCostUSD    float64 // half-spread cost crossing the book
	SlippageCostUSD  float64 // expected slippage cost
	FundingCostUSD   float64 // perpetual funding over the holding period
	HoldingCostUSD   float64 // financing/borrow cost over the holding period

	// Per-component costs (bps of notional)
	ExecutionCostBps float64
	SpreadCostBps    float64
	SlippageCostBps  float64
	FundingCostBps   float64

	// Totals
	TotalCostUSD float64
	TotalCostBps float64

	IsMaker bool // whether the order is expected to be a maker (post-only/limit passive)
}

// FeeOptimizer computes total execution cost per venue and ranks by lowest cost.
type FeeOptimizer struct {
	// DefaultHoldingHours is used to annualise funding/holding costs when the
	// caller does not specify a holding period.
	DefaultHoldingHours float64
	// AnnualBorrowRateBps is the financing cost for leveraged/borrowed notional.
	AnnualBorrowRateBps float64
}

// NewFeeOptimizer returns a fee optimizer with institutional defaults.
func NewFeeOptimizer() *FeeOptimizer {
	return &FeeOptimizer{
		DefaultHoldingHours: 8.0,   // typical intraday hold
		AnnualBorrowRateBps: 500.0, // 5% annual financing
	}
}

// CostInput holds the parameters required to compute a venue cost.
type CostInput struct {
	VenueID        VenueID
	NotionalUSD    float64
	IsMaker        bool
	Fees           FeeStructure
	SpreadBps      float64
	ExpSlippageBps float64
	FundingBps     float64 // funding rate per 8h interval (sign matters: + pays longs)
	Side           string
	HoldingHours   float64 // 0 → DefaultHoldingHours
}

// Compute calculates the full cost breakdown for one venue.
func (o *FeeOptimizer) Compute(in CostInput) CostBreakdown {
	cb := CostBreakdown{VenueID: in.VenueID, NotionalUSD: in.NotionalUSD, IsMaker: in.IsMaker}
	if in.NotionalUSD <= 0 {
		return cb
	}

	// Execution (exchange) fee.
	feeBps := in.Fees.TakerBps
	if in.IsMaker {
		feeBps = in.Fees.MakerBps
	}
	cb.ExecutionCostBps = feeBps
	cb.ExecutionCostUSD = in.NotionalUSD * feeBps / 10000

	// Spread cost — a taker crosses half the spread; a maker pays ~0 (earns it).
	if !in.IsMaker {
		cb.SpreadCostBps = in.SpreadBps / 2
		cb.SpreadCostUSD = in.NotionalUSD * cb.SpreadCostBps / 10000
	}

	// Slippage cost.
	cb.SlippageCostBps = in.ExpSlippageBps
	cb.SlippageCostUSD = in.NotionalUSD * in.ExpSlippageBps / 10000

	// Funding cost over the holding period (perpetuals settle every 8h).
	holdingHours := in.HoldingHours
	if holdingHours <= 0 {
		holdingHours = o.DefaultHoldingHours
	}
	fundingIntervals := holdingHours / 8.0
	// A long pays funding when funding rate is positive; a short receives it.
	directional := in.FundingBps * fundingIntervals
	if !isBuy(in.Side) {
		directional = -directional
	}
	cb.FundingCostBps = directional
	cb.FundingCostUSD = in.NotionalUSD * directional / 10000

	// Holding/financing cost (always a cost, proportional to time).
	years := holdingHours / (24 * 365)
	cb.HoldingCostUSD = in.NotionalUSD * (o.AnnualBorrowRateBps / 10000) * years

	cb.TotalCostUSD = cb.ExecutionCostUSD + cb.SpreadCostUSD + cb.SlippageCostUSD +
		cb.FundingCostUSD + cb.HoldingCostUSD
	if in.NotionalUSD > 0 {
		cb.TotalCostBps = cb.TotalCostUSD / in.NotionalUSD * 10000
	}
	return cb
}

// CheapestVenue returns the venue ID with the lowest total cost from a set of breakdowns.
func (o *FeeOptimizer) CheapestVenue(breakdowns []CostBreakdown) (VenueID, CostBreakdown, bool) {
	if len(breakdowns) == 0 {
		return "", CostBreakdown{}, false
	}
	best := breakdowns[0]
	for _, cb := range breakdowns[1:] {
		if cb.TotalCostUSD < best.TotalCostUSD {
			best = cb
		}
	}
	return best.VenueID, best, true
}

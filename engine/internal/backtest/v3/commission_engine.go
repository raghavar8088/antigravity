package v3

// CommissionSchedule holds the maker/taker fee rates for one tier.
type CommissionSchedule struct {
	MakerBps float64 // negative = rebate
	TakerBps float64
}

// ExchangeFeeTable maps volume tiers to commission schedules.
type ExchangeFeeTable struct {
	Exchange Exchange
	Tiers    map[VolumeTier]CommissionSchedule
}

var feeSchedules = map[Exchange]ExchangeFeeTable{
	ExchangeBinance: {
		Exchange: ExchangeBinance,
		Tiers: map[VolumeTier]CommissionSchedule{
			TierStandard: {MakerBps: 2.0, TakerBps: 4.0},   // 0.02% / 0.04%
			TierTier1:    {MakerBps: 1.6, TakerBps: 4.0},   // >$1M 30d
			TierTier2:    {MakerBps: 1.4, TakerBps: 3.5},   // >$5M
			TierTier3:    {MakerBps: 1.2, TakerBps: 3.2},   // >$25M
			TierVIP:      {MakerBps: 0.8, TakerBps: 2.0},   // >$100M
		},
	},
	ExchangeBybit: {
		Exchange: ExchangeBybit,
		Tiers: map[VolumeTier]CommissionSchedule{
			TierStandard: {MakerBps: -1.0, TakerBps: 6.0},  // -0.01% rebate, 0.06% taker
			TierTier1:    {MakerBps: -1.0, TakerBps: 5.5},
			TierTier2:    {MakerBps: -1.0, TakerBps: 5.0},
			TierTier3:    {MakerBps: -1.0, TakerBps: 4.5},
			TierVIP:      {MakerBps: -1.0, TakerBps: 3.0},
		},
	},
	ExchangeOKX: {
		Exchange: ExchangeOKX,
		Tiers: map[VolumeTier]CommissionSchedule{
			TierStandard: {MakerBps: 2.0, TakerBps: 5.0},
			TierTier1:    {MakerBps: 1.5, TakerBps: 4.5},
			TierTier2:    {MakerBps: 1.0, TakerBps: 4.0},
			TierTier3:    {MakerBps: 0.8, TakerBps: 3.5},
			TierVIP:      {MakerBps: 0.5, TakerBps: 2.5},
		},
	},
	ExchangeDelta: {
		Exchange: ExchangeDelta,
		Tiers: map[VolumeTier]CommissionSchedule{
			TierStandard: {MakerBps: 2.0, TakerBps: 5.0},
			TierTier1:    {MakerBps: 1.5, TakerBps: 4.5},
			TierTier2:    {MakerBps: 1.0, TakerBps: 4.0},
			TierTier3:    {MakerBps: 0.8, TakerBps: 3.5},
			TierVIP:      {MakerBps: 0.5, TakerBps: 2.5},
		},
	},
}

// CommissionEngine computes exchange-specific trading fees.
type CommissionEngine struct {
	schedule CommissionSchedule
	exchange Exchange
	tier     VolumeTier
}

// NewCommissionEngine constructs an engine for the given exchange and volume tier.
func NewCommissionEngine(exchange Exchange, tier VolumeTier) *CommissionEngine {
	table, ok := feeSchedules[exchange]
	if !ok {
		table = feeSchedules[ExchangeBinance]
	}
	schedule, ok := table.Tiers[tier]
	if !ok {
		schedule = table.Tiers[TierStandard]
	}
	return &CommissionEngine{schedule: schedule, exchange: exchange, tier: tier}
}

// CommissionResult is the per-trade fee calculation.
type CommissionResult struct {
	Exchange       Exchange
	Tier           VolumeTier
	Class          MakerTakerClass
	FeeBps         float64
	FeeUSD         float64
	IsRebate       bool
	RoundTripBps   float64
	RoundTripUSD   float64
}

// Calculate returns the fee for a single leg (entry or exit).
func (c *CommissionEngine) Calculate(notionalUSD float64, class MakerTakerClass) CommissionResult {
	var feeBps float64
	if class == ClassMaker {
		feeBps = c.schedule.MakerBps
	} else {
		feeBps = c.schedule.TakerBps
	}
	feeUSD := notionalUSD * feeBps / 10_000
	return CommissionResult{
		Exchange:  c.exchange,
		Tier:      c.tier,
		Class:     class,
		FeeBps:    feeBps,
		FeeUSD:    feeUSD,
		IsRebate:  feeBps < 0,
	}
}

// RoundTrip computes combined entry + exit fees for a complete trade.
func (c *CommissionEngine) RoundTrip(notionalUSD float64, entryClass, exitClass MakerTakerClass) CommissionResult {
	entry := c.Calculate(notionalUSD, entryClass)
	exit := c.Calculate(notionalUSD, exitClass)
	totalUSD := entry.FeeUSD + exit.FeeUSD
	totalBps := entry.FeeBps + exit.FeeBps
	return CommissionResult{
		Exchange:     c.exchange,
		Tier:         c.tier,
		Class:        entryClass, // entry classification dominates
		FeeBps:       entry.FeeBps,
		FeeUSD:       entry.FeeUSD,
		IsRebate:     entry.IsRebate,
		RoundTripBps: totalBps,
		RoundTripUSD: totalUSD,
	}
}

// Schedule exposes the underlying schedule for reporting.
func (c *CommissionEngine) Schedule() CommissionSchedule {
	return c.schedule
}

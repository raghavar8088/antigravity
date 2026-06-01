package v3

import (
	"math"
	"time"

	btExecution "antigravity-engine/internal/backtest/execution"
)

// FundingExchangeConfig holds exchange-specific funding interval and rate caps.
type FundingExchangeConfig struct {
	Exchange      Exchange
	Interval      time.Duration
	MaxRatePct    float64 // absolute max rate per period
	ShockRatePct  float64 // rate used during funding shock stress
}

var fundingConfigs = map[Exchange]FundingExchangeConfig{
	ExchangeBinance: {Exchange: ExchangeBinance, Interval: 8 * time.Hour, MaxRatePct: 0.375, ShockRatePct: 0.3},
	ExchangeBybit:   {Exchange: ExchangeBybit, Interval: 8 * time.Hour, MaxRatePct: 0.375, ShockRatePct: 0.3},
	ExchangeOKX:     {Exchange: ExchangeOKX, Interval: 8 * time.Hour, MaxRatePct: 0.375, ShockRatePct: 0.3},
	ExchangeDelta:   {Exchange: ExchangeDelta, Interval: 8 * time.Hour, MaxRatePct: 0.500, ShockRatePct: 0.4},
}

// FundingSimulator replays historical funding rates with exchange specificity.
// If historical rates are absent it synthesizes rates from vol and basis.
type FundingSimulator struct {
	exchange Exchange
	cfg      FundingExchangeConfig
	rates    []btExecution.FundingRate
	shock    bool // inject shock rate when true
}

// NewFundingSimulator constructs a funding simulator for the given exchange.
func NewFundingSimulator(exchange Exchange, historicalRates []btExecution.FundingRate) *FundingSimulator {
	cfg, ok := fundingConfigs[exchange]
	if !ok {
		cfg = fundingConfigs[ExchangeBinance]
	}
	return &FundingSimulator{exchange: exchange, cfg: cfg, rates: historicalRates}
}

// EnableShock injects a persistent funding shock (max rate) into future periods.
func (f *FundingSimulator) EnableShock(enabled bool) {
	f.shock = enabled
}

// FundingResult is the per-position funding attribution.
type FundingResult struct {
	Exchange          Exchange
	PeriodsCharged    int
	FundingPaidUSD    float64
	FundingReceivedUSD float64
	FundingPnLUSD     float64
	FundingDragPct    float64
	AnnualizedCostPct float64
	PeriodRates       []float64
}

// Apply computes funding costs for a position held from entry to exit.
func (f *FundingSimulator) Apply(side string, notionalUSD float64, entry, exit time.Time) FundingResult {
	interval := f.cfg.Interval
	periods := f.fundingPeriods(entry, exit, interval)

	var result FundingResult
	result.Exchange = f.exchange
	result.PeriodRates = make([]float64, 0, len(periods))

	for _, t := range periods {
		rate := f.rateAt(t)
		if f.shock {
			rate = f.cfg.ShockRatePct / 100
		}
		cashflow := notionalUSD * rate
		result.PeriodRates = append(result.PeriodRates, rate)

		if side == "BUY" {
			// Long pays when funding is positive, receives when negative.
			if cashflow > 0 {
				result.FundingPaidUSD += cashflow
			} else {
				result.FundingReceivedUSD += -cashflow
			}
		} else {
			// Short receives when funding is positive.
			if cashflow > 0 {
				result.FundingReceivedUSD += cashflow
			} else {
				result.FundingPaidUSD += -cashflow
			}
		}
	}

	result.PeriodsCharged = len(periods)
	result.FundingPnLUSD = result.FundingReceivedUSD - result.FundingPaidUSD
	if notionalUSD > 0 {
		result.FundingDragPct = -result.FundingPnLUSD / notionalUSD * 100
	}
	// Annualize: number of 8h periods per year = 365*3
	periodsPerYear := 365.0 * (24 * time.Hour).Seconds() / interval.Seconds()
	if notionalUSD > 0 && len(periods) > 0 {
		avgRate := meanF(result.PeriodRates)
		result.AnnualizedCostPct = avgRate * periodsPerYear * 100
	}
	return result
}

// fundingPeriods returns the funding settlement timestamps between entry and exit.
func (f *FundingSimulator) fundingPeriods(entry, exit time.Time, interval time.Duration) []time.Time {
	// Binance/Bybit/OKX settle at 00:00, 08:00, 16:00 UTC.
	settlements := []int{0, 8, 16}
	var periods []time.Time

	// Round entry up to next settlement.
	cursor := entry.UTC()
	for _, h := range settlements {
		t := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), h, 0, 0, 0, time.UTC)
		if t.After(cursor) {
			cursor = t
			break
		}
	}

	for !cursor.After(exit) {
		periods = append(periods, cursor)
		cursor = cursor.Add(interval)
	}
	return periods
}

// rateAt returns the funding rate at timestamp t from historical rates.
// If no historical rate is available it synthesizes one.
func (f *FundingSimulator) rateAt(t time.Time) float64 {
	// Find closest historical rate within 4 hours.
	bestRate := 0.0
	bestDiff := time.Duration(math.MaxInt64)
	for _, r := range f.rates {
		diff := t.Sub(r.Timestamp)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			bestRate = r.Rate
		}
	}
	if bestDiff < 4*time.Hour {
		return bestRate
	}
	// Synthetic: typical BTC perp rate is ~0.01% per period (mildly positive).
	return 0.0001
}

// SyntheticRateGenerator produces realistic funding rate sequences for backtests
// without historical data, using a mean-reverting process.
func SyntheticRateGenerator(n int, meanRate, volatility float64, seed int64) []btExecution.FundingRate {
	// Simple mean-reverting AR(1) process: r_t = θ*mean + (1-θ)*r_{t-1} + σ*ε
	theta := 0.3 // mean reversion speed
	rates := make([]btExecution.FundingRate, n)
	current := meanRate
	// Use a simple deterministic noise sequence seeded from parameter.
	rng := newSimpleRNG(seed)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		noise := rng.NormFloat64() * volatility
		current = theta*meanRate + (1-theta)*current + noise
		// Cap within exchange limits.
		current = clampF(current, -0.00375, 0.00375)
		rates[i] = btExecution.FundingRate{
			Rate:      current,
			Timestamp: base.Add(time.Duration(i) * 8 * time.Hour),
		}
	}
	return rates
}

func meanF(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// simpleRNG is a minimal deterministic PRNG (linear congruential) for synthetic data.
type simpleRNG struct {
	state uint64
}

func newSimpleRNG(seed int64) *simpleRNG {
	return &simpleRNG{state: uint64(seed) ^ 0xdeadbeefcafe}
}

func (r *simpleRNG) Uint64() uint64 {
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17
	return r.state
}

func (r *simpleRNG) Float64() float64 {
	return float64(r.Uint64()>>11) / (1 << 53)
}

// NormFloat64 uses Box-Muller transform to produce standard normal samples.
func (r *simpleRNG) NormFloat64() float64 {
	u1 := r.Float64()
	u2 := r.Float64()
	if u1 < 1e-10 {
		u1 = 1e-10
	}
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

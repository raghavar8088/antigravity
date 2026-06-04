package phase22e

import (
	"sort"
	"time"
)

// Validator runs the full Phase 22E profitability validation pipeline.
type Validator struct {
	TotalCapital float64 // USD capital available for deployment
}

// NewValidator creates a Validator with a given deployable capital amount.
func NewValidator(totalCapital float64) *Validator {
	return &Validator{TotalCapital: totalCapital}
}

// Run executes the complete validation pipeline and returns a ValidationResult.
//
// Pipeline:
//   1. Sort trades by exit time.
//   2. Assign market regimes.
//   3. Compute portfolio-level metrics.
//   4. Compute per-strategy metrics.
//   5. Rank strategies and assign capital.
//   6. Issue certification.
func (v *Validator) Run(trades []TradeRecord) ValidationResult {
	if len(trades) == 0 {
		result := ValidationResult{GeneratedAt: time.Now().UTC()}
		result.Status = StatusNotApproved
		result.Failed = []string{"no trades provided"}
		return result
	}

	// sort chronologically
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].ExitTime.Before(trades[j].ExitTime)
	})

	// assign regimes
	trades = AssignRegimes(trades)

	// portfolio metrics — pass initial NAV so drawdown is % of capital, not % of peak P&L
	pm := computePortfolioMetrics(trades, v.TotalCapital)

	// per-strategy metrics — allocate a typical budget for drawdown normalisation
	// Use totalCapital / number of strategies as the per-strategy starting NAV.
	stratMap := groupByStrategy(trades)
	stratCount := float64(len(stratMap))
	if stratCount == 0 {
		stratCount = 1
	}
	perStratNAV := v.TotalCapital / stratCount
	stratMetrics := make([]StrategyMetrics, 0, len(stratMap))
	for _, stratTrades := range stratMap {
		sm := computeStrategyMetrics(stratTrades[0].StrategyID, stratTrades, perStratNAV)
		stratMetrics = append(stratMetrics, sm)
	}

	// rank + capital allocation
	ranked := RankStrategies(stratMetrics, v.TotalCapital)

	// time range
	dataFrom := trades[0].EntryTime
	dataTo := trades[len(trades)-1].ExitTime

	result := ValidationResult{
		GeneratedAt: time.Now().UTC(),
		DataFrom:    dataFrom,
		DataTo:      dataTo,
		Portfolio:   pm,
		Strategies:  ranked,
	}

	Certify(&result)
	return result
}

// groupByStrategy buckets trades by StrategyID preserving order.
func groupByStrategy(trades []TradeRecord) map[string][]TradeRecord {
	m := make(map[string][]TradeRecord)
	for _, t := range trades {
		m[t.StrategyID] = append(m[t.StrategyID], t)
	}
	return m
}

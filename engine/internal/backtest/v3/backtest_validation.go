package v3

import (
	"fmt"
	"math"
)

// ValidationConfig controls how simulation error is measured.
type ValidationConfig struct {
	MaxSimErrorPct float64 // fail if error > this threshold (default 10%)
}

// HistoricalFill is a reference fill from live or paper trading for comparison.
type HistoricalFill struct {
	OrderID        string
	Symbol         string
	Side           string
	RequestedQty   float64
	FilledQty      float64
	FillPrice      float64
	SpreadBps      float64
	SlippageBps    float64
	CommissionUSD  float64
}

// SimulatedFill is the corresponding simulated fill from V3 backtest.
type SimulatedFill struct {
	OrderID       string
	Symbol        string
	Side          string
	RequestedQty  float64
	FilledQty     float64
	FillPrice     float64
	SpreadBps     float64
	SlippageBps   float64
	CommissionUSD float64
}

// ValidationPair links a historical fill to its simulated counterpart.
type ValidationPair struct {
	Historical HistoricalFill
	Simulated  SimulatedFill
}

// ValidationReport is the result of comparing simulation to historical fills.
type ValidationReport struct {
	Pairs            int
	FillPriceErrorPct  float64 // mean absolute error in fill price
	SlippageErrorPct   float64 // mean absolute error in slippage bps
	SpreadErrorPct     float64 // mean absolute error in spread bps
	FillQtyErrorPct    float64 // mean absolute error in fill quantity
	CommissionErrorPct float64
	CompositErrorPct   float64 // weighted composite
	PassesThreshold    bool
	MaxAllowedErrorPct float64
	Details            []ValidationPairResult
}

// ValidationPairResult is a per-fill comparison result.
type ValidationPairResult struct {
	OrderID          string
	FillPriceErrorPct float64
	SlippageErrorPct  float64
	SpreadErrorPct    float64
	QtyErrorPct       float64
}

// Validate compares simulated fills to historical reference fills and returns
// a ValidationReport. Passes if composite error is below cfg.MaxSimErrorPct.
func Validate(pairs []ValidationPair, cfg ValidationConfig) (ValidationReport, error) {
	maxErr := cfg.MaxSimErrorPct
	if maxErr <= 0 {
		maxErr = 10.0
	}
	if len(pairs) == 0 {
		return ValidationReport{MaxAllowedErrorPct: maxErr, PassesThreshold: true}, nil
	}

	report := ValidationReport{
		Pairs:            len(pairs),
		MaxAllowedErrorPct: maxErr,
		Details:          make([]ValidationPairResult, 0, len(pairs)),
	}

	for _, p := range pairs {
		detail := ValidationPairResult{OrderID: p.Historical.OrderID}

		// Fill price error.
		if p.Historical.FillPrice > 0 {
			detail.FillPriceErrorPct = math.Abs(p.Simulated.FillPrice-p.Historical.FillPrice) / p.Historical.FillPrice * 100
		}

		// Slippage bps error.
		if p.Historical.SlippageBps > 0 {
			detail.SlippageErrorPct = math.Abs(p.Simulated.SlippageBps-p.Historical.SlippageBps) / p.Historical.SlippageBps * 100
		}

		// Spread bps error.
		if p.Historical.SpreadBps > 0 {
			detail.SpreadErrorPct = math.Abs(p.Simulated.SpreadBps-p.Historical.SpreadBps) / p.Historical.SpreadBps * 100
		}

		// Fill quantity error.
		if p.Historical.FilledQty > 0 {
			detail.QtyErrorPct = math.Abs(p.Simulated.FilledQty-p.Historical.FilledQty) / p.Historical.FilledQty * 100
		}

		report.FillPriceErrorPct += detail.FillPriceErrorPct
		report.SlippageErrorPct += detail.SlippageErrorPct
		report.SpreadErrorPct += detail.SpreadErrorPct
		report.FillQtyErrorPct += detail.QtyErrorPct
		report.Details = append(report.Details, detail)
	}

	n := float64(len(pairs))
	report.FillPriceErrorPct /= n
	report.SlippageErrorPct /= n
	report.SpreadErrorPct /= n
	report.FillQtyErrorPct /= n

	// Commission error separately.
	commErr := 0.0
	for _, p := range pairs {
		if p.Historical.CommissionUSD > 0 {
			commErr += math.Abs(p.Simulated.CommissionUSD-p.Historical.CommissionUSD) / p.Historical.CommissionUSD * 100
		}
	}
	report.CommissionErrorPct = commErr / n

	// Weighted composite: fill price 40%, slippage 30%, spread 20%, qty 10%.
	report.CompositErrorPct = report.FillPriceErrorPct*0.40 +
		report.SlippageErrorPct*0.30 +
		report.SpreadErrorPct*0.20 +
		report.FillQtyErrorPct*0.10

	report.PassesThreshold = report.CompositErrorPct <= maxErr

	if !report.PassesThreshold {
		return report, fmt.Errorf(
			"simulation validation failed: composite error %.2f%% exceeds threshold %.2f%%",
			report.CompositErrorPct, maxErr,
		)
	}
	return report, nil
}

// ValidateFromTrades builds ValidationPairs by matching simulated V3Trade fills
// to historical fills by order ID prefix and runs Validate.
func ValidateFromTrades(trades []V3Trade, historical []HistoricalFill, cfg ValidationConfig) (ValidationReport, error) {
	hMap := make(map[string]HistoricalFill, len(historical))
	for _, h := range historical {
		hMap[h.OrderID] = h
	}

	pairs := make([]ValidationPair, 0, len(trades))
	for _, t := range trades {
		h, ok := hMap[t.ID]
		if !ok {
			continue
		}
		notional := t.EntryPrice * t.Size
		simSlipBps := 0.0
		simSpreadBps := 0.0
		if notional > 0 {
			simSlipBps = t.SlippageCostUSD / notional * 10_000
			simSpreadBps = t.SpreadCostUSD / notional * 10_000
		}
		pairs = append(pairs, ValidationPair{
			Historical: h,
			Simulated: SimulatedFill{
				OrderID:       t.ID,
				Symbol:        t.Symbol,
				Side:          string(t.Side),
				RequestedQty:  t.Size,
				FilledQty:     t.EntryFill.FilledQuantity,
				FillPrice:     t.EntryFill.FillPrice,
				SpreadBps:     simSpreadBps,
				SlippageBps:   simSlipBps,
				CommissionUSD: t.CommissionUSD,
			},
		})
	}

	return Validate(pairs, cfg)
}

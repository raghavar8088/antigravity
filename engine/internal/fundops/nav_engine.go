// Phase 20B — NAV Calculation Engine
// Deterministic daily/intraday/monthly/year-end NAV with full historical reconstruction.
package fundops

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ─── NAV Inputs ───────────────────────────────────────────────────────────────

// Position is a single portfolio position for NAV calculation.
type Position struct {
	Symbol       string
	Quantity     float64
	MarketPrice  float64 // current mark-to-market price
	AverageEntry float64 // average entry price
	Side         string  // LONG / SHORT
}

// NAVInputs captures all inputs required for one NAV calculation.
type NAVInputs struct {
	FundID          string
	AsOf            time.Time
	Cash            float64     // settled cash balance
	Positions       []Position  // open positions marked to market
	RealizedPnLYTD  float64     // year-to-date realised PnL
	AccruedFees     float64     // total accrued management + performance fees
	AccruedExpenses float64     // prime brokerage, admin, custody fees
	FundingAccrued  float64     // accrued funding costs (perpetual futures)
	PriorNAV        float64     // prior period NAV for return calculation
	PriorYearNAV    float64     // year-start NAV for YTD return
	TotalUnits      float64     // current outstanding units
}

// NAVResult is the output of one NAV calculation.
type NAVResult struct {
	FundID          string
	AsOf            time.Time
	Cash            float64
	MarketValue     float64 // gross market value of positions
	UnrealizedPnL   float64
	RealizedPnL     float64
	AccruedFees     float64
	AccruedExpenses float64
	FundingAccrued  float64
	GrossNAV        float64 // before fee deductions
	TotalNAV        float64 // net NAV = Gross - Fees - Expenses
	NAVPerUnit      float64
	TotalUnits      float64
	PeriodReturn    float64 // (NAV - PriorNAV) / PriorNAV
	YTDReturn       float64 // (NAV - PriorYearNAV) / PriorYearNAV
	LongExposure    float64
	ShortExposure   float64
	GrossExposure   float64
	NetExposure     float64
	Leverage        float64
	CalculatedAt    time.Time
}

// NAVFrequency defines the NAV calculation frequency.
type NAVFrequency string

const (
	NAVIntraday  NAVFrequency = "INTRADAY"
	NAVDaily     NAVFrequency = "DAILY"
	NAVMonthly   NAVFrequency = "MONTHLY"
	NAVYearEnd   NAVFrequency = "YEAR_END"
)

// ─── NAV Engine ───────────────────────────────────────────────────────────────

// NAVEngine calculates fund NAV with institutional accounting standards.
type NAVEngine struct {
	store EventStore
}

// NewNAVEngine creates a NAV engine backed by the fund event store.
func NewNAVEngine(store EventStore) *NAVEngine {
	return &NAVEngine{store: store}
}

// Calculate computes fund NAV from inputs and persists the result.
//
// Formula:
//   NAV = Cash + MarketValue + UnrealizedPnL - AccruedFees - AccruedExpenses - FundingAccrued
func (e *NAVEngine) Calculate(ctx context.Context, inputs NAVInputs) (NAVResult, error) {
	if inputs.TotalUnits <= 0 {
		return NAVResult{}, fmt.Errorf("navengine: total units must be positive (got %.6f)", inputs.TotalUnits)
	}

	// Compute market value and unrealized PnL from positions.
	var marketValue, unrealizedPnL, longExposure, shortExposure float64
	for _, pos := range inputs.Positions {
		posValue := pos.Quantity * pos.MarketPrice
		posUPnL := pos.Quantity * (pos.MarketPrice - pos.AverageEntry)
		if pos.Side == "SHORT" {
			posValue = -posValue
			posUPnL = -posUPnL
			shortExposure += math.Abs(posValue)
		} else {
			longExposure += posValue
		}
		marketValue += posValue
		unrealizedPnL += posUPnL
	}
	grossExposure := longExposure + shortExposure
	netExposure := longExposure - shortExposure

	// Gross NAV before deductions.
	grossNAV := inputs.Cash + marketValue + unrealizedPnL + inputs.RealizedPnLYTD

	// Net NAV: deduct accrued fees, expenses, and funding costs.
	totalNAV := grossNAV - inputs.AccruedFees - inputs.AccruedExpenses - inputs.FundingAccrued

	// NAV per unit.
	navPerUnit := totalNAV / inputs.TotalUnits

	// Period return.
	periodReturn := 0.0
	if inputs.PriorNAV > 0 {
		periodReturn = (navPerUnit - inputs.PriorNAV) / inputs.PriorNAV
	}
	ytdReturn := 0.0
	if inputs.PriorYearNAV > 0 {
		ytdReturn = (navPerUnit - inputs.PriorYearNAV) / inputs.PriorYearNAV
	}

	// Leverage = gross exposure / NAV.
	leverage := 0.0
	if totalNAV > 0 {
		leverage = grossExposure / totalNAV
	}

	result := NAVResult{
		FundID:          inputs.FundID,
		AsOf:            inputs.AsOf,
		Cash:            inputs.Cash,
		MarketValue:     marketValue,
		UnrealizedPnL:   unrealizedPnL,
		RealizedPnL:     inputs.RealizedPnLYTD,
		AccruedFees:     inputs.AccruedFees,
		AccruedExpenses: inputs.AccruedExpenses,
		FundingAccrued:  inputs.FundingAccrued,
		GrossNAV:        grossNAV,
		TotalNAV:        totalNAV,
		NAVPerUnit:      navPerUnit,
		TotalUnits:      inputs.TotalUnits,
		PeriodReturn:    periodReturn,
		YTDReturn:       ytdReturn,
		LongExposure:    longExposure,
		ShortExposure:   shortExposure,
		GrossExposure:   grossExposure,
		NetExposure:     netExposure,
		Leverage:        leverage,
		CalculatedAt:    time.Now().UTC(),
	}

	// Persist NAV event.
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggNAV,
		AggregateID:   inputs.FundID,
		FundID:        inputs.FundID,
		EventType:     EvtNAVCalculated,
		Payload: NAVCalculatedPayload{
			FundID: inputs.FundID, AsOf: inputs.AsOf,
			TotalNAV: totalNAV, NAVPerUnit: navPerUnit,
			TotalUnits: inputs.TotalUnits, Cash: inputs.Cash,
			MarketValue: marketValue, UnrealizedPnL: unrealizedPnL,
			RealizedPnL: inputs.RealizedPnLYTD,
			AccruedFees: inputs.AccruedFees,
			AccruedExpenses: inputs.AccruedExpenses,
			PeriodReturn: periodReturn * 100, YTDReturn: ytdReturn * 100,
		},
	})
	if err != nil {
		return result, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return result, fmt.Errorf("navengine: persist NAVCalculated: %w", err)
	}
	return result, nil
}

// ReconstructHistory rebuilds the full NAV history from the event log.
// This is deterministic and used for audit purposes.
func (e *NAVEngine) ReconstructHistory(ctx context.Context, fundID string) ([]NAVPoint, error) {
	evts, err := e.store.Replay(ctx, AggNAV, fundID)
	if err != nil {
		return nil, fmt.Errorf("navengine: reconstruct history: %w", err)
	}
	var history []NAVPoint
	for _, ev := range evts {
		if ev.EventType == EvtNAVCalculated {
			var p NAVCalculatedPayload
			if err := unmarshal(ev.Payload, &p); err == nil {
				history = append(history, NAVPoint{
					AsOf: p.AsOf, TotalNAV: p.TotalNAV, NAVPerUnit: p.NAVPerUnit,
					TotalUnits: p.TotalUnits, PeriodReturn: p.PeriodReturn, YTDReturn: p.YTDReturn,
				})
			}
		}
	}
	return history, nil
}

// AnnualizedReturn computes the annualised return over a full NAV history.
func AnnualizedReturn(history []NAVPoint) float64 {
	if len(history) < 2 {
		return 0
	}
	first := history[0]
	last := history[len(history)-1]
	if first.NAVPerUnit <= 0 {
		return 0
	}
	totalReturn := (last.NAVPerUnit - first.NAVPerUnit) / first.NAVPerUnit
	days := last.AsOf.Sub(first.AsOf).Hours() / 24
	if days <= 0 {
		return 0
	}
	// CAGR = (1 + totalReturn)^(365/days) - 1
	return math.Pow(1+totalReturn, 365/days) - 1
}

// MaxDrawdown computes the maximum peak-to-trough drawdown over NAV history.
func MaxDrawdown(history []NAVPoint) float64 {
	if len(history) == 0 {
		return 0
	}
	peak := history[0].NAVPerUnit
	maxDD := 0.0
	for _, p := range history {
		if p.NAVPerUnit > peak {
			peak = p.NAVPerUnit
		}
		if peak > 0 {
			dd := (peak - p.NAVPerUnit) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

// SharpeRatio computes the annualised Sharpe ratio over NAV history.
// riskFreeRate should be the annual risk-free rate (e.g. 0.05 = 5%).
func SharpeRatio(history []NAVPoint, riskFreeRate float64) float64 {
	if len(history) < 2 {
		return 0
	}
	// Daily returns.
	returns := make([]float64, len(history)-1)
	for i := 1; i < len(history); i++ {
		if history[i-1].NAVPerUnit > 0 {
			returns[i-1] = (history[i].NAVPerUnit - history[i-1].NAVPerUnit) / history[i-1].NAVPerUnit
		}
	}
	mean, std := 0.0, 0.0
	for _, r := range returns {
		mean += r
	}
	mean /= float64(len(returns))
	for _, r := range returns {
		d := r - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(returns)))
	if std == 0 {
		return 0
	}
	dailyRFR := riskFreeRate / 365
	return ((mean - dailyRFR) / std) * math.Sqrt(365)
}

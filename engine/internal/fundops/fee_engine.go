// Phase 20J — Fund Fees Engine
// Industry-standard hedge fund fee accounting: management fee, performance fee,
// high-water mark, and hurdle rate. All fee events are immutable and replayable.
package fundops

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ─── Fee Engine ───────────────────────────────────────────────────────────────

// FeeEngine computes and accrues management and performance fees.
type FeeEngine struct {
	store          EventStore
	fundID         string
	managementFee  float64 // annual rate, e.g. 0.02 = 2%
	performanceFee float64 // rate on profits above HWM, e.g. 0.20 = 20%
	hurdleRate     float64 // annual hurdle, e.g. 0.05 = 5%
	hwm            float64 // high-water mark NAV per unit
	accruedMgmt    float64
	accruedPerf    float64
}

// NewFeeEngine creates a fee engine with institutional hedge fund fee structure.
func NewFeeEngine(store EventStore, fundID string, mgmtFee, perfFee, hurdleRate, initialHWM float64) *FeeEngine {
	return &FeeEngine{
		store:          store,
		fundID:         fundID,
		managementFee:  mgmtFee,
		performanceFee: perfFee,
		hurdleRate:     hurdleRate,
		hwm:            initialHWM,
	}
}

// AccrueManagementFee accrues the daily management fee and persists the event.
//
// Daily Management Fee = AUM × Annual Rate × (1 / 365)
func (e *FeeEngine) AccrueManagementFee(ctx context.Context, navUSD float64, asOf time.Time) (float64, error) {
	if navUSD <= 0 || e.managementFee <= 0 {
		return 0, nil
	}
	dailyFee := navUSD * e.managementFee / 365.0

	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggFee,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtFeeAccrued,
		Payload: FeeAccruedPayload{
			FundID: e.fundID, FeeType: "MANAGEMENT",
			AmountUSD: dailyFee, Period: "DAILY",
			AsOf: asOf, NAV: navUSD, Rate: e.managementFee,
		},
	})
	if err != nil {
		return 0, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return 0, fmt.Errorf("feeengine: persist mgmt fee: %w", err)
	}
	e.accruedMgmt += dailyFee
	return dailyFee, nil
}

// AccruePerformanceFee accrues the performance fee for a period.
//
// Performance Fee calculation:
//   1. Only charged when NAVPerUnit > HWM (above high-water mark).
//   2. Only charged on returns above the hurdle rate.
//   3. Performance Fee = max(0, (Return - Hurdle) × PerfFeeRate × NAV)
func (e *FeeEngine) AccruePerformanceFee(ctx context.Context, navPerUnit, navUSD float64, periodDays int, asOf time.Time) (float64, error) {
	if navPerUnit <= e.hwm {
		return 0, nil // below HWM — no performance fee
	}

	// Annualise the hurdle rate to the period.
	periodHurdle := math.Pow(1+e.hurdleRate, float64(periodDays)/365) - 1
	periodReturn := (navPerUnit - e.hwm) / e.hwm

	excessReturn := periodReturn - periodHurdle
	if excessReturn <= 0 {
		return 0, nil // below hurdle — no performance fee
	}

	perfFee := excessReturn * e.performanceFee * navUSD
	if perfFee <= 0 {
		return 0, nil
	}

	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggFee,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtFeeAccrued,
		Payload: FeeAccruedPayload{
			FundID: e.fundID, FeeType: "PERFORMANCE",
			AmountUSD: perfFee, Period: fmt.Sprintf("%dD", periodDays),
			AsOf: asOf, NAV: navUSD, Rate: e.performanceFee,
		},
	})
	if err != nil {
		return 0, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return 0, fmt.Errorf("feeengine: persist perf fee: %w", err)
	}
	e.accruedPerf += perfFee
	return perfFee, nil
}

// PayFees crystallises and pays all accrued fees for a period.
func (e *FeeEngine) PayFees(ctx context.Context, feeType string, payDate time.Time) (float64, error) {
	var amount float64
	switch feeType {
	case "MANAGEMENT":
		amount = e.accruedMgmt
	case "PERFORMANCE":
		amount = e.accruedPerf
	default:
		return 0, fmt.Errorf("feeengine: unknown fee type %q", feeType)
	}
	if amount <= 0 {
		return 0, nil
	}

	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggFee,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtFeePaid,
		Payload: FeePaidPayload{
			FundID: e.fundID, FeeType: feeType,
			AmountUSD: amount, PaidDate: payDate,
		},
	})
	if err != nil {
		return 0, err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return 0, fmt.Errorf("feeengine: persist fee payment: %w", err)
	}

	switch feeType {
	case "MANAGEMENT":
		e.accruedMgmt = 0
	case "PERFORMANCE":
		e.accruedPerf = 0
	}
	return amount, nil
}

// UpdateHighWaterMark updates the HWM if the current NAV per unit is above it.
// Must be called after each performance fee crystallisation.
func (e *FeeEngine) UpdateHighWaterMark(ctx context.Context, navPerUnit float64) error {
	if navPerUnit <= e.hwm {
		return nil // HWM only moves up
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggFee,
		AggregateID:   e.fundID,
		FundID:        e.fundID,
		EventType:     EvtHWMUpdated,
		Payload:       map[string]any{"hwm": navPerUnit, "prev_hwm": e.hwm, "updated_at": time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		return fmt.Errorf("feeengine: persist HWM update: %w", err)
	}
	e.hwm = navPerUnit
	return nil
}

// AccruedManagementFee returns the current accrued management fee.
func (e *FeeEngine) AccruedManagementFee() float64 { return e.accruedMgmt }

// AccruedPerformanceFee returns the current accrued performance fee.
func (e *FeeEngine) AccruedPerformanceFee() float64 { return e.accruedPerf }

// TotalAccruedFees returns all accrued fees.
func (e *FeeEngine) TotalAccruedFees() float64 { return e.accruedMgmt + e.accruedPerf }

// HighWaterMark returns the current HWM.
func (e *FeeEngine) HighWaterMark() float64 { return e.hwm }

// AnnualManagementFee computes the annual management fee for a given AUM.
func AnnualManagementFee(aumUSD, rate float64) float64 {
	return aumUSD * rate
}

// PerformanceFeeAmount computes the performance fee for a period.
// Returns 0 if below HWM or hurdle.
func PerformanceFeeAmount(navPerUnit, hwm, hurdleAnnual float64, periodDays int, navUSD, perfFeeRate float64) float64 {
	if navPerUnit <= hwm {
		return 0
	}
	periodHurdle := math.Pow(1+hurdleAnnual, float64(periodDays)/365) - 1
	excessReturn := (navPerUnit-hwm)/hwm - periodHurdle
	if excessReturn <= 0 {
		return 0
	}
	return excessReturn * perfFeeRate * navUSD
}

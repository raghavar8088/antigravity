// Phase 20A — Fund Accounting Core
// Event-sourced fund aggregate with state machine enforcement.
package fundops

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ─── Fund Aggregate ───────────────────────────────────────────────────────────

// Fund is the root aggregate for all fund accounting operations.
// All mutations go through this aggregate; it enforces invariants before
// emitting events to the store.
type Fund struct {
	FundID         string
	Name           string
	Strategy       string
	Currency       string
	Status         FundStatus
	InceptionDate  time.Time
	ManagementFee  float64 // fractional, e.g. 0.02
	PerformanceFee float64 // fractional, e.g. 0.20
	HurdleRate     float64 // fractional, e.g. 0.05
	TotalUnits     float64
	TotalNAV       float64
	NAVPerUnit     float64
	Version        int64

	store EventStore
}

// NewFund creates and persists a new fund aggregate.
func NewFund(ctx context.Context, store EventStore, input FundCreatedPayload) (*Fund, error) {
	if input.FundID == "" {
		return nil, errors.New("fundops: fund id required")
	}
	if input.Name == "" {
		return nil, errors.New("fundops: fund name required")
	}
	if input.ManagementFee < 0 || input.ManagementFee > 0.10 {
		return nil, fmt.Errorf("fundops: management fee %.2f%% out of range (0–10%%)", input.ManagementFee*100)
	}
	if input.PerformanceFee < 0 || input.PerformanceFee > 0.50 {
		return nil, fmt.Errorf("fundops: performance fee %.2f%% out of range (0–50%%)", input.PerformanceFee*100)
	}

	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggFund,
		AggregateID:   input.FundID,
		FundID:        input.FundID,
		EventType:     EvtFundCreated,
		Payload:       input,
	})
	if err != nil {
		return nil, err
	}
	if _, err := store.Append(ctx, ev); err != nil {
		return nil, fmt.Errorf("fundops: persist FundCreated: %w", err)
	}

	// Initial NAV per unit: if initial NAV provided, set to 1000 (institutional standard).
	initialPerUnit := 1000.0
	if input.InitialNAV > 0 {
		initialPerUnit = 1000.0
	}

	return &Fund{
		FundID: input.FundID, Name: input.Name,
		Strategy: input.Strategy, Currency: input.Currency,
		Status: FundStatusActive, InceptionDate: input.InceptionDate,
		ManagementFee: input.ManagementFee, PerformanceFee: input.PerformanceFee,
		HurdleRate: input.HurdleRate,
		TotalUnits: input.InitialNAV / initialPerUnit,
		TotalNAV: input.InitialNAV, NAVPerUnit: initialPerUnit,
		Version: 1, store: store,
	}, nil
}

// LoadFund reconstitutes a fund aggregate by replaying its events.
func LoadFund(ctx context.Context, store EventStore, fundID string) (*Fund, error) {
	result, err := ReplayFund(ctx, store, fundID)
	if err != nil {
		return nil, fmt.Errorf("fundops: load fund %s: %w", fundID, err)
	}
	fp := result.Fund
	return &Fund{
		FundID: fp.FundID, Name: fp.Name, Strategy: fp.Strategy,
		Currency: fp.Currency, Status: fp.Status,
		InceptionDate: fp.InceptionDate,
		ManagementFee: fp.ManagementFee, PerformanceFee: fp.PerformanceFee,
		HurdleRate: fp.HurdleRate,
		TotalUnits: fp.TotalUnits, TotalNAV: fp.TotalNAV, NAVPerUnit: fp.NAVPerUnit,
		Version: int64(result.TotalEvents), store: store,
	}, nil
}

// Close marks the fund as closed. Requires all capital to be redeemed first.
func (f *Fund) Close(ctx context.Context) error {
	if f.Status != FundStatusActive {
		return fmt.Errorf("fundops: cannot close fund in state %s", f.Status)
	}
	if f.TotalUnits > 0.001 {
		return fmt.Errorf("fundops: cannot close fund with %.4f outstanding units", f.TotalUnits)
	}
	ev, err := NewFundEvent(NewEventInput{
		AggregateType: AggFund, AggregateID: f.FundID,
		FundID: f.FundID, EventType: EvtFundClosed,
		Payload: map[string]any{"fund_id": f.FundID, "closed_at": time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if _, err := f.store.Append(ctx, ev); err != nil {
		return err
	}
	f.Status = FundStatusClosed
	f.Version++
	return nil
}

// IssueUnits increases the fund's unit count and NAV on subscription.
func (f *Fund) IssueUnits(units, navPerUnit float64) {
	f.TotalUnits += units
	f.TotalNAV += units * navPerUnit
	if f.TotalUnits > 0 {
		f.NAVPerUnit = f.TotalNAV / f.TotalUnits
	}
}

// RetireUnits decreases the fund's unit count and NAV on redemption.
func (f *Fund) RetireUnits(units, navPerUnit float64) error {
	if units > f.TotalUnits+1e-9 {
		return fmt.Errorf("fundops: cannot redeem %.6f units, only %.6f available", units, f.TotalUnits)
	}
	f.TotalUnits -= units
	f.TotalNAV -= units * navPerUnit
	if f.TotalUnits <= 0 {
		f.TotalNAV = 0
		f.TotalUnits = 0
	} else {
		f.NAVPerUnit = f.TotalNAV / f.TotalUnits
	}
	return nil
}

// UpdateNAV sets the current NAV per unit and total NAV.
func (f *Fund) UpdateNAV(totalNAV float64) {
	f.TotalNAV = totalNAV
	if f.TotalUnits > 0 {
		f.NAVPerUnit = totalNAV / f.TotalUnits
	}
}

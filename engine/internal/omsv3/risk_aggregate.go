package omsv3

import (
	"encoding/json"
	"fmt"

	"antigravity-engine/internal/ledger"
)

// RiskAggregate is the event-sourced read model of the risk engine's volatile
// state. It is rebuilt exclusively from RISK ledger events, making risk history
// fully replayable after a crash.
//
// The RiskAggregate does NOT replace the in-memory RiskEngine (which runs the
// real-time validation logic). Instead it is the authoritative historical record:
//
//   RiskEngine.Validate() → emits EventRiskApproved/Blocked
//   RiskAggregate.ApplyEvent() ← replays those events on boot
//
// The risk engine rehydrates its exposure and P&L counters from the aggregate
// during crash recovery, eliminating the need to query SQLite or MongoDB.
type RiskAggregate struct {
	AccountID string

	// Running totals — rebuilt from events
	CurrentExposureBTC  float64
	DailyPnLUSD         float64
	WeeklyPnLUSD        float64
	MonthlyPnLUSD       float64
	TotalLossUSD        float64
	HighWatermarkUSD    float64

	// Check statistics
	TotalChecks    int
	TotalApproved  int
	TotalBlocked   int
	TotalViolations int

	// Violation counts by type
	ExposureViolations     int
	DrawdownViolations     int
	DailyLossViolations    int
	MarginViolations       int
	LeverageViolations     int
	ConcentrationViolations int
	CorrelationViolations  int

	// Kill switch
	KillSwitchActive bool
	KillSwitchTrigger string

	// Audit fields
	Version int64
	Events  []ledger.Event
}

// NewRiskAggregate creates an empty risk aggregate for the given account.
func NewRiskAggregate(accountID string) *RiskAggregate {
	return &RiskAggregate{AccountID: accountID}
}

// ApplyEvent applies one RISK ledger event to the risk aggregate.
// All state mutations happen exclusively here — no direct field writes outside Apply.
func (r *RiskAggregate) ApplyEvent(event ledger.Event) error {
	if event.AggregateType != ledger.AggregateRisk {
		return fmt.Errorf("omsv3.RiskAggregate: expected RISK event, got %s", event.AggregateType)
	}

	switch event.EventType {
	case ledger.EventRiskApproved:
		var payload ledger.RiskCheckPayload
		if err := r.unmarshal(event, &payload); err != nil {
			return err
		}
		r.TotalChecks++
		r.TotalApproved++
		// Update exposure from approved trade
		r.CurrentExposureBTC = payload.ProposedExposureBTC

	case ledger.EventRiskBlocked:
		var payload ledger.RiskCheckPayload
		if err := r.unmarshal(event, &payload); err != nil {
			return err
		}
		r.TotalChecks++
		r.TotalBlocked++

	case ledger.EventRiskViolation:
		r.TotalViolations++

	case ledger.EventExposureLimitExceeded:
		r.ExposureViolations++

	case ledger.EventMaxDrawdownBreached:
		r.DrawdownViolations++

	case ledger.EventRiskDailyLossLimitExceeded:
		r.DailyLossViolations++

	case ledger.EventRiskMarginViolation:
		r.MarginViolations++

	case ledger.EventRiskLeverageViolation:
		r.LeverageViolations++

	case ledger.EventRiskConcentrationViolation:
		r.ConcentrationViolations++

	case ledger.EventRiskCorrelationViolation:
		r.CorrelationViolations++

	case ledger.EventKillSwitchTriggered:
		r.KillSwitchActive = true
		r.KillSwitchTrigger = string(event.EventType)

	case ledger.EventKillSwitchReleased:
		r.KillSwitchActive = false
		r.KillSwitchTrigger = ""

	case ledger.EventRiskTriggered, ledger.EventPortfolioHeatExceeded,
		ledger.EventVaRBreach, ledger.EventCVaRBreach,
		ledger.EventFundingExposureExceeded, ledger.EventRiskCheckStarted:
		// Informational events — no counter mutation required.

	default:
		// Unknown RISK event type — forward-compatible: silently accepted.
	}

	r.Version = event.SequenceNo
	r.Events = append(r.Events, event)
	return nil
}

// ApplyPositionClose updates the risk aggregate's PnL counters from a position
// close event. This is called during replay when a POSITION_CLOSED event is seen,
// so the risk aggregate stays in sync with realised P&L without requiring the
// RiskEngine to emit a separate risk event for every close.
func (r *RiskAggregate) ApplyPositionClose(event ledger.Event) {
	if event.EventType != ledger.EventPositionClosed &&
		event.EventType != ledger.EventPositionLiquidated {
		return
	}
	var payload ledger.PositionClosedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return
	}
	r.DailyPnLUSD += payload.NetPnLUSD
	r.WeeklyPnLUSD += payload.NetPnLUSD
	r.MonthlyPnLUSD += payload.NetPnLUSD
	if payload.NetPnLUSD < 0 {
		r.TotalLossUSD += -payload.NetPnLUSD
	}
	// Update high watermark
	equity := r.HighWatermarkUSD + payload.NetPnLUSD
	if equity > r.HighWatermarkUSD {
		r.HighWatermarkUSD = equity
	}
}

// ReplayRiskAggregate rebuilds a RiskAggregate by replaying all account events.
// It processes both RISK events (for check/violation counts) and POSITION_CLOSED
// events (for P&L accumulation).
func ReplayRiskAggregate(accountID string, events []ledger.Event) (*RiskAggregate, error) {
	agg := NewRiskAggregate(accountID)
	for _, e := range events {
		switch e.AggregateType {
		case ledger.AggregateRisk:
			if err := agg.ApplyEvent(e); err != nil {
				return nil, fmt.Errorf("ReplayRiskAggregate: %w", err)
			}
		case ledger.AggregatePosition:
			agg.ApplyPositionClose(e)
		}
	}
	return agg, nil
}

func (r *RiskAggregate) unmarshal(event ledger.Event, dst any) error {
	if len(event.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(event.Payload, dst); err != nil {
		return fmt.Errorf("omsv3.RiskAggregate: unmarshal %s: %w", event.EventType, err)
	}
	return nil
}

// ApprovalRate returns the fraction of risk checks that were approved.
func (r *RiskAggregate) ApprovalRate() float64 {
	if r.TotalChecks == 0 {
		return 0
	}
	return float64(r.TotalApproved) / float64(r.TotalChecks)
}

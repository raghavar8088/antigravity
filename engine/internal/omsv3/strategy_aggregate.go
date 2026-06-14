package omsv3

import (
	"encoding/json"
	"fmt"

	"antigravity-engine/internal/ledger"
)

// StrategyState is the lifecycle state of a strategy registered in OMS v3.
type StrategyState string

const (
	StrategyStateEnabled  StrategyState = "ENABLED"
	StrategyStateDisabled StrategyState = "DISABLED"
	StrategyStatePaused   StrategyState = "PAUSED"
	StrategyStateResumed  StrategyState = "RESUMED"
	StrategyStatePromoted StrategyState = "PROMOTED"
	StrategyStateDemoted  StrategyState = "DEMOTED"
)

// StrategyAggregate is the event-sourced aggregate for a single strategy's lifecycle.
// It tracks registration, enable/disable/pause state and allocation changes.
type StrategyAggregate struct {
	ID             string
	AccountID      string
	Name           string
	State          StrategyState
	AllocPct       float64
	AllocUSD       float64
	ProfitFactor   float64
	Version        int64
	Events         []ledger.Event
}

// NewStrategyAggregate constructs an empty aggregate for the given strategy ID.
func NewStrategyAggregate(id string) *StrategyAggregate {
	return &StrategyAggregate{ID: id, State: StrategyStateDisabled}
}

// IsActive returns true when the strategy is in a trading-eligible state.
func (a *StrategyAggregate) IsActive() bool {
	return a.State == StrategyStateEnabled ||
		a.State == StrategyStateResumed ||
		a.State == StrategyStatePromoted
}

// ApplyEvent advances the aggregate state based on a ledger event.
func (a *StrategyAggregate) ApplyEvent(e ledger.Event) error {
	a.Version++
	a.Events = append(a.Events, e)

	type lifecyclePayload struct {
		StrategyName string  `json:"strategy_name"`
		AccountID    string  `json:"account_id"`
		ProfitFactor float64 `json:"profit_factor"`
		Reason       string  `json:"reason"`
	}
	var p lifecyclePayload
	if len(e.Payload) > 0 {
		_ = json.Unmarshal(e.Payload, &p)
	}
	if p.StrategyName != "" {
		a.Name = p.StrategyName
	}
	if p.AccountID != "" {
		a.AccountID = p.AccountID
	}
	if p.ProfitFactor > 0 {
		a.ProfitFactor = p.ProfitFactor
	}

	switch e.EventType {
	case ledger.EventStrategyRegistered, ledger.EventStrategyEnabled:
		a.State = StrategyStateEnabled
	case ledger.EventStrategyDisabled:
		a.State = StrategyStateDisabled
	case ledger.EventStrategyPaused:
		a.State = StrategyStatePaused
	case ledger.EventStrategyResumed:
		a.State = StrategyStateResumed
	case ledger.EventStrategyPromoted:
		a.State = StrategyStatePromoted
	case ledger.EventStrategyDemoted:
		a.State = StrategyStateDemoted
	case ledger.EventStrategyAllocationChanged:
		// handled by ApplyAllocationChange; no state transition
	default:
		return fmt.Errorf("omsv3.StrategyAggregate: unknown event type %q", e.EventType)
	}
	return nil
}

// ApplyAllocationChange updates capital allocation from an EventStrategyAllocationChanged event.
func (a *StrategyAggregate) ApplyAllocationChange(e ledger.Event) error {
	type allocPayload struct {
		AllocPct float64 `json:"alloc_pct"`
		AllocUSD float64 `json:"alloc_usd"`
	}
	var p allocPayload
	if err := json.Unmarshal(e.Payload, &p); err != nil {
		return fmt.Errorf("omsv3.StrategyAggregate: decode alloc payload: %w", err)
	}
	if p.AllocPct > 0 {
		a.AllocPct = p.AllocPct
	}
	if p.AllocUSD > 0 {
		a.AllocUSD = p.AllocUSD
	}
	a.Version++
	a.Events = append(a.Events, e)
	return nil
}

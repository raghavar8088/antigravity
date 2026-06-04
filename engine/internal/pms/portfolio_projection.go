package pms

import (
	"encoding/json"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// PortfolioProjectionState is the read-model produced by replaying portfolio events.
// Unlike the aggregate, it is optimised for queries, not for mutation.
type PortfolioProjectionState struct {
	PortfolioID  string          `json:"portfolio_id"`
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	ParentID     string          `json:"parent_id,omitempty"`
	Status       string          `json:"status"`
	CurrentNAV   float64         `json:"current_nav_usd"`
	AllocatedPct float64         `json:"allocated_pct"`
	CashPct      float64         `json:"cash_pct"`
	StrategyCount int            `json:"strategy_count"`
	Allocations  []AllocationProjection `json:"allocations"`
	Version      int64           `json:"version"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// AllocationProjection is the read-model for one strategy allocation.
type AllocationProjection struct {
	StrategyID   string  `json:"strategy_id"`
	StrategyName string  `json:"strategy_name"`
	AllocPct     float64 `json:"alloc_pct"`
	AllocUSD     float64 `json:"alloc_usd"`
	Method       string  `json:"method"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PortfolioProjection is a CQRS read-model projection that rebuilds from events
// in a single O(n) pass. It holds no domain logic — it is a query-optimised view.
type PortfolioProjection struct {
	states map[string]*PortfolioProjectionState // keyed by portfolioID
}

// NewPortfolioProjection creates an empty projection.
func NewPortfolioProjection() *PortfolioProjection {
	return &PortfolioProjection{
		states: make(map[string]*PortfolioProjectionState),
	}
}

// Apply processes one event and updates the projection state.
// This is the single pass O(n) event handler.
func (p *PortfolioProjection) Apply(ev ledger.Event) error {
	switch ev.AggregateType {
	case AggregatePortfolio:
		return p.applyPortfolioEvent(ev)
	default:
		return nil // ignore non-portfolio events
	}
}

func (p *PortfolioProjection) applyPortfolioEvent(ev ledger.Event) error {
	switch ev.EventType {
	case EventPortfolioCreated:
		var payload PortfolioCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("projection: %w", err)
		}
		p.states[payload.PortfolioID] = &PortfolioProjectionState{
			PortfolioID: payload.PortfolioID,
			Name:        payload.Name,
			Type:        payload.Type,
			ParentID:    payload.ParentID,
			Status:      "ACTIVE",
			CurrentNAV:  payload.InitialNAV,
			AllocatedPct: 0,
			CashPct:     100,
			Allocations: []AllocationProjection{},
			Version:     1,
			UpdatedAt:   payload.CreatedAt,
		}

	case EventPortfolioNAVUpdated:
		var payload PortfolioNAVUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("projection: %w", err)
		}
		if s, ok := p.states[payload.PortfolioID]; ok {
			s.CurrentNAV = payload.CurrentNAV
			s.UpdatedAt = payload.ComputedAt
			s.Version++
			p.recomputeAllocations(s)
		}

	case EventAllocationCreated:
		var payload AllocationCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("projection: %w", err)
		}
		s, ok := p.states[payload.PortfolioID]
		if !ok {
			return nil
		}
		s.Allocations = append(s.Allocations, AllocationProjection{
			StrategyID:   payload.StrategyID,
			StrategyName: payload.StrategyName,
			AllocPct:     payload.AllocPct,
			AllocUSD:     payload.AllocUSD,
			Method:       payload.Method,
			UpdatedAt:    ev.CreatedAt,
		})
		s.Version++
		s.UpdatedAt = ev.CreatedAt
		p.recomputeAllocations(s)

	case EventAllocationChanged:
		var payload AllocationChangedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("projection: %w", err)
		}
		s, ok := p.states[payload.PortfolioID]
		if !ok {
			return nil
		}
		for i, a := range s.Allocations {
			if a.StrategyID == payload.StrategyID {
				s.Allocations[i].AllocPct = payload.NewPct
				s.Allocations[i].AllocUSD = payload.NewUSD
				s.Allocations[i].UpdatedAt = ev.CreatedAt
				break
			}
		}
		s.Version++
		s.UpdatedAt = ev.CreatedAt
		p.recomputeAllocations(s)

	case EventAllocationRemoved:
		var payload AllocationRemovedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("projection: %w", err)
		}
		s, ok := p.states[payload.PortfolioID]
		if !ok {
			return nil
		}
		filtered := s.Allocations[:0]
		for _, a := range s.Allocations {
			if a.StrategyID != payload.StrategyID {
				filtered = append(filtered, a)
			}
		}
		s.Allocations = filtered
		s.Version++
		s.UpdatedAt = ev.CreatedAt
		p.recomputeAllocations(s)

	case EventPortfolioDeleted:
		if s, ok := p.states[ev.AggregateID]; ok {
			s.Status = "CLOSED"
			s.Version++
			s.UpdatedAt = ev.CreatedAt
		}
	}
	return nil
}

func (p *PortfolioProjection) recomputeAllocations(s *PortfolioProjectionState) {
	total := 0.0
	for _, a := range s.Allocations {
		total += a.AllocPct
	}
	s.AllocatedPct = total
	s.CashPct = 100.0 - total
	if s.CashPct < 0 {
		s.CashPct = 0
	}
	s.StrategyCount = len(s.Allocations)
}

// State returns the projection state for one portfolio.
func (p *PortfolioProjection) State(portfolioID string) (*PortfolioProjectionState, bool) {
	s, ok := p.states[portfolioID]
	return s, ok
}

// All returns all projected portfolio states.
func (p *PortfolioProjection) All() []*PortfolioProjectionState {
	out := make([]*PortfolioProjectionState, 0, len(p.states))
	for _, s := range p.states {
		out = append(out, s)
	}
	return out
}

// ReplayAll rebuilds the projection from a slice of events in one O(n) pass.
func (p *PortfolioProjection) ReplayAll(events []ledger.Event) error {
	for _, ev := range events {
		if err := p.Apply(ev); err != nil {
			return fmt.Errorf("portfolio projection replay at seq %d: %w", ev.SequenceNo, err)
		}
	}
	return nil
}

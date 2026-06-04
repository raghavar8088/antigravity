package pms

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// PortfolioType defines the classification of a portfolio.
type PortfolioType string

const (
	PortfolioTypeMaster  PortfolioType = "MASTER"
	PortfolioTypeSub     PortfolioType = "SUB"
	PortfolioTypeManaged PortfolioType = "MANAGED"
	PortfolioTypeProp    PortfolioType = "PROP"
	PortfolioTypePaper   PortfolioType = "PAPER"
)

// PortfolioStatus tracks the lifecycle state of a portfolio.
type PortfolioStatus string

const (
	PortfolioStatusActive   PortfolioStatus = "ACTIVE"
	PortfolioStatusSuspended PortfolioStatus = "SUSPENDED"
	PortfolioStatusClosed   PortfolioStatus = "CLOSED"
)

// AllocationRecord is an immutable snapshot of a single strategy allocation.
type AllocationRecord struct {
	AllocationID   string    `json:"allocation_id"`
	StrategyID     string    `json:"strategy_id"`
	StrategyName   string    `json:"strategy_name"`
	Method         string    `json:"method"`
	AllocPct       float64   `json:"alloc_pct"`
	AllocUSD       float64   `json:"alloc_usd"`
	MaxDrawdownPct float64   `json:"max_drawdown_pct"`
	MaxPositionPct float64   `json:"max_position_pct"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RiskBudget defines portfolio-level risk limits.
type RiskBudget struct {
	MaxHeatPct        float64 `json:"max_heat_pct"`
	MaxVaR95Pct       float64 `json:"max_var95_pct"`
	MaxCVaR95Pct      float64 `json:"max_cvar95_pct"`
	MaxDrawdownPct    float64 `json:"max_drawdown_pct"`
	MaxDailyLossPct   float64 `json:"max_daily_loss_pct"`
	MaxWeeklyLossPct  float64 `json:"max_weekly_loss_pct"`
	MaxMonthlyLossPct float64 `json:"max_monthly_loss_pct"`
	MaxGrossExpPct    float64 `json:"max_gross_exp_pct"`
	MaxNetExpPct      float64 `json:"max_net_exp_pct"`
}

// DefaultRiskBudget returns conservative institutional defaults.
func DefaultRiskBudget() RiskBudget {
	return RiskBudget{
		MaxHeatPct:        15.0,
		MaxVaR95Pct:       3.0,
		MaxCVaR95Pct:      5.0,
		MaxDrawdownPct:    20.0,
		MaxDailyLossPct:   3.0,
		MaxWeeklyLossPct:  7.0,
		MaxMonthlyLossPct: 15.0,
		MaxGrossExpPct:    200.0,
		MaxNetExpPct:      100.0,
	}
}

// Portfolio is the core aggregate. It is the single source of truth for:
//   - Allocated capital per strategy
//   - Risk budget limits
//   - Portfolio NAV and performance
//   - Sub-portfolio membership
type Portfolio struct {
	mu sync.RWMutex

	// Identity
	PortfolioID  string        `json:"portfolio_id"`
	Name         string        `json:"name"`
	Type         PortfolioType `json:"type"`
	ParentID     string        `json:"parent_id,omitempty"`
	BaseCurrency string        `json:"base_currency"`
	Description  string        `json:"description,omitempty"`
	Status       PortfolioStatus `json:"status"`

	// Capital
	InitialNAV   float64 `json:"initial_nav_usd"`
	CurrentNAV   float64 `json:"current_nav_usd"`
	CashReserve  float64 `json:"cash_reserve_usd"`
	AllocatedPct float64 `json:"allocated_pct"` // sum of all strategy alloc pcts

	// Allocations keyed by strategyID
	Allocations map[string]*AllocationRecord `json:"allocations"`

	// Risk constraints
	RiskBudget RiskBudget `json:"risk_budget"`

	// Sub-portfolios (only meaningful for MASTER type)
	SubPortfolioIDs []string `json:"sub_portfolio_ids,omitempty"`

	// Event sourcing
	Version   int64          `json:"version"`
	Events    []ledger.Event `json:"-"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// NewPortfolio constructs an empty portfolio aggregate.
func NewPortfolio(id string) *Portfolio {
	return &Portfolio{
		PortfolioID: id,
		Allocations: make(map[string]*AllocationRecord),
		Status:      PortfolioStatusActive,
		RiskBudget:  DefaultRiskBudget(),
	}
}

// ApplyEvent mutates the portfolio state for one ledger event.
// All state transitions flow exclusively through this method.
func (p *Portfolio) ApplyEvent(ev ledger.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch ev.EventType {
	case EventPortfolioCreated:
		var payload PortfolioCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal PortfolioCreated: %w", err)
		}
		p.PortfolioID = payload.PortfolioID
		p.Name = payload.Name
		p.Type = PortfolioType(payload.Type)
		p.ParentID = payload.ParentID
		p.BaseCurrency = payload.BaseCurrency
		p.Description = payload.Description
		p.InitialNAV = payload.InitialNAV
		p.CurrentNAV = payload.InitialNAV
		p.CashReserve = payload.InitialNAV
		p.Status = PortfolioStatusActive
		p.CreatedAt = payload.CreatedAt
		p.UpdatedAt = payload.CreatedAt

	case EventPortfolioNAVUpdated:
		var payload PortfolioNAVUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal NAVUpdated: %w", err)
		}
		p.CurrentNAV = payload.CurrentNAV
		p.UpdatedAt = payload.ComputedAt
		p.recomputeCash()

	case EventAllocationCreated:
		var payload AllocationCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal AllocationCreated: %w", err)
		}
		p.Allocations[payload.StrategyID] = &AllocationRecord{
			AllocationID:   payload.AllocationID,
			StrategyID:     payload.StrategyID,
			StrategyName:   payload.StrategyName,
			Method:         payload.Method,
			AllocPct:       payload.AllocPct,
			AllocUSD:       payload.AllocUSD,
			MaxDrawdownPct: payload.MaxDrawdownPct,
			MaxPositionPct: payload.MaxPositionPct,
			CreatedAt:      ev.CreatedAt,
			UpdatedAt:      ev.CreatedAt,
		}
		p.recomputeAllocated()

	case EventAllocationChanged:
		var payload AllocationChangedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal AllocationChanged: %w", err)
		}
		if rec, ok := p.Allocations[payload.StrategyID]; ok {
			rec.AllocPct = payload.NewPct
			rec.AllocUSD = payload.NewUSD
			rec.UpdatedAt = ev.CreatedAt
		}
		p.recomputeAllocated()

	case EventAllocationRemoved:
		var payload AllocationRemovedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal AllocationRemoved: %w", err)
		}
		delete(p.Allocations, payload.StrategyID)
		p.recomputeAllocated()

	case EventRiskBudgetCreated, EventRiskBudgetChanged:
		var payload RiskBudgetCreatedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal RiskBudget: %w", err)
		}
		p.RiskBudget = RiskBudget{
			MaxHeatPct:        payload.MaxHeatPct,
			MaxVaR95Pct:       payload.MaxVaR95Pct,
			MaxCVaR95Pct:      payload.MaxCVaR95Pct,
			MaxDrawdownPct:    payload.MaxDrawdownPct,
			MaxDailyLossPct:   payload.MaxDailyLossPct,
			MaxWeeklyLossPct:  payload.MaxWeeklyLossPct,
			MaxMonthlyLossPct: payload.MaxMonthlyLossPct,
			MaxGrossExpPct:    payload.MaxGrossExpPct,
			MaxNetExpPct:      payload.MaxNetExpPct,
		}
		p.UpdatedAt = ev.CreatedAt

	case EventPortfolioDeleted:
		p.Status = PortfolioStatusClosed
		p.UpdatedAt = ev.CreatedAt

	case EventMasterAllocationChanged:
		var payload MasterAllocationChangedPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return fmt.Errorf("pms: unmarshal MasterAllocationChanged: %w", err)
		}
		ids := make([]string, 0, len(payload.SubAllocations))
		for _, sa := range payload.SubAllocations {
			ids = append(ids, sa.SubPortfolioID)
		}
		p.SubPortfolioIDs = ids
		p.UpdatedAt = ev.CreatedAt
	}

	p.Version++
	p.Events = append(p.Events, ev)
	return nil
}

// recomputeAllocated recalculates the allocated percentage from all allocation records.
func (p *Portfolio) recomputeAllocated() {
	total := 0.0
	for _, a := range p.Allocations {
		total += a.AllocPct
	}
	p.AllocatedPct = total
	p.recomputeCash()
}

// recomputeCash recomputes the cash reserve as unallocated NAV.
func (p *Portfolio) recomputeCash() {
	allocated := p.CurrentNAV * p.AllocatedPct / 100.0
	p.CashReserve = p.CurrentNAV - allocated
	if p.CashReserve < 0 {
		p.CashReserve = 0
	}
}

// BudgetFor returns the capital budget in USD for a given strategyID.
// Returns 0 if the strategy has no allocation.
func (p *Portfolio) BudgetFor(strategyID string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	rec, ok := p.Allocations[strategyID]
	if !ok {
		return 0
	}
	return p.CurrentNAV * rec.AllocPct / 100.0
}

// IsOverAllocated returns true if the sum of allocation percentages exceeds 100%.
func (p *Portfolio) IsOverAllocated() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.AllocatedPct > 100.0
}

// Snapshot returns a read-only copy of the portfolio state.
func (p *Portfolio) Snapshot() PortfolioSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	allocCopy := make(map[string]AllocationRecord, len(p.Allocations))
	for k, v := range p.Allocations {
		allocCopy[k] = *v
	}
	return PortfolioSnapshot{
		PortfolioID:     p.PortfolioID,
		Name:            p.Name,
		Type:            p.Type,
		ParentID:        p.ParentID,
		Status:          p.Status,
		InitialNAV:      p.InitialNAV,
		CurrentNAV:      p.CurrentNAV,
		CashReserve:     p.CashReserve,
		AllocatedPct:    p.AllocatedPct,
		Allocations:     allocCopy,
		RiskBudget:      p.RiskBudget,
		SubPortfolioIDs: append([]string(nil), p.SubPortfolioIDs...),
		Version:         p.Version,
		UpdatedAt:       p.UpdatedAt,
	}
}

// PortfolioSnapshot is an immutable read-only view.
type PortfolioSnapshot struct {
	PortfolioID     string
	Name            string
	Type            PortfolioType
	ParentID        string
	Status          PortfolioStatus
	InitialNAV      float64
	CurrentNAV      float64
	CashReserve     float64
	AllocatedPct    float64
	Allocations     map[string]AllocationRecord
	RiskBudget      RiskBudget
	SubPortfolioIDs []string
	Version         int64
	UpdatedAt       time.Time
}

// ReplayPortfolio rebuilds a portfolio aggregate from an ordered event slice.
func ReplayPortfolio(events []ledger.Event) (*Portfolio, error) {
	if len(events) == 0 {
		return nil, errors.New("pms: no events to replay")
	}
	p := NewPortfolio(events[0].AggregateID)
	for _, ev := range events {
		if err := p.ApplyEvent(ev); err != nil {
			return nil, fmt.Errorf("pms: replay at seq %d: %w", ev.SequenceNo, err)
		}
	}
	return p, nil
}

// ── PortfolioManager ──────────────────────────────────────────────────────────

// PortfolioManager is the in-process registry of all active portfolios.
// It is the single capital-allocation authority above OMS and strategies.
type PortfolioManager struct {
	mu         sync.RWMutex
	portfolios map[string]*Portfolio
	store      ledger.Store
}

// NewPortfolioManager constructs a PortfolioManager backed by a ledger store.
func NewPortfolioManager(store ledger.Store) *PortfolioManager {
	return &PortfolioManager{
		portfolios: make(map[string]*Portfolio),
		store:      store,
	}
}

// CreatePortfolio creates and registers a new portfolio, emitting a creation event.
func (m *PortfolioManager) CreatePortfolio(ctx context.Context, payload PortfolioCreatedPayload) (*Portfolio, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.portfolios[payload.PortfolioID]; exists {
		return nil, fmt.Errorf("pms: portfolio %s already exists", payload.PortfolioID)
	}
	p := NewPortfolio(payload.PortfolioID)
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   payload.PortfolioID,
		EventType:     EventPortfolioCreated,
		AccountID:     payload.PortfolioID,
		Payload:       payload,
		Source:        "pms.manager",
		CreatedAt:     payload.CreatedAt,
	})
	if err != nil {
		return nil, err
	}
	if err := p.ApplyEvent(ev); err != nil {
		return nil, err
	}
	if m.store != nil {
		m.store.Append(ctx, ev) //nolint:errcheck
	}
	m.portfolios[payload.PortfolioID] = p
	return p, nil
}

// Get returns the portfolio aggregate for the given ID.
func (m *PortfolioManager) Get(portfolioID string) (*Portfolio, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.portfolios[portfolioID]
	if !ok {
		return nil, fmt.Errorf("pms: portfolio %s not found", portfolioID)
	}
	return p, nil
}

// BudgetFor is a safe convenience accessor for strategy capital budgets.
func (m *PortfolioManager) BudgetFor(portfolioID, strategyID string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.portfolios[portfolioID]
	if !ok {
		return 0
	}
	return p.BudgetFor(strategyID)
}

// UpdateNAV updates a portfolio's NAV and emits a NAV-updated event.
func (m *PortfolioManager) UpdateNAV(ctx context.Context, portfolioID string, newNAV float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.portfolios[portfolioID]
	if !ok {
		return fmt.Errorf("pms: portfolio %s not found", portfolioID)
	}
	snap := p.Snapshot()
	payload := PortfolioNAVUpdatedPayload{
		PortfolioID: portfolioID,
		PreviousNAV: snap.CurrentNAV,
		CurrentNAV:  newNAV,
		DeltaUSD:    newNAV - snap.CurrentNAV,
		DeltaPct:    (newNAV - snap.CurrentNAV) / snap.CurrentNAV * 100,
		ComputedAt:  time.Now().UTC(),
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   portfolioID,
		EventType:     EventPortfolioNAVUpdated,
		AccountID:     portfolioID,
		Payload:       payload,
		Source:        "pms.manager",
	})
	if err != nil {
		return err
	}
	if err := p.ApplyEvent(ev); err != nil {
		return err
	}
	if m.store != nil {
		m.store.Append(ctx, ev) //nolint:errcheck
	}
	return nil
}

// All returns snapshots of all managed portfolios.
func (m *PortfolioManager) All() []PortfolioSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PortfolioSnapshot, 0, len(m.portfolios))
	for _, p := range m.portfolios {
		out = append(out, p.Snapshot())
	}
	return out
}

// LoadFromEvents bootstraps the manager by replaying a slice of events.
func (m *PortfolioManager) LoadFromEvents(events []ledger.Event) error {
	byAggregate := make(map[string][]ledger.Event)
	for _, ev := range events {
		if ev.AggregateType == AggregatePortfolio {
			byAggregate[ev.AggregateID] = append(byAggregate[ev.AggregateID], ev)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, evts := range byAggregate {
		p, err := ReplayPortfolio(evts)
		if err != nil {
			return fmt.Errorf("pms: load portfolio %s: %w", id, err)
		}
		m.portfolios[id] = p
	}
	return nil
}

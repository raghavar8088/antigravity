package pms

import (
	"encoding/json"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── AllocationProjectionView ──────────────────────────────────────────────────

// AllocationProjectionView tracks the current allocation state across all portfolios.
type AllocationProjectionView struct {
	// portfolioID → strategyID → allocation
	entries map[string]map[string]AllocationProjection
}

func NewAllocationProjectionView() *AllocationProjectionView {
	return &AllocationProjectionView{entries: make(map[string]map[string]AllocationProjection)}
}

func (v *AllocationProjectionView) Apply(ev ledger.Event) error {
	switch ev.EventType {
	case EventAllocationCreated:
		var p AllocationCreatedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("alloc_projection: %w", err)
		}
		if v.entries[p.PortfolioID] == nil {
			v.entries[p.PortfolioID] = make(map[string]AllocationProjection)
		}
		v.entries[p.PortfolioID][p.StrategyID] = AllocationProjection{
			StrategyID:   p.StrategyID,
			StrategyName: p.StrategyName,
			AllocPct:     p.AllocPct,
			AllocUSD:     p.AllocUSD,
			Method:       p.Method,
			UpdatedAt:    ev.CreatedAt,
		}
	case EventAllocationChanged:
		var p AllocationChangedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("alloc_projection: %w", err)
		}
		if m := v.entries[p.PortfolioID]; m != nil {
			if a, ok := m[p.StrategyID]; ok {
				a.AllocPct = p.NewPct
				a.AllocUSD = p.NewUSD
				a.UpdatedAt = ev.CreatedAt
				m[p.StrategyID] = a
			}
		}
	case EventAllocationRemoved:
		var p AllocationRemovedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("alloc_projection: %w", err)
		}
		if m := v.entries[p.PortfolioID]; m != nil {
			delete(m, p.StrategyID)
		}
	}
	return nil
}

// GetAllocations returns all strategy allocations for a portfolio.
func (v *AllocationProjectionView) GetAllocations(portfolioID string) []AllocationProjection {
	m := v.entries[portfolioID]
	out := make([]AllocationProjection, 0, len(m))
	for _, a := range m {
		out = append(out, a)
	}
	return out
}

// ── RiskBudgetProjectionView ──────────────────────────────────────────────────

// RiskBudgetProjectionView tracks the declared risk budget per portfolio.
type RiskBudgetProjectionView struct {
	budgets map[string]RiskBudget // portfolioID → RiskBudget
}

func NewRiskBudgetProjectionView() *RiskBudgetProjectionView {
	return &RiskBudgetProjectionView{budgets: make(map[string]RiskBudget)}
}

func (v *RiskBudgetProjectionView) Apply(ev ledger.Event) error {
	switch ev.EventType {
	case EventRiskBudgetCreated, EventRiskBudgetChanged:
		var p RiskBudgetCreatedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("risk_budget_projection: %w", err)
		}
		v.budgets[p.PortfolioID] = RiskBudget{
			MaxHeatPct:        p.MaxHeatPct,
			MaxVaR95Pct:       p.MaxVaR95Pct,
			MaxCVaR95Pct:      p.MaxCVaR95Pct,
			MaxDrawdownPct:    p.MaxDrawdownPct,
			MaxDailyLossPct:   p.MaxDailyLossPct,
			MaxWeeklyLossPct:  p.MaxWeeklyLossPct,
			MaxMonthlyLossPct: p.MaxMonthlyLossPct,
			MaxGrossExpPct:    p.MaxGrossExpPct,
			MaxNetExpPct:      p.MaxNetExpPct,
		}
	}
	return nil
}

func (v *RiskBudgetProjectionView) Get(portfolioID string) (RiskBudget, bool) {
	b, ok := v.budgets[portfolioID]
	return b, ok
}

// ── ExposureProjectionView ────────────────────────────────────────────────────

// ExposureProjectionRecord is a point-in-time exposure observation.
type ExposureProjectionRecord struct {
	PortfolioID  string
	GrossExpPct  float64
	NetExpPct    float64
	RecordedAt   time.Time
}

// ExposureProjectionView tracks exposure threshold breach events.
type ExposureProjectionView struct {
	breaches []ExposureProjectionRecord
}

func NewExposureProjectionView() *ExposureProjectionView {
	return &ExposureProjectionView{}
}

func (v *ExposureProjectionView) Apply(ev ledger.Event) error {
	if ev.EventType == EventExposureThresholdExceeded {
		var p ExposureThresholdExceededPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("exposure_projection: %w", err)
		}
		v.breaches = append(v.breaches, ExposureProjectionRecord{
			PortfolioID: p.PortfolioID,
			GrossExpPct: p.CurrentPct,
			RecordedAt:  ev.CreatedAt,
		})
	}
	return nil
}

func (v *ExposureProjectionView) Breaches(portfolioID string) []ExposureProjectionRecord {
	out := make([]ExposureProjectionRecord, 0)
	for _, b := range v.breaches {
		if b.PortfolioID == portfolioID {
			out = append(out, b)
		}
	}
	return out
}

// ── AccountProjectionView ─────────────────────────────────────────────────────

// AccountProjectionRecord is the read-model for one account.
type AccountProjectionRecord struct {
	AccountID   string
	Name        string
	Type        string
	PortfolioID string
	Status      string
	InitialNAV  float64
	CreatedAt   time.Time
	ClosedAt    time.Time
}

// AccountProjectionView projects account lifecycle events.
type AccountProjectionView struct {
	accounts map[string]*AccountProjectionRecord
}

func NewAccountProjectionView() *AccountProjectionView {
	return &AccountProjectionView{accounts: make(map[string]*AccountProjectionRecord)}
}

func (v *AccountProjectionView) Apply(ev ledger.Event) error {
	switch ev.EventType {
	case EventAccountCreated:
		var p AccountCreatedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("account_projection: %w", err)
		}
		v.accounts[p.AccountID] = &AccountProjectionRecord{
			AccountID:   p.AccountID,
			Name:        p.Name,
			Type:        p.Type,
			PortfolioID: p.PortfolioID,
			Status:      "ACTIVE",
			InitialNAV:  p.InitialNAV,
			CreatedAt:   ev.CreatedAt,
		}
	case EventAccountClosed:
		var p AccountClosedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("account_projection: %w", err)
		}
		if a := v.accounts[p.AccountID]; a != nil {
			a.Status = "CLOSED"
			a.ClosedAt = ev.CreatedAt
		}
	}
	return nil
}

func (v *AccountProjectionView) All() []AccountProjectionRecord {
	out := make([]AccountProjectionRecord, 0, len(v.accounts))
	for _, a := range v.accounts {
		out = append(out, *a)
	}
	return out
}

// ── PerformanceProjectionView ─────────────────────────────────────────────────

// NAVDataPoint is a time-series observation of portfolio NAV.
type NAVDataPoint struct {
	PortfolioID string
	NAV         float64
	DeltaPct    float64
	RecordedAt  time.Time
}

// PerformanceProjectionView builds a NAV time series from portfolio events.
type PerformanceProjectionView struct {
	navSeries map[string][]NAVDataPoint // portfolioID → time series
}

func NewPerformanceProjectionView() *PerformanceProjectionView {
	return &PerformanceProjectionView{navSeries: make(map[string][]NAVDataPoint)}
}

func (v *PerformanceProjectionView) Apply(ev ledger.Event) error {
	if ev.EventType == EventPortfolioNAVUpdated {
		var p PortfolioNAVUpdatedPayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return fmt.Errorf("perf_projection: %w", err)
		}
		v.navSeries[p.PortfolioID] = append(v.navSeries[p.PortfolioID], NAVDataPoint{
			PortfolioID: p.PortfolioID,
			NAV:         p.CurrentNAV,
			DeltaPct:    p.DeltaPct,
			RecordedAt:  p.ComputedAt,
		})
	}
	return nil
}

func (v *PerformanceProjectionView) NAVSeries(portfolioID string) []NAVDataPoint {
	return append([]NAVDataPoint(nil), v.navSeries[portfolioID]...)
}

// DailyReturns extracts the daily return fractions from the NAV series.
func (v *PerformanceProjectionView) DailyReturns(portfolioID string) []float64 {
	series := v.navSeries[portfolioID]
	out := make([]float64, 0, len(series))
	for _, dp := range series {
		out = append(out, dp.DeltaPct/100.0)
	}
	return out
}

// ── PMSProjectionSet — composite projection set ───────────────────────────────

// PMSProjectionSet holds all PMS projections and dispatches events to each.
// This is the primary CQRS read-model used by query handlers.
type PMSProjectionSet struct {
	Portfolio   *PortfolioProjection
	Allocation  *AllocationProjectionView
	RiskBudget  *RiskBudgetProjectionView
	Exposure    *ExposureProjectionView
	Account     *AccountProjectionView
	Performance *PerformanceProjectionView
}

// NewPMSProjectionSet creates a fully initialised projection set.
func NewPMSProjectionSet() *PMSProjectionSet {
	return &PMSProjectionSet{
		Portfolio:   NewPortfolioProjection(),
		Allocation:  NewAllocationProjectionView(),
		RiskBudget:  NewRiskBudgetProjectionView(),
		Exposure:    NewExposureProjectionView(),
		Account:     NewAccountProjectionView(),
		Performance: NewPerformanceProjectionView(),
	}
}

// Apply dispatches one event to all projections. O(1) per event.
func (s *PMSProjectionSet) Apply(ev ledger.Event) error {
	if err := s.Portfolio.Apply(ev); err != nil {
		return err
	}
	if err := s.Allocation.Apply(ev); err != nil {
		return err
	}
	if err := s.RiskBudget.Apply(ev); err != nil {
		return err
	}
	if err := s.Exposure.Apply(ev); err != nil {
		return err
	}
	if err := s.Account.Apply(ev); err != nil {
		return err
	}
	if err := s.Performance.Apply(ev); err != nil {
		return err
	}
	return nil
}

// ReplayAll rebuilds all projections in a single O(n) pass over the event stream.
func (s *PMSProjectionSet) ReplayAll(events []ledger.Event) error {
	for _, ev := range events {
		if err := s.Apply(ev); err != nil {
			return fmt.Errorf("pms projection set replay at seq %d: %w", ev.SequenceNo, err)
		}
	}
	return nil
}

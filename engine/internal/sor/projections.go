package sor

import (
	"encoding/json"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── RoutingProjection ─────────────────────────────────────────────────────────

// RoutingRecord is the read-model for one parent routing decision.
type RoutingRecord struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	Symbol              string    `json:"symbol"`
	Side                string    `json:"side"`
	RequestedQty        float64   `json:"requested_qty"`
	Algo                string    `json:"algo"`
	WinningVenue        VenueID   `json:"winning_venue"`
	SplitMethod         string    `json:"split_method"`
	ChildCount          int       `json:"child_count"`
	FilledQty           float64   `json:"filled_qty"`
	AvgPrice            float64   `json:"avg_price"`
	Complete            bool      `json:"complete"`
	State               string    `json:"state"` // CREATED|PLANNED|EXECUTING|COMPLETED|CANCELLED
	CreatedAt           time.Time `json:"created_at"`
	CompletedAt         time.Time `json:"completed_at"`
}

// RoutingProjection rebuilds routing records from the event stream.
type RoutingProjection struct {
	records map[string]*RoutingRecord
}

func NewRoutingProjection() *RoutingProjection {
	return &RoutingProjection{records: make(map[string]*RoutingRecord)}
}

func (p *RoutingProjection) Apply(ev ledger.Event) error {
	if ev.AggregateType != AggregateRoute {
		return nil
	}
	switch ev.EventType {
	case EventRouteCreated:
		var pl RouteCreatedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("routing_projection: %w", err)
		}
		p.records[pl.ParentClientOrderID] = &RoutingRecord{
			ParentClientOrderID: pl.ParentClientOrderID,
			Symbol:              pl.Symbol,
			Side:                pl.Side,
			RequestedQty:        pl.Quantity,
			Algo:                pl.Algo,
			State:               "CREATED",
			CreatedAt:           pl.CreatedAt,
		}
	case EventBestExecutionCalculated:
		var pl BestExecutionCalculatedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("routing_projection: %w", err)
		}
		if r := p.records[pl.ParentClientOrderID]; r != nil {
			r.WinningVenue = pl.WinningVenue
		}
	case EventExecutionPlanned:
		var pl ExecutionPlannedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("routing_projection: %w", err)
		}
		if r := p.records[pl.ParentClientOrderID]; r != nil {
			r.SplitMethod = pl.SplitMethod
			r.ChildCount = pl.ChildCount
			r.State = "PLANNED"
		}
	case EventExecutionStarted:
		var pl ExecutionStartedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("routing_projection: %w", err)
		}
		if r := p.records[pl.ParentClientOrderID]; r != nil {
			r.State = "EXECUTING"
		}
	case EventExecutionCompleted:
		var pl ExecutionCompletedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("routing_projection: %w", err)
		}
		if r := p.records[pl.ParentClientOrderID]; r != nil {
			r.FilledQty = pl.FilledQty
			r.AvgPrice = pl.AvgPrice
			r.Complete = pl.FilledQty >= pl.RequestedQty*0.99
			r.State = "COMPLETED"
			r.CompletedAt = pl.CompletedAt
		}
	case EventExecutionCancelled:
		var pl ExecutionCancelledPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("routing_projection: %w", err)
		}
		if r := p.records[pl.ParentClientOrderID]; r != nil {
			r.State = "CANCELLED"
			r.FilledQty = pl.FilledQty
		}
	}
	return nil
}

func (p *RoutingProjection) Get(parentID string) (*RoutingRecord, bool) {
	r, ok := p.records[parentID]
	return r, ok
}

func (p *RoutingProjection) All() []RoutingRecord {
	out := make([]RoutingRecord, 0, len(p.records))
	for _, r := range p.records {
		out = append(out, *r)
	}
	return out
}

// ── VenueProjection ───────────────────────────────────────────────────────────

// VenueRecord is the read-model for one venue's activity & health.
type VenueRecord struct {
	VenueID       VenueID   `json:"venue_id"`
	HealthScore   float64   `json:"health_score"`
	Status        string    `json:"status"`
	Selections    int64     `json:"selections"`
	Rejections    int64     `json:"rejections"`
	Failures      int64     `json:"failures"`
	Recoveries    int64     `json:"recoveries"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// VenueProjection rebuilds venue activity from the event stream.
type VenueProjection struct {
	records map[VenueID]*VenueRecord
}

func NewVenueProjection() *VenueProjection {
	return &VenueProjection{records: make(map[VenueID]*VenueRecord)}
}

func (p *VenueProjection) rec(id VenueID) *VenueRecord {
	r, ok := p.records[id]
	if !ok {
		r = &VenueRecord{VenueID: id, HealthScore: 100, Status: "ACTIVE"}
		p.records[id] = r
	}
	return r
}

func (p *VenueProjection) Apply(ev ledger.Event) error {
	switch ev.EventType {
	case EventVenueSelected:
		var pl VenueSelectedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("venue_projection: %w", err)
		}
		r := p.rec(pl.VenueID)
		r.Selections++
		r.LastUpdatedAt = pl.SelectedAt
	case EventVenueRejected:
		var pl VenueRejectedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("venue_projection: %w", err)
		}
		r := p.rec(pl.VenueID)
		r.Rejections++
		r.LastUpdatedAt = pl.RejectedAt
	case EventVenueFailed:
		var pl VenueFailedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("venue_projection: %w", err)
		}
		r := p.rec(pl.VenueID)
		r.Failures++
		r.HealthScore = pl.HealthScore
		r.Status = "DOWN"
		r.LastUpdatedAt = pl.FailedAt
	case EventVenueRecovered:
		var pl VenueRecoveredPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("venue_projection: %w", err)
		}
		r := p.rec(pl.VenueID)
		r.Recoveries++
		r.HealthScore = pl.HealthScore
		r.Status = "ACTIVE"
		r.LastUpdatedAt = pl.RecoveredAt
	case EventHealthScoreChanged:
		var pl HealthScoreChangedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("venue_projection: %w", err)
		}
		r := p.rec(pl.VenueID)
		r.HealthScore = pl.NewScore
		r.Status = pl.Status
		r.LastUpdatedAt = pl.ChangedAt
	}
	return nil
}

func (p *VenueProjection) All() []VenueRecord {
	out := make([]VenueRecord, 0, len(p.records))
	for _, r := range p.records {
		out = append(out, *r)
	}
	return out
}

// ── LiquidityProjection ───────────────────────────────────────────────────────

// LiquidityProjection tracks per-venue fill counts as a proxy for realized
// executable liquidity (where orders actually got done).
type LiquidityProjection struct {
	fillsByVenue map[VenueID]int64
	qtyByVenue   map[VenueID]float64
}

func NewLiquidityProjection() *LiquidityProjection {
	return &LiquidityProjection{
		fillsByVenue: make(map[VenueID]int64),
		qtyByVenue:   make(map[VenueID]float64),
	}
}

func (p *LiquidityProjection) Apply(ev ledger.Event) error {
	if ev.EventType == EventChildFilled {
		var pl ChildFilledPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("liquidity_projection: %w", err)
		}
		p.fillsByVenue[pl.VenueID]++
		p.qtyByVenue[pl.VenueID] += pl.FilledQty
	}
	return nil
}

func (p *LiquidityProjection) FilledQty(v VenueID) float64 { return p.qtyByVenue[v] }
func (p *LiquidityProjection) Fills(v VenueID) int64       { return p.fillsByVenue[v] }

// ── ExecutionProjection ───────────────────────────────────────────────────────

// ExecutionProjection tracks child-fill records for execution analytics.
type ExecutionProjection struct {
	fills []ChildFilledPayload
}

func NewExecutionProjection() *ExecutionProjection {
	return &ExecutionProjection{}
}

func (p *ExecutionProjection) Apply(ev ledger.Event) error {
	if ev.EventType == EventChildFilled {
		var pl ChildFilledPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("execution_projection: %w", err)
		}
		p.fills = append(p.fills, pl)
	}
	return nil
}

func (p *ExecutionProjection) Fills() []ChildFilledPayload { return p.fills }

// ── HealthProjection ──────────────────────────────────────────────────────────

// HealthProjection tracks the latest health score per venue from events.
type HealthProjection struct {
	scores map[VenueID]float64
	status map[VenueID]string
}

func NewHealthProjection() *HealthProjection {
	return &HealthProjection{
		scores: make(map[VenueID]float64),
		status: make(map[VenueID]string),
	}
}

func (p *HealthProjection) Apply(ev ledger.Event) error {
	if ev.EventType == EventHealthScoreChanged {
		var pl HealthScoreChangedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("health_projection: %w", err)
		}
		p.scores[pl.VenueID] = pl.NewScore
		p.status[pl.VenueID] = pl.Status
	}
	return nil
}

func (p *HealthProjection) Score(v VenueID) (float64, bool) {
	s, ok := p.scores[v]
	return s, ok
}

// ── CostProjection ────────────────────────────────────────────────────────────

// CostRecord aggregates realized execution costs/slippage per parent order.
type CostRecord struct {
	ParentClientOrderID string
	TotalFeesUSD        float64
	RealizedSlippageBps float64
	VenuesUsed          int
}

// CostProjection tracks realized cost/slippage from completed executions.
type CostProjection struct {
	records       map[string]*CostRecord
	totalFeesUSD  float64
	slippageSum   float64
	slippageCount int64
}

func NewCostProjection() *CostProjection {
	return &CostProjection{records: make(map[string]*CostRecord)}
}

func (p *CostProjection) Apply(ev ledger.Event) error {
	if ev.EventType == EventExecutionCompleted {
		var pl ExecutionCompletedPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil {
			return fmt.Errorf("cost_projection: %w", err)
		}
		p.records[pl.ParentClientOrderID] = &CostRecord{
			ParentClientOrderID: pl.ParentClientOrderID,
			TotalFeesUSD:        pl.TotalFeesUSD,
			RealizedSlippageBps: pl.RealizedSlippageBps,
			VenuesUsed:          len(pl.VenuesUsed),
		}
		p.totalFeesUSD += pl.TotalFeesUSD
		p.slippageSum += pl.RealizedSlippageBps
		p.slippageCount++
	}
	return nil
}

func (p *CostProjection) TotalFeesUSD() float64 { return p.totalFeesUSD }
func (p *CostProjection) AvgSlippageBps() float64 {
	if p.slippageCount == 0 {
		return 0
	}
	return p.slippageSum / float64(p.slippageCount)
}

// ── SORProjectionSet — composite ──────────────────────────────────────────────

// SORProjectionSet holds all SOR projections and dispatches events to each.
type SORProjectionSet struct {
	Routing   *RoutingProjection
	Venue     *VenueProjection
	Liquidity *LiquidityProjection
	Execution *ExecutionProjection
	Health    *HealthProjection
	Cost      *CostProjection
}

// NewSORProjectionSet creates a fully initialised projection set.
func NewSORProjectionSet() *SORProjectionSet {
	return &SORProjectionSet{
		Routing:   NewRoutingProjection(),
		Venue:     NewVenueProjection(),
		Liquidity: NewLiquidityProjection(),
		Execution: NewExecutionProjection(),
		Health:    NewHealthProjection(),
		Cost:      NewCostProjection(),
	}
}

// Apply dispatches one event to all projections. O(1) per event.
func (s *SORProjectionSet) Apply(ev ledger.Event) error {
	if err := s.Routing.Apply(ev); err != nil {
		return err
	}
	if err := s.Venue.Apply(ev); err != nil {
		return err
	}
	if err := s.Liquidity.Apply(ev); err != nil {
		return err
	}
	if err := s.Execution.Apply(ev); err != nil {
		return err
	}
	if err := s.Health.Apply(ev); err != nil {
		return err
	}
	if err := s.Cost.Apply(ev); err != nil {
		return err
	}
	return nil
}

// ReplayAll rebuilds all projections in a single O(n) pass.
func (s *SORProjectionSet) ReplayAll(events []ledger.Event) error {
	for _, ev := range events {
		if err := s.Apply(ev); err != nil {
			return fmt.Errorf("sor projection set replay at seq %d: %w", ev.SequenceNo, err)
		}
	}
	return nil
}

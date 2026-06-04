package sor

import (
	"context"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── SOR Aggregate Types ───────────────────────────────────────────────────────

const (
	AggregateRoute ledger.AggregateType = "SOR_ROUTE" // keyed by parent ClientOrderID
	AggregateVenue ledger.AggregateType = "SOR_VENUE" // keyed by VenueID
)

// ── SOR Event Types ───────────────────────────────────────────────────────────

const (
	EventRouteCreated            ledger.EventType = "SOR_ROUTE_CREATED"
	EventVenueSelected           ledger.EventType = "SOR_VENUE_SELECTED"
	EventExecutionPlanned        ledger.EventType = "SOR_EXECUTION_PLANNED"
	EventOrderSplit              ledger.EventType = "SOR_ORDER_SPLIT"
	EventVenueRejected           ledger.EventType = "SOR_VENUE_REJECTED"
	EventVenueFailed             ledger.EventType = "SOR_VENUE_FAILED"
	EventVenueRecovered          ledger.EventType = "SOR_VENUE_RECOVERED"
	EventExecutionStarted        ledger.EventType = "SOR_EXECUTION_STARTED"
	EventExecutionCompleted      ledger.EventType = "SOR_EXECUTION_COMPLETED"
	EventExecutionCancelled      ledger.EventType = "SOR_EXECUTION_CANCELLED"
	EventBestExecutionCalculated ledger.EventType = "SOR_BEST_EXECUTION_CALCULATED"
	EventSlippageRecorded        ledger.EventType = "SOR_SLIPPAGE_RECORDED"
	EventHealthScoreChanged      ledger.EventType = "SOR_HEALTH_SCORE_CHANGED"
	EventChildFilled             ledger.EventType = "SOR_CHILD_FILLED"
)

// ── Payloads ──────────────────────────────────────────────────────────────────

type RouteCreatedPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	Symbol              string    `json:"symbol"`
	Side                string    `json:"side"`
	Quantity            float64   `json:"quantity"`
	OrderType           string    `json:"order_type"`
	Algo                string    `json:"algo"`
	Urgency             string    `json:"urgency"`
	StrategyName        string    `json:"strategy_name"`
	CreatedAt           time.Time `json:"created_at"`
}

type VenueSelectedPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	Symbol              string    `json:"symbol"`
	VenueID             VenueID   `json:"venue_id"`
	Rank                int       `json:"rank"`
	Score               float64   `json:"score"`
	Reason              string    `json:"reason"`
	SelectedAt          time.Time `json:"selected_at"`
}

type BestExecutionCalculatedPayload struct {
	ParentClientOrderID string                `json:"parent_client_order_id"`
	Symbol              string                `json:"symbol"`
	Side                string                `json:"side"`
	Quantity            float64               `json:"quantity"`
	Scores              []BestExecutionScore  `json:"scores"`
	WinningVenue        VenueID               `json:"winning_venue"`
	CalculatedAt        time.Time             `json:"calculated_at"`
}

type ExecutionPlannedPayload struct {
	ParentClientOrderID string         `json:"parent_client_order_id"`
	Symbol              string         `json:"symbol"`
	Algo                string         `json:"algo"`
	SplitMethod         string         `json:"split_method"`
	ChildCount          int            `json:"child_count"`
	VenueAllocations    map[VenueID]float64 `json:"venue_allocations"`
	PlannedAt           time.Time      `json:"planned_at"`
}

type OrderSplitPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	ChildOrderID        string    `json:"child_order_id"`
	VenueID             VenueID   `json:"venue_id"`
	Symbol              string    `json:"symbol"`
	Side                string    `json:"side"`
	Quantity            float64   `json:"quantity"`
	SequenceIndex       int       `json:"sequence_index"`
	ScheduledAt         time.Time `json:"scheduled_at"`
}

type VenueRejectedPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	ChildOrderID        string    `json:"child_order_id"`
	VenueID             VenueID   `json:"venue_id"`
	Reason              string    `json:"reason"`
	RejectedAt          time.Time `json:"rejected_at"`
}

type VenueFailedPayload struct {
	VenueID    VenueID   `json:"venue_id"`
	Reason     string    `json:"reason"`
	HealthScore float64  `json:"health_score"`
	FailedAt   time.Time `json:"failed_at"`
}

type VenueRecoveredPayload struct {
	VenueID     VenueID   `json:"venue_id"`
	HealthScore float64   `json:"health_score"`
	RecoveredAt time.Time `json:"recovered_at"`
}

type ExecutionStartedPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	ChildCount          int       `json:"child_count"`
	StartedAt           time.Time `json:"started_at"`
}

type ExecutionCompletedPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	Symbol              string    `json:"symbol"`
	RequestedQty        float64   `json:"requested_qty"`
	FilledQty           float64   `json:"filled_qty"`
	AvgPrice            float64   `json:"avg_price"`
	TotalFeesUSD        float64   `json:"total_fees_usd"`
	RealizedSlippageBps float64   `json:"realized_slippage_bps"`
	VenuesUsed          []VenueID `json:"venues_used"`
	DurationMs          int64     `json:"duration_ms"`
	CompletedAt         time.Time `json:"completed_at"`
}

type ExecutionCancelledPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	Reason              string    `json:"reason"`
	FilledQty           float64   `json:"filled_qty"`
	CancelledAt         time.Time `json:"cancelled_at"`
}

type ChildFilledPayload struct {
	ParentClientOrderID string    `json:"parent_client_order_id"`
	ChildOrderID        string    `json:"child_order_id"`
	VenueID             VenueID   `json:"venue_id"`
	Symbol              string    `json:"symbol"`
	Side                string    `json:"side"`
	FilledQty           float64   `json:"filled_qty"`
	AvgPrice            float64   `json:"avg_price"`
	FeesUSD             float64   `json:"fees_usd"`
	SlippageBps         float64   `json:"slippage_bps"`
	FilledAt            time.Time `json:"filled_at"`
}

type SlippageRecordedPayload struct {
	VenueID         VenueID   `json:"venue_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	Quantity        float64   `json:"quantity"`
	ExpectedBps     float64   `json:"expected_bps"`
	RealizedBps     float64   `json:"realized_bps"`
	ImplementationShortfallBps float64 `json:"implementation_shortfall_bps"`
	RecordedAt      time.Time `json:"recorded_at"`
}

type HealthScoreChangedPayload struct {
	VenueID      VenueID   `json:"venue_id"`
	PreviousScore float64  `json:"previous_score"`
	NewScore     float64   `json:"new_score"`
	Status       string    `json:"status"`
	Trigger      string    `json:"trigger"`
	ChangedAt    time.Time `json:"changed_at"`
}

// ── Emission helpers ──────────────────────────────────────────────────────────

func emit(ctx context.Context, store ledger.Store, input ledger.NewEventInput) {
	if store == nil {
		return
	}
	ev, err := ledger.NewEvent(input)
	if err != nil {
		return
	}
	store.Append(ctx, ev) //nolint:errcheck
}

func emitRoute(ctx context.Context, store ledger.Store, parentID string, et ledger.EventType, symbol, strategy string, payload any) {
	emit(ctx, store, ledger.NewEventInput{
		AggregateType: AggregateRoute,
		AggregateID:   parentID,
		EventType:     et,
		Symbol:        symbol,
		StrategyID:    strategy,
		Payload:       payload,
		Source:        "sor",
	})
}

func emitVenue(ctx context.Context, store ledger.Store, venueID VenueID, et ledger.EventType, payload any) {
	emit(ctx, store, ledger.NewEventInput{
		AggregateType: AggregateVenue,
		AggregateID:   string(venueID),
		EventType:     et,
		Payload:       payload,
		Source:        "sor.venue",
	})
}

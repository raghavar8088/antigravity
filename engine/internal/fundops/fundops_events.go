// Package fundops implements Phase 20 — Fund Operations Layer.
// It is the administrative authority above PMS: managing investors, NAV,
// capital flows, tax lots, fees, compliance, and audit for the entire fund.
//
// Every fund operation emits an immutable, hash-verified event. The event
// log is the single source of truth for all fund accounting state.
package fundops

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Schema ───────────────────────────────────────────────────────────────────

const SchemaVersion = "fo1"

// ─── Event Types ──────────────────────────────────────────────────────────────

type EventType string

const (
	// Fund lifecycle
	EvtFundCreated EventType = "FUND_CREATED"
	EvtFundClosed  EventType = "FUND_CLOSED"
	EvtFundUpdated EventType = "FUND_UPDATED"

	// Investor lifecycle
	EvtInvestorCreated EventType = "INVESTOR_CREATED"
	EvtInvestorUpdated EventType = "INVESTOR_UPDATED"
	EvtInvestorClosed  EventType = "INVESTOR_CLOSED"

	// Capital flows
	EvtCapitalSubscribed  EventType = "CAPITAL_SUBSCRIBED"
	EvtCapitalRedeemed    EventType = "CAPITAL_REDEEMED"
	EvtCapitalTransferred EventType = "CAPITAL_TRANSFERRED"
	EvtDistributionPaid   EventType = "DISTRIBUTION_PAID"
	EvtCapitalCallIssued  EventType = "CAPITAL_CALL_ISSUED"

	// NAV
	EvtNAVCalculated EventType = "NAV_CALCULATED"

	// Fees
	EvtFeeAccrued EventType = "FEE_ACCRUED"
	EvtFeePaid    EventType = "FEE_PAID"
	EvtHWMUpdated EventType = "HIGH_WATER_MARK_UPDATED"

	// Tax lots
	EvtTaxLotCreated EventType = "TAX_LOT_CREATED"
	EvtTaxLotClosed  EventType = "TAX_LOT_CLOSED"
	EvtTaxLotPartial EventType = "TAX_LOT_PARTIAL_CLOSE"

	// Compliance
	EvtCompliancePass      EventType = "COMPLIANCE_PASS"
	EvtComplianceViolation EventType = "COMPLIANCE_VIOLATION"
	EvtComplianceCleared   EventType = "COMPLIANCE_CLEARED"

	// Audit & Reporting
	EvtAuditExportGenerated    EventType = "AUDIT_EXPORT_GENERATED"
	EvtInvestorReportGenerated EventType = "INVESTOR_REPORT_GENERATED"

	// Attribution
	EvtAttributionCalculated EventType = "ATTRIBUTION_CALCULATED"
)

// AggregateType identifies the fund operations aggregate.
type AggregateType string

const (
	AggFund       AggregateType = "FUND"
	AggInvestor   AggregateType = "INVESTOR"
	AggCapital    AggregateType = "CAPITAL"
	AggNAV        AggregateType = "NAV"
	AggFee        AggregateType = "FEE"
	AggTaxLot     AggregateType = "TAX_LOT"
	AggCompliance AggregateType = "COMPLIANCE"
	AggAudit      AggregateType = "AUDIT"
	AggReport     AggregateType = "REPORT"
)

// ─── Core Event ───────────────────────────────────────────────────────────────

// FundEvent is an immutable fund operations event.
type FundEvent struct {
	EventID        string        `json:"event_id"`
	Schema         string        `json:"schema"`
	AggregateType  AggregateType `json:"aggregate_type"`
	AggregateID    string        `json:"aggregate_id"`
	FundID         string        `json:"fund_id"`
	EventType      EventType     `json:"event_type"`
	CorrelationID  string        `json:"correlation_id,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	PayloadHash    string        `json:"payload_hash"`
	SequenceNo     int64         `json:"sequence_no"`
	CreatedAt      time.Time     `json:"created_at"`
}

func (e FundEvent) ValidateHash() bool {
	h := sha256.Sum256(e.Payload)
	return hex.EncodeToString(h[:]) == e.PayloadHash
}

// NewEventInput carries all parameters to create a FundEvent.
type NewEventInput struct {
	AggregateType  AggregateType
	AggregateID    string
	FundID         string
	EventType      EventType
	CorrelationID  string
	IdempotencyKey string
	Payload        any
	CreatedAt      time.Time
}

// NewFundEvent constructs and hashes a FundEvent.
func NewFundEvent(in NewEventInput) (FundEvent, error) {
	if in.AggregateType == "" {
		return FundEvent{}, errors.New("fundops: aggregate type required")
	}
	if in.AggregateID == "" {
		return FundEvent{}, errors.New("fundops: aggregate id required")
	}
	if in.EventType == "" {
		return FundEvent{}, errors.New("fundops: event type required")
	}
	payload, err := json.Marshal(in.Payload)
	if err != nil {
		return FundEvent{}, fmt.Errorf("fundops: marshal payload: %w", err)
	}
	if string(payload) == "null" {
		payload = []byte("{}")
	}
	h := sha256.Sum256(payload)
	id, err := randomID()
	if err != nil {
		return FundEvent{}, err
	}
	createdAt := in.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return FundEvent{
		EventID:        id,
		Schema:         SchemaVersion,
		AggregateType:  in.AggregateType,
		AggregateID:    in.AggregateID,
		FundID:         in.FundID,
		EventType:      in.EventType,
		CorrelationID:  in.CorrelationID,
		IdempotencyKey: in.IdempotencyKey,
		Payload:        payload,
		PayloadHash:    hex.EncodeToString(h[:]),
		CreatedAt:      createdAt,
	}, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("fundops: random id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ─── Event Store ──────────────────────────────────────────────────────────────

var (
	ErrDuplicateEvent = errors.New("fundops: duplicate event")
	ErrHashMismatch   = errors.New("fundops: payload hash mismatch")
)

// EventStore is the interface for the fund operations event log.
type EventStore interface {
	Append(ctx context.Context, ev FundEvent) (FundEvent, error)
	Replay(ctx context.Context, aggType AggregateType, aggID string) ([]FundEvent, error)
	ReplayFund(ctx context.Context, fundID string) ([]FundEvent, error)
	TotalCount() int
}

// MemoryEventStore is a thread-safe in-memory fund event store.
type MemoryEventStore struct {
	mu          sync.RWMutex
	events      []FundEvent
	byAggregate map[string][]FundEvent // aggType:aggID → events
	byFund      map[string][]FundEvent // fundID → events
	eventIDs    map[string]struct{}
	idempotency map[string]string
}

// NewMemoryEventStore creates an empty in-memory fund event store.
func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		byAggregate: make(map[string][]FundEvent),
		byFund:      make(map[string][]FundEvent),
		eventIDs:    make(map[string]struct{}),
		idempotency: make(map[string]string),
	}
}

func (s *MemoryEventStore) Append(ctx context.Context, ev FundEvent) (FundEvent, error) {
	if ctx.Err() != nil {
		return FundEvent{}, ctx.Err()
	}
	if !ev.ValidateHash() {
		return FundEvent{}, ErrHashMismatch
	}
	if ev.AggregateType == "" || ev.AggregateID == "" || ev.EventType == "" {
		return FundEvent{}, errors.New("fundops: aggregate type, id, and event type required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.eventIDs[ev.EventID]; ok {
		return FundEvent{}, ErrDuplicateEvent
	}
	if ev.IdempotencyKey != "" {
		if existingID, ok := s.idempotency[ev.IdempotencyKey]; ok {
			return FundEvent{}, fmt.Errorf("%w: key used by %s", ErrDuplicateEvent, existingID)
		}
	}

	aggKey := string(ev.AggregateType) + ":" + ev.AggregateID
	ev.SequenceNo = int64(len(s.byAggregate[aggKey]) + 1)

	s.events = append(s.events, ev)
	s.byAggregate[aggKey] = append(s.byAggregate[aggKey], ev)
	s.byFund[ev.FundID] = append(s.byFund[ev.FundID], ev)
	s.eventIDs[ev.EventID] = struct{}{}
	if ev.IdempotencyKey != "" {
		s.idempotency[ev.IdempotencyKey] = ev.EventID
	}
	return ev, nil
}

func (s *MemoryEventStore) Replay(ctx context.Context, aggType AggregateType, aggID string) ([]FundEvent, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := string(aggType) + ":" + aggID
	return cloneEvents(s.byAggregate[key]), nil
}

func (s *MemoryEventStore) ReplayFund(ctx context.Context, fundID string) ([]FundEvent, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	evts := cloneEvents(s.byFund[fundID])
	sort.SliceStable(evts, func(i, j int) bool {
		return evts[i].CreatedAt.Before(evts[j].CreatedAt)
	})
	return evts, nil
}

func (s *MemoryEventStore) TotalCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func cloneEvents(src []FundEvent) []FundEvent {
	out := make([]FundEvent, len(src))
	copy(out, src)
	return out
}

// ─── Shared Domain Types ──────────────────────────────────────────────────────

// Money represents a monetary amount with currency.
type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

func USD(amount float64) Money { return Money{Amount: amount, Currency: "USD"} }

// FundStatus represents the operational status of a fund.
type FundStatus string

const (
	FundStatusActive   FundStatus = "ACTIVE"
	FundStatusClosed   FundStatus = "CLOSED"
	FundStatusSuspended FundStatus = "SUSPENDED"
)

// InvestorStatus represents the status of an investor account.
type InvestorStatus string

const (
	InvestorStatusActive   InvestorStatus = "ACTIVE"
	InvestorStatusRedeemed InvestorStatus = "REDEEMED"
	InvestorStatusSuspended InvestorStatus = "SUSPENDED"
)

// ─── Event Payloads ───────────────────────────────────────────────────────────

type FundCreatedPayload struct {
	FundID         string     `json:"fund_id"`
	Name           string     `json:"name"`
	Strategy       string     `json:"strategy"`
	Currency       string     `json:"currency"`
	InceptionDate  time.Time  `json:"inception_date"`
	ManagementFee  float64    `json:"management_fee_pct"`   // e.g. 0.02 = 2%
	PerformanceFee float64    `json:"performance_fee_pct"`  // e.g. 0.20 = 20%
	HurdleRate     float64    `json:"hurdle_rate_pct"`      // e.g. 0.05 = 5%
	InitialNAV     float64    `json:"initial_nav_usd"`
}

type InvestorCreatedPayload struct {
	InvestorID   string `json:"investor_id"`
	FundID       string `json:"fund_id"`
	Name         string `json:"name"`
	EntityType   string `json:"entity_type"` // INDIVIDUAL, INSTITUTION, TRUST
	TaxID        string `json:"tax_id,omitempty"`
	JurisdictionCode string `json:"jurisdiction_code"`
}

type CapitalSubscribedPayload struct {
	InvestorID     string    `json:"investor_id"`
	FundID         string    `json:"fund_id"`
	AmountUSD      float64   `json:"amount_usd"`
	UnitsIssued    float64   `json:"units_issued"`
	NAVPerUnit     float64   `json:"nav_per_unit"`
	EffectiveDate  time.Time `json:"effective_date"`
	SubscriptionID string    `json:"subscription_id"`
}

type CapitalRedeemedPayload struct {
	InvestorID    string    `json:"investor_id"`
	FundID        string    `json:"fund_id"`
	AmountUSD     float64   `json:"amount_usd"`
	UnitsRedeemed float64   `json:"units_redeemed"`
	NAVPerUnit    float64   `json:"nav_per_unit"`
	EffectiveDate time.Time `json:"effective_date"`
	RedemptionID  string    `json:"redemption_id"`
}

type CapitalTransferredPayload struct {
	FromInvestorID string    `json:"from_investor_id"`
	ToInvestorID   string    `json:"to_investor_id"`
	FundID         string    `json:"fund_id"`
	UnitsTransferred float64 `json:"units_transferred"`
	EffectiveDate  time.Time `json:"effective_date"`
}

type DistributionPaidPayload struct {
	FundID        string    `json:"fund_id"`
	InvestorID    string    `json:"investor_id"`
	AmountUSD     float64   `json:"amount_usd"`
	PerUnitAmount float64   `json:"per_unit_amount"`
	EffectiveDate time.Time `json:"effective_date"`
	DistribType   string    `json:"distribution_type"` // INCOME, CAPITAL_GAIN, RETURN_OF_CAPITAL
}

type NAVCalculatedPayload struct {
	FundID          string    `json:"fund_id"`
	AsOf            time.Time `json:"as_of"`
	TotalNAV        float64   `json:"total_nav_usd"`
	NAVPerUnit      float64   `json:"nav_per_unit"`
	TotalUnits      float64   `json:"total_units"`
	Cash            float64   `json:"cash_usd"`
	MarketValue     float64   `json:"market_value_usd"`
	UnrealizedPnL   float64   `json:"unrealized_pnl_usd"`
	RealizedPnL     float64   `json:"realized_pnl_usd"`
	AccruedFees     float64   `json:"accrued_fees_usd"`
	AccruedExpenses float64   `json:"accrued_expenses_usd"`
	PeriodReturn    float64   `json:"period_return_pct"`
	YTDReturn       float64   `json:"ytd_return_pct"`
}

type FeeAccruedPayload struct {
	FundID      string    `json:"fund_id"`
	FeeType     string    `json:"fee_type"` // MANAGEMENT, PERFORMANCE
	AmountUSD   float64   `json:"amount_usd"`
	Period      string    `json:"period"` // DAILY, MONTHLY, ANNUAL
	AsOf        time.Time `json:"as_of"`
	NAV         float64   `json:"nav_usd"`
	Rate        float64   `json:"rate"`
}

type FeePaidPayload struct {
	FundID      string    `json:"fund_id"`
	FeeType     string    `json:"fee_type"`
	AmountUSD   float64   `json:"amount_usd"`
	PaidDate    time.Time `json:"paid_date"`
}

type TaxLotCreatedPayload struct {
	LotID          string    `json:"lot_id"`
	FundID         string    `json:"fund_id"`
	InvestorID     string    `json:"investor_id"`
	Symbol         string    `json:"symbol"`
	Quantity       float64   `json:"quantity"`
	CostBasisUSD   float64   `json:"cost_basis_usd"`
	AcquisitionDate time.Time `json:"acquisition_date"`
	FillID         string    `json:"fill_id"`
}

type TaxLotClosedPayload struct {
	LotID           string    `json:"lot_id"`
	FundID          string    `json:"fund_id"`
	Symbol          string    `json:"symbol"`
	QuantityClosed  float64   `json:"quantity_closed"`
	CostBasisUSD    float64   `json:"cost_basis_usd"`
	ProceedsUSD     float64   `json:"proceeds_usd"`
	RealizedGainUSD float64   `json:"realized_gain_usd"`
	HoldingDays     int       `json:"holding_days"`
	IsLongTerm      bool      `json:"is_long_term"` // holding > 365 days
	ClosedDate      time.Time `json:"closed_date"`
}

type ComplianceViolationPayload struct {
	FundID      string    `json:"fund_id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	ViolationType string  `json:"violation_type"`
	ActualValue float64   `json:"actual_value"`
	Limit       float64   `json:"limit"`
	Symbol      string    `json:"symbol,omitempty"`
	DetectedAt  time.Time `json:"detected_at"`
	Severity    string    `json:"severity"` // WARNING, BREACH, CRITICAL
}

type AttributionCalculatedPayload struct {
	FundID       string             `json:"fund_id"`
	Period       string             `json:"period"`
	AsOf         time.Time          `json:"as_of"`
	TotalReturn  float64            `json:"total_return_pct"`
	Attribution  []AttributionEntry `json:"attribution"`
}

type AttributionEntry struct {
	Category    string  `json:"category"` // STRATEGY, ASSET, EXCHANGE, SECTOR
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Return      float64 `json:"return_pct"`
	Contribution float64 `json:"contribution_pct"`
}

type AuditExportPayload struct {
	FundID      string    `json:"fund_id"`
	ExportID    string    `json:"export_id"`
	FromDate    time.Time `json:"from_date"`
	ToDate      time.Time `json:"to_date"`
	Format      string    `json:"format"`
	RecordCount int64     `json:"record_count"`
	GeneratedBy string    `json:"generated_by"`
}

type InvestorReportPayload struct {
	FundID      string    `json:"fund_id"`
	InvestorID  string    `json:"investor_id"`
	ReportType  string    `json:"report_type"` // DAILY, MONTHLY, QUARTERLY, ANNUAL
	Period      string    `json:"period"`
	GeneratedAt time.Time `json:"generated_at"`
}

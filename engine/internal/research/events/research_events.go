// Package events defines the event-sourced backbone for the Phase 19
// Quant Research Platform V2. Every research action emits a typed event;
// the event log is the authoritative record of all research activity.
//
// Research events are stored in an isolated research-only event store —
// they are NEVER written to the production ledger and carry no broker
// credentials or execution instructions.
package events

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

// ─── Event Types ─────────────────────────────────────────────────────────────

type EventType string

const (
	// Feature Store
	EvtFeatureCreated EventType = "FEATURE_CREATED"
	EvtFeatureUpdated EventType = "FEATURE_UPDATED"
	EvtFeatureDeprecated EventType = "FEATURE_DEPRECATED"

	// Experiments
	EvtExperimentStarted   EventType = "EXPERIMENT_STARTED"
	EvtExperimentCompleted EventType = "EXPERIMENT_COMPLETED"
	EvtExperimentFailed    EventType = "EXPERIMENT_FAILED"

	// Models
	EvtModelTrained    EventType = "MODEL_TRAINED"
	EvtModelValidated  EventType = "MODEL_VALIDATED"
	EvtModelApproved   EventType = "MODEL_APPROVED"
	EvtModelRejected   EventType = "MODEL_REJECTED"

	// Alpha / Signal
	EvtAlphaDecayDetected EventType = "ALPHA_DECAY_DETECTED"
	EvtAlphaRestored      EventType = "ALPHA_RESTORED"

	// Promotion
	EvtStrategyPromoted    EventType = "STRATEGY_PROMOTED"
	EvtStrategyRejected    EventType = "STRATEGY_REJECTED"
	EvtPromotionGatePass   EventType = "PROMOTION_GATE_PASS"
	EvtPromotionGateFail   EventType = "PROMOTION_GATE_FAIL"

	// Walk-Forward
	EvtWalkForwardStarted   EventType = "WALKFORWARD_STARTED"
	EvtWalkForwardCompleted EventType = "WALKFORWARD_COMPLETED"

	// Monte Carlo
	EvtMonteCarloStarted   EventType = "MONTECARLO_STARTED"
	EvtMonteCarloCompleted EventType = "MONTECARLO_COMPLETED"

	// Regime
	EvtRegimeTransition EventType = "REGIME_TRANSITION"

	// Data Lake
	EvtDatasetRegistered EventType = "DATASET_REGISTERED"
	EvtDatasetVersioned  EventType = "DATASET_VERSIONED"
)

// AggregateType identifies the research aggregate a research event belongs to.
type AggregateType string

const (
	AggFeature     AggregateType = "FEATURE"
	AggExperiment  AggregateType = "EXPERIMENT"
	AggModel       AggregateType = "MODEL"
	AggPromotion   AggregateType = "PROMOTION"
	AggWalkForward AggregateType = "WALKFORWARD"
	AggMonteCarlo  AggregateType = "MONTECARLO"
	AggRegime      AggregateType = "REGIME"
	AggDataset     AggregateType = "DATASET"
)

// SchemaVersion is the current research event schema version.
const SchemaVersion = "r1"

// ResearchEvent is the immutable unit of research history.
type ResearchEvent struct {
	EventID       string        `json:"event_id"`
	Schema        string        `json:"schema"`
	AggregateType AggregateType `json:"aggregate_type"`
	AggregateID   string        `json:"aggregate_id"`
	EventType     EventType     `json:"event_type"`
	ResearcherID  string        `json:"researcher_id"`
	ExperimentID  string        `json:"experiment_id,omitempty"`
	CorrelationID string        `json:"correlation_id,omitempty"`
	IdempotencyKey string       `json:"idempotency_key,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	PayloadHash   string        `json:"payload_hash"`
	SequenceNo    int64         `json:"sequence_no"`
	CreatedAt     time.Time     `json:"created_at"`
}

func (e ResearchEvent) ValidateHash() bool {
	h := sha256.Sum256(e.Payload)
	return hex.EncodeToString(h[:]) == e.PayloadHash
}

// NewEventInput carries all parameters needed to create a ResearchEvent.
type NewEventInput struct {
	AggregateType  AggregateType
	AggregateID    string
	EventType      EventType
	ResearcherID   string
	ExperimentID   string
	CorrelationID  string
	IdempotencyKey string
	Payload        any
	CreatedAt      time.Time
}

// NewResearchEvent creates and hashes a ResearchEvent.
func NewResearchEvent(in NewEventInput) (ResearchEvent, error) {
	if in.AggregateType == "" {
		return ResearchEvent{}, errors.New("research/events: aggregate type required")
	}
	if in.AggregateID == "" {
		return ResearchEvent{}, errors.New("research/events: aggregate id required")
	}
	if in.EventType == "" {
		return ResearchEvent{}, errors.New("research/events: event type required")
	}

	payload, err := json.Marshal(in.Payload)
	if err != nil {
		return ResearchEvent{}, fmt.Errorf("research/events: marshal payload: %w", err)
	}
	if string(payload) == "null" {
		payload = []byte("{}")
	}
	h := sha256.Sum256(payload)

	eventID, err := randomID()
	if err != nil {
		return ResearchEvent{}, err
	}
	createdAt := in.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	return ResearchEvent{
		EventID:        eventID,
		Schema:         SchemaVersion,
		AggregateType:  in.AggregateType,
		AggregateID:    in.AggregateID,
		EventType:      in.EventType,
		ResearcherID:   in.ResearcherID,
		ExperimentID:   in.ExperimentID,
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
		return "", fmt.Errorf("research/events: random id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ─── Research Event Store ─────────────────────────────────────────────────────

var (
	ErrDuplicateEvent = errors.New("research/events: duplicate event")
	ErrHashMismatch   = errors.New("research/events: payload hash mismatch")
)

// Store is the interface for the research event log.
type Store interface {
	Append(ctx context.Context, event ResearchEvent) (ResearchEvent, error)
	Replay(ctx context.Context, aggType AggregateType, aggID string) ([]ResearchEvent, error)
	ReplayAll(ctx context.Context) ([]ResearchEvent, error)
	ReplayByExperiment(ctx context.Context, experimentID string) ([]ResearchEvent, error)
	TotalCount() int
}

// MemoryStore is a thread-safe in-memory research event store.
// Use PostgreSQL-backed store in production for durability.
type MemoryStore struct {
	mu           sync.RWMutex
	events       []ResearchEvent
	byAggregate  map[string][]ResearchEvent
	byExperiment map[string][]ResearchEvent
	eventIDs     map[string]struct{}
	idempotency  map[string]string
}

// NewMemoryStore creates a new in-memory research event store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byAggregate:  make(map[string][]ResearchEvent),
		byExperiment: make(map[string][]ResearchEvent),
		eventIDs:     make(map[string]struct{}),
		idempotency:  make(map[string]string),
	}
}

func (s *MemoryStore) Append(ctx context.Context, ev ResearchEvent) (ResearchEvent, error) {
	if ctx.Err() != nil {
		return ResearchEvent{}, ctx.Err()
	}
	if !ev.ValidateHash() {
		return ResearchEvent{}, ErrHashMismatch
	}
	if ev.AggregateType == "" || ev.AggregateID == "" || ev.EventType == "" {
		return ResearchEvent{}, errors.New("research/events: aggregate type, id, and event type are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.eventIDs[ev.EventID]; ok {
		return ResearchEvent{}, ErrDuplicateEvent
	}
	if ev.IdempotencyKey != "" {
		if existingID, ok := s.idempotency[ev.IdempotencyKey]; ok {
			return ResearchEvent{}, fmt.Errorf("%w: idempotency key used by %s", ErrDuplicateEvent, existingID)
		}
	}

	key := string(ev.AggregateType) + ":" + ev.AggregateID
	ev.SequenceNo = int64(len(s.byAggregate[key]) + 1)

	s.events = append(s.events, ev)
	s.byAggregate[key] = append(s.byAggregate[key], ev)
	s.eventIDs[ev.EventID] = struct{}{}
	if ev.IdempotencyKey != "" {
		s.idempotency[ev.IdempotencyKey] = ev.EventID
	}
	if ev.ExperimentID != "" {
		s.byExperiment[ev.ExperimentID] = append(s.byExperiment[ev.ExperimentID], ev)
	}
	return ev, nil
}

func (s *MemoryStore) Replay(ctx context.Context, aggType AggregateType, aggID string) ([]ResearchEvent, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := string(aggType) + ":" + aggID
	return cloneEvents(s.byAggregate[key]), nil
}

func (s *MemoryStore) ReplayAll(ctx context.Context) ([]ResearchEvent, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := cloneEvents(s.events)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) ReplayByExperiment(ctx context.Context, experimentID string) ([]ResearchEvent, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneEvents(s.byExperiment[experimentID]), nil
}

func (s *MemoryStore) TotalCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func cloneEvents(src []ResearchEvent) []ResearchEvent {
	out := make([]ResearchEvent, len(src))
	copy(out, src)
	return out
}

// ─── Common Payloads ─────────────────────────────────────────────────────────

type FeatureCreatedPayload struct {
	FeatureID   string   `json:"feature_id"`
	Name        string   `json:"name"`
	Version     int      `json:"version"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type ExperimentStartedPayload struct {
	ExperimentID string         `json:"experiment_id"`
	Name         string         `json:"name"`
	StrategyID   string         `json:"strategy_id"`
	Parameters   map[string]any `json:"parameters"`
	DatasetID    string         `json:"dataset_id"`
}

type ExperimentCompletedPayload struct {
	ExperimentID string             `json:"experiment_id"`
	Metrics      ExperimentMetrics  `json:"metrics"`
	DurationMs   int64              `json:"duration_ms"`
}

type ExperimentMetrics struct {
	SharpeRatio  float64 `json:"sharpe_ratio"`
	SortinoRatio float64 `json:"sortino_ratio"`
	MaxDrawdown  float64 `json:"max_drawdown_pct"`
	WinRate      float64 `json:"win_rate"`
	ProfitFactor float64 `json:"profit_factor"`
	Alpha        float64 `json:"alpha"`
	Beta         float64 `json:"beta"`
	TotalTrades  int     `json:"total_trades"`
	NetPnLUSD    float64 `json:"net_pnl_usd"`
}

type ModelTrainedPayload struct {
	ModelID        string         `json:"model_id"`
	ModelType      string         `json:"model_type"`
	ExperimentID   string         `json:"experiment_id"`
	FeatureIDs     []string       `json:"feature_ids"`
	Hyperparameters map[string]any `json:"hyperparameters"`
	TrainMetrics   map[string]float64 `json:"train_metrics"`
	ValMetrics     map[string]float64 `json:"val_metrics"`
}

type AlphaDecayPayload struct {
	StrategyID  string  `json:"strategy_id"`
	HalfLifeDays float64 `json:"half_life_days"`
	CurrentIC   float64 `json:"current_ic"`
	BaselineIC  float64 `json:"baseline_ic"`
	DecayPct    float64 `json:"decay_pct"`
	Regime      string  `json:"regime"`
}

type PromotionPayload struct {
	StrategyID  string            `json:"strategy_id"`
	FromState   string            `json:"from_state"`
	ToState     string            `json:"to_state"`
	GateResults map[string]bool   `json:"gate_results"`
	ApprovedBy  string            `json:"approved_by,omitempty"`
	Reason      string            `json:"reason,omitempty"`
}

type RegimeTransitionPayload struct {
	FromRegime string  `json:"from_regime"`
	ToRegime   string  `json:"to_regime"`
	ADX        float64 `json:"adx"`
	ATRPct     float64 `json:"atr_pct"`
	Confidence float64 `json:"confidence"`
}

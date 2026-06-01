package ledger

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type AggregateType string

const (
	AggregateOrder          AggregateType = "ORDER"
	AggregatePosition       AggregateType = "POSITION"
	AggregateRisk           AggregateType = "RISK"
	AggregateAccount        AggregateType = "ACCOUNT"
	AggregateMarketData     AggregateType = "MARKET_DATA"
	AggregateReconciliation AggregateType = "RECONCILIATION"
	AggregateStrategy       AggregateType = "STRATEGY"
	AggregateExchange       AggregateType = "EXCHANGE"
	AggregateSystem         AggregateType = "SYSTEM"
)

type EventType string

const (
	// ── Order lifecycle ──────────────────────────────────────────────────────────
	EventOrderCreated         EventType = "ORDER_CREATED"
	EventOrderValidated       EventType = "ORDER_VALIDATED"
	EventOrderAccepted        EventType = "ORDER_ACCEPTED"
	EventOrderSubmitted       EventType = "ORDER_SUBMITTED"
	EventOrderAcked           EventType = "ORDER_ACKED"
	EventOrderPartial         EventType = "ORDER_PARTIAL"
	EventOrderFilled          EventType = "ORDER_FILLED"
	EventOrderCancelled       EventType = "ORDER_CANCELLED"
	EventOrderRejected        EventType = "ORDER_REJECTED"
	EventOrderExpired         EventType = "ORDER_EXPIRED"
	EventOrderReplaceRequested EventType = "ORDER_REPLACE_REQUESTED"
	EventOrderReplaced        EventType = "ORDER_REPLACED"

	// ── Position lifecycle ───────────────────────────────────────────────────────
	EventPositionOpened      EventType = "POSITION_OPENED"
	EventPositionScaled      EventType = "POSITION_SCALED"
	EventPositionChanged     EventType = "POSITION_CHANGED"
	EventPositionReduced     EventType = "POSITION_REDUCED"
	EventPositionClosed      EventType = "POSITION_CLOSED"
	EventPositionLiquidated  EventType = "POSITION_LIQUIDATED"
	EventPositionTransferred EventType = "POSITION_TRANSFERRED"

	// Position adaptive stop/target management
	EventPositionSLMoved            EventType = "POSITION_SL_MOVED"
	EventPositionBreakevenActivated EventType = "POSITION_BREAKEVEN_ACTIVATED"
	EventPositionTrailingActivated  EventType = "POSITION_TRAILING_ACTIVATED"
	EventPositionTPMoved            EventType = "POSITION_TP_MOVED"
	EventPositionRiskAdjusted       EventType = "POSITION_RISK_ADJUSTED"

	// ── Risk ────────────────────────────────────────────────────────────────────
	EventRiskCheckStarted             EventType = "RISK_CHECK_STARTED"
	EventRiskApproved                 EventType = "RISK_APPROVED"
	EventRiskBlocked                  EventType = "RISK_BLOCKED"
	EventRiskViolation                EventType = "RISK_VIOLATION"
	EventRiskTriggered                EventType = "RISK_TRIGGERED"
	EventExposureLimitExceeded        EventType = "EXPOSURE_LIMIT_EXCEEDED"
	EventPortfolioHeatExceeded        EventType = "PORTFOLIO_HEAT_EXCEEDED"
	EventVaRBreach                    EventType = "VAR_BREACH"
	EventCVaRBreach                   EventType = "CVAR_BREACH"
	EventMaxDrawdownBreached          EventType = "MAX_DRAWDOWN_BREACHED"
	EventFundingExposureExceeded      EventType = "FUNDING_EXPOSURE_EXCEEDED"
	EventRiskDailyLossLimitExceeded   EventType = "RISK_DAILY_LOSS_LIMIT_EXCEEDED"
	EventRiskMarginViolation          EventType = "RISK_MARGIN_VIOLATION"
	EventRiskLeverageViolation        EventType = "RISK_LEVERAGE_VIOLATION"
	EventRiskConcentrationViolation   EventType = "RISK_CONCENTRATION_VIOLATION"
	EventRiskCorrelationViolation     EventType = "RISK_CORRELATION_VIOLATION"

	// ── Kill switch ──────────────────────────────────────────────────────────────
	EventKillSwitchTriggered EventType = "KILLSWITCH_TRIGGERED"
	EventKillSwitchReleased  EventType = "KILLSWITCH_RELEASED"

	// ── Strategy lifecycle ───────────────────────────────────────────────────────
	EventStrategyRegistered        EventType = "STRATEGY_REGISTERED"
	EventStrategyEnabled           EventType = "STRATEGY_ENABLED"
	EventStrategyDisabled          EventType = "STRATEGY_DISABLED"
	EventStrategyPaused            EventType = "STRATEGY_PAUSED"
	EventStrategyResumed           EventType = "STRATEGY_RESUMED"
	EventStrategyPromoted          EventType = "STRATEGY_PROMOTED"
	EventStrategyDemoted           EventType = "STRATEGY_DEMOTED"
	EventStrategyAllocationChanged EventType = "STRATEGY_ALLOCATION_CHANGED"

	// ── Exchange connectivity ────────────────────────────────────────────────────
	EventExchangeConnected      EventType = "EXCHANGE_CONNECTED"
	EventExchangeDisconnected   EventType = "EXCHANGE_DISCONNECTED"
	EventExchangeReconnected    EventType = "EXCHANGE_RECONNECTED"
	EventExchangeOrderRejected  EventType = "EXCHANGE_ORDER_REJECTED"
	EventExchangeLatencySpike   EventType = "EXCHANGE_LATENCY_SPIKE"
	EventExchangeRateLimitHit   EventType = "EXCHANGE_RATE_LIMIT_HIT"
	EventExchangeDataGapDetected EventType = "EXCHANGE_DATA_GAP_DETECTED"
	EventExchangeOutage          EventType = "EXCHANGE_OUTAGE"
	EventMarketDataStale         EventType = "MARKET_DATA_STALE"
	EventMarketDataRecovered     EventType = "MARKET_DATA_RECOVERED"
	EventReconciliationStarted   EventType = "RECONCILIATION_STARTED"

	// ── System lifecycle ─────────────────────────────────────────────────────────
	EventEngineStarting    EventType = "ENGINE_STARTING"
	EventEngineStarted     EventType = "ENGINE_STARTED"
	EventEngineStopping    EventType = "ENGINE_STOPPING"
	EventEngineStopped     EventType = "ENGINE_STOPPED"
	EventReplayStarted     EventType = "REPLAY_STARTED"
	EventReplayCompleted   EventType = "REPLAY_COMPLETED"
	EventProjectionRebuilt EventType = "PROJECTION_REBUILT"
	EventSnapshotCreated   EventType = "SNAPSHOT_CREATED"
	EventSnapshotRestored  EventType = "SNAPSHOT_RESTORED"

	// ── Reconciliation ───────────────────────────────────────────────────────────
	EventReconciliationMismatch  EventType = "RECONCILIATION_MISMATCH"
	EventReconciliationAlert     EventType = "RECONCILIATION_ALERT"
	EventReconciliationCorrected EventType = "RECONCILIATION_CORRECTED"
	EventReconciliationResolved  EventType = "RECONCILIATION_RESOLVED"
)

type Event struct {
	EventID        string          `json:"event_id"`
	SchemaVersion  int             `json:"schema_version"`
	AggregateType  AggregateType   `json:"aggregate_type"`
	AggregateID    string          `json:"aggregate_id"`
	SequenceNo     int64           `json:"sequence_no"`
	EventType      EventType       `json:"event_type"`
	AccountID      string          `json:"account_id,omitempty"`
	StrategyID     string          `json:"strategy_id,omitempty"`
	Symbol         string          `json:"symbol,omitempty"`
	CorrelationID  string          `json:"correlation_id,omitempty"`
	CausationID    string          `json:"causation_id,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	PayloadHash    string          `json:"payload_hash"`
	CreatedAt      time.Time       `json:"created_at"`
	Source         string          `json:"source"`
}

type NewEventInput struct {
	AggregateType  AggregateType
	AggregateID    string
	EventType      EventType
	AccountID      string
	StrategyID     string
	Symbol         string
	CorrelationID  string
	CausationID    string
	IdempotencyKey string
	Payload        any
	Source         string
	CreatedAt      time.Time
}

func NewEvent(input NewEventInput) (Event, error) {
	if input.AggregateType == "" {
		return Event{}, errors.New("ledger: aggregate type is required")
	}
	if input.AggregateID == "" {
		return Event{}, errors.New("ledger: aggregate id is required")
	}
	if input.EventType == "" {
		return Event{}, errors.New("ledger: event type is required")
	}
	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return Event{}, fmt.Errorf("ledger: marshal payload: %w", err)
	}
	if string(payload) == "null" {
		payload = []byte("{}")
	}
	eventID, err := randomID()
	if err != nil {
		return Event{}, err
	}
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	ev := Event{
		EventID:        eventID,
		SchemaVersion:  CurrentSchemaVersion,
		AggregateType:  input.AggregateType,
		AggregateID:    input.AggregateID,
		EventType:      input.EventType,
		AccountID:      input.AccountID,
		StrategyID:     input.StrategyID,
		Symbol:         input.Symbol,
		CorrelationID:  input.CorrelationID,
		CausationID:    input.CausationID,
		IdempotencyKey: input.IdempotencyKey,
		Payload:        payload,
		PayloadHash:    PayloadHash(payload),
		CreatedAt:      createdAt,
		Source:         input.Source,
	}
	if ev.Source == "" {
		ev.Source = "unknown"
	}
	return ev, nil
}

func PayloadHash(payload json.RawMessage) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (e Event) ValidateHash() bool {
	return e.PayloadHash == PayloadHash(e.Payload)
}

func (e Event) AggregateKey() string {
	return string(e.AggregateType) + ":" + e.AggregateID
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("ledger: random id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

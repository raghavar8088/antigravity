package reconciliationv2

import (
	"time"

	"antigravity-engine/internal/ledger"
)

// Phase 15E canonical reconciliation event types.
// Extends ledger's existing RECONCILIATION_STARTED, RECONCILIATION_MISMATCH,
// RECONCILIATION_ALERT, RECONCILIATION_CORRECTED, RECONCILIATION_RESOLVED.
const (
	EventReconciliationCompleted    ledger.EventType = "RECONCILIATION_COMPLETED"
	EventBalanceMismatchDetected    ledger.EventType = "BALANCE_MISMATCH_DETECTED"
	EventPositionMismatchDetected   ledger.EventType = "POSITION_MISMATCH_DETECTED"
	EventOrderMismatchDetected      ledger.EventType = "ORDER_MISMATCH_DETECTED"
	EventFillMismatchDetected       ledger.EventType = "FILL_MISMATCH_DETECTED"
	EventExposureMismatchDetected   ledger.EventType = "EXPOSURE_MISMATCH_DETECTED"
	EventPnLMismatchDetected        ledger.EventType = "PNL_MISMATCH_DETECTED"
	EventRepairStarted              ledger.EventType = "RECONCILIATION_REPAIR_STARTED"
	EventRepairCompleted            ledger.EventType = "RECONCILIATION_REPAIR_COMPLETED"
	EventRepairFailed               ledger.EventType = "RECONCILIATION_REPAIR_FAILED"
	EventManualInterventionRequired ledger.EventType = "MANUAL_INTERVENTION_REQUIRED"
	EventExchangeAuthorityVerified  ledger.EventType = "EXCHANGE_AUTHORITY_VERIFIED"
)

// DriftSeverity classifies the severity of a detected reconciliation mismatch.
type DriftSeverity string

const (
	SeverityLow      DriftSeverity = "LOW"
	SeverityMedium   DriftSeverity = "MEDIUM"
	SeverityHigh     DriftSeverity = "HIGH"
	SeverityCritical DriftSeverity = "CRITICAL"
)

// MismatchDomain identifies which aspect of account state produced the mismatch.
type MismatchDomain string

const (
	DomainBalance  MismatchDomain = "balance"
	DomainPosition MismatchDomain = "position"
	DomainOrder    MismatchDomain = "order"
	DomainFill     MismatchDomain = "fill"
	DomainExposure MismatchDomain = "exposure"
	DomainPnL      MismatchDomain = "pnl"
	DomainFull     MismatchDomain = "full"
)

// ReconciliationStartedPayload is emitted at the start of each reconciliation cycle.
type ReconciliationStartedPayload struct {
	RunID     string         `json:"run_id"`
	Domain    MismatchDomain `json:"domain"`
	Exchange  string         `json:"exchange"`
	AccountID string         `json:"account_id"`
	StartedAt time.Time      `json:"started_at"`
}

// ReconciliationCompletedPayload is emitted after a successful reconciliation cycle.
type ReconciliationCompletedPayload struct {
	RunID         string         `json:"run_id"`
	Domain        MismatchDomain `json:"domain"`
	Exchange      string         `json:"exchange"`
	AccountID     string         `json:"account_id"`
	MismatchCount int            `json:"mismatch_count"`
	RepairCount   int            `json:"repair_count"`
	DriftScore    float64        `json:"drift_score"`
	DurationMs    int64          `json:"duration_ms"`
	CompletedAt   time.Time      `json:"completed_at"`
}

// MismatchPayload is the base embedded struct for all mismatch events.
type MismatchPayload struct {
	RunID         string         `json:"run_id"`
	Domain        MismatchDomain `json:"domain"`
	MismatchType  string         `json:"mismatch_type"`
	Exchange      string         `json:"exchange"`
	AccountID     string         `json:"account_id"`
	Severity      DriftSeverity  `json:"severity"`
	Symbol        string         `json:"symbol,omitempty"`
	Reference     string         `json:"reference,omitempty"`
	ExchangeValue string         `json:"exchange_value"`
	InternalValue string         `json:"internal_value"`
	DriftPct      float64        `json:"drift_pct"`
	Message       string         `json:"message"`
	DetectedAt    time.Time      `json:"detected_at"`
}

// BalanceMismatchPayload is emitted when exchange and internal balances diverge.
type BalanceMismatchPayload struct {
	MismatchPayload
	Asset             string  `json:"asset"`
	ExchangeEquity    float64 `json:"exchange_equity_usd"`
	InternalEquity    float64 `json:"internal_equity_usd"`
	ExchangeAvailable float64 `json:"exchange_available_usd"`
	InternalAvailable float64 `json:"internal_available_usd"`
	ExchangeMargin    float64 `json:"exchange_margin_usd"`
	InternalMargin    float64 `json:"internal_margin_usd"`
}

// PositionMismatchPayload is emitted when positions diverge between exchange and OMS.
type PositionMismatchPayload struct {
	MismatchPayload
	PositionID        string  `json:"position_id,omitempty"`
	ExchangeQty       float64 `json:"exchange_qty"`
	InternalQty       float64 `json:"internal_qty"`
	ExchangePrice     float64 `json:"exchange_price"`
	InternalPrice     float64 `json:"internal_price"`
	ExchangeSide      string  `json:"exchange_side"`
	InternalSide      string  `json:"internal_side"`
	ExchangeUnrealPnL float64 `json:"exchange_unreal_pnl_usd"`
	InternalUnrealPnL float64 `json:"internal_unreal_pnl_usd"`
}

// OrderMismatchPayload is emitted when order state diverges between exchange and OMS.
type OrderMismatchPayload struct {
	MismatchPayload
	ClientOrderID     string  `json:"client_order_id,omitempty"`
	ExchangeOrderID   string  `json:"exchange_order_id,omitempty"`
	ExchangeStatus    string  `json:"exchange_status"`
	InternalStatus    string  `json:"internal_status"`
	ExchangeQty       float64 `json:"exchange_qty"`
	InternalQty       float64 `json:"internal_qty"`
	ExchangeFilledQty float64 `json:"exchange_filled_qty"`
	InternalFilledQty float64 `json:"internal_filled_qty"`
	ExchangePrice     float64 `json:"exchange_price"`
	InternalPrice     float64 `json:"internal_price"`
}

// FillMismatchPayload is emitted when fills diverge between exchange and OMS.
// Fill reconciliation must be exact — zero tolerance for drift.
type FillMismatchPayload struct {
	MismatchPayload
	FillID            string    `json:"fill_id,omitempty"`
	ClientOrderID     string    `json:"client_order_id,omitempty"`
	ExchangeOrderID   string    `json:"exchange_order_id,omitempty"`
	ExchangePrice     float64   `json:"exchange_price"`
	InternalPrice     float64   `json:"internal_price"`
	ExchangeQty       float64   `json:"exchange_qty"`
	InternalQty       float64   `json:"internal_qty"`
	ExchangeFee       float64   `json:"exchange_fee_usd"`
	InternalFee       float64   `json:"internal_fee_usd"`
	ExchangeTimestamp time.Time `json:"exchange_timestamp"`
	InternalTimestamp time.Time `json:"internal_timestamp"`
}

// ExposureMismatchPayload is emitted when portfolio exposure diverges.
type ExposureMismatchPayload struct {
	MismatchPayload
	ExchangeGrossUSD   float64 `json:"exchange_gross_usd"`
	InternalGrossUSD   float64 `json:"internal_gross_usd"`
	ExchangeNetUSD     float64 `json:"exchange_net_usd"`
	InternalNetUSD     float64 `json:"internal_net_usd"`
	ExchangeLongUSD    float64 `json:"exchange_long_usd"`
	InternalLongUSD    float64 `json:"internal_long_usd"`
	ExchangeShortUSD   float64 `json:"exchange_short_usd"`
	InternalShortUSD   float64 `json:"internal_short_usd"`
	RiskEngineGrossUSD float64 `json:"risk_engine_gross_usd"`
}

// RepairPayload is shared by RepairStarted, RepairCompleted, RepairFailed events.
type RepairPayload struct {
	RunID       string         `json:"run_id"`
	RepairID    string         `json:"repair_id"`
	RepairType  string         `json:"repair_type"` // projection_rebuild|aggregate_rebuild|replay|state_sync|correction_event
	Domain      MismatchDomain `json:"domain"`
	Exchange    string         `json:"exchange"`
	AccountID   string         `json:"account_id"`
	MismatchRef string         `json:"mismatch_ref"`
	Description string         `json:"description"`
	Automatic   bool           `json:"automatic"`
	Error       string         `json:"error,omitempty"`
	InitiatedAt time.Time      `json:"initiated_at"`
	CompletedAt time.Time      `json:"completed_at,omitempty"`
}

// ManualInterventionPayload is emitted when automatic repair is not safe to perform.
type ManualInterventionPayload struct {
	RunID      string         `json:"run_id"`
	Domain     MismatchDomain `json:"domain"`
	Exchange   string         `json:"exchange"`
	AccountID  string         `json:"account_id"`
	Severity   DriftSeverity  `json:"severity"`
	Reference  string         `json:"reference"`
	Message    string         `json:"message"`
	Reason     string         `json:"reason"`
	RequiredAt time.Time      `json:"required_at"`
}

// ExchangeAuthorityVerifiedPayload is emitted when exchange confirms internal state matches.
type ExchangeAuthorityVerifiedPayload struct {
	RunID      string         `json:"run_id"`
	Domain     MismatchDomain `json:"domain"`
	Exchange   string         `json:"exchange"`
	AccountID  string         `json:"account_id"`
	VerifiedAt time.Time      `json:"verified_at"`
}

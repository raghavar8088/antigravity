// Package pilot implements the Phase 23 Live Trading Pilot & Capital Scaling Framework.
// It governs staged live deployment, capital scaling, strategy ranking, and go-live certification.
package pilot

import (
	"context"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── Pilot Aggregate Types ─────────────────────────────────────────────────────

const (
	AggregatePilot   ledger.AggregateType = "PILOT"
	AggregateStage   ledger.AggregateType = "PILOT_STAGE"
	AggregateMetrics ledger.AggregateType = "PILOT_METRICS"
)

// ── Pilot Event Types ─────────────────────────────────────────────────────────

const (
	EventPilotInitialized         ledger.EventType = "PILOT_INITIALIZED"
	EventPilotStageAdvanced       ledger.EventType = "PILOT_STAGE_ADVANCED"
	EventPilotStrategyDeployed    ledger.EventType = "PILOT_STRATEGY_DEPLOYED"
	EventPilotStrategyRetracted   ledger.EventType = "PILOT_STRATEGY_RETRACTED"
	EventPilotCapitalScaledUp     ledger.EventType = "PILOT_CAPITAL_SCALED_UP"
	EventPilotCapitalScaledDown   ledger.EventType = "PILOT_CAPITAL_SCALED_DOWN"
	EventPilotCertificationUpdated ledger.EventType = "PILOT_CERTIFICATION_UPDATED"
	EventPilotMetricsSnapshotted  ledger.EventType = "PILOT_METRICS_SNAPSHOTTED"
	EventPilotAutoDownscale       ledger.EventType = "PILOT_AUTO_DOWNSCALE_TRIGGERED"
	EventPilotAutoUpscaleApproved ledger.EventType = "PILOT_AUTO_UPSCALE_APPROVED"
	EventPilotRiskViolation       ledger.EventType = "PILOT_RISK_VIOLATION"
	EventPilotHalted              ledger.EventType = "PILOT_HALTED"
	EventPilotResumed             ledger.EventType = "PILOT_RESUMED"
)

// ── Stage ─────────────────────────────────────────────────────────────────────

// Stage represents the current deployment stage of the live pilot.
type Stage int

const (
	StageNotStarted Stage = 0
	Stage1          Stage = 1 // top 3 strategies, 0.25–0.50% capital each
	Stage2          Stage = 2 // top 5 strategies, 0.50–1.00% capital each
	Stage3          Stage = 3 // top 10 strategies, 1.00–2.00% capital each
	Stage4          Stage = 4 // full validated portfolio, open-ended
)

func (s Stage) String() string {
	switch s {
	case Stage1:
		return "STAGE_1"
	case Stage2:
		return "STAGE_2"
	case Stage3:
		return "STAGE_3"
	case Stage4:
		return "STAGE_4"
	default:
		return "NOT_STARTED"
	}
}

// MaxStrategies returns the maximum number of strategies allowed in this stage.
// Stage4 returns 0, meaning unlimited.
func (s Stage) MaxStrategies() int {
	switch s {
	case Stage1:
		return 3
	case Stage2:
		return 5
	case Stage3:
		return 10
	default:
		return 0
	}
}

// CapitalRangePct returns the allowed per-strategy capital allocation range.
func (s Stage) CapitalRangePct() (min, max float64) {
	switch s {
	case Stage1:
		return 0.0025, 0.0050
	case Stage2:
		return 0.0050, 0.0100
	case Stage3:
		return 0.0100, 0.0200
	case Stage4:
		return 0.0200, 0.0500
	default:
		return 0, 0
	}
}

// DurationDays returns the minimum number of days required before stage advancement.
func (s Stage) DurationDays() int {
	switch s {
	case Stage1:
		return 30
	case Stage2:
		return 30
	case Stage3:
		return 60
	default:
		return 0
	}
}

// ── Certification Status ──────────────────────────────────────────────────────

// CertificationStatus represents the current go-live certification verdict.
type CertificationStatus int

const (
	CertNotReady                     CertificationStatus = 0
	CertReadyForPilot                CertificationStatus = 1
	CertReadyForLimitedScaling       CertificationStatus = 2
	CertReadyForInstitutionalScaling CertificationStatus = 3
)

func (c CertificationStatus) String() string {
	switch c {
	case CertReadyForPilot:
		return "READY_FOR_PILOT_DEPLOYMENT"
	case CertReadyForLimitedScaling:
		return "READY_FOR_LIMITED_SCALING"
	case CertReadyForInstitutionalScaling:
		return "READY_FOR_INSTITUTIONAL_SCALING"
	default:
		return "NOT_READY_FOR_LIVE_CAPITAL"
	}
}

// ── Payloads ──────────────────────────────────────────────────────────────────

type PilotInitializedPayload struct {
	PilotID      string    `json:"pilot_id"`
	AccountID    string    `json:"account_id"`
	TotalCapital float64   `json:"total_capital_usd"`
	InitiatedBy  string    `json:"initiated_by"`
	InitiatedAt  time.Time `json:"initiated_at"`
}

type StageAdvancedPayload struct {
	PilotID       string             `json:"pilot_id"`
	PreviousStage Stage              `json:"previous_stage"`
	NewStage      Stage              `json:"new_stage"`
	Reason        string             `json:"reason"`
	AdvancedAt    time.Time          `json:"advanced_at"`
	Metrics       LiveMetricsSummary `json:"qualifying_metrics"`
}

type StrategyDeployedPayload struct {
	PilotID      string    `json:"pilot_id"`
	StrategyID   string    `json:"strategy_id"`
	StrategyName string    `json:"strategy_name"`
	Stage        Stage     `json:"stage"`
	AllocPct     float64   `json:"alloc_pct"`
	AllocUSD     float64   `json:"alloc_usd"`
	Rank         int       `json:"rank"`
	DeployedAt   time.Time `json:"deployed_at"`
}

type StrategyRetractedPayload struct {
	PilotID      string    `json:"pilot_id"`
	StrategyID   string    `json:"strategy_id"`
	StrategyName string    `json:"strategy_name"`
	Reason       string    `json:"reason"`
	RetractedAt  time.Time `json:"retracted_at"`
}

type CapitalScaledPayload struct {
	PilotID          string    `json:"pilot_id"`
	StrategyID       string    `json:"strategy_id"`
	Direction        string    `json:"direction"` // UP | DOWN
	PreviousAllocPct float64   `json:"previous_alloc_pct"`
	NewAllocPct      float64   `json:"new_alloc_pct"`
	PreviousAllocUSD float64   `json:"previous_alloc_usd"`
	NewAllocUSD      float64   `json:"new_alloc_usd"`
	Trigger          string    `json:"trigger"`
	MetricValue      float64   `json:"metric_value"`
	ScaledAt         time.Time `json:"scaled_at"`
}

type CertificationUpdatedPayload struct {
	PilotID   string              `json:"pilot_id"`
	Previous  CertificationStatus `json:"previous_status"`
	Current   CertificationStatus `json:"current_status"`
	Reasons   []string            `json:"reasons"`
	Metrics   LiveMetricsSummary  `json:"metrics"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type MetricsSnapshotPayload struct {
	PilotID   string             `json:"pilot_id"`
	Stage     Stage              `json:"stage"`
	Metrics   LiveMetricsSummary `json:"metrics"`
	SnappedAt time.Time          `json:"snapped_at"`
}

type AutoDownscalePayload struct {
	PilotID     string    `json:"pilot_id"`
	StrategyID  string    `json:"strategy_id"`
	Trigger     string    `json:"trigger"` // PF_BREACH | SHARPE_BREACH | DD_BREACH | LOSING_WEEKS | RISK_VIOLATION
	MetricValue float64   `json:"metric_value"`
	Threshold   float64   `json:"threshold"`
	TriggeredAt time.Time `json:"triggered_at"`
}

type PilotHaltedPayload struct {
	PilotID  string    `json:"pilot_id"`
	Reason   string    `json:"reason"`
	HaltedBy string    `json:"halted_by"`
	HaltedAt time.Time `json:"halted_at"`
}

type RiskViolationPayload struct {
	PilotID       string    `json:"pilot_id"`
	StrategyID    string    `json:"strategy_id,omitempty"`
	ViolationType string    `json:"violation_type"`
	Details       string    `json:"details"`
	OccurredAt    time.Time `json:"occurred_at"`
}

// LiveMetricsSummary is a compact snapshot embedded in events and certification payloads.
type LiveMetricsSummary struct {
	TotalTrades       int     `json:"total_trades"`
	ProfitFactor      float64 `json:"profit_factor"`
	WinRate           float64 `json:"win_rate"`
	SharpeRatio       float64 `json:"sharpe_ratio"`
	MaxDrawdown       float64 `json:"max_drawdown_pct"`
	ConsecutiveLosses int     `json:"consecutive_losing_weeks"`
	PositiveMonths    int     `json:"positive_months"`
	RiskViolations    int     `json:"risk_violations"`
}

// ── Emission helpers ──────────────────────────────────────────────────────────

func emitPilotEvent(ctx context.Context, store ledger.Store, input ledger.NewEventInput) {
	if store == nil {
		return
	}
	ev, err := ledger.NewEvent(input)
	if err != nil {
		return
	}
	store.Append(ctx, ev) //nolint:errcheck
}

func EmitPilotInitialized(ctx context.Context, store ledger.Store, p PilotInitializedPayload) {
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   p.PilotID,
		EventType:     EventPilotInitialized,
		AccountID:     p.AccountID,
		Payload:       p,
		Source:        "pilot",
	})
}

func EmitStageAdvanced(ctx context.Context, store ledger.Store, pilotID, accountID string, p StageAdvancedPayload) {
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   pilotID,
		EventType:     EventPilotStageAdvanced,
		AccountID:     accountID,
		Payload:       p,
		Source:        "pilot.deployment",
	})
}

func EmitStrategyDeployed(ctx context.Context, store ledger.Store, accountID string, p StrategyDeployedPayload) {
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   p.PilotID,
		EventType:     EventPilotStrategyDeployed,
		AccountID:     accountID,
		StrategyID:    p.StrategyID,
		Payload:       p,
		Source:        "pilot.deployment",
	})
}

func EmitCapitalScaled(ctx context.Context, store ledger.Store, accountID string, p CapitalScaledPayload) {
	et := EventPilotCapitalScaledUp
	if p.Direction == "DOWN" {
		et = EventPilotCapitalScaledDown
	}
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   p.PilotID,
		EventType:     et,
		AccountID:     accountID,
		StrategyID:    p.StrategyID,
		Payload:       p,
		Source:        "pilot.capital",
	})
}

func EmitCertificationUpdated(ctx context.Context, store ledger.Store, accountID string, p CertificationUpdatedPayload) {
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   p.PilotID,
		EventType:     EventPilotCertificationUpdated,
		AccountID:     accountID,
		Payload:       p,
		Source:        "pilot.certification",
	})
}

func EmitAutoDownscale(ctx context.Context, store ledger.Store, accountID string, p AutoDownscalePayload) {
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   p.PilotID,
		EventType:     EventPilotAutoDownscale,
		AccountID:     accountID,
		StrategyID:    p.StrategyID,
		Payload:       p,
		Source:        "pilot.capital",
	})
}

func EmitMetricsSnapshot(ctx context.Context, store ledger.Store, accountID string, p MetricsSnapshotPayload) {
	emitPilotEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregateMetrics,
		AggregateID:   p.PilotID,
		EventType:     EventPilotMetricsSnapshotted,
		AccountID:     accountID,
		Payload:       p,
		Source:        "pilot.metrics",
	})
}

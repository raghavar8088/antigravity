package riskv3

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/ledger"
)

// ─── Risk event payloads ──────────────────────────────────────────────────────

// RiskCheckPayload is attached to EventRiskApproved and EventRiskBlocked
// when emitted by Risk Engine V3. Extends the v1 payload with portfolio metrics.
type RiskCheckPayload struct {
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	RequestedSize float64 `json:"requested_size"`
	ApprovedSize  float64 `json:"approved_size"`

	// Snapshot of key risk metrics at decision time
	HeatPct          float64 `json:"heat_pct"`
	VaR95Pct         float64 `json:"var_95_pct"`
	CVaR95Pct        float64 `json:"cvar_95_pct"`
	DrawdownPct      float64 `json:"drawdown_pct"`
	DailyLossPct     float64 `json:"daily_loss_pct"`
	CorrelationRisk  float64 `json:"correlation_risk"`
	ConcentrationScore float64 `json:"concentration_score"`
	RiskScore        int     `json:"risk_score"`

	Reason     string   `json:"reason,omitempty"`
	Violations []string `json:"violations,omitempty"` // violation type strings

	Strategy  string    `json:"strategy"`
	CheckedAt time.Time `json:"checked_at"`
}

// PortfolioSnapshotPayload is attached to EventPortfolioSnapshot events
// emitted periodically (e.g. every 5 minutes) for analytics.
type PortfolioSnapshotPayload struct {
	EquityUSD          float64   `json:"equity_usd"`
	HeatPct            float64   `json:"heat_pct"`
	DailyVaR95Pct      float64   `json:"daily_var_95_pct"`
	CVaR95Pct          float64   `json:"cvar_95_pct"`
	DrawdownPct        float64   `json:"drawdown_pct"`
	DailyLossPct       float64   `json:"daily_loss_pct"`
	OpenPositions      int       `json:"open_positions"`
	GrossExposurePct   float64   `json:"gross_exposure_pct"`
	MaxSymbolConc      float64   `json:"max_symbol_concentration_pct"`
	MaxCorrelation     float64   `json:"max_pairwise_correlation"`
	RiskScore          int       `json:"risk_score"`
	SnapshotAt         time.Time `json:"snapshot_at"`
}

// ─── RiskAggregate ────────────────────────────────────────────────────────────

// RiskAggregateState is the replayable state of the portfolio risk aggregate.
// It is rebuilt exclusively from ledger events — no direct field assignment.
// Every field can be derived from the event log alone.
type RiskAggregateState struct {
	AccountID string `json:"account_id"`

	// Decision counters
	TotalChecks    int64 `json:"total_checks"`
	TotalApproved  int64 `json:"total_approved"`
	TotalBlocked   int64 `json:"total_blocked"`
	ViolationCount int64 `json:"violation_count"`
	KillSwitchHits int64 `json:"kill_switch_hits"`

	// Last observed portfolio metrics (from most recent PortfolioSnapshot event)
	LastHeatPct        float64 `json:"last_heat_pct"`
	LastVaR95Pct       float64 `json:"last_var_95_pct"`
	LastCVaR95Pct      float64 `json:"last_cvar_95_pct"`
	LastDrawdownPct    float64 `json:"last_drawdown_pct"`
	LastDailyLossPct   float64 `json:"last_daily_loss_pct"`
	LastRiskScore      int     `json:"last_risk_score"`
	LastOpenPositions  int     `json:"last_open_positions"`

	// Watermarks
	PeakHeatPct     float64 `json:"peak_heat_pct"`
	PeakVaR95Pct    float64 `json:"peak_var_95_pct"`
	PeakDrawdownPct float64 `json:"peak_drawdown_pct"`
	PeakDailyLoss   float64 `json:"peak_daily_loss"`

	// Version (count of events applied)
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ApplyRiskEvent updates the aggregate state for a single ledger event.
// Only RISK aggregate events are accepted.
func ApplyRiskEvent(state *RiskAggregateState, event ledger.Event) error {
	if event.AggregateType != ledger.AggregateRisk {
		return fmt.Errorf("riskv3: expected RISK event, got %s", event.AggregateType)
	}

	switch event.EventType {
	case ledger.EventRiskApproved:
		var payload RiskCheckPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state.TotalChecks++
			state.TotalApproved++
			updateWatermarks(state, payload)
		}

	case ledger.EventRiskBlocked, ledger.EventRiskViolation:
		var payload RiskCheckPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil {
			state.TotalChecks++
			state.TotalBlocked++
			if len(payload.Violations) > 0 {
				state.ViolationCount++
			}
			updateWatermarks(state, payload)
		}

	case ledger.EventKillSwitchTriggered:
		state.KillSwitchHits++

	case ledger.EventPortfolioHeatExceeded,
		ledger.EventVaRBreach,
		ledger.EventCVaRBreach,
		ledger.EventMaxDrawdownBreached,
		ledger.EventExposureLimitExceeded:
		state.ViolationCount++
	}

	state.Version++
	state.UpdatedAt = event.CreatedAt
	return nil
}

// ReplayRiskAggregate rebuilds the RiskAggregateState by replaying all RISK
// events for the given account from the ledger.
func ReplayRiskAggregate(ctx context.Context, store ledger.Store, accountID string) (RiskAggregateState, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return RiskAggregateState{}, fmt.Errorf("riskv3.ReplayRiskAggregate: %w", err)
	}
	state := RiskAggregateState{AccountID: accountID}
	for _, event := range events {
		if event.AggregateType != ledger.AggregateRisk {
			continue
		}
		if err := ApplyRiskEvent(&state, event); err != nil {
			log.Printf("[RISK V3] ReplayRiskAggregate: skipping event %s: %v", event.EventID, err)
		}
	}
	return state, nil
}

// ─── Risk event emission helpers ─────────────────────────────────────────────

// EmitRiskApproved appends EventRiskApproved to the ledger for an approved order.
func EmitRiskApproved(ctx context.Context, store ledger.Store, accountID, clientOrderID string, decision OrderDecision, strategyName string) {
	payload := buildCheckPayload(clientOrderID, decision, strategyName)
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateRisk,
		AggregateID:   "portfolio-" + accountID,
		AccountID:     accountID,
		EventType:     ledger.EventRiskApproved,
		StrategyID:    strategyName,
		Payload:       payload,
		Source:        "riskv3-engine",
	})
	if err != nil {
		log.Printf("[RISK V3] EmitRiskApproved: build event: %v", err)
		return
	}
	if _, err := store.Append(ctx, event); err != nil {
		log.Printf("[RISK V3] EmitRiskApproved: append: %v", err)
	}
}

// EmitRiskBlocked appends EventRiskBlocked to the ledger for a rejected order.
func EmitRiskBlocked(ctx context.Context, store ledger.Store, accountID, clientOrderID string, decision OrderDecision, strategyName string) {
	payload := buildCheckPayload(clientOrderID, decision, strategyName)
	eventType := ledger.EventRiskBlocked
	if len(decision.Violations) > 0 {
		eventType = ledger.EventRiskViolation
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateRisk,
		AggregateID:   "portfolio-" + accountID,
		AccountID:     accountID,
		EventType:     eventType,
		StrategyID:    strategyName,
		Payload:       payload,
		Source:        "riskv3-engine",
	})
	if err != nil {
		log.Printf("[RISK V3] EmitRiskBlocked: build event: %v", err)
		return
	}
	if _, err := store.Append(ctx, event); err != nil {
		log.Printf("[RISK V3] EmitRiskBlocked: append: %v", err)
	}
}

// EmitPortfolioSnapshot appends a periodic portfolio risk snapshot event.
// Called by the engine on a timer (e.g. every 5 minutes) to provide a
// recoverable history of portfolio metrics for the dashboard.
func EmitPortfolioSnapshot(ctx context.Context, store ledger.Store, accountID string, metrics PortfolioMetrics) {
	payload := PortfolioSnapshotPayload{
		EquityUSD:        metrics.EquityUSD,
		HeatPct:          metrics.HeatPct,
		DailyVaR95Pct:    metrics.DailyVaR95Pct,
		CVaR95Pct:        metrics.CVaR95Pct,
		DrawdownPct:      metrics.DrawdownPct,
		DailyLossPct:     metrics.DailyLossPct,
		OpenPositions:    metrics.OpenPositions,
		GrossExposurePct: metrics.GrossExposurePct,
		MaxSymbolConc:    metrics.MaxSymbolConcentrationPct,
		MaxCorrelation:   metrics.MaxPairwiseCorr,
		RiskScore:        metrics.RiskScore,
		SnapshotAt:       metrics.ComputedAt,
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateRisk,
		AggregateID:   "portfolio-" + accountID,
		AccountID:     accountID,
		EventType:     ledger.EventRiskCheckStarted, // re-use as "portfolio checkpoint"
		Payload:       payload,
		Source:        "riskv3-engine",
	})
	if err != nil {
		log.Printf("[RISK V3] EmitPortfolioSnapshot: build event: %v", err)
		return
	}
	if _, err := store.Append(ctx, event); err != nil {
		log.Printf("[RISK V3] EmitPortfolioSnapshot: append: %v", err)
	}
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func buildCheckPayload(clientOrderID string, decision OrderDecision, strategyName string) RiskCheckPayload {
	violationTypes := make([]string, len(decision.Violations))
	for i, v := range decision.Violations {
		violationTypes[i] = string(v.Type)
	}
	return RiskCheckPayload{
		ClientOrderID:      clientOrderID,
		ApprovedSize:       decision.ApprovedSize,
		RequestedSize:      decision.RequestedSize,
		HeatPct:            decision.HeatPct,
		VaR95Pct:           decision.VaR95Pct,
		CVaR95Pct:          decision.CVaR95Pct,
		DrawdownPct:        decision.DrawdownPct,
		DailyLossPct:       decision.DailyLossPct,
		CorrelationRisk:    decision.CorrelationRisk,
		ConcentrationScore: decision.ConcentrationRisk,
		RiskScore:          decision.RiskScore,
		Reason:             decision.Reason,
		Violations:         violationTypes,
		Strategy:           strategyName,
		CheckedAt:          decision.CheckedAt,
	}
}

func updateWatermarks(state *RiskAggregateState, payload RiskCheckPayload) {
	state.LastHeatPct = payload.HeatPct
	state.LastVaR95Pct = payload.VaR95Pct
	state.LastCVaR95Pct = payload.CVaR95Pct
	state.LastDrawdownPct = payload.DrawdownPct
	state.LastDailyLossPct = payload.DailyLossPct
	state.LastRiskScore = payload.RiskScore

	if payload.HeatPct > state.PeakHeatPct {
		state.PeakHeatPct = payload.HeatPct
	}
	if payload.VaR95Pct > state.PeakVaR95Pct {
		state.PeakVaR95Pct = payload.VaR95Pct
	}
	if payload.DrawdownPct > state.PeakDrawdownPct {
		state.PeakDrawdownPct = payload.DrawdownPct
	}
	if payload.DailyLossPct > state.PeakDailyLoss {
		state.PeakDailyLoss = payload.DailyLossPct
	}
}

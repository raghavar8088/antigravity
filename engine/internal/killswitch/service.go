package killswitch

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

type Trigger string

const (
	TriggerDailyLoss          Trigger = "DAILY_LOSS_BREACH"
	TriggerExchangeOutage     Trigger = "EXCHANGE_OUTAGE"
	TriggerDataFeedOutage     Trigger = "DATA_FEED_OUTAGE"
	TriggerOMSDesync          Trigger = "OMS_DESYNC"
	TriggerRiskServiceFailure Trigger = "RISK_SERVICE_FAILURE"
	TriggerPositionDrift      Trigger = "LARGE_POSITION_DRIFT"
	TriggerFundingShock       Trigger = "FUNDING_SHOCK"
	TriggerLiquidationSpike   Trigger = "LIQUIDATION_EVENT_SPIKE"
	TriggerManualOperator     Trigger = "MANUAL_OPERATOR_TRIGGER"
)

type Action string

const (
	ActionCancelOpenOrders Action = "CANCEL_OPEN_ORDERS"
	ActionBlockNewOrders   Action = "BLOCK_NEW_ORDERS"
	ActionFlattenPositions Action = "FLATTEN_POSITIONS"
	ActionSendAlerts       Action = "SEND_ALERTS"
)

type Executor interface {
	CancelOpenOrders(ctx context.Context, reason string) error
	FlattenPositions(ctx context.Context, reason string) error
	SendAlert(ctx context.Context, event Activation) error
}

type Activation struct {
	Trigger     Trigger   `json:"trigger"`
	Reason      string    `json:"reason"`
	OperatorID  string    `json:"operator_id,omitempty"`
	Actions     []Action  `json:"actions"`
	ActivatedAt time.Time `json:"activated_at"`
}

type Service struct {
	mu        sync.RWMutex
	active    bool
	reason    string
	ledger    ledger.Store
	executor  Executor
	accountID string
}

func NewService(store ledger.Store, executor Executor, accountID string) *Service {
	return &Service{ledger: store, executor: executor, accountID: accountID}
}

// RestoreFromLedger replays kill-switch events to restore in-memory active state
// after engine restart. When a stale OMS_DESYNC was caused by reconciliation false
// positives, auto-releases so trading can resume without manual operator action.
func (s *Service) RestoreFromLedger(ctx context.Context) error {
	if s.ledger == nil {
		return nil
	}
	events, err := s.ledger.ReplayAccount(ctx, s.accountID)
	if err != nil {
		return err
	}
	active := false
	reason := ""
	var lastTrigger Trigger
	for _, ev := range events {
		if ev.AggregateType != ledger.AggregateRisk {
			continue
		}
		switch ev.EventType {
		case ledger.EventKillSwitchTriggered:
			var act Activation
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &act)
			}
			active = true
			reason = act.Reason
			lastTrigger = act.Trigger
		case ledger.EventKillSwitchReleased:
			active = false
			reason = ""
		}
	}
	s.mu.Lock()
	s.active = active
	s.reason = reason
	s.mu.Unlock()

	if active && shouldAutoReleaseReconFalsePositive(reason) {
		log.Printf("[KILL SWITCH] auto-releasing stale kill switch from reconciliation false positive: %s", reason)
		return s.Release(ctx, lastTrigger, "startup-healer", "auto-release reconciliation false positive")
	}
	if active {
		log.Printf("[KILL SWITCH] restored active state from ledger: %s", reason)
	}
	return nil
}

func shouldAutoReleaseReconFalsePositive(reason string) bool {
	r := strings.ToLower(reason)
	if !strings.Contains(r, "reconciliation") {
		return false
	}
	for _, marker := range []string{"equity_drift", "available_margin_drift", "ghost_position", "missing_position"} {
		if strings.Contains(r, marker) {
			return true
		}
	}
	return false
}

// RestoreStateOnStartup checks the ledger for a persisted kill switch activation
// from a previous session. If found and not auto-released, the engine starts HALTED.
// Returns true if the kill switch was restored as active (engine will start halted).
// Call this BEFORE starting the trading loop.
func (s *Service) RestoreStateOnStartup(ctx context.Context) bool {
	if err := s.RestoreFromLedger(ctx); err != nil {
		log.Printf("[KILL SWITCH] RestoreStateOnStartup: failed to restore from ledger: %v — starting normally", err)
		return false
	}
	s.mu.RLock()
	active := s.active
	reason := s.reason
	s.mu.RUnlock()
	if active {
		log.Printf("[KILL SWITCH] ⚠️  RESTORED HALTED STATE from previous session reason=%q action_required='POST /api/admin/ks/release to resume trading'", reason)
	} else {
		log.Printf("[KILL SWITCH] no persisted activation found — starting normally")
	}
	return active
}

func (s *Service) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

func (s *Service) Reason() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reason
}

func (s *Service) Trigger(ctx context.Context, activation Activation) error {
	if activation.Trigger == "" {
		return errors.New("killswitch: trigger is required")
	}
	if activation.ActivatedAt.IsZero() {
		activation.ActivatedAt = time.Now().UTC()
	}
	if len(activation.Actions) == 0 {
		activation.Actions = []Action{ActionCancelOpenOrders, ActionBlockNewOrders, ActionSendAlerts}
	}
	if err := s.persist(ctx, activation); err != nil {
		return err
	}

	s.mu.Lock()
	s.active = true
	s.reason = activation.Reason
	s.mu.Unlock()

	if s.executor == nil {
		return nil
	}
	for _, action := range activation.Actions {
		switch action {
		case ActionCancelOpenOrders:
			if err := s.executor.CancelOpenOrders(ctx, activation.Reason); err != nil {
				return err
			}
		case ActionFlattenPositions:
			if err := s.executor.FlattenPositions(ctx, activation.Reason); err != nil {
				return err
			}
		case ActionSendAlerts:
			if err := s.executor.SendAlert(ctx, activation); err != nil {
				return err
			}
		}
	}
	return nil
}

// Release deactivates the kill switch and emits EventKillSwitchReleased to
// the ledger. Requires manual operator action — the switch never self-releases.
// releasedBy identifies the operator or service performing the release.
func (s *Service) Release(ctx context.Context, originalTrigger Trigger, releasedBy, reason string) error {
	if s.ledger != nil {
		payload := ledger.KillSwitchReleasedPayload{
			OriginalTrigger: string(originalTrigger),
			ReleasedBy:      releasedBy,
			Reason:          reason,
		}
		event, err := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateRisk,
			AggregateID:   string(originalTrigger),
			AccountID:     s.accountID,
			EventType:     ledger.EventKillSwitchReleased,
			Payload:       payload,
			Source:        "kill-switch-service",
		})
		if err != nil {
			return err
		}
		if _, err := s.ledger.Append(ctx, event); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.active = false
	s.reason = ""
	s.mu.Unlock()
	return nil
}

func (s *Service) ResetForTest() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
	s.reason = ""
}

func (s *Service) persist(ctx context.Context, activation Activation) error {
	if s.ledger == nil {
		return nil
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateRisk,
		AggregateID:   string(activation.Trigger),
		AccountID:     s.accountID,
		EventType:     ledger.EventKillSwitchTriggered,
		Payload:       activation,
		Source:        "kill-switch-service",
	})
	if err != nil {
		return err
	}
	_, err = s.ledger.Append(ctx, event)
	return err
}

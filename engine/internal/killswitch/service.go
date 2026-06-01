package killswitch

import (
	"context"
	"errors"
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

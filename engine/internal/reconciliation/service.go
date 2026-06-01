package reconciliation

import (
	"context"
	"time"

	"antigravity-engine/internal/ledger"
)

type Snapshot struct {
	OMSOrders         []OMSOrder
	ExchangeOrders    []ExchangeOrder
	OMSPositions      []OMSPosition
	ExchangePositions []ExchangePosition
	OMSBalance        BalanceSnapshot
	ExchangeBalance   BalanceSnapshot
	AccountID         string
}

type SnapshotProvider interface {
	Snapshot(ctx context.Context) (Snapshot, error)
}

type Service struct {
	provider  SnapshotProvider
	ledger    ledger.Store
	orders    OrderMismatchDetector
	positions PositionDriftDetector
	balances  BalanceDriftDetector
	interval  time.Duration
}

func NewService(provider SnapshotProvider, store ledger.Store, interval time.Duration) *Service {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Service{
		provider:  provider,
		ledger:    store,
		orders:    OrderMismatchDetector{StaleAfter: 30 * time.Second},
		positions: PositionDriftDetector{ToleranceBTC: 1e-8},
		balances:  BalanceDriftDetector{ToleranceUSD: 1},
		interval:  interval,
	}
}

func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = s.Check(ctx)
		}
	}
}

func (s *Service) Check(ctx context.Context) ([]Alert, error) {
	snapshot, err := s.provider.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	alerts := []Alert{}
	alerts = append(alerts, s.orders.Detect(snapshot.OMSOrders, snapshot.ExchangeOrders, now)...)
	alerts = append(alerts, s.positions.Detect(snapshot.OMSPositions, snapshot.ExchangePositions, now)...)
	alerts = append(alerts, s.balances.Detect(snapshot.OMSBalance, snapshot.ExchangeBalance, now)...)
	for _, alert := range alerts {
		if err := s.appendAlert(ctx, snapshot.AccountID, alert); err != nil {
			return alerts, err
		}
	}
	return alerts, nil
}

func (s *Service) appendAlert(ctx context.Context, accountID string, alert Alert) error {
	if s.ledger == nil {
		return nil
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateReconciliation,
		AggregateID:   alert.Reference,
		AccountID:     accountID,
		EventType:     ledger.EventReconciliationAlert,
		Symbol:        alert.Symbol,
		Payload:       alert,
		Source:        "reconciliation-service",
	})
	if err != nil {
		return err
	}
	_, err = s.ledger.Append(ctx, event)
	return err
}

// Resolve records that a previously detected mismatch has been repaired.
// Every repair emits EventReconciliationResolved with the full before/after state
// so the permanent audit trail reflects both the problem and its resolution.
func (s *Service) Resolve(
	ctx context.Context,
	accountID string,
	alertReference string,
	alertType string,
	expectedState string,
	actualState string,
	repairAction string,
	repairResult string,
	resolvedBy string,
) error {
	if s.ledger == nil {
		return nil
	}
	payload := ledger.ReconciliationResolvedPayload{
		AlertReference: alertReference,
		AlertType:      alertType,
		ExpectedState:  expectedState,
		ActualState:    actualState,
		RepairAction:   repairAction,
		RepairResult:   repairResult,
		ResolvedBy:     resolvedBy,
	}
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateReconciliation,
		AggregateID:   alertReference,
		AccountID:     accountID,
		EventType:     ledger.EventReconciliationResolved,
		Payload:       payload,
		Source:        "reconciliation-service",
	})
	if err != nil {
		return err
	}
	_, err = s.ledger.Append(ctx, event)
	return err
}

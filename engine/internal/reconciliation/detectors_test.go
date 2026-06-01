package reconciliation

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

func TestOrderMismatchDetectorFindsMissingFillAndDuplicateOrder(t *testing.T) {
	now := time.Now().UTC()
	detector := OrderMismatchDetector{StaleAfter: time.Minute}
	alerts := detector.Detect(
		[]OMSOrder{{ClientOrderID: "cl-1", ExchangeOrderID: "ex-1", Symbol: "BTCUSDT", State: OrderStateAcknowledged, Quantity: 1, FilledQuantity: 0.1, UpdatedAt: now}},
		[]ExchangeOrder{
			{ClientOrderID: "cl-1", ExchangeOrderID: "ex-1", Symbol: "BTCUSDT", Quantity: 1, FilledQuantity: 0.5},
			{ClientOrderID: "cl-1", ExchangeOrderID: "ex-2", Symbol: "BTCUSDT", Quantity: 1, FilledQuantity: 0},
		},
		now,
	)
	if len(alerts) != 2 {
		t.Fatalf("expected 2 alerts, got %d: %#v", len(alerts), alerts)
	}
}

func TestPositionDriftDetectorFindsDrift(t *testing.T) {
	alerts := PositionDriftDetector{ToleranceBTC: 0.001}.Detect(
		[]OMSPosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.2}},
		[]ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.15}},
		time.Now().UTC(),
	)
	if len(alerts) != 1 || alerts[0].Type != AlertPositionDrift {
		t.Fatalf("expected position drift alert, got %#v", alerts)
	}
}

func TestServiceAppendsReconciliationAlertEvent(t *testing.T) {
	store := ledger.NewMemoryStore()
	provider := staticProvider{snapshot: Snapshot{
		AccountID:       "acct-1",
		OMSBalance:      BalanceSnapshot{EquityUSD: 100, CashUSD: 100},
		ExchangeBalance: BalanceSnapshot{EquityUSD: 90, CashUSD: 90},
	}}
	service := NewService(provider, store, time.Second)

	alerts, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	events, err := store.Replay(context.Background(), ledger.AggregateReconciliation, "account")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(events) != 1 || events[0].EventType != ledger.EventReconciliationAlert {
		t.Fatalf("expected reconciliation alert event, got %#v", events)
	}
}

type staticProvider struct {
	snapshot Snapshot
}

func (s staticProvider) Snapshot(context.Context) (Snapshot, error) {
	return s.snapshot, nil
}

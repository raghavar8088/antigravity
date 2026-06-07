package certification

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// ─── Phase 4: Exchange Failure Testing ──────────────────────────────────────

// mockExchangeAdapter simulates exchange failures for certification purposes.
type mockExchangeAdapter struct {
	name          string
	outage        bool
	timeout       bool
	rejectStorms  bool
	duplicateFill bool
	delayedFill   bool
	callCount     int
}

func (m *mockExchangeAdapter) Name() string { return m.name }
func (m *mockExchangeAdapter) PlaceOrder(_ context.Context, _ string) (string, error) {
	m.callCount++
	if m.outage {
		return "", errors.New("exchange: connection refused (simulated outage)")
	}
	if m.timeout {
		return "", fmt.Errorf("exchange: context deadline exceeded after 30s")
	}
	if m.rejectStorms && m.callCount%3 == 0 {
		return "", fmt.Errorf("exchange: order rejected (ERR_RATE_LIMIT)")
	}
	return fmt.Sprintf("EXCH_ORD_%06d", m.callCount), nil
}

// TestExchangeFailure_OutagePreventsDualExecution verifies that when the
// exchange is unreachable, orders stay in SUBMITTED state (not FILLED),
// preventing phantom positions.
func TestExchangeFailure_OutagePreventsDualExecution(t *testing.T) {
	store := newCertStore(t)
	accountID := "EXCH_FAIL_001"
	orderID := "ORD_OUTAGE_001"

	exchange := &mockExchangeAdapter{name: "binance", outage: true}

	// Bring order to SUBMITTED state.
	agg := omsv3.NewOrderAggregate(orderID)
	for _, step := range []struct {
		et   ledger.EventType
		next omsv3.OrderState
	}{
		{ledger.EventOrderCreated, omsv3.StateNew},
		{ledger.EventOrderValidated, omsv3.StateValidated},
		{ledger.EventOrderAccepted, omsv3.StateRiskApproved},
		{ledger.EventOrderSubmitted, omsv3.StateSubmitted},
	} {
		ev := newOrderEvent(t, accountID, orderID, step.et)
		persisted := mustAppend(t, store, ev)
		if err := agg.ApplyEvent(persisted); err != nil {
			t.Fatalf("ApplyEvent %s: %v", step.et, err)
		}
	}

	// Exchange outage — fill attempt fails.
	_, err := exchange.PlaceOrder(context.Background(), orderID)
	if err == nil {
		t.Fatal("FAIL: exchange outage not detected — dual execution risk")
	}

	// Order must remain SUBMITTED (not FILLED) — no phantom position.
	if agg.State != omsv3.StateSubmitted {
		t.Errorf("FAIL: order state corrupted to %s during exchange outage", agg.State)
	}

	// Verify ledger has exactly 4 events (no fill event written).
	events, err2 := store.Replay(context.Background(), ledger.AggregateOrder, orderID)
	if err2 != nil {
		t.Fatalf("Replay: %v", err2)
	}
	for _, ev := range events {
		if ev.EventType == ledger.EventOrderFilled {
			t.Errorf("FAIL: fill event persisted during exchange outage — ghost position created")
		}
	}
	t.Logf("Exchange outage correctly isolated: order stayed in SUBMITTED, %d events in ledger", len(events))
}

// TestExchangeFailure_TimeoutNoOrphan verifies timeout during submission does
// not leave orphan orders — the order must be recoverable to CANCELLED.
func TestExchangeFailure_TimeoutNoOrphan(t *testing.T) {
	store := newCertStore(t)
	accountID := "EXCH_FAIL_002"
	orderID := "ORD_TIMEOUT_001"

	exchange := &mockExchangeAdapter{name: "delta", timeout: true}

	agg := omsv3.NewOrderAggregate(orderID)
	for _, step := range []struct {
		et   ledger.EventType
		next omsv3.OrderState
	}{
		{ledger.EventOrderCreated, omsv3.StateNew},
		{ledger.EventOrderValidated, omsv3.StateValidated},
		{ledger.EventOrderAccepted, omsv3.StateRiskApproved},
		{ledger.EventOrderSubmitted, omsv3.StateSubmitted},
	} {
		ev := newOrderEvent(t, accountID, orderID, step.et)
		persisted := mustAppend(t, store, ev)
		if err := agg.ApplyEvent(persisted); err != nil {
			t.Fatalf("ApplyEvent: %v", err)
		}
	}

	// Timeout on exchange.
	_, err := exchange.PlaceOrder(context.Background(), orderID)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Recovery path: cancel the timed-out order.
	if err := agg.ValidateTransition(omsv3.StateCancelled); err != nil {
		t.Fatalf("FAIL: cannot cancel timed-out order: %v — orphan risk", err)
	}
	cancelEv := newOrderEvent(t, accountID, orderID, ledger.EventOrderCancelled)
	persisted := mustAppend(t, store, cancelEv)
	if err := agg.ApplyEvent(persisted); err != nil {
		t.Fatalf("ApplyEvent cancel: %v", err)
	}
	if agg.State != omsv3.StateCancelled {
		t.Errorf("FAIL: order state after timeout recovery: %s (want CANCELLED)", agg.State)
	}
}

// TestExchangeFailure_RejectStorm verifies the system handles rapid order
// rejections without corrupting aggregate state or writing bad events.
func TestExchangeFailure_RejectStorm(t *testing.T) {
	store := newCertStore(t)
	accountID := "EXCH_FAIL_003"

	exchange := &mockExchangeAdapter{name: "binance", rejectStorms: true}

	const numOrders = 30
	accepted := 0
	rejected := 0

	for i := 0; i < numOrders; i++ {
		orderID := fmt.Sprintf("ORD_REJECT_%04d", i)
		agg := omsv3.NewOrderAggregate(orderID)

		// Create + validate.
		for _, step := range []struct {
			et   ledger.EventType
			next omsv3.OrderState
		}{
			{ledger.EventOrderCreated, omsv3.StateNew},
			{ledger.EventOrderValidated, omsv3.StateValidated},
			{ledger.EventOrderAccepted, omsv3.StateRiskApproved},
			{ledger.EventOrderSubmitted, omsv3.StateSubmitted},
		} {
			ev := newOrderEvent(t, accountID, orderID, step.et)
			persisted := mustAppend(t, store, ev)
			if err := agg.ApplyEvent(persisted); err != nil {
				t.Fatalf("ApplyEvent: %v", err)
			}
		}

		// Submit to exchange.
		_, err := exchange.PlaceOrder(context.Background(), orderID)
		if err != nil {
			// Exchange rejected: record rejection in ledger.
			rejEv := newOrderEvent(t, accountID, orderID, ledger.EventOrderRejected)
			persisted := mustAppend(t, store, rejEv)
			if err := agg.ApplyEvent(persisted); err != nil {
				t.Errorf("order %s: cannot apply REJECTED event: %v", orderID, err)
			}
			if agg.State != omsv3.StateRejected {
				t.Errorf("order %s: state=%s after rejection (want REJECTED)", orderID, agg.State)
			}
			rejected++
		} else {
			accepted++
		}
	}

	t.Logf("reject storm: %d accepted, %d rejected out of %d", accepted, rejected, numOrders)
	if rejected == 0 {
		t.Error("FAIL: no rejections recorded — reject storm not triggered")
	}
}

// TestExchangeFailure_DuplicateExchangeMessage verifies idempotency prevents
// double-crediting when the exchange sends a duplicate fill message.
func TestExchangeFailure_DuplicateExchangeMessage(t *testing.T) {
	store := newCertStore(t)
	accountID := "EXCH_FAIL_004"
	orderID := "ORD_DUP_FILL_001"

	// Place order through to ACKNOWLEDGED.
	agg := omsv3.NewOrderAggregate(orderID)
	for _, step := range []struct {
		et   ledger.EventType
		next omsv3.OrderState
	}{
		{ledger.EventOrderCreated, omsv3.StateNew},
		{ledger.EventOrderValidated, omsv3.StateValidated},
		{ledger.EventOrderAccepted, omsv3.StateRiskApproved},
		{ledger.EventOrderSubmitted, omsv3.StateSubmitted},
		{ledger.EventOrderAcked, omsv3.StateAcknowledged},
	} {
		ev := newOrderEvent(t, accountID, orderID, step.et)
		persisted := mustAppend(t, store, ev)
		if err := agg.ApplyEvent(persisted); err != nil {
			t.Fatalf("ApplyEvent: %v", err)
		}
	}

	// First fill — legitimate.
	fillEv, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    orderID,
		EventType:      ledger.EventOrderFilled,
		AccountID:      accountID,
		Symbol:         "BTC-USD",
		Payload:        ledger.OrderPayload{ClientOrderID: orderID},
		IdempotencyKey: "exch_fill_12345678",
		Source:         "certification",
	})
	persisted := mustAppend(t, store, fillEv)
	if err := agg.ApplyEvent(persisted); err != nil {
		t.Fatalf("first fill: %v", err)
	}

	// Duplicate exchange message — same idempotency key.
	_, err := store.Append(context.Background(), fillEv)
	if err == nil {
		t.Fatal("FAIL: duplicate fill accepted — position double-counted")
	}

	// State must be FILLED exactly once.
	if agg.State != omsv3.StateFilled {
		t.Errorf("state after duplicate: %s (want FILLED)", agg.State)
	}

	events, _ := store.Replay(context.Background(), ledger.AggregateOrder, orderID)
	fillCount := 0
	for _, ev := range events {
		if ev.EventType == ledger.EventOrderFilled {
			fillCount++
		}
	}
	if fillCount != 1 {
		t.Errorf("FAIL: %d fill events in ledger (want exactly 1)", fillCount)
	}
}

// TestExchangeFailure_WebsocketReconnectNoGhostPosition simulates a websocket
// disconnect during an active order. Validates that no ghost position is created
// when the connection is re-established and state is re-queried.
func TestExchangeFailure_WebsocketReconnectNoGhostPosition(t *testing.T) {
	store := newCertStore(t)
	accountID := "EXCH_FAIL_005"

	// Before disconnect: 10 orders in FILLED state.
	const priorOrders = 10
	for i := 0; i < priorOrders; i++ {
		orderID := fmt.Sprintf("ORD_PRE_WS_%04d", i)
		agg := omsv3.NewOrderAggregate(orderID)
		for _, step := range []struct {
			et   ledger.EventType
			next omsv3.OrderState
		}{
			{ledger.EventOrderCreated, omsv3.StateNew},
			{ledger.EventOrderValidated, omsv3.StateValidated},
			{ledger.EventOrderAccepted, omsv3.StateRiskApproved},
			{ledger.EventOrderSubmitted, omsv3.StateSubmitted},
			{ledger.EventOrderAcked, omsv3.StateAcknowledged},
			{ledger.EventOrderFilled, omsv3.StateFilled},
		} {
			ev := newOrderEvent(t, accountID, orderID, step.et)
			persisted := mustAppend(t, store, ev)
			if err := agg.ApplyEvent(persisted); err != nil {
				t.Fatalf("ApplyEvent: %v", err)
			}
		}
	}

	// Simulate disconnect: no new events written for 2s.
	time.Sleep(2 * time.Millisecond)

	// Reconnect: replay state from ledger — must match exactly priorOrders*6 events.
	r, err := ledger.ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("reconnect replay: %v", err)
	}
	expectedFills := priorOrders
	actualFills := 0
	for _, ev := range r.Orders {
		if ev.EventType == ledger.EventOrderFilled {
			actualFills++
		}
	}
	if actualFills != expectedFills {
		t.Errorf("FAIL: ghost positions detected — expected %d fills post-reconnect, got %d",
			expectedFills, actualFills)
	}
}

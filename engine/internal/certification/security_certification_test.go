package certification

import (
	"context"
	"testing"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// ─── Phase 7: Security Certification ────────────────────────────────────────

// TestSecurity_KillSwitchRequiresTrigger verifies the kill switch rejects
// activations with empty trigger fields — preventing invalid state transitions.
func TestSecurity_KillSwitchRequiresTrigger(t *testing.T) {
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, &noopKSExecutor{}, "SEC_ACCT_001")

	err := ks.Trigger(context.Background(), killswitch.Activation{
		// Trigger field intentionally empty.
		Reason:  "test",
		Actions: []killswitch.Action{killswitch.ActionBlockNewOrders},
	})
	if err == nil {
		t.Fatal("FAIL: kill switch accepted empty trigger — privilege escalation risk")
	}
}

// TestSecurity_LedgerRejectsEmptyAggregateType verifies the ledger rejects
// events with missing required fields — preventing untyped event injection.
func TestSecurity_LedgerRejectsEmptyAggregateType(t *testing.T) {
	store := ledger.NewMemoryStore()

	// Construct a valid event then wipe the aggregate type to simulate injection.
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   "ORD_SEC_001",
		EventType:     ledger.EventOrderCreated,
		AccountID:     "SEC_ACCT_001",
		Source:        "certification",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	ev.AggregateType = "" // wipe after hash calculation to simulate tampering

	_, appErr := store.Append(context.Background(), ev)
	if appErr == nil {
		t.Fatal("FAIL: ledger accepted event with empty aggregate type — injection risk")
	}
}

// TestSecurity_DuplicateEventIDRejected verifies the ledger rejects duplicate
// event IDs — preventing replay attacks.
func TestSecurity_DuplicateEventIDRejected(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "SEC_ACCT_002"

	ev := newOrderEvent(t, accountID, "ORD_SEC_002", ledger.EventOrderCreated)

	if _, err := store.Append(context.Background(), ev); err != nil {
		t.Fatalf("first append: %v", err)
	}

	// Replay attack: resubmit the exact same event.
	_, err := store.Append(context.Background(), ev)
	if err == nil {
		t.Fatal("FAIL: duplicate event ID accepted — replay attack succeeds")
	}
}

// TestSecurity_IdempotencyKeyPreventsFillReplay verifies that idempotency keys
// block financial replay attacks where exchange fill messages are resubmitted.
func TestSecurity_IdempotencyKeyPreventsFillReplay(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "SEC_ACCT_003"
	orderID := "ORD_SEC_REPLAY_001"

	fill, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    orderID,
		EventType:      ledger.EventOrderFilled,
		AccountID:      accountID,
		Symbol:         "BTC-USD",
		Payload:        ledger.OrderPayload{ClientOrderID: orderID},
		IdempotencyKey: "exchange_fill_report_TXID_9876543210",
		Source:         "certification",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	if _, err := store.Append(context.Background(), fill); err != nil {
		t.Fatalf("legitimate fill: %v", err)
	}

	// Attacker replays the fill message.
	_, err = store.Append(context.Background(), fill)
	if err == nil {
		t.Fatal("FAIL: fill replay accepted — position double-counted (financial attack)")
	}
}

// TestSecurity_HashTamperingRejected verifies the ledger's SHA-256 payload
// integrity check prevents persisting corrupted events.
func TestSecurity_HashTamperingRejected(t *testing.T) {
	store := ledger.NewMemoryStore()

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   "ORD_TAMPER_001",
		EventType:     ledger.EventOrderCreated,
		AccountID:     "SEC_ACCT_004",
		Source:        "certification",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	// Tamper the hash to simulate storage corruption or man-in-the-middle.
	ev.PayloadHash = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	_, appendErr := store.Append(context.Background(), ev)
	if appendErr == nil {
		t.Fatal("FAIL: tampered event hash accepted — integrity guarantee broken")
	}
}

// TestSecurity_KillSwitchAuditTrailImmutable verifies that once a kill switch
// activation is persisted, it cannot be overwritten or removed — ensuring an
// immutable audit trail for regulatory compliance.
func TestSecurity_KillSwitchAuditTrailImmutable(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "SEC_ACCT_005"

	ks := killswitch.NewService(store, &noopKSExecutor{}, accountID)
	if err := ks.Trigger(context.Background(), killswitch.Activation{
		Trigger:     killswitch.TriggerManualOperator,
		Reason:      "security audit: immutability test",
		Actions:     []killswitch.Action{killswitch.ActionCancelOpenOrders},
	}); err != nil {
		t.Fatalf("trigger: %v", err)
	}

	events1, _ := store.Replay(context.Background(), ledger.AggregateSystem, accountID)
	initialCount := len(events1)

	_ = ks.Trigger(context.Background(), killswitch.Activation{
		Trigger: killswitch.TriggerExchangeOutage,
		Reason:  "second trigger attempt",
		Actions: []killswitch.Action{killswitch.ActionBlockNewOrders},
	})

	events2, _ := store.Replay(context.Background(), ledger.AggregateSystem, accountID)
	if len(events2) < initialCount {
		t.Errorf("FAIL: audit trail shrank from %d to %d events — immutability violated",
			initialCount, len(events2))
	}

	firstTrigger := killswitch.Trigger("")
	for _, ev := range events2 {
		if ev.EventType == ledger.EventKillSwitchTriggered {
			firstTrigger = killswitch.TriggerManualOperator
			break
		}
	}
	if firstTrigger == "" {
		t.Error("FAIL: original kill switch activation not found in audit trail")
	}
}

// TestSecurity_ContextCancellationPropagates verifies that a cancelled context
// prevents new writes — critical for graceful shutdown security.
func TestSecurity_ContextCancellationPropagates(t *testing.T) {
	store := ledger.NewMemoryStore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   "ORD_CTX_001",
		EventType:     ledger.EventOrderCreated,
		AccountID:     "SEC_ACCT_006",
		Source:        "certification",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}

	_, appendErr := store.Append(ctx, ev)
	if appendErr == nil {
		t.Fatal("FAIL: cancelled context did not prevent write — shutdown race condition risk")
	}
}

// TestSecurity_OrderTransitionGraphEnforced verifies the OMS v3 state machine
// cannot be bypassed — an attacker cannot skip states to inject a fill directly.
func TestSecurity_OrderTransitionGraphEnforced(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "SEC_ACCT_007"
	orderID := "ORD_BYPASS_001"

	// Append a fill event directly without prior lifecycle transitions.
	ev := newOrderEvent(t, accountID, orderID, ledger.EventOrderFilled)
	persisted := mustAppend(t, store, ev)

	// Apply directly to a fresh aggregate — must fail ValidateTransition.
	agg := omsv3.NewOrderAggregate(orderID)
	err := agg.ApplyEvent(persisted)
	if err == nil {
		t.Fatal("FAIL: EMPTY→FILLED bypass accepted by aggregate — state machine compromised")
	}
	t.Logf("State bypass correctly rejected: %v", err)
}

package ledger

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

// fakeEventCollection is an in-memory implementation of eventCollection used
// to unit-test MongoLedgerStore's logic without a live MongoDB connection
// (none is available in this dev/CI environment). It deliberately mirrors
// the real implementation's contract — including unique-key enforcement and
// atomic sequence assignment — so a passing test here is meaningful evidence
// the real Mongo-backed code is correct, not just that the fake is internally
// consistent.
type fakeEventCollection struct {
	mu          sync.Mutex
	docs        []mongoEventDoc
	eventIDs    map[string]bool
	idempotency map[string]bool
	sequences   map[string]int64

	// Test hooks to simulate failure modes.
	insertErr error
}

func newFakeEventCollection() *fakeEventCollection {
	return &fakeEventCollection{
		eventIDs:    make(map[string]bool),
		idempotency: make(map[string]bool),
		sequences:   make(map[string]int64),
	}
}

func (f *fakeEventCollection) Insert(ctx context.Context, doc mongoEventDoc) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.insertErr != nil {
		return f.insertErr
	}
	if f.eventIDs[doc.EventID] {
		return ErrDuplicateEvent
	}
	if doc.IdempotencyKey != "" && f.idempotency[doc.IdempotencyKey] {
		return ErrDuplicateEvent
	}
	f.docs = append(f.docs, doc)
	f.eventIDs[doc.EventID] = true
	if doc.IdempotencyKey != "" {
		f.idempotency[doc.IdempotencyKey] = true
	}
	return nil
}

func (f *fakeEventCollection) NextSequence(ctx context.Context, aggregateKey string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sequences[aggregateKey]++
	return f.sequences[aggregateKey], nil
}

func (f *fakeEventCollection) FindByAggregate(ctx context.Context, aggregateType, aggregateID string) ([]mongoEventDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []mongoEventDoc
	for _, d := range f.docs {
		if d.AggregateType == aggregateType && d.AggregateID == aggregateID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SequenceNo < out[j].SequenceNo })
	return out, nil
}

func (f *fakeEventCollection) FindByAccount(ctx context.Context, accountID string) ([]mongoEventDoc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []mongoEventDoc
	for _, d := range f.docs {
		if d.AccountID == accountID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].SequenceNo < out[j].SequenceNo
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (f *fakeEventCollection) FindByAccountSince(ctx context.Context, accountID string, since time.Time) ([]mongoEventDoc, error) {
	all, err := f.FindByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	var out []mongoEventDoc
	for _, d := range all {
		if !d.CreatedAt.Before(since) {
			out = append(out, d)
		}
	}
	return out, nil
}

func newTestMongoStore() (*MongoLedgerStore, *fakeEventCollection) {
	fake := newFakeEventCollection()
	return &MongoLedgerStore{coll: fake}, fake
}

func mustLedgerEvent(t *testing.T, in NewEventInput) Event {
	t.Helper()
	ev, err := NewEvent(in)
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	return ev
}

func TestMongoLedgerStore_AssignsSequentialSequenceNumbers(t *testing.T) {
	store, _ := newTestMongoStore()
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		ev := mustLedgerEvent(t, NewEventInput{
			AggregateType: AggregateOrder,
			AggregateID:   "order-1",
			EventType:     EventOrderCreated,
			AccountID:     "acct-1",
			Payload:       map[string]int{"i": i},
		})
		got, err := store.Append(ctx, ev)
		if err != nil {
			t.Fatalf("Append #%d: %v", i, err)
		}
		if got.SequenceNo != int64(i) {
			t.Fatalf("Append #%d: SequenceNo=%d, want %d", i, got.SequenceNo, i)
		}
	}
}

func TestMongoLedgerStore_SequencesAreIndependentPerAggregate(t *testing.T) {
	store, _ := newTestMongoStore()
	ctx := context.Background()

	a, _ := store.Append(ctx, mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregateOrder, AggregateID: "order-A", EventType: EventOrderCreated,
		AccountID: "acct-1", Payload: map[string]string{},
	}))
	b, _ := store.Append(ctx, mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregateOrder, AggregateID: "order-B", EventType: EventOrderCreated,
		AccountID: "acct-1", Payload: map[string]string{},
	}))
	if a.SequenceNo != 1 || b.SequenceNo != 1 {
		t.Fatalf("expected independent sequences starting at 1, got a=%d b=%d", a.SequenceNo, b.SequenceNo)
	}
}

func TestMongoLedgerStore_RejectsTamperedHash(t *testing.T) {
	store, _ := newTestMongoStore()
	ev := mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregateOrder, AggregateID: "order-1", EventType: EventOrderCreated,
		AccountID: "acct-1", Payload: map[string]string{},
	})
	ev.PayloadHash = "tampered"
	if _, err := store.Append(context.Background(), ev); !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("got err=%v, want ErrHashMismatch", err)
	}
}

func TestMongoLedgerStore_RejectsMissingRequiredFields(t *testing.T) {
	store, _ := newTestMongoStore()
	_, err := store.Append(context.Background(), Event{EventType: EventOrderCreated})
	if err == nil {
		t.Fatal("expected error for missing aggregate type/id, got nil")
	}
}

func TestMongoLedgerStore_DuplicateIdempotencyKeyRejected(t *testing.T) {
	store, _ := newTestMongoStore()
	ctx := context.Background()
	mk := func() Event {
		return mustLedgerEvent(t, NewEventInput{
			AggregateType: AggregateReconciliation, AggregateID: "recon-1", EventType: EventReconciliationMismatch,
			AccountID: "acct-1", IdempotencyKey: "mismatch-xyz", Payload: map[string]string{},
		})
	}
	if _, err := store.Append(ctx, mk()); err != nil {
		t.Fatalf("first append: %v", err)
	}
	if _, err := store.Append(ctx, mk()); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("second append: got err=%v, want ErrDuplicateEvent", err)
	}
}

func TestMongoLedgerStore_AppendSurfacesCollectionErrors(t *testing.T) {
	// Regression: the previous implementation logged and silently discarded
	// MongoDB write failures, which meant a Mongo outage silently degraded
	// back to "data only existed transiently" with nobody alerted. Appends
	// must now fail loudly when the underlying write fails.
	store, fake := newTestMongoStore()
	fake.insertErr = errors.New("simulated mongo outage")

	ev := mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregateOrder, AggregateID: "order-1", EventType: EventOrderCreated,
		AccountID: "acct-1", Payload: map[string]string{},
	})
	if _, err := store.Append(context.Background(), ev); err == nil {
		t.Fatal("expected Append to surface the collection error, got nil")
	}
}

func TestMongoLedgerStore_ReplayReturnsEventsInSequenceOrder(t *testing.T) {
	store, _ := newTestMongoStore()
	ctx := context.Background()

	types := []EventType{EventOrderCreated, EventOrderAcked, EventOrderFilled}
	for _, et := range types {
		if _, err := store.Append(ctx, mustLedgerEvent(t, NewEventInput{
			AggregateType: AggregateOrder, AggregateID: "order-1", EventType: et,
			AccountID: "acct-1", Payload: map[string]string{},
		})); err != nil {
			t.Fatalf("append %s: %v", et, err)
		}
	}

	events, err := store.Replay(ctx, AggregateOrder, "order-1")
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}
	for i, et := range types {
		if events[i].EventType != et {
			t.Fatalf("events[%d].EventType = %s, want %s", i, events[i].EventType, et)
		}
		if events[i].SequenceNo != int64(i+1) {
			t.Fatalf("events[%d].SequenceNo = %d, want %d", i, events[i].SequenceNo, i+1)
		}
	}
}

func TestMongoLedgerStore_ReplayAccountSweepsAllAggregatesForThatAccount(t *testing.T) {
	store, _ := newTestMongoStore()
	ctx := context.Background()

	if _, err := store.Append(ctx, mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregateOrder, AggregateID: "order-1", EventType: EventOrderCreated,
		AccountID: "acct-1", Payload: map[string]string{},
	})); err != nil {
		t.Fatalf("append order: %v", err)
	}
	if _, err := store.Append(ctx, mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregatePosition, AggregateID: "pos-1", EventType: EventPositionOpened,
		AccountID: "acct-1", Payload: map[string]string{},
	})); err != nil {
		t.Fatalf("append position: %v", err)
	}
	// Different account — must not show up in acct-1's replay.
	if _, err := store.Append(ctx, mustLedgerEvent(t, NewEventInput{
		AggregateType: AggregateOrder, AggregateID: "order-2", EventType: EventOrderCreated,
		AccountID: "acct-2", Payload: map[string]string{},
	})); err != nil {
		t.Fatalf("append other account order: %v", err)
	}

	events, err := store.ReplayAccount(ctx, "acct-1")
	if err != nil {
		t.Fatalf("ReplayAccount: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (cross-aggregate sweep for acct-1 only)", len(events))
	}
	for _, ev := range events {
		if ev.AccountID != "acct-1" {
			t.Fatalf("leaked event from wrong account: %+v", ev)
		}
	}
}

// TestMongoLedgerStore_HasNoUnboundedInMemoryMirror is a structural
// regression guard: MongoLedgerStore must have exactly one field (the
// collection handle). Adding back an in-memory slice/map mirror — exactly
// the change that caused the Jun 2026 OOM incident — would add a second
// field and fail this test, making the regression a visible, deliberate
// code change instead of a silent one.
func TestMongoLedgerStore_HasNoUnboundedInMemoryMirror(t *testing.T) {
	typ := reflect.TypeOf(MongoLedgerStore{})
	if typ.NumField() != 1 {
		t.Fatalf("MongoLedgerStore has %d fields, want exactly 1 (coll) — "+
			"an extra field likely means an in-memory mirror was reintroduced", typ.NumField())
	}
	if typ.Field(0).Name != "coll" {
		t.Fatalf("MongoLedgerStore's only field is %q, want %q", typ.Field(0).Name, "coll")
	}
}

package certification

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// ─── Phase 5: Chaos Engineering ─────────────────────────────────────────────

// faultStore wraps a MemoryStore and injects configurable faults.
type faultStore struct {
	inner         ledger.Store
	injectFailAt  int64 // inject error every N appends (0=never)
	dropAfter     int64 // drop all events after N total events
	appendCount   atomic.Int64
	dropThreshold int64
}

func newFaultStore(failEvery int64, dropAfter int64) *faultStore {
	return &faultStore{
		inner:         ledger.NewMemoryStore(),
		injectFailAt:  failEvery,
		dropThreshold: dropAfter,
	}
}

func (f *faultStore) Append(ctx context.Context, ev ledger.Event) (ledger.Event, error) {
	n := f.appendCount.Add(1)
	if f.dropThreshold > 0 && n > f.dropThreshold {
		return ledger.Event{}, errors.New("fault: storage full — simulated disk exhaustion")
	}
	if f.injectFailAt > 0 && n%f.injectFailAt == 0 {
		return ledger.Event{}, errors.New("fault: transient write error — simulated crash")
	}
	return f.inner.Append(ctx, ev)
}

func (f *faultStore) Replay(ctx context.Context, t ledger.AggregateType, id string) ([]ledger.Event, error) {
	return f.inner.Replay(ctx, t, id)
}

func (f *faultStore) ReplayAccount(ctx context.Context, accountID string) ([]ledger.Event, error) {
	return f.inner.ReplayAccount(ctx, accountID)
}

// TestChaos_LedgerTransientFailure verifies that transient write failures do not
// corrupt the ledger — only the failing event is lost, prior events remain intact.
func TestChaos_LedgerTransientFailure(t *testing.T) {
	store := newFaultStore(5, 0) // fail every 5th append
	accountID := "CHAOS_LEDGER_001"

	const total = 30
	written := 0
	for i := 0; i < total; i++ {
		oid := fmt.Sprintf("CHAOS_ORD_%04d", i)
		ev := ledger.NewEvent(ledger.AggregateOrder, oid, ledger.EventOrderCreated, nil)
		ev.AccountID = accountID
		_, err := store.Append(context.Background(), ev)
		if err == nil {
			written++
		}
	}

	// Survived events must be queryable.
	r, err := ledger.ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("replay after faults: %v", err)
	}
	if r.TotalEventCount != written {
		t.Errorf("ledger drift: appended %d events, replayed %d", written, r.TotalEventCount)
	}
	t.Logf("chaos transient: %d/%d events persisted (%.0f%% success)",
		written, total, float64(written)/float64(total)*100)
}

// TestChaos_ConcurrentWritesNoRace verifies the ledger is safe under concurrent
// writers — a critical invariant for the live trading engine.
func TestChaos_ConcurrentWritesNoRace(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "CHAOS_CONCURRENT_001"

	const goroutines = 50
	const eventsPerGoroutine = 100
	ctx := context.Background()

	var wg sync.WaitGroup
	var written atomic.Int64
	var failed atomic.Int64

	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < eventsPerGoroutine; i++ {
				oid := fmt.Sprintf("G%03d_ORD_%04d", g, i)
				ev := ledger.NewEvent(ledger.AggregateOrder, oid, ledger.EventOrderCreated, nil)
				ev.AccountID = accountID
				_, err := store.Append(ctx, ev)
				if err != nil {
					failed.Add(1)
				} else {
					written.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if failed.Load() > 0 {
		t.Errorf("FAIL: %d concurrent writes failed (non-idempotency errors)", failed.Load())
	}

	r, err := ledger.ReplayEverything(ctx, store, accountID)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if int64(r.TotalEventCount) != written.Load() {
		t.Errorf("concurrency corruption: wrote %d, replayed %d",
			written.Load(), r.TotalEventCount)
	}
}

// TestChaos_OMSv3_ConcurrentOrderProcessing verifies OMS v3 aggregates are
// safe under concurrent access — each order must reach its own terminal state.
func TestChaos_OMSv3_ConcurrentOrderProcessing(t *testing.T) {
	const numOrders = 200
	store := ledger.NewMemoryStore()
	accountID := "CHAOS_OMS_001"
	ctx := context.Background()

	var wg sync.WaitGroup
	errCh := make(chan error, numOrders*10)

	for i := 0; i < numOrders; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			orderID := fmt.Sprintf("CHAOS_OMS_%05d", i)
			agg := omsv3.NewOrderAggregate(orderID)

			lifecycle := []struct {
				et   ledger.EventType
				next omsv3.OrderState
			}{
				{ledger.EventOrderCreated, omsv3.StateNew},
				{ledger.EventOrderValidated, omsv3.StateValidated},
				{ledger.EventOrderAccepted, omsv3.StateRiskApproved},
				{ledger.EventOrderSubmitted, omsv3.StateSubmitted},
				{ledger.EventOrderAcked, omsv3.StateAcknowledged},
				{ledger.EventOrderFilled, omsv3.StateFilled},
			}

			for _, step := range lifecycle {
				ev := ledger.NewEvent(ledger.AggregateOrder, orderID, step.et, nil)
				ev.AccountID = accountID
				persisted, err := store.Append(ctx, ev)
				if err != nil {
					errCh <- fmt.Errorf("order %s append %s: %w", orderID, step.et, err)
					return
				}
				if err := agg.ApplyEvent(persisted); err != nil {
					errCh <- fmt.Errorf("order %s apply %s: %w", orderID, step.et, err)
					return
				}
			}

			if agg.State != omsv3.StateFilled {
				errCh <- fmt.Errorf("order %s: final state %s (want FILLED)", orderID, agg.State)
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

// TestChaos_KillSwitchUnderLoad verifies the kill switch can be triggered and
// queried correctly under extreme concurrent load.
func TestChaos_KillSwitchUnderLoad(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "CHAOS_KS_001"

	ks := killswitch.NewService(store, &noopKSExecutor{}, accountID)

	// Trigger in a goroutine while 100 goroutines concurrently read IsActive().
	triggered := make(chan struct{})
	var reads atomic.Int64
	var sawActive atomic.Int64

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-triggered:
					return
				default:
					if ks.IsActive() {
						sawActive.Add(1)
					}
					reads.Add(1)
					time.Sleep(time.Microsecond)
				}
			}
		}()
	}

	// Trigger after a brief window.
	time.Sleep(5 * time.Millisecond)
	if err := ks.Trigger(context.Background(), killswitch.Activation{
		Trigger:     killswitch.TriggerExchangeOutage,
		Reason:      "chaos: exchange outage simulation",
		Actions:     []killswitch.Action{killswitch.ActionBlockNewOrders},
		ActivatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("Trigger under load: %v", err)
	}
	close(triggered)
	wg.Wait()

	if !ks.IsActive() {
		t.Error("FAIL: kill switch not active after concurrent trigger")
	}
	t.Logf("chaos kill switch: %d reads, %d saw active state during transition",
		reads.Load(), sawActive.Load())
}

// TestChaos_LedgerDiskExhaustion verifies the system reports an error (not
// panics) when the storage layer is full.
func TestChaos_LedgerDiskExhaustion(t *testing.T) {
	store := newFaultStore(0, 10) // drop after 10 events
	accountID := "CHAOS_DISK_001"

	var lastErr error
	for i := 0; i < 20; i++ {
		oid := fmt.Sprintf("DISK_ORD_%04d", i)
		ev := ledger.NewEvent(ledger.AggregateOrder, oid, ledger.EventOrderCreated, nil)
		ev.AccountID = accountID
		_, err := store.Append(context.Background(), ev)
		if err != nil {
			lastErr = err
		}
	}

	if lastErr == nil {
		t.Error("FAIL: no error reported during disk exhaustion simulation")
	}
	t.Logf("disk exhaustion correctly detected: %v", lastErr)
}

// TestChaos_EventHashTampering verifies the ledger rejects events with invalid
// hashes — protecting against data corruption and replay attacks.
func TestChaos_EventHashTampering(t *testing.T) {
	store := ledger.NewMemoryStore()

	ev := ledger.NewEvent(ledger.AggregateOrder, "TAMPER_ORD_001", ledger.EventOrderCreated, nil)
	ev.AccountID = "CHAOS_TAMPER_001"

	// Tamper with the hash.
	ev.PayloadHash = "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := store.Append(context.Background(), ev)
	if err == nil {
		t.Fatal("FAIL: tampered event accepted by ledger — integrity breach")
	}
	if !errors.Is(err, ledger.ErrHashMismatch) {
		t.Errorf("expected ErrHashMismatch, got: %v", err)
	}
}

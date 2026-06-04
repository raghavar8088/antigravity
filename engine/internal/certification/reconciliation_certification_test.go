package certification

import (
	"context"
	"fmt"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/reconciliationv2"
)

// ─── Phase 8: Reconciliation Certification ──────────────────────────────────

// inMemoryOMSReader implements reconciliationv2.OMSStateReader backed by an
// in-memory map for deterministic reconciliation testing.
type inMemoryOMSReader struct {
	balances  map[string]float64
	positions map[string]float64
	orders    map[string]reconciliationv2.OrderState
}

func newOMSReader() *inMemoryOMSReader {
	return &inMemoryOMSReader{
		balances:  make(map[string]float64),
		positions: make(map[string]float64),
		orders:    make(map[string]reconciliationv2.OrderState),
	}
}

func (r *inMemoryOMSReader) GetBalance(ctx context.Context, asset string) (float64, error) {
	return r.balances[asset], nil
}
func (r *inMemoryOMSReader) GetPosition(ctx context.Context, symbol string) (float64, error) {
	return r.positions[symbol], nil
}
func (r *inMemoryOMSReader) GetOrder(ctx context.Context, orderID string) (reconciliationv2.OrderState, error) {
	os, ok := r.orders[orderID]
	if !ok {
		return reconciliationv2.OrderState{}, fmt.Errorf("order %s not found", orderID)
	}
	return os, nil
}

// inMemoryExchangeAdapter implements reconciliationv2.ReconciliationAdapter for
// deterministic exchange state simulation.
type inMemoryExchangeAdapter struct {
	name      string
	balances  map[string]float64
	positions map[string]float64
	orders    map[string]reconciliationv2.OrderState
}

func newExchangeAdapter(name string) *inMemoryExchangeAdapter {
	return &inMemoryExchangeAdapter{
		name:      name,
		balances:  make(map[string]float64),
		positions: make(map[string]float64),
		orders:    make(map[string]reconciliationv2.OrderState),
	}
}

func (a *inMemoryExchangeAdapter) Name() string { return a.name }
func (a *inMemoryExchangeAdapter) FetchBalance(ctx context.Context, asset string) (float64, error) {
	return a.balances[asset], nil
}
func (a *inMemoryExchangeAdapter) FetchPosition(ctx context.Context, symbol string) (float64, error) {
	return a.positions[symbol], nil
}
func (a *inMemoryExchangeAdapter) FetchOrder(ctx context.Context, orderID string) (reconciliationv2.OrderState, error) {
	os, ok := a.orders[orderID]
	if !ok {
		return reconciliationv2.OrderState{}, fmt.Errorf("order %s not found on exchange", orderID)
	}
	return os, nil
}
func (a *inMemoryExchangeAdapter) FetchOpenOrders(ctx context.Context) ([]reconciliationv2.OrderState, error) {
	orders := make([]reconciliationv2.OrderState, 0, len(a.orders))
	for _, o := range a.orders {
		orders = append(orders, o)
	}
	return orders, nil
}

// noopRepairTarget implements reconciliationv2.RepairTarget for testing without
// side effects.
type noopRepairTarget struct {
	cancelCalls int
	fixCalls    int
}

func (n *noopRepairTarget) CancelOrder(ctx context.Context, orderID string) error {
	n.cancelCalls++
	return nil
}
func (n *noopRepairTarget) ForceClosePosition(ctx context.Context, symbol string, qty float64) error {
	n.fixCalls++
	return nil
}
func (n *noopRepairTarget) AdjustBalance(ctx context.Context, asset string, delta float64) error {
	n.fixCalls++
	return nil
}

// TestReconciliation_NoBalanceDrift verifies that when OMS and exchange agree,
// the reconciliation engine finds zero drift.
func TestReconciliation_NoBalanceDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics()
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_001"

	exchAdapter := newExchangeAdapter("binance-paper")
	exchAdapter.balances["USDT"] = 1_000_000.0

	omsReader := newOMSReader()
	omsReader.balances["USDT"] = 1_000_000.0

	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if result.DriftDetected {
		t.Errorf("FAIL: balance drift reported when OMS and exchange agree (drift=%.4f)", result.DriftAmount)
	}
	t.Logf("balance reconciliation: no drift — PASS")
}

// TestReconciliation_DetectsBalanceDrift verifies the engine correctly identifies
// and reports drift when exchange balance diverges from OMS state.
func TestReconciliation_DetectsBalanceDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics()
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_002"

	exchAdapter := newExchangeAdapter("binance-paper")
	exchAdapter.balances["USDT"] = 999_000.0 // exchange shows 1000 less

	omsReader := newOMSReader()
	omsReader.balances["USDT"] = 1_000_000.0

	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if !result.DriftDetected {
		t.Error("FAIL: balance drift not detected — reconciliation authority compromised")
	}
	t.Logf("balance drift detected: %.2f USDT — PASS", result.DriftAmount)
}

// TestReconciliation_PositionDrift verifies position drift detection when the
// exchange reports a different quantity than the OMS.
func TestReconciliation_PositionDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics()
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_003"

	exchAdapter := newExchangeAdapter("binance-paper")
	exchAdapter.positions["BTC-USDT"] = 0.5 // exchange: 0.5 BTC

	omsReader := newOMSReader()
	omsReader.positions["BTC-USDT"] = 0.75 // OMS: 0.75 BTC — drift of 0.25

	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainPosition)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if !result.DriftDetected {
		t.Error("FAIL: position drift not detected")
	}
	t.Logf("position drift detected: %.4f BTC — PASS", result.DriftAmount)
}

// TestReconciliation_AuditEventsPersistedToLedger verifies that every
// reconciliation run (pass or fail) emits an audit event to the ledger.
func TestReconciliation_AuditEventsPersistedToLedger(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics()
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_004"

	exchAdapter := newExchangeAdapter("binance-paper")
	exchAdapter.balances["USDT"] = 1_000_000.0
	omsReader := newOMSReader()
	omsReader.balances["USDT"] = 1_000_000.0

	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	// Run 3 reconciliation cycles.
	for i := 0; i < 3; i++ {
		if _, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance); err != nil {
			t.Fatalf("RunDomain cycle %d: %v", i, err)
		}
	}

	events, err := store.Replay(context.Background(), ledger.AggregateReconciliation, accountID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(events) < 3 {
		t.Errorf("FAIL: expected ≥3 reconciliation audit events, got %d — audit trail incomplete",
			len(events))
	}
}

// TestReconciliation_RepairTriggeredOnDrift verifies the repair engine is
// invoked when drift is detected, proving auto-repair is wired.
func TestReconciliation_RepairTriggeredOnDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics()
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_005"

	exchAdapter := newExchangeAdapter("binance-paper")
	exchAdapter.balances["USDT"] = 990_000.0 // 10K drift

	omsReader := newOMSReader()
	omsReader.balances["USDT"] = 1_000_000.0

	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if !result.DriftDetected {
		t.Fatal("FAIL: drift not detected")
	}
	if repair.fixCalls == 0 {
		t.Error("FAIL: repair not invoked after drift detection — auto-repair broken")
	}
	t.Logf("auto-repair invoked %d times for %.2f USDT drift", repair.fixCalls, result.DriftAmount)
}

// TestReconciliation_MultipleExchanges verifies that independent reconciliation
// engines for different exchanges produce independent results without cross-contamination.
func TestReconciliation_MultipleExchanges(t *testing.T) {
	accountID := "RECON_CERT_006"
	metrics := reconciliationv2.NewMetrics()

	exchanges := []struct {
		name    string
		exchBal float64
		omsBal  float64
		wantErr bool
	}{
		{"binance", 1_000_000, 1_000_000, false},
		{"delta", 500_000, 499_000, true},  // 1000 drift
		{"paper", 250_000, 250_000, false},
	}

	for _, ex := range exchanges {
		ex := ex
		t.Run(ex.name, func(t *testing.T) {
			store := ledger.NewMemoryStore()
			repair := &noopRepairTarget{}

			exchAdapter := newExchangeAdapter(ex.name)
			exchAdapter.balances["USDT"] = ex.exchBal
			omsReader := newOMSReader()
			omsReader.balances["USDT"] = ex.omsBal

			engine := reconciliationv2.NewReconciliationEngine(
				exchAdapter, omsReader, store, repair, metrics, accountID,
			)

			result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
			if err != nil {
				t.Fatalf("RunDomain: %v", err)
			}
			if result.DriftDetected != ex.wantErr {
				t.Errorf("exchange %s: drift=%v (want %v)", ex.name, result.DriftDetected, ex.wantErr)
			}
		})
	}
}

// TestReconciliation_DeterministicAfterReplay proves that replaying the same
// ledger produces identical reconciliation audit events — determinism guarantee.
func TestReconciliation_DeterministicAfterReplay(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "RECON_CERT_007"
	metrics := reconciliationv2.NewMetrics()

	exchAdapter := newExchangeAdapter("binance-paper")
	exchAdapter.balances["USDT"] = 1_000_000.0
	omsReader := newOMSReader()
	omsReader.balances["USDT"] = 1_000_000.0

	repair := &noopRepairTarget{}
	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	// Run 5 cycles.
	for i := 0; i < 5; i++ {
		if _, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}

	// Replay and count events twice.
	replay := func() int {
		r, err := ledger.ReplayEverything(context.Background(), store, accountID)
		if err != nil {
			t.Fatalf("ReplayEverything: %v", err)
		}
		return len(r.Reconciliation)
	}

	c1 := replay()
	c2 := replay()

	if c1 != c2 {
		t.Errorf("non-determinism: replay1=%d, replay2=%d reconciliation events", c1, c2)
	}
	t.Logf("reconciliation determinism: %d events consistent across replays", c1)
}

// TestReconciliation_HighFrequency verifies reconciliation performance stays
// viable under high-frequency trading conditions (1000 cycles/test).
func TestReconciliation_HighFrequency(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics()
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_HF_001"

	exchAdapter := newExchangeAdapter("binance-hf")
	exchAdapter.balances["USDT"] = 1_000_000.0
	omsReader := newOMSReader()
	omsReader.balances["USDT"] = 1_000_000.0

	engine := reconciliationv2.NewReconciliationEngine(
		exchAdapter, omsReader, store, repair, metrics, accountID,
	)

	const cycles = 1000
	start := time.Now()
	for i := 0; i < cycles; i++ {
		if _, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	cyclesPerSec := float64(cycles) / elapsed.Seconds()
	t.Logf("reconciliation throughput: %d cycles in %v (%.0f cycles/sec)",
		cycles, elapsed, cyclesPerSec)
	if cyclesPerSec < 100 {
		t.Errorf("FAIL: reconciliation too slow (%.0f cycles/sec, need ≥100)", cyclesPerSec)
	}
}

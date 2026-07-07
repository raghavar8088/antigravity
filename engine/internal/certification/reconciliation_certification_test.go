package certification

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/reconciliationv2"
)

// ─── Phase 8: Reconciliation Certification ──────────────────────────────────

// inMemoryOMSReader implements reconciliationv2.OMSStateReader.
type inMemoryOMSReader struct {
	equityUSD float64
	positions []reconciliationv2.OMSPosition
}

func (r *inMemoryOMSReader) GetOMSSnapshot(_ context.Context, accountID string) (reconciliationv2.OMSSnapshot, error) {
	return reconciliationv2.OMSSnapshot{
		AccountID: accountID,
		Balance: reconciliationv2.OMSBalanceSnapshot{
			EquityUSD:    r.equityUSD,
			AvailableUSD: r.equityUSD,
		},
		Positions: r.positions,
	}, nil
}

// inMemoryExchangeAdapter implements reconciliationv2.ReconciliationAdapter.
type inMemoryExchangeAdapter struct {
	name      string
	equityUSD float64
	positions []reconciliationv2.ExchangePosition
}

func (a *inMemoryExchangeAdapter) Name() string { return a.name }

func (a *inMemoryExchangeAdapter) GetAccountState(ctx context.Context) (reconciliationv2.AccountState, error) {
	balances, _ := a.GetBalances(ctx)
	positions, _ := a.GetPositions(ctx)
	return reconciliationv2.AccountState{
		Exchange:    a.name,
		Balances:    balances,
		Positions:   positions,
		RetrievedAt: time.Now().UTC(),
	}, nil
}

func (a *inMemoryExchangeAdapter) GetBalances(_ context.Context) ([]reconciliationv2.AssetBalance, error) {
	return []reconciliationv2.AssetBalance{
		{Asset: "USDT", WalletBalance: a.equityUSD, Available: a.equityUSD, EquityUSD: a.equityUSD},
	}, nil
}

func (a *inMemoryExchangeAdapter) GetPositions(_ context.Context) ([]reconciliationv2.ExchangePosition, error) {
	return a.positions, nil
}

func (a *inMemoryExchangeAdapter) GetOrders(_ context.Context, _ string, _ time.Time) ([]reconciliationv2.ExchangeOrder, error) {
	return nil, nil
}

func (a *inMemoryExchangeAdapter) GetOpenOrders(_ context.Context, _ string) ([]reconciliationv2.ExchangeOrder, error) {
	return nil, nil
}

func (a *inMemoryExchangeAdapter) GetFills(_ context.Context, _ string, _ time.Time) ([]reconciliationv2.ExchangeFill, error) {
	return nil, nil
}

func (a *inMemoryExchangeAdapter) GetTrades(_ context.Context, _ string, _ time.Time) ([]reconciliationv2.ExchangeTrade, error) {
	return nil, nil
}

func (a *inMemoryExchangeAdapter) GetFunding(_ context.Context, _ string, _ time.Time) ([]reconciliationv2.FundingPayment, error) {
	return nil, nil
}

func (a *inMemoryExchangeAdapter) HealthCheck(_ context.Context) error {
	return nil
}

// noopRepairTarget implements reconciliationv2.RepairTarget.
type noopRepairTarget struct {
	rebuildCalls int
}

func (n *noopRepairTarget) RebuildProjections(_ context.Context, _ string) error {
	n.rebuildCalls++
	return nil
}

func (n *noopRepairTarget) RebuildAggregate(_ context.Context, _, _ string) error {
	n.rebuildCalls++
	return nil
}

func newEngine(
	exchEquity float64,
	omsEquity float64,
	exchPositions []reconciliationv2.ExchangePosition,
	omsPositions []reconciliationv2.OMSPosition,
	accountID string,
) (*reconciliationv2.ReconciliationEngine, *noopRepairTarget) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics("test")
	repair := &noopRepairTarget{}
	adapter := &inMemoryExchangeAdapter{name: "binance-paper", equityUSD: exchEquity, positions: exchPositions}
	omsReader := &inMemoryOMSReader{equityUSD: omsEquity, positions: omsPositions}
	return reconciliationv2.NewReconciliationEngine(adapter, omsReader, store, repair, metrics, accountID), repair
}

// TestReconciliation_NoBalanceDrift verifies zero drift when OMS and exchange agree.
func TestReconciliation_NoBalanceDrift(t *testing.T) {
	engine, _ := newEngine(1_000_000, 1_000_000, nil, nil, "RECON_CERT_001")
	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if (result.MismatchCount > 0) {
		t.Errorf("FAIL: balance drift reported when OMS and exchange agree (drift=%.4f)", result.DriftScore)
	}
	t.Logf("balance reconciliation: no drift — PASS")
}

// TestReconciliation_DetectsBalanceDrift verifies drift is detected when exchange diverges.
func TestReconciliation_DetectsBalanceDrift(t *testing.T) {
	engine, _ := newEngine(999_000, 1_000_000, nil, nil, "RECON_CERT_002")
	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if !(result.MismatchCount > 0) {
		t.Error("FAIL: balance drift not detected — reconciliation authority compromised")
	}
	t.Logf("balance drift detected: %.2f USDT — PASS", result.DriftScore)
}

// TestReconciliation_PositionDrift verifies position drift detection.
func TestReconciliation_PositionDrift(t *testing.T) {
	exchPositions := []reconciliationv2.ExchangePosition{
		{Symbol: "BTC-USDT", Side: "LONG", Quantity: 0.5},
	}
	omsPositions := []reconciliationv2.OMSPosition{
		{Symbol: "BTC-USDT", Side: "LONG", Quantity: 0.75, State: "OPEN"},
	}
	engine, _ := newEngine(1_000_000, 1_000_000, exchPositions, omsPositions, "RECON_CERT_003")
	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainPosition)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if !(result.MismatchCount > 0) {
		t.Error("FAIL: position drift not detected")
	}
	t.Logf("position drift detected: %.4f BTC — PASS", result.DriftScore)
}

// TestReconciliation_AuditEventsPersistedToLedger verifies every run with a
// detected mismatch persists an audit event to the ledger. Cycles with zero
// drift correctly emit nothing to the ledger (see the "not persisted" comment
// on RunDomain — an unconditional per-cycle ledger write was the Jun 2026 OOM
// incident), so this test must actually produce drift across all 3 cycles to
// exercise the real invariant.
func TestReconciliation_AuditEventsPersistedToLedger(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics("test")
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_004"
	adapter := &inMemoryExchangeAdapter{name: "binance-paper", equityUSD: 990_000}
	omsReader := &inMemoryOMSReader{equityUSD: 1_000_000}
	engine := reconciliationv2.NewReconciliationEngine(adapter, omsReader, store, repair, metrics, accountID)

	for i := 0; i < 3; i++ {
		if _, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance); err != nil {
			t.Fatalf("RunDomain cycle %d: %v", i, err)
		}
	}

	events, err := store.ReplayAccount(context.Background(), accountID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(events) < 3 {
		t.Errorf("FAIL: expected ≥3 reconciliation audit events, got %d — audit trail incomplete", len(events))
	}
}

// TestReconciliation_RepairTriggeredOnDrift verifies balance drift is escalated
// for manual intervention rather than silently auto-rebuilt. Auto-rebuilding a
// projection can't fix a real exchange/ledger balance disagreement — it would
// just paper over a genuine discrepancy — so RepairEngine correctly routes
// balance-domain mismatches to RepairTypeManualIntervention (see the
// "MANUAL INTERVENTION REQUIRED" log line in engine.go) instead of calling
// RepairTarget.Rebuild*. This asserts that escalation, not the rebuild-call
// counter the original version of this test checked.
func TestReconciliation_RepairTriggeredOnDrift(t *testing.T) {
	engine, _ := newEngine(990_000, 1_000_000, nil, nil, "RECON_CERT_005")
	result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}
	if !(result.MismatchCount > 0) {
		t.Fatal("FAIL: drift not detected")
	}
	if result.EscalateCount == 0 {
		t.Error("FAIL: drift detected but not escalated for manual intervention")
	}
	t.Logf("escalated %d mismatches for manual intervention on %.2f USDT drift", result.EscalateCount, result.DriftScore)
}

// TestReconciliation_MultipleExchanges verifies independent engines produce independent results.
func TestReconciliation_MultipleExchanges(t *testing.T) {
	accountID := "RECON_CERT_006"
	metrics := reconciliationv2.NewMetrics("test")

	exchanges := []struct {
		name      string
		exchBal   float64
		omsBal    float64
		wantDrift bool
	}{
		{"binance", 1_000_000, 1_000_000, false},
		{"delta", 499_000, 500_000, true},
		{"paper", 250_000, 250_000, false},
	}

	for _, ex := range exchanges {
		ex := ex
		t.Run(ex.name, func(t *testing.T) {
			store := ledger.NewMemoryStore()
			repair := &noopRepairTarget{}
			adapter := &inMemoryExchangeAdapter{name: ex.name, equityUSD: ex.exchBal}
			omsReader := &inMemoryOMSReader{equityUSD: ex.omsBal}
			engine := reconciliationv2.NewReconciliationEngine(adapter, omsReader, store, repair, metrics, accountID)
			result, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance)
			if err != nil {
				t.Fatalf("RunDomain: %v", err)
			}
			if (result.MismatchCount > 0) != ex.wantDrift {
				t.Errorf("exchange %s: drift=%v (want %v)", ex.name, (result.MismatchCount > 0), ex.wantDrift)
			}
		})
	}
}

// TestReconciliation_DeterministicAfterReplay proves replay produces identical events.
func TestReconciliation_DeterministicAfterReplay(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "RECON_CERT_007"
	metrics := reconciliationv2.NewMetrics("test")
	adapter := &inMemoryExchangeAdapter{name: "binance-paper", equityUSD: 1_000_000}
	omsReader := &inMemoryOMSReader{equityUSD: 1_000_000}
	repair := &noopRepairTarget{}
	engine := reconciliationv2.NewReconciliationEngine(adapter, omsReader, store, repair, metrics, accountID)

	for i := 0; i < 5; i++ {
		if _, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}

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

// TestReconciliation_HighFrequency verifies reconciliation stays viable at 1000 cycles.
func TestReconciliation_HighFrequency(t *testing.T) {
	store := ledger.NewMemoryStore()
	metrics := reconciliationv2.NewMetrics("test")
	repair := &noopRepairTarget{}
	accountID := "RECON_CERT_HF_001"
	adapter := &inMemoryExchangeAdapter{name: "binance-hf", equityUSD: 1_000_000}
	omsReader := &inMemoryOMSReader{equityUSD: 1_000_000}
	engine := reconciliationv2.NewReconciliationEngine(adapter, omsReader, store, repair, metrics, accountID)

	const cycles = 1000
	start := time.Now()
	for i := 0; i < cycles; i++ {
		if _, err := engine.RunDomain(context.Background(), reconciliationv2.DomainBalance); err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	cyclesPerSec := float64(cycles) / elapsed.Seconds()
	t.Logf("reconciliation throughput: %d cycles in %v (%.0f cycles/sec)", cycles, elapsed, cyclesPerSec)
	if cyclesPerSec < 100 {
		t.Errorf("FAIL: reconciliation too slow (%.0f cycles/sec, need ≥100)", cyclesPerSec)
	}
}

package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// newTestAuthority wires a complete authority with stub components.
func newTestAuthority(t *testing.T, adapter ReconciliationAdapter, snap OMSSnapshot) *ExchangeReconciliationAuthority {
	t.Helper()
	store := ledger.NewMemoryStore()
	omsReader := &stubOMSStateReader{snapshot: snap}
	target := &stubRepairTarget{}
	cfg := ScheduleConfig{
		OrdersInterval:    1 * time.Hour, // long intervals so no auto-ticks in tests
		PositionsInterval: 1 * time.Hour,
		BalancesInterval:  1 * time.Hour,
		ExposureInterval:  1 * time.Hour,
		FullAuditInterval: 1 * time.Hour,
	}
	return NewExchangeReconciliationAuthority(adapter, omsReader, store, target, "test-account", &cfg)
}

// ─── Authority construction ───────────────────────────────────────────────────

func TestAuthority_New(t *testing.T) {
	adapter := newStubAdapter("binance")
	auth := newTestAuthority(t, adapter, OMSSnapshot{})
	if auth == nil {
		t.Fatal("authority should not be nil")
	}
	status := auth.Status()
	if status.Exchange != "binance" {
		t.Errorf("expected exchange=binance, got %s", status.Exchange)
	}
	if status.Running {
		t.Error("authority should not be running before Start()")
	}
}

// ─── Authority.RunNow — clean state ──────────────────────────────────────────

func TestAuthority_RunNow_CleanState(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.balances = []AssetBalance{{Asset: "USDT", EquityUSD: 100000, Available: 80000, MarginUsed: 20000}}
	adapter.positions = []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.0, EntryPrice: 50000, NotionalUSD: 50000}}

	snap := OMSSnapshot{
		Balance: OMSBalanceSnapshot{
			EquityUSD:     100000,
			AvailableUSD:  80000,
			MarginUsedUSD: 20000,
		},
		Positions: []OMSPosition{
			{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.0, EntryPrice: 50000, State: "OPEN"},
		},
		GrossNotional: 50000,
		NetNotional:   50000,
	}

	auth := newTestAuthority(t, adapter, snap)
	entry, err := auth.RunNow(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.MismatchCount != 0 {
		t.Errorf("expected 0 mismatches for clean state, got %d: %+v", entry.MismatchCount, entry.Mismatches)
	}
	if entry.DriftScore != 0 {
		t.Errorf("expected drift=0, got %.2f", entry.DriftScore)
	}
}

// ─── Authority.RunNow — ghost position ───────────────────────────────────────

func TestAuthority_RunNow_GhostPosition(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.positions = []ExchangePosition{
		{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000},
	}
	adapter.balances = []AssetBalance{{Asset: "USDT", EquityUSD: 100000, Available: 80000}}

	snap := OMSSnapshot{
		Balance:   OMSBalanceSnapshot{EquityUSD: 100000, AvailableUSD: 80000},
		Positions: []OMSPosition{}, // OMS has no positions
	}

	auth := newTestAuthority(t, adapter, snap)
	entry, err := auth.RunNow(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundGhost := false
	for _, m := range entry.Mismatches {
		if m.Type == "ghost_position" && m.Severity == SeverityCritical {
			foundGhost = true
		}
	}
	if !foundGhost {
		t.Errorf("expected CRITICAL ghost_position mismatch: %+v", entry.Mismatches)
	}
	if entry.DriftScore == 0 {
		t.Error("expected non-zero drift score with ghost position")
	}
}

// ─── Authority.RunDomain — balance domain ────────────────────────────────────

func TestAuthority_RunDomain_Balance_Drift(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.balances = []AssetBalance{{Asset: "USDT", EquityUSD: 105000}} // 5% drift

	snap := OMSSnapshot{Balance: OMSBalanceSnapshot{EquityUSD: 100000}}
	auth := newTestAuthority(t, adapter, snap)

	entry, err := auth.RunDomain(context.Background(), DomainBalance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.MismatchCount == 0 {
		t.Error("expected equity drift mismatch")
	}
	if entry.DriftScore == 0 {
		t.Error("expected non-zero drift score")
	}
}

// ─── Authority.RunDomain — order domain ──────────────────────────────────────

func TestAuthority_RunDomain_Order_FilledButOMSUnknown(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.orders = []ExchangeOrder{
		{ClientOrderID: "order-abc", Status: "FILLED", Quantity: 1.0, FilledQuantity: 1.0},
	}

	snap := OMSSnapshot{
		OpenOrders: []OMSOrder{
			{ClientOrderID: "order-abc", State: "SUBMITTED", Quantity: 1.0},
		},
	}
	auth := newTestAuthority(t, adapter, snap)

	entry, err := auth.RunDomain(context.Background(), DomainOrder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, m := range entry.Mismatches {
		if m.Type == "wrong_status" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected wrong_status mismatch: %+v", entry.Mismatches)
	}
}

// ─── Authority status after cycles ───────────────────────────────────────────

func TestAuthority_Status_UpdatesAfterCycles(t *testing.T) {
	adapter := newStubAdapter("binance")
	snap := OMSSnapshot{}
	auth := newTestAuthority(t, adapter, snap)

	// Run 3 cycles.
	for i := 0; i < 3; i++ {
		if _, err := auth.RunNow(context.Background()); err != nil {
			t.Fatalf("cycle %d error: %v", i, err)
		}
	}

	status := auth.Status()
	if status.TotalCycles != 3 {
		t.Errorf("expected 3 total cycles, got %d", status.TotalCycles)
	}
	if len(status.RecentCycles) == 0 {
		t.Error("expected recent cycles in status")
	}
}

// ─── Authority crash recovery validation ─────────────────────────────────────

func TestAuthority_ValidateCrashRecovery_Clean(t *testing.T) {
	adapter := newStubAdapter("binance")
	snap := OMSSnapshot{} // no positions, no orders
	auth := newTestAuthority(t, adapter, snap)

	mismatches, err := auth.ValidateCrashRecovery(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("expected 0 mismatches for clean state, got %d", len(mismatches))
	}
}

func TestAuthority_ValidateCrashRecovery_WithDrift(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.positions = []ExchangePosition{
		{Symbol: "ETHUSDT", Side: "SHORT", Quantity: 5.0}, // ghost
	}
	snap := OMSSnapshot{}
	auth := newTestAuthority(t, adapter, snap)

	mismatches, err := auth.ValidateCrashRecovery(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mismatches) == 0 {
		t.Error("expected mismatches after crash recovery with ghost position")
	}
}

// ─── DefaultScheduleConfig ────────────────────────────────────────────────────

func TestDefaultScheduleConfig(t *testing.T) {
	cfg := DefaultScheduleConfig()
	if cfg.OrdersInterval != 2*time.Second {
		t.Errorf("expected 2s orders interval, got %v", cfg.OrdersInterval)
	}
	if cfg.PositionsInterval != 5*time.Second {
		t.Errorf("expected 5s positions interval, got %v", cfg.PositionsInterval)
	}
	if cfg.BalancesInterval != 10*time.Second {
		t.Errorf("expected 10s balances interval, got %v", cfg.BalancesInterval)
	}
	if cfg.FullAuditInterval != 60*time.Second {
		t.Errorf("expected 60s full audit interval, got %v", cfg.FullAuditInterval)
	}
}

// ─── newRunID and newMismatchID uniqueness ─────────────────────────────────────

func TestIDGeneration_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := newRunID()
		if seen[id] {
			t.Fatalf("duplicate run ID: %s", id)
		}
		seen[id] = true

		mid := newMismatchID()
		if seen[mid] {
			t.Fatalf("duplicate mismatch ID: %s", mid)
		}
		seen[mid] = true
	}
}

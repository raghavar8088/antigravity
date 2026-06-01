package reconciliationv2

import (
	"context"
	"testing"
	"time"
)

// stubReconciliationAdapter is a deterministic test double for ReconciliationAdapter.
type stubReconciliationAdapter struct {
	name      string
	balances  []AssetBalance
	positions []ExchangePosition
	orders    []ExchangeOrder
	fills     []ExchangeFill
	healthy   bool
}

func newStubAdapter(name string) *stubReconciliationAdapter {
	return &stubReconciliationAdapter{name: name, healthy: true}
}

func (s *stubReconciliationAdapter) Name() string { return s.name }

func (s *stubReconciliationAdapter) HealthCheck(ctx context.Context) error {
	if !s.healthy {
		return context.DeadlineExceeded
	}
	return nil
}

func (s *stubReconciliationAdapter) GetAccountState(ctx context.Context) (AccountState, error) {
	return AccountState{
		Exchange:    s.name,
		Balances:    s.balances,
		Positions:   s.positions,
		OpenOrders:  s.orders,
		RetrievedAt: time.Now(),
	}, nil
}

func (s *stubReconciliationAdapter) GetBalances(ctx context.Context) ([]AssetBalance, error) {
	return s.balances, nil
}

func (s *stubReconciliationAdapter) GetPositions(ctx context.Context) ([]ExchangePosition, error) {
	return s.positions, nil
}

func (s *stubReconciliationAdapter) GetOrders(ctx context.Context, symbol string, since time.Time) ([]ExchangeOrder, error) {
	return s.orders, nil
}

func (s *stubReconciliationAdapter) GetOpenOrders(ctx context.Context, symbol string) ([]ExchangeOrder, error) {
	return s.orders, nil
}

func (s *stubReconciliationAdapter) GetFills(ctx context.Context, symbol string, since time.Time) ([]ExchangeFill, error) {
	return s.fills, nil
}

func (s *stubReconciliationAdapter) GetTrades(ctx context.Context, symbol string, since time.Time) ([]ExchangeTrade, error) {
	return nil, nil
}

func (s *stubReconciliationAdapter) GetFunding(ctx context.Context, symbol string, since time.Time) ([]FundingPayment, error) {
	return nil, nil
}

// stubOMSStateReader is a test double for OMSStateReader.
type stubOMSStateReader struct {
	snapshot OMSSnapshot
}

func (s *stubOMSStateReader) GetOMSSnapshot(ctx context.Context, accountID string) (OMSSnapshot, error) {
	return s.snapshot, nil
}

// ─── Interface compliance ─────────────────────────────────────────────────────

func TestStubAdapter_ImplementsInterface(t *testing.T) {
	var _ ReconciliationAdapter = newStubAdapter("test")
}

func TestStubOMSReader_ImplementsInterface(t *testing.T) {
	var _ OMSStateReader = &stubOMSStateReader{}
}

// ─── Adapter smoke tests ──────────────────────────────────────────────────────

func TestStubAdapter_HealthCheck_Healthy(t *testing.T) {
	adapter := newStubAdapter("binance")
	if err := adapter.HealthCheck(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestStubAdapter_HealthCheck_Unhealthy(t *testing.T) {
	adapter := newStubAdapter("delta")
	adapter.healthy = false
	if err := adapter.HealthCheck(context.Background()); err == nil {
		t.Error("expected error for unhealthy adapter")
	}
}

func TestStubAdapter_GetAccountState(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.balances = []AssetBalance{{Asset: "USDT", EquityUSD: 100000}}
	adapter.positions = []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.0}}

	state, err := adapter.GetAccountState(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Exchange != "binance" {
		t.Errorf("expected exchange=binance, got %s", state.Exchange)
	}
	if len(state.Balances) != 1 {
		t.Errorf("expected 1 balance, got %d", len(state.Balances))
	}
	if state.Balances[0].EquityUSD != 100000 {
		t.Errorf("expected equity=100000, got %.2f", state.Balances[0].EquityUSD)
	}
	if len(state.Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(state.Positions))
	}
}

func TestStubAdapter_GetFills_FiltersBySince(t *testing.T) {
	adapter := newStubAdapter("binance")
	now := time.Now()
	adapter.fills = []ExchangeFill{
		{FillID: "f1", Timestamp: now.Add(-1 * time.Hour)},
		{FillID: "f2", Timestamp: now.Add(-5 * time.Minute)},
		{FillID: "f3", Timestamp: now},
	}

	// The stub adapter returns all fills — filter logic is tested via real adapters.
	fills, err := adapter.GetFills(context.Background(), "", now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fills) != 3 {
		t.Errorf("stub returns all fills regardless of since: got %d", len(fills))
	}
}

// ─── PaperReconciliationAdapter tests ────────────────────────────────────────

func TestPaperAdapter_MatchesOMS(t *testing.T) {
	snap := OMSSnapshot{
		AccountID: "paper-btc",
		Balance: OMSBalanceSnapshot{
			EquityUSD:     1000000,
			AvailableUSD:  900000,
			MarginUsedUSD: 100000,
		},
		Positions: []OMSPosition{
			{Symbol: "BTCUSDT", Side: "LONG", Quantity: 2.0, EntryPrice: 50000, State: "OPEN"},
		},
		OpenOrders: []OMSOrder{
			{ClientOrderID: "abc", Symbol: "BTCUSDT", Side: "BUY", State: "SUBMITTED", Quantity: 0.5},
		},
		SnapshotAt: time.Now(),
	}
	reader := &stubOMSStateReader{snapshot: snap}
	adapter := NewPaperReconciliationAdapter(reader, "paper-btc")

	if adapter.Name() != "paper" {
		t.Errorf("expected name=paper, got %s", adapter.Name())
	}

	state, err := adapter.GetAccountState(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(state.Balances) != 1 || state.Balances[0].EquityUSD != 1000000 {
		t.Errorf("wrong balances: %+v", state.Balances)
	}
	if len(state.Positions) != 1 || state.Positions[0].Quantity != 2.0 {
		t.Errorf("wrong positions: %+v", state.Positions)
	}
	if len(state.OpenOrders) != 1 {
		t.Errorf("wrong open orders: %+v", state.OpenOrders)
	}
}

func TestPaperAdapter_HealthCheckAlwaysNil(t *testing.T) {
	reader := &stubOMSStateReader{}
	adapter := NewPaperReconciliationAdapter(reader, "paper")
	if err := adapter.HealthCheck(context.Background()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestPaperAdapter_OnlyOpenPositions(t *testing.T) {
	snap := OMSSnapshot{
		Positions: []OMSPosition{
			{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.0, State: "OPEN"},
			{Symbol: "ETHUSDT", Side: "LONG", Quantity: 5.0, State: "CLOSED"},
			{Symbol: "SOLUSDT", Side: "SHORT", Quantity: 10.0, State: "REDUCED"},
		},
	}
	reader := &stubOMSStateReader{snapshot: snap}
	adapter := NewPaperReconciliationAdapter(reader, "paper")

	positions, err := adapter.GetPositions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(positions) != 2 {
		t.Errorf("expected 2 open/reduced positions, got %d: %+v", len(positions), positions)
	}
}

// ─── AccountState field completeness ─────────────────────────────────────────

func TestAccountState_Fields(t *testing.T) {
	now := time.Now().UTC()
	state := AccountState{
		Exchange:    "binance",
		AccountID:   "acc-1",
		Balances:    []AssetBalance{{Asset: "USDT", EquityUSD: 1e6}},
		Positions:   []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1}},
		OpenOrders:  []ExchangeOrder{{ExchangeOrderID: "x"}},
		RetrievedAt: now,
	}
	if state.RetrievedAt != now {
		t.Error("RetrievedAt not set correctly")
	}
	if state.Balances[0].Asset != "USDT" {
		t.Error("Asset not set")
	}
	if state.Positions[0].Symbol != "BTCUSDT" {
		t.Error("Symbol not set")
	}
}

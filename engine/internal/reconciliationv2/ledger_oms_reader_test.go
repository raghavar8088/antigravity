package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
	"antigravity-engine/internal/positions"
)

func TestLedgerOMSStateReader_GetOMSSnapshot_Empty(t *testing.T) {
	store := ledger.NewMemoryStore()
	reader := NewLedgerOMSStateReader(store, "btc-paper-1", LedgerOMSReaderConfig{
		InitialBalanceUSD: 1_000_000,
	})
	snap, err := reader.GetOMSSnapshot(context.Background(), "btc-paper-1")
	if err != nil {
		t.Fatalf("GetOMSSnapshot: %v", err)
	}
	if snap.AccountID != "btc-paper-1" {
		t.Fatalf("accountID=%s", snap.AccountID)
	}
	if len(snap.Positions) != 0 {
		t.Fatalf("expected no positions, got %d", len(snap.Positions))
	}
	if snap.Balance.EquityUSD != 1_000_000 {
		t.Fatalf("empty ledger equity=%f want 1_000_000", snap.Balance.EquityUSD)
	}
}

func TestBuildLedgerBalanceSnapshot_UsesInitialBalanceNotPnLAlone(t *testing.T) {
	pnl := omsv3.PnLProjection{TotalPnLUSD: 250.50}
	bal := buildLedgerBalanceSnapshot(LedgerOMSReaderConfig{InitialBalanceUSD: 1_000_000}, pnl, nil)
	if bal.EquityUSD != 1_000_250.50 {
		t.Fatalf("equity=%f want 1000250.50", bal.EquityUSD)
	}
	if bal.RealizedPnL != 250.50 {
		t.Fatalf("realized=%f", bal.RealizedPnL)
	}
}

func TestPositionSideKey_NormalizesBuyLong(t *testing.T) {
	if positionSideKey("BTC-USD", "BUY") != positionSideKey("BTC-USD", "LONG") {
		t.Fatal("BUY and LONG should produce same key")
	}
	if positionSideKey("BTCUSDT", "SELL") != positionSideKey("BTC-USD", "SHORT") {
		t.Fatal("SELL/BTCUSDT should normalize to SHORT/BTC-USD key")
	}
}

func TestPositionManagerExchangeAdapter_GetPositions(t *testing.T) {
	posMgr := positions.NewManager()
	adapter := NewPositionManagerExchangeAdapter(posMgr, func() float64 { return 1_000_000 }, "btc-paper-1")
	positions, err := adapter.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected empty positions, got %d", len(positions))
	}
	if adapter.Name() != "engine-runtime" {
		t.Fatalf("name=%s", adapter.Name())
	}
}

func TestBalanceDriftDetector_NoDriftWithFixedProjection(t *testing.T) {
	det := BalanceDriftDetector{}
	runtimeEquity := 1_000_000.0
	balances := []AssetBalance{{
		Asset:     "USD",
		EquityUSD: runtimeEquity,
		Available: runtimeEquity,
	}}
	oms := buildLedgerBalanceSnapshot(LedgerOMSReaderConfig{InitialBalanceUSD: 1_000_000}, omsv3.PnLProjection{}, nil)
	mismatches := det.Detect(balances, oms, time.Now())
	for _, m := range mismatches {
		if m.Type == "equity_drift" && m.Severity == SeverityCritical {
			t.Fatalf("unexpected critical equity drift: %s", m.Message)
		}
	}
}

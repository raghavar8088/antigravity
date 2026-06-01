package reconciliationv2

import (
	"testing"
	"time"
)

func TestAbsDrift(t *testing.T) {
	cases := []struct {
		a, b    float64
		wantPct float64 // approximate — we check within 0.01
	}{
		{100, 100, 0},
		{100, 101, 0.99}, // max(100,101)=101 → 1/101*100 ≈ 0.99
		{100, 99, 1.0},   // max(100,99)=100 → 1/100*100 = 1.0
		{0, 0, 0},
		{100, 0, 100},
	}
	for _, c := range cases {
		got := absDrift(c.a, c.b)
		diff := got - c.wantPct
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			t.Errorf("absDrift(%.2f, %.2f) = %.4f, want ≈ %.4f", c.a, c.b, got, c.wantPct)
		}
	}
}

func TestClassifySeverity(t *testing.T) {
	cases := []struct {
		drift float64
		want  DriftSeverity
	}{
		{0.001, SeverityLow},
		{0.05, SeverityMedium},
		{0.5, SeverityHigh},
		{2.0, SeverityCritical},
	}
	for _, c := range cases {
		got := classifySeverity(c.drift)
		if got != c.want {
			t.Errorf("classifySeverity(%.3f) = %s, want %s", c.drift, got, c.want)
		}
	}
}

// ─── Balance Drift Detector ───────────────────────────────────────────────────

func TestBalanceDriftDetector_NoDrift(t *testing.T) {
	det := BalanceDriftDetector{}
	balances := []AssetBalance{{
		Asset:     "USDT",
		EquityUSD: 100000,
		Available: 80000,
		MarginUsed: 20000,
	}}
	oms := OMSBalanceSnapshot{
		EquityUSD:     100000,
		AvailableUSD:  80000,
		MarginUsedUSD: 20000,
	}
	mismatches := det.Detect(balances, oms, time.Now())
	if len(mismatches) != 0 {
		t.Errorf("expected 0 mismatches, got %d", len(mismatches))
	}
}

func TestBalanceDriftDetector_EquityDrift(t *testing.T) {
	det := BalanceDriftDetector{}
	balances := []AssetBalance{{
		Asset:     "USDT",
		EquityUSD: 100100,
		Available: 80000,
		MarginUsed: 20000,
	}}
	oms := OMSBalanceSnapshot{
		EquityUSD:     100000,
		AvailableUSD:  80000,
		MarginUsedUSD: 20000,
	}
	mismatches := det.Detect(balances, oms, time.Now())
	if len(mismatches) == 0 {
		t.Fatal("expected equity drift mismatch")
	}
	m := mismatches[0]
	if m.Type != "equity_drift" {
		t.Errorf("expected equity_drift, got %s", m.Type)
	}
	if m.Domain != DomainBalance {
		t.Errorf("expected domain balance, got %s", m.Domain)
	}
}

func TestBalanceDriftDetector_CriticalDrift(t *testing.T) {
	det := BalanceDriftDetector{}
	balances := []AssetBalance{{
		Asset:     "USDT",
		EquityUSD: 103000, // 3% drift
		Available: 80000,
	}}
	oms := OMSBalanceSnapshot{EquityUSD: 100000, AvailableUSD: 80000}
	mismatches := det.Detect(balances, oms, time.Now())
	if len(mismatches) == 0 {
		t.Fatal("expected mismatch")
	}
	if mismatches[0].Severity != SeverityCritical {
		t.Errorf("expected CRITICAL, got %s", mismatches[0].Severity)
	}
}

// ─── Position Drift Detector ──────────────────────────────────────────────────

func TestPositionDriftDetector_NoDrift(t *testing.T) {
	det := PositionDriftDetector{}
	exch := []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000}}
	oms := []OMSPosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000, State: "OPEN"}}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 0 {
		t.Errorf("expected 0 mismatches, got %d: %+v", len(mismatches), mismatches)
	}
}

func TestPositionDriftDetector_GhostPosition(t *testing.T) {
	det := PositionDriftDetector{}
	exch := []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000}}
	oms := []OMSPosition{} // OMS has nothing
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 ghost_position mismatch, got %d", len(mismatches))
	}
	if mismatches[0].Type != "ghost_position" {
		t.Errorf("expected ghost_position, got %s", mismatches[0].Type)
	}
	if mismatches[0].Severity != SeverityCritical {
		t.Errorf("expected CRITICAL severity")
	}
	if mismatches[0].AutoRepairable {
		t.Error("ghost position should not be auto-repairable")
	}
}

func TestPositionDriftDetector_MissingPosition(t *testing.T) {
	det := PositionDriftDetector{}
	exch := []ExchangePosition{} // exchange has nothing
	oms := []OMSPosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000, State: "OPEN"}}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 missing_position, got %d", len(mismatches))
	}
	if mismatches[0].Type != "missing_position" {
		t.Errorf("expected missing_position, got %s", mismatches[0].Type)
	}
}

func TestPositionDriftDetector_QuantityDrift(t *testing.T) {
	det := PositionDriftDetector{}
	exch := []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.6, EntryPrice: 50000}}
	oms := []OMSPosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000, State: "OPEN"}}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) == 0 {
		t.Fatal("expected qty drift mismatch")
	}
	found := false
	for _, m := range mismatches {
		if m.Type == "wrong_quantity" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected wrong_quantity mismatch: %+v", mismatches)
	}
}

func TestPositionDriftDetector_DuplicateOMS(t *testing.T) {
	det := PositionDriftDetector{}
	exch := []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000}}
	oms := []OMSPosition{
		{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.5, EntryPrice: 50000, State: "OPEN"},
		{Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.5, EntryPrice: 50000, State: "OPEN"},
	}
	mismatches := det.Detect(exch, oms, time.Now())
	found := false
	for _, m := range mismatches {
		if m.Type == "duplicate_position" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate_position mismatch: %+v", mismatches)
	}
}

// ─── Order Drift Detector ─────────────────────────────────────────────────────

func TestOrderDriftDetector_NoDrift(t *testing.T) {
	det := OrderDriftDetector{}
	exch := []ExchangeOrder{{ExchangeOrderID: "123", ClientOrderID: "abc", Symbol: "BTCUSDT", Status: "NEW", Quantity: 1.0, FilledQuantity: 0}}
	oms := []OMSOrder{{ClientOrderID: "abc", ExchangeOrderID: "123", Symbol: "BTCUSDT", State: "SUBMITTED", Quantity: 1.0, FilledQuantity: 0}}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 0 {
		t.Errorf("expected 0 mismatches, got %d: %+v", len(mismatches), mismatches)
	}
}

func TestOrderDriftDetector_GhostOrder(t *testing.T) {
	det := OrderDriftDetector{}
	exch := []ExchangeOrder{{ExchangeOrderID: "999", ClientOrderID: "", Symbol: "BTCUSDT", Status: "NEW", Quantity: 1.0}}
	oms := []OMSOrder{}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 ghost_order, got %d", len(mismatches))
	}
	if mismatches[0].Type != "ghost_order" {
		t.Errorf("expected ghost_order, got %s", mismatches[0].Type)
	}
}

func TestOrderDriftDetector_FilledButOMSDoesntKnow(t *testing.T) {
	det := OrderDriftDetector{}
	exch := []ExchangeOrder{{ClientOrderID: "abc", Status: "FILLED", Quantity: 1.0, FilledQuantity: 1.0}}
	oms := []OMSOrder{{ClientOrderID: "abc", State: "SUBMITTED", Quantity: 1.0}}
	mismatches := det.Detect(exch, oms, time.Now())
	found := false
	for _, m := range mismatches {
		if m.Type == "wrong_status" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected wrong_status mismatch: %+v", mismatches)
	}
}

// ─── Fill Drift Detector ──────────────────────────────────────────────────────

func TestFillDriftDetector_NoDrift(t *testing.T) {
	det := FillDriftDetector{}
	ts := time.Now().UTC()
	exch := []ExchangeFill{{FillID: "f1", ExchangeOrderID: "o1", Price: 50000, Quantity: 1.0, FeeUSD: 25.0, Timestamp: ts}}
	oms := []OMSFill{{FillID: "f1", ExchangeOrderID: "o1", Price: 50000, Quantity: 1.0, FeeUSD: 25.0, Timestamp: ts}}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 0 {
		t.Errorf("expected 0 mismatches, got %d: %+v", len(mismatches), mismatches)
	}
}

func TestFillDriftDetector_MissingFill(t *testing.T) {
	det := FillDriftDetector{}
	exch := []ExchangeFill{{FillID: "f1", Price: 50000, Quantity: 1.0, FeeUSD: 25.0, Timestamp: time.Now()}}
	oms := []OMSFill{}
	mismatches := det.Detect(exch, oms, time.Now())
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 missing_fill, got %d", len(mismatches))
	}
	if mismatches[0].Type != "missing_fill" {
		t.Errorf("expected missing_fill, got %s", mismatches[0].Type)
	}
	if mismatches[0].Severity != SeverityCritical {
		t.Errorf("missing fill must be CRITICAL")
	}
	if mismatches[0].AutoRepairable {
		t.Error("missing fill must not be auto-repairable")
	}
}

func TestFillDriftDetector_WrongPrice(t *testing.T) {
	det := FillDriftDetector{}
	ts := time.Now().UTC()
	exch := []ExchangeFill{{FillID: "f1", Price: 50100, Quantity: 1.0, FeeUSD: 25.0, Timestamp: ts}}
	oms := []OMSFill{{FillID: "f1", Price: 50000, Quantity: 1.0, FeeUSD: 25.0, Timestamp: ts}}
	mismatches := det.Detect(exch, oms, time.Now())
	found := false
	for _, m := range mismatches {
		if m.Type == "wrong_fill_price" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected wrong_fill_price: %+v", mismatches)
	}
}

// ─── Exposure Drift Detector ──────────────────────────────────────────────────

func TestExposureDriftDetector_NoDrift(t *testing.T) {
	det := ExposureDriftDetector{}
	positions := []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.0, MarkPrice: 50000, NotionalUSD: 50000}}
	oms := OMSSnapshot{GrossNotional: 50000, NetNotional: 50000}
	mismatches := det.Detect(positions, oms, 0, time.Now())
	if len(mismatches) != 0 {
		t.Errorf("expected 0 mismatches, got %d: %+v", len(mismatches), mismatches)
	}
}

func TestExposureDriftDetector_GrossDrift(t *testing.T) {
	det := ExposureDriftDetector{}
	positions := []ExchangePosition{{Symbol: "BTCUSDT", Side: "LONG", Quantity: 1.0, MarkPrice: 50000, NotionalUSD: 55000}}
	oms := OMSSnapshot{GrossNotional: 50000, NetNotional: 50000}
	mismatches := det.Detect(positions, oms, 0, time.Now())
	found := false
	for _, m := range mismatches {
		if m.Type == "gross_notional_drift" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected gross_notional_drift: %+v", mismatches)
	}
}

// ─── DriftSeverityScore ───────────────────────────────────────────────────────

func TestDriftSeverityScore_Empty(t *testing.T) {
	if score := DriftSeverityScore(nil); score != 0 {
		t.Errorf("expected 0, got %.2f", score)
	}
}

func TestDriftSeverityScore_Cap(t *testing.T) {
	mismatches := make([]Mismatch, 10)
	for i := range mismatches {
		mismatches[i].Severity = SeverityCritical
	}
	score := DriftSeverityScore(mismatches)
	if score != 100 {
		t.Errorf("expected 100, got %.2f", score)
	}
}

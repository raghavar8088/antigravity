package delta

import "testing"

// Regression: a broker-rejected order must never be recorded as an open live
// trade. UpdateTradeAfterFill previously stamped Status=OPEN unconditionally,
// overwriting the FAILED status set by the rejection path. The phantom "open"
// trade made the engine count 2 open trades while Delta reported 1 position,
// which auto-disarmed the Live Engine with reconciliation_mismatch on every arm.
func TestUpdateTradeAfterFill_DoesNotResurrectFailedTrade(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{{
		ID:            "DLT-0001",
		PaperTradeID:  "paper-1",
		Status:        "FAILED",
		FailureReason: `Delta order rejected (HTTP 400): {"error":{"code":"invalid_contract"}}`,
	}}

	// The institutional path calls this after the handler returns, even on reject.
	b.UpdateTradeAfterFill("DLT-0001", PlaceOrderResult{}, 123, 1, 64800, b.trades[0].ExpiryTime)

	if got := b.trades[0].Status; got != "FAILED" {
		t.Fatalf("rejected trade must stay FAILED, got %q", got)
	}
	if len(b.OpenTrades()) != 0 {
		t.Fatalf("a rejected trade must not count as open, got %d open", len(b.OpenTrades()))
	}

	// And it must not be registered in the open mapping either.
	b.RegisterOpenMapping("paper-1", "DLT-0001")
	if _, mapped := b.openByPaperID["paper-1"]; mapped {
		t.Fatal("a rejected trade must not be registered as an open position")
	}
}

// No broker order id means nothing actually filled — treat as FAILED, not OPEN.
func TestUpdateTradeAfterFill_NoOrderIDIsNotOpen(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{{ID: "DLT-0002", Status: "OPEN"}}

	b.UpdateTradeAfterFill("DLT-0002", PlaceOrderResult{OrderID: ""}, 1, 1, 100, b.trades[0].ExpiryTime)

	if b.trades[0].Status != "FAILED" {
		t.Fatalf("missing broker order id must mark FAILED, got %q", b.trades[0].Status)
	}
}

// A genuine fill (real broker order id) still records as OPEN with its details.
func TestUpdateTradeAfterFill_RealFillRecordsOpen(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{{ID: "DLT-0003", PaperTradeID: "paper-3", Status: "PENDING"}}

	b.UpdateTradeAfterFill("DLT-0003", PlaceOrderResult{
		OrderID: "998877", Symbol: "C-BTC-66000-070826", Price: 2.33,
	}, 42, 1, 66000, b.trades[0].ExpiryTime)

	tr := b.trades[0]
	if tr.Status != "OPEN" || tr.DeltaOrderID != "998877" || tr.DeltaSymbol != "C-BTC-66000-070826" {
		t.Fatalf("a real fill must record OPEN with broker details, got %+v", tr)
	}
	if len(b.OpenTrades()) != 1 {
		t.Fatalf("expected 1 open trade, got %d", len(b.OpenTrades()))
	}
}

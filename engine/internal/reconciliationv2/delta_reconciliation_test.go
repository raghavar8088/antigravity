package reconciliationv2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Regression coverage for the Jun 2026 incident: Delta Exchange returns
// numeric fields (balance, size, prices) as JSON strings, and /v2/positions
// requires a product_id/underlying_asset_symbol filter that an account-wide
// sweep can't supply. Both bugs made every reconciliation cycle fail
// continuously, which fed an unbounded in-memory ledger leak and eventually
// got the engine OOM-killed roughly every 24-36 hours.

func TestNumString_AcceptsStringEncoding(t *testing.T) {
	var n numString
	if err := json.Unmarshal([]byte(`"1234.56"`), &n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1234.56 {
		t.Fatalf("got %v, want 1234.56", n)
	}
}

func TestNumString_AcceptsNumberEncoding(t *testing.T) {
	var n numString
	if err := json.Unmarshal([]byte(`1234.56`), &n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1234.56 {
		t.Fatalf("got %v, want 1234.56", n)
	}
}

func TestNumString_HandlesEmptyAndNull(t *testing.T) {
	for _, raw := range []string{`""`, `null`} {
		var n numString
		if err := json.Unmarshal([]byte(raw), &n); err != nil {
			t.Fatalf("raw=%s: unexpected error: %v", raw, err)
		}
		if n != 0 {
			t.Fatalf("raw=%s: got %v, want 0", raw, n)
		}
	}
}

func TestNumString_RejectsGarbage(t *testing.T) {
	var n numString
	if err := json.Unmarshal([]byte(`"not-a-number"`), &n); err == nil {
		t.Fatal("expected error for non-numeric string, got nil")
	}
}

func newTestAdapter(serverURL string) *DeltaReconciliationAdapter {
	return &DeltaReconciliationAdapter{
		apiKey:    "test-key",
		apiSecret: "test-secret",
		http:      http.DefaultClient,
		baseURL:   serverURL,
	}
}

func TestGetBalances_ParsesStringEncodedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/wallet/balances" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":[{"asset_symbol":"USDT","balance":"1000.50","order_margin":"10.25","position_margin":"5.00","unrealized_funding_pnl":"-0.10","unrealized_pnl":"2.75","available_balance":"984.50"}]}`))
	}))
	defer srv.Close()

	adapter := newTestAdapter(srv.URL)
	balances, err := adapter.GetBalances(context.Background())
	if err != nil {
		t.Fatalf("GetBalances failed: %v", err)
	}
	if len(balances) != 1 {
		t.Fatalf("got %d balances, want 1", len(balances))
	}
	b := balances[0]
	if b.Asset != "USDT" || b.WalletBalance != 1000.50 || b.Available != 984.50 {
		t.Fatalf("unexpected balance: %+v", b)
	}
	if b.MarginUsed != 15.25 {
		t.Fatalf("got MarginUsed=%v, want 15.25", b.MarginUsed)
	}
}

func TestGetPositions_UsesMarginedEndpointAndParsesStringFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The old, broken implementation called /v2/positions, which 400s
		// without a product filter. The fix must call /v2/positions/margined.
		if r.URL.Path != "/v2/positions/margined" {
			t.Fatalf("expected /v2/positions/margined, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"result":[{"product_id":27,"product_symbol":"BTCUSD","side":"buy","size":"0.5","entry_price":"63000.00","mark_price":"63250.00","unrealised_pnl":"125.00","margin":"500.00"}]}`))
	}))
	defer srv.Close()

	adapter := newTestAdapter(srv.URL)
	positions, err := adapter.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("GetPositions failed: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("got %d positions, want 1", len(positions))
	}
	p := positions[0]
	if p.Symbol != "BTCUSD" || p.Quantity != 0.5 || p.EntryPrice != 63000.00 {
		t.Fatalf("unexpected position: %+v", p)
	}
	if p.UnrealizedPnL != 125.00 || p.MarginUsed != 500.00 {
		t.Fatalf("unexpected pnl/margin: %+v", p)
	}
}

func TestGetPositions_SkipsZeroSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"result":[{"product_symbol":"BTCUSD","side":"buy","size":"0"}]}`))
	}))
	defer srv.Close()

	adapter := newTestAdapter(srv.URL)
	positions, err := adapter.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("GetPositions failed: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("got %d positions, want 0 (zero-size should be filtered)", len(positions))
	}
}

// A control that cannot fail is not a control. Reconciliation must never read a
// non-position response as "zero positions = reconciled". These lock the
// silent-false-green paths: a Delta error envelope, an unrecognized body, an
// HTTP error, and unparseable JSON must all surface as errors — only a genuine
// success:true payload may be trusted (including a genuinely empty position set).

func TestGetPositions_ErrorEnvelopeIsNotSilentlyEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Delta returns HTTP 200 with success:false on auth/permission failures.
		w.Write([]byte(`{"success":false,"error":{"code":"unauthorized"}}`))
	}))
	defer srv.Close()

	positions, err := newTestAdapter(srv.URL).GetPositions(context.Background())
	if err == nil {
		t.Fatalf("expected error on success:false envelope, got nil with %d positions", len(positions))
	}
}

func TestGetPositions_UnrecognizedBodyIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Parses as JSON but is not a positions payload (no success flag).
		w.Write([]byte(`{"foo":"bar"}`))
	}))
	defer srv.Close()

	positions, err := newTestAdapter(srv.URL).GetPositions(context.Background())
	if err == nil {
		t.Fatalf("expected error on body missing success flag, got nil with %d positions", len(positions))
	}
}

func TestGetPositions_HTTPErrorIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success":false}`))
	}))
	defer srv.Close()

	if _, err := newTestAdapter(srv.URL).GetPositions(context.Background()); err == nil {
		t.Fatal("expected error on HTTP 401, got nil")
	}
}

func TestGetPositions_UnparseableBodyIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`<html>gateway timeout</html>`))
	}))
	defer srv.Close()

	if _, err := newTestAdapter(srv.URL).GetPositions(context.Background()); err == nil {
		t.Fatal("expected error on unparseable body, got nil")
	}
}

func TestGetPositions_EmptyButSuccessfulIsTrusted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"success":true,"result":[]}`))
	}))
	defer srv.Close()

	positions, err := newTestAdapter(srv.URL).GetPositions(context.Background())
	if err != nil {
		t.Fatalf("genuine empty position set must be trusted, got error: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("got %d positions, want 0", len(positions))
	}
}

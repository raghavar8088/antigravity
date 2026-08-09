package delta

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Perpetual sizing is where a scalp strategy reaching real money can go most
// badly wrong, and it fails quietly.
//
// The options path can hardcode 0.001 BTC per contract because every BTC option
// is that size. Perpetuals are not: Delta's live tickers report contract values
// spanning a THOUSANDFOLD range, verified before this was written —
//
//	BTCUSD 0.001   ETHUSD 0.01   BNBUSD 0.1   ADAUSD 1.0   SOLUSD 1.0   XRPUSD 1.0
//
// The two symbols on the scalp leaderboard, ADAUSD and BNBUSD, sit at the far
// end of that range from BTC. Sizing an ADAUSD ticket with the options
// assumption asks for a thousand times the intended position.

// Verbatim shape from /v2/tickers?contract_types=perpetual_futures, with the
// real contract values.
const realPerpTickers = `{"success":true,"result":[
{"symbol":"BTCUSD","product_id":27,"contract_type":"perpetual_futures","contract_value":"0.001000000000000000","tick_size":"0.500000000000000000","mark_price":"63083.98754328"},
{"symbol":"ADAUSD","product_id":16,"contract_type":"perpetual_futures","contract_value":"1.000000000000000000","tick_size":"0.000010000000000000","mark_price":"0.17258828"},
{"symbol":"BNBUSD","product_id":21,"contract_type":"perpetual_futures","contract_value":"0.100000000000000000","tick_size":"0.001000000000000000","mark_price":"579.25787832"},
{"symbol":"ETHUSD","product_id":3136,"contract_type":"perpetual_futures","contract_value":"0.010000000000000000","tick_size":"0.050000000000000000","mark_price":"1867.22931325"}
]}`

func registryFrom(t *testing.T, payload string) *PerpRegistry {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)

	r := NewPerpRegistry()
	r.baseURL = srv.URL
	if err := r.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return r
}

// THE TRAP. A $3,000 scalp ticket on ADAUSD at $0.1726:
//
//	correct:  3000 / (0.1726 x 1.0)   = 17,382 contracts
//	with 0.001: 3000 / (0.1726 x 0.001) = 17,382,000 contracts
//
// The wrong number is not obviously absurd on its face — it is just a big
// integer — and the venue would reject it for margin, which reads as an
// unrelated problem rather than as a sizing bug.
func TestPerpSizing_UsesTheVenuesContractValueNotBTCs(t *testing.T) {
	r := registryFrom(t, realPerpTickers)

	n, p, err := r.SizeContracts("ADAUSD", 3000, 0.17258828)
	if err != nil {
		t.Fatalf("SizeContracts: %v", err)
	}
	if p.ContractValue != 1.0 {
		t.Fatalf("ADAUSD contract value read as %v, want 1.0", p.ContractValue)
	}
	if n != 17382 {
		t.Errorf("sized %d contracts, want 17382; a 0.001 assumption would give ~17,382,000", n)
	}

	// And the same notional on BTCUSD must land somewhere entirely different.
	nb, _, err := r.SizeContracts("BTCUSD", 3000, 63083.98754328)
	if err != nil {
		t.Fatalf("SizeContracts BTC: %v", err)
	}
	if nb != 47 {
		t.Errorf("BTCUSD sized %d contracts, want 47", nb)
	}
}

func TestPerpSizing_HandlesEveryListedContractValue(t *testing.T) {
	r := registryFrom(t, realPerpTickers)
	for _, tc := range []struct {
		sym   string
		price float64
		want  int
	}{
		{"BNBUSD", 579.25787832, 51},   // 3000 / (579.26 x 0.1)
		{"ETHUSD", 1867.22931325, 160}, // 3000 / (1867.23 x 0.01)
	} {
		n, _, err := r.SizeContracts(tc.sym, 3000, tc.price)
		if err != nil {
			t.Errorf("%s: %v", tc.sym, err)
			continue
		}
		if n != tc.want {
			t.Errorf("%s sized %d, want %d", tc.sym, n, tc.want)
		}
	}
}

// An unknown symbol must REFUSE, not fall back to a default. A default is
// exactly how the 0.001 assumption would find its way back in.
func TestPerpSizing_RefusesAnUnknownSymbol(t *testing.T) {
	r := registryFrom(t, realPerpTickers)
	if _, _, err := r.SizeContracts("DOGEUSD", 3000, 0.4); !errors.Is(err, ErrUnknownPerp) {
		t.Fatalf("unknown symbol gave %v, want ErrUnknownPerp — never a guessed contract value", err)
	}
}

// Stale product data must refuse too: a product ID that has moved routes the
// order to a different instrument entirely.
func TestPerpSizing_RefusesWhenTheRegistryIsStale(t *testing.T) {
	r := registryFrom(t, realPerpTickers)
	r.mu.Lock()
	r.fetchedAt = time.Now().Add(-2 * perpRegistryMaxAge)
	r.mu.Unlock()

	if _, _, err := r.SizeContracts("ADAUSD", 3000, 0.17); !errors.Is(err, ErrPerpRegistryStale) {
		t.Fatalf("stale registry gave %v, want ErrPerpRegistryStale", err)
	}
}

// Rounding DOWN matters: rounding up places a bigger position than asked for,
// which on a leveraged perpetual is the direction that hurts.
func TestPerpSizing_RoundsDown(t *testing.T) {
	r := registryFrom(t, realPerpTickers)
	// 1.9 contracts' worth of BNBUSD.
	n, _, err := r.SizeContracts("BNBUSD", 579.25787832*0.1*1.9, 579.25787832)
	if err != nil {
		t.Fatalf("SizeContracts: %v", err)
	}
	if n != 1 {
		t.Errorf("sized %d, want 1 — a partial contract must round down, never up", n)
	}
}

// A notional too small for even one contract must error rather than return 0.
// A zero-size order is silently a no-op, which looks like a strategy that
// declined to trade.
func TestPerpSizing_RefusesASubContractNotional(t *testing.T) {
	r := registryFrom(t, realPerpTickers)
	if _, _, err := r.SizeContracts("BTCUSD", 1, 63083.98); err == nil {
		t.Fatal("$1 on BTCUSD sized without error; it is below one contract and must be refused")
	}
}

// Products missing a contract value or product ID must be DROPPED, not admitted
// with zeros: a zero contract value divides by zero, and product ID 0 routes an
// order to whatever Delta has there.
func TestPerpRegistry_DropsUnusableProducts(t *testing.T) {
	r := registryFrom(t, `{"success":true,"result":[
	{"symbol":"GOODUSD","product_id":9,"contract_value":"1.0","tick_size":"0.01","mark_price":"10"},
	{"symbol":"NOVALUE","product_id":10,"contract_value":"0","tick_size":"0.01","mark_price":"10"},
	{"symbol":"NOID","product_id":0,"contract_value":"1.0","tick_size":"0.01","mark_price":"10"},
	{"symbol":"","product_id":11,"contract_value":"1.0","tick_size":"0.01","mark_price":"10"}]}`)

	if r.Count() != 1 {
		t.Fatalf("registry holds %d products, want 1 — the rest are unusable", r.Count())
	}
	if _, ok := r.Lookup("NOVALUE"); ok {
		t.Error("a product with contract_value 0 was admitted; sizing would divide by zero")
	}
	if _, ok := r.Lookup("NOID"); ok {
		t.Error("a product with product_id 0 was admitted; orders would route to ID 0")
	}
}

// A response with nothing usable must be an error, so the registry keeps its
// previous good data rather than silently emptying.
func TestPerpRegistry_EmptyResponseIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": []any{}})
	}))
	defer srv.Close()

	reg := NewPerpRegistry()
	reg.baseURL = srv.URL
	if err := reg.Refresh(context.Background()); err == nil {
		t.Fatal("an empty product list refreshed without error")
	}
	if reg.Count() != 0 {
		t.Errorf("registry populated from an empty response")
	}
}

// Before any refresh, nothing is tradeable at all.
func TestPerpRegistry_NothingIsTradeableBeforeRefresh(t *testing.T) {
	r := NewPerpRegistry()
	if r.Count() != 0 {
		t.Fatalf("fresh registry already holds %d products", r.Count())
	}
	if _, _, err := r.SizeContracts("BTCUSD", 3000, 63000); err == nil {
		t.Fatal("sized an order before the registry was ever populated")
	}
}

func TestPerpRegistry_LookupIsCaseAndSpaceTolerant(t *testing.T) {
	r := registryFrom(t, realPerpTickers)
	for _, s := range []string{"adausd", " ADAUSD ", "AdaUsd"} {
		if _, ok := r.Lookup(s); !ok {
			t.Errorf("Lookup(%q) missed", s)
		}
	}
}

// Maintenance margin must come from the PRODUCT, not a constant.
//
// The bridge hardcoded 0.5%, true for 13 of the 92 perpetuals this desk trades.
// 56 require 2.5%, where the real liquidation distance at 10x is 7.50% and not
// the 9.50% the constant implied. Harmless at today's 0.35-0.98% stops, and
// exactly the shape of the 2026-08-01 failure: an assumed margin number put the
// liquidation price inside the strategy's own stop and the venue closed every
// losing trade first.
func TestPerpProduct_MaintenanceMarginIsReadPerContract(t *testing.T) {
	p, ok := perpFromRow(perpTickerRow{
		Symbol: "BEATUSD", ProductID: 1, ContractValue: "1", TickSize: "0.001",
		MarkPrice: "2.1", MaintenanceMargin: "2.5",
	}, time.Now())
	if !ok {
		t.Fatal("a well-formed row was rejected")
	}
	if p.MaintenanceMarginPct != 2.5 {
		t.Errorf("maintenance margin %.2f, want the product's 2.5", p.MaintenanceMarginPct)
	}
	if got := LiquidationDistanceFraction(PerpLeverage, p.MaintenanceMarginPct); math.Abs(got-0.075) > 1e-9 {
		t.Errorf("liquidation distance %.4f, want 0.075 — at 2.5%% maintenance it is 7.5%%, not 9.5%%", got)
	}
	// The stop must still be reachable there, or the guard would block a symbol
	// the desk can legitimately trade.
	if !StopIsReachable(2.1, 2.1*(1-0.0098), PerpLeverage, p.MaintenanceMarginPct) {
		t.Error("the widest stop was judged unreachable at 2.5% maintenance; it has 7.5% of room")
	}
}

// A missing value must fall back to the WIDEST margin Delta uses, not the
// narrowest. Guessing low overstates the liquidation distance and would approve
// a stop the venue pre-empts — the failure this whole guard exists for.
func TestPerpProduct_UnknownMaintenanceMarginFailsConservative(t *testing.T) {
	for _, raw := range []string{"", "not-a-number", "0", "-1"} {
		p, ok := perpFromRow(perpTickerRow{
			Symbol: "X", ProductID: 1, ContractValue: "1", TickSize: "0.01",
			MarkPrice: "10", MaintenanceMargin: raw,
		}, time.Now())
		if !ok {
			t.Fatalf("row with maintenance %q was rejected entirely", raw)
		}
		if p.MaintenanceMarginPct != perpFallbackMaintenanceMarginPct {
			t.Errorf("maintenance %q -> %.2f, want the conservative %.2f",
				raw, p.MaintenanceMarginPct, perpFallbackMaintenanceMarginPct)
		}
	}
}

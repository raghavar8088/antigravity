package marketdata

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// The IV index moved from Deribit's DVOL to Delta's own chain, because the desks
// that consume it quote against Delta's options. Two things could break silently
// in that move: the unit, and the underlying filter.

// Shapes are taken from the live endpoint
// (https://api.india.delta.exchange/v2/tickers?contract_types=call_options,put_options),
// including the flat underlying_asset_symbol that /v2/tickers uses.
const realDeltaOptionTickers = `{"success":true,"result":[
{"symbol":"P-BTC-85000-280826","contract_type":"put_options","strike_price":"85000","spot_price":"62993.7","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.10641259"}},
{"symbol":"C-XAUT-4180-010826","contract_type":"call_options","strike_price":"4180","spot_price":"4045.06","underlying_asset_symbol":"XAUT","quotes":{"mark_iv":"0.24837116"}}
]}`

func parseTickers(t *testing.T, raw string) []deltaOptionTicker {
	t.Helper()
	var r deltaOptionTickersResponse
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return r.Result
}

// TRAP 1: mark_iv is a DECIMAL; the index is consumed as a PERCENTAGE.
//
// RealizedVol here is computed as "stdev * sqrt(24*365) * 100 // comparable to
// DVOL". Leave the decimal unscaled and implied vol reads a hundred times
// smaller than realised, so every IV-vs-RV comparison inverts: options look
// permanently cheap and vol-selling strategies see edge that is not there.
func TestIVIndex_ConvertsDecimalIVToPercent(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// One ATM call and one ATM put, both at 0.30 = 30%.
	raw := `{"success":true,"result":[
	{"symbol":"C-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.30"}},
	{"symbol":"P-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.30"}}]}`

	got, err := dvolFromChain(parseTickers(t, raw), now)
	if err != nil {
		t.Fatalf("dvolFromChain: %v", err)
	}
	if math.Abs(got-30.0) > 1e-9 {
		t.Fatalf("index = %v, want 30 (percent). A raw decimal would read 0.30", got)
	}
}

// TRAP 2: underlying_asset_symbol is FLAT on /v2/tickers and NESTED on
// /v2/products. Reading the wrong one filters every BTC contract out and the
// index reports nothing — with no error to notice.
func TestIVIndex_FiltersToBTCAndReadsEitherUnderlyingShape(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// Flat spelling, mixed with a non-BTC underlying that must be excluded.
	got, err := dvolFromChain(parseTickers(t, realDeltaOptionTickers), now)
	if err != nil {
		t.Fatalf("flat underlying: %v", err)
	}
	if math.Abs(got-10.641259) > 1e-6 {
		t.Errorf("index = %v; want the BTC put's 10.64%%, with the XAUT contract excluded", got)
	}

	// Nested spelling must work too.
	nested := `{"success":true,"result":[
	{"symbol":"C-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset":{"symbol":"BTC"},"quotes":{"mark_iv":"0.40"}}]}`
	got, err = dvolFromChain(parseTickers(t, nested), now)
	if err != nil {
		t.Fatalf("nested underlying: %v", err)
	}
	if math.Abs(got-40.0) > 1e-9 {
		t.Errorf("nested underlying gave %v, want 40", got)
	}
}

// The index must be built from the expiry nearest 30 days, not from whatever
// contract happens to be listed first. A 0-DTE contract's IV is a different
// quantity entirely.
func TestIVIndex_PicksTheExpiryNearestThirtyDays(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	raw := `{"success":true,"result":[
	{"symbol":"C-BTC-63000-020826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.90"}},
	{"symbol":"C-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.35"}},
	{"symbol":"C-BTC-63000-291226","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.55"}}]}`

	got, err := dvolFromChain(parseTickers(t, raw), now)
	if err != nil {
		t.Fatalf("dvolFromChain: %v", err)
	}
	if math.Abs(got-35.0) > 1e-9 {
		t.Errorf("index = %v, want 35 — the 30-Aug expiry is nearest the 30-day tenor", got)
	}
}

// Within the chosen expiry it must take the strike nearest spot, not the
// furthest. A deep OTM wing's IV is a skew reading, not a level reading.
func TestIVIndex_PicksTheStrikeNearestSpot(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	raw := `{"success":true,"result":[
	{"symbol":"C-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.30"}},
	{"symbol":"C-BTC-120000-310826","strike_price":"120000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.95"}}]}`

	got, err := dvolFromChain(parseTickers(t, raw), now)
	if err != nil {
		t.Fatalf("dvolFromChain: %v", err)
	}
	if math.Abs(got-30.0) > 1e-9 {
		t.Errorf("index = %v, want 30 — the 120k wing is far OTM", got)
	}
}

// Expired contracts carry no meaningful IV and must not be selected.
func TestIVIndex_IgnoresExpiredContracts(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	raw := `{"success":true,"result":[
	{"symbol":"C-BTC-63000-010726","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.99"}},
	{"symbol":"C-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0.30"}}]}`

	got, err := dvolFromChain(parseTickers(t, raw), now)
	if err != nil {
		t.Fatalf("dvolFromChain: %v", err)
	}
	if math.Abs(got-30.0) > 1e-9 {
		t.Errorf("index = %v; the July contract had already expired", got)
	}
}

// An empty or unusable chain must be an error, not 0. Zero implied volatility is
// a meaningful reading — it would mean the market expects no movement at all —
// so returning it on failure asserts something false rather than nothing.
func TestIVIndex_UnusableChainErrorsRatherThanReturningZero(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	for _, raw := range []string{
		`{"success":true,"result":[]}`,
		`{"success":true,"result":[{"symbol":"C-ETH-3000-310826","strike_price":"3000","spot_price":"3000","underlying_asset_symbol":"ETH","quotes":{"mark_iv":"0.5"}}]}`,
		`{"success":true,"result":[{"symbol":"C-BTC-63000-310826","strike_price":"63000","spot_price":"63000","underlying_asset_symbol":"BTC","quotes":{"mark_iv":"0"}}]}`,
	} {
		if v, err := dvolFromChain(parseTickers(t, raw), now); err == nil {
			t.Errorf("payload %s returned %v with no error", raw, v)
		}
	}
}

func TestDeltaOptionExpiry_ParsesDelasSymbolFormat(t *testing.T) {
	got, ok := deltaOptionExpiry("P-BTC-85000-280826")
	if !ok {
		t.Fatal("failed to parse a real Delta option symbol")
	}
	want := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expiry = %s, want %s (DDMMYY)", got, want)
	}
	for _, bad := range []string{"", "BTCUSD", "C-BTC-85000", "C-BTC-85000-28082", "C-BTC-85000-ABCDEF"} {
		if _, ok := deltaOptionExpiry(bad); ok {
			t.Errorf("%q parsed as a valid expiry", bad)
		}
	}
}

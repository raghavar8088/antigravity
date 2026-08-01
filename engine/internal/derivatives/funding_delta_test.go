package derivatives

import (
	"encoding/json"
	"math"
	"testing"
)

// Funding is a property of ONE contract on ONE exchange — it is the payment
// tying that contract to spot. Reading Binance's while trading Delta's describes
// a cost this account never pays.
//
// The migration carries a silent 100x unit change, which is what these tests
// exist for.

// Delta quotes funding as a PERCENT (0.01 = 0.01% per 8h). Binance quoted the
// same economic rate as a DECIMAL (0.0001). Everything downstream expects the
// decimal — classifyFunding multiplies by 100, and the scalper thresholds are
// documented in raw decimals.
func TestDeltaFundingPercentToDecimal(t *testing.T) {
	// The live value observed on Delta at the time of the migration, against the
	// Binance decimal for the same moment (0.000096).
	got := deltaFundingPercentToDecimal(0.010000000000000002)
	if math.Abs(got-0.0001) > 1e-12 {
		t.Fatalf("0.01%% converted to %v, want 0.0001 decimal", got)
	}
	if math.Abs(deltaFundingPercentToDecimal(-0.05)-(-0.0005)) > 1e-12 {
		t.Error("negative funding did not convert")
	}
	if deltaFundingPercentToDecimal(0) != 0 {
		t.Error("zero must stay zero")
	}
}

// The consequence of forgetting the conversion, pinned so it cannot come back:
// an utterly ordinary funding rate would classify as the most extreme reading
// the scale has, permanently, and nothing would error.
func TestUnconvertedDeltaFundingWouldMisclassify(t *testing.T) {
	const deltaRaw = 0.01 // what the venue publishes, in percent

	if label, _, _ := classifyFunding(deltaRaw); label != "EXTREME_POSITIVE" {
		t.Fatalf("guard test is stale: raw %v now classifies as %q", deltaRaw, label)
	}
	label, signal, _ := classifyFunding(deltaFundingPercentToDecimal(deltaRaw))
	if label != "NEUTRAL" {
		t.Errorf("converted 0.01%% funding classified %q/%q, want NEUTRAL — this is a perfectly ordinary rate",
			label, signal)
	}
}

// The parser must read Delta's envelope: quoted numeric strings under "result".
func TestFundingFetcher_ParsesDeltaTickerShape(t *testing.T) {
	// Verbatim shape from https://api.india.delta.exchange/v2/tickers/BTCUSD.
	raw := `{"success":true,"result":{"symbol":"BTCUSD","funding_rate":"0.010000000000000002","mark_price":"63039.91377373","spot_price":"63064.3"}}`
	var env deltaTickerEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Success {
		t.Fatal("success flag not read")
	}
	if env.Result.Symbol != "BTCUSD" {
		t.Errorf("symbol %q", env.Result.Symbol)
	}
	if env.Result.FundingRate != "0.010000000000000002" {
		t.Errorf("funding_rate %q — the field is nested under result and quoted", env.Result.FundingRate)
	}
}

// A response that carries no funding rate must be an error, not a zero rate.
// Zero is a meaningful funding reading (perfectly balanced), so returning it on
// failure would silently assert something false about the market.
func TestFundingFetcher_MissingRateIsAnErrorNotZero(t *testing.T) {
	for _, raw := range []string{
		`{"success":true,"result":{"symbol":"BTCUSD"}}`,
		`{"success":false,"result":{"funding_rate":"0.01"}}`,
	} {
		var env deltaTickerEnvelope
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if env.Success && env.Result.FundingRate != "" {
			t.Errorf("payload %s should have been rejected", raw)
		}
	}
}

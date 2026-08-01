package funding

import (
	"context"
	"math"
	"testing"
)

// The collector injects funding straight into the alpha scalpers. It read
// Binance and Bybit, so those strategies reasoned about leverage in books this
// engine does not trade.

// Delta quotes funding as a PERCENT; every consumer here expects a DECIMAL.
// Skipping the conversion is silent — an unremarkable rate arrives a hundred
// times too large and every funding-sensitive strategy reads a permanent
// extreme.
func TestDeltaFundingPercentToDecimal(t *testing.T) {
	if got := DeltaFundingPercentToDecimal(0.01); math.Abs(got-0.0001) > 1e-12 {
		t.Errorf("0.01%% -> %v, want 0.0001", got)
	}
	if got := DeltaFundingPercentToDecimal(-0.05); math.Abs(got-(-0.0005)) > 1e-12 {
		t.Errorf("negative funding -> %v, want -0.0005", got)
	}
}

// Callers are configured with Binance-era symbols. Delta lists no "BTCUSDT", and
// asking for one returns an error rather than a rate — so without translation
// the funding feed would stop, quietly, on every existing deployment.
func TestDeltaPerpSymbol_TranslatesBinanceNotation(t *testing.T) {
	for in, want := range map[string]string{
		"BTCUSDT": "BTCUSD",
		"ETHUSDT": "ETHUSD",
		"BTCUSD":  "BTCUSD",
		"":        "BTCUSD",
	} {
		if got := DeltaPerpSymbol(in); got != want {
			t.Errorf("DeltaPerpSymbol(%q) = %q, want %q", in, got, want)
		}
	}
}

// The collector must route "delta" somewhere rather than falling through to the
// unsupported-exchange error.
func TestCollector_SupportsDelta(t *testing.T) {
	c := NewCollector()
	_, err := c.Fetch(context.Background(), "delta", "BTCUSD")
	if err != nil && err.Error() == `unsupported funding exchange "delta"` {
		t.Fatal("delta is not wired into Fetch")
	}
	// Any other error (network, etc.) is acceptable here; routing is what matters.
}

func TestCollector_StillRejectsUnknownExchanges(t *testing.T) {
	if _, err := NewCollector().Fetch(context.Background(), "nosuchvenue", "BTCUSD"); err == nil {
		t.Fatal("an unknown exchange must error rather than silently returning a zero snapshot")
	}
}

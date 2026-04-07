package marketdata

import "testing"

func TestParseNifty50Quote(t *testing.T) {
	payload := nseAllIndicesResponse{
		Timestamp: "07-Apr-2026 15:30:00",
		Data: []nseAllIndicesRecord{
			{
				Index:         "NIFTY 50",
				IndexSymbol:   "NIFTY 50",
				Last:          "24,567.80",
				Variation:     "112.45",
				PercentChange: "0.46",
			},
		},
	}

	quote, err := parseNifty50Quote(payload)
	if err != nil {
		t.Fatalf("parseNifty50Quote returned error: %v", err)
	}

	if quote.Index != nifty50IndexName {
		t.Fatalf("expected index %q, got %q", nifty50IndexName, quote.Index)
	}
	if quote.Price != 24567.80 {
		t.Fatalf("expected price 24567.80, got %f", quote.Price)
	}
	if quote.Change != 112.45 {
		t.Fatalf("expected change 112.45, got %f", quote.Change)
	}
	if quote.PercentChange != 0.46 {
		t.Fatalf("expected percent change 0.46, got %f", quote.PercentChange)
	}
	if quote.ExchangeTime != payload.Timestamp {
		t.Fatalf("expected timestamp %q, got %q", payload.Timestamp, quote.ExchangeTime)
	}
}

func TestParseNSEFloatSupportsNumbersAndStrings(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  float64
	}{
		{name: "float", value: 24501.35, want: 24501.35},
		{name: "string", value: "24,501.35", want: 24501.35},
		{name: "int", value: 24501, want: 24501},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseNSEFloat(tc.value)
			if err != nil {
				t.Fatalf("parseNSEFloat returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %f, got %f", tc.want, got)
			}
		})
	}
}

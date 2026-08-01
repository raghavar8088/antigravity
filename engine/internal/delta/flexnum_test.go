package delta

import (
	"encoding/json"
	"testing"
)

// Regression: /v2/positions/margined identifies the instrument as product_symbol
// (symbol is usually absent). Reading only `symbol` produced an empty instrument,
// which blanked the positions UI and made real option positions unadoptable by
// custody — they could not be recognised as options.
func TestPositionsPayload_UsesProductSymbol(t *testing.T) {
	var out struct {
		Result []struct {
			ProductSymbol string  `json:"product_symbol"`
			Symbol        string  `json:"symbol"`
			Size          flexNum `json:"size"`
		} `json:"result"`
	}
	body := `{"result":[{"product_symbol":"P-BTC-64800-290726","size":1}]}`
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sym := out.Result[0].ProductSymbol
	if sym == "" {
		sym = out.Result[0].Symbol
	}
	if sym != "P-BTC-64800-290726" {
		t.Fatalf("instrument must resolve from product_symbol, got %q", sym)
	}
	if !IsOptionSymbol(sym) {
		t.Fatal("resolved symbol must be recognised as an option so custody can adopt it")
	}
}

// Delta returns option position `size` as a JSON number but prices as strings.
// flexNum must parse both, so a real option position never fails to unmarshal.
func TestFlexNum_ParsesNumberAndString(t *testing.T) {
	var out struct {
		Size  flexNum `json:"size"`
		Price flexNum `json:"price"`
		Empty flexNum `json:"empty"`
		Null  flexNum `json:"null"`
	}
	body := `{"size":1,"price":"0.51","empty":"","null":null}`
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("unmarshal failed (the exact bug that hid the real position): %v", err)
	}
	if float64(out.Size) != 1 {
		t.Fatalf("size(number) got %v want 1", float64(out.Size))
	}
	if float64(out.Price) != 0.51 {
		t.Fatalf("price(string) got %v want 0.51", float64(out.Price))
	}
	if out.Empty != 0 || out.Null != 0 {
		t.Fatalf("empty/null must be 0, got %v %v", out.Empty, out.Null)
	}
}

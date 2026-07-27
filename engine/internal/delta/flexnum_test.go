package delta

import (
	"encoding/json"
	"testing"
)

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

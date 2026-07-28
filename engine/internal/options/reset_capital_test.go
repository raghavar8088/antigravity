package options

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Reset must be able to re-base the desk on a caller-chosen balance, and must
// keep its old behaviour when no balance is supplied — an empty POST from the
// existing UI has to keep working exactly as before.

func TestResetAccountWith_UsesRequestedCapital(t *testing.T) {
	e := NewEngine()
	snap := e.ResetAccountWith(4200)
	if snap.Balance != 4200 {
		t.Fatalf("balance = %v, want 4200", snap.Balance)
	}
	if len(snap.Trades) != 0 {
		t.Errorf("reset must clear trades, got %d", len(snap.Trades))
	}
}

func TestResetAccountWith_NonPositiveFallsBackToDefault(t *testing.T) {
	e := NewEngine()
	want := getInitialOptionsBalanceUSD()
	for _, v := range []float64{0, -1, -0.0001} {
		if got := e.ResetAccountWith(v).Balance; got != want {
			t.Errorf("ResetAccountWith(%v) = %v, want env default %v", v, got, want)
		}
	}
	// The zero-arg form must stay identical to the old behaviour.
	if got := e.ResetAccount().Balance; got != want {
		t.Errorf("ResetAccount() = %v, want %v", got, want)
	}
}

func TestParseRequestedCapital(t *testing.T) {
	cases := []struct {
		name string
		body string
		want float64
	}{
		{"valid", `{"initialCapital": 2500.5}`, 2500.5},
		{"absent field", `{}`, 0},
		{"empty body", ``, 0},
		{"malformed json", `{not json`, 0},
		{"zero", `{"initialCapital": 0}`, 0},
		{"negative", `{"initialCapital": -100}`, 0},
		// A bad body must never fail the reset — it degrades to the default.
		{"wrong type", `{"initialCapital": "lots"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/options/reset", strings.NewReader(tc.body))
			if got := parseRequestedCapital(r); got != tc.want {
				t.Errorf("parseRequestedCapital(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

// An empty POST is what the pre-existing reset button sends; it must still work.
func TestHandleReset_EmptyBodyStillResets(t *testing.T) {
	e := NewEngine()
	r := httptest.NewRequest("POST", "/api/options/reset", strings.NewReader(""))
	w := httptest.NewRecorder()
	e.HandleReset(w, r)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"status":"reset"`) {
		t.Errorf("body = %q, want a reset status", w.Body.String())
	}
}

func TestHandleReset_AppliesCapitalFromBody(t *testing.T) {
	e := NewEngine()
	r := httptest.NewRequest("POST", "/api/options/reset", strings.NewReader(`{"initialCapital":777}`))
	w := httptest.NewRecorder()
	e.HandleReset(w, r)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := e.ExportState().Balance; got != 777 {
		t.Errorf("balance after reset = %v, want 777", got)
	}
}

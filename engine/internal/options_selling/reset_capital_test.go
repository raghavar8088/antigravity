package options_selling

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Same contract as the buying desk: reset can re-base on a chosen balance, and
// an empty POST keeps the previous behaviour so the existing UI is unaffected.

func TestResetAccountWith_UsesRequestedCapital(t *testing.T) {
	e := NewEngine()
	snap := e.ResetAccountWith(3300)
	if snap.Balance != 3300 {
		t.Fatalf("balance = %v, want 3300", snap.Balance)
	}
	if len(snap.Trades) != 0 {
		t.Errorf("reset must clear trades, got %d", len(snap.Trades))
	}
}

// A missing or nonsensical capital must fall back to the desk's CONFIGURED size,
// never to something arbitrary.
//
// With hunt mode on — the default, and how this desk actually runs — that size
// is the hunt floor: every strategy funded with its own stake. Falling back to
// the bare env default left the desk unable to open most of its positions, so
// the leaderboard measured which strategy signalled first rather than which had
// an edge.
func TestResetAccountWith_NonPositiveFallsBackToTheDesksConfiguredSize(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	e := NewEngine()
	want := huntDeskBalance(len(e.states))
	for _, v := range []float64{0, -1} {
		if got := e.ResetAccountWith(v).Balance; got != want {
			t.Errorf("ResetAccountWith(%v) = %v, want the hunt-funded size %v", v, got, want)
		}
	}
	if got := e.ResetAccount().Balance; got != want {
		t.Errorf("ResetAccount() = %v, want %v", got, want)
	}
}

// With hunt mode OFF the rotating roster shares one pot, so the bare configured
// default is correct and the floor must not inflate it.
func TestResetAccountWith_HuntModeOffUsesTheBareDefault(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "false")
	t.Setenv("INITIAL_OPTIONS_BALANCE_USD", "2500")
	e := NewEngine()
	if got := e.ResetAccountWith(0).Balance; got != 2500 {
		t.Errorf("ResetAccountWith(0) = %v with hunt mode off, want the env default 2500", got)
	}
}

func TestParseRequestedCapital(t *testing.T) {
	cases := []struct {
		name string
		body string
		want float64
	}{
		{"valid", `{"initialCapital": 1500}`, 1500},
		{"absent", `{}`, 0},
		{"empty", ``, 0},
		{"malformed", `{oops`, 0},
		{"negative", `{"initialCapital": -5}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/options-selling/reset", strings.NewReader(tc.body))
			if got := parseRequestedCapital(r); got != tc.want {
				t.Errorf("parseRequestedCapital(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestHandleReset_AppliesCapitalFromBody(t *testing.T) {
	e := NewEngine()
	r := httptest.NewRequest("POST", "/api/options-selling/reset", strings.NewReader(`{"initialCapital":999}`))
	w := httptest.NewRecorder()
	e.HandleReset(w, r)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := e.ExportState().Balance; got != 999 {
		t.Errorf("balance after reset = %v, want 999", got)
	}
}

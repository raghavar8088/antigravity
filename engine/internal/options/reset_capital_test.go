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

// A missing or nonsensical capital must fall back to the desk's CONFIGURED size,
// never to something arbitrary.
//
// With hunt mode on — the default, and how these desks actually run — the
// configured size is the hunt floor: every strategy funded with its own stake.
// Falling back to the bare env default instead left the desk unable to open most
// of its positions, so the leaderboard measured which strategy signalled first
// rather than which had an edge.
func TestResetAccountWith_NonPositiveFallsBackToTheDesksConfiguredSize(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	e := NewEngine()
	want := huntDeskBalance(len(e.states))
	for _, v := range []float64{0, -1, -0.0001} {
		if got := e.ResetAccountWith(v).Balance; got != want {
			t.Errorf("ResetAccountWith(%v) = %v, want the hunt-funded size %v", v, got, want)
		}
	}
	// The zero-arg form must behave identically.
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

// A reset must leave the desk able to run the hunt it was built for.
//
// ResetAccountWith did not apply the hunt-mode funding floor that NewEngine
// applies, so a reset refunded the desk to the bare configured default — $100
// against a hundred-odd strategies each wanting a $1,000 stake. Nothing fails
// visibly at that balance: the desk simply cannot open most of its positions, so
// the leaderboard begins measuring which strategy signalled FIRST rather than
// which has an edge. That is the exact failure hunt mode exists to prevent, and
// every reset silently re-introduced it.
func TestResetAccount_KeepsTheDeskFundedForTheHunt(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	t.Setenv("INITIAL_OPTIONS_BALANCE_USD", "100")

	e := NewEngine()
	built := e.balance

	snap := e.ResetAccountWith(0) // 0 = "use the default", the path the API takes

	want := huntDeskBalance(len(e.states))
	if snap.Balance < want {
		t.Fatalf("reset funded the desk with $%.2f for %d strategies; hunt mode needs $%.2f "+
			"(it was built with $%.2f)", snap.Balance, len(e.states), want, built)
	}
	if snap.Balance != built {
		t.Errorf("reset balance $%.2f != as-built $%.2f; a reset should restore the desk, not change its size",
			snap.Balance, built)
	}
}

// An explicit capital request must still win — the reset API accepts
// {"initialCapital": N} and an operator asking for a specific size means it.
func TestResetAccount_ExplicitCapitalOverridesTheHuntFloor(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	e := NewEngine()
	if snap := e.ResetAccountWith(5000); snap.Balance != 5000 {
		t.Errorf("explicit $5000 became $%.2f", snap.Balance)
	}
}

package liveengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler(authorized bool) (*Handler, *Controller) {
	ctrl := New(Hooks{
		IsConfigured:       func() bool { return true },
		KillSwitchActive:   func() bool { return false },
		SetEffectorEnabled: func(bool) {},
		CloseAll:           func(context.Context) (map[string]any, error) { return map[string]any{"closed": 0}, nil },
	})
	authz := Authorizer(func(*http.Request) (string, bool) { return "operator", authorized })
	return NewHandler(ctrl, DataProviders{}, authz), ctrl
}

func do(h *Handler, method, action, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/live-engine/"+action, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHTTP_StateIsGetOnly(t *testing.T) {
	h, _ := newTestHandler(true)
	if rec := do(h, http.MethodPost, "state", ""); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /state should be 405, got %d", rec.Code)
	}
	if rec := do(h, http.MethodGet, "state", ""); rec.Code != http.StatusOK {
		t.Fatalf("GET /state should be 200, got %d", rec.Code)
	}
}

func TestHTTP_ArmRequiresAuthorization(t *testing.T) {
	h, ctrl := newTestHandler(false) // not authorized
	rec := do(h, http.MethodPost, "arm", `{"confirmation":"`+ArmConfirmationPhrase+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unauthorized arm should be 403, got %d", rec.Code)
	}
	if ctrl.IsArmed() {
		t.Fatal("must not arm without authorization")
	}
}

func TestHTTP_ArmRequiresConfirmationPhrase(t *testing.T) {
	h, ctrl := newTestHandler(true)
	rec := do(h, http.MethodPost, "arm", `{"confirmation":"nope"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad confirmation should be 400, got %d", rec.Code)
	}
	if ctrl.IsArmed() {
		t.Fatal("must not arm with wrong confirmation")
	}
}

func TestHTTP_ArmDisarmHappyPath(t *testing.T) {
	h, ctrl := newTestHandler(true)
	rec := do(h, http.MethodPost, "arm", `{"confirmation":"`+ArmConfirmationPhrase+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("arm should be 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !ctrl.IsArmed() {
		t.Fatal("controller should be armed")
	}
	var out struct {
		OK    bool `json:"ok"`
		State struct {
			Armed bool `json:"armed"`
		} `json:"state"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if !out.OK || !out.State.Armed {
		t.Fatalf("arm response should report armed: %s", rec.Body.String())
	}
	rec = do(h, http.MethodPost, "disarm", `{"reason":"done"}`)
	if rec.Code != http.StatusOK || ctrl.IsArmed() {
		t.Fatalf("disarm should succeed and leave disarmed; code=%d armed=%v", rec.Code, ctrl.IsArmed())
	}
}

func TestHTTP_CloseAllRequiresAuthorization(t *testing.T) {
	h, _ := newTestHandler(false)
	if rec := do(h, http.MethodPost, "close-all", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("unauthorized close-all should be 403, got %d", rec.Code)
	}
}

func TestHTTP_AccountAppliesCeiling(t *testing.T) {
	ctrl := New(Hooks{})
	h := NewHandler(ctrl, DataProviders{
		Account: func(context.Context) (AccountView, error) {
			return AccountView{EquityUSD: 5000, Source: "delta", CeilingUSD: 999999}, nil
		},
	}, func(*http.Request) (string, bool) { return "op", true })

	rec := do(h, http.MethodGet, "account", "")
	var acct AccountView
	_ = json.Unmarshal(rec.Body.Bytes(), &acct)
	if acct.CeilingUSD != MaxTradableUSD {
		t.Fatalf("ceiling must be enforced server-side at $%.0f, got $%.2f", MaxTradableUSD, acct.CeilingUSD)
	}
	if acct.TradableUSD != MaxTradableUSD {
		t.Fatalf("tradable must cap at $%.0f even with $5000 equity, got $%.2f", MaxTradableUSD, acct.TradableUSD)
	}
}

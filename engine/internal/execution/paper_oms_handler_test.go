package execution

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestPaperOMSHandler_MutationsBlockedWithoutOverride(t *testing.T) {
	t.Setenv("PAPER_OMS_ADMIN_OVERRIDE", "")

	h := &PaperOMSHandler{OMS: NewPaperOMS(10_000), Symbol: "BTCUSDT"}

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/paper/open", `{"side":"LONG","entryPrice":60000,"notional":100,"leverage":10,"slPct":1,"tpPct":2,"holdMinutes":60}`},
		{http.MethodPost, "/paper/tick", `{"markPrice":61000}`},
		{http.MethodPost, "/paper/close/test-id", `{}`},
		{http.MethodPost, "/paper/reset", `{}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected 403, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPaperOMSHandler_MutationsAllowedWithOverride(t *testing.T) {
	t.Setenv("PAPER_OMS_ADMIN_OVERRIDE", "test-override-token")

	h := &PaperOMSHandler{OMS: NewPaperOMS(10_000), Symbol: "BTCUSDT"}

	req := httptest.NewRequest(http.MethodPost, "/paper/open", bytes.NewBufferString(
		`{"side":"LONG","entryPrice":60000,"notional":100,"leverage":10,"slPct":1,"tpPct":2,"holdMinutes":60}`,
	))
	req.Header.Set(paperOMSAdminOverrideHeader, "test-override-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPaperOMSHandler_StateReadAlwaysAllowed(t *testing.T) {
	os.Unsetenv("PAPER_OMS_ADMIN_OVERRIDE")

	h := &PaperOMSHandler{OMS: NewPaperOMS(10_000), Symbol: "BTCUSDT"}
	req := httptest.NewRequest(http.MethodGet, "/paper/state", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

package liveengine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AccountView is the real broker account, with the server ceiling applied and
// every value labeled with its source and age so the UI never shows a stale
// number as if it were fresh.
type AccountView struct {
	EquityUSD        float64 `json:"equityUsd"`
	TradableUSD      float64 `json:"tradableUsd"` // min(equity, ceiling)
	CeilingUSD       float64 `json:"ceilingUsd"`
	AvailableUSD     float64 `json:"availableUsd"`
	MarginUsedUSD    float64 `json:"marginUsedUsd"`
	OpenRiskUSD      float64 `json:"openRiskUsd"`
	RealizedTodayUSD float64 `json:"realizedTodayUsd"`
	// InceptionEquityUSD is the wallet balance first recorded, and ROI is
	// measured against it.
	//
	// Recorded rather than assumed: without a baseline there is no honest ROI,
	// and picking one — the configured desk equity, say — would divide by a
	// number that was never in the wallet and report a return that never
	// happened. Zero means no baseline has been captured yet, and the UI must
	// show no ROI rather than treat it as a starting balance of nothing.
	InceptionEquityUSD float64   `json:"inceptionEquityUsd"`
	InceptionAt        time.Time `json:"inceptionAt,omitempty"`
	// Set when the baseline has been deliberately re-based.
	//
	// Carried to the UI so a 0.00% can never read as a lifetime figure. An
	// account re-based this morning and an account that has never lost a rupee
	// display the same number, and only these fields tell them apart.
	InceptionResets      int       `json:"inceptionResets,omitempty"`
	InceptionResetAt     time.Time `json:"inceptionResetAt,omitempty"`
	InceptionResetFrom   float64   `json:"inceptionResetFrom,omitempty"`
	InceptionResetReason string    `json:"inceptionResetReason,omitempty"`
	ROIUSD               float64   `json:"roiUsd"`
	ROIPct               float64   `json:"roiPct"`
	DistanceToBreaker    float64   `json:"distanceToBreakerPct"`
	Source               string    `json:"source"`
	AsOf                 time.Time `json:"asOf"`
	Stale                bool      `json:"stale"`
}

// ReconciliationView reports engine state vs. Delta truth. Mismatch is shown loudly.
type ReconciliationView struct {
	Matched         bool      `json:"matched"`
	EnginePositions int       `json:"enginePositions"`
	DeltaPositions  int       `json:"deltaPositions"`
	Mismatches      []string  `json:"mismatches"`
	AsOf            time.Time `json:"asOf"`
	Error           string    `json:"error,omitempty"`
}

// DataProviders supply the read-model the handler serves. Each is optional; a nil
// provider yields an empty payload rather than a crash.
type DataProviders struct {
	Account        func(ctx context.Context) (AccountView, error)
	Positions      func(ctx context.Context) ([]map[string]any, error)
	Orders         func(ctx context.Context) ([]map[string]any, error)
	Roster         func(ctx context.Context) ([]StrategyEligibility, error)
	Reconciliation func(ctx context.Context) (ReconciliationView, error)
	// ClosedPositions returns positions this engine opened and has since closed
	// (SL/TP/expiry), with their realised outcome.
	ClosedPositions func(ctx context.Context) ([]map[string]any, error)
	// DailyPnl aggregates closed positions by IST day: capital deployed, realised
	// PnL, ROI on that capital, trade count and win rate.
	DailyPnl func(ctx context.Context) ([]map[string]any, error)
	// AllowList returns the current live-enabled strategy names; SetAllowList
	// replaces it. Both optional; when nil the strategy toggle is unavailable.
	AllowList    func() []string
	SetAllowList func(names []string) error
	// ClearHistory wipes the CLOSED/FAILED trade record and returns the number of
	// rows dropped. Open positions must survive it. Optional; when nil the
	// action reports itself unavailable rather than silently succeeding.
	ClearHistory func() int
}

// Authorizer returns the acting principal and whether the request may perform a
// mutation (arm/disarm/close-all). Reads do not require mutation authority.
type Authorizer func(r *http.Request) (actor string, allowed bool)

// Handler serves the Live Engine HTTP surface. Mutations are POST-only and
// require the Authorizer to allow them; reads are GET.
type Handler struct {
	ctrl  *Controller
	data  DataProviders
	authz Authorizer
}

// NewHandler builds the handler. authz must be non-nil for mutations to be
// reachable; a nil authz denies every mutation (fail closed).
func NewHandler(ctrl *Controller, data DataProviders, authz Authorizer) *Handler {
	return &Handler{ctrl: ctrl, data: data, authz: authz}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ServeHTTP routes /api/live-engine/<action>. It is mounted by main.go.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Path
	if i := strings.LastIndex(action, "/api/live-engine/"); i >= 0 {
		action = action[i+len("/api/live-engine/"):]
	}
	action = strings.Trim(action, "/")

	switch action {
	case "state":
		h.requireGet(w, r, func() { writeJSON(w, http.StatusOK, h.ctrl.Snapshot()) })
	case "audit":
		h.requireGet(w, r, func() { writeJSON(w, http.StatusOK, map[string]any{"entries": h.ctrl.Audit()}) })
	case "account":
		h.requireGet(w, r, func() { h.serveAccount(w, r) })
	case "positions":
		h.requireGet(w, r, func() { h.serveList(w, r, h.data.Positions) })
	case "closed-positions":
		h.requireGet(w, r, func() { h.serveList(w, r, h.data.ClosedPositions) })
	case "daily-pnl":
		h.requireGet(w, r, func() { h.serveList(w, r, h.data.DailyPnl) })
	case "orders":
		h.requireGet(w, r, func() { h.serveList(w, r, h.data.Orders) })
	case "roster":
		h.requireGet(w, r, func() { h.serveRoster(w, r) })
	case "reconciliation":
		h.requireGet(w, r, func() { h.serveReconciliation(w, r) })
	case "strategy":
		h.serveStrategyToggle(w, r)
	case "kill-switch":
		h.serveKillSwitchToggle(w, r)
	case "arm":
		h.serveArm(w, r)
	case "disarm":
		h.serveDisarm(w, r)
	case "close-all":
		h.serveCloseAll(w, r)
	case "clear-history":
		h.serveClearHistory(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown live-engine action: " + action})
	}
}

func (h *Handler) requireGet(w http.ResponseWriter, r *http.Request, fn func()) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	fn()
}

// authorizeMutation enforces POST + mutation authority. Returns the actor.
func (h *Handler) authorizeMutation(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return "", false
	}
	if h.authz == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "mutations not authorized"})
		return "", false
	}
	actor, ok := h.authz(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not authorized to change live-engine state"})
		return "", false
	}
	return actor, true
}

// serveStrategyToggle enables/disables one strategy in the live allow-list.
// POST { "strategy": "<name>", "enabled": true|false }. Authorized mutation.
func (h *Handler) serveStrategyToggle(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.authorizeMutation(w, r); !ok {
		return
	}
	if h.data.AllowList == nil || h.data.SetAllowList == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "strategy allow-list not configurable on this deployment"})
		return
	}
	var body struct {
		Strategy string `json:"strategy"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Strategy == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "need {strategy, enabled}"})
		return
	}
	current := h.data.AllowList()
	next := make([]string, 0, len(current)+1)
	seen := false
	for _, n := range current {
		if n == body.Strategy {
			seen = true
			if !body.Enabled {
				continue // drop it
			}
		}
		next = append(next, n)
	}
	if body.Enabled && !seen {
		next = append(next, body.Strategy)
	}
	if err := h.data.SetAllowList(next); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "allowList": next})
}

// serveKillSwitchToggle halts or resumes trading via the institutional kill
// switch. POST { "active": true|false, "reason": "..." }. Authorized mutation.
// Halting is always permitted; it also disarms the Live Engine.
func (h *Handler) serveKillSwitchToggle(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorizeMutation(w, r)
	if !ok {
		return
	}
	var body struct {
		Active bool   `json:"active"`
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "need {active}"})
		return
	}
	if err := h.ctrl.SetKillSwitch(r.Context(), body.Active, actor, body.Reason); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "state": h.ctrl.Snapshot()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": h.ctrl.Snapshot()})
}

func (h *Handler) serveArm(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorizeMutation(w, r)
	if !ok {
		return
	}
	var body struct {
		Confirmation string `json:"confirmation"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if err := h.ctrl.Arm(actor, body.Confirmation); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error(), "state": h.ctrl.Snapshot()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": h.ctrl.Snapshot()})
}

func (h *Handler) serveDisarm(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorizeMutation(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Reason == "" {
		body.Reason = "manual disarm"
	}
	h.ctrl.Disarm(actor, body.Reason)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "state": h.ctrl.Snapshot()})
}

// serveClearHistory wipes the closed-trade record, keeping open positions.
//
// Behind the same mutation authorisation as arming: it cannot lose money, but it
// destroys the record the promotion gate reads, and an accidental GET-triggered
// clear would be unrecoverable from the UI.
func (h *Handler) serveClearHistory(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorizeMutation(w, r)
	if !ok {
		return
	}
	if h.data.ClearHistory == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "clear-history not wired on this engine"})
		return
	}
	n := h.data.ClearHistory()
	h.ctrl.RecordClearHistory(actor, fmt.Sprintf("cleared=%d closed/failed rows; open positions untouched", n))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cleared": n})
}

func (h *Handler) serveCloseAll(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.authorizeMutation(w, r)
	if !ok {
		return
	}
	result, err := h.ctrl.CloseAll(r.Context(), actor)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": result, "state": h.ctrl.Snapshot()})
}

func (h *Handler) serveAccount(w http.ResponseWriter, r *http.Request) {
	if h.data.Account == nil {
		writeJSON(w, http.StatusOK, AccountView{CeilingUSD: MaxTradableUSD, Source: "unavailable", Stale: true})
		return
	}
	acct, err := h.data.Account(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, AccountView{CeilingUSD: MaxTradableUSD, Source: "error: " + err.Error(), Stale: true})
		return
	}
	// Apply the ceiling server-side regardless of what the provider reported.
	acct.CeilingUSD = MaxTradableUSD
	acct.TradableUSD = h.ctrl.TradableEquityUSD(acct.EquityUSD)
	writeJSON(w, http.StatusOK, acct)
}

func (h *Handler) serveList(w http.ResponseWriter, r *http.Request, provider func(context.Context) ([]map[string]any, error)) {
	if provider == nil {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	items, err := provider(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error(), "items": []any{}})
		return
	}
	if items == nil {
		items = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) serveRoster(w http.ResponseWriter, r *http.Request) {
	if h.data.Roster == nil {
		writeJSON(w, http.StatusOK, []StrategyEligibility{})
		return
	}
	items, err := h.data.Roster(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error(), "items": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) serveReconciliation(w http.ResponseWriter, r *http.Request) {
	if h.data.Reconciliation == nil {
		writeJSON(w, http.StatusOK, ReconciliationView{Matched: true, AsOf: h.ctrl.now()})
		return
	}
	view, err := h.data.Reconciliation(r.Context())
	if err != nil {
		view.Error = err.Error()
	}
	writeJSON(w, http.StatusOK, view)
}

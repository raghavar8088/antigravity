package execution

// PaperOMSHandler exposes the PaperOMS over HTTP so the Next.js client can
// delegate execution to the engine rather than running state machines in the
// browser (Epic 1 — canonical execution and state determinism).
//
// Routes (register in main.go):
//   GET  /paper/state              → OMSStateSnapshot (equity, positions, recent trades)
//   POST /paper/open               → open a paper position (admin override only)
//   POST /paper/close/{id}         → manually close a position (admin override only)
//   POST /paper/tick               → feed a mark-price tick (admin override only)
//   POST /paper/reset              → clear all state (admin override only)
//
// Mutation routes are disabled unless the caller supplies header
// X-Paper-Oms-Admin-Override matching env PAPER_OMS_ADMIN_OVERRIDE.
// The autonomous Orchestrator tick loop is the sole production execution path.

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

const paperOMSAdminOverrideHeader = "X-Paper-Oms-Admin-Override"

// PaperOMSHandler binds HTTP endpoints to a PaperOMS instance.
type PaperOMSHandler struct {
	OMS    *PaperOMS
	Symbol string // default symbol (BTC) for /paper/state without query param
}

func (h *PaperOMSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip any prefix the mux left (e.g. /paper/); work only on the tail.
	path := strings.TrimPrefix(r.URL.Path, "/paper")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodGet && (path == "state" || path == ""):
		h.handleState(w, r)
	case r.Method == http.MethodPost && path == "open":
		h.handleOpen(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(path, "close/"):
		id := strings.TrimPrefix(path, "close/")
		h.handleClose(w, r, id)
	case r.Method == http.MethodPost && path == "tick":
		h.handleTick(w, r)
	case r.Method == http.MethodPost && path == "reset":
		h.handleReset(w, r)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

func paperOMSMutationAllowed(r *http.Request) bool {
	expected := strings.TrimSpace(os.Getenv("PAPER_OMS_ADMIN_OVERRIDE"))
	if expected == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get(paperOMSAdminOverrideHeader))
	return got != "" && got == expected
}

func (h *PaperOMSHandler) requireMutationOverride(w http.ResponseWriter, r *http.Request) bool {
	if paperOMSMutationAllowed(r) {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{
		"error": "paper OMS mutations disabled — autonomous orchestrator is execution authority; set PAPER_OMS_ADMIN_OVERRIDE and send X-Paper-Oms-Admin-Override for emergency local override only",
	})
	return false
}

// ── GET /paper/state ──────────────────────────────────────────────────────────

func (h *PaperOMSHandler) handleState(w http.ResponseWriter, r *http.Request) {
	sym := r.URL.Query().Get("symbol")
	if sym == "" {
		sym = h.Symbol
	}
	snap := h.OMS.State(sym)
	writeJSON(w, http.StatusOK, snap)
}

// ── POST /paper/open ─────────────────────────────────────────────────────────

type openReqBody struct {
	Symbol         string  `json:"symbol"`
	Side           string  `json:"side"`
	EntryPrice     float64 `json:"entryPrice"`
	Notional       float64 `json:"notional"`
	Leverage       float64 `json:"leverage"`
	SLPct          float64 `json:"slPct"`
	TPPct          float64 `json:"tpPct"`
	HoldMinutes    float64 `json:"holdMinutes"`
	StrategyID     int     `json:"strategyId"`
	StrategyName   string  `json:"strategyName"`
	TemplateFamily string  `json:"templateFamily"`
	ModuleKey      string  `json:"moduleKey"`
	SlippageBps    float64 `json:"slippageBps"`
}

func (h *PaperOMSHandler) handleOpen(w http.ResponseWriter, r *http.Request) {
	if !h.requireMutationOverride(w, r) {
		return
	}
	var body openReqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Symbol == "" {
		body.Symbol = h.Symbol
	}

	id, err := h.OMS.OpenPosition(OpenPositionRequest{
		Symbol:         body.Symbol,
		Side:           body.Side,
		EntryPrice:     body.EntryPrice,
		Notional:       body.Notional,
		Leverage:       body.Leverage,
		SLPct:          body.SLPct,
		TPPct:          body.TPPct,
		HoldMinutes:    body.HoldMinutes,
		StrategyID:     body.StrategyID,
		StrategyName:   body.StrategyName,
		TemplateFamily: body.TemplateFamily,
		ModuleKey:      body.ModuleKey,
		SlippageBps:    body.SlippageBps,
	}, time.Now().UTC())

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "id": id})
}

// ── POST /paper/close/{id} ────────────────────────────────────────────────────

func (h *PaperOMSHandler) handleClose(w http.ResponseWriter, r *http.Request, id string) {
	if !h.requireMutationOverride(w, r) {
		return
	}
	if id == "" {
		http.Error(w, "missing position id", http.StatusBadRequest)
		return
	}
	trade, err := h.OMS.ClosePosition(id, time.Now().UTC())
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "trade": trade})
}

// ── POST /paper/tick ──────────────────────────────────────────────────────────

type tickReqBody struct {
	Symbol    string  `json:"symbol"`
	MarkPrice float64 `json:"markPrice"`
	// Optional ISO timestamp; defaults to server time
	NowUTC string `json:"nowUTC"`
}

func (h *PaperOMSHandler) handleTick(w http.ResponseWriter, r *http.Request) {
	if !h.requireMutationOverride(w, r) {
		return
	}
	var body tickReqBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Symbol == "" {
		body.Symbol = h.Symbol
	}

	nowUTC := time.Now().UTC()
	if body.NowUTC != "" {
		if parsed, err := time.Parse(time.RFC3339, body.NowUTC); err == nil {
			nowUTC = parsed
		}
	}

	closed := h.OMS.Tick(OMSTick{
		Symbol:    body.Symbol,
		MarkPrice: body.MarkPrice,
		NowUTC:    nowUTC,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":     true,
		"closed": closed,
		"count":  len(closed),
	})
}

// ── POST /paper/reset ─────────────────────────────────────────────────────────

func (h *PaperOMSHandler) handleReset(w http.ResponseWriter, r *http.Request) {
	if !h.requireMutationOverride(w, r) {
		return
	}
	h.OMS.Reset()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// ── helper ────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

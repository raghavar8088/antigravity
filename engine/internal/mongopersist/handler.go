package mongopersist

// REST handler for Phase 30 observability, recovery, and health endpoints.
//
// Mount with:
//
//	mux.Handle("/phase30/", http.StripPrefix("/phase30", mongopersist.NewHandler(client)))

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler is the HTTP handler for all Phase 30 REST endpoints.
type Handler struct{ c *Client }

// NewHandler returns an http.Handler that serves Phase 30 endpoints.
func NewHandler(c *Client) http.Handler {
	h := &Handler{c: c}
	mux := http.NewServeMux()

	// Observability
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/stats", h.stats)

	// Phase 30A — certification evidence recovery
	mux.HandleFunc("/phase/24", h.phaseN(24))
	mux.HandleFunc("/phase/25", h.phaseN(25))
	mux.HandleFunc("/phase/26", h.phaseN(26))
	mux.HandleFunc("/phase/27", h.phaseN(27))
	mux.HandleFunc("/phase/28", h.phaseN(28))
	mux.HandleFunc("/phase/29", h.phaseN(29))
	mux.HandleFunc("/phase/all", h.phaseAll)

	// Phase 30B — engine live state
	mux.HandleFunc("/positions/open", h.openPositions)
	mux.HandleFunc("/risk", h.risk)
	mux.HandleFunc("/health/strategies", h.strategyHealth)
	mux.HandleFunc("/allocations", h.allocations)
	mux.HandleFunc("/execintel/summary", h.execIntelSummary)
	mux.HandleFunc("/killswitch", h.killSwitchState)
	mux.HandleFunc("/certifications", h.certifications)

	return mux
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// GET /phase30/health — MongoDB ping + collection counts.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	mongoOK := h.c.PingMongo(ctx) == nil
	stats, _ := h.c.CollectionStats(ctx)
	writeJSON(w, map[string]interface{}{
		"mongo_reachable": mongoOK,
		"collections":     stats,
	})
}

// GET /phase30/stats — document counts for every Phase 30 collection.
func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.c.CollectionStats(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

// phaseN returns a handler for GET /phase30/phase/{N}.
func (h *Handler) phaseN(n int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var (
			doc interface{}
			err error
		)
		switch n {
		case 24:
			doc, err = h.c.LoadLatestPhase24(ctx)
		case 25:
			doc, err = h.c.LoadLatestPhase25(ctx)
		case 26:
			doc, err = h.c.LoadLatestPhase26(ctx)
		case 27:
			doc, err = h.c.LoadLatestPhase27(ctx)
		case 28:
			doc, err = h.c.LoadLatestPhase28(ctx)
		case 29:
			doc, err = h.c.LoadLatestPhase29(ctx)
		}
		if err != nil {
			writeErr(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if doc == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "no phase " + strconv.Itoa(n) + " result stored yet",
			})
			return
		}
		writeJSON(w, doc)
	}
}

// GET /phase30/phase/all — all phase results in one response.
func (h *Handler) phaseAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	all, err := h.c.LoadAllPhaseResults(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, all)
}

// GET /phase30/positions/open
func (h *Handler) openPositions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docs, err := h.c.LoadOpenPositions(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, docs)
}

// GET /phase30/risk?source=engine-live
func (h *Handler) risk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	source := r.URL.Query().Get("source")
	if source == "" {
		source = "engine-live"
	}
	doc, err := h.c.LoadRisk(ctx, source)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, doc)
}

// GET /phase30/health/strategies
func (h *Handler) strategyHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docs, err := h.c.LoadAllHealth(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, docs)
}

// GET /phase30/allocations
func (h *Handler) allocations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docs, err := h.c.LoadAllAllocations(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, docs)
}

// GET /phase30/execintel/summary
func (h *Handler) execIntelSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	docs, err := h.c.LoadExecIntelSummary(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, docs)
}

// GET /phase30/killswitch
func (h *Handler) killSwitchState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	events, err := h.c.LoadKillSwitchState(ctx)
	if err != nil {
		writeErr(w, err.Error(), http.StatusInternalServerError)
		return
	}
	active, _ := h.c.IsKillSwitchActive(ctx)
	writeJSON(w, map[string]interface{}{
		"active": active,
		"events": events,
	})
}

// GET /phase30/certifications?tier=DEPLOY
func (h *Handler) certifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tier := r.URL.Query().Get("tier")
	var docs []interface{}
	if tier != "" {
		raw, err := h.c.LoadCertificationsByTier(ctx, tier)
		if err != nil {
			writeErr(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, d := range raw {
			docs = append(docs, d)
		}
	} else {
		raw, err := h.c.LoadAllCertifications(ctx)
		if err != nil {
			writeErr(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, d := range raw {
			docs = append(docs, d)
		}
	}
	writeJSON(w, docs)
}

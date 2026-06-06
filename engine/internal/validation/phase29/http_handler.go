package phase29

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Handler29 serves Phase 29 REST endpoints.
type Handler29 struct {
	mu     sync.RWMutex
	result *Phase29Result
	outDir string
}

// NewHandler29 creates a handler. outDir is where reports are written.
func NewHandler29(outDir string) *Handler29 {
	return &Handler29{outDir: outDir}
}

// RegisterRoutes registers all Phase 29 REST endpoints on mux.
func (h *Handler29) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/phase29/run", h.handleRun)
	mux.HandleFunc("/api/phase29/result", h.handleResult)
	mux.HandleFunc("/api/phase29/verdict", h.handleVerdict)
	mux.HandleFunc("/api/phase29/championship", h.handleChampionship)
	mux.HandleFunc("/api/phase29/alpha-championship", h.handleAlphaChamp)
	mux.HandleFunc("/api/phase29/retirement", h.handleRetirement)
	mux.HandleFunc("/api/phase29/portfolios", h.handlePortfolios)
	mux.HandleFunc("/api/phase29/capital-deployment", h.handleCapitalDeployment)
	mux.HandleFunc("/api/phase29/edge-retention", h.handleEdgeRetention)
	mux.HandleFunc("/api/phase29/execution-quality", h.handleExecQuality)
	mux.HandleFunc("/api/phase29/institutional-cert", h.handleInstitutionalCert)
	mux.HandleFunc("/api/phase29/reports", h.handleReportsList)
}

func (h *Handler29) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	jsonErr(w, map[string]string{
		"status":  "error",
		"message": "Phase 29 requires upstream Phase 24–28 results. Call RunFromPhases() programmatically or supply pre-computed results via the bundle endpoint.",
	})
}

func (h *Handler29) handleResult(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	res := h.result
	h.mu.RUnlock()
	if res == nil {
		jsonErr(w, map[string]string{"status": "INSUFFICIENT_EVIDENCE", "message": "Phase 29 has not been run yet"})
		return
	}
	jsonOK(w, res)
}

func (h *Handler29) handleVerdict(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.FinalVerdict) })
}
func (h *Handler29) handleChampionship(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.Championship) })
}
func (h *Handler29) handleAlphaChamp(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.AlphaChamp) })
}
func (h *Handler29) handleRetirement(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.Retirement) })
}
func (h *Handler29) handlePortfolios(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.Portfolios) })
}
func (h *Handler29) handleCapitalDeployment(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.CapitalDeployment) })
}
func (h *Handler29) handleEdgeRetention(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.EdgeRetention) })
}
func (h *Handler29) handleExecQuality(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.ExecutionQuality) })
}
func (h *Handler29) handleInstitutionalCert(w http.ResponseWriter, r *http.Request) {
	h.withResult(w, func(res *Phase29Result) { jsonOK(w, res.InstitutionalCert) })
}

func (h *Handler29) handleReportsList(w http.ResponseWriter, r *http.Request) {
	if h.outDir == "" {
		jsonErr(w, map[string]string{"message": "no output directory configured"})
		return
	}
	files, _ := filepath.Glob(filepath.Join(h.outDir, "*.md"))
	names := make([]string, 0, len(files))
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			names = append(names, filepath.Base(f))
		}
	}
	jsonOK(w, map[string]interface{}{"reports": names, "directory": h.outDir})
}

func (h *Handler29) withResult(w http.ResponseWriter, fn func(*Phase29Result)) {
	h.mu.RLock()
	res := h.result
	h.mu.RUnlock()
	if res == nil {
		jsonErr(w, map[string]string{"status": "INSUFFICIENT_EVIDENCE", "message": "Phase 29 has not been run yet"})
		return
	}
	fn(res)
}

// SetResult stores a completed Phase 29 result and writes all reports.
func (h *Handler29) SetResult(res *Phase29Result) error {
	h.mu.Lock()
	h.result = res
	h.mu.Unlock()
	if h.outDir != "" {
		return WriteAllReports(res, h.outDir)
	}
	return nil
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(v)
}

package phase27

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handler27 serves Phase 27 REST endpoints.
type Handler27 struct {
	result  *Phase27Result
	reports map[string]string
}

// NewHandler27 creates a Handler27.
func NewHandler27(r *Phase27Result) *Handler27 {
	return &Handler27{result: r, reports: BuildReports(r)}
}

// RegisterRoutes wires all Phase 27 endpoints to the mux.
func RegisterRoutes(mux *http.ServeMux, result *Phase27Result) {
	h := NewHandler27(result)
	mux.HandleFunc("/api/phase27/", h.ServeHTTP)
}

func (h *Handler27) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/phase27/")
	path = strings.TrimSuffix(path, "/")
	w.Header().Set("Content-Type", "application/json")

	switch {
	case path == "attribution":
		jsonOK(w, h.result.AttributionRecords)
	case path == "alpha-performance":
		jsonOK(w, h.result.AlphaPerformance)
	case path == "pareto":
		jsonOK(w, h.result.ParetoAnalysis)
	case path == "championship":
		jsonOK(w, h.result.Championship)
	case path == "overlap":
		jsonOK(w, h.result.OverlapAnalysis)
	case path == "redundant":
		jsonOK(w, h.result.RedundantFamilies)
	case strings.HasPrefix(path, "report/"):
		name := strings.TrimPrefix(path, "report/")
		content, ok := h.reports[name]
		if !ok {
			http.Error(w, `{"error":"report not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/markdown")
		fmt.Fprint(w, content)
	case path == "metrics/prometheus":
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "phase27_total_trades_attributed %d\n", len(h.result.AttributionRecords))
		fmt.Fprintf(w, "phase27_alpha_families_analyzed %d\n", len(h.result.AlphaPerformance))
		fmt.Fprintf(w, "phase27_total_platform_pnl_usd %.2f\n", h.result.TotalPlatformPnL)
		fmt.Fprintf(w, "phase27_redundant_families %d\n", len(h.result.RedundantFamilies))
		fmt.Fprintf(w, "phase27_insufficient_evidence %d\n", len(h.result.InsufficientEvidence))
		fmt.Fprintf(w, "phase27_pareto_top1_pct %.2f\n", h.result.ParetoAnalysis.Top1AlphaPct)
		fmt.Fprintf(w, "phase27_pareto_top3_pct %.2f\n", h.result.ParetoAnalysis.Top3AlphaPct)
		fmt.Fprintf(w, "phase27_pareto_top5_pct %.2f\n", h.result.ParetoAnalysis.Top5AlphaPct)
	default:
		http.Error(w, `{"error":"unknown endpoint"}`, http.StatusNotFound)
	}
}

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, `{"error":"encoding failed"}`, http.StatusInternalServerError)
	}
}

package phase26

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handler26 serves Phase 26 REST endpoints.
type Handler26 struct {
	result  *Phase26Result
	reports map[string]string
}

func NewHandler26(r *Phase26Result) *Handler26 {
	return &Handler26{
		result:  r,
		reports: BuildReports(r),
	}
}

// RegisterRoutes wires all Phase 26 endpoints to the given mux.
func RegisterRoutes(mux *http.ServeMux, result *Phase26Result) {
	h := NewHandler26(result)
	mux.HandleFunc("/api/phase26/", h.ServeHTTP)
}

func (h *Handler26) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/phase26/")
	path = strings.TrimSuffix(path, "/")
	w.Header().Set("Content-Type", "application/json")

	switch {
	case path == "eligibility":
		jsonOK(w, h.result.EligibleStrategies)
	case path == "tier1":
		jsonOK(w, h.result.Tier1Strategies)
	case path == "tier2":
		jsonOK(w, h.result.Tier2Strategies)
	case path == "tier2/milestones":
		jsonOK(w, h.result.Tier2Milestones)
	case path == "edge-retention":
		jsonOK(w, h.result.EdgeRetention)
	case path == "drift":
		jsonOK(w, h.result.DriftRecords)
	case path == "certification":
		all := append(h.result.Tier1Strategies, h.result.Tier2Strategies...)
		jsonOK(w, all)
	case path == "certified":
		jsonOK(w, h.result.InstitutionalCertified)
	case path == "retired":
		jsonOK(w, h.result.Retired)
	case path == "verdict":
		jsonOK(w, h.result.FinalVerdict)
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
		fmt.Fprintf(w, "phase26_total_eligible %d\n", h.result.TotalEligible)
		fmt.Fprintf(w, "phase26_total_certified %d\n", h.result.TotalCertified)
		fmt.Fprintf(w, "phase26_tier1_count %d\n", len(h.result.Tier1Strategies))
		fmt.Fprintf(w, "phase26_tier2_count %d\n", len(h.result.Tier2Strategies))
		fmt.Fprintf(w, "phase26_retired_count %d\n", len(h.result.Retired))
		fmt.Fprintf(w, "phase26_platform_pf %.4f\n", h.result.FinalVerdict.PlatformPF)
		fmt.Fprintf(w, "phase26_platform_sharpe %.4f\n", h.result.FinalVerdict.PlatformSharpe)
		fmt.Fprintf(w, "phase26_platform_net_pnl_usd %.2f\n", h.result.FinalVerdict.PlatformNetPnL)
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

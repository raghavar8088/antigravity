package phase28

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handler28 serves Phase 28 REST endpoints.
type Handler28 struct {
	result  *Phase28Result
	reports map[string]string
}

// NewHandler28 creates a Handler28.
func NewHandler28(r *Phase28Result) *Handler28 {
	return &Handler28{result: r, reports: BuildReports(r)}
}

// RegisterRoutes wires all Phase 28 endpoints to the mux.
func RegisterRoutes(mux *http.ServeMux, result *Phase28Result) {
	h := NewHandler28(result)
	mux.HandleFunc("/api/phase28/", h.ServeHTTP)
}

func (h *Handler28) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/phase28/")
	path = strings.TrimSuffix(path, "/")
	w.Header().Set("Content-Type", "application/json")

	switch {
	case path == "correlation":
		jsonOK(w, h.result.CorrelationMatrix)
	case path == "portfolio-a":
		jsonOK(w, h.result.PortfolioA)
	case path == "portfolio-b":
		jsonOK(w, h.result.PortfolioB)
	case path == "portfolio-c":
		jsonOK(w, h.result.PortfolioC)
	case path == "portfolio-d":
		jsonOK(w, h.result.PortfolioD)
	case path == "allocations":
		jsonOK(w, h.result.StrategyAllocations)
	case strings.HasPrefix(path, "capital/"):
		amount := strings.TrimPrefix(path, "capital/")
		for _, plan := range h.result.CapitalPlans {
			if fmt.Sprintf("%.0f", plan.Capital) == amount {
				jsonOK(w, plan)
				return
			}
		}
		http.Error(w, `{"error":"capital plan not found"}`, http.StatusNotFound)
	case path == "failure-analysis":
		jsonOK(w, h.result.FailureAnalysis)
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
		fmt.Fprintf(w, "phase28_strategies_in_portfolio_a %d\n", len(h.result.PortfolioA.Weights))
		fmt.Fprintf(w, "phase28_strategies_in_portfolio_d %d\n", len(h.result.PortfolioD.Weights))
		fmt.Fprintf(w, "phase28_portfolio_a_sharpe %.4f\n", h.result.PortfolioA.ExpectedSharpe)
		fmt.Fprintf(w, "phase28_portfolio_d_sharpe %.4f\n", h.result.PortfolioD.ExpectedSharpe)
		fmt.Fprintf(w, "phase28_portfolio_d_max_dd_pct %.4f\n", h.result.PortfolioD.ExpectedMaxDD)
		fmt.Fprintf(w, "phase28_portfolio_d_ror %.6f\n", h.result.PortfolioD.ExpectedRoR)
		fmt.Fprintf(w, "phase28_institutional_justified %d\n", boolToInt(h.result.FinalVerdict.Q15_InstitutionalJustified))
		fmt.Fprintf(w, "phase28_scale_capital_justified %d\n", boolToInt(h.result.FinalVerdict.Q16_ScaleCapitalJustified))
		fmt.Fprintf(w, "phase28_correlation_matrix_strategies %d\n", len(h.result.CorrelationMatrix.Strategies))
		fmt.Fprintf(w, "phase28_total_allocation_plans %d\n", len(h.result.CapitalPlans))
		fmt.Fprintf(w, "phase28_failure_candidates %d\n", len(h.result.FailureAnalysis))
		fmt.Fprintf(w, "phase28_strategy_allocations %d\n", len(h.result.StrategyAllocations))
		fmt.Fprintf(w, "phase28_portfolio_avg_corr %.4f\n", h.result.PortfolioD.AverageCorrelation)
		fmt.Fprintf(w, "phase28_portfolio_divers_score %.2f\n", h.result.PortfolioD.DiversScore)
		fmt.Fprintf(w, "phase28_expected_annual_return %.4f\n", h.result.FinalVerdict.Q11_ExpectedAnnualReturn)
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

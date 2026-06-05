package phase25

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler25 holds a completed Phase25Result and serves 15 REST endpoints.
type Handler25 struct {
	result Phase25Result
}

func NewHandler25(r Phase25Result) *Handler25 {
	return &Handler25{result: r}
}

// ServeHTTP routes /api/phase25/* requests.
func (h *Handler25) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/phase25/")
	path = strings.TrimSuffix(path, "/")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	switch path {
	case "eligibility":
		jsonOK(w, h.result.Eligibility)
	case "live-trades":
		jsonOK(w, h.result.LiveTrades)
	case "live-metrics":
		jsonOK(w, h.result.LiveMetrics)
	case "edge-drift":
		jsonOK(w, h.result.EdgeDrift)
	case "edge-drift/summary":
		jsonOK(w, DriftSummary(h.result.EdgeDrift))
	case "alpha-survival":
		jsonOK(w, h.result.AlphaSurvival)
	case "alpha-survival/champion":
		if len(h.result.AlphaSurvival) > 0 {
			jsonOK(w, h.result.AlphaSurvival[0])
		} else {
			http.Error(w, `{"error":"no alpha data"}`, http.StatusNotFound)
		}
	case "capital-escalation":
		jsonOK(w, h.result.CapEscalation)
	case "capital-escalation/total":
		jsonOK(w, map[string]float64{
			"total_capital_usd": TotalCapitalDeployed(h.result.CapEscalation),
		})
	case "auto-demotion":
		jsonOK(w, h.result.Demotions)
	case "portfolio-heat":
		jsonOK(w, h.result.PortfolioHeat)
	case "execution-quality":
		jsonOK(w, h.result.ExecQuality)
	case "monthly/strategy":
		jsonOK(w, h.result.MonthlyStrategies)
	case "monthly/alpha":
		jsonOK(w, h.result.MonthlyAlphas)
	case "monthly/portfolio":
		jsonOK(w, h.result.MonthlyPortfolios)
	case "monthly/capital":
		jsonOK(w, h.result.MonthlyCapital)
	case "final-verdict":
		jsonOK(w, h.result.Verdict)
	case "summary":
		jsonOK(w, map[string]interface{}{
			"generated_at":             h.result.GeneratedAt,
			"live_days":                h.result.Config.LiveDays,
			"total_eligible":           h.result.TotalEligible,
			"total_live_trades":        len(h.result.LiveTrades),
			"total_candles":            h.result.TotalCandles,
			"total_live_certified":     h.result.TotalLiveCertified,
			"total_demoted":            h.result.TotalDemoted,
			"total_retired":            h.result.TotalRetired,
			"platform_live_pf":         h.result.PlatformLivePF,
			"platform_live_sharpe":     h.result.PlatformLiveSharpe,
			"platform_live_net_pnl":    h.result.PlatformLiveNetPnLUSD,
			"portfolio_heat":           h.result.PortfolioHeat.TotalHeat,
			"exec_quality_score":       h.result.ExecQuality.QualityScore,
			"overall_verdict":          h.result.Verdict.OverallVerdict,
			"live_deployment_approved": h.result.Verdict.Q13_LiveDeploymentJustified,
			"institutional_approved":   h.result.Verdict.Q14_InstitutionalCapJustified,
		})
	default:
		http.Error(w, `{"error":"unknown endpoint","available_endpoints":[`+endpointList+`]}`, http.StatusNotFound)
	}
}

const endpointList = `"eligibility","live-trades","live-metrics","edge-drift","edge-drift/summary",` +
	`"alpha-survival","alpha-survival/champion","capital-escalation","capital-escalation/total",` +
	`"auto-demotion","portfolio-heat","execution-quality",` +
	`"monthly/strategy","monthly/alpha","monthly/portfolio","monthly/capital",` +
	`"final-verdict","summary"`

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

package phase24

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler24 returns an http.Handler that serves all Phase 24 REST endpoints.
// All results are pre-computed and served from the in-memory result.
//
// Endpoints:
//
//	GET /api/phase24/data-certification
//	GET /api/phase24/strategy-evidence
//	GET /api/phase24/alpha-championship
//	GET /api/phase24/walk-forward
//	GET /api/phase24/monte-carlo
//	GET /api/phase24/regime-edge
//	GET /api/phase24/retirement
//	GET /api/phase24/capital-certification
//	GET /api/phase24/top3-portfolio
//	GET /api/phase24/top5-portfolio
//	GET /api/phase24/top10-portfolio
//	GET /api/phase24/leaderboard
//	GET /api/phase24/final-verdict
//	GET /api/phase24/summary
//	GET /api/phase24/health
func Handler24(r Phase24Result) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/phase24/data-certification", jsonHandler(r.DataCert))
	mux.HandleFunc("/api/phase24/strategy-evidence", func(w http.ResponseWriter, req *http.Request) {
		jsonWrite(w, r.Metrics)
	})
	mux.HandleFunc("/api/phase24/alpha-championship", jsonHandler(r.AlphaChampionship))
	mux.HandleFunc("/api/phase24/walk-forward", jsonHandler(r.WalkForward))
	mux.HandleFunc("/api/phase24/monte-carlo", func(w http.ResponseWriter, req *http.Request) {
		jsonWrite(w, r.MonteCarlo)
	})
	mux.HandleFunc("/api/phase24/regime-edge", jsonHandler(r.RegimeProfiles))
	mux.HandleFunc("/api/phase24/retirement", jsonHandler(r.Retirement))
	mux.HandleFunc("/api/phase24/capital-certification", jsonHandler(r.CapCerts))
	mux.HandleFunc("/api/phase24/top3-portfolio", jsonHandler(r.Top3Portfolio))
	mux.HandleFunc("/api/phase24/top5-portfolio", jsonHandler(r.Top5Portfolio))
	mux.HandleFunc("/api/phase24/top10-portfolio", jsonHandler(r.Top10Portfolio))
	mux.HandleFunc("/api/phase24/leaderboard", jsonHandler(r.AllRanked))
	mux.HandleFunc("/api/phase24/final-verdict", jsonHandler(r.Verdict))
	mux.HandleFunc("/api/phase24/summary", jsonHandler(summaryOf(r)))
	mux.HandleFunc("/api/phase24/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","phase":"24"}`))
	})
	return mux
}

type phase24Summary struct {
	GeneratedAt              string  `json:"generatedAt"`
	Symbol                   string  `json:"symbol"`
	TotalTrades              int     `json:"totalTrades"`
	TotalStrategies          int     `json:"totalStrategies"`
	CertifiedStrategies      int     `json:"certifiedStrategies"`
	DeployNowStrategies      int     `json:"deployNowStrategies"`
	RetiredStrategies        int     `json:"retiredStrategies"`
	PlatformPF               float64 `json:"platformPF"`
	PlatformSharpe           float64 `json:"platformSharpe"`
	PlatformNetPnLUSD        float64 `json:"platformNetPnLUSD"`
	DeployApproved           bool    `json:"deployApproved"`
	HasRealEdge              bool    `json:"hasRealEdge"`
	Top5Strategies           []string `json:"top5Strategies"`
	BestAlpha                string  `json:"bestAlpha"`
}

func summaryOf(r Phase24Result) phase24Summary {
	bestAlpha := ""
	if len(r.AlphaChampionship) > 0 {
		bestAlpha = string(r.AlphaChampionship[0].Engine)
	}
	top5 := r.Verdict.Q15_Top5Strategies
	return phase24Summary{
		GeneratedAt:         r.GeneratedAt.String(),
		Symbol:              r.Config.Symbol,
		TotalTrades:         len(r.AllTrades),
		TotalStrategies:     r.TotalStrategiesEvaluated,
		CertifiedStrategies: r.TotalCertified,
		DeployNowStrategies: r.TotalDeployNow,
		RetiredStrategies:   r.TotalRetired,
		PlatformPF:          r.PlatformPF,
		PlatformSharpe:      r.PlatformSharpe,
		PlatformNetPnLUSD:   r.PlatformNetPnLUSD,
		DeployApproved:      r.Verdict.Q17_ApproveDeployment,
		HasRealEdge:         r.Verdict.Q1_HasRealEdge,
		Top5Strategies:      top5,
		BestAlpha:           bestAlpha,
	}
}

func jsonHandler(v interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		jsonWrite(w, v)
	}
}

func jsonWrite(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		if !strings.Contains(err.Error(), "broken pipe") {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

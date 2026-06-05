package phase23c

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Handler23C returns an http.Handler exposing Phase 23C edge discovery results.
func Handler23C(result Phase23CResult) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/phase23c/edge-discovery", jsonHandler(result.AllRanked))
	mux.HandleFunc("/api/phase23c/top20", jsonHandler(result.Top20))
	mux.HandleFunc("/api/phase23c/top10", jsonHandler(result.Top10))
	mux.HandleFunc("/api/phase23c/top5-portfolio", jsonHandler(result.Top5Portfolio))
	mux.HandleFunc("/api/phase23c/alpha-championship", jsonHandler(result.AlphaChampionship))
	mux.HandleFunc("/api/phase23c/eliminated", jsonHandler(result.Eliminated))
	mux.HandleFunc("/api/phase23c/final-verdict", jsonHandler(result.FinalVerdict))
	mux.HandleFunc("/api/phase23c/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":              "ok",
			"generatedAt":         result.GeneratedAt,
			"totalEvaluated":      result.TotalStrategiesEvaluated,
			"totalWithEdge":       result.TotalWithEdge,
			"totalDeployNow":      result.TotalDeployNow,
			"totalRetired":        result.TotalRetired,
			"platformNetPnL":      result.PlatformNetPnLUSD,
			"platformPF":          result.PlatformProfitFactor,
			"platformSharpe":      result.PlatformSharpe,
			"deployCapitalToday":  result.FinalVerdict.Q11_DeployCapitalToday,
		})
	})

	return mux
}

func jsonHandler(v interface{}) http.HandlerFunc {
	var (
		once sync.Once
		data []byte
	)
	return func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() {
			data, _ = json.Marshal(v)
		})
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}
}

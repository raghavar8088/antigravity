package phase23b

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Handler23B returns an http.Handler that exposes Phase 23B validation results
// via a REST API.  Results are computed once and cached.
func Handler23B(result Phase23BResult) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/phase23b/data-audit", jsonHandler(result.DataAudit))
	mux.HandleFunc("/api/phase23b/synthetic-removal", jsonHandler(result.SyntheticRemoval))
	mux.HandleFunc("/api/phase23b/replay-results", jsonHandler(result.StrategyReplays))
	mux.HandleFunc("/api/phase23b/cost-model", jsonHandler(result.CostBreakdowns))
	mux.HandleFunc("/api/phase23b/certified-trades/summary", jsonHandler(map[string]interface{}{
		"totalTrades":       result.TotalTrades,
		"totalStrategies":   result.TotalStrategies,
		"tradesPerStrategy": result.TradesPerStrategy,
		"tradesPerAlpha":    result.TradesPerAlpha,
		"tradesPerRegime":   result.TradesPerRegime,
		"coverageDays":      result.CoverageDays,
	}))
	mux.HandleFunc("/api/phase23b/walk-forward", jsonHandler(result.WalkForward))
	mux.HandleFunc("/api/phase23b/monte-carlo", jsonHandler(result.MonteCarlo))
	mux.HandleFunc("/api/phase23b/regimes", jsonHandler(result.RegimeProfiles))
	mux.HandleFunc("/api/phase23b/capital-certification", jsonHandler(result.CapCertifications))
	mux.HandleFunc("/api/phase23b/metrics", jsonHandler(result.Metrics))
	mux.HandleFunc("/api/phase23b/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":         "ok",
			"generatedAt":    result.GeneratedAt,
			"totalTrades":    result.TotalTrades,
			"totalStrategies": result.TotalStrategies,
			"certified":      result.CertifiedStrategies,
			"retired":        result.RetiredStrategies,
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

// LiveHandler23B runs the full pipeline on every request (for development use).
func LiveHandler23B(cfg ReplayConfig) http.Handler {
	var (
		mu     sync.RWMutex
		cached *Phase23BResult
		lastRun time.Time
		ttl    = 6 * time.Hour
	)

	runPipeline := func() (*Phase23BResult, error) {
		p := NewPipeline23B(cfg)
		result, err := p.Run()
		if err != nil {
			return nil, err
		}
		return &result, nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/phase23b/run", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		if cached != nil && time.Since(lastRun) < ttl {
			data, _ := json.Marshal(cached)
			mu.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
		mu.RUnlock()

		result, err := runPipeline()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mu.Lock()
		cached = result
		lastRun = time.Now()
		mu.Unlock()

		data, _ := json.Marshal(result)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
	return mux
}

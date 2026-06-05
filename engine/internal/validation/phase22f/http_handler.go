package phase22f

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

// Handler creates an http.ServeMux that exposes Phase 22F REST endpoints.
// The provided result is cached on first access.
func Handler(result Phase22FResult) http.Handler {
	mux := http.NewServeMux()
	cache := &resultCache{result: result}

	mux.HandleFunc("/api/phase22f/certification", cache.handleCertification)
	mux.HandleFunc("/api/phase22f/edge-verdict", cache.handleEdgeVerdict)
	mux.HandleFunc("/api/phase22f/top20", cache.handleTop20)
	mux.HandleFunc("/api/phase22f/campaign", cache.handleCampaign)
	mux.HandleFunc("/api/phase22f/alpha-engines", cache.handleAlphaEngines)
	mux.HandleFunc("/api/phase22f/monte-carlo", cache.handleMonteCarlo)
	mux.HandleFunc("/api/phase22f/regimes", cache.handleRegimes)
	mux.HandleFunc("/api/phase22f/portfolios", cache.handlePortfolios)
	mux.HandleFunc("/api/phase22f/capital-allocation", cache.handleCapitalAllocation)
	mux.HandleFunc("/api/phase22f/elimination", cache.handleElimination)
	mux.HandleFunc("/api/phase22f/tiers", cache.handleTiers)
	mux.HandleFunc("/api/phase22f/execution-correlation", cache.handleExecCorrelation)
	mux.HandleFunc("/api/phase22f/health", handleHealth)
	mux.HandleFunc("/metrics", cache.handlePrometheusMetrics)

	return mux
}

type resultCache struct {
	mu     sync.RWMutex
	result Phase22FResult
}

func (c *resultCache) get() Phase22FResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result
}

// Update replaces the cached result (thread-safe).
func (c *resultCache) Update(r Phase22FResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.result = r
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "phase": "22F"})
}

func (c *resultCache) handleCertification(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	writeJSON(w, map[string]interface{}{
		"generated_at":       r.GeneratedAt,
		"total_trades":       r.TotalTrades,
		"total_strategies":   r.TotalStrategies,
		"passed_strategies":  r.PassedStrategies,
		"failed_strategies":  r.FailedStrategies,
		"system_has_edge":    r.EdgeVerdict.SystemHasEdge,
		"confidence":         r.EdgeVerdict.Confidence,
		"data_integrity":     r.DataIntegrity.Passed,
		"portfolio_pf":       r.EdgeVerdict.ExpectedPortfolioPF,
		"portfolio_sharpe":   r.EdgeVerdict.ExpectedSharpe,
		"expected_drawdown":  r.EdgeVerdict.ExpectedDrawdown,
	})
}

func (c *resultCache) handleEdgeVerdict(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().EdgeVerdict)
}

func (c *resultCache) handleTop20(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().Top20)
}

func (c *resultCache) handleCampaign(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().Campaign)
}

func (c *resultCache) handleAlphaEngines(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().AlphaValidation)
}

func (c *resultCache) handleMonteCarlo(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	writeJSON(w, map[string]interface{}{
		"portfolio": r.PortfolioMC,
		"strategies": r.MonteCarlo,
	})
}

func (c *resultCache) handleRegimes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().RegimePerf)
}

func (c *resultCache) handlePortfolios(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().Portfolios)
}

func (c *resultCache) handleCapitalAllocation(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().CapitalAllocation)
}

func (c *resultCache) handleElimination(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().Elimination)
}

func (c *resultCache) handleTiers(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	writeJSON(w, map[string]interface{}{
		"tiers":  r.CertificationTiers,
		"counts": TierCounts22(r.CertificationTiers),
	})
}

func (c *resultCache) handleExecCorrelation(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, c.get().ExecCorrelation)
}

// handlePrometheusMetrics exposes key metrics in Prometheus text format.
func (c *resultCache) handlePrometheusMetrics(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	v := r.EdgeVerdict
	edgeVal := 0.0
	if v.SystemHasEdge {
		edgeVal = 1.0
	}
	intVal := 0.0
	if r.DataIntegrity.Passed {
		intVal = 1.0
	}

	lines := []string{
		`# HELP phase22f_system_has_edge 1 if system has confirmed trading edge`,
		`# TYPE phase22f_system_has_edge gauge`,
		fmtMetric("phase22f_system_has_edge", edgeVal),

		`# HELP phase22f_portfolio_pf Portfolio profit factor`,
		`# TYPE phase22f_portfolio_pf gauge`,
		fmtMetric("phase22f_portfolio_pf", v.ExpectedPortfolioPF),

		`# HELP phase22f_portfolio_sharpe Portfolio Sharpe ratio`,
		`# TYPE phase22f_portfolio_sharpe gauge`,
		fmtMetric("phase22f_portfolio_sharpe", v.ExpectedSharpe),

		`# HELP phase22f_portfolio_drawdown Expected portfolio drawdown pct`,
		`# TYPE phase22f_portfolio_drawdown gauge`,
		fmtMetric("phase22f_portfolio_drawdown", v.ExpectedDrawdown),

		`# HELP phase22f_strategies_passed Count of strategies that passed validation`,
		`# TYPE phase22f_strategies_passed gauge`,
		fmtMetric("phase22f_strategies_passed", float64(v.StrategiesPassed)),

		`# HELP phase22f_strategies_failed Count of strategies that failed validation`,
		`# TYPE phase22f_strategies_failed gauge`,
		fmtMetric("phase22f_strategies_failed", float64(v.StrategiesFailed)),

		`# HELP phase22f_total_trades Total certified trade count`,
		`# TYPE phase22f_total_trades gauge`,
		fmtMetric("phase22f_total_trades", float64(r.TotalTrades)),

		`# HELP phase22f_data_integrity_passed 1 if data integrity certification passed`,
		`# TYPE phase22f_data_integrity_passed gauge`,
		fmtMetric("phase22f_data_integrity_passed", intVal),

		`# HELP phase22f_portfolio_mc_prob_grow Monte Carlo probability of portfolio growth`,
		`# TYPE phase22f_portfolio_mc_prob_grow gauge`,
		fmtMetric("phase22f_portfolio_mc_prob_grow", r.PortfolioMC.ProbabilityGrow),

		`# HELP phase22f_portfolio_mc_prob_ruin Monte Carlo probability of portfolio ruin`,
		`# TYPE phase22f_portfolio_mc_prob_ruin gauge`,
		fmtMetric("phase22f_portfolio_mc_prob_ruin", r.PortfolioMC.ProbabilityRuin),
	}

	for _, line := range lines {
		_, _ = w.Write([]byte(line + "\n"))
	}
}

func fmtMetric(name string, val float64) string {
	return jsonFloat(name+" ", val)
}

func jsonFloat(prefix string, f float64) string {
	return prefix + formatFloat(f)
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 6, 64)
}

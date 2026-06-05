package phase23a

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

// Handler23A creates a ServeMux for Phase 23A REST endpoints.
func Handler23A(result Phase23AResult) http.Handler {
	mux := http.NewServeMux()
	c := &cache23A{result: result}

	mux.HandleFunc("/api/phase23a/certification", c.certification)
	mux.HandleFunc("/api/phase23a/final-verdict", c.finalVerdict)
	mux.HandleFunc("/api/phase23a/top10", c.top10)
	mux.HandleFunc("/api/phase23a/walk-forward", c.walkForward)
	mux.HandleFunc("/api/phase23a/monte-carlo", c.monteCarlo)
	mux.HandleFunc("/api/phase23a/alpha-rankings", c.alphaRankings)
	mux.HandleFunc("/api/phase23a/portfolios", c.portfolios)
	mux.HandleFunc("/api/phase23a/capital-plan", c.capitalPlan)
	mux.HandleFunc("/api/phase23a/elimination", c.elimination)
	mux.HandleFunc("/api/phase23a/edge-certifications", c.edgeCerts)
	mux.HandleFunc("/api/phase23a/regimes", c.regimes)
	mux.HandleFunc("/api/phase23a/execution-impact", c.execImpact)
	mux.HandleFunc("/api/phase23a/readiness", c.readiness)
	mux.HandleFunc("/api/phase23a/health", handleHealth23A)
	mux.HandleFunc("/metrics", c.prometheusMetrics)
	return mux
}

type cache23A struct {
	mu     sync.RWMutex
	result Phase23AResult
}

func (c *cache23A) get() Phase23AResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.result
}

func writeJSON23(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func handleHealth23A(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, map[string]string{"status": "ok", "phase": "23A"})
}

func (c *cache23A) certification(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	v := r.FinalVerdict
	writeJSON23(w, map[string]interface{}{
		"generated_at":          r.GeneratedAt,
		"total_strategies":      r.TotalStrategies,
		"validated_strategies":  r.ValidatedStrategies,
		"certified_strategies":  r.CertifiedStrategies,
		"retired_strategies":    r.RetiredStrategies,
		"deploy_today":          v.Q10_DeployToday,
		"deploy_reason":         v.DeployTodayReason,
		"portfolio_pf":          v.Q6_PortfolioProfile.ExpectedPF,
		"portfolio_sharpe":      v.Q6_PortfolioProfile.ExpectedSharpe,
		"portfolio_cagr":        v.Q6_PortfolioProfile.ExpectedCAGR,
		"portfolio_max_dd":      v.Q6_PortfolioProfile.ExpectedMaxDD,
		"mc_stability":          r.PortfolioMC.Stability,
		"institutional_ready":   v.Q8_InstitutionalReady,
		"profitable_after_costs": v.Q7_ProfitableAfterCosts,
	})
}

func (c *cache23A) finalVerdict(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().FinalVerdict)
}

func (c *cache23A) top10(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	limit := 10
	if len(r.FinalRanking) < limit {
		limit = len(r.FinalRanking)
	}
	writeJSON23(w, r.FinalRanking[:limit])
}

func (c *cache23A) walkForward(w http.ResponseWriter, _ *http.Request) {
	// Return summary only (full windows could be large)
	r := c.get()
	type summary struct {
		StrategyID   string  `json:"strategy_id"`
		StrategyName string  `json:"strategy_name"`
		Windows      int     `json:"windows"`
		AvgValidPF   float64 `json:"avg_valid_pf"`
		AvgSharpe    float64 `json:"avg_sharpe"`
		Consistency  float64 `json:"consistency_pct"`
		Degradation  float64 `json:"degradation"`
		IsConsistent bool    `json:"is_consistent"`
	}
	summaries := make([]summary, 0, len(r.WalkForward))
	for _, rpt := range r.WalkForward {
		summaries = append(summaries, summary{
			StrategyID:   rpt.StrategyID,
			StrategyName: rpt.StrategyName,
			Windows:      len(rpt.Windows),
			AvgValidPF:   rpt.AvgValidPF,
			AvgSharpe:    rpt.AvgValidSharpe,
			Consistency:  rpt.Consistency,
			Degradation:  rpt.Degradation,
			IsConsistent: rpt.IsConsistent,
		})
	}
	writeJSON23(w, summaries)
}

func (c *cache23A) monteCarlo(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	writeJSON23(w, map[string]interface{}{
		"portfolio":  r.PortfolioMC,
		"strategies": r.MonteCarlo,
	})
}

func (c *cache23A) alphaRankings(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().AlphaRankings)
}

func (c *cache23A) portfolios(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().Portfolios)
}

func (c *cache23A) capitalPlan(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().DeploymentPlan)
}

func (c *cache23A) elimination(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().Eliminated)
}

func (c *cache23A) edgeCerts(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().EdgeCertifications)
}

func (c *cache23A) regimes(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().RegimePerf)
}

func (c *cache23A) execImpact(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().ExecutionImpact)
}

func (c *cache23A) readiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON23(w, c.get().Readiness)
}

func (c *cache23A) prometheusMetrics(w http.ResponseWriter, _ *http.Request) {
	r := c.get()
	v := r.FinalVerdict
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	deployVal := 0.0
	if v.Q10_DeployToday {
		deployVal = 1.0
	}
	instVal := 0.0
	if v.Q8_InstitutionalReady {
		instVal = 1.0
	}
	profVal := 0.0
	if v.Q7_ProfitableAfterCosts {
		profVal = 1.0
	}

	metrics := []struct{ name, help, val string }{
		{"phase23a_deploy_today", "1 if system approved for live deployment today", strconv.FormatFloat(deployVal, 'f', 0, 64)},
		{"phase23a_institutional_ready", "1 if system meets institutional readiness criteria", strconv.FormatFloat(instVal, 'f', 0, 64)},
		{"phase23a_profitable_after_costs", "1 if system profitable after fees and slippage", strconv.FormatFloat(profVal, 'f', 0, 64)},
		{"phase23a_portfolio_pf", "Portfolio profit factor from walk-forward validation", strconv.FormatFloat(v.Q6_PortfolioProfile.ExpectedPF, 'f', 6, 64)},
		{"phase23a_portfolio_sharpe", "Portfolio Sharpe ratio from walk-forward validation", strconv.FormatFloat(v.Q6_PortfolioProfile.ExpectedSharpe, 'f', 6, 64)},
		{"phase23a_portfolio_cagr", "Expected portfolio CAGR percent", strconv.FormatFloat(v.Q6_PortfolioProfile.ExpectedCAGR, 'f', 6, 64)},
		{"phase23a_portfolio_max_dd", "Expected portfolio max drawdown percent", strconv.FormatFloat(v.Q6_PortfolioProfile.ExpectedMaxDD, 'f', 6, 64)},
		{"phase23a_certified_strategies", "Count of strategies passing edge certification", strconv.FormatFloat(float64(r.CertifiedStrategies), 'f', 0, 64)},
		{"phase23a_retired_strategies", "Count of strategies flagged for retirement", strconv.FormatFloat(float64(r.RetiredStrategies), 'f', 0, 64)},
		{"phase23a_live_capital_strategies", "Count of strategies approved for live capital", strconv.FormatFloat(float64(len(v.Q5_LiveCapital)), 'f', 0, 64)},
		{"phase23a_mc_p_grow", "Portfolio Monte Carlo P(growth)", strconv.FormatFloat(r.PortfolioMC.ProbabilityGrow, 'f', 6, 64)},
		{"phase23a_mc_p_ruin", "Portfolio Monte Carlo P(ruin)", strconv.FormatFloat(r.PortfolioMC.ProbabilityRuin, 'f', 6, 64)},
	}
	for _, m := range metrics {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %s\n", m.name, m.help, m.name, m.name, m.val)
	}
}

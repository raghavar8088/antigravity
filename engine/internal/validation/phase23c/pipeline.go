package phase23c

import (
	"log"
	"time"

	"antigravity-engine/internal/validation/phase23b"
)

// Pipeline23C orchestrates the full Phase 23C edge discovery process.
type Pipeline23C struct {
	TotalCapital float64
}

// NewPipeline23C creates a Phase 23C pipeline.
func NewPipeline23C(totalCapital float64) *Pipeline23C {
	return &Pipeline23C{TotalCapital: totalCapital}
}

// Run executes all Phase 23C phases against the Phase 23B result.
func (p *Pipeline23C) Run(source *phase23b.Phase23BResult) Phase23CResult {
	result := Phase23CResult{
		GeneratedAt: time.Now().UTC(),
		Source23B:   source,
	}

	// ── Phase 23C.1 — Edge Discovery and Full Ranking ─────────────────────────
	log.Println("[23C.1] Building evidence-based strategy ranking...")
	result.AllRanked = RankAllStrategies(source)
	result.TotalStrategiesEvaluated = len(result.AllRanked)
	log.Printf("[23C.1] Ranked %d strategies", len(result.AllRanked))

	// ── Phase 23C.2 — Top 20 ─────────────────────────────────────────────────
	log.Println("[23C.2] Selecting Top 20...")
	result.Top20 = SelectTop(result.AllRanked, 20)

	// ── Phase 23C.3 — Top 10 ─────────────────────────────────────────────────
	log.Println("[23C.3] Selecting Top 10...")
	result.Top10 = SelectTop(result.AllRanked, 10)

	// ── Phase 23C.4 — Top 5 Portfolio ────────────────────────────────────────
	log.Println("[23C.4] Constructing Top 5 Portfolio...")
	result.Top5Portfolio = BuildTop5Portfolio(result.AllRanked, source.Metrics, p.TotalCapital)

	// ── Phase 23C.5 — Alpha Championship ─────────────────────────────────────
	log.Println("[23C.5] Running Alpha Engine Championship...")
	result.AlphaChampionship = BuildAlphaChampionship(source)

	// ── Phase 23C.6 — Strategy Elimination ───────────────────────────────────
	log.Println("[23C.6] Eliminating strategies failing hard gates...")
	result.Eliminated = EliminateStrategies(source)
	result.TotalRetired = len(result.Eliminated)
	log.Printf("[23C.6] %d strategies permanently eliminated", result.TotalRetired)

	// ── Phase 23C.7 — Final Deployment Decision ───────────────────────────────
	log.Println("[23C.7] Building final deployment verdict...")
	result.FinalVerdict = BuildFinalVerdict(source, result.AllRanked,
		result.AlphaChampionship, result.Eliminated, result.Top5Portfolio)

	// Summary counts
	for _, r := range result.AllRanked {
		if r.DeploymentStatus == "DEPLOY_NOW" {
			result.TotalDeployNow++
		}
		if r.ProfitFactor >= 1.30 && r.Sharpe >= 1.50 {
			result.TotalWithEdge++
		}
	}
	for _, t := range source.CertifiedTrades {
		result.PlatformNetPnLUSD += t.NetPnLUSD
	}

	sumWin, sumLoss := 0.0, 0.0
	for _, t := range source.CertifiedTrades {
		if t.NetPnLUSD > 0 {
			sumWin += t.NetPnLUSD
		} else {
			sumLoss += -t.NetPnLUSD
		}
	}
	if sumLoss > 0 {
		result.PlatformProfitFactor = sumWin / sumLoss
	}

	// Platform Sharpe: average across all strategy Sharpes (weighted by trade count)
	totalTrades := 0
	weightedSharpe := 0.0
	for _, m := range source.Metrics {
		weightedSharpe += m.Sharpe * float64(m.TotalTrades)
		totalTrades += m.TotalTrades
	}
	if totalTrades > 0 {
		result.PlatformSharpe = weightedSharpe / float64(totalTrades)
	}

	log.Printf("[23C.7] Final verdict: deploy=%v, %d strategies with edge, %d deployable",
		result.FinalVerdict.Q11_DeployCapitalToday,
		result.TotalWithEdge,
		result.TotalDeployNow)

	return result
}

// phase23a is the CLI runner for Phase 23A: Massive Historical Validation &
// Institutional Edge Certification.
//
// Usage:
//
//	go run ./cmd/phase23a/main.go [--out=<dir>] [--capital=<usd>] [--months=<n>] [--serve=<addr>]
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"antigravity-engine/internal/validation/phase23a"
)

func main() {
	outDir := flag.String("out", "phase23a_reports", "directory to write report files")
	capital := flag.Float64("capital", 1_000_000, "total deployable capital in USD")
	months := flag.Int("months", 24, "months of historical data to simulate (12 min, 24 preferred)")
	serve := flag.String("serve", "", "if set, start REST API on this address e.g. :8082")
	seed := flag.Int64("seed", 42, "random seed for reproducible demo data")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	rng := rand.New(rand.NewSource(*seed))

	fmt.Printf("Phase 23A — Institutional Historical Validation\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Capital:      $%.0f\n", *capital)
	fmt.Printf("Dataset:      %d months of BTC-USDT 1m candles (synthetic)\n", *months)
	fmt.Printf("Strategies:   %d\n", len(phase23a.DefaultStrategySpecs()))
	fmt.Printf("Output dir:   %s\n\n", *outDir)

	// ── Phase 2: Generate synthetic candle history ────────────────────────────
	fmt.Printf("Generating %d months of BTC-USDT 1m candles...", *months)
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := phase23a.GenerateSyntheticCandles(rng, from, *months)
	fmt.Printf(" done (%d candles)\n", len(candles))

	// ── Run pipeline ──────────────────────────────────────────────────────────
	fmt.Println("Running Phase 23A pipeline (14 phases)...")
	p := phase23a.NewPipeline23A(*capital)
	p.SetRNG(rng)
	result := p.Run(candles, phase23a.DefaultStrategySpecs(), nil)

	// ── Console summary ───────────────────────────────────────────────────────
	v := result.FinalVerdict
	mc := result.PortfolioMC

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	deployLabel := "NO — DO NOT DEPLOY"
	if v.Q10_DeployToday {
		deployLabel = "YES — APPROVED FOR LIVE DEPLOYMENT"
	}
	fmt.Printf("  DEPLOY LIVE CAPITAL TODAY: %s\n", deployLabel)
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  Readiness:          %s\n", okStr(result.Readiness.Passed))
	fmt.Printf("  Dataset Quality:    %.1f/100\n", result.Dataset.QualityScore)
	fmt.Printf("  Strategies tested:  %d\n", result.TotalStrategies)
	fmt.Printf("  WF windows/strat:   %d\n", avgWindows(result.WalkForward))
	fmt.Printf("  Certified (full):   %d\n", countFullCerts(result.EdgeCertifications))
	fmt.Printf("  Partial cert:       %d\n", countPartialCerts(result.EdgeCertifications))
	fmt.Printf("  Retired:            %d\n", result.RetiredStrategies)
	fmt.Println()
	fmt.Printf("  Portfolio PF:       %.3f\n", v.Q6_PortfolioProfile.ExpectedPF)
	fmt.Printf("  Portfolio Sharpe:   %.2f\n", v.Q6_PortfolioProfile.ExpectedSharpe)
	fmt.Printf("  Expected CAGR:      %.1f%%\n", v.Q6_PortfolioProfile.ExpectedCAGR)
	fmt.Printf("  Max Drawdown:       %.1f%%\n", v.Q6_PortfolioProfile.ExpectedMaxDD)
	fmt.Printf("  MC Stability:       %s  (P(grow)=%.0f%%  P(ruin)=%.1f%%)\n",
		mc.Stability, mc.ProbabilityGrow*100, mc.ProbabilityRuin*100)
	fmt.Println()

	fmt.Printf("  Live capital approved:   %d strategy(ies)\n", len(v.Q5_LiveCapital))
	for i, s := range v.Q5_LiveCapital {
		fmt.Printf("    %d. %s\n", i+1, s)
	}
	fmt.Println()

	fmt.Println("  TOP 5 STRATEGIES:")
	for _, rs := range result.FinalRanking[:min5(5, len(result.FinalRanking))] {
		fmt.Printf("    %d. %-35s PF=%.2f  Sharpe=%.2f  CAGR=%.1f%%  %s\n",
			rs.Rank, rs.StrategyName, rs.ProfitFactor, rs.Sharpe, rs.CAGR, rs.CertificationTier)
	}
	fmt.Println()

	if len(result.AlphaRankings) > 0 {
		fmt.Printf("  Strongest alpha: %s (PF=%.2f)\n", result.AlphaRankings[0].AlphaEngine, result.AlphaRankings[0].ProfitFactor)
	}
	fmt.Println()

	// ── Write reports ────────────────────────────────────────────────────────
	fmt.Printf("Writing 14 institutional reports to: %s\n", *outDir)
	if err := phase23a.WriteAllReports(result, *outDir); err != nil {
		log.Fatalf("write reports: %v", err)
	}

	for _, r := range []string{
		"VALIDATION_READINESS_REPORT.md",
		"DATA_INTEGRITY_REPORT.md",
		"TOP20_SELECTION_REPORT.md",
		"WALK_FORWARD_REPORT.md",
		"MONTE_CARLO_REPORT.md",
		"REGIME_ANALYSIS_REPORT.md",
		"EXECUTION_IMPACT_REPORT.md",
		"ALPHA_ENGINE_RANKINGS.md",
		"PORTFOLIO_CONSTRUCTION_REPORT.md",
		"ELIMINATION_REPORT.md",
		"EDGE_CERTIFICATION_REPORT.md",
		"FINAL_RANKING_REPORT.md",
		"CAPITAL_DEPLOYMENT_PLAN.md",
		"PHASE23A_FINAL_CERTIFICATION.md",
	} {
		fmt.Printf("  OK  %s/%s\n", *outDir, r)
	}
	fmt.Println()

	// ── Optional REST server ──────────────────────────────────────────────────
	if *serve != "" {
		handler := phase23a.Handler23A(result)
		fmt.Printf("Phase 23A REST API on http://%s\n", *serve)
		fmt.Println("  GET /api/phase23a/certification")
		fmt.Println("  GET /api/phase23a/final-verdict")
		fmt.Println("  GET /api/phase23a/top10")
		fmt.Println("  GET /api/phase23a/walk-forward")
		fmt.Println("  GET /api/phase23a/capital-plan")
		fmt.Println("  GET /metrics  (Prometheus)")
		log.Fatal(http.ListenAndServe(*serve, handler))
	}

	if v.Q10_DeployToday {
		fmt.Printf("\nFINAL: APPROVED FOR LIVE DEPLOYMENT\n%s\n", v.DeployTodayReason)
	} else {
		fmt.Printf("\nFINAL: NOT APPROVED — %s\n", v.DeployTodayReason)
	}
}

func okStr(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func avgWindows(reports []phase23a.WalkForwardReport) int {
	if len(reports) == 0 {
		return 0
	}
	total := 0
	for _, r := range reports {
		total += len(r.Windows)
	}
	return total / len(reports)
}

func countFullCerts(certs []phase23a.EdgeCertification) int {
	n := 0
	for _, c := range certs {
		if c.Certified {
			n++
		}
	}
	return n
}

func countPartialCerts(certs []phase23a.EdgeCertification) int {
	n := 0
	for _, c := range certs {
		if c.PartialCredit && !c.Certified {
			n++
		}
	}
	return n
}

func min5(a, b int) int {
	if a < b {
		return a
	}
	return b
}

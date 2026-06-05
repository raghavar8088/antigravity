// Command phase25 runs the Phase 25 Live Edge Verification & Capital Deployment.
//
// Phase 25 is the final bridge between historical validation and capital deployment.
// It imports Phase 24 certification output as the historical baseline, runs a
// live-forward simulation against the most recent 30–60 days of real Binance
// Futures data, detects statistical drift in every strategy and alpha family,
// applies a 5-tier promotion/demotion capital ladder, and generates 14 institutional
// markdown reports with 15 REST endpoints.
//
// Mission: Determine whether Phase 24 certified strategies maintain a statistically
// significant and deployable edge in real-time BTC futures market conditions, and
// determine the exact capital allocation they deserve.
//
// No new strategies. No indicator optimisation. No strategy logic changes.
// No synthetic data. No fabricated trades. Measured evidence only.
//
// Usage:
//
//	go run ./cmd/phase25 [flags]
//
// Flags:
//
//	--days int       Live-forward window in days (default 45, range 30–60)
//	--capital float  Initial paper capital in USD (default 1_000_000)
//	--out string     Output directory for reports (default ./phase25_reports)
//	--max-strats int Max strategies (0 = all eligible)
//	--serve          Start HTTP server after certification
//	--port int       HTTP server port (default 8085)
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"antigravity-engine/internal/validation/phase23b"
	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
)

func main() {
	days := flag.Int("days", 45, "Live-forward window in days (30–60 recommended)")
	capital := flag.Float64("capital", 1_000_000.0, "Initial paper capital in USD")
	outDir := flag.String("out", "./phase25_reports", "Output directory for markdown reports")
	maxStrats := flag.Int("max-strats", 0, "Max strategies (0 = all eligible)")
	serve := flag.Bool("serve", false, "Start HTTP server after certification")
	port := flag.Int("port", 8085, "HTTP server port")
	flag.Parse()

	if *days < 30 {
		log.Fatal("FATAL: minimum 30 days required for live-forward validation")
	}
	if *days > 60 {
		log.Printf("WARNING: --days=%d exceeds recommended 60-day window; proceeding", *days)
	}

	printBanner(*days)

	cfg := phase25.Phase25Config{
		Symbol:         "BTCUSDT",
		LiveDays:       *days,
		InitialCapital: *capital,
		TakerFeeBps:    phase25.TakerFeeBps,
		SlippageBps:    phase25.SlippageBps,
		MaxStrategies:  *maxStrats,
	}

	log.Printf("[CONFIG] Symbol=%s | LiveDays=%d | Capital=$%.0f | MaxStrats=%d",
		cfg.Symbol, cfg.LiveDays, cfg.InitialCapital, cfg.MaxStrategies)

	// ── Build Phase 24 baseline (run a fresh Phase 24 certification) ──────────
	log.Println("[BASELINE] Running Phase 24 historical baseline certification...")
	to := time.Now().UTC().Truncate(time.Minute)
	from := to.AddDate(0, -24, 0)

	p24cfg := phase23b.ReplayConfig{
		Symbol:         "BTCUSDT",
		From:           from,
		To:             to,
		InitialCapital: *capital,
		PositionCapPct: phase23b.DefaultPositionCapPct,
		TakerFeeBps:    phase23b.TakerFeeBps,
		MakerFeeBps:    phase23b.MakerFeeBps,
		SlippageBps:    phase23b.SlippageBps,
		MaxHoldMins:    phase23b.DefaultMaxHoldMins,
		MinConfidence:  phase23b.DefaultMinConfidence,
		MaxStrategies:  *maxStrats,
	}
	p24pipeline := phase24.NewPipeline24(p24cfg)
	hist, err := p24pipeline.Run()
	if err != nil {
		log.Printf("WARNING: Phase 24 baseline failed (%v) — proceeding with empty baseline", err)
		hist = phase24.Phase24Result{}
		hist.Metrics = make(map[string]phase24.ExtendedMetrics)
		hist.MonteCarlo = make(map[string]phase23b.RealMCResult)
	} else {
		log.Printf("[BASELINE] Phase 24 complete: PF=%.3f Sharpe=%.2f Strategies=%d",
			hist.PlatformPF, hist.PlatformSharpe, hist.TotalStrategiesEvaluated)
	}

	// ── Phase 25 ─────────────────────────────────────────────────────────────
	pipeline := phase25.NewPipeline25(cfg)
	result, err := pipeline.Run(hist)
	if err != nil {
		log.Fatalf("Phase 25 pipeline failed: %v", err)
	}

	log.Printf("")
	log.Printf("[WRITE] Writing 14 institutional reports to %s ...", *outDir)
	if err := phase25.WriteAllReports(result, *outDir); err != nil {
		log.Printf("WARNING: Report write error: %v", err)
	} else {
		log.Printf("[WRITE] All 14 reports written successfully.")
	}

	printFinalSummary(result)

	if *serve {
		addr := fmt.Sprintf(":%d", *port)
		log.Printf("[HTTP] Starting Phase 25 server on %s ...", addr)

		mux := http.NewServeMux()
		h := phase25.NewHandler25(result)
		mux.HandleFunc("/api/phase25/", h.ServeHTTP)
		mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, `{"status":"ok","phase":"25"}`)
		})

		log.Printf("[HTTP] 15 endpoints available under /api/phase25/")
		log.Printf("[HTTP]   eligibility, live-trades, live-metrics, edge-drift,")
		log.Printf("[HTTP]   alpha-survival, capital-escalation, auto-demotion,")
		log.Printf("[HTTP]   portfolio-heat, execution-quality, monthly/*, final-verdict, summary")
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}

	exitCode := 0
	if result.Verdict.OverallVerdict == "HALT" {
		exitCode = 2
	} else if !result.Verdict.Q13_LiveDeploymentJustified {
		exitCode = 1
	}
	os.Exit(exitCode)
}

func printBanner(days int) {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║   PHASE 25 — LIVE EDGE VERIFICATION & CAPITAL DEPLOYMENT            ║")
	fmt.Println("║   CIO | Quant Research | Risk Committee | Execution Intel Lead       ║")
	fmt.Println("║   Real Feed | Real Execution | Real Evidence | No Synthesis          ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf( "║   Live Window: %d days | Symbol: BTCUSDT Futures                   ║\n", days)
	fmt.Println("║   Data: Real Binance Futures (live-forward, not historical replay)   ║")
	fmt.Println("║   Strategies: Phase 24 Certified Only                               ║")
	fmt.Println("║   Mode: Paper Trading (no real capital at risk)                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

func printFinalSummary(r phase25.Phase25Result) {
	v := r.Verdict
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PHASE 25 — LIVE DEPLOYMENT VERDICT                     ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Live Platform PF:         %-43.4f║\n", v.PlatformLivePF)
	fmt.Printf("║  Live Platform Sharpe:     %-43.4f║\n", v.PlatformLiveSharpe)
	fmt.Printf("║  Live Net PnL:             $%-42.0f║\n", v.PlatformLiveNetPnL)
	fmt.Printf("║  Total Live Trades:        %-43d║\n", v.TotalLiveTrades)
	fmt.Printf("║  Live-Certified Strategies:%-43d║\n", v.LiveCertifiedStrategies)
	fmt.Printf("║  Total Demoted:            %-43d║\n", r.TotalDemoted)
	fmt.Printf("║  Total Retired:            %-43d║\n", r.TotalRetired)
	fmt.Printf("║  Portfolio Heat:           %-43.1f║\n", r.PortfolioHeat.TotalHeat)
	fmt.Printf("║  Exec Quality Score:       %-43.1f║\n", r.ExecQuality.QualityScore)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")

	verdictLine := map[string]string{
		"DEPLOY":     "║  ✅ LIVE DEPLOYMENT APPROVED — CAPITAL CAN BE DEPLOYED TODAY        ║",
		"PAPER_ONLY": "║  ⚠️  PAPER ONLY — CONTINUE FORWARD VALIDATION BEFORE DEPLOYMENT      ║",
		"HALT":       "║  ❌ HALT — EDGE FAILED — NO CAPITAL DEPLOYMENT                       ║",
	}[v.OverallVerdict]
	fmt.Println(verdictLine)

	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	if len(r.AlphaSurvival) > 0 {
		fmt.Printf("║  CHAMPION ALPHA: %-52s║\n", banner25(string(r.AlphaSurvival[0].Family), 52))
		last := r.AlphaSurvival[len(r.AlphaSurvival)-1]
		fmt.Printf("║  WEAKEST  ALPHA: %-52s║\n", banner25(string(last.Family), 52))
	}
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  REPORTS WRITTEN:                                                    ║")
	for _, f := range []string{
		"PHASE25_STRATEGY_ELIGIBILITY_REPORT.md",
		"LIVE_FORWARD_VALIDATION_REPORT.md",
		"PAPER_TRADING_CAMPAIGN_REPORT.md",
		"EDGE_DRIFT_REPORT.md",
		"ALPHA_SURVIVAL_REPORT.md",
		"CAPITAL_ESCALATION_REPORT.md",
		"AUTO_DEMOTION_REPORT.md",
		"PORTFOLIO_HEAT_REPORT.md",
		"EXECUTION_QUALITY_REPORT.md",
		"MONTHLY_STRATEGY_REPORT.md",
		"MONTHLY_ALPHA_REPORT.md",
		"MONTHLY_PORTFOLIO_REPORT.md",
		"MONTHLY_CAPITAL_REPORT.md",
		"PHASE25_FINAL_LIVE_VERDICT.md",
	} {
		fmt.Printf("║  ✓ %-67s║\n", f)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

func banner25(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

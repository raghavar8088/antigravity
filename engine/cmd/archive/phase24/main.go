// Command phase24 runs the Phase 24 Institutional Real Edge Certification.
//
// This is the definitive certification run.  It loads real BTC Futures data
// across 1m / 3m / 5m / 15m timeframes, replays the full strategy inventory
// using actual Strategy.OnCandle() execution, and produces 12 institutional
// markdown reports with a final deployment verdict.
//
// Usage:
//
//	go run ./cmd/phase24 [flags]
//
// Flags:
//
//	--months int      History depth in months (default 24, minimum 12)
//	--capital float   Initial capital in USD (default 1_000_000)
//	--out string      Output directory for reports (default ./phase24_reports)
//	--max-strats int  Max strategies to replay (0 = all)
//	--serve           Start HTTP server after certification
//	--port int        HTTP server port (default 8084)
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
)

func main() {
	months := flag.Int("months", 24, "History depth in months (24 preferred, 12 minimum)")
	capital := flag.Float64("capital", 1_000_000.0, "Initial capital in USD")
	outDir := flag.String("out", "./phase24_reports", "Output directory for markdown reports")
	maxStrats := flag.Int("max-strats", 0, "Max strategies (0 = all)")
	serve := flag.Bool("serve", false, "Start HTTP server after certification completes")
	port := flag.Int("port", 8084, "HTTP server port")
	flag.Parse()

	if *months < 12 {
		log.Fatal("FATAL: minimum 12 months required for institutional certification")
	}

	printBanner()

	to := time.Now().UTC().Truncate(time.Minute)
	from := to.AddDate(0, -*months, 0)

	cfg := phase23b.ReplayConfig{
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

	log.Printf("[CONFIG] Symbol=%s | %s → %s | Capital=$%.0f | MaxStrats=%d",
		cfg.Symbol, cfg.From.Format("2006-01-02"), cfg.To.Format("2006-01-02"),
		cfg.InitialCapital, cfg.MaxStrategies)

	pipeline := phase24.NewPipeline24(cfg)
	result, err := pipeline.Run()
	if err != nil {
		log.Fatalf("Phase 24 pipeline failed: %v", err)
	}

	log.Printf("")
	log.Printf("[WRITE] Writing 12 institutional reports to %s ...", *outDir)
	if err := phase24.WriteAllReports(result, *outDir); err != nil {
		log.Printf("WARNING: Report write error: %v", err)
	} else {
		log.Printf("[WRITE] All 12 reports written successfully.")
	}

	printFinalSummary(result)

	if *serve {
		addr := fmt.Sprintf(":%d", *port)
		log.Printf("[HTTP] Starting server on %s ...", addr)

		mux := http.NewServeMux()
		h := phase24.Handler24(result)
		mux.HandleFunc("/api/phase24/", h.ServeHTTP)
		mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, `{"status":"ok","phase":"24"}`)
		})

		log.Printf("[HTTP] Endpoints: data-certification, strategy-evidence, alpha-championship,")
		log.Printf("[HTTP]            walk-forward, monte-carlo, regime-edge, retirement,")
		log.Printf("[HTTP]            capital-certification, top3/5/10-portfolio, leaderboard,")
		log.Printf("[HTTP]            final-verdict, summary, health")
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}

	exitCode := 0
	if !result.Verdict.Q17_ApproveDeployment {
		exitCode = 1
	}
	os.Exit(exitCode)
}

func printBanner() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║      PHASE 24 — INSTITUTIONAL REAL EDGE CERTIFICATION               ║")
	fmt.Println("║      Chief Investment Officer | Quant Research | Risk Committee      ║")
	fmt.Println("║      Real Data | Real Execution | Real Evidence | No Synthesis       ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Timeframes: 1m / 3m / 5m / 15m                                    ║")
	fmt.Println("║  Data: Binance Futures klines + funding + OI                        ║")
	fmt.Println("║  Execution: Strategy.OnCandle() — real signal generation             ║")
	fmt.Println("║  Costs: Taker fee 0.04% + Slippage 0.03% + Funding                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

func printFinalSummary(r phase24.Phase24Result) {
	v := r.Verdict
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║              PHASE 24 — INSTITUTIONAL CERTIFICATION VERDICT          ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Platform Profit Factor:   %-43.4f║\n", v.PlatformPF)
	fmt.Printf("║  Platform Sharpe:          %-43.4f║\n", v.PlatformSharpe)
	fmt.Printf("║  Platform Net PnL:         $%-42.0f║\n", v.PlatformNetPnL)
	fmt.Printf("║  Total Certified Trades:   %-43d║\n", len(r.AllTrades))
	fmt.Printf("║  Strategies Evaluated:     %-43d║\n", v.TotalStrategies)
	fmt.Printf("║  Deploy-Ready:             %-43d║\n", v.DeployStrategies)
	fmt.Printf("║  Retired:                  %-43d║\n", v.RetiredStrategies)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")

	if v.Q1_HasRealEdge {
		fmt.Println("║  REAL EDGE:    ✅ CONFIRMED                                          ║")
	} else {
		fmt.Println("║  REAL EDGE:    ❌ NOT CONFIRMED AT INSTITUTIONAL STANDARD             ║")
	}
	if v.Q10_ProfitableAfterCosts {
		fmt.Println("║  AFTER COSTS:  ✅ PROFITABLE                                         ║")
	} else {
		fmt.Println("║  AFTER COSTS:  ❌ NOT PROFITABLE                                     ║")
	}
	if v.Q12_InstitutionalGrade {
		fmt.Println("║  INST. GRADE:  ✅ YES                                                ║")
	} else {
		fmt.Println("║  INST. GRADE:  ❌ NOT YET                                            ║")
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	if v.Q17_ApproveDeployment {
		fmt.Println("║  ✅ DEPLOYMENT APPROVED — CAPITAL CAN BE DEPLOYED TODAY              ║")
	} else {
		fmt.Println("║  ❌ DEPLOYMENT NOT APPROVED — CONTINUE PAPER TRADING                 ║")
	}
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  TOP 5 STRATEGIES:                                                   ║")
	for i, s := range v.Q15_Top5Strategies {
		fmt.Printf("║    #%d %-64s║\n", i+1, truncateBanner(s, 64))
	}
	if len(r.AlphaChampionship) > 0 {
		fmt.Printf("║  CHAMPION ALPHA: %-52s║\n", truncateBanner(string(r.AlphaChampionship[0].Engine), 52))
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("Reports written to: %s\n", "./phase24_reports")
	fmt.Println("Files:")
	for _, f := range []string{
		"DATA_CERTIFICATION_REPORT.md",
		"STRATEGY_EVIDENCE_REPORT.md",
		"ALPHA_CHAMPIONSHIP_REPORT.md",
		"WALK_FORWARD_CERTIFICATION.md",
		"MONTE_CARLO_CERTIFICATION.md",
		"REGIME_EDGE_REPORT.md",
		"STRATEGY_RETIREMENT_REPORT.md",
		"CAPITAL_CERTIFICATION_REPORT.md",
		"TOP3_PORTFOLIO.md",
		"TOP5_PORTFOLIO.md",
		"TOP10_PORTFOLIO.md",
		"PHASE24_FINAL_VERDICT.md",
	} {
		fmt.Printf("  ✓ %s\n", f)
	}
	fmt.Println("")
}

func truncateBanner(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// Command phase23b runs the Phase 23B + 23C institutional validation pipeline.
//
// It loads real BTC futures market data from Binance, runs real strategy instances
// via Strategy.OnCandle(), and produces a fully evidence-backed certification
// of which strategies have deployable trading edge.
//
// Usage:
//
//	go run ./cmd/phase23b [flags]
//
// Flags:
//
//	--months int     History depth in months (default 24)
//	--capital float  Initial capital in USD (default 1000000)
//	--out string     Output directory for reports (default ./phase23b_reports)
//	--max-strats int Max strategies to run (0 = all, default 50)
//	--serve          Start HTTP server after validation
//	--port int       HTTP port (default 8082)
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"antigravity-engine/internal/validation/phase23b"
	"antigravity-engine/internal/validation/phase23c"
)

func main() {
	months := flag.Int("months", 24, "History depth in months (24 preferred, 12 minimum)")
	capital := flag.Float64("capital", 1_000_000.0, "Initial capital in USD")
	outDir := flag.String("out", "./phase23b_reports", "Output directory for markdown reports")
	maxStrats := flag.Int("max-strats", 50, "Max strategies to replay (0 = all)")
	serve := flag.Bool("serve", false, "Start HTTP server after pipeline completes")
	port := flag.Int("port", 8082, "HTTP server port")
	flag.Parse()

	if *months < 12 {
		log.Fatal("FATAL: minimum 12 months of data required for institutional validation")
	}

	log.Printf("═══════════════════════════════════════════════════════════════")
	log.Printf("  PHASE 23B + 23C — INSTITUTIONAL EDGE VALIDATION")
	log.Printf("  Real BTC Futures Data | Real Strategy Execution | No Synthesis")
	log.Printf("═══════════════════════════════════════════════════════════════")

	// ── Build config ──────────────────────────────────────────────────────────
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

	log.Printf("[CONFIG] Symbol=%s | Period=%s→%s | Capital=$%.0f | MaxStrategies=%d",
		cfg.Symbol, cfg.From.Format("2006-01-02"), cfg.To.Format("2006-01-02"),
		cfg.InitialCapital, cfg.MaxStrategies)

	// ── Phase 23B ─────────────────────────────────────────────────────────────
	log.Println("")
	log.Println("── PHASE 23B: REAL MARKET EVIDENCE INTEGRATION ──")
	log.Println("")

	pipeline23b := phase23b.NewPipeline23B(cfg)
	result23b, err := pipeline23b.Run()
	if err != nil {
		log.Fatalf("Phase 23B pipeline failed: %v", err)
	}

	log.Printf("Phase 23B complete:")
	log.Printf("  Candles: %d (%.0f days)", result23b.TotalCandles, result23b.CoverageDays)
	log.Printf("  Certified trades: %d", result23b.TotalTrades)
	log.Printf("  Strategies: %d validated, %d certified, %d retired",
		result23b.ValidatedStrategies, result23b.CertifiedStrategies, result23b.RetiredStrategies)

	// ── Phase 23C ─────────────────────────────────────────────────────────────
	log.Println("")
	log.Println("── PHASE 23C: INSTITUTIONAL EDGE DISCOVERY ──")
	log.Println("")

	pipeline23c := phase23c.NewPipeline23C(*capital)
	result23c := pipeline23c.Run(&result23b)

	log.Printf("Phase 23C complete:")
	log.Printf("  Strategies with edge: %d/%d", result23c.TotalWithEdge, result23c.TotalStrategiesEvaluated)
	log.Printf("  Deploy Now: %d | Retired: %d", result23c.TotalDeployNow, result23c.TotalRetired)
	log.Printf("  Platform PF: %.3f | Net PnL: $%.0f", result23c.PlatformProfitFactor, result23c.PlatformNetPnLUSD)
	if result23c.FinalVerdict.Q11_DeployCapitalToday {
		log.Printf("  ✅ CAPITAL DEPLOYMENT APPROVED")
	} else {
		log.Printf("  ❌ CAPITAL DEPLOYMENT NOT APPROVED")
	}

	// ── Write Reports ─────────────────────────────────────────────────────────
	log.Printf("")
	log.Printf("Writing Phase 23B reports to %s ...", *outDir)
	if err := phase23b.WriteAllReports(result23b, *outDir); err != nil {
		log.Printf("WARNING: Phase 23B report write error: %v", err)
	}

	log.Printf("Writing Phase 23C reports to %s ...", *outDir)
	if err := phase23c.WriteAllReports(result23c, *outDir); err != nil {
		log.Printf("WARNING: Phase 23C report write error: %v", err)
	}

	printFinalSummary(result23c)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	if *serve {
		addr := fmt.Sprintf(":%d", *port)
		log.Printf("Starting HTTP server on %s ...", addr)

		mux := http.NewServeMux()
		// Mount both APIs on the same mux
		b23Handler := phase23b.Handler23B(result23b)
		c23Handler := phase23c.Handler23C(result23c)

		mux.HandleFunc("/api/phase23b/", func(w http.ResponseWriter, r *http.Request) {
			b23Handler.ServeHTTP(w, r)
		})
		mux.HandleFunc("/api/phase23c/", func(w http.ResponseWriter, r *http.Request) {
			c23Handler.ServeHTTP(w, r)
		})
		mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, `{"status":"ok","phase":"23B+23C"}`)
		})

		log.Printf("Phase 23B endpoints: /api/phase23b/{data-audit,synthetic-removal,replay-results,cost-model,certified-trades/summary,walk-forward,monte-carlo,regimes,capital-certification,metrics,health}")
		log.Printf("Phase 23C endpoints: /api/phase23c/{edge-discovery,top20,top10,top5-portfolio,alpha-championship,eliminated,final-verdict,health}")

		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}

	os.Exit(0)
}

func printFinalSummary(r phase23c.Phase23CResult) {
	v := r.FinalVerdict
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║         PHASE 23B + 23C — INSTITUTIONAL VERDICT             ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Platform Profit Factor:  %-34.3f║\n", r.PlatformProfitFactor)
	fmt.Printf("║  Platform Net PnL:        $%-33.0f║\n", r.PlatformNetPnLUSD)
	fmt.Printf("║  Platform Sharpe:         %-34.2f║\n", r.PlatformSharpe)
	fmt.Printf("║  Strategies With Edge:    %-34d║\n", r.TotalWithEdge)
	fmt.Printf("║  Deploy Now:              %-34d║\n", r.TotalDeployNow)
	fmt.Printf("║  Retired:                 %-34d║\n", r.TotalRetired)
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")

	if v.Q11_DeployCapitalToday {
		fmt.Println("║  CAPITAL DEPLOYMENT:      ✅ APPROVED — DEPLOY TODAY         ║")
	} else {
		fmt.Println("║  CAPITAL DEPLOYMENT:      ❌ NOT APPROVED                    ║")
	}

	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  TOP 5 STRATEGIES:                                           ║")
	for i, r2 := range v.Q10_TrueTop5 {
		line := fmt.Sprintf("║    #%d %-30s PF=%.3f Sharpe=%.2f     ║",
			i+1, truncate(r2.StrategyName, 30), r2.ProfitFactor, r2.Sharpe)
		fmt.Println(line)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

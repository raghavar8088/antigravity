// Command phase29 runs Phase 29: Institutional Evidence Extraction & Final Deployment Decision.
//
// Phase 29 is the final synthesis layer. It orchestrates the complete
// Phase 24 → 25 → 26 → 27 → 28 pipeline and produces 10 institutional
// markdown reports plus a final deployment verdict.
//
// Usage:
//
//	go run ./cmd/phase29 [flags]
//
// Flags:
//
//	--months int      History depth in months (default 24, minimum 12)
//	--capital float   Initial capital in USD (default 1_000_000)
//	--out string      Output directory for reports (default ./phase29_reports)
//	--max-strats int  Max strategies to replay (0 = all)
//	--serve           Start HTTP server after all reports are written
//	--port int        HTTP server port (default 8089)
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"antigravity-engine/internal/validation/phase23b"
	"antigravity-engine/internal/validation/phase29"
)

func main() {
	months := flag.Int("months", 24, "History depth in months (24 preferred, 12 minimum)")
	capital := flag.Float64("capital", 1_000_000.0, "Initial capital in USD")
	outDir := flag.String("out", "./phase29_reports", "Output directory for markdown reports")
	maxStrats := flag.Int("max-strats", 0, "Max strategies (0 = all)")
	serve := flag.Bool("serve", false, "Start HTTP server after reports are written")
	port := flag.Int("port", 8089, "HTTP server port")
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

	log.Printf("[PHASE29] Running complete institutional evidence pipeline...")
	log.Printf("[PHASE29] Phase 24 → Phase 25 → Phase 26 → Phase 27 → Phase 28 → Phase 29")

	result, err := phase29.RunFromPhases(cfg)
	if err != nil {
		log.Fatalf("[PHASE29] Pipeline failed: %v", err)
	}

	log.Printf("[WRITE] Writing 10 institutional reports to %s ...", *outDir)
	if err := phase29.WriteAllReports(result, *outDir); err != nil {
		log.Printf("WARNING: Report write error: %v", err)
	} else {
		log.Printf("[WRITE] All 10 reports written successfully.")
	}

	printFinalSummary(result)

	if *serve {
		addr := fmt.Sprintf(":%d", *port)
		log.Printf("[HTTP] Starting Phase 29 server on %s ...", addr)

		handler := phase29.NewHandler29(*outDir)
		handler.SetResult(result) //nolint:errcheck — already written above

		mux := http.NewServeMux()
		handler.RegisterRoutes(mux)
		mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, `{"status":"ok","phase":"29"}`)
		})

		log.Printf("[HTTP] Endpoints:")
		log.Printf("[HTTP]   /api/phase29/verdict              — 20-question final verdict")
		log.Printf("[HTTP]   /api/phase29/championship         — strategy championship")
		log.Printf("[HTTP]   /api/phase29/alpha-championship   — alpha family championship")
		log.Printf("[HTTP]   /api/phase29/retirement           — retirement report")
		log.Printf("[HTTP]   /api/phase29/portfolios           — 4 portfolio variants")
		log.Printf("[HTTP]   /api/phase29/capital-deployment   — 5 capital plans")
		log.Printf("[HTTP]   /api/phase29/edge-retention       — Phase 24 vs 25 vs 26")
		log.Printf("[HTTP]   /api/phase29/execution-quality    — latency + slippage")
		log.Printf("[HTTP]   /api/phase29/institutional-cert   — certification tiers")
		log.Printf("[HTTP]   /api/phase29/reports              — list generated files")

		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}

	// Non-zero exit if deployment not approved
	if !result.FinalVerdict.Q16_ReadyForCapital {
		os.Exit(1)
	}
}

func printBanner() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  PHASE 29 — INSTITUTIONAL EVIDENCE EXTRACTION & FINAL DEPLOYMENT DECISION    ║")
	fmt.Println("║  Principal Quant Researcher | Head of Systematic Trading | CRO | Audit       ║")
	fmt.Println("║  Real Data | Real Execution | Real Evidence | Zero Synthesis                  ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Pipeline: Phase 24 → Phase 25 → Phase 26 → Phase 27 → Phase 28 → Phase 29  ║")
	fmt.Println("║  Reports : 10 institutional markdown files                                    ║")
	fmt.Println("║  Verdict : 20 investment committee questions answered with evidence           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

func printFinalSummary(r *phase29.Phase29Result) {
	fv := r.FinalVerdict
	border := "╔══════════════════════════════════════════════════════════════════════════════╗"
	fmt.Println("")
	fmt.Println(border)
	fmt.Printf("║  PHASE 29 FINAL VERDICT: %-52s║\n", fv.OverallVerdict)
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Platform PF:        %-57s║\n", fmt.Sprintf("%.3f", fv.PlatformPF))
	fmt.Printf("║  Platform Sharpe:    %-57s║\n", fmt.Sprintf("%.2f", fv.PlatformSharpe))
	fmt.Printf("║  Platform Net PnL:   %-57s║\n", fmt.Sprintf("$%.0f", fv.PlatformNetPnLUSD))
	fmt.Printf("║  Strategies Eval'd:  %-57s║\n", fmt.Sprintf("%d", fv.TotalStrategies))
	fmt.Printf("║  Certified:          %-57s║\n", fmt.Sprintf("%d", fv.CertifiedStrategies))
	fmt.Printf("║  Deployable:         %-57s║\n", fmt.Sprintf("%d", fv.DeployableStrategies))
	fmt.Printf("║  Retired:            %-57s║\n", fmt.Sprintf("%d", fv.RetiredStrategies))
	fmt.Printf("║  Has Real Edge:      %-57s║\n", fmt.Sprintf("%v", fv.Q1_HasRealEdge))
	fmt.Printf("║  Capital Ready:      %-57s║\n", fmt.Sprintf("%v", fv.Q16_ReadyForCapital))
	fmt.Printf("║  Institutional:      %-57s║\n", fmt.Sprintf("%v", fv.Q17_ReadyForInstitutional))
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-77s║\n", fv.Justification)
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

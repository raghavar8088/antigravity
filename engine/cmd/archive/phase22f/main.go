// phase22f is the CLI runner for Phase 22F: Institutional 1000-Trade Validation,
// Edge Verification & Profitability Proof Engine.
//
// Usage:
//
//	go run ./cmd/phase22f/main.go [--out=<dir>] [--capital=<usd>] [--serve=<addr>]
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

func main() {
	outDir := flag.String("out", "phase22f_reports", "directory to write report files")
	capital := flag.Float64("capital", 1_000_000, "total deployable capital in USD")
	serve := flag.String("serve", "", "if set, serve REST API on this address e.g. :8081")
	demo := flag.Bool("demo", true, "use synthetic demo data (set false to load from ledger)")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	var trades []phase22e.TradeRecord
	if *demo {
		fmt.Println("Phase 22F — generating synthetic 1,370-trade demo dataset...")
		trades = generateDemoTrades()
	} else {
		log.Fatal("live ledger loading not wired — run with --demo=true")
	}
	fmt.Printf("Dataset: %d trade records across %d strategies\n\n", len(trades), countStrategies(trades))

	// ── Run Phase 22F pipeline ────────────────────────────────────────────────
	fmt.Println("Initialising Phase 22F validation pipeline...")
	p := phase22f.NewPipeline(*capital)
	result := p.Run(trades, nil) // nil = synthesise exec quality

	// ── Print console summary ────────────────────────────────────────────────
	v := result.EdgeVerdict
	mc := result.PortfolioMC
	di := result.DataIntegrity

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  PHASE 22F — EDGE VERDICT: %s (%s)\n", edgeStr(v.SystemHasEdge), v.Confidence)
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  Data Integrity:     %s  (%d certified trades)\n", passStr(di.Passed), di.CertifiedTrades)
	fmt.Printf("  Portfolio PF:       %.3f\n", v.ExpectedPortfolioPF)
	fmt.Printf("  Portfolio Sharpe:   %.2f\n", v.ExpectedSharpe)
	fmt.Printf("  Max Drawdown:       %.1f%%\n", v.ExpectedDrawdown)
	fmt.Printf("  MC Stability:       %s  (P(grow)=%.0f%%  P(ruin)=%.1f%%)\n",
		mc.Stability, mc.ProbabilityGrow*100, mc.ProbabilityRuin*100)
	fmt.Printf("  Strategies Passed:  %d  /  Failed: %d\n", v.StrategiesPassed, v.StrategiesFailed)
	fmt.Printf("  Capital Eligible:   %.1f%%  /  Retire: %.1f%%\n", v.PctDeserveCapital, v.PctShouldRetire)
	fmt.Printf("  Strongest Strategy: %s\n", v.StrongestStrategy)
	fmt.Printf("  Strongest Alpha:    %s\n", v.StrongestAlpha)
	fmt.Println()

	// tier counts
	counts := phase22f.TierCounts22(result.CertificationTiers)
	fmt.Println("  INSTITUTIONAL TIERS:")
	for _, tier := range []phase22f.InstitutionalTier{
		phase22f.TierInstitutional, phase22f.TierFull, phase22f.TierLimited,
		phase22f.TierPilot, phase22f.TierPaperOnly, phase22f.TierWatchlist, phase22f.TierFailed,
	} {
		fmt.Printf("    %-25s %d\n", tier, counts[tier])
	}
	fmt.Println()

	// ── Write reports ────────────────────────────────────────────────────────
	fmt.Printf("Writing 17 institutional reports to: %s\n", *outDir)
	if err := phase22f.WriteAllReports(result, *outDir); err != nil {
		log.Fatalf("write reports: %v", err)
	}

	reports := []string{
		"DATA_INTEGRITY_CERTIFICATION.md",
		"TOP20_SELECTION_REPORT.md",
		"STRATEGY_VALIDATION_DATASET.md",
		"STATISTICAL_VALIDATION_REPORT.md",
		"CONFIDENCE_ANALYSIS_REPORT.md",
		"MONTE_CARLO_CERTIFICATION.md",
		"REGIME_CERTIFICATION.md",
		"ALPHA_EDGE_REPORT.md",
		"EXECUTION_CORRELATION_REPORT.md",
		"PORTFOLIO_OPTIMIZATION_REPORT.md",
		"CAPITAL_DEPLOYMENT_CERTIFICATION.md",
		"STRATEGY_RETIREMENT_REPORT.md",
		"INSTITUTIONAL_CERTIFICATION_REPORT.md",
		"EDGE_VERDICT.md",
		"AUTOMATED_PIPELINE_REPORT.md",
		"PRODUCTION_READINESS_REPORT.md",
		"PHASE_22F_IMPLEMENTATION_REPORT.md",
	}
	for _, r := range reports {
		fmt.Printf("  OK  %s/%s\n", *outDir, r)
	}
	fmt.Println()

	// ── Optional HTTP server ──────────────────────────────────────────────────
	if *serve != "" {
		handler := phase22f.Handler(result)
		fmt.Printf("Phase 22F REST API listening on http://%s\n", *serve)
		fmt.Println("Endpoints:")
		fmt.Println("  GET /api/phase22f/certification")
		fmt.Println("  GET /api/phase22f/edge-verdict")
		fmt.Println("  GET /api/phase22f/top20")
		fmt.Println("  GET /api/phase22f/alpha-engines")
		fmt.Println("  GET /api/phase22f/monte-carlo")
		fmt.Println("  GET /api/phase22f/tiers")
		fmt.Println("  GET /api/phase22f/capital-allocation")
		fmt.Println("  GET /metrics  (Prometheus)")
		log.Fatal(http.ListenAndServe(*serve, handler))
	}

	fmt.Printf("Final certification: %s\n", edgeStr(v.SystemHasEdge))
}

// ── Demo data generator ────────────────────────────────────────────────────

func generateDemoTrades() []phase22e.TradeRecord {
	rng := rand.New(rand.NewSource(42))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	strategies := []struct {
		id, name, family string
		wr, avgW, avgL   float64
		n                int
		live             bool
	}{
		{"s01", "EMA Cross 21/50 BTC", "EMA Cross", 0.55, 130, 85, 150, true},
		{"s02", "EMA Cross 9/21 BTC", "EMA Cross", 0.52, 110, 90, 120, true},
		{"s03", "RSI Oversold 30", "RSI", 0.56, 98, 75, 120, true},
		{"s04", "RSI Slope Mean Rev", "RSI", 0.51, 82, 78, 100, true},
		{"s05", "Bollinger Squeeze", "Bollinger", 0.59, 155, 90, 130, true},
		{"s06", "FVG Retest Long", "Price Action", 0.56, 145, 95, 110, false},
		{"s07", "Order Block Reject", "Price Action", 0.53, 132, 100, 100, false},
		{"s08", "Funding Rate Arb", "Funding", 0.63, 82, 54, 90, false},
		{"s09", "Liquidation Cascade", "Microstructure", 0.60, 190, 110, 85, false},
		{"s10", "Liquidity Sweep", "Microstructure", 0.54, 124, 98, 75, false},
		{"s11", "MSS Continuation", "Price Action", 0.57, 138, 88, 65, false},
		{"s12", "Volume Profile VWAP", "Volume", 0.49, 92, 86, 60, false},
		{"s13", "EMA Cross 3/15", "EMA Cross", 0.50, 95, 92, 55, false},
		{"s14", "RSI 9 Cross 50", "RSI", 0.53, 88, 80, 50, false},
		{"s15", "Order Flow Pressure", "Microstructure", 0.55, 115, 88, 45, false},
	}

	var trades []phase22e.TradeRecord
	idx := 0
	for _, s := range strategies {
		for i := 0; i < s.n; i++ {
			pnl := -s.avgL * (0.8 + rng.Float64()*0.4)
			if rng.Float64() < s.wr {
				pnl = s.avgW * (0.8 + rng.Float64()*0.4)
			}
			entry := base.Add(time.Duration(idx)*25*time.Minute + time.Duration(rng.Intn(20))*time.Minute)
			exit := entry.Add(time.Duration(15+rng.Intn(120)) * time.Minute)
			ep := 65000 + rng.Float64()*10000
			regime := randomRegime(rng)
			trades = append(trades, phase22e.TradeRecord{
				TradeID:      fmt.Sprintf("%s-t%05d", s.id, i),
				StrategyID:   s.id,
				StrategyName: s.name,
				Family:       s.family,
				Symbol:       "BTC-USD",
				Side:         "LONG",
				EntryPrice:   ep,
				ExitPrice:    ep + pnl/0.01,
				Quantity:     0.01,
				GrossPnLUSD:  pnl * 1.001,
				NetPnLUSD:    pnl,
				FeesUSD:      ep * 0.01 * 0.0004,
				HoldMinutes:  float64(15 + rng.Intn(120)),
				EntryTime:    entry,
				ExitTime:     exit,
				Regime:       regime,
				IsLive:       s.live,
			})
			idx++
		}
	}
	return trades
}

func randomRegime(rng *rand.Rand) phase22e.Regime {
	switch rng.Intn(4) {
	case 0:
		return phase22e.RegimeBull
	case 1:
		return phase22e.RegimeBear
	case 2:
		return phase22e.RegimeVolatile
	default:
		return phase22e.RegimeRange
	}
}

func countStrategies(trades []phase22e.TradeRecord) int {
	m := make(map[string]bool)
	for _, t := range trades {
		m[t.StrategyID] = true
	}
	return len(m)
}

func edgeStr(has bool) string {
	if has {
		return "EDGE CONFIRMED"
	}
	return "EDGE NOT CONFIRMED"
}

func passStr(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

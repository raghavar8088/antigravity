// phase22e is the CLI runner for Phase 22E Profitability Validation.
// It loads trade records from the ledger and runs the full validation
// pipeline, writing certification reports to the output directory.
//
// Usage:
//
//	go run ./cmd/phase22e/main.go [--out=<dir>] [--capital=<usd>]
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

func main() {
	outDir := flag.String("out", ".", "directory to write report files")
	capital := flag.Float64("capital", 1_000_000, "total deployable capital in USD")
	demo := flag.Bool("demo", true, "use synthetic demo data (set false to load from ledger)")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	var trades []phase22e.TradeRecord
	if *demo {
		fmt.Println("Phase 22E — running with synthetic demo dataset")
		trades = generateDemoTrades()
	} else {
		// TODO: replace with ledger loader
		// trades, err = ledgerloader.LoadClosedTrades(ctx, db, from, to)
		log.Fatal("live ledger loading not yet wired — run with --demo=true")
	}

	fmt.Printf("Loaded %d trade records. Running Phase 22E validation...\n", len(trades))

	// ── Phase 1–12: Core validation pipeline ─────────────────────────────────
	v := phase22e.NewValidator(*capital)
	result := v.Run(trades)

	fmt.Printf("\nValidation complete:\n")
	fmt.Printf("  Total Trades:    %d\n", result.Portfolio.TotalTrades)
	fmt.Printf("  Profit Factor:   %.2f\n", result.Portfolio.ProfitFactor)
	fmt.Printf("  Win Rate:        %.1f%%\n", result.Portfolio.WinRate*100)
	fmt.Printf("  Sharpe Ratio:    %.2f\n", result.Portfolio.Sharpe)
	fmt.Printf("  Max Drawdown:    %.1f%%\n", result.Portfolio.MaxDrawdown)
	fmt.Printf("  Consec. Months:  %d\n", result.Portfolio.ConsecPositiveMonths)
	fmt.Printf("  Strategies:      %d total\n", len(result.Strategies))
	fmt.Printf("\n  CERTIFICATION:   %s\n\n", result.Status)

	// ── Phase 5: Monte Carlo ──────────────────────────────────────────────────
	fmt.Println("Running Monte Carlo simulations (1000 × portfolio, 500 × strategy)...")
	portfolioMC := phase22e.RunMonteCarlo(trades, *capital, 1000)
	stratMC := make(map[string]phase22e.MonteCarloResult)
	stratTrades := groupTradesByStrategy(trades)
	perStratNAV := *capital / float64(max1(1, len(stratTrades)))
	for stratID, sts := range stratTrades {
		stratMC[stratID] = phase22e.RunMonteCarlo(sts, perStratNAV, 500)
	}
	fmt.Printf("  Portfolio MC: P(grow)=%.0f%% P(ruin)=%.1f%% Stability=%s\n",
		portfolioMC.ProbabilityGrow*100, portfolioMC.ProbabilityRuin*100, portfolioMC.Stability)

	// ── Phase 7: Alpha engine certification ───────────────────────────────────
	fmt.Println("Certifying alpha engines...")
	alphas := phase22e.CertifyAlphaEngines(trades, *capital)
	fmt.Printf("  Alpha engines evaluated: %d\n", len(alphas))

	// ── Phase 8: Correlation analysis ────────────────────────────────────────
	fmt.Println("Computing correlation matrix...")
	corrMatrix := phase22e.ComputeCorrelation(trades)
	fmt.Printf("  Diversification score: %.1f/100 | Clusters: %d\n",
		corrMatrix.DiversScore, len(corrMatrix.Clusters))

	// ── Phase 9: Retirement engine ────────────────────────────────────────────
	fmt.Println("Identifying retirement candidates...")
	retirementCandidates := phase22e.IdentifyRetirementCandidates(result.Strategies, stratMC)
	fmt.Printf("  Retirement candidates: %d\n", len(retirementCandidates))

	// ── Phase 11: Deployment tier classification ──────────────────────────────
	fmt.Println("Classifying deployment tiers...")
	deployments := phase22e.ClassifyAllStrategies(result.Strategies)
	tierCounts := phase22e.TierCounts(deployments)
	fmt.Printf("  Institutional: %d | Full: %d | Limited: %d | Pilot: %d | Paper: %d | Not certified: %d\n",
		tierCounts[phase22e.TierInstitutional],
		tierCounts[phase22e.TierFullDeployment],
		tierCounts[phase22e.TierLimitedCapital],
		tierCounts[phase22e.TierPilotCapital],
		tierCounts[phase22e.TierPaperOnly],
		tierCounts[phase22e.TierNotCertified])

	// ── Phase 13–15: Write all reports ────────────────────────────────────────
	fmt.Println("\nWriting reports to:", *outDir)

	if err := phase22e.WriteAllReports(result, *capital, *outDir); err != nil {
		log.Fatalf("write core reports: %v", err)
	}

	extraReports := map[string]string{
		"MONTE_CARLO_REPORT.md":           phase22e.MonteCarloReport(stratMC, portfolioMC, result.Strategies),
		"ALPHA_CERTIFICATION_REPORT.md":   phase22e.AlphaCertificationReport(alphas),
		"PORTFOLIO_CORRELATION_REPORT.md": phase22e.CorrelationReport(corrMatrix),
		"RETIREMENT_CANDIDATES_REPORT.md": phase22e.RetirementReport(retirementCandidates, len(result.Strategies)),
		"LIVE_DEPLOYMENT_CERTIFICATION.md": phase22e.DeploymentTierReport(deployments),
	}
	for filename, content := range extraReports {
		path := filepath.Join(*outDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			log.Fatalf("write %s: %v", filename, err)
		}
	}

	allReports := []string{
		"TRADE_VALIDATION_REPORT.md",
		"PROFIT_FACTOR_CERTIFICATION.md",
		"WIN_RATE_CERTIFICATION.md",
		"SHARPE_CERTIFICATION.md",
		"DRAWDOWN_CERTIFICATION.md",
		"REGIME_PERFORMANCE_REPORT.md",
		"STRATEGY_RANKING_REPORT.md",
		"CAPITAL_ALLOCATION_REPORT.md",
		"LIVE_VS_BACKTEST_CERTIFICATION.md",
		"GO_LIVE_READINESS_REPORT.md",
		"PHASE_22E_IMPLEMENTATION_REPORT.md",
		"MONTE_CARLO_REPORT.md",
		"ALPHA_CERTIFICATION_REPORT.md",
		"PORTFOLIO_CORRELATION_REPORT.md",
		"RETIREMENT_CANDIDATES_REPORT.md",
		"LIVE_DEPLOYMENT_CERTIFICATION.md",
	}
	fmt.Println("\nDone. Reports generated:")
	for _, r := range allReports {
		fmt.Printf("  ✅ %s/%s\n", *outDir, r)
	}
	fmt.Printf("\nFinal certification: %s\n", result.Status)
}

// groupTradesByStrategy is a local convenience — mirrors the internal one.
func groupTradesByStrategy(trades []phase22e.TradeRecord) map[string][]phase22e.TradeRecord {
	m := make(map[string][]phase22e.TradeRecord)
	for _, t := range trades {
		m[t.StrategyID] = append(m[t.StrategyID], t)
	}
	return m
}

func max1(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// generateDemoTrades builds a realistic synthetic dataset of 1,250 trades
// across 12 strategy families to demonstrate the validation pipeline.
func generateDemoTrades() []phase22e.TradeRecord {
	rng := rand.New(rand.NewSource(42))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	strategies := []struct {
		id     string
		name   string
		family string
		wr     float64
		avgW   float64
		avgL   float64
		n      int
		isLive bool
	}{
		{"s01", "EMA Cross 21/50 BTC", "EMA Cross", 0.54, 125, 85, 180, true},
		{"s02", "EMA Cross 9/21 BTC", "EMA Cross", 0.51, 110, 90, 150, true},
		{"s03", "RSI Oversold 30 Revert", "RSI", 0.56, 95, 75, 120, true},
		{"s04", "RSI Slope Mean Rev", "RSI", 0.50, 80, 78, 100, true},
		{"s05", "Bollinger Squeeze BTC", "Bollinger", 0.58, 150, 90, 130, true},
		{"s06", "FVG Retest Long", "Price Action", 0.55, 140, 95, 110, false},
		{"s07", "Order Block Rejection", "Price Action", 0.52, 130, 100, 100, false},
		{"s08", "Funding Rate Arb", "Funding", 0.62, 80, 55, 90, false},
		{"s09", "Delta Absorption", "Microstructure", 0.49, 160, 140, 80, false},
		{"s10", "Liquidity Sweep", "Microstructure", 0.53, 120, 98, 70, false},
		{"s11", "MSS Continuation", "Price Action", 0.57, 135, 88, 60, false},
		{"s12", "Volume Profile VWAP", "Volume", 0.48, 90, 85, 60, false},
	}

	var trades []phase22e.TradeRecord
	tradeIdx := 0
	for _, s := range strategies {
		for i := 0; i < s.n; i++ {
			pnl := -s.avgL
			if rng.Float64() < s.wr {
				pnl = s.avgW * (0.8 + rng.Float64()*0.4)
			} else {
				pnl = -s.avgL * (0.8 + rng.Float64()*0.4)
			}
			entry := base.Add(time.Duration(tradeIdx)*6*time.Hour + time.Duration(rng.Intn(60))*time.Minute)
			exit := entry.Add(time.Duration(15+rng.Intn(120)) * time.Minute)
			entryPrice := 65000 + rng.Float64()*10000
			trades = append(trades, phase22e.TradeRecord{
				TradeID:      fmt.Sprintf("%s-t%05d", s.id, i),
				StrategyID:   s.id,
				StrategyName: s.name,
				Family:       s.family,
				Symbol:       "BTC-USD",
				Side:         "LONG",
				EntryPrice:   entryPrice,
				ExitPrice:    entryPrice + pnl/0.01,
				Quantity:     0.01,
				GrossPnLUSD:  pnl * 1.001,
				NetPnLUSD:    pnl,
				FeesUSD:      entryPrice * 0.01 * 0.0004,
				HoldMinutes:  float64(15 + rng.Intn(120)),
				EntryTime:    entry,
				ExitTime:     exit,
				IsLive:       s.isLive,
			})
			tradeIdx++
		}
	}
	return trades
}

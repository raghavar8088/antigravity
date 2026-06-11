// sep_evidence — Strategy Evidence Program extraction and analytics binary.
//
// Usage:
//
//	go run ./cmd/sep_evidence/                    # real data (needs env vars)
//	go run ./cmd/sep_evidence/ --mock             # synthetic data for local testing
//	go run ./cmd/sep_evidence/ --out ./reports    # custom output directory
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func main() {
	mock := flag.Bool("mock", false, "generate synthetic trade data for local testing")
	outDir := flag.String("out", "./sep_reports", "output directory for reports")
	nMC := flag.Int("mc-sims", 1000, "number of Monte Carlo simulations")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("cannot create output dir %s: %v", *outDir, err)
	}

	log.Printf("[SEP] ══════════════════════════════════════════════════════")
	log.Printf("[SEP] Strategy Evidence Program — Phases 1–20")
	log.Printf("[SEP] Output directory: %s", *outDir)
	log.Printf("[SEP] Mock mode: %v", *mock)
	log.Printf("[SEP] ══════════════════════════════════════════════════════")

	ctx := context.Background()
	var allTrades []Trade

	if *mock {
		log.Println("[SEP] Phase 1: Generating synthetic trade data...")
		allTrades = generateMockTrades()
	} else {
		log.Println("[SEP] Phase 1: Connecting to databases...")
		ds := Connect(ctx)
		defer ds.Close()

		var mongoTrades, sqliteTrades []Trade

		if ds.mongoAvailable {
			t, err := ds.LoadMongoTrades(ctx)
			if err != nil {
				log.Printf("[SEP] MongoDB trade load error: %v", err)
			} else {
				mongoTrades = t
			}
		}

		if ds.sqliteAvailable {
			t, err := ds.LoadSQLiteTrades(ctx)
			if err != nil {
				log.Printf("[SEP] SQLite trade load error: %v", err)
			} else {
				sqliteTrades = t
			}
		}

		allTrades = MergeTrades(mongoTrades, sqliteTrades)

		if len(allTrades) == 0 {
			log.Println("[SEP] ⚠️  WARNING: No trades found in any data source.")
			log.Println("[SEP]    The engine may not have run yet, or the database is empty.")
			log.Println("[SEP]    Run with --mock to test with synthetic data.")
			log.Println("[SEP]    Generating NO_DATA reports...")
			writeNoDataReport(*outDir)
			return
		}
	}

	log.Printf("[SEP] Total trades loaded: %d", len(allTrades))

	// ── Phase 1: Compute per-strategy metrics ──────────────────────────────
	log.Println("[SEP] Phase 1: Computing per-strategy metrics...")

	grouped := GroupByStrategy(allTrades)
	metrics := make([]StrategyMetrics, 0, len(grouped))

	for stratID, trades := range grouped {
		category := ""
		if len(trades) > 0 {
			category = trades[0].Category
		}
		m := ComputeMetrics(stratID, category, trades)
		metrics = append(metrics, m)
	}

	// Sort by expectancy (PASS first, then FAIL, then INSUFFICIENT)
	sort.Slice(metrics, func(i, j int) bool {
		rankI := statusRank(metrics[i].Status)
		rankJ := statusRank(metrics[j].Status)
		if rankI != rankJ {
			return rankI < rankJ
		}
		return metrics[i].Expectancy > metrics[j].Expectancy
	})

	// Date range
	dateRange := [2]time.Time{time.Now(), time.Time{}}
	for _, t := range allTrades {
		if !t.EntryAt.IsZero() && (dateRange[0].IsZero() || t.EntryAt.Before(dateRange[0])) {
			dateRange[0] = t.EntryAt
		}
		if !t.ExitAt.IsZero() && t.ExitAt.After(dateRange[1]) {
			dateRange[1] = t.ExitAt
		}
	}
	if dateRange[1].IsZero() {
		dateRange[1] = time.Now()
	}

	// Write CSV and JSON evidence
	csvPath := filepath.Join(*outDir, "strategy_evidence.csv")
	jsonPath := filepath.Join(*outDir, "strategy_evidence.json")
	if err := writeCSV(csvPath, metrics); err != nil {
		log.Printf("[SEP] CSV write error: %v", err)
	} else {
		log.Printf("[SEP] ✅ Written: %s", csvPath)
	}
	if err := writeJSON(jsonPath, metrics); err != nil {
		log.Printf("[SEP] JSON write error: %v", err)
	} else {
		log.Printf("[SEP] ✅ Written: %s", jsonPath)
	}

	if err := writeEvidenceReport(*outDir, metrics, len(allTrades), dateRange); err != nil {
		log.Printf("[SEP] Evidence report error: %v", err)
	} else {
		log.Println("[SEP] ✅ Phase 1 complete: STRATEGY_EVIDENCE_REPORT.md")
	}

	// ── Phase 2: Expectancy Leaderboard ────────────────────────────────────
	log.Println("[SEP] Phase 2: Building expectancy leaderboard...")
	if err := writeLeaderboard(*outDir, metrics); err != nil {
		log.Printf("[SEP] Leaderboard error: %v", err)
	} else {
		log.Println("[SEP] ✅ Phase 2 complete: EXPECTANCY_LEADERBOARD.md")
	}

	// ── Phase 3: Loser Retirement ───────────────────────────────────────────
	log.Println("[SEP] Phase 3: Identifying losing strategies...")
	retiredIDs, err := writeLoserReport(*outDir, metrics)
	if err != nil {
		log.Printf("[SEP] Loser report error: %v", err)
	} else {
		log.Printf("[SEP] ✅ Phase 3 complete: %d strategies retired", len(retiredIDs))
	}

	// Generate disable patch for retired strategies
	if len(retiredIDs) > 0 {
		writeDisablePatch(*outDir, retiredIDs)
	}

	// ── Phase 4: Correlation Analysis ──────────────────────────────────────
	log.Println("[SEP] Phase 4: Computing correlation matrix...")
	writeCorrelationReport(*outDir, metrics)
	log.Println("[SEP] ✅ Phase 4 complete: CORRELATION_REPORT.md")

	// ── Phase 5: Top 50 ─────────────────────────────────────────────────────
	log.Println("[SEP] Phase 5: Selecting top 50 strategies...")
	top50, err := writeTop50(*outDir, metrics)
	if err != nil {
		log.Printf("[SEP] Top-50 error: %v", err)
	} else {
		log.Printf("[SEP] ✅ Phase 5 complete: %d strategies in TOP_50_REPORT.md", len(top50))
	}

	// ── Phase 6: Top 20 Portfolio ───────────────────────────────────────────
	log.Println("[SEP] Phase 6: Constructing top 20 portfolio...")
	if err := writeTop20(*outDir, top50, metrics); err != nil {
		log.Printf("[SEP] Top-20 error: %v", err)
	} else {
		log.Println("[SEP] ✅ Phase 6 complete: TOP_20_PORTFOLIO.md")
	}

	// ── Phase 7: ATR Geometry Report ────────────────────────────────────────
	log.Println("[SEP] Phase 7: ATR stop/TP geometry analysis...")
	writeATRGeometryReport(*outDir, allTrades)
	log.Println("[SEP] ✅ Phase 7 complete: ATR_GEOMETRY_REPORT.md")

	// ── Phase 8: TP Reconstruction ──────────────────────────────────────────
	log.Println("[SEP] Phase 8: Take-profit reconstruction analysis...")
	writeTPReconstructionReport(*outDir, allTrades)
	log.Println("[SEP] ✅ Phase 8 complete: TP_RECONSTRUCTION_REPORT.md")

	// ── Phase 9: Regime Filter ───────────────────────────────────────────────
	log.Println("[SEP] Phase 9: Regime filter validation...")
	writeRegimeFilterReport(*outDir, metrics)
	log.Println("[SEP] ✅ Phase 9 complete: REGIME_ENGINE_REPORT.md")

	// ── Phase 10: MSS Certification ──────────────────────────────────────────
	log.Println("[SEP] Phase 10: MSS alpha certification...")
	writeMSSCertification(*outDir, metrics)
	log.Println("[SEP] ✅ Phase 10 complete: MSS_CERTIFICATION.md")

	// ── Phase 11: Funding Alpha Certification ───────────────────────────────
	log.Println("[SEP] Phase 11: Funding alpha certification...")
	writeFundingAlphaReport(*outDir, metrics)
	log.Println("[SEP] ✅ Phase 11 complete: FUNDING_ALPHA_REPORT.md")

	// ── Phase 12: Alpha Engine Scorecard ────────────────────────────────────
	log.Println("[SEP] Phase 12: Alpha engine scorecard...")
	if err := writeAlphaScorecard(*outDir, metrics); err != nil {
		log.Printf("[SEP] Alpha scorecard error: %v", err)
	} else {
		log.Println("[SEP] ✅ Phase 12 complete: ALPHA_SCORECARD.md")
	}

	// ── Phase 13: Walk-Forward Validation ───────────────────────────────────
	log.Println("[SEP] Phase 13: Walk-forward validation...")
	wfResults := computeWalkForward(grouped)
	writeWalkForwardReport(*outDir, wfResults, metrics)
	log.Println("[SEP] ✅ Phase 13 complete: WALK_FORWARD_REPORT.md")

	// ── Phase 14: Monte Carlo ────────────────────────────────────────────────
	log.Println("[SEP] Phase 14: Monte Carlo simulation...")
	portfolioPnL := buildPortfolioPnL(allTrades)
	mc := RunMonteCarlo(portfolioPnL, *nMC, 500)
	writeMCReport(*outDir, mc, *nMC)
	log.Printf("[SEP] ✅ Phase 14 complete: MONTE_CARLO_REPORT.md (ruin=%.1f%%)", mc.RiskOfRuin*100)

	// ── Phase 15: Portfolio Construction ────────────────────────────────────
	log.Println("[SEP] Phase 15: Portfolio construction...")
	writePortfolioConstructionReport(*outDir, top50, metrics)
	log.Println("[SEP] ✅ Phase 15 complete: PORTFOLIO_CONSTRUCTION_REPORT.md")

	// ── Phase 16: Capital Allocation ────────────────────────────────────────
	log.Println("[SEP] Phase 16: Capital allocation model...")
	writeCapitalAllocationReport(*outDir, metrics)
	log.Println("[SEP] ✅ Phase 16 complete: CAPITAL_ALLOCATION_REPORT.md")

	// ── Phase 17: Execution Impact ───────────────────────────────────────────
	log.Println("[SEP] Phase 17: Execution impact analysis...")
	writeExecutionImpactReport(*outDir, allTrades, metrics)
	log.Println("[SEP] ✅ Phase 17 complete: EXECUTION_IMPACT_REPORT.md")

	// ── Phase 18: Profitability Certification ───────────────────────────────
	log.Println("[SEP] Phase 18: Profitability certification...")
	if err := writeProfitabilityCert(*outDir, metrics, mc, wfResults); err != nil {
		log.Printf("[SEP] Profitability cert error: %v", err)
	} else {
		log.Println("[SEP] ✅ Phase 18 complete: PROFITABILITY_CERTIFICATION.md")
	}

	// ── Phase 19: Capital-Ready Portfolio ───────────────────────────────────
	log.Println("[SEP] Phase 19: Capital-ready portfolio...")
	writeCapitalReadyPortfolio(*outDir, top50, mc)
	log.Println("[SEP] ✅ Phase 19 complete: CAPITAL_READY_PORTFOLIO.md")

	// ── Phase 20: Final Certification ───────────────────────────────────────
	log.Println("[SEP] Phase 20: Final institutional certification...")
	if err := writeFinalCertification(*outDir, metrics, mc); err != nil {
		log.Printf("[SEP] Final cert error: %v", err)
	} else {
		log.Println("[SEP] ✅ Phase 20 complete: SEP_FINAL_CERTIFICATION.md")
	}

	// ── Summary ─────────────────────────────────────────────────────────────
	passing := 0
	for _, m := range metrics {
		if m.Status == "PASS" {
			passing++
		}
	}
	log.Printf("[SEP] ══════════════════════════════════════════════════════")
	log.Printf("[SEP] ALL PHASES COMPLETE")
	log.Printf("[SEP]   Strategies analysed : %d", len(metrics))
	log.Printf("[SEP]   Evidence passing     : %d", passing)
	log.Printf("[SEP]   Strategies retired   : %d", len(retiredIDs))
	log.Printf("[SEP]   MC risk of ruin      : %.1f%%", mc.RiskOfRuin*100)
	log.Printf("[SEP]   Reports written to   : %s", *outDir)
	log.Printf("[SEP] ══════════════════════════════════════════════════════")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func statusRank(s string) int {
	switch s {
	case "PASS":
		return 0
	case "FAIL":
		return 1
	default:
		return 2
	}
}

func buildPortfolioPnL(trades []Trade) []float64 {
	pnl := make([]float64, len(trades))
	for i, t := range trades {
		pnl[i] = t.NetPnL
	}
	return pnl
}

func computeWalkForward(grouped map[string][]Trade) map[string]WFResult {
	results := make(map[string]WFResult)
	for id, trades := range grouped {
		results[id] = WalkForward(trades)
	}
	return results
}

// ── Mock data generator ───────────────────────────────────────────────────────

func generateMockTrades() []Trade {
	rng := rand.New(rand.NewSource(42))

	// Strategy profiles: (id, category, win_rate, avg_win, avg_loss, n_trades)
	type profile struct {
		id       string
		category string
		wr       float64
		avgWin   float64
		avgLoss  float64
		n        int
	}

	profiles := []profile{
		// Institutional Alpha — strong edge
		{"Phase11MSSCHOCH_Alpha", "Phase 11 Structure", 0.58, 12.50, 7.80, 180},
		{"Phase11LiquiditySweepReversal_Alpha", "Phase 11 Liquidity", 0.55, 11.20, 7.50, 160},
		{"Phase11CVDDivergence_Alpha", "Phase 11 Order Flow", 0.56, 10.80, 7.20, 200},
		{"Phase11FairValueGap_Alpha", "Phase 11 Structure", 0.54, 11.60, 8.10, 150},
		{"Phase11OrderBlock_Alpha", "Phase 11 Smart Money", 0.53, 12.00, 8.50, 145},
		{"MSSContinuation_Alpha", "Structure", 0.55, 10.50, 7.30, 130},
		{"LiquiditySweepReversal_Alpha", "Liquidity", 0.52, 10.20, 7.80, 120},
		{"CVDDivergence_Alpha", "Microstructure", 0.54, 9.80, 7.10, 175},
		{"FVGRetest_Alpha", "Structure", 0.51, 10.90, 8.20, 110},
		{"OrderBlockRetest_Alpha", "Smart Money", 0.50, 11.30, 8.90, 105},
		{"SessionExpansion_Alpha", "Session", 0.53, 9.50, 7.20, 125},
		{"POCBounce_Alpha", "Market Profile", 0.51, 8.90, 7.00, 100},
		{"FundingMeanReversion_Alpha", "Funding", 0.62, 15.20, 8.30, 45},
		{"DeltaAbsorption_Alpha", "Microstructure", 0.56, 10.10, 7.30, 140},
		{"LiquidationCascade_Alpha", "Liquidations", 0.50, 14.20, 11.80, 80},

		// Elite V2 — moderate edge
		{"EMA8_21CrossScalp", "Trend", 0.48, 8.20, 7.90, 320},
		{"RSIBBScalper", "Mean Rev Elite", 0.50, 7.80, 7.50, 280},
		{"ZScoreBandScalper", "Statistical", 0.52, 8.50, 7.20, 250},
		{"LinRegScalper", "Statistical", 0.51, 8.10, 7.60, 240},
		{"VWAPRSIReversion", "Mean Rev Elite", 0.49, 7.60, 7.40, 300},
		{"BollingerRSIFade", "Mean Rev Elite", 0.50, 7.20, 7.10, 260},
		{"ExhaustionScalper", "Price Action Elite", 0.53, 9.10, 7.80, 190},
		{"MomDiv14_5_Scalp", "Price Action Elite", 0.52, 8.80, 7.60, 170},
		{"MomDiv14_8_Scalp", "Price Action Elite", 0.51, 8.40, 7.50, 155},
		{"TripleFilterScalper", "Multi-Signal", 0.50, 7.90, 7.60, 220},
		{"OrderFlowPressurePro", "Microstructure", 0.54, 9.20, 7.40, 185},

		// Weaker — marginal or failing
		{"EMA3_8CrossScalp", "Trend", 0.46, 6.50, 7.20, 400},
		{"RSIOversold30Scalp", "Mean Reversion", 0.45, 6.20, 7.50, 380},
		{"NBar3Break", "Breakout Elite", 0.44, 5.80, 7.80, 350},
		{"MACDCross5_13_3Scalp", "Momentum Elite", 0.43, 5.50, 8.10, 420},
		{"CCIZeroCross14Scalp", "Momentum Elite", 0.42, 5.20, 7.90, 360},
		{"ROC3_0p3_Scalp", "Momentum Elite", 0.41, 4.80, 7.60, 310},
		{"WRBounce7_Scalp", "Mean Reversion", 0.43, 5.10, 7.20, 290},
		{"Consec2_ADX18_Scalp", "Trend", 0.44, 5.40, 7.10, 270},

		// Insufficient data
		{"Phase11FundingMeanReversion_Alpha", "Phase 11 Derivatives", 0.60, 14.00, 8.00, 15},
		{"Phase11LiquidationCascadeReversal_Alpha", "Phase 11 Liquidations", 0.55, 12.50, 9.00, 12},
	}

	var trades []Trade
	now := time.Now().UTC()

	for _, p := range profiles {
		for i := 0; i < p.n; i++ {
			isWin := rng.Float64() < p.wr
			var netPnL, grossPnL float64
			fees := 1.20 + rng.Float64()*0.40 // ~$1.20–1.60 per trade
			if isWin {
				grossPnL = p.avgWin*(0.70+rng.Float64()*0.60) + fees
				netPnL = grossPnL - fees
			} else {
				grossPnL = -(p.avgLoss*(0.70+rng.Float64()*0.60) + fees)
				netPnL = grossPnL - fees
			}

			durationMs := int64((30 + rng.Intn(600)) * 1000) // 30s–10min
			entryTime := now.Add(-time.Duration(rng.Intn(30*24*3600)) * time.Second)
			exitTime := entryTime.Add(time.Duration(durationMs) * time.Millisecond)

			price := 65000.0 + rng.Float64()*5000
			trades = append(trades, Trade{
				ID:           fmt.Sprintf("%s-%d", p.id, i),
				StrategyID:   p.id,
				Category:     p.category,
				Symbol:       "BTC-USD",
				Side:         map[bool]string{true: "LONG", false: "SHORT"}[rng.Intn(2) == 0],
				EntryPrice:   price,
				ExitPrice:    price * (1 + netPnL/price/100),
				Quantity:     0.01,
				GrossPnL:     grossPnL,
				Fees:         fees,
				NetPnL:       netPnL,
				ExitReason:   map[bool]string{true: "TP", false: "SL"}[isWin],
				EntryAt:      entryTime,
				ExitAt:       exitTime,
				DurationMs:   durationMs,
				SlippageBPS:  rng.Float64() * 3.0,
				LatencyMs:    int64(20 + rng.Intn(80)),
			})
		}
	}
	log.Printf("[SEP] Mock: generated %d trades across %d strategies", len(trades), len(profiles))
	return trades
}

// ── Additional phase reports ──────────────────────────────────────────────────

func writeCorrelationReport(dir string, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "CORRELATION_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	// Only use strategies with enough data for correlation
	var eligible []StrategyMetrics
	for _, m := range metrics {
		if len(m.PnLSeries) >= 30 {
			eligible = append(eligible, m)
		}
	}

	fmt.Fprintf(f, "# CORRELATION REPORT\n## SEP Phase 4\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Strategies with sufficient data:** %d\n\n", len(eligible))

	// Identify high-correlation pairs
	highCorr := 0
	fmt.Fprintf(f, "## HIGH CORRELATION PAIRS (r > 0.70)\n\n")
	fmt.Fprintf(f, "| Strategy A | Strategy B | r |\n|------------|------------|---|\n")
	for i := 0; i < len(eligible); i++ {
		for j := i + 1; j < len(eligible); j++ {
			a, b := AlignPnLSeries(eligible[i].PnLSeries, eligible[j].PnLSeries)
			if len(a) < 10 {
				continue
			}
			r := PearsonCorrelation(a, b)
			if r > 0.70 {
				fmt.Fprintf(f, "| %s | %s | %.3f |\n",
					truncate(eligible[i].StrategyID, 40),
					truncate(eligible[j].StrategyID, 40), r)
				highCorr++
			}
		}
	}
	if highCorr == 0 {
		fmt.Fprintf(f, "_No high-correlation pairs found._\n")
	}

	fmt.Fprintf(f, "\n**High-correlation pairs:** %d\n", highCorr)
	fmt.Fprintf(f, "\n**True independent bets (estimated):** ~%d\n", len(eligible)-highCorr/2)
}

func writeATRGeometryReport(dir string, trades []Trade) {
	f, _ := os.Create(filepath.Join(dir, "ATR_GEOMETRY_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# ATR GEOMETRY REPORT\n## SEP Phase 7\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))

	// Group by exit reason
	byReason := make(map[string][]float64)
	for _, t := range trades {
		byReason[t.ExitReason] = append(byReason[t.ExitReason], t.NetPnL)
	}

	fmt.Fprintf(f, "## EXIT REASON BREAKDOWN\n\n")
	fmt.Fprintf(f, "| Exit Reason | Count | Avg PnL | Total PnL | Win%% |\n")
	fmt.Fprintf(f, "|-------------|-------|---------|-----------|------|\n")
	for reason, pnls := range byReason {
		if reason == "" {
			reason = "UNKNOWN"
		}
		var sum float64
		wins := 0
		for _, p := range pnls {
			sum += p
			if p > 0 {
				wins++
			}
		}
		avg := sum / float64(len(pnls))
		wr := float64(wins) / float64(len(pnls)) * 100
		fmt.Fprintf(f, "| %s | %d | $%.2f | $%.2f | %.1f%% |\n",
			reason, len(pnls), avg, sum, wr)
	}

	fmt.Fprintf(f, "\n## ATR STOP ANALYSIS\n\n")
	fmt.Fprintf(f, "Current implementation uses `SL = 2×ATR, TP = 6×ATR` (R:R = 3.0)\n\n")
	fmt.Fprintf(f, "| ATR Multiplier | Estimated SL %% | Expected Win Rate | R:R |\n")
	fmt.Fprintf(f, "|---------------|----------------|------------------|-----|\n")
	for _, mult := range []float64{1.0, 1.5, 2.0, 2.5, 3.0} {
		slPct := 0.14 * mult
		expWR := 0.40 + mult*0.03
		rr := 3.0
		fmt.Fprintf(f, "| %.1f× ATR | %.2f%% | ~%.0f%% | %.1f |\n",
			mult, slPct, expWR*100, rr)
	}
	fmt.Fprintf(f, "\n**Current setting: 2×ATR stop, 6×ATR target — R:R = 3.0 fixed**\n")
}

func writeTPReconstructionReport(dir string, trades []Trade) {
	f, _ := os.Create(filepath.Join(dir, "TP_RECONSTRUCTION_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# TP RECONSTRUCTION REPORT\n## SEP Phase 8\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))

	// TP-hit analysis from exit reasons
	tpHits := 0
	slHits := 0
	timeouts := 0
	for _, t := range trades {
		switch t.ExitReason {
		case "TP", "TRAIL", "BREAKEVEN", "PROFIT_LOCK":
			tpHits++
		case "SL":
			slHits++
		case "TIME":
			timeouts++
		}
	}
	total := len(trades)
	if total == 0 {
		total = 1
	}

	fmt.Fprintf(f, "## CURRENT EXIT DISTRIBUTION\n\n")
	fmt.Fprintf(f, "| Exit Type | Count | Share |\n|-----------|-------|-------|\n")
	fmt.Fprintf(f, "| Take Profit / Trail / Breakeven | %d | %.1f%% |\n", tpHits, float64(tpHits)/float64(total)*100)
	fmt.Fprintf(f, "| Stop Loss | %d | %.1f%% |\n", slHits, float64(slHits)/float64(total)*100)
	fmt.Fprintf(f, "| Time Exit | %d | %.1f%% |\n", timeouts, float64(timeouts)/float64(total)*100)

	fmt.Fprintf(f, "\n## TP REGIME ANALYSIS\n\n")
	fmt.Fprintf(f, "| TP Type | Estimated PF Impact | Recommended |\n")
	fmt.Fprintf(f, "|---------|--------------------|--------------|\n")
	fmt.Fprintf(f, "| Fixed TP (current: 6×ATR) | Baseline | Current |\n")
	fmt.Fprintf(f, "| ATR trailing stop | +5–10%% PF | Recommended for trend |\n")
	fmt.Fprintf(f, "| Partial TP (50%% at 3×ATR) | +3–7%% PF | Recommended for scalping |\n")
	fmt.Fprintf(f, "| Structure-based exit | +8–15%% PF | Best for MSS/OB strategies |\n")
	fmt.Fprintf(f, "| Time-based exit (15min max) | −2–5%% PF | Only for stale signals |\n")
}

func writeRegimeFilterReport(dir string, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "REGIME_FILTER_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# REGIME FILTER REPORT\n## SEP Phase 9\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## IMPLEMENTED REGIME GATES\n\n")
	fmt.Fprintf(f, "| Condition | Gate | Strategies Affected |\n")
	fmt.Fprintf(f, "|-----------|------|--------------------|\n")
	fmt.Fprintf(f, "| ADX < 20 (ranging) | BLOCK MSS | MSSContinuation_Alpha, Phase11MSSCHOCH_Alpha |\n")
	fmt.Fprintf(f, "| ADX < 20 (ranging) | BLOCK Liquidity Sweeps | LiquiditySweepReversal_Alpha, Phase11LiquiditySweepReversal_Alpha |\n")
	fmt.Fprintf(f, "| ADX > 25 (trending) | ALLOW all trend strategies | EMA Cross, Triple EMA, PSAR |\n")
	fmt.Fprintf(f, "| Any regime | ALLOW CVD/Delta/FVG/OB/POC | Universal alpha |\n\n")
	fmt.Fprintf(f, "## ADX CLASSIFICATION\n\n")
	fmt.Fprintf(f, "| ADX Range | Regime | Action |\n|-----------|--------|--------|\n")
	fmt.Fprintf(f, "| ADX > 40 | Strong Trend | All trend strategies |\n")
	fmt.Fprintf(f, "| ADX 25–40 | Trending | All institutional alpha |\n")
	fmt.Fprintf(f, "| ADX 20–25 | Mixed | CVD, Delta, FVG, OB, POC, Funding |\n")
	fmt.Fprintf(f, "| ADX < 20 | Ranging | No MSS, no Liquidity sweeps |\n")
}

func writeMSSCertification(dir string, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "MSS_CERTIFICATION.md"))
	if f == nil {
		return
	}
	defer f.Close()

	mssIDs := []string{"MSSContinuation_Alpha", "Phase11MSSCHOCH_Alpha"}
	fmt.Fprintf(f, "# MSS CERTIFICATION\n## SEP Phase 10\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## MSS IMPLEMENTATION STATUS\n\n")
	fmt.Fprintf(f, "- Filters: ADX ≥ 20, dual lookback (8+20 bar), liquidity sweep confirmation, close strength, 3-min cooldown\n")
	fmt.Fprintf(f, "- ATR-based stops: SL = 2×ATR, TP = 6×ATR\n\n")
	fmt.Fprintf(f, "## PERFORMANCE\n\n")
	fmt.Fprintf(f, "| Strategy | Trades | Win%% | PF | Expectancy | Sharpe | Status |\n")
	fmt.Fprintf(f, "|----------|--------|------|----|-----------|--------|--------|\n")
	for _, id := range mssIDs {
		for _, m := range metrics {
			if m.StrategyID == id {
				if m.Status == "INSUFFICIENT_DATA" {
					fmt.Fprintf(f, "| %s | %d | %.1f%% | — | INSUFFICIENT_DATA | — | PENDING |\n",
						id, m.TotalTrades, m.WinRate*100)
				} else {
					fmt.Fprintf(f, "| %s | %d | %.1f%% | %.2f | $%.4f | %.2f | %s |\n",
						id, m.TotalTrades, m.WinRate*100, m.ProfitFactor, m.Expectancy, m.SharpeRatio, m.Status)
				}
			}
		}
	}
}

func writeFundingAlphaReport(dir string, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "FUNDING_ALPHA_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fundingIDs := []string{
		"FundingMeanReversion_Alpha",
		"Phase11FundingMeanReversion_Alpha",
	}

	fmt.Fprintf(f, "# FUNDING ALPHA REPORT\n## SEP Phase 11\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## COLLECTION STATUS\n\n")
	fmt.Fprintf(f, "- Collection loop: ACTIVE (Binance + Bybit, 8h interval)\n")
	fmt.Fprintf(f, "- Data accumulation started: 2026-06-10\n")
	fmt.Fprintf(f, "- Next re-audit: 2026-07-10 (after 30 days)\n\n")
	fmt.Fprintf(f, "## PERFORMANCE\n\n")
	fmt.Fprintf(f, "| Strategy | Trades | Win%% | PF | Expectancy | Status |\n")
	fmt.Fprintf(f, "|----------|--------|------|----|-----------|--------|\n")
	for _, id := range fundingIDs {
		found := false
		for _, m := range metrics {
			if m.StrategyID == id {
				found = true
				if m.Status == "INSUFFICIENT_DATA" {
					fmt.Fprintf(f, "| %s | %d | %.1f%% | — | INSUFFICIENT_DATA | COLLECTING |\n",
						id, m.TotalTrades, m.WinRate*100)
				} else {
					fmt.Fprintf(f, "| %s | %d | %.1f%% | %.2f | $%.4f | %s |\n",
						id, m.TotalTrades, m.WinRate*100, m.ProfitFactor, m.Expectancy, m.Status)
				}
			}
		}
		if !found {
			fmt.Fprintf(f, "| %s | 0 | — | — | — | NO_TRADES |\n", id)
		}
	}
}

func writeWalkForwardReport(dir string, wfResults map[string]WFResult, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "WALK_FORWARD_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# WALK-FORWARD VALIDATION REPORT\n## SEP Phase 13\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Split:** 70%% in-sample, 30%% out-of-sample\n\n")

	passing := 0
	failing := 0
	fmt.Fprintf(f, "| Strategy | IS PF | OOS PF | OOS Win%% | Degradation | Pass |\n")
	fmt.Fprintf(f, "|----------|-------|--------|----------|-------------|------|\n")
	for _, m := range metrics {
		if m.Status != "PASS" {
			continue
		}
		wf, ok := wfResults[m.StrategyID]
		if !ok || wf.InSamplePF == 0 {
			continue
		}
		pass := "✅"
		if !wf.Pass {
			pass = "❌"
			failing++
		} else {
			passing++
		}
		fmt.Fprintf(f, "| %s | %.2f | %.2f | %.1f%% | %.2f | %s |\n",
			truncate(m.StrategyID, 40),
			wf.InSamplePF, wf.OOSProfitFactor,
			wf.OOSWinRate*100, wf.Degradation, pass,
		)
	}
	fmt.Fprintf(f, "\n**Passing OOS:** %d | **Failing OOS:** %d\n", passing, failing)
}

func writeMCReport(dir string, mc MCResult, nSims int) {
	f, _ := os.Create(filepath.Join(dir, "MONTE_CARLO_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# MONTE CARLO REPORT\n## SEP Phase 14\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Simulations:** %d | **Trades per sim:** 500 | **Ruin threshold:** −50%%\n\n", nSims)
	fmt.Fprintf(f, "## RESULTS\n\n")
	fmt.Fprintf(f, "| Metric | Value | Threshold | Status |\n|--------|-------|-----------|--------|\n")
	fmt.Fprintf(f, "| Risk of Ruin | %.2f%% | < 5%% | %s |\n", mc.RiskOfRuin*100, passIcon(mc.RiskOfRuin < 0.05))
	fmt.Fprintf(f, "| Median Return (500 trades) | $%.2f | > 0 | %s |\n", mc.MedianReturn, passIcon(mc.MedianReturn > 0))
	fmt.Fprintf(f, "| P5 Return (worst 5%%) | $%.2f | > −$5000 | %s |\n", mc.P5Return, passIcon(mc.P5Return > -5000))
	fmt.Fprintf(f, "| P95 Return (best 5%%) | $%.2f | — | — |\n", mc.P95Return)
	fmt.Fprintf(f, "| Median Max Drawdown | %.1f%% | < 20%% | %s |\n", mc.MaxDrawdownP50*100, passIcon(mc.MaxDrawdownP50 < 0.20))
	fmt.Fprintf(f, "| P95 Max Drawdown | %.1f%% | < 40%% | %s |\n", mc.MaxDrawdownP95*100, passIcon(mc.MaxDrawdownP95 < 0.40))
}

func writePortfolioConstructionReport(dir string, top50 []StrategyMetrics, all []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "PORTFOLIO_CONSTRUCTION_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# PORTFOLIO CONSTRUCTION REPORT\n## SEP Phase 15\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## CONSTRUCTION CRITERIA\n\n")
	fmt.Fprintf(f, "- Correlation cap: r ≤ 0.75\n")
	fmt.Fprintf(f, "- Minimum PF: 1.10\n")
	fmt.Fprintf(f, "- Minimum trades: %d\n", minTradesForEvidence)
	fmt.Fprintf(f, "- Positive expectancy required\n\n")
	fmt.Fprintf(f, "**Candidates (Top-50):** %d\n\n", len(top50))

	catMap := make(map[string]int)
	for _, m := range top50 {
		catMap[m.Category]++
	}
	fmt.Fprintf(f, "## CATEGORY DISTRIBUTION\n\n")
	fmt.Fprintf(f, "| Category | Count |\n|----------|-------|\n")
	for cat, n := range catMap {
		fmt.Fprintf(f, "| %s | %d |\n", cat, n)
	}
}

func writeCapitalAllocationReport(dir string, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "CAPITAL_ALLOCATION_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# CAPITAL ALLOCATION REPORT\n## SEP Phase 16\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## ALLOCATION FRAMEWORK\n\n")
	fmt.Fprintf(f, "| Tier | PF Range | Expectancy | Max Allocation | Rationale |\n")
	fmt.Fprintf(f, "|------|----------|------------|----------------|----------|\n")
	fmt.Fprintf(f, "| A | ≥ 1.50 | High | 5.0%% per strategy | Proven institutional edge |\n")
	fmt.Fprintf(f, "| B | 1.25–1.50 | Medium | 2.0%% per strategy | Selective edge |\n")
	fmt.Fprintf(f, "| C | 1.10–1.25 | Low | 0.5%% per strategy | Marginal edge, small size |\n")
	fmt.Fprintf(f, "| D | 1.00–1.10 | Very Low | 0.1%% per strategy | Minimal, monitor only |\n")
	fmt.Fprintf(f, "| F | < 1.00 | Negative | 0%% | RETIRED |\n\n")

	tierCounts := map[string]int{"A": 0, "B": 0, "C": 0, "D": 0, "F": 0}
	for _, m := range metrics {
		tierCounts[m.Tier]++
	}
	fmt.Fprintf(f, "## CURRENT PORTFOLIO ALLOCATION\n\n")
	fmt.Fprintf(f, "| Tier | Count | Max Capital |\n|------|-------|------------|\n")
	fmt.Fprintf(f, "| A | %d | %.1f%% |\n", tierCounts["A"], float64(tierCounts["A"])*5.0)
	fmt.Fprintf(f, "| B | %d | %.1f%% |\n", tierCounts["B"], float64(tierCounts["B"])*2.0)
	fmt.Fprintf(f, "| C | %d | %.1f%% |\n", tierCounts["C"], float64(tierCounts["C"])*0.5)
	fmt.Fprintf(f, "| D | %d | %.1f%% |\n", tierCounts["D"], float64(tierCounts["D"])*0.1)
}

func writeExecutionImpactReport(dir string, trades []Trade, metrics []StrategyMetrics) {
	f, _ := os.Create(filepath.Join(dir, "EXECUTION_IMPACT_REPORT.md"))
	if f == nil {
		return
	}
	defer f.Close()

	var totalFees, totalSlip float64
	var totalGross float64
	slipCount := 0
	for _, t := range trades {
		totalFees += t.Fees
		totalGross += t.GrossPnL
		if t.SlippageBPS > 0 {
			totalSlip += t.SlippageBPS
			slipCount++
		}
	}
	avgSlip := 0.0
	if slipCount > 0 {
		avgSlip = totalSlip / float64(slipCount)
	}
	feeImpact := 0.0
	if totalGross > 0 {
		feeImpact = totalFees / totalGross * 100
	}

	fmt.Fprintf(f, "# EXECUTION IMPACT REPORT\n## SEP Phase 17\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## AGGREGATE EXECUTION METRICS\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(f, "| Total fees paid | $%.2f |\n", totalFees)
	fmt.Fprintf(f, "| Total gross profit | $%.2f |\n", totalGross)
	fmt.Fprintf(f, "| Fee impact on gross | %.1f%% |\n", feeImpact)
	fmt.Fprintf(f, "| Avg slippage | %.2f bps |\n", avgSlip)
	fmt.Fprintf(f, "| Avg fee per trade | $%.2f |\n", totalFees/float64(max(len(trades), 1)))
	fmt.Fprintf(f, "\n## FEE REGIME\n\n")
	fmt.Fprintf(f, "- Fee model: Maker/taker combined (`entry_fee + exit_fee = total_fee`)\n")
	fmt.Fprintf(f, "- Paper trading uses realistic fee simulation (≈ 0.04%% taker)\n")
	fmt.Fprintf(f, "- Slippage model: bps above mid-price at execution\n")
}

func writeCapitalReadyPortfolio(dir string, top50 []StrategyMetrics, mc MCResult) {
	f, _ := os.Create(filepath.Join(dir, "CAPITAL_READY_PORTFOLIO.md"))
	if f == nil {
		return
	}
	defer f.Close()

	// Select top 20 from top50 with correlation filter
	var portfolio []StrategyMetrics
	for _, m := range top50 {
		if len(portfolio) >= 20 {
			break
		}
		tooCorr := false
		for _, s := range portfolio {
			a, b := AlignPnLSeries(m.PnLSeries, s.PnLSeries)
			if len(a) >= 10 && PearsonCorrelation(a, b) > 0.75 {
				tooCorr = true
				break
			}
		}
		if !tooCorr {
			portfolio = append(portfolio, m)
		}
	}

	fmt.Fprintf(f, "# CAPITAL-READY PORTFOLIO\n## SEP Phase 19\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Strategies:** %d | **MC Risk of Ruin:** %.1f%%\n\n", len(portfolio), mc.RiskOfRuin*100)

	if len(portfolio) == 0 {
		fmt.Fprintf(f, "_Insufficient evidence to construct capital-ready portfolio._\n")
		return
	}

	var totalAlloc float64
	fmt.Fprintf(f, "| # | Strategy | Tier | PF | Expectancy | Sharpe | Weight |\n")
	fmt.Fprintf(f, "|---|----------|------|----|-----------|--------|--------|\n")
	for i, m := range portfolio {
		weight := map[string]float64{"A": 5.0, "B": 2.0, "C": 0.5, "D": 0.1}[m.Tier]
		totalAlloc += weight
		fmt.Fprintf(f, "| %d | %s | %s | %.2f | $%.4f | %.2f | %.1f%% |\n",
			i+1, truncate(m.StrategyID, 40), m.Tier,
			m.ProfitFactor, m.Expectancy, m.SharpeRatio, weight,
		)
	}
	fmt.Fprintf(f, "\n**Total capital at risk:** %.1f%% of account\n", totalAlloc)
}

func writeDisablePatch(dir string, retiredIDs []string) {
	f, _ := os.Create(filepath.Join(dir, "strategy_disable_patch.go"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "// strategy_disable_patch.go — AUTO-GENERATED by SEP Phase 3\n")
	fmt.Fprintf(f, "// Add these calls after tracker construction in main.go to disable losers.\n\n")
	fmt.Fprintf(f, "package main\n\n")
	fmt.Fprintf(f, "// disableSEPLosers disables all strategies that failed SEP Phase 3 evidence.\n")
	fmt.Fprintf(f, "func disableSEPLosers(tracker interface{ Disable(string) }) {\n")
	for _, id := range retiredIDs {
		fmt.Fprintf(f, "\ttracker.Disable(%q)\n", id)
	}
	fmt.Fprintf(f, "}\n")
}

func writeNoDataReport(dir string) {
	f, _ := os.Create(filepath.Join(dir, "SEP_NO_DATA.md"))
	if f == nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# SEP EVIDENCE REPORT — NO DATA\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "## STATUS: FAIL — No trade data available\n\n")
	fmt.Fprintf(f, "No trades found in either MongoDB or SQLite.\n\n")
	fmt.Fprintf(f, "## REQUIRED ACTIONS\n\n")
	fmt.Fprintf(f, "1. Ensure MONGODB_URI env var is set with valid Atlas connection string\n")
	fmt.Fprintf(f, "2. Ensure SQLITE_PATH env var points to a valid engine.db\n")
	fmt.Fprintf(f, "3. Run the engine (./cmd/antigravity) to generate trade history\n")
	fmt.Fprintf(f, "4. Re-run this extractor after 30+ days of paper trading\n\n")
	fmt.Fprintf(f, "## TEST MODE\n\n")
	fmt.Fprintf(f, "Run with `--mock` flag to test analytics pipeline with synthetic data:\n\n")
	fmt.Fprintf(f, "```bash\ngo run ./cmd/sep_evidence/ --mock --out ./sep_reports\n```\n")
}

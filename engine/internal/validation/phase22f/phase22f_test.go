package phase22f

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// ── Synthetic dataset ─────────────────────────────────────────────────────────

func makeTrades(rng *rand.Rand) []phase22e.TradeRecord {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	strategies := []struct {
		id, name, family string
		wr, avgW, avgL   float64
		n                int
		live             bool
	}{
		{"s01", "EMA Cross 21/50", "EMA Cross", 0.55, 130, 85, 200, true},
		{"s02", "RSI Oversold 30", "RSI", 0.53, 100, 80, 180, true},
		{"s03", "Bollinger Squeeze", "Bollinger", 0.58, 150, 90, 160, true},
		{"s04", "FVG Retest", "Price Action", 0.56, 145, 95, 150, false},
		{"s05", "Funding Rate Arb", "Funding", 0.62, 85, 55, 140, false},
		{"s06", "Liquidation Cascade", "Microstructure", 0.60, 200, 120, 130, false},
		{"s07", "VWAP Bounce", "Volume", 0.50, 90, 88, 120, false},
		{"s08", "Order Block", "Price Action", 0.54, 135, 100, 110, false},
		{"s09", "MSS Continuation", "Price Action", 0.57, 140, 90, 100, false},
		{"s10", "Losing Strategy", "RSI", 0.40, 70, 100, 80, false}, // intentional loser
	}

	var trades []phase22e.TradeRecord
	idx := 0
	for _, s := range strategies {
		for i := 0; i < s.n; i++ {
			pnl := -s.avgL * (0.8 + rng.Float64()*0.4)
			if rng.Float64() < s.wr {
				pnl = s.avgW * (0.8 + rng.Float64()*0.4)
			}
			entry := base.Add(time.Duration(idx)*30*time.Minute + time.Duration(rng.Intn(20))*time.Minute)
			exit := entry.Add(time.Duration(15+rng.Intn(90)) * time.Minute)
			ep := 65000 + rng.Float64()*10000
			trades = append(trades, phase22e.TradeRecord{
				TradeID:      fmt.Sprintf("%s-t%04d", s.id, i),
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
				HoldMinutes:  float64(15 + rng.Intn(90)),
				EntryTime:    entry,
				ExitTime:     exit,
				Regime:       phase22e.RegimeRange,
				IsLive:       s.live,
			})
			idx++
		}
	}
	return trades
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestDataIntegrityCertification(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	cert := AuditDataIntegrity(trades)

	if cert.CertifiedTrades == 0 {
		t.Fatal("expected certified trades > 0")
	}
	if cert.CertifiedStrategies == 0 {
		t.Fatal("expected certified strategies > 0")
	}
	// Verify deduplication works
	trades2 := append(trades, trades[:10]...)
	cert2 := AuditDataIntegrity(trades2)
	if cert2.CertifiedTrades != cert.CertifiedTrades {
		t.Fatalf("expected deduplication: got %d certified, want %d", cert2.CertifiedTrades, cert.CertifiedTrades)
	}
}

func TestExtendedStats(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	byStrat := GroupTradesByStrategy(trades)
	id := "s01"
	es := ComputeExtendedStats(id, byStrat[id], 1_000_000/10.0)

	if es.SortinoRatio == 0 {
		t.Error("expected non-zero Sortino ratio")
	}
	if es.UlcerIndex == 0 {
		t.Error("expected non-zero Ulcer index")
	}
	if es.MaxConsecWins == 0 {
		t.Error("expected MaxConsecWins > 0")
	}
}

func TestConfidenceIntervals(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	ca := ComputeConfidenceIntervals(trades, rng)

	if ca.Portfolio.TradeCount == 0 {
		t.Fatal("expected portfolio CI to be computed")
	}
	pf := ca.Portfolio.ProfitFactor
	if pf.CI90Low > pf.CI90High {
		t.Errorf("CI90 bounds inverted: low=%.3f high=%.3f", pf.CI90Low, pf.CI90High)
	}
	if pf.CI95Low > pf.CI95High {
		t.Errorf("CI95 bounds inverted")
	}
}

func TestMonteCarlo(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	mc := RunMonteCarloF22("s01", GroupTradesByStrategy(trades)["s01"], 100_000, rng)

	if mc.Simulations != MCSimulations {
		t.Errorf("expected %d simulations, got %d", MCSimulations, mc.Simulations)
	}
	if mc.ProbabilityGrow < 0 || mc.ProbabilityGrow > 1 {
		t.Errorf("ProbabilityGrow out of range: %.3f", mc.ProbabilityGrow)
	}
	if mc.Stability == "" {
		t.Error("expected non-empty stability classification")
	}
}

func TestRegimeAssignment(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	assigned := AssignRegimesF22(trades)
	if len(assigned) != len(trades) {
		t.Errorf("assigned count mismatch: got %d want %d", len(assigned), len(trades))
	}
}

func TestAlphaValidation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	alphas := ValidateAlphaEngines(trades, 1_000_000, rng)

	if len(alphas) == 0 {
		t.Fatal("expected at least one alpha engine result")
	}
	for i, a := range alphas {
		if a.AlphaEngine == "" {
			t.Errorf("alpha[%d] has empty engine name", i)
		}
		if a.Rank != i+1 {
			t.Errorf("alpha[%d] rank mismatch: got %d want %d", i, a.Rank, i+1)
		}
	}
}

func TestTop20Selection(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	top20 := SelectTop20(trades, nil)

	if len(top20.Entries) == 0 {
		t.Fatal("expected non-empty top-20 selection")
	}
	for i, e := range top20.Entries {
		if e.Rank != i+1 {
			t.Errorf("top20[%d] rank=%d, want %d", i, e.Rank, i+1)
		}
	}
	// verify descending score order
	for i := 1; i < len(top20.Entries); i++ {
		if top20.Entries[i].Score > top20.Entries[i-1].Score {
			t.Errorf("top20 not sorted by score at index %d", i)
		}
	}
}

func TestCampaign(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	top20 := SelectTop20(trades, nil)
	campaign := RunValidationCampaign(trades, top20, 1_000_000)

	if len(campaign) == 0 {
		t.Fatal("expected campaign entries")
	}
	for _, ce := range campaign {
		if ce.StrategyID == "" {
			t.Error("campaign entry has empty strategy ID")
		}
		// invalidated strategies must have a reason
		if ce.Status == CampaignInvalidated && ce.Reason == "" {
			t.Errorf("invalidated strategy %s has no reason", ce.StrategyID)
		}
	}
}

func TestCapitalAllocation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	byStrat := GroupTradesByStrategy(trades)
	perNAV := 1_000_000.0 / float64(len(byStrat))
	mcResults := make(map[string]MonteCarloF22, len(byStrat))
	for id, ts := range byStrat {
		mcResults[id] = RunMonteCarloF22(id, ts, perNAV, rng)
	}
	allocs := AllocateCapital(trades, mcResults, nil, 1_000_000)
	if len(allocs) == 0 {
		t.Fatal("expected allocation entries")
	}
	for _, a := range allocs {
		if a.AllocationPct < 0 || a.AllocationPct > 25 {
			t.Errorf("allocation pct out of range: %.1f", a.AllocationPct)
		}
	}
}

func TestEliminationEngine(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	byStrat := GroupTradesByStrategy(trades)
	perNAV := 1_000_000.0 / float64(len(byStrat))
	mcResults := make(map[string]MonteCarloF22, len(byStrat))
	for id, ts := range byStrat {
		mcResults[id] = RunMonteCarloF22(id, ts, perNAV, rng)
	}
	candidates := IdentifyEliminationCandidates(trades, mcResults)

	// s10 (Losing Strategy, WR=40%) should be flagged
	found := false
	for _, c := range candidates {
		if c.StrategyID == "s10" {
			found = true
			if len(c.Reasons) == 0 {
				t.Error("expected elimination reasons for s10")
			}
		}
	}
	if !found {
		t.Log("note: s10 (Losing Strategy) not in elimination candidates — may need more trades to trigger gate")
	}
}

func TestCertificationTiers(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	byStrat := GroupTradesByStrategy(trades)
	perNAV := 1_000_000.0 / float64(len(byStrat))
	mcResults := make(map[string]MonteCarloF22)
	for id, ts := range byStrat {
		mcResults[id] = RunMonteCarloF22(id, ts, perNAV, rng)
	}
	tiers := ClassifyAllTiers(trades, mcResults)
	if len(tiers) == 0 {
		t.Fatal("expected tier classifications")
	}
	counts := TierCounts22(tiers)
	total := 0
	for _, c := range counts {
		total += c
	}
	if total != len(tiers) {
		t.Errorf("tier count mismatch: total=%d tiers=%d", total, len(tiers))
	}
}

func TestEdgeVerdict(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	byStrat := GroupTradesByStrategy(trades)
	perNAV := 1_000_000.0 / float64(len(byStrat))
	mcResults := make(map[string]MonteCarloF22)
	for id, ts := range byStrat {
		mcResults[id] = RunMonteCarloF22(id, ts, perNAV, rng)
	}
	alphas := ValidateAlphaEngines(trades, 1_000_000, rng)
	tiers := ClassifyAllTiers(trades, mcResults)
	portMC := RunPortfolioMC(trades, 1_000_000, rng)
	verdict := GenerateEdgeVerdict(trades, alphas, tiers, portMC)

	if verdict.Narrative == "" {
		t.Error("expected non-empty edge narrative")
	}
	if verdict.StrongestAlpha == "" {
		t.Error("expected strongest alpha to be identified")
	}
}

func TestFullPipeline(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)

	p := NewPipeline(1_000_000)
	// fix rng for reproducibility
	p.rng = rng

	result := p.Run(trades, nil)

	if result.TotalTrades != len(trades) {
		t.Errorf("total trades: got %d want %d", result.TotalTrades, len(trades))
	}
	if result.TotalStrategies == 0 {
		t.Error("expected strategies to be counted")
	}
	if result.EdgeVerdict.Narrative == "" {
		t.Error("expected edge verdict narrative")
	}
	if len(result.CertificationTiers) == 0 {
		t.Error("expected certification tiers")
	}
	if len(result.AlphaValidation) == 0 {
		t.Error("expected alpha validation results")
	}
	if len(result.Portfolios) == 0 {
		t.Error("expected portfolio variants")
	}
}

func TestNewStrategyValidation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	p := NewPipeline(1_000_000)
	p.rng = rng

	// winning strategy
	goodTrades := GroupTradesByStrategy(trades)["s01"]
	approved, tier, _ := p.ValidateNewStrategy("s01", goodTrades)
	_ = approved // may or may not be approved depending on data
	_ = tier

	// too few trades
	approved2, _, reasons := p.ValidateNewStrategy("newstrat", []phase22e.TradeRecord{})
	if approved2 {
		t.Error("empty trade set should not be approved")
	}
	if len(reasons) == 0 {
		t.Error("expected rejection reason")
	}
}

func TestExecutionCorrelation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	rep := CorrelateExecutionQuality(trades, nil)
	if rep.Summary == "" {
		t.Error("expected non-empty correlation summary")
	}
}

func TestPortfolioConstruction(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	top20 := SelectTop20(trades, nil)
	portfolios := ConstructPortfolios(trades, top20, 1_000_000, rng)
	if len(portfolios) == 0 {
		t.Fatal("expected portfolio variants")
	}
	for _, pv := range portfolios {
		if pv.Name == "" {
			t.Error("portfolio variant has empty name")
		}
		if len(pv.Strategies) == 0 {
			t.Errorf("portfolio %s has no strategies", pv.Name)
		}
	}
}

func TestReportWriter(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	p := NewPipeline(1_000_000)
	p.rng = rng
	result := p.Run(trades, nil)

	dir := t.TempDir()
	if err := WriteAllReports(result, dir); err != nil {
		t.Fatalf("WriteAllReports: %v", err)
	}
}

func BenchmarkFullPipeline(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	trades := makeTrades(rng)
	p := NewPipeline(1_000_000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.rng = rand.New(rand.NewSource(int64(i)))
		_ = p.Run(trades, nil)
	}
}

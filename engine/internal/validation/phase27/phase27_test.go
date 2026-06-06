package phase27_test

import (
	"testing"
	"time"

	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
)

func makeLiveTrades(fam phase25.AlphaFamily, n int, winRate float64) []phase25.LiveTrade {
	trades := make([]phase25.LiveTrade, n)
	for i := range trades {
		isWin := float64(i)/float64(n) < winRate
		pnl := -50.0
		if isWin {
			pnl = 100.0
		}
		trades[i] = phase25.LiveTrade{
			TradeID:      string(fam) + "_" + string(rune('A'+i%26)),
			StrategyName: "strat_" + string(fam),
			AlphaFamily:  fam,
			EntryTime:    time.Now().Add(-time.Duration(n-i) * time.Hour),
			NetPnLUSD:    pnl,
			GrossPnLUSD:  pnl + 10,
			FeeUSD:       5,
			SlippageUSD:  3,
			FundingUSD:   2,
			IsWin:        isWin,
		}
	}
	return trades
}

func TestBuildAttributionRecords(t *testing.T) {
	trades := makeLiveTrades(phase25.FamilyMomentum, 50, 0.6)
	records := phase27.BuildAttributionRecords(trades)
	if len(records) != 50 {
		t.Errorf("expected 50 records, got %d", len(records))
	}
	for _, r := range records {
		if r.AlphaFamily != phase25.FamilyMomentum {
			t.Errorf("expected Momentum family, got %s", r.AlphaFamily)
		}
	}
}

func TestBuildFamilyPerf_ProfitFactor(t *testing.T) {
	trades := makeLiveTrades(phase25.FamilyFVG, 60, 0.60)
	records := phase27.BuildAttributionRecords(trades)
	perf := phase27.BuildFamilyPerf(phase25.FamilyFVG, records, 0)
	if perf.ProfitFactor <= 0 {
		t.Errorf("expected positive PF, got %.3f", perf.ProfitFactor)
	}
	if perf.Trades != 60 {
		t.Errorf("expected 60 trades, got %d", perf.Trades)
	}
}

func TestBuildFamilyPerf_InsufficientFlag(t *testing.T) {
	trades := makeLiveTrades(phase25.FamilyFunding, 10, 0.6) // < 30
	records := phase27.BuildAttributionRecords(trades)
	_, _, insufficient := phase27.BuildAllFamilyPerfs(records)
	found := false
	for _, f := range insufficient {
		if f == string(phase25.FamilyFunding) {
			found = true
		}
	}
	if !found {
		t.Error("expected Funding to be in insufficient evidence list")
	}
}

func TestBuildPareto_Top1(t *testing.T) {
	perfs := []phase27.AlphaFamilyPerf{
		{Family: phase25.FamilyMomentum, NetPnLUSD: 1000},
		{Family: phase25.FamilyTrend, NetPnLUSD: 500},
		{Family: phase25.FamilyFVG, NetPnLUSD: 300},
	}
	pa := phase27.BuildPareto(perfs, 1800)
	expectedTop1 := 1000.0 / 1800.0 * 100
	if pa.Top1AlphaPct < expectedTop1-0.1 || pa.Top1AlphaPct > expectedTop1+0.1 {
		t.Errorf("Top1AlphaPct: expected %.1f, got %.1f", expectedTop1, pa.Top1AlphaPct)
	}
	if pa.Top1Family != phase25.FamilyMomentum {
		t.Errorf("Top1Family: expected Momentum, got %s", pa.Top1Family)
	}
}

func TestBuildChampionship_Champion(t *testing.T) {
	perfs := []phase27.AlphaFamilyPerf{
		{Family: phase25.FamilyMomentum, ProfitFactor: 2.0, Sharpe: 2.5, Expectancy: 80, NetPnLUSD: 5000, Trades: 50},
	}
	champs := phase27.BuildChampionship(perfs)
	if len(champs) == 0 {
		t.Fatal("expected at least one champion record")
	}
	if champs[0].Verdict != phase27.AlphaChampion {
		t.Errorf("expected CHAMPION, got %s", champs[0].Verdict)
	}
}

func TestBuildChampionship_Eliminated(t *testing.T) {
	perfs := []phase27.AlphaFamilyPerf{
		{Family: phase25.FamilyFunding, ProfitFactor: 0.8, Sharpe: -0.5, NetPnLUSD: -500, Trades: 50},
	}
	champs := phase27.BuildChampionship(perfs)
	if champs[0].Verdict != phase27.AlphaEliminated {
		t.Errorf("expected ELIMINATED, got %s", champs[0].Verdict)
	}
}

func TestBuildOverlapAnalysis_ZeroWithOneFamily(t *testing.T) {
	perfs := []phase27.AlphaFamilyPerf{
		{Family: phase25.FamilyMomentum},
	}
	records := phase27.BuildAttributionRecords(makeLiveTrades(phase25.FamilyMomentum, 30, 0.6))
	result := phase27.BuildOverlapAnalysis(perfs, records)
	if len(result) != 0 {
		t.Errorf("expected 0 overlap records for single family, got %d", len(result))
	}
}

func TestPipelineRun_BasicFlow(t *testing.T) {
	trades := append(
		makeLiveTrades(phase25.FamilyMomentum, 50, 0.6),
		makeLiveTrades(phase25.FamilyTrend, 40, 0.55)...,
	)
	p25 := &phase25.Phase25Result{
		LiveTrades:  trades,
		LiveMetrics: map[string]phase25.LiveMetrics{},
	}
	p26 := &phase26.Phase26Result{
		Tier2Milestones: map[string][]phase26.MilestoneReport{},
	}
	result, err := phase27.Run(p25, p26)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.AttributionRecords) != 90 {
		t.Errorf("expected 90 records, got %d", len(result.AttributionRecords))
	}
	if len(result.AlphaPerformance) != 2 {
		t.Errorf("expected 2 families, got %d", len(result.AlphaPerformance))
	}
}

func TestPipelineRun_NilInputErrors(t *testing.T) {
	_, err := phase27.Run(nil, nil)
	if err == nil {
		t.Error("expected error for nil inputs")
	}
}

func TestBuildReports_Keys(t *testing.T) {
	r := &phase27.Phase27Result{GeneratedAt: time.Now()}
	reports := phase27.BuildReports(r)
	for _, key := range []string{"attribution", "alpha_performance", "pareto", "championship", "overlap", "executive_summary"} {
		if _, ok := reports[key]; !ok {
			t.Errorf("missing report: %s", key)
		}
	}
}

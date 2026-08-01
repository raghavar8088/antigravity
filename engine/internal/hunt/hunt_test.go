package hunt

import (
	"testing"
	"time"
)

// The hunt's whole safety property is that growth ranks but does not authorise.
// These tests pin the arithmetic behind that, and the specific ways a lucky
// account imitates a real one.

func tr(strategy string, net float64, day int) Trade {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return Trade{
		Strategy: strategy,
		NetPnL:   net,
		GrossPnL: net + 0.10, // a small fee on every trade
		Fees:     0.10,
		ClosedAt: base.AddDate(0, 0, day),
	}
}

func TestBuildAccounts_TracksCapitalFromStake(t *testing.T) {
	accts := BuildAccounts("buying", []Trade{
		tr("S1", 100, 1), tr("S1", -40, 2), tr("S1", 60, 3),
	}, DefaultStartingCapital)

	if len(accts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(accts))
	}
	a := accts[0]
	if a.StartingCapital != 1000 {
		t.Errorf("starting capital = %.2f, want 1000", a.StartingCapital)
	}
	if a.Capital != 1120 {
		t.Errorf("capital = %.2f, want 1120 (1000 +100 -40 +60)", a.Capital)
	}
	if got := a.GrowthPct; got < 11.9 || got > 12.1 {
		t.Errorf("growth = %.2f%%, want ~12%%", got)
	}
	if a.Trades != 3 || a.Wins != 2 || a.Losses != 1 {
		t.Errorf("counts wrong: %d trades %d wins %d losses", a.Trades, a.Wins, a.Losses)
	}
	// Tolerance, not equality: 0.1+0.1+0.1 is not exactly 0.3 in binary floating
	// point, and asserting equality here would fail on correct arithmetic.
	if a.Fees < 0.29 || a.Fees > 0.31 {
		t.Errorf("fees = %.4f, want ~0.30", a.Fees)
	}
}

// Scalp runs strategy x symbol. A strategy that works on BTC and fails on DOGE
// is two results, not an average of one.
func TestBuildAccounts_SeparatesSymbolsIntoOwnAccounts(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	trades := []Trade{
		{Strategy: "S1", Symbol: "BTCUSD", NetPnL: 50, ClosedAt: base},
		{Strategy: "S1", Symbol: "DOGEUSD", NetPnL: -50, ClosedAt: base},
	}
	accts := BuildAccounts("scalp", trades, 1000)
	if len(accts) != 2 {
		t.Fatalf("accounts = %d, want 2 (one per strategy x symbol)", len(accts))
	}
	for _, a := range accts {
		if a.Symbol == "" {
			t.Error("symbol lost from the account key")
		}
	}
}

// Max drawdown must follow the real time sequence, not the input order.
func TestBuildAccounts_DrawdownUsesChronologicalOrder(t *testing.T) {
	// Supplied out of order on purpose.
	accts := BuildAccounts("buying", []Trade{
		tr("S1", 100, 3), tr("S1", -200, 2), tr("S1", 100, 1),
	}, 1000)
	a := accts[0]

	// Chronological: 1000 -> 1100 -> 900 -> 1000. Peak 1100, trough 900 => 18.18%.
	if a.MaxDrawdown < 0.18 || a.MaxDrawdown > 0.19 {
		t.Errorf("maxDD = %.4f, want ~0.1818 from the chronological walk", a.MaxDrawdown)
	}
}

// An unbeaten record is a small sample, not an infinite profit factor. Reporting
// +Inf would top every sort on two trades.
func TestBuildAccounts_NoLossesDoesNotProduceInfinitePF(t *testing.T) {
	a := BuildAccounts("buying", []Trade{tr("S1", 10, 1), tr("S1", 10, 2)}, 1000)[0]
	if a.ProfitFactor != 0 {
		t.Errorf("PF = %v for a record with no losses; must not be +Inf", a.ProfitFactor)
	}
}

func TestLeaderboard_RanksByGrowthThenSampleSize(t *testing.T) {
	accts := []Account{
		{Key: "small-sample", GrowthPct: 50, Trades: 3},
		{Key: "big-winner", GrowthPct: 80, Trades: 400},
		{Key: "same-growth-more-evidence", GrowthPct: 50, Trades: 500},
	}
	lb := Leaderboard(accts)
	if lb[0].Key != "big-winner" {
		t.Errorf("top = %q, want big-winner", lb[0].Key)
	}
	// Equal growth: more evidence wins.
	if lb[1].Key != "same-growth-more-evidence" {
		t.Errorf("second = %q, want the higher-sample account", lb[1].Key)
	}
}

// The core protection: a spectacular but tiny record must NOT pass.
func TestGate_RejectsSpectacularSmallSample(t *testing.T) {
	a := Account{
		Key: "lucky", Trades: 5, DaysLive: 2,
		ProfitFactor: 9, MaxDrawdown: 0.01, NetPnL: 400,
		Expectancy: 80, FirstHalfNet: 200, SecondHalfNet: 200,
		GrowthPct: 40,
	}
	v := DefaultGate.Evaluate(a)
	if v.Pass {
		t.Fatal("a 5-trade, 2-day record passed the gate; that is exactly the coin flip the gate exists to stop")
	}
	if len(v.Failures) < 2 {
		t.Errorf("expected both the trade and day minimums to fail, got %v", v.Failures)
	}
	if v.Progress <= 0 || v.Progress >= 1 {
		t.Errorf("progress = %.2f, want a fraction toward the sample minimum", v.Progress)
	}
}

// A record carried by one lucky streak must fail even with a strong total.
func TestGate_RejectsOneSidedRecord(t *testing.T) {
	a := Account{
		Key: "streak", Trades: 300, DaysLive: 60,
		ProfitFactor: 1.5, MaxDrawdown: 0.10, NetPnL: 500,
		Expectancy: 1.6, FeeDragPct: 10,
		FirstHalfNet: 900, SecondHalfNet: -400, // all the money came early
	}
	v := DefaultGate.Evaluate(a)
	if v.Pass {
		t.Fatal("a record with a negative second half passed; one streak is not an edge")
	}
}

// An edge that exists only before fees is not an edge.
func TestGate_RejectsFeeEatenEdge(t *testing.T) {
	a := Account{
		Key: "fee-eaten", Trades: 400, DaysLive: 45,
		ProfitFactor: 1.25, MaxDrawdown: 0.12, NetPnL: 20,
		Expectancy: 0.05, FeeDragPct: 85, // fees ate most of gross
		FirstHalfNet: 12, SecondHalfNet: 8,
	}
	if DefaultGate.Evaluate(a).Pass {
		t.Fatal("a strategy whose fees ate 85% of gross profit passed the gate")
	}
}

func TestGate_AcceptsGenuinelyQualifiedRecord(t *testing.T) {
	a := Account{
		Key: "real", Trades: 350, DaysLive: 45,
		ProfitFactor: 1.4, MaxDrawdown: 0.15, NetPnL: 300,
		Expectancy: 0.86, FeeDragPct: 18,
		FirstHalfNet: 160, SecondHalfNet: 140,
	}
	v := DefaultGate.Evaluate(a)
	if !v.Pass {
		t.Fatalf("a qualified record failed: %v", v.Failures)
	}
}

// The multiple-comparison guard: passers must be reported against the number
// noise alone would produce.
func TestSummarise_ReportsChanceBaseline(t *testing.T) {
	// 200 eligible accounts, none passing.
	accts := make([]Account, 0, 200)
	for i := 0; i < 200; i++ {
		accts = append(accts, Account{
			Key:             string(rune('a'+i%26)) + string(rune('0'+i/26)),
			StartingCapital: 1000, Capital: 1000,
			Trades: 250, DaysLive: 40,
		})
	}
	s := Summarise(accts, DefaultGate)

	if s.EligibleForGate != 200 {
		t.Fatalf("eligible = %d, want 200", s.EligibleForGate)
	}
	if s.ExpectedByChance <= 0 {
		t.Fatal("chance baseline must be reported for a 200-way search")
	}
	if s.Interpretation == "" {
		t.Fatal("interpretation must be rendered so the baseline is not ignored")
	}
	// With 0 passers against ~10 expected, this must read as indistinguishable.
	if !contains(s.Interpretation, "NOT distinguishable") {
		t.Errorf("interpretation = %q, want it to flag noise", s.Interpretation)
	}
}

func TestSummarise_TotalsCapitalAcrossHunt(t *testing.T) {
	accts := []Account{
		{StartingCapital: 1000, Capital: 1200},
		{StartingCapital: 1000, Capital: 900},
	}
	s := Summarise(accts, DefaultGate)
	if s.Funded != 2000 {
		t.Errorf("funded = %.0f, want 2000", s.Funded)
	}
	if s.Current != 2100 {
		t.Errorf("current = %.0f, want 2100", s.Current)
	}
	if s.GrowthPct < 4.9 || s.GrowthPct > 5.1 {
		t.Errorf("growth = %.2f%%, want ~5%%", s.GrowthPct)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

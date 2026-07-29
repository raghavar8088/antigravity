package hunt

import "testing"

func qualified() Account {
	return Account{
		Key: "real", StartingCapital: 1000, Capital: 1300,
		Trades: 350, DaysLive: 45,
		ProfitFactor: 1.4, MaxDrawdown: 0.15, NetPnL: 300,
		Expectancy: 0.86, FeeDragPct: 18,
		FirstHalfNet: 160, SecondHalfNet: 140,
	}
}

// A gate pass is necessary but not sufficient — promotion also has to be
// expressible at the live desk's $100 ceiling.
func TestCheckPromotable_GateFailureBlocks(t *testing.T) {
	a := qualified()
	a.Trades = 10 // no longer enough sample

	c := CheckPromotable(a, DefaultGate, TypicalContractCostUSD)
	if c.Ready {
		t.Fatal("a candidate that fails the gate was marked ready to promote")
	}
	if len(c.Blockers) == 0 {
		t.Error("gate failures must be reported as blockers")
	}
}

func TestCheckPromotable_QualifiedCandidateIsReady(t *testing.T) {
	c := CheckPromotable(qualified(), DefaultGate, TypicalContractCostUSD)
	if !c.Ready {
		t.Fatalf("a qualified candidate was blocked: %v", c.Blockers)
	}
	if c.ScaleNote == "" {
		t.Error("the size difference between hunt and live must always be stated")
	}
}

// The live desk buys ONE contract at a time under a $100 ceiling. If a single
// contract costs more than the ceiling, the strategy cannot trade at all — the
// exact class of failure that had 16 live orders per cycle dying on the
// execution floor.
func TestCheckPromotable_BlocksWhenOneContractExceedsCeiling(t *testing.T) {
	c := CheckPromotable(qualified(), DefaultGate, 150.0) // $150 > $100 ceiling
	if c.Ready {
		t.Fatal("promoted a strategy whose single contract costs more than the live ceiling")
	}
	found := false
	for _, b := range c.Blockers {
		if len(b) > 0 && b[0] == 'o' { // "one contract costs ..."
			found = true
		}
	}
	if !found {
		t.Errorf("expected a contract-cost blocker, got %v", c.Blockers)
	}
}

// A thin per-trade edge is precisely what a 10x size reduction destroys.
func TestCheckPromotable_WarnsOnThinEdge(t *testing.T) {
	a := qualified()
	a.Expectancy = 0.01 // 0.1 bps of a $1,000 account

	c := CheckPromotable(a, DefaultGate, TypicalContractCostUSD)
	if len(c.Warnings) == 0 {
		t.Fatal("a sub-basis-point edge must warn: contract rounding at live size can erase it")
	}
}

// Payoff shape, not track record, decides this one.
func TestSellingPromotion_RefusesNakedShorts(t *testing.T) {
	p := SellingPromotionPolicy{AllowDefinedRiskSpreads: true}

	// Even a spectacular record must not promote as a naked short.
	a := qualified()
	a.ProfitFactor = 5
	a.NetPnL = 5000

	c := p.CheckSellingPromotable(a, false)
	if c.Ready {
		t.Fatal("a naked short was approved for a $100 live account; loss is unbounded")
	}
	if len(c.Blockers) == 0 {
		t.Error("the refusal must state why")
	}
}

func TestSellingPromotion_AllowsDefinedRiskSpread(t *testing.T) {
	p := SellingPromotionPolicy{AllowDefinedRiskSpreads: true}
	if c := p.CheckSellingPromotable(qualified(), true); !c.Ready {
		t.Fatalf("a defined-risk spread was blocked: %v", c.Blockers)
	}
}

func TestSellingPromotion_PolicyCanForbidEvenSpreads(t *testing.T) {
	p := SellingPromotionPolicy{AllowDefinedRiskSpreads: false}
	if c := p.CheckSellingPromotable(qualified(), true); c.Ready {
		t.Fatal("policy disallows spreads but one was approved")
	}
}

package cryptofno

import (
	"testing"
	"time"
)

// The requirement in one sentence: selling a call and a put must reserve a lot,
// and buying wings against them must reserve much less. Per-leg margining cannot
// do that — it charges an iron condor exactly like the naked strangle inside it.
// These tests pin the difference.

const (
	testSpot = 65000.0
	testCV   = 0.001 // Delta option contract = 0.001 BTC
	testIV   = 0.55
)

func expiry() time.Time { return time.Now().Add(7 * 24 * time.Hour) }

func leg(t OptionType, side Side, strike float64, lots int, premium float64) Leg {
	return Leg{
		Symbol: string(t) + "-BTC", Type: t, Side: side, Strike: strike,
		Expiry: expiry(), Lots: lots, PremiumPerBTC: premium,
		IV: testIV, ContractValue: testCV,
	}
}

// A short strangle has unbounded loss on both sides and must reserve heavily.
func TestPortfolioMargin_NakedStrangleIsExpensive(t *testing.T) {
	strangle := []Leg{
		leg(TypeCall, SideSell, 65000, 100, 1500),
		leg(TypePut, SideSell, 65000, 100, 1500),
	}
	r := PortfolioMargin(strangle, testSpot, DefaultMarginParams)

	if r.RequiredUSD <= 0 {
		t.Fatal("a naked short strangle reserved nothing")
	}
	if r.WorstCaseLossUSD <= 0 {
		t.Error("scan found no adverse scenario for a naked short")
	}
	t.Logf("naked strangle: required $%.2f (worst loss $%.2f at spot %.0f)",
		r.RequiredUSD, r.WorstCaseLossUSD, r.WorstCaseSpot)
}

// THE headline behaviour: add long wings and the requirement must fall sharply.
func TestPortfolioMargin_WingsCollapseTheRequirement(t *testing.T) {
	strangle := []Leg{
		leg(TypeCall, SideSell, 65000, 100, 1500),
		leg(TypePut, SideSell, 65000, 100, 1500),
	}
	condor := append(append([]Leg{}, strangle...),
		leg(TypeCall, SideBuy, 70000, 100, 400),
		leg(TypePut, SideBuy, 60000, 100, 400),
	)

	naked := PortfolioMargin(strangle, testSpot, DefaultMarginParams)
	hedged := PortfolioMargin(condor, testSpot, DefaultMarginParams)

	t.Logf("naked  $%.2f", naked.RequiredUSD)
	t.Logf("hedged $%.2f  (credit $%.2f vs standalone $%.2f)",
		hedged.RequiredUSD, hedged.HedgeCreditUSD, hedged.StandaloneUSD)

	if hedged.RequiredUSD >= naked.RequiredUSD {
		t.Fatalf("hedged basket required $%.2f, not less than the naked $%.2f — "+
			"the wings earned no credit, which means legs are being margined independently",
			hedged.RequiredUSD, naked.RequiredUSD)
	}
	// The benefit must be material, not a rounding artefact.
	if hedged.RequiredUSD > naked.RequiredUSD*0.6 {
		t.Errorf("hedged requirement is %.0f%% of naked; wings should cut it far more",
			hedged.RequiredUSD/naked.RequiredUSD*100)
	}
	if hedged.HedgeCreditUSD <= 0 {
		t.Error("hedge credit must be reported so the benefit is visible")
	}
}

// A long-only basket cannot lose more than the debit paid, so no scenario scan
// may reserve more than that. Reserving more would block trades with no tail risk.
func TestPortfolioMargin_LongOnlyCappedAtPremiumPaid(t *testing.T) {
	longs := []Leg{
		leg(TypeCall, SideBuy, 66000, 50, 900),
		leg(TypePut, SideBuy, 64000, 50, 900),
	}
	r := PortfolioMargin(longs, testSpot, DefaultMarginParams)

	wantDebit := longs[0].PremiumUSD() + longs[1].PremiumUSD()
	if r.RequiredUSD > wantDebit+0.01 {
		t.Fatalf("long-only basket required $%.2f, more than the $%.2f debit — "+
			"max loss on a long option IS the premium", r.RequiredUSD, wantDebit)
	}
	if r.Basis == "" {
		t.Error("basis must explain which rule bound the answer")
	}
}

// A vertical spread's loss is capped by the strike width; the requirement must
// reflect that rather than the naked short it contains.
func TestPortfolioMargin_VerticalSpreadBoundedByWidth(t *testing.T) {
	spread := []Leg{
		leg(TypeCall, SideSell, 65000, 100, 1500),
		leg(TypeCall, SideBuy, 66000, 100, 1100),
	}
	r := PortfolioMargin(spread, testSpot, DefaultMarginParams)

	// Max loss ≈ (width - net credit) x contract value x lots.
	width := (66000.0 - 65000.0) * testCV * 100
	if r.RequiredUSD > width*1.5 {
		t.Fatalf("call spread required $%.2f against a $%.2f strike width — "+
			"a defined-risk spread must not be margined like a naked short",
			r.RequiredUSD, width)
	}
	t.Logf("call spread: required $%.2f vs $%.2f width", r.RequiredUSD, width)
}

// A far-OTM short can sit outside the scan's worst point yet still carries
// unbounded risk. Zero margin on a naked short is never correct.
func TestPortfolioMargin_FarOTMShortStillReserves(t *testing.T) {
	far := []Leg{leg(TypeCall, SideSell, 200000, 10, 5)}
	r := PortfolioMargin(far, testSpot, DefaultMarginParams)

	if r.RequiredUSD <= 0 {
		t.Fatal("a far-OTM naked short reserved nothing; loss is still unbounded")
	}
	t.Logf("far OTM short: required $%.2f (basis: %s)", r.RequiredUSD, r.Basis)
}

// More short lots must require more margin — a monotonicity check that catches
// sign errors the headline tests would miss.
func TestPortfolioMargin_ScalesWithSize(t *testing.T) {
	small := PortfolioMargin([]Leg{leg(TypeCall, SideSell, 65000, 10, 1500)}, testSpot, DefaultMarginParams)
	big := PortfolioMargin([]Leg{leg(TypeCall, SideSell, 65000, 100, 1500)}, testSpot, DefaultMarginParams)

	if big.RequiredUSD <= small.RequiredUSD {
		t.Fatalf("10x the lots required $%.2f vs $%.2f — margin must scale with size",
			big.RequiredUSD, small.RequiredUSD)
	}
}

// Netting across different underlyings would hand out a credit the market will
// not honour: a BTC short is not hedged by an ETH long.
func TestGroupBaskets_SeparatesUnderlyings(t *testing.T) {
	legs := []Leg{
		{Symbol: "C-BTC-65000", Type: TypeCall, Side: SideSell, Lots: 1, ContractValue: testCV},
		{Symbol: "C-ETH-3000", Type: TypeCall, Side: SideBuy, Lots: 1, ContractValue: testCV},
	}
	underlying := func(l Leg) string {
		if len(l.Symbol) > 3 && l.Symbol[2:5] == "BTC" {
			return "BTC"
		}
		return "ETH"
	}
	groups := GroupBaskets(legs, underlying)
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 — a BTC short is not hedged by an ETH long", len(groups))
	}
}

// An empty basket must be free, and must say so rather than returning a bare 0.
func TestPortfolioMargin_EmptyBasket(t *testing.T) {
	r := PortfolioMargin(nil, testSpot, DefaultMarginParams)
	if r.RequiredUSD != 0 || r.Basis == "" {
		t.Errorf("empty basket = $%.2f basis=%q, want 0 with an explanation", r.RequiredUSD, r.Basis)
	}
}

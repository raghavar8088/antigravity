package cryptofno

import (
	"errors"
	"testing"
)

const feeRate = 0.0005

func newTestBook(t *testing.T) (*Book, string) {
	t.Helper()
	b := NewBook()
	accts := b.Accounts(nil)
	if len(accts) != 1 {
		t.Fatalf("new book has %d accounts, want 1 default", len(accts))
	}
	return b, accts[0].ID
}

func TestNewBook_HasUsableDefaultAccount(t *testing.T) {
	b, id := newTestBook(t)
	v, err := b.AccountView(id, nil)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if v.InitialCapitalUSD != DefaultAccountCapitalUSD {
		t.Errorf("default capital $%.0f, want $%.0f", v.InitialCapitalUSD, DefaultAccountCapitalUSD)
	}
	if v.AvailableUSD != DefaultAccountCapitalUSD {
		t.Errorf("available $%.0f on a fresh account, want the full capital", v.AvailableUSD)
	}
}

func TestCreateAccount_RejectsDuplicateName(t *testing.T) {
	b, _ := newTestBook(t)
	if _, err := b.CreateAccount("Alpha", 50_000); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := b.CreateAccount("alpha", 50_000); err == nil {
		t.Fatal("duplicate name (case-insensitive) was accepted")
	}
}

// Editing capital must ADJUST the book, not reset it — an account that has
// traded carries realised P&L, and overwriting the balance would erase that
// record while looking like a harmless top-up.
func TestEditAccount_CapitalChangePreservesRealisedPnl(t *testing.T) {
	b, id := newTestBook(t)

	b.mu.Lock()
	b.accounts[id].RealisedPnlUSD = -250
	b.mu.Unlock()

	if _, err := b.EditAccount(id, "", 200_000); err != nil {
		t.Fatalf("edit: %v", err)
	}
	v, _ := b.AccountView(id, nil)
	if v.InitialCapitalUSD != 200_000 {
		t.Errorf("capital = $%.0f, want $200,000", v.InitialCapitalUSD)
	}
	if v.RealisedPnlUSD != -250 {
		t.Errorf("realised P&L = $%.0f, want -250 preserved across a capital edit", v.RealisedPnlUSD)
	}
	if v.EquityUSD != 199_750 {
		t.Errorf("equity = $%.0f, want $199,750 (capital + realised)", v.EquityUSD)
	}
}

func TestEditAccount_RenameOnlyLeavesCapitalAlone(t *testing.T) {
	b, id := newTestBook(t)
	if _, err := b.EditAccount(id, "Renamed", 0); err != nil {
		t.Fatalf("edit: %v", err)
	}
	v, _ := b.AccountView(id, nil)
	if v.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", v.Name)
	}
	if v.InitialCapitalUSD != DefaultAccountCapitalUSD {
		t.Errorf("capital changed to $%.0f during a rename", v.InitialCapitalUSD)
	}
}

// Cutting capital below what open positions reserve would create an instantly
// under-margined book, so it must be refused rather than silently allowed.
func TestEditAccount_RefusesCapitalBelowMarginInUse(t *testing.T) {
	b, id := newTestBook(t)
	legs := []Leg{leg(TypeCall, SideSell, 65000, 500, 1500), leg(TypePut, SideSell, 65000, 500, 1500)}
	if _, err := b.ExecuteBasket(id, "BTC", legs, testSpot, feeRate); err != nil {
		t.Fatalf("execute: %v", err)
	}
	v, _ := b.AccountView(id, map[string]float64{"BTC": testSpot})
	if v.MarginUsedUSD <= 0 {
		t.Fatal("basket reserved no margin")
	}

	if _, err := b.EditAccount(id, "", 1.0); err == nil {
		t.Fatalf("capital cut to $1 accepted while $%.2f is reserved", v.MarginUsedUSD)
	}
}

// The gate: a basket the account cannot afford must be REFUSED, not partially
// filled. A partial fill on a hedged structure is the dangerous outcome.
func TestExecuteBasket_RejectsWhenCapitalInsufficient(t *testing.T) {
	b, _ := newTestBook(t)
	small, err := b.CreateAccount("Tiny", 50)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	legs := []Leg{
		leg(TypeCall, SideSell, 65000, 1000, 1500),
		leg(TypePut, SideSell, 65000, 1000, 1500),
	}
	pos, err := b.ExecuteBasket(small.ID, "BTC", legs, testSpot, feeRate)
	if err == nil {
		t.Fatal("a $50 account was allowed to sell a 1,000-lot strangle")
	}
	if pos != nil {
		t.Fatal("a rejected basket still produced a position")
	}

	var insuff ErrInsufficientCapital
	if !errors.As(err, &insuff) {
		t.Fatalf("error type = %T, want ErrInsufficientCapital with the numbers", err)
	}
	if insuff.Shortfall <= 0 {
		t.Error("shortfall must be reported so the UI can say how much is missing")
	}

	// Nothing may have been recorded.
	if got := len(b.Positions(small.ID, false)); got != 0 {
		t.Fatalf("%d position(s) recorded after a rejected basket — fills must be atomic", got)
	}
}

// The hedge benefit must be usable, not just displayed: a condor an account
// cannot afford as a naked strangle should still execute.
func TestExecuteBasket_HedgedBasketFitsWhereNakedDoesNot(t *testing.T) {
	b, _ := newTestBook(t)
	acct, _ := b.CreateAccount("Hedged", 400)

	naked := []Leg{
		leg(TypeCall, SideSell, 65000, 100, 1500),
		leg(TypePut, SideSell, 65000, 100, 1500),
	}
	if _, err := b.ExecuteBasket(acct.ID, "BTC", naked, testSpot, feeRate); err == nil {
		t.Fatal("naked strangle fit inside $400; the test needs a tighter account")
	}

	condor := append(append([]Leg{}, naked...),
		leg(TypeCall, SideBuy, 70000, 100, 400),
		leg(TypePut, SideBuy, 60000, 100, 400),
	)
	pos, err := b.ExecuteBasket(acct.ID, "BTC", condor, testSpot, feeRate)
	if err != nil {
		t.Fatalf("hedged condor rejected from the same account: %v", err)
	}
	if pos.Label != "Iron Condor" {
		t.Errorf("label = %q, want Iron Condor", pos.Label)
	}
}

// A basket with an unpriced leg must not fill: filling at a premium of zero
// would book a free option.
func TestExecuteBasket_RefusesUnpricedLeg(t *testing.T) {
	b, id := newTestBook(t)
	bad := leg(TypeCall, SideSell, 65000, 1, 0) // no premium
	if _, err := b.ExecuteBasket(id, "BTC", []Leg{bad}, testSpot, feeRate); err == nil {
		t.Fatal("a leg with no live premium was filled")
	}
}

// Entry fees are paid whether or not the basket profits, so they must hit the
// book immediately rather than being deferred to exit.
func TestExecuteBasket_ChargesEntryFeesImmediately(t *testing.T) {
	b, id := newTestBook(t)
	before, _ := b.AccountView(id, nil)

	legs := []Leg{leg(TypeCall, SideSell, 65000, 10, 1500)}
	pos, err := b.ExecuteBasket(id, "BTC", legs, testSpot, feeRate)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	after, _ := b.AccountView(id, nil)

	if pos.FeesUSD <= 0 {
		t.Fatal("no entry fee charged")
	}
	if after.RealisedPnlUSD >= before.RealisedPnlUSD {
		t.Errorf("realised P&L did not fall by the entry fee (%.4f -> %.4f)",
			before.RealisedPnlUSD, after.RealisedPnlUSD)
	}
}

func TestCloseBasket_BooksResultAndFreesMargin(t *testing.T) {
	b, id := newTestBook(t)
	legs := []Leg{leg(TypeCall, SideSell, 65000, 10, 1500)}
	pos, err := b.ExecuteBasket(id, "BTC", legs, testSpot, feeRate)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	held, _ := b.AccountView(id, map[string]float64{"BTC": testSpot})
	if held.MarginUsedUSD <= 0 {
		t.Fatal("open basket reserved nothing")
	}

	if _, err := b.CloseBasket(id, pos.ID, testSpot, feeRate, "manual"); err != nil {
		t.Fatalf("close: %v", err)
	}

	freed, _ := b.AccountView(id, map[string]float64{"BTC": testSpot})
	if freed.MarginUsedUSD != 0 {
		t.Errorf("margin still $%.2f after close; a closed basket reserves nothing", freed.MarginUsedUSD)
	}
	if freed.OpenBaskets != 0 {
		t.Errorf("%d baskets still open after close", freed.OpenBaskets)
	}
}

func TestDeleteAccount_RefusesWithOpenPositions(t *testing.T) {
	b, _ := newTestBook(t)
	acct, _ := b.CreateAccount("Doomed", 100_000)
	if _, err := b.ExecuteBasket(acct.ID, "BTC", []Leg{leg(TypeCall, SideSell, 65000, 1, 1500)}, testSpot, feeRate); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if err := b.DeleteAccount(acct.ID); err == nil {
		t.Fatal("deleted an account with open positions, orphaning them")
	}
}

func TestDeleteAccount_RefusesLastAccount(t *testing.T) {
	b, id := newTestBook(t)
	if err := b.DeleteAccount(id); err == nil {
		t.Fatal("deleted the only account, leaving the desk unusable")
	}
}

// Two accounts are separate books: one must not hedge or fund the other.
func TestAccounts_DoNotNetAcrossBooks(t *testing.T) {
	b, first := newTestBook(t)
	second, _ := b.CreateAccount("Second", 100_000)

	if _, err := b.ExecuteBasket(first, "BTC", []Leg{leg(TypeCall, SideSell, 65000, 100, 1500)}, testSpot, feeRate); err != nil {
		t.Fatalf("execute: %v", err)
	}

	v2, _ := b.AccountView(second.ID, map[string]float64{"BTC": testSpot})
	if v2.MarginUsedUSD != 0 {
		t.Errorf("second account shows $%.2f margin from the first account's trade", v2.MarginUsedUSD)
	}
	if v2.OpenBaskets != 0 {
		t.Errorf("second account shows %d baskets belonging to the first", v2.OpenBaskets)
	}
}

func TestLabelFor_NamesCommonStructures(t *testing.T) {
	cases := []struct {
		want string
		legs []Leg
	}{
		{"Short Strangle", []Leg{leg(TypeCall, SideSell, 65000, 1, 100), leg(TypePut, SideSell, 65000, 1, 100)}},
		{"Iron Condor", []Leg{
			leg(TypeCall, SideSell, 65000, 1, 100), leg(TypePut, SideSell, 65000, 1, 100),
			leg(TypeCall, SideBuy, 70000, 1, 20), leg(TypePut, SideBuy, 60000, 1, 20)}},
		{"Call Spread", []Leg{leg(TypeCall, SideSell, 65000, 1, 100), leg(TypeCall, SideBuy, 66000, 1, 60)}},
		{"Long Strangle", []Leg{leg(TypeCall, SideBuy, 66000, 1, 100), leg(TypePut, SideBuy, 64000, 1, 100)}},
	}
	for _, c := range cases {
		if got := LabelFor(c.legs); got != c.want {
			t.Errorf("LabelFor = %q, want %q", got, c.want)
		}
	}
}

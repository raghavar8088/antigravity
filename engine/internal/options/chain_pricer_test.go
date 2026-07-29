package options

import (
	"testing"
	"time"
)

// Phase 2b swapped the synthetic Black-Scholes chain for the real Delta chain on
// the option-BUYING desk. These pin the behaviours that swap requires.

type stubPricer struct {
	quote      ChainQuote
	resolveErr error
	marks      map[string]float64
	resolves   int
}

func (s *stubPricer) ResolveEntry(optType string, strike float64, expiry time.Time) (ChainQuote, error) {
	s.resolves++
	if s.resolveErr != nil {
		return ChainQuote{}, s.resolveErr
	}
	return s.quote, nil
}

func (s *stubPricer) MarkFor(symbol string) (float64, bool) {
	m, ok := s.marks[symbol]
	return m, ok
}

// With no pricer set the desk must behave exactly as before, or the A/B
// comparison between model and venue is not a comparison.
func TestChainPricer_NilKeepsBlackScholes(t *testing.T) {
	e := NewEngine()
	if e.UsingRealChain() {
		t.Fatal("a fresh engine must default to the model, not the venue")
	}
	e.SetChainPricer(&stubPricer{})
	if !e.UsingRealChain() {
		t.Fatal("SetChainPricer did not switch the desk onto the venue")
	}
	e.SetChainPricer(nil)
	if e.UsingRealChain() {
		t.Fatal("passing nil must restore model pricing")
	}
}

// A position priced on the venue must be re-priced on the venue. Falling back to
// the model would mark a venue entry against a model exit and book the gap as
// P&L that never existed.
func TestMarkToMarket_UsesVenueMarkForVenuePosition(t *testing.T) {
	e := NewEngine()
	e.SetChainPricer(&stubPricer{marks: map[string]float64{"C-BTC-64000-300726": 900}})

	pos := &OptionPosition{
		OptionType: Call, Strike: 64000, ExpiryTime: time.Now().Add(24 * time.Hour),
		EntryPremium: 500, CurrentPremium: 500, Quantity: 0.01,
		ContractSymbol: "C-BTC-64000-300726",
	}
	e.lastPrice = 64000

	e.markToMarketPositionLocked(pos, 0.5, 0.8, 0.5, time.Now())

	if pos.CurrentPremium != 900 {
		t.Fatalf("CurrentPremium = %.2f, want the venue mark 900", pos.CurrentPremium)
	}
}

// A missing quote is missing information, not a new valuation. Substituting a
// model price would silently invent a mark the market never showed.
func TestMarkToMarket_HoldsLastPremiumWhenQuoteMissing(t *testing.T) {
	e := NewEngine()
	e.SetChainPricer(&stubPricer{marks: map[string]float64{}}) // nothing quoted

	pos := &OptionPosition{
		OptionType: Call, Strike: 64000, ExpiryTime: time.Now().Add(24 * time.Hour),
		EntryPremium: 500, CurrentPremium: 777, Quantity: 0.01,
		ContractSymbol: "C-BTC-64000-300726",
	}
	e.lastPrice = 64000

	e.markToMarketPositionLocked(pos, 0.5, 0.8, 0.5, time.Now())

	if pos.CurrentPremium != 777 {
		t.Fatalf("CurrentPremium = %.2f; an unquoted contract must HOLD its last mark, not take a model price",
			pos.CurrentPremium)
	}
}

// A position with no venue symbol (model-priced) must keep using the model even
// when a pricer is attached, so switching sources mid-flight cannot corrupt
// positions opened under the other regime.
func TestMarkToMarket_ModelPositionUnaffectedByPricer(t *testing.T) {
	e := NewEngine()
	e.SetChainPricer(&stubPricer{marks: map[string]float64{"OTHER": 12345}})

	pos := &OptionPosition{
		OptionType: Call, Strike: 64000, ExpiryTime: time.Now().Add(24 * time.Hour),
		EntryPremium: 500, CurrentPremium: 500, Quantity: 0.01,
		ContractSymbol: "", // opened under the model
	}
	e.lastPrice = 64000

	e.markToMarketPositionLocked(pos, 0.5, 0.8, 0.5, time.Now())

	if pos.CurrentPremium == 12345 {
		t.Fatal("a model-priced position took a venue mark for a different contract")
	}
}

func TestChainSkips_CountersStartClean(t *testing.T) {
	e := NewEngine()
	s := e.ChainSkips()
	if s.NoContract != 0 || s.NoMark != 0 || s.Filled != 0 {
		t.Fatalf("counters not zero on a fresh engine: %+v", s)
	}
}

func TestChainQuote_SpreadPct(t *testing.T) {
	q := ChainQuote{PremiumPerBTC: 411.79, Bid: 408, Ask: 414}
	got := q.SpreadPct()
	// The real C-BTC-64000-300726 quote measured on Delta: ~1.46%.
	if got < 0.014 || got > 0.016 {
		t.Errorf("SpreadPct() = %.4f, want ~0.0146", got)
	}
	// One-sided book reports 0 rather than a fabricated spread.
	if got := (ChainQuote{PremiumPerBTC: 400, Ask: 410}).SpreadPct(); got != 0 {
		t.Errorf("one-sided SpreadPct() = %.4f, want 0", got)
	}
}

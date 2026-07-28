package delta

import (
	"testing"
	"time"
)

// Custody invariant: a position this app opened survives a restart, so the
// monitor keeps managing it to SL/TP instead of orphaning real money.
func TestCustody_TradesSurviveRestart(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())

	b1 := &Bridge{openByPaperID: map[string]string{}}
	b1.trades = []LiveTrade{{
		ID: "DLT-0007", PaperTradeID: "paper-7", Status: "OPEN",
		DeltaSymbol: "P-BTC-64800-290726", ProductID: 4242, Contracts: 1,
		FillPrice: 0.51, OpenedAt: time.Now().UTC(),
	}}
	b1.PersistTrades()

	// A fresh process (empty memory) restores custody from disk.
	b2 := &Bridge{openByPaperID: map[string]string{}}
	b2.RestoreTrades()

	if len(b2.OpenTrades()) != 1 {
		t.Fatalf("restart must not orphan the open position, got %d open", len(b2.OpenTrades()))
	}
	got := b2.OpenTrades()[0]
	if got.ID != "DLT-0007" || got.ProductID != 4242 {
		t.Fatalf("restored the wrong trade: %+v", got)
	}
	// The open index must be rebuilt so OnClose can find it by paper id.
	if _, ok := b2.openByPaperID["paper-7"]; !ok {
		t.Fatal("open index must be rebuilt on restore so the position can be closed")
	}
	// Sequence continues so a new trade cannot collide with a restored id.
	if b2.seq != 7 {
		t.Fatalf("seq must resume past restored ids, got %d", b2.seq)
	}
}

func TestCustody_RestoreWithNoFileIsSafe(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	b := &Bridge{openByPaperID: map[string]string{}}
	b.RestoreTrades() // must not panic on a fresh install
	if len(b.OpenTrades()) != 0 {
		t.Fatal("no custody file means no open trades")
	}
}

func TestIsOptionSymbol_OnlyAdoptsOptions(t *testing.T) {
	for _, s := range []string{"P-BTC-64800-290726", "C-BTC-66000-070826", "c-eth-1-1"} {
		if !IsOptionSymbol(s) {
			t.Fatalf("%s should be adoptable (option)", s)
		}
	}
	// Perps/spot belong to other desks and must never be adopted by the Live Engine.
	for _, s := range []string{"BTCUSD", "ETHUSD", "BTC-USD", ""} {
		if IsOptionSymbol(s) {
			t.Fatalf("%s must NOT be adopted (not an option)", s)
		}
	}
}

func TestOptionTypeFromSymbol(t *testing.T) {
	if optionTypeFromSymbol("P-BTC-64800-290726") != "PUT" {
		t.Fatal("P- prefix is a PUT")
	}
	if optionTypeFromSymbol("C-BTC-66000-070826") != "CALL" {
		t.Fatal("C- prefix is a CALL")
	}
}

package orderbook

import (
	"math"
	"testing"
)

// Of every feed in this application the order book is the one where using
// another exchange makes least sense: a bid wall, a spread, a depth imbalance
// are facts about the resting orders in ONE book. These tests pin the two ways
// the Binance -> Delta move could silently produce a wrong book.

// Verbatim from the live Delta socket (wss://socket.india.delta.exchange,
// channel l2_orderbook). Note "buy"/"sell", not "bids"/"asks", and note that
// size is a NUMBER OF CONTRACTS.
const realDeltaL2 = `{"type":"l2_orderbook","symbol":"BTCUSD","buy":[{"depth":"4433","limit_price":"63047.5","size":4433},{"depth":"9856","limit_price":"63047.0","size":5423}],"sell":[{"depth":"3288","limit_price":"63048.5","size":3288},{"depth":"7000","limit_price":"63049.0","size":3712}]}`

// TRAP 1: size is in contracts. Unscaled, a 4,433-contract bid reads as 4,433
// BTC instead of 4.433 — a thousandfold overstatement of liquidity that would
// make every wall threshold calibrated in BTC stop firing.
func TestDepth_ScalesContractsToBTC(t *testing.T) {
	d := NewDepthSubscriber()
	d.processMessage([]byte(realDeltaL2))

	d.mu.RLock()
	book := d.book
	d.mu.RUnlock()

	if len(book.Bids) != 2 || len(book.Asks) != 2 {
		t.Fatalf("book has %d bids / %d asks, want 2/2", len(book.Bids), len(book.Asks))
	}
	if math.Abs(book.Bids[0].Quantity-4.433) > 1e-9 {
		t.Errorf("top bid quantity %v, want 4.433 BTC (4433 contracts x 0.001)", book.Bids[0].Quantity)
	}
	if math.Abs(book.Asks[0].Quantity-3.288) > 1e-9 {
		t.Errorf("top ask quantity %v, want 3.288 BTC", book.Asks[0].Quantity)
	}
	if book.Symbol != depthSymbol {
		t.Errorf("symbol %q, want %q", book.Symbol, depthSymbol)
	}
}

// TRAP 2: the field names changed. Decoding Binance's "bids"/"asks" against
// Delta's payload yields two empty slices — an order book that is silently,
// permanently flat, with no error anywhere.
func TestDepth_ReadsDeltaFieldNamesNotBinances(t *testing.T) {
	d := NewDepthSubscriber()
	d.processMessage([]byte(realDeltaL2))

	d.mu.RLock()
	empty := len(d.book.Bids) == 0 && len(d.book.Asks) == 0
	d.mu.RUnlock()
	if empty {
		t.Fatal("book is empty after a real Delta payload — the decoder is reading the wrong field names")
	}
}

// Prices must still sort correctly: bids descending, asks ascending.
func TestDepth_SortsSides(t *testing.T) {
	d := NewDepthSubscriber()
	d.processMessage([]byte(realDeltaL2))

	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.book.Bids[0].Price < d.book.Bids[1].Price {
		t.Error("bids not sorted descending")
	}
	if d.book.Asks[0].Price > d.book.Asks[1].Price {
		t.Error("asks not sorted ascending")
	}
}

// The subscribe acknowledgement and any other channel must not blank the book.
// Overwriting a good book with an empty one on every unrelated message would
// make depth analysis flicker between real and flat.
func TestDepth_IgnoresNonDepthMessages(t *testing.T) {
	d := NewDepthSubscriber()
	d.processMessage([]byte(realDeltaL2))

	for _, other := range []string{
		`{"type":"subscriptions","channels":[{"name":"l2_orderbook","symbols":["BTCUSD"]}]}`,
		`{"type":"v2/ticker","symbol":"BTCUSD","mark_price":"63046.3"}`,
		`{"type":"l2_orderbook","symbol":"BTCUSD","buy":[],"sell":[]}`,
		`not json`,
	} {
		d.processMessage([]byte(other))
		d.mu.RLock()
		n := len(d.book.Bids)
		d.mu.RUnlock()
		if n == 0 {
			t.Fatalf("message %q emptied the order book", other)
		}
	}
}

// Malformed levels are dropped rather than admitted at zero. A zero-priced level
// would corrupt the mid and every metric derived from it.
func TestDepth_DropsUnusableLevels(t *testing.T) {
	got := parseDeltaLevels([]deltaDepthLevel{
		{LimitPrice: "63000.0", Size: 100},
		{LimitPrice: "0", Size: 100},
		{LimitPrice: "abc", Size: 100},
		{LimitPrice: "63001.0", Size: 0},
		{LimitPrice: "63002.0", Size: -5},
	})
	if len(got) != 1 {
		t.Fatalf("kept %d levels, want 1 — only the first is usable", len(got))
	}
	if got[0].Price != 63000.0 || math.Abs(got[0].Quantity-0.1) > 1e-9 {
		t.Errorf("kept level %+v, want price 63000 quantity 0.1", got[0])
	}
}

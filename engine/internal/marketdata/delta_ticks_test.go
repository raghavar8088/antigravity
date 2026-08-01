package marketdata

import (
	"math"
	"testing"
	"time"
)

// These payloads are VERBATIM from the live Delta India socket
// (wss://socket.india.delta.exchange), captured before the client was written.
// Delta's schema has already caused one incident in this codebase — a nested vs
// flat field on /v2/tickers that would have turned 457 contracts into 0 marks —
// so the parser is pinned against real bytes rather than against documentation.

const (
	// A live trade. price is a STRING, size is a NUMBER OF CONTRACTS, and
	// timestamp is MICROSECONDS.
	realDeltaTradeBuy  = `{"size":100,"timestamp":1785518171665030,"type":"all_trades","symbol":"BTCUSD","product_id":27,"price":"62881.0","buyer_role":"taker","seller_role":"maker"}`
	realDeltaTradeSell = `{"size":40,"timestamp":1785518170279135,"type":"all_trades","symbol":"BTCUSD","product_id":27,"price":"62881.0","buyer_role":"maker","seller_role":"taker"}`

	// The subscribe acknowledgement.
	realDeltaSubAck = `{"channels":[{"name":"all_trades","symbols":["BTCUSD"]}],"type":"subscriptions"}`

	// The initial all_trades SNAPSHOT: recent HISTORY, no "type" field.
	realDeltaSnapshot = `{"symbol":"BTCUSD","trades":[{"buyer_role":"maker","price":"62881.0","seller_role":"taker","size":40,"timestamp":1785518170279135},{"buyer_role":"taker","price":"62878.5","seller_role":"maker","size":3,"timestamp":1785518169907290}]}`

	// A ticker message, which is where contract_value comes from.
	realDeltaTicker = `{"mark_price":"62876.14476717","size":12894393,"timestamp":1785518170109175,"type":"v2/ticker","symbol":"BTCUSD","spot_price":"62891.8","contract_value":"0.001","volume":12894.393000010552}`
)

func drain(c *DeltaTickClient) []Tick {
	var out []Tick
	for {
		select {
		case t := <-c.ch:
			out = append(out, t)
		default:
			return out
		}
	}
}

// TRAP 1: size is in CONTRACTS. Coinbase reported a 0.1 BTC trade as 0.1; Delta
// reports it as 100. Passing that through unscaled inflates every volume, CVD
// and liquidity reading a thousandfold.
func TestDeltaTicks_SizeIsScaledFromContractsToBTC(t *testing.T) {
	c := NewDeltaTickClient()
	c.handleMessage([]byte(realDeltaTradeBuy))

	got := drain(c)
	if len(got) != 1 {
		t.Fatalf("expected 1 tick, got %d", len(got))
	}
	// 100 contracts x 0.001 BTC = 0.1 BTC.
	if math.Abs(got[0].Quantity-0.1) > 1e-9 {
		t.Errorf("quantity %v; want 0.1 BTC — a raw contract count would be 100, a 1000x overstatement",
			got[0].Quantity)
	}
}

// The venue's own contract value must win over the assumed constant, so a
// relisting cannot silently rescale what every strategy sees as volume.
func TestDeltaTicks_ContractValueIsLearnedFromTheVenue(t *testing.T) {
	c := NewDeltaTickClient()
	// Venue says 0.01, not the assumed 0.001.
	c.handleMessage([]byte(`{"type":"v2/ticker","symbol":"BTCUSD","contract_value":"0.01"}`))
	c.handleMessage([]byte(realDeltaTradeBuy))

	got := drain(c)
	if len(got) != 1 {
		t.Fatalf("expected 1 tick, got %d", len(got))
	}
	if math.Abs(got[0].Quantity-1.0) > 1e-9 {
		t.Errorf("quantity %v; want 1.0 (100 x 0.01) — the venue's value must override the default",
			got[0].Quantity)
	}
}

func TestDeltaTicks_FallsBackToListedContractValue(t *testing.T) {
	c := NewDeltaTickClient()
	if got := c.contractValueFor("BTCUSD"); got != deltaDefaultContractValueBTC {
		t.Errorf("before any ticker, contract value = %v, want %v", got, deltaDefaultContractValueBTC)
	}
}

// TRAP 2: timestamps are microseconds. Read as milliseconds, 1785518171665030
// lands in the year 58500 — every tick infinitely in the future, and any
// staleness check comparing against now silently broken.
func TestDeltaTicks_TimestampIsMicrosecondsNotMillis(t *testing.T) {
	c := NewDeltaTickClient()
	c.handleMessage([]byte(realDeltaTradeBuy))

	got := drain(c)
	if len(got) != 1 {
		t.Fatalf("expected 1 tick, got %d", len(got))
	}
	when := time.UnixMilli(got[0].TimeMs).UTC()
	if when.Year() != 2026 {
		t.Fatalf("tick timestamp resolved to %s (year %d); the venue sends microseconds",
			when.Format(time.RFC3339), when.Year())
	}
	// Sanity: it must not be wildly in the future relative to the wall clock.
	if when.After(time.Now().UTC().Add(365 * 24 * time.Hour)) {
		t.Errorf("tick time %s is more than a year ahead of now", when)
	}
}

// TRAP 3: the first all_trades message is a snapshot of recent HISTORY with no
// "type" field. Emitting it would replay stale trades into the strategies at
// connect time — and again on every single reconnect.
func TestDeltaTicks_SnapshotIsNotReplayedAsLiveTicks(t *testing.T) {
	c := NewDeltaTickClient()
	c.handleMessage([]byte(realDeltaSnapshot))

	if got := drain(c); len(got) != 0 {
		t.Fatalf("the connect-time snapshot produced %d live tick(s); it is history, not new trades", len(got))
	}
}

func TestDeltaTicks_SubscribeAckIsIgnored(t *testing.T) {
	c := NewDeltaTickClient()
	c.handleMessage([]byte(realDeltaSubAck))
	if got := drain(c); len(got) != 0 {
		t.Fatalf("the subscribe ack produced %d tick(s)", len(got))
	}
}

// The aggressor is the taker. Getting this backwards inverts CVD, which is the
// input to a whole family of strategies here.
func TestDeltaTicks_SideIsTheTakersSide(t *testing.T) {
	c := NewDeltaTickClient()
	c.handleMessage([]byte(realDeltaTradeBuy))  // buyer_role: taker
	c.handleMessage([]byte(realDeltaTradeSell)) // seller_role: taker

	got := drain(c)
	if len(got) != 2 {
		t.Fatalf("expected 2 ticks, got %d", len(got))
	}
	if got[0].Side != "BUY" {
		t.Errorf("buyer_role=taker gave %q, want BUY", got[0].Side)
	}
	if got[1].Side != "SELL" {
		t.Errorf("seller_role=taker gave %q, want SELL", got[1].Side)
	}
}

func TestDeltaTicks_PriceParsesFromAQuotedString(t *testing.T) {
	c := NewDeltaTickClient()
	c.handleMessage([]byte(realDeltaTradeBuy))
	got := drain(c)
	if len(got) != 1 {
		t.Fatalf("expected 1 tick, got %d", len(got))
	}
	if math.Abs(got[0].Price-62881.0) > 1e-6 {
		t.Errorf("price %v, want 62881.0", got[0].Price)
	}
	if got[0].Symbol != "BTCUSD" {
		t.Errorf("symbol %q, want BTCUSD", got[0].Symbol)
	}
}

// Malformed or empty payloads must be dropped, never emitted as a zero-price
// tick. A zero price reaching the strategies is worse than no tick at all.
func TestDeltaTicks_RejectsUnusablePayloads(t *testing.T) {
	for _, raw := range []string{
		`not json`,
		`{"type":"all_trades","symbol":"BTCUSD","price":"0","size":100,"timestamp":1785518171665030}`,
		`{"type":"all_trades","symbol":"BTCUSD","price":"62881.0","size":0,"timestamp":1785518171665030}`,
		`{"type":"all_trades","symbol":"BTCUSD","price":"abc","size":100,"timestamp":1785518171665030}`,
		`{}`,
	} {
		c := NewDeltaTickClient()
		c.handleMessage([]byte(raw))
		if got := drain(c); len(got) != 0 {
			t.Errorf("payload %q produced a tick %+v; it should have been dropped", raw, got[0])
		}
	}
}

// A slow consumer must not block the socket reader — falling behind on reads is
// what turns a busy market into a disconnect. The loss must be counted, not
// silent.
func TestDeltaTicks_DropsRatherThanBlockingWhenTheConsumerStalls(t *testing.T) {
	c := NewDeltaTickClient()
	c.ch = make(chan Tick, 1) // stand in for a consumer that has stopped reading

	for i := 0; i < 5; i++ {
		c.handleMessage([]byte(realDeltaTradeBuy))
	}
	if c.dropped.Load() == 0 {
		t.Error("no drops recorded; a full buffer must be counted so the loss is visible")
	}
	if c.ticks.Load() != 1 {
		t.Errorf("accepted %d ticks into a buffer of 1", c.ticks.Load())
	}
}

// Health has to distinguish a quiet market from a dead feed.
func TestDeltaTicks_HealthReportsSourceAndCounts(t *testing.T) {
	c := NewDeltaTickClient()
	if h := c.Health(); h.Source != tickSourceNone {
		t.Errorf("before connecting, source = %q, want %q", h.Source, tickSourceNone)
	}
	c.handleMessage([]byte(realDeltaTradeBuy))
	h := c.Health()
	if h.Source != tickSourceDelta {
		t.Errorf("after a Delta tick, source = %q, want %q", h.Source, tickSourceDelta)
	}
	if h.Ticks != 1 {
		t.Errorf("Ticks = %d, want 1", h.Ticks)
	}
	if h.LastTickAt.IsZero() {
		t.Error("LastTickAt not set; a stalled feed would be indistinguishable from a quiet one")
	}
}

// It must be substitutable for the Coinbase client it replaces.
func TestDeltaTickClient_SatisfiesMarketDataClient(t *testing.T) {
	var _ MarketDataClient = NewDeltaTickClient()
}

func TestDeltaTicks_ConnectRejectsAnEmptySymbolList(t *testing.T) {
	if err := NewDeltaTickClient().Connect(nil, nil); err == nil {
		t.Error("Connect with no symbols must fail rather than open a socket that streams nothing")
	}
}

// The deployed desks are configured with Coinbase product IDs from before the
// venue moved. Delta lists nothing called "BTC-USD", and subscribing to a symbol
// that does not exist fails SILENTLY — no error, no reconnect, just a feed that
// never delivers a tick. Every existing deployment would have broken that way.
func TestDeltaSymbolFor_TranslatesDeployedConfigurations(t *testing.T) {
	cases := map[string]string{
		// What the running containers actually set today.
		"BTC-USD": "BTCUSD",
		"ETH-USD": "ETHUSD",
		// Binance notation, used by the warmup env vars.
		"BTCUSDT": "BTCUSD",
		"ETHUSDT": "ETHUSD",
		// Already Delta notation — must pass through untouched.
		"BTCUSD": "BTCUSD",
		"SOLUSD": "SOLUSD",
		// Tolerant of case and whitespace from a hand-edited .env.
		" btc-usd ": "BTCUSD",
		"":          "",
	}
	for in, want := range cases {
		if got := DeltaSymbolFor(in); got != want {
			t.Errorf("DeltaSymbolFor(%q) = %q, want %q", in, got, want)
		}
	}
}

// Delta quotes size in contracts on EVERY surface — trades, ticker and candle
// history. The exported constant is what the REST paths divide by, and it must
// stay in step with the default the socket client assumes, or the two adapters
// would disagree about what a unit of volume means.
func TestDeltaContractValue_IsConsistentAcrossAdapters(t *testing.T) {
	if DeltaContractValueBTC != deltaDefaultContractValueBTC {
		t.Fatalf("exported %v != socket default %v; REST history and the tick stream would scale volume differently",
			DeltaContractValueBTC, deltaDefaultContractValueBTC)
	}
	if DeltaContractValueBTC <= 0 {
		t.Fatal("contract value must be positive; a zero would erase all volume")
	}
}

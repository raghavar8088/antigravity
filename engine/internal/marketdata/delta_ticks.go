package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Delta Exchange tick stream.
//
// This engine executes on Delta. Until now its live tick feed came from
// Coinbase spot (BTC-USD), so 600 strategies were scored on one venue's trades
// and the orders went to another's book. Those are different instruments —
// Coinbase BTC-USD is spot, Delta BTCUSD is a perpetual future with its own
// basis, its own liquidity and its own microstructure — and a strategy tuned on
// the first is not evidence about the second.
//
// DeltaTickClient replaces it with Delta's own trade stream, so signal and
// execution finally read the same book.
//
// # Three things the venue swap had to get right
//
// All three were verified against the live socket before this was written, not
// inferred from documentation.
//
//  1. SIZE IS IN CONTRACTS, NOT BTC. Coinbase reports a trade of 0.1 BTC as
//     size 0.1; Delta reports the same trade as size 100, because one BTCUSD
//     contract is 0.001 BTC. Passing that through unscaled would hand every
//     volume-, CVD- and liquidity-based strategy numbers inflated a
//     thousandfold. The contract value is learned from the venue's own ticker
//     rather than hardcoded — see contractValueFor.
//
//  2. TIMESTAMPS ARE MICROSECONDS. Coinbase sends RFC3339; Delta sends
//     1785518171665030. Read as milliseconds that is the year 58500, which
//     would put every tick infinitely far in the future and silently break any
//     staleness check that compares against now.
//
//  3. THE FIRST MESSAGE IS A SNAPSHOT, NOT A TRADE. Subscribing to all_trades
//     returns {"symbol":...,"trades":[...]} — a batch of recent history with no
//     "type" field. Emitting those as live ticks would replay old trades into
//     the strategies at connect time, and again on every reconnect.
//
// # Side convention
//
// Delta names both roles on every trade. The aggressor is the taker, so
// buyer_role=taker is a buy and seller_role=taker is a sell. That is
// unambiguous.
//
// It is worth knowing that this may not match the Coinbase-era convention. The
// old client read Coinbase's `side`, which on a match message is the side of the
// resting MAKER order, and mapped it directly. If that reading is right, CVD
// sign is inverted between the two eras. Either way the two records are not
// comparable, which is the same conclusion the venue change forces on its own.

// tickSourceDelta and friends label which venue a tick actually came from, so a
// desk running on fallback prices is visible rather than assumed.
const (
	tickSourceDelta   = "delta"
	tickSourceFallbwk = "fallback"
	tickSourceNone    = "none"
)

// defaultDeltaWSURL is Delta India's public market-data socket. No auth: this is
// public trade data, and this client never sends credentials.
const defaultDeltaWSURL = "wss://socket.india.delta.exchange"

// DeltaContractValueBTC is BTC per contract on Delta's BTC perpetual.
//
// Delta quotes SIZE IN CONTRACTS everywhere — the trade stream, the ticker and
// the candle history — while Coinbase and Binance quote size in BTC. Every
// adapter from Delta into this engine has to divide by it, or volume readings
// arrive inflated a thousandfold.
//
// The tick client learns the live value from the venue's own ticker and warns on
// a mismatch (see learnContractValue); REST history carries no such field, so
// this constant is what those paths use.
const DeltaContractValueBTC = 0.001

// deltaDefaultContractValueBTC is the assumed BTC per contract until the venue's
// ticker states otherwise. A mismatch is logged loudly rather than silently
// rescaling every strategy's view of volume.
const deltaDefaultContractValueBTC = DeltaContractValueBTC

func deltaWSURL() string {
	if u := strings.TrimSpace(os.Getenv("DELTA_WS_URL")); u != "" {
		return u
	}
	return defaultDeltaWSURL
}

// DeltaTickClient streams live trades from Delta and satisfies MarketDataClient,
// so it drops into any orchestrator that previously took the Coinbase client.
type DeltaTickClient struct {
	ch      chan Tick
	symbols []string

	conn   *websocket.Conn
	connMu sync.Mutex

	// contractValue maps symbol -> BTC per contract, learned from the venue's
	// v2/ticker feed. Guarded by cvMu.
	contractValue map[string]float64
	cvMu          sync.RWMutex

	// Observability. A feed that has silently stopped delivering looks exactly
	// like a quiet market unless someone is counting.
	ticks      atomic.Int64
	dropped    atomic.Int64
	lastTickMs atomic.Int64
	source     atomic.Value // string
	lastErr    atomic.Value // string

	closed atomic.Bool

	// onAggTrade, when set, receives every trade in the AggTrade shape so CVD can
	// be computed from Delta's own taker classification. Previously CVD came from
	// a Binance aggTrade socket, which meant order flow was measured on a
	// different book from the one being traded — and CVD is precisely a
	// statement about who is lifting whose offers on THIS venue.
	onAggTrade func(AggTrade)
	hookMu     sync.RWMutex
}

// SetAggTradeHook registers a callback receiving every trade as an AggTrade.
// Set it before Connect.
func (c *DeltaTickClient) SetAggTradeHook(fn func(AggTrade)) {
	c.hookMu.Lock()
	c.onAggTrade = fn
	c.hookMu.Unlock()
}

// NewDeltaTickClient builds the client. Nothing connects until Connect.
func NewDeltaTickClient() *DeltaTickClient {
	c := &DeltaTickClient{
		// Same depth as the Coinbase client it replaces. Delta's perp is busy;
		// an undersized buffer would drop ticks during bursts, which is exactly
		// when the strategies most need them.
		ch:            make(chan Tick, 10000),
		contractValue: map[string]float64{},
	}
	c.source.Store(tickSourceNone)
	c.lastErr.Store("")
	return c
}

// Connect subscribes to the given symbols and keeps the socket alive.
func (c *DeltaTickClient) Connect(ctx context.Context, symbols []string) error {
	if len(symbols) == 0 {
		return fmt.Errorf("marketdata: DeltaTickClient needs at least one symbol")
	}
	c.symbols = symbols
	go c.keepConnected(ctx)
	return nil
}

func (c *DeltaTickClient) keepConnected(ctx context.Context) {
	// Backoff grows on repeated failure so a venue outage does not turn into a
	// reconnect storm against the same API this engine trades through.
	backoff := 2 * time.Second
	const maxBackoff = 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		if c.closed.Load() {
			return
		}

		if err := c.dial(); err != nil {
			c.noteError(err)
			c.source.Store(tickSourceNone)
			log.Printf("[DELTA TICKS] dial failed: %v — retrying in %s", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < maxBackoff {
				backoff *= 2
			}
			continue
		}

		backoff = 2 * time.Second // reset only after a connection actually succeeded
		c.listen(ctx)

		if c.closed.Load() || ctx.Err() != nil {
			return
		}
		c.source.Store(tickSourceNone)
		log.Printf("[DELTA TICKS] stream disconnected — reconnecting in 5s")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *DeltaTickClient) dial() error {
	url := deltaWSURL()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	// Subscribe to trades AND the ticker. The ticker is not used for pricing —
	// it is how the contract value is learned, so sizes are scaled by the
	// venue's own number instead of a constant that could drift out of date.
	sub := map[string]any{
		"type": "subscribe",
		"payload": map[string]any{
			"channels": []map[string]any{
				{"name": "all_trades", "symbols": c.symbols},
				{"name": "v2/ticker", "symbols": c.symbols},
			},
		},
	}
	if err := conn.WriteJSON(sub); err != nil {
		conn.Close()
		return fmt.Errorf("subscribe: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	log.Printf("[DELTA TICKS] connected to %s — streaming %s", url, strings.Join(c.symbols, ","))
	return nil
}

func (c *DeltaTickClient) listen(ctx context.Context) {
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return
	}
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// A read deadline is what turns a half-open socket into a reconnect. A
		// TCP connection to a venue that has stopped sending can stay "open"
		// indefinitely, and the desk would sit still believing the market had
		// gone quiet.
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			c.noteError(err)
			log.Printf("[DELTA TICKS] read error: %v", err)
			return
		}
		c.handleMessage(msg)
	}
}

// deltaTradeMsg is one live trade. price is a quoted string and size is a plain
// number of CONTRACTS; timestamp is microseconds since epoch.
type deltaTradeMsg struct {
	Type        string      `json:"type"`
	Symbol      string      `json:"symbol"`
	Price       interface{} `json:"price"`
	Size        interface{} `json:"size"`
	BuyerRole   string      `json:"buyer_role"`
	SellerRole  string      `json:"seller_role"`
	TimestampUS int64       `json:"timestamp"`
	ProductID   int64       `json:"product_id"`
	// ContractValue arrives on v2/ticker messages, not on trades.
	ContractValue interface{} `json:"contract_value"`
}

func (c *DeltaTickClient) handleMessage(msg []byte) {
	var m deltaTradeMsg
	if err := json.Unmarshal(msg, &m); err != nil {
		return
	}

	switch m.Type {
	case "v2/ticker":
		c.learnContractValue(m)
		return
	case "all_trades":
		c.emit(m)
		return
	case "subscriptions", "":
		// The subscribe acknowledgement, or the initial all_trades SNAPSHOT
		// ({"symbol":...,"trades":[...]}) which carries no type. Both are
		// deliberately dropped: the snapshot is recent HISTORY, and feeding it
		// to the strategies would replay old trades as if they had just
		// happened — once at startup and again on every reconnect.
		return
	default:
		return
	}
}

// learnContractValue records BTC-per-contract from the venue and warns when it
// differs from what this client assumed.
func (c *DeltaTickClient) learnContractValue(m deltaTradeMsg) {
	cv, err := parseFlexibleFloat(m.ContractValue)
	if err != nil || cv <= 0 {
		return
	}
	sym := m.Symbol
	if sym == "" {
		return
	}

	c.cvMu.Lock()
	prev, had := c.contractValue[sym]
	c.contractValue[sym] = cv
	c.cvMu.Unlock()

	if !had && cv != deltaDefaultContractValueBTC {
		log.Printf("[DELTA TICKS] %s contract value is %g BTC, not the assumed %g — sizes now scaled by the venue's value",
			sym, cv, deltaDefaultContractValueBTC)
	} else if had && prev != cv {
		log.Printf("[DELTA TICKS] %s contract value CHANGED %g -> %g", sym, prev, cv)
	}
}

// contractValueFor returns BTC per contract for a symbol, falling back to the
// listed BTCUSD value until the ticker has been seen.
func (c *DeltaTickClient) contractValueFor(symbol string) float64 {
	c.cvMu.RLock()
	cv, ok := c.contractValue[symbol]
	c.cvMu.RUnlock()
	if ok && cv > 0 {
		return cv
	}
	return deltaDefaultContractValueBTC
}

func (c *DeltaTickClient) emit(m deltaTradeMsg) {
	price, err := parseFlexibleFloat(m.Price)
	if err != nil || price <= 0 {
		return
	}
	contracts, err := parseFlexibleFloat(m.Size)
	if err != nil || contracts <= 0 {
		return
	}

	// Contracts -> BTC. Without this every downstream volume figure is inflated
	// by 1/contractValue (a thousandfold on BTCUSD).
	qty := contracts * c.contractValueFor(m.Symbol)

	// The aggressor is the taker.
	side := "SELL"
	if strings.EqualFold(m.BuyerRole, "taker") {
		side = "BUY"
	}

	// Microseconds -> milliseconds. Delta sends µs; every consumer here expects
	// ms, and the difference is a factor of a thousand on the clock.
	timeMs := m.TimestampUS / 1000
	if timeMs <= 0 {
		timeMs = time.Now().UnixMilli()
	}

	t := Tick{
		Symbol:   m.Symbol,
		Price:    price,
		Quantity: qty,
		Side:     side,
		// Delta does not publish a trade id on this channel. product_id keeps the
		// field meaningful (which instrument) rather than leaving it zero.
		TradeID: m.ProductID,
		TimeMs:  timeMs,
	}

	c.hookMu.RLock()
	hook := c.onAggTrade
	c.hookMu.RUnlock()
	if hook != nil {
		// IsBuyerMaker means the SELLER was the aggressor, which is exactly
		// buyer_role == "maker" on Delta.
		hook(AggTrade{
			Price:        price,
			Quantity:     qty,
			IsBuyerMaker: side == "SELL",
			Timestamp:    time.UnixMilli(timeMs).UTC(),
		})
	}

	select {
	case c.ch <- t:
		c.ticks.Add(1)
		c.lastTickMs.Store(timeMs)
		c.source.Store(tickSourceDelta)
	default:
		// Never block the socket reader on a slow consumer: falling behind on
		// reads is what turns a busy market into a disconnect. Dropped ticks are
		// counted so the loss is visible instead of silent.
		c.dropped.Add(1)
	}
}

func (c *DeltaTickClient) noteError(err error) {
	if err != nil {
		c.lastErr.Store(err.Error())
	}
}

// GetTickChannel satisfies MarketDataClient.
func (c *DeltaTickClient) GetTickChannel() <-chan Tick { return c.ch }

// Close stops reconnection and drops the socket.
func (c *DeltaTickClient) Close() error {
	c.closed.Store(true)
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// DeltaTickHealth is what an operator needs to tell a quiet market from a dead
// feed.
type DeltaTickHealth struct {
	Source     string    `json:"source"`
	Ticks      int64     `json:"ticks"`
	Dropped    int64     `json:"dropped"`
	LastTickAt time.Time `json:"lastTickAt"`
	AgeSeconds float64   `json:"ageSeconds"`
	LastError  string    `json:"lastError,omitempty"`
}

// Health reports the current state of the feed.
func (c *DeltaTickClient) Health() DeltaTickHealth {
	h := DeltaTickHealth{
		Source:  c.sourceString(),
		Ticks:   c.ticks.Load(),
		Dropped: c.dropped.Load(),
	}
	if ms := c.lastTickMs.Load(); ms > 0 {
		h.LastTickAt = time.UnixMilli(ms).UTC()
		h.AgeSeconds = time.Since(h.LastTickAt).Seconds()
	}
	if s, ok := c.lastErr.Load().(string); ok {
		h.LastError = s
	}
	return h
}

func (c *DeltaTickClient) sourceString() string {
	if s, ok := c.source.Load().(string); ok {
		return s
	}
	return tickSourceNone
}

// Source is the venue the most recent tick came from.
func (c *DeltaTickClient) Source() string { return c.sourceString() }

var _ MarketDataClient = (*DeltaTickClient)(nil)

// parseTickPrice is exported-for-test shorthand used by the unit tests to pin
// the string/number tolerance the venue actually requires.
func parseTickPrice(v interface{}) (float64, error) {
	switch t := v.(type) {
	case string:
		return strconv.ParseFloat(strings.TrimSpace(t), 64)
	default:
		return parseFlexibleFloat(v)
	}
}

// DeltaSymbolFor translates a symbol written in another venue's notation into
// Delta's.
//
// This exists because the deployed desks are configured with Coinbase product
// IDs — PRE_LIVE_FEED_SYMBOL=BTC-USD, ETH-USD — set long before the feed moved
// to Delta. Delta lists no product called "BTC-USD". Without translation the
// socket would subscribe to a symbol that does not exist and simply never
// deliver a tick: no error, no reconnect, no log line, just a desk that thinks
// the market has gone silent. Every existing deployment would have broken the
// same quiet way.
//
// A symbol already in Delta notation passes through untouched.
func DeltaSymbolFor(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return s
	}
	// Coinbase products are BASE-QUOTE. Delta perps are BASEQUOTE, and quote
	// "USD" on Coinbase is "USD" on Delta's perp too (BTC-USD -> BTCUSD).
	if i := strings.IndexByte(s, '-'); i > 0 {
		base, quote := s[:i], s[i+1:]
		// Binance-style USDT maps onto Delta's USD-quoted perpetual.
		if quote == "USDT" {
			quote = "USD"
		}
		return base + quote
	}
	// Binance spot notation (BTCUSDT) -> Delta perp (BTCUSD).
	if strings.HasSuffix(s, "USDT") {
		return strings.TrimSuffix(s, "USDT") + "USD"
	}
	return s
}

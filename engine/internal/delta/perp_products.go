package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Delta perpetual product registry.
//
// The Live Engine trades BTC options, where one contract is always 0.001 BTC and
// the constant can safely be hardcoded (see OptionContractSizeBTC). Perpetuals
// are not like that. Delta's own tickers report:
//
//	BTCUSD  contract_value 0.001
//	ETHUSD  contract_value 0.01
//	BNBUSD  contract_value 0.1
//	ADAUSD  contract_value 1.0
//	SOLUSD  contract_value 1.0
//	XRPUSD  contract_value 1.0
//
// A THOUSANDFOLD spread between the smallest and largest. Sizing an ADAUSD order
// with the options desk's 0.001 assumption would ask for a thousand times the
// intended position — on a $3,000 scalp ticket that is a $3,000,000 order. It
// would not be rejected for being nonsensical; it would be rejected, if at all,
// for insufficient margin, which looks like an unrelated problem.
//
// So no perpetual is tradeable here until its contract value has been read from
// the venue. SizeContracts refuses rather than guesses, and there is deliberately
// no default: a default is exactly how the 0.001 assumption would creep back in.

// PerpProduct is one listed perpetual, as Delta describes it.
type PerpProduct struct {
	Symbol string
	// ProductID is what POST /v2/orders needs; symbols are not accepted.
	ProductID int
	// ContractValue is the underlying quantity per contract (1 ADAUSD contract
	// = 1 ADA; 1 BTCUSD contract = 0.001 BTC).
	ContractValue float64
	// TickSize is the minimum price increment. A limit price off the tick is
	// rejected by the venue.
	TickSize float64
	// MarkPrice at the time the registry was refreshed. Used for sizing, not for
	// P&L.
	MarkPrice float64
	FetchedAt time.Time
}

// NotionalPerContract is the USD value of one contract at the given price.
func (p PerpProduct) NotionalPerContract(price float64) float64 {
	return price * p.ContractValue
}

// PerpRegistry caches Delta's perpetual products.
//
// It is read-mostly and refreshed on an interval: contract values and product
// IDs change only on a relisting, but they DO change, and a stale ID would route
// an order to the wrong instrument.
type PerpRegistry struct {
	mu        sync.RWMutex
	bySymbol  map[string]PerpProduct
	fetchedAt time.Time
	lastErr   string

	client  *http.Client
	baseURL string
}

// NewPerpRegistry builds an empty registry. Nothing is tradeable until Refresh
// succeeds.
func NewPerpRegistry() *PerpRegistry {
	base := strings.TrimSpace(os.Getenv("DELTA_API_BASE_URL"))
	if base == "" {
		base = "https://api.india.delta.exchange"
	}
	return &PerpRegistry{
		bySymbol: map[string]PerpProduct{},
		client:   &http.Client{Timeout: 20 * time.Second},
		baseURL:  strings.TrimRight(base, "/"),
	}
}

// perpTickerRow is Delta's /v2/tickers entry. Every numeric is a quoted string.
type perpTickerRow struct {
	Symbol        string `json:"symbol"`
	ProductID     int    `json:"product_id"`
	ContractType  string `json:"contract_type"`
	ContractValue string `json:"contract_value"`
	TickSize      string `json:"tick_size"`
	MarkPrice     string `json:"mark_price"`
}

type perpTickersResponse struct {
	Success bool            `json:"success"`
	Result  []perpTickerRow `json:"result"`
}

// Refresh reloads every listed perpetual from the venue.
func (r *PerpRegistry) Refresh(ctx context.Context) error {
	url := r.baseURL + "/v2/tickers?contract_types=perpetual_futures"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		r.noteErr(err.Error())
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		e := fmt.Errorf("delta tickers status %d", resp.StatusCode)
		r.noteErr(e.Error())
		return e
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<23))
	if err != nil {
		r.noteErr(err.Error())
		return err
	}
	var parsed perpTickersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		r.noteErr(err.Error())
		return err
	}
	if !parsed.Success {
		e := fmt.Errorf("delta tickers returned success=false")
		r.noteErr(e.Error())
		return e
	}

	next := make(map[string]PerpProduct, len(parsed.Result))
	now := time.Now().UTC()
	for _, row := range parsed.Result {
		p, ok := perpFromRow(row, now)
		if !ok {
			continue
		}
		next[p.Symbol] = p
	}
	if len(next) == 0 {
		e := fmt.Errorf("delta returned no usable perpetual products")
		r.noteErr(e.Error())
		return e
	}

	r.mu.Lock()
	r.bySymbol = next
	r.fetchedAt = now
	r.lastErr = ""
	r.mu.Unlock()
	return nil
}

// perpFromRow converts one ticker row, rejecting anything unusable.
//
// A product missing its contract value or product ID is DROPPED rather than
// admitted with a zero: a zero contract value would make every size calculation
// divide by zero, and a zero product ID would route an order to whatever Delta
// has at ID 0.
func perpFromRow(row perpTickerRow, now time.Time) (PerpProduct, bool) {
	if row.Symbol == "" || row.ProductID <= 0 {
		return PerpProduct{}, false
	}
	cv, err := strconv.ParseFloat(row.ContractValue, 64)
	if err != nil || cv <= 0 {
		return PerpProduct{}, false
	}
	tick, _ := strconv.ParseFloat(row.TickSize, 64)
	mark, _ := strconv.ParseFloat(row.MarkPrice, 64)
	return PerpProduct{
		Symbol:        row.Symbol,
		ProductID:     row.ProductID,
		ContractValue: cv,
		TickSize:      tick,
		MarkPrice:     mark,
		FetchedAt:     now,
	}, true
}

func (r *PerpRegistry) noteErr(msg string) {
	r.mu.Lock()
	r.lastErr = msg
	r.mu.Unlock()
}

// Lookup returns a perpetual by symbol.
func (r *PerpRegistry) Lookup(symbol string) (PerpProduct, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.bySymbol[strings.ToUpper(strings.TrimSpace(symbol))]
	return p, ok
}

// Count is how many perpetuals are known.
func (r *PerpRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bySymbol)
}

// Age is how long since the last successful refresh.
func (r *PerpRegistry) Age() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.fetchedAt.IsZero() {
		return time.Duration(1<<62 - 1)
	}
	return time.Since(r.fetchedAt)
}

// LastError is the most recent refresh failure, or "".
func (r *PerpRegistry) LastError() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastErr
}

// ErrUnknownPerp means the symbol has no entry in the registry, so its contract
// value is unknown and no order may be sized for it.
var ErrUnknownPerp = fmt.Errorf("delta: perpetual not in registry — contract value unknown, refusing to size an order")

// ErrPerpRegistryStale means the registry has not refreshed recently enough to
// be trusted for order routing.
var ErrPerpRegistryStale = fmt.Errorf("delta: perpetual registry is stale — refusing to size an order on old contract data")

// perpRegistryMaxAge is how old the registry may be and still be used to route
// an order. Contract values change only on a relisting, but a stale product ID
// routes to the wrong instrument, so this is deliberately short.
const perpRegistryMaxAge = 30 * time.Minute

// SizeContracts converts a USD notional into a whole number of contracts.
//
// It returns an error rather than a guess when the symbol is unknown or the
// registry is stale. There is no fallback contract value on purpose: the whole
// reason this type exists is that the options desk's 0.001 is wrong by up to a
// factor of a thousand for these instruments, and a default is how that
// assumption would return.
func (r *PerpRegistry) SizeContracts(symbol string, notionalUSD, price float64) (int, PerpProduct, error) {
	p, ok := r.Lookup(symbol)
	if !ok {
		return 0, PerpProduct{}, fmt.Errorf("%w: %s", ErrUnknownPerp, symbol)
	}
	if r.Age() > perpRegistryMaxAge {
		return 0, p, fmt.Errorf("%w: %s (age %s)", ErrPerpRegistryStale, symbol, r.Age().Truncate(time.Second))
	}
	if price <= 0 {
		price = p.MarkPrice
	}
	if price <= 0 {
		return 0, p, fmt.Errorf("delta: no usable price for %s", symbol)
	}
	if notionalUSD <= 0 {
		return 0, p, fmt.Errorf("delta: notional must be positive, got %.2f", notionalUSD)
	}

	perContract := p.NotionalPerContract(price)
	if perContract <= 0 {
		return 0, p, fmt.Errorf("delta: %s has a non-positive contract notional", symbol)
	}
	// Round DOWN. Rounding up would place a position larger than the caller
	// asked for, which on a leveraged perpetual is the direction that hurts.
	n := int(notionalUSD / perContract)
	if n < 1 {
		return 0, p, fmt.Errorf("delta: $%.2f is below one %s contract ($%.4f) — order would be zero-sized",
			notionalUSD, symbol, perContract)
	}
	return n, p, nil
}

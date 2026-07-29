package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ChainContract is one live option listed on Delta.
//
// Only listed contracts exist. A strategy that wants a strike Delta does not
// list cannot trade it at any price — which is precisely the constraint the
// synthetic Black-Scholes chain hid, because that model can price any strike at
// any expiry on demand.
type ChainContract struct {
	ProductID  int
	Symbol     string
	OptionType string // "CALL" | "PUT"
	Strike     float64
	Expiry     time.Time
}

// ListOptionChain returns every live option contract for an underlying
// (e.g. "BTC"), in one request.
//
// This is deliberately a whole-chain fetch rather than a per-strike lookup: the
// hunt runs ~100 option strategies, and if each resolved its own contract that
// would be ~100 requests per cycle. One snapshot serves them all.
func (c *Client) ListOptionChain(ctx context.Context, underlying string) ([]ChainContract, error) {
	// states=live is essential — without it the list includes expired and
	// settled contracts, and ordering one returns HTTP 400 invalid_contract.
	const path = "/v2/products?contract_types=put_options,call_options&states=live&page_size=1000"
	data, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("option chain list failed (HTTP %d)", status)
	}

	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID              int    `json:"id"`
			Symbol          string `json:"symbol"`
			ContractType    string `json:"contract_type"`
			State           string `json:"state"`
			StrikePrice     string `json:"strike_price"`
			SettlementTime  string `json:"settlement_time"`
			UnderlyingAsset struct {
				Symbol string `json:"symbol"`
			} `json:"underlying_asset"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("option chain parse: %w", err)
	}
	if !resp.Success {
		return nil, fmt.Errorf("option chain: success=false")
	}

	want := strings.ToUpper(strings.TrimSpace(underlying))
	out := make([]ChainContract, 0, len(resp.Result))
	for _, p := range resp.Result {
		if want != "" && !strings.EqualFold(p.UnderlyingAsset.Symbol, want) {
			continue
		}
		if !strings.EqualFold(p.State, "live") {
			continue
		}
		strike, err := strconv.ParseFloat(p.StrikePrice, 64)
		if err != nil || strike <= 0 {
			continue
		}
		expiry, err := time.Parse(time.RFC3339, p.SettlementTime)
		if err != nil {
			continue
		}
		optType := "CALL"
		if strings.EqualFold(p.ContractType, "put_options") {
			optType = "PUT"
		}
		out = append(out, ChainContract{
			ProductID:  p.ID,
			Symbol:     p.Symbol,
			OptionType: optType,
			Strike:     strike,
			Expiry:     expiry.UTC(),
		})
	}
	return out, nil
}

// TickerMark is a live mark for one option symbol.
type TickerMark struct {
	Symbol string
	// MarkPerBTC is the quoted premium in USD per BTC of underlying. Multiply by
	// OptionContractSizeBTC for the USD cost of one contract.
	MarkPerBTC float64
	Bid        float64
	Ask        float64
}

// ListOptionTickers fetches marks for every live option in one request.
//
// Same reasoning as ListOptionChain: one batch call per cycle, shared by every
// strategy, instead of one lookup per strategy per tick.
func (c *Client) ListOptionTickers(ctx context.Context, underlying string) (map[string]TickerMark, error) {
	const path = "/v2/tickers?contract_types=put_options,call_options"
	data, status, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("option tickers failed (HTTP %d)", status)
	}

	// NOTE the schema difference from /v2/products, which is easy to get wrong:
	// tickers expose a FLAT `underlying_asset_symbol` string, while products
	// nest it as `underlying_asset.symbol`. Filtering tickers on the nested
	// shape silently matches nothing — 457 BTC contracts become 0 marks and
	// every strike resolution fails with "no contract".
	//
	// Bid/ask live under `quotes`, not at the top level, and every numeric field
	// is quoted as a string ("21241.9"), hence flexNum.
	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			Symbol                string  `json:"symbol"`
			MarkPrice             flexNum `json:"mark_price"`
			UnderlyingAssetSymbol string  `json:"underlying_asset_symbol"`
			Quotes                struct {
				BestBid flexNum `json:"best_bid"`
				BestAsk flexNum `json:"best_ask"`
			} `json:"quotes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("option tickers parse: %w", err)
	}

	want := strings.ToUpper(strings.TrimSpace(underlying))
	out := make(map[string]TickerMark, len(resp.Result))
	for _, tk := range resp.Result {
		if want != "" && tk.UnderlyingAssetSymbol != "" &&
			!strings.EqualFold(tk.UnderlyingAssetSymbol, want) {
			continue
		}
		mark := float64(tk.MarkPrice)
		if mark <= 0 {
			continue
		}
		out[tk.Symbol] = TickerMark{
			Symbol:     tk.Symbol,
			MarkPerBTC: mark,
			Bid:        float64(tk.Quotes.BestBid),
			Ask:        float64(tk.Quotes.BestAsk),
		}
	}
	return out, nil
}

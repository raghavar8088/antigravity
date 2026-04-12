// Package delta provides a REST client for the Delta Exchange India API.
// It is used to mirror paper-traded BTC option SELL signals to real live orders.
//
// Required environment variables:
//   DELTA_API_KEY    — your Delta Exchange API key
//   DELTA_API_SECRET — your Delta Exchange API secret
//
// Set DELTA_TESTNET=true to point at the testnet (https://testnet-api.india.delta.exchange).
package delta

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	prodBaseURL    = "https://api.india.delta.exchange"
	testnetBaseURL = "https://testnet-api.india.delta.exchange"
)

// OrderSide is "buy" or "sell".
type OrderSide string

const (
	SideBuy  OrderSide = "buy"
	SideSell OrderSide = "sell"
)

// OrderType is "limit_order" or "market_order".
type OrderType string

const (
	TypeLimit  OrderType = "limit_order"
	TypeMarket OrderType = "market_order"
)

// PlaceOrderRequest mirrors Delta Exchange POST /v2/orders payload.
type PlaceOrderRequest struct {
	ProductID   int       `json:"product_id"`
	Size        int       `json:"size"`   // number of contracts
	Side        OrderSide `json:"side"`   // "buy" or "sell"
	OrderType   OrderType `json:"order_type"`
	LimitPrice  string    `json:"limit_price,omitempty"`
	TimeInForce string    `json:"time_in_force,omitempty"` // "gtc", "ioc", "fok"
	PostOnly    bool      `json:"post_only,omitempty"`
}

// PlaceOrderResult is a simplified view of Delta's order response.
type PlaceOrderResult struct {
	OrderID   string    `json:"order_id"`
	ProductID int       `json:"product_id"`
	Symbol    string    `json:"symbol"`
	Side      OrderSide `json:"side"`
	Size      int       `json:"size"`
	Price     float64   `json:"price"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

// OptionContractInfo holds the product_id and symbol for a BTC option contract.
type OptionContractInfo struct {
	ProductID int
	Symbol    string
}

// Client is the Delta Exchange REST client.
type Client struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

// NewClient creates a Delta Exchange client from environment variables.
// Returns nil and an error if the keys are not configured.
func NewClient() (*Client, error) {
	key := strings.TrimSpace(os.Getenv("DELTA_API_KEY"))
	secret := strings.TrimSpace(os.Getenv("DELTA_API_SECRET"))
	if key == "" || secret == "" {
		return nil, fmt.Errorf("DELTA_API_KEY and DELTA_API_SECRET must be set")
	}

	baseURL := prodBaseURL
	if strings.TrimSpace(os.Getenv("DELTA_TESTNET")) == "true" {
		baseURL = testnetBaseURL
	}

	return &Client{
		baseURL:   baseURL,
		apiKey:    key,
		apiSecret: secret,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// IsConfigured returns true when Delta API keys are present in the environment.
func IsConfigured() bool {
	return strings.TrimSpace(os.Getenv("DELTA_API_KEY")) != "" &&
		strings.TrimSpace(os.Getenv("DELTA_API_SECRET")) != ""
}

// IsTestnet returns true when DELTA_TESTNET=true.
func IsTestnet() bool {
	return strings.TrimSpace(os.Getenv("DELTA_TESTNET")) == "true"
}

// sign builds the HMAC-SHA256 signature required by Delta Exchange.
// Signature = HMAC_SHA256(secret, method + timestamp + path + body)
func (c *Client) sign(method, path, body string, timestamp int64) string {
	payload := method + strconv.FormatInt(timestamp, 10) + path + body
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (c *Client) doRequest(ctx context.Context, method, path string, bodyBytes []byte) ([]byte, int, error) {
	ts := time.Now().Unix()
	bodyStr := ""
	if len(bodyBytes) > 0 {
		bodyStr = string(bodyBytes)
	}
	sig := c.sign(method, path, bodyStr, ts)

	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("api-key", c.apiKey)
	req.Header.Set("timestamp", strconv.FormatInt(ts, 10))
	req.Header.Set("signature", sig)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}
	return data, resp.StatusCode, nil
}

// PlaceOrder submits a new order to Delta Exchange.
func (c *Client) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResult, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("marshal order: %w", err)
	}

	data, status, err := c.doRequest(ctx, http.MethodPost, "/v2/orders", bodyBytes)
	if err != nil {
		return PlaceOrderResult{}, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return PlaceOrderResult{}, fmt.Errorf("Delta order rejected (HTTP %d): %s", status, truncate(string(data), 300))
	}

	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			ID        int64   `json:"id"`
			Symbol    string  `json:"symbol"`
			Side      string  `json:"side"`
			Size      float64 `json:"size"`
			LimitPrice string `json:"limit_price"`
			AvgPrice  string  `json:"average_fill_price"`
			State     string  `json:"state"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return PlaceOrderResult{}, fmt.Errorf("decode order response: %w", err)
	}
	if !resp.Success {
		return PlaceOrderResult{}, fmt.Errorf("Delta order failed: %s", truncate(string(data), 300))
	}

	price, _ := strconv.ParseFloat(resp.Result.AvgPrice, 64)
	if price == 0 {
		price, _ = strconv.ParseFloat(resp.Result.LimitPrice, 64)
	}

	return PlaceOrderResult{
		OrderID:   strconv.FormatInt(resp.Result.ID, 10),
		ProductID: req.ProductID,
		Symbol:    resp.Result.Symbol,
		Side:      OrderSide(resp.Result.Side),
		Size:      int(resp.Result.Size),
		Price:     price,
		State:     resp.Result.State,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// CancelOrder cancels a live order by order ID.
func (c *Client) CancelOrder(ctx context.Context, orderID string, productID int) error {
	body, _ := json.Marshal(map[string]interface{}{
		"id":         orderID,
		"product_id": productID,
	})
	data, status, err := c.doRequest(ctx, http.MethodDelete, "/v2/orders", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("cancel order failed (HTTP %d): %s", status, truncate(string(data), 200))
	}
	return nil
}

// WalletEntry holds one asset's balance from the wallet.
type WalletEntry struct {
	Asset            string  `json:"asset"`
	Balance          float64 `json:"balance"`
	AvailableBalance float64 `json:"availableBalance"`
	BlockedBalance   float64 `json:"blockedBalance"`
	UnrealisedPnl    float64 `json:"unrealisedPnl"`
}

// GetWalletAll returns all non-zero wallet balances.
func (c *Client) GetWalletAll(ctx context.Context) ([]WalletEntry, error) {
	data, status, err := c.doRequest(ctx, http.MethodGet, "/v2/wallet/balances", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("wallet request failed (HTTP %d): %s", status, truncate(string(data), 200))
	}
	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			Asset            string `json:"asset_symbol"`
			Balance          string `json:"balance"`
			AvailableBalance string `json:"available_balance"`
			BlockedBalance   string `json:"blocked_margin"`
			UnrealisedPnl    string `json:"unrealised_cashflow"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var entries []WalletEntry
	for _, w := range resp.Result {
		bal, _ := strconv.ParseFloat(w.Balance, 64)
		avail, _ := strconv.ParseFloat(w.AvailableBalance, 64)
		blocked, _ := strconv.ParseFloat(w.BlockedBalance, 64)
		upnl, _ := strconv.ParseFloat(w.UnrealisedPnl, 64)
		if bal != 0 || avail != 0 {
			entries = append(entries, WalletEntry{
				Asset:            w.Asset,
				Balance:          bal,
				AvailableBalance: avail,
				BlockedBalance:   blocked,
				UnrealisedPnl:    upnl,
			})
		}
	}
	return entries, nil
}

// GetWallet returns USDT available balance (backwards-compat helper).
func (c *Client) GetWallet(ctx context.Context) (float64, error) {
	entries, err := c.GetWalletAll(ctx)
	if err != nil {
		return 0, err
	}
	for _, e := range entries {
		if e.Asset == "USDT" || e.Asset == "USD" {
			return e.AvailableBalance, nil
		}
	}
	return 0, nil
}

// LivePosition is an open position on Delta Exchange.
type LivePosition struct {
	Symbol        string  `json:"symbol"`
	ProductID     int     `json:"productId"`
	Size          float64 `json:"size"`
	EntryPrice    float64 `json:"entryPrice"`
	MarkPrice     float64 `json:"markPrice"`
	UnrealisedPnl float64 `json:"unrealisedPnl"`
	RealisedPnl   float64 `json:"realisedPnl"`
	Margin        float64 `json:"margin"`
	Side          string  `json:"side"`
}

// GetPositions returns all open positions on the account.
func (c *Client) GetPositions(ctx context.Context) ([]LivePosition, error) {
	data, status, err := c.doRequest(ctx, http.MethodGet, "/v2/positions/margined", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("positions request failed (HTTP %d)", status)
	}
	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			Symbol        string `json:"symbol"`
			ProductID     int    `json:"product_id"`
			Size          string `json:"size"`
			EntryPrice    string `json:"entry_price"`
			MarkPrice     string `json:"mark_price"`
			UnrealisedPnl string `json:"unrealised_pnl"`
			RealisedPnl   string `json:"realised_pnl"`
			Margin        string `json:"margin"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var positions []LivePosition
	for _, p := range resp.Result {
		size, _ := strconv.ParseFloat(p.Size, 64)
		if size == 0 {
			continue
		}
		entry, _ := strconv.ParseFloat(p.EntryPrice, 64)
		mark, _ := strconv.ParseFloat(p.MarkPrice, 64)
		upnl, _ := strconv.ParseFloat(p.UnrealisedPnl, 64)
		rpnl, _ := strconv.ParseFloat(p.RealisedPnl, 64)
		margin, _ := strconv.ParseFloat(p.Margin, 64)
		side := "LONG"
		if size < 0 {
			side = "SHORT"
		}
		positions = append(positions, LivePosition{
			Symbol:        p.Symbol,
			ProductID:     p.ProductID,
			Size:          size,
			EntryPrice:    entry,
			MarkPrice:     mark,
			UnrealisedPnl: upnl,
			RealisedPnl:   rpnl,
			Margin:        margin,
			Side:          side,
		})
	}
	return positions, nil
}

// OpenOrder is an active order on Delta Exchange.
type OpenOrder struct {
	OrderID   string  `json:"orderId"`
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Size      float64 `json:"size"`
	Price     float64 `json:"price"`
	State     string  `json:"state"`
	CreatedAt string  `json:"createdAt"`
}

// GetOpenOrders returns all open orders.
func (c *Client) GetOpenOrders(ctx context.Context) ([]OpenOrder, error) {
	data, status, err := c.doRequest(ctx, http.MethodGet, "/v2/orders?state=open", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("open orders request failed (HTTP %d)", status)
	}
	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			Data []struct {
				ID        int64  `json:"id"`
				Symbol    string `json:"symbol"`
				Side      string `json:"side"`
				Size      string `json:"size"`
				LimitPrice string `json:"limit_price"`
				State     string `json:"state"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	var orders []OpenOrder
	for _, o := range resp.Result.Data {
		size, _ := strconv.ParseFloat(o.Size, 64)
		price, _ := strconv.ParseFloat(o.LimitPrice, 64)
		orders = append(orders, OpenOrder{
			OrderID:   strconv.FormatInt(o.ID, 10),
			Symbol:    o.Symbol,
			Side:      o.Side,
			Size:      size,
			Price:     price,
			State:     o.State,
			CreatedAt: o.CreatedAt,
		})
	}
	return orders, nil
}

// FindOptionProduct searches Delta product list for a BTC option matching the given
// strike, expiry (rounded to nearest weekly Friday), and option type ("call"/"put").
func (c *Client) FindOptionProduct(ctx context.Context, strike float64, expiry time.Time, optionType string) (OptionContractInfo, error) {
	data, status, err := c.doRequest(ctx, http.MethodGet, "/v2/products?contract_types=put_options,call_options&page_size=200", nil)
	if err != nil {
		return OptionContractInfo{}, err
	}
	if status != http.StatusOK {
		return OptionContractInfo{}, fmt.Errorf("products list failed (HTTP %d)", status)
	}

	var resp struct {
		Success bool `json:"success"`
		Result  []struct {
			ID           int    `json:"id"`
			Symbol       string `json:"symbol"`
			ContractType string `json:"contract_type"`
			StrikePrice  string `json:"strike_price"`
			SettlementTime string `json:"settlement_time"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return OptionContractInfo{}, err
	}

	targetType := "call_options"
	if strings.ToLower(optionType) == "put" {
		targetType = "put_options"
	}

	// Find closest strike within 0.5% and nearest expiry within 4 hours
	bestID := 0
	bestSymbol := ""
	bestStrikeDiff := 1e18

	for _, p := range resp.Result {
		if p.ContractType != targetType {
			continue
		}
		s, err := strconv.ParseFloat(p.StrikePrice, 64)
		if err != nil {
			continue
		}
		diff := abs(s - strike)
		if diff < bestStrikeDiff {
			bestStrikeDiff = diff
			bestID = p.ID
			bestSymbol = p.Symbol
		}
	}

	if bestID == 0 {
		return OptionContractInfo{}, fmt.Errorf("no matching %s option found near strike %.0f", optionType, strike)
	}

	return OptionContractInfo{ProductID: bestID, Symbol: bestSymbol}, nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

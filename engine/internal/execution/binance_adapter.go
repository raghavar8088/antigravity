package execution

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"antigravity-engine/internal/exchange"
)

// BinanceAdapter wraps the existing BinanceLiveClient to satisfy ExchangeAdapter.
// It owns exchange API communication ONLY. No order or position state is stored here.
//
// State ownership: OMS v3 / ledger.
// This adapter's responsibility: API calls → raw exchange responses.
type BinanceAdapter struct {
	auth    *exchange.Authenticator
	http    *http.Client
	baseURL string
}

// NewBinanceAdapter creates a BinanceAdapter from API credentials.
func NewBinanceAdapter(apiKey, apiSecret string) *BinanceAdapter {
	return &BinanceAdapter{
		auth:    exchange.NewAuthenticator(apiKey, apiSecret),
		http:    &http.Client{Timeout: 5 * time.Second},
		baseURL: "https://api.binance.com",
	}
}

func (a *BinanceAdapter) Name() string { return "binance" }

// PlaceOrder submits a market or limit order to Binance REST API.
func (a *BinanceAdapter) PlaceOrder(ctx context.Context, req OrderRequest) (OrderAck, error) {
	log.Printf("[BINANCE] PlaceOrder | clientOrderID=%s symbol=%s side=%s qty=%.6f type=%s",
		req.ClientOrderID, req.Symbol, req.Side, req.Quantity, req.OrderType)

	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", req.Side)
	params.Set("type", binanceOrderType(req.OrderType))
	params.Set("quantity", fmt.Sprintf("%.5f", req.Quantity))
	params.Set("newClientOrderId", req.ClientOrderID)
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	if req.OrderType == "LIMIT" || req.OrderType == "POST_ONLY" {
		params.Set("price", fmt.Sprintf("%.2f", req.LimitPrice))
		params.Set("timeInForce", binanceTIF(req.TimeInForce))
	}

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return OrderAck{Status: "REJECTED", RejectReason: err.Error()}, fmt.Errorf("binance: sign request: %w", err)
	}

	fullURL := fmt.Sprintf("%s/api/v3/order?%s", a.baseURL, signedQuery)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, nil)
	if err != nil {
		return OrderAck{}, fmt.Errorf("binance: build request: %w", err)
	}
	httpReq.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return OrderAck{}, fmt.Errorf("binance: http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OrderAck{
			ClientOrderID: req.ClientOrderID,
			Status:        "REJECTED",
			RejectReason:  fmt.Sprintf("HTTP %d", resp.StatusCode),
		}, fmt.Errorf("binance: order rejected with HTTP %d", resp.StatusCode)
	}

	log.Printf("[BINANCE] Order accepted | clientOrderID=%s", req.ClientOrderID)
	return OrderAck{
		ClientOrderID:  req.ClientOrderID,
		Status:         "ACCEPTED",
		AcknowledgedAt: time.Now().UTC(),
	}, nil
}

// CancelOrder sends a DELETE /api/v3/order request to Binance.
func (a *BinanceAdapter) CancelOrder(ctx context.Context, exchangeOrderID, symbol string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", exchangeOrderID)
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return fmt.Errorf("binance: sign cancel request: %w", err)
	}

	fullURL := fmt.Sprintf("%s/api/v3/order?%s", a.baseURL, signedQuery)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, nil)
	if err != nil {
		return fmt.Errorf("binance: build cancel request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("binance: http delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binance: cancel order %s returned HTTP %d", exchangeOrderID, resp.StatusCode)
	}
	return nil
}

// GetOrder fetches the status of a single order from Binance.
func (a *BinanceAdapter) GetOrder(ctx context.Context, exchangeOrderID, symbol string) (ExchangeOrderStatus, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", exchangeOrderID)
	params.Set("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return ExchangeOrderStatus{}, fmt.Errorf("binance: sign get-order request: %w", err)
	}

	fullURL := fmt.Sprintf("%s/api/v3/order?%s", a.baseURL, signedQuery)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return ExchangeOrderStatus{}, fmt.Errorf("binance: build get-order request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	resp, err := a.http.Do(req)
	if err != nil {
		return ExchangeOrderStatus{}, fmt.Errorf("binance: http get order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ExchangeOrderStatus{ExchangeOrderID: exchangeOrderID, Status: "NOT_FOUND"}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return ExchangeOrderStatus{}, fmt.Errorf("binance: get order %s returned HTTP %d", exchangeOrderID, resp.StatusCode)
	}

	// Full JSON parsing would go here in production.
	// Returning a minimal placeholder until Binance order parsing is fully wired.
	return ExchangeOrderStatus{
		ExchangeOrderID: exchangeOrderID,
		Symbol:          symbol,
		Status:          "UNKNOWN",
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

// GetPosition queries the net BTC position from the Binance Spot account balance.
func (a *BinanceAdapter) GetPosition(ctx context.Context, symbol string) (ExchangePositionStatus, error) {
	// GET /api/v3/account for spot holdings.
	// Full implementation queries free + locked balances for the base asset.
	log.Printf("[BINANCE] GetPosition %s (returns 0 — live wiring pending)", symbol)
	return ExchangePositionStatus{Symbol: symbol, UpdatedAt: time.Now().UTC()}, nil
}

// GetBalance returns the account equity from the Binance Spot account.
func (a *BinanceAdapter) GetBalance(ctx context.Context) (ExchangeBalanceStatus, error) {
	log.Println("[BINANCE] GetBalance (returns 0 — live wiring pending)")
	return ExchangeBalanceStatus{UpdatedAt: time.Now().UTC()}, nil
}

// ─── Binance helpers ──────────────────────────────────────────────────────────

func binanceOrderType(t string) string {
	switch t {
	case "LIMIT", "POST_ONLY":
		return "LIMIT_MAKER"
	case "IOC":
		return "MARKET"
	default:
		return "MARKET"
	}
}

func binanceTIF(tif string) string {
	switch tif {
	case "IOC":
		return "IOC"
	case "FOK":
		return "FOK"
	default:
		return "GTC"
	}
}

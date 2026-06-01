package reconciliationv2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"antigravity-engine/internal/exchange"
)

// BinanceFuturesReconciliationAdapter implements ReconciliationAdapter for
// Binance USDT-M Perpetual Futures (fapi.binance.com).
// All reads are from REST endpoints — no order mutations are performed here.
type BinanceFuturesReconciliationAdapter struct {
	auth    *exchange.Authenticator
	http    *http.Client
	baseURL string
}

// NewBinanceFuturesReconciliationAdapter creates the adapter from API credentials.
func NewBinanceFuturesReconciliationAdapter(apiKey, apiSecret string) *BinanceFuturesReconciliationAdapter {
	return &BinanceFuturesReconciliationAdapter{
		auth:    exchange.NewAuthenticator(apiKey, apiSecret),
		http:    &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://fapi.binance.com",
	}
}

func (a *BinanceFuturesReconciliationAdapter) Name() string { return "binance" }

// HealthCheck pings the Binance FAPI server time endpoint.
func (a *BinanceFuturesReconciliationAdapter) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/fapi/v1/ping", nil)
	if err != nil {
		return fmt.Errorf("binance recon: build ping: %w", err)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("binance recon: ping: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("binance recon: ping HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetAccountState returns the complete Binance futures account snapshot.
func (a *BinanceFuturesReconciliationAdapter) GetAccountState(ctx context.Context) (AccountState, error) {
	balances, err := a.GetBalances(ctx)
	if err != nil {
		return AccountState{}, err
	}
	positions, err := a.GetPositions(ctx)
	if err != nil {
		return AccountState{}, err
	}
	openOrders, err := a.GetOpenOrders(ctx, "")
	if err != nil {
		return AccountState{}, err
	}
	return AccountState{
		Exchange:    a.Name(),
		Balances:    balances,
		Positions:   positions,
		OpenOrders:  openOrders,
		RetrievedAt: time.Now().UTC(),
	}, nil
}

// GetBalances fetches USDT futures account balances from /fapi/v2/account.
func (a *BinanceFuturesReconciliationAdapter) GetBalances(ctx context.Context) ([]AssetBalance, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return nil, fmt.Errorf("binance recon: sign balances: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/fapi/v2/account?"+signedQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("binance recon: build balance request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	body, err := a.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("binance recon: get account: %w", err)
	}

	var resp struct {
		Assets []struct {
			Asset              string  `json:"asset"`
			WalletBalance      float64 `json:"walletBalance,string"`
			UnrealizedProfit   float64 `json:"unrealizedProfit,string"`
			MarginBalance      float64 `json:"marginBalance,string"`
			AvailableBalance   float64 `json:"availableBalance,string"`
			InitialMargin      float64 `json:"initialMargin,string"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("binance recon: unmarshal balances: %w", err)
	}

	result := make([]AssetBalance, 0, len(resp.Assets))
	for _, a := range resp.Assets {
		if a.WalletBalance == 0 && a.UnrealizedProfit == 0 {
			continue // skip zero-balance assets
		}
		result = append(result, AssetBalance{
			Asset:         a.Asset,
			WalletBalance: a.WalletBalance,
			Available:     a.AvailableBalance,
			MarginUsed:    a.InitialMargin,
			UnrealizedPnL: a.UnrealizedProfit,
			EquityUSD:     a.MarginBalance,
		})
	}
	return result, nil
}

// GetPositions fetches all open futures positions from /fapi/v2/positionRisk.
func (a *BinanceFuturesReconciliationAdapter) GetPositions(ctx context.Context) ([]ExchangePosition, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return nil, fmt.Errorf("binance recon: sign positions: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/fapi/v2/positionRisk?"+signedQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("binance recon: build positions request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	body, err := a.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("binance recon: get positions: %w", err)
	}

	var raw []struct {
		Symbol           string  `json:"symbol"`
		PositionAmt      float64 `json:"positionAmt,string"`
		EntryPrice       float64 `json:"entryPrice,string"`
		MarkPrice        float64 `json:"markPrice,string"`
		UnrealizedProfit float64 `json:"unrealizedProfit,string"`
		LiquidationPrice float64 `json:"liquidationPrice,string"`
		Leverage         float64 `json:"leverage,string"`
		Notional         float64 `json:"notional,string"`
		IsolatedMargin   float64 `json:"isolatedMargin,string"`
		PositionSide     string  `json:"positionSide"`
		UpdateTime       int64   `json:"updateTime"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance recon: unmarshal positions: %w", err)
	}

	var result []ExchangePosition
	for _, p := range raw {
		if p.PositionAmt == 0 {
			continue
		}
		side := "LONG"
		qty := p.PositionAmt
		if p.PositionAmt < 0 {
			side = "SHORT"
			qty = -qty
		}
		if p.PositionSide == "SHORT" {
			side = "SHORT"
		}
		result = append(result, ExchangePosition{
			Symbol:           p.Symbol,
			Side:             side,
			Quantity:         qty,
			EntryPrice:       p.EntryPrice,
			MarkPrice:        p.MarkPrice,
			LiquidationPrice: p.LiquidationPrice,
			Leverage:         p.Leverage,
			NotionalUSD:      absF(p.Notional),
			MarginUsed:       p.IsolatedMargin,
			UnrealizedPnL:    p.UnrealizedProfit,
			UpdatedAt:        time.UnixMilli(p.UpdateTime).UTC(),
		})
	}
	return result, nil
}

// GetOrders fetches historical orders from /fapi/v1/allOrders.
func (a *BinanceFuturesReconciliationAdapter) GetOrders(ctx context.Context, symbol string, since time.Time) ([]ExchangeOrder, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	if !since.IsZero() {
		params.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	}
	params.Set("limit", "1000")

	return a.fetchOrders(ctx, "/fapi/v1/allOrders", params)
}

// GetOpenOrders fetches all open orders from /fapi/v1/openOrders.
func (a *BinanceFuturesReconciliationAdapter) GetOpenOrders(ctx context.Context, symbol string) ([]ExchangeOrder, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	return a.fetchOrders(ctx, "/fapi/v1/openOrders", params)
}

func (a *BinanceFuturesReconciliationAdapter) fetchOrders(ctx context.Context, path string, params url.Values) ([]ExchangeOrder, error) {
	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return nil, fmt.Errorf("binance recon: sign orders: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path+"?"+signedQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("binance recon: build orders request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	body, err := a.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("binance recon: get orders: %w", err)
	}

	var raw []struct {
		OrderID          int64   `json:"orderId"`
		ClientOrderID    string  `json:"clientOrderId"`
		Symbol           string  `json:"symbol"`
		Side             string  `json:"side"`
		Status           string  `json:"status"`
		Type             string  `json:"type"`
		OrigQty          float64 `json:"origQty,string"`
		ExecutedQty      float64 `json:"executedQty,string"`
		Price            float64 `json:"price,string"`
		AvgPrice         float64 `json:"avgPrice,string"`
		Commission       float64 `json:"commission,string"`
		Time             int64   `json:"time"`
		UpdateTime       int64   `json:"updateTime"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance recon: unmarshal orders: %w", err)
	}

	result := make([]ExchangeOrder, 0, len(raw))
	for _, o := range raw {
		remaining := o.OrigQty - o.ExecutedQty
		if remaining < 0 {
			remaining = 0
		}
		result = append(result, ExchangeOrder{
			ExchangeOrderID:  strconv.FormatInt(o.OrderID, 10),
			ClientOrderID:    o.ClientOrderID,
			Symbol:           o.Symbol,
			Side:             o.Side,
			Status:           o.Status,
			OrderType:        o.Type,
			Quantity:         o.OrigQty,
			FilledQuantity:   o.ExecutedQty,
			RemainingQty:     remaining,
			Price:            o.Price,
			AverageFillPrice: o.AvgPrice,
			FeesUSD:          o.Commission,
			CreatedAt:        time.UnixMilli(o.Time).UTC(),
			UpdatedAt:        time.UnixMilli(o.UpdateTime).UTC(),
		})
	}
	return result, nil
}

// GetFills fetches account fills (user trades) from /fapi/v1/userTrades.
func (a *BinanceFuturesReconciliationAdapter) GetFills(ctx context.Context, symbol string, since time.Time) ([]ExchangeFill, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	if !since.IsZero() {
		params.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	}
	params.Set("limit", "1000")

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return nil, fmt.Errorf("binance recon: sign fills: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/fapi/v1/userTrades?"+signedQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("binance recon: build fills request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	body, err := a.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("binance recon: get fills: %w", err)
	}

	var raw []struct {
		ID              int64   `json:"id"`
		OrderID         int64   `json:"orderId"`
		Symbol          string  `json:"symbol"`
		Side            string  `json:"side"`
		Price           float64 `json:"price,string"`
		Qty             float64 `json:"qty,string"`
		Commission      float64 `json:"commission,string"`
		CommissionAsset string  `json:"commissionAsset"`
		Buyer           bool    `json:"buyer"`
		Maker           bool    `json:"maker"`
		Time            int64   `json:"time"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance recon: unmarshal fills: %w", err)
	}

	result := make([]ExchangeFill, 0, len(raw))
	for _, f := range raw {
		result = append(result, ExchangeFill{
			FillID:          strconv.FormatInt(f.ID, 10),
			ExchangeOrderID: strconv.FormatInt(f.OrderID, 10),
			Symbol:          f.Symbol,
			Side:            f.Side,
			Price:           f.Price,
			Quantity:        f.Qty,
			FeeUSD:          f.Commission,
			FeeAsset:        f.CommissionAsset,
			IsMaker:         f.Maker,
			Timestamp:       time.UnixMilli(f.Time).UTC(),
		})
	}
	return result, nil
}

// GetTrades returns empty for Binance — use GetFills for account-specific trades.
func (a *BinanceFuturesReconciliationAdapter) GetTrades(ctx context.Context, symbol string, since time.Time) ([]ExchangeTrade, error) {
	return nil, nil
}

// GetFunding fetches funding rate payments from /fapi/v1/income.
func (a *BinanceFuturesReconciliationAdapter) GetFunding(ctx context.Context, symbol string, since time.Time) ([]FundingPayment, error) {
	params := url.Values{}
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Set("incomeType", "FUNDING_FEE")
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	if !since.IsZero() {
		params.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	}
	params.Set("limit", "1000")

	signedQuery, err := a.auth.SignBinanceRequest(params)
	if err != nil {
		return nil, fmt.Errorf("binance recon: sign funding: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		a.baseURL+"/fapi/v1/income?"+signedQuery, nil)
	if err != nil {
		return nil, fmt.Errorf("binance recon: build funding request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.auth.APIKey)

	body, err := a.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("binance recon: get funding: %w", err)
	}

	var raw []struct {
		Symbol     string  `json:"symbol"`
		IncomeType string  `json:"incomeType"`
		Income     float64 `json:"income,string"`
		Time       int64   `json:"time"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance recon: unmarshal funding: %w", err)
	}

	result := make([]FundingPayment, 0, len(raw))
	for _, f := range raw {
		result = append(result, FundingPayment{
			Symbol:    f.Symbol,
			AmountUSD: f.Income,
			PaidAt:    time.UnixMilli(f.Time).UTC(),
		})
	}
	return result, nil
}

// ─── Binance helpers ──────────────────────────────────────────────────────────

func (a *BinanceFuturesReconciliationAdapter) doRequest(req *http.Request) ([]byte, error) {
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2MB limit
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

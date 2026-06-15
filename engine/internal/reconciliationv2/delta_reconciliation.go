package reconciliationv2

import (
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

	"antigravity-engine/internal/delta"
)

// DeltaReconciliationAdapter implements ReconciliationAdapter for Delta Exchange.
// Uses HMAC-SHA256 authentication on all private endpoints.
// Docs: https://docs.delta.exchange/
type DeltaReconciliationAdapter struct {
	apiKey    string
	apiSecret string
	http      *http.Client
	baseURL   string
}

// NewDeltaReconciliationAdapter creates the adapter from Delta Exchange API credentials.
func NewDeltaReconciliationAdapter(apiKey, apiSecret string) *DeltaReconciliationAdapter {
	return &DeltaReconciliationAdapter{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http:      &http.Client{Timeout: 10 * time.Second},
		baseURL:   deltaReconciliationBaseURL(),
	}
}

// NewDeltaReconciliationAdapterFromEnv builds the adapter when DELTA_API_KEY/SECRET are set.
func NewDeltaReconciliationAdapterFromEnv() (*DeltaReconciliationAdapter, error) {
	key := strings.TrimSpace(os.Getenv("DELTA_API_KEY"))
	secret := strings.TrimSpace(os.Getenv("DELTA_API_SECRET"))
	if key == "" || secret == "" {
		return nil, fmt.Errorf("DELTA_API_KEY and DELTA_API_SECRET must be set")
	}
	return NewDeltaReconciliationAdapter(key, secret), nil
}

func deltaReconciliationBaseURL() string {
	if delta.IsTestnet() {
		return "https://cdn-ind.testnet.deltaex.org"
	}
	return "https://cdn.india.deltaex.org"
}

func (a *DeltaReconciliationAdapter) Name() string { return "delta" }

// HealthCheck pings the Delta Exchange public status endpoint.
func (a *DeltaReconciliationAdapter) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/v2/products?limit=1", nil)
	if err != nil {
		return fmt.Errorf("delta recon: build health check: %w", err)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("delta recon: health check: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delta recon: health check HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetAccountState returns the complete Delta Exchange account snapshot.
func (a *DeltaReconciliationAdapter) GetAccountState(ctx context.Context) (AccountState, error) {
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

// GetBalances fetches wallet balances from /v2/wallet/balances.
func (a *DeltaReconciliationAdapter) GetBalances(ctx context.Context) ([]AssetBalance, error) {
	body, err := a.get(ctx, "/v2/wallet/balances", "")
	if err != nil {
		return nil, fmt.Errorf("delta recon: get balances: %w", err)
	}

	var resp struct {
		Result []struct {
			Asset              string  `json:"asset_symbol"`
			Balance            float64 `json:"balance"`
			OrderMargin        float64 `json:"order_margin"`
			PositionMargin     float64 `json:"position_margin"`
			UnrealizedFunding  float64 `json:"unrealized_funding_pnl"`
			UnrealizedPnL      float64 `json:"unrealized_pnl"`
			AvailableBalance   float64 `json:"available_balance"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("delta recon: unmarshal balances: %w", err)
	}

	result := make([]AssetBalance, 0, len(resp.Result))
	for _, b := range resp.Result {
		if b.Balance == 0 {
			continue
		}
		margin := b.OrderMargin + b.PositionMargin
		equity := b.Balance + b.UnrealizedPnL + b.UnrealizedFunding
		result = append(result, AssetBalance{
			Asset:         b.Asset,
			WalletBalance: b.Balance,
			Available:     b.AvailableBalance,
			MarginUsed:    margin,
			UnrealizedPnL: b.UnrealizedPnL + b.UnrealizedFunding,
			EquityUSD:     equity,
		})
	}
	return result, nil
}

// GetPositions fetches open positions from /v2/positions.
func (a *DeltaReconciliationAdapter) GetPositions(ctx context.Context) ([]ExchangePosition, error) {
	body, err := a.get(ctx, "/v2/positions", "")
	if err != nil {
		return nil, fmt.Errorf("delta recon: get positions: %w", err)
	}

	var resp struct {
		Result []struct {
			ProductID        int     `json:"product_id"`
			ProductSymbol    string  `json:"product_symbol"`
			Side             string  `json:"side"`
			Size             float64 `json:"size"`
			EntryPrice       float64 `json:"entry_price"`
			MarkPrice        float64 `json:"mark_price"`
			LiquidationPrice float64 `json:"liquidation_price"`
			Leverage         float64 `json:"leverage"`
			Margin           float64 `json:"margin"`
			UnrealizedPnL    float64 `json:"unrealized_pnl"`
			NotionalValue    float64 `json:"notional_value"`
			UpdatedAt        string  `json:"updated_at"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("delta recon: unmarshal positions: %w", err)
	}

	var result []ExchangePosition
	for _, p := range resp.Result {
		if p.Size == 0 {
			continue
		}
		side := strings.ToUpper(p.Side)
		updatedAt, _ := time.Parse(time.RFC3339, p.UpdatedAt)
		result = append(result, ExchangePosition{
			Symbol:           p.ProductSymbol,
			Side:             side,
			Quantity:         p.Size,
			EntryPrice:       p.EntryPrice,
			MarkPrice:        p.MarkPrice,
			LiquidationPrice: p.LiquidationPrice,
			Leverage:         p.Leverage,
			NotionalUSD:      p.NotionalValue,
			MarginUsed:       p.Margin,
			UnrealizedPnL:    p.UnrealizedPnL,
			UpdatedAt:        updatedAt,
		})
	}
	return result, nil
}

// GetOrders fetches historical orders from /v2/orders.
func (a *DeltaReconciliationAdapter) GetOrders(ctx context.Context, symbol string, since time.Time) ([]ExchangeOrder, error) {
	path := "/v2/orders"
	query := ""
	if symbol != "" {
		query = "?product_symbol=" + symbol
	}
	return a.fetchDeltaOrders(ctx, path+query)
}

// GetOpenOrders fetches open orders from /v2/orders?state=open.
func (a *DeltaReconciliationAdapter) GetOpenOrders(ctx context.Context, symbol string) ([]ExchangeOrder, error) {
	path := "/v2/orders?state=open"
	if symbol != "" {
		path += "&product_symbol=" + symbol
	}
	return a.fetchDeltaOrders(ctx, path)
}

func (a *DeltaReconciliationAdapter) fetchDeltaOrders(ctx context.Context, path string) ([]ExchangeOrder, error) {
	body, err := a.get(ctx, path, "")
	if err != nil {
		return nil, fmt.Errorf("delta recon: get orders: %w", err)
	}

	var resp struct {
		Result []struct {
			ID              int64   `json:"id"`
			ClientOrderID   string  `json:"client_order_id"`
			ProductSymbol   string  `json:"product_symbol"`
			Side            string  `json:"side"`
			State           string  `json:"state"`
			OrderType       string  `json:"order_type"`
			Size            float64 `json:"size"`
			FilledSize      float64 `json:"unfilled_size"`
			LimitPrice      float64 `json:"limit_price"`
			AverageFilledPrice float64 `json:"average_fill_price"`
			PaidCommission  float64 `json:"paid_commission"`
			CreatedAt       string  `json:"created_at"`
			UpdatedAt       string  `json:"updated_at"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("delta recon: unmarshal orders: %w", err)
	}

	result := make([]ExchangeOrder, 0, len(resp.Result))
	for _, o := range resp.Result {
		filledQty := o.Size - o.FilledSize
		if filledQty < 0 {
			filledQty = 0
		}
		createdAt, _ := time.Parse(time.RFC3339, o.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, o.UpdatedAt)
		result = append(result, ExchangeOrder{
			ExchangeOrderID:  strconv.FormatInt(o.ID, 10),
			ClientOrderID:    o.ClientOrderID,
			Symbol:           o.ProductSymbol,
			Side:             strings.ToUpper(o.Side),
			Status:           strings.ToUpper(o.State),
			OrderType:        o.OrderType,
			Quantity:         o.Size,
			FilledQuantity:   filledQty,
			RemainingQty:     o.FilledSize,
			Price:            o.LimitPrice,
			AverageFillPrice: o.AverageFilledPrice,
			FeesUSD:          o.PaidCommission,
			CreatedAt:        createdAt,
			UpdatedAt:        updatedAt,
		})
	}
	return result, nil
}

// GetFills fetches account fills from /v2/fills.
func (a *DeltaReconciliationAdapter) GetFills(ctx context.Context, symbol string, since time.Time) ([]ExchangeFill, error) {
	path := "/v2/fills"
	if symbol != "" {
		path += "?product_symbol=" + symbol
	}
	body, err := a.get(ctx, path, "")
	if err != nil {
		return nil, fmt.Errorf("delta recon: get fills: %w", err)
	}

	var resp struct {
		Result []struct {
			ID            int64   `json:"id"`
			OrderID       int64   `json:"order_id"`
			Symbol        string  `json:"product_symbol"`
			Side          string  `json:"side"`
			Price         float64 `json:"fill_price"`
			Quantity      float64 `json:"size"`
			Commission    float64 `json:"commission"`
			RoleType      string  `json:"role"`
			CreatedAt     string  `json:"created_at"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("delta recon: unmarshal fills: %w", err)
	}

	result := make([]ExchangeFill, 0, len(resp.Result))
	for _, f := range resp.Result {
		ts, _ := time.Parse(time.RFC3339, f.CreatedAt)
		if !since.IsZero() && ts.Before(since) {
			continue
		}
		result = append(result, ExchangeFill{
			FillID:          strconv.FormatInt(f.ID, 10),
			ExchangeOrderID: strconv.FormatInt(f.OrderID, 10),
			Symbol:          f.Symbol,
			Side:            strings.ToUpper(f.Side),
			Price:           f.Price,
			Quantity:        f.Quantity,
			FeeUSD:          f.Commission,
			IsMaker:         f.RoleType == "maker",
			Timestamp:       ts,
		})
	}
	return result, nil
}

// GetTrades returns empty — use GetFills for account-specific executions.
func (a *DeltaReconciliationAdapter) GetTrades(ctx context.Context, symbol string, since time.Time) ([]ExchangeTrade, error) {
	return nil, nil
}

// GetFunding fetches funding payments from /v2/fills?type=funding.
func (a *DeltaReconciliationAdapter) GetFunding(ctx context.Context, symbol string, since time.Time) ([]FundingPayment, error) {
	path := "/v2/fills?fill_type=funding"
	if symbol != "" {
		path += "&product_symbol=" + symbol
	}
	body, err := a.get(ctx, path, "")
	if err != nil {
		return nil, fmt.Errorf("delta recon: get funding: %w", err)
	}

	var resp struct {
		Result []struct {
			Symbol    string  `json:"product_symbol"`
			Price     float64 `json:"fill_price"`
			Quantity  float64 `json:"size"`
			CreatedAt string  `json:"created_at"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("delta recon: unmarshal funding: %w", err)
	}

	result := make([]FundingPayment, 0, len(resp.Result))
	for _, f := range resp.Result {
		ts, _ := time.Parse(time.RFC3339, f.CreatedAt)
		if !since.IsZero() && ts.Before(since) {
			continue
		}
		result = append(result, FundingPayment{
			Symbol:    f.Symbol,
			AmountUSD: f.Price * f.Quantity,
			PaidAt:    ts,
		})
	}
	return result, nil
}

// ─── Delta helpers ─────────────────────────────────────────────────────────────

func (a *DeltaReconciliationAdapter) get(ctx context.Context, path, body string) ([]byte, error) {
	// Delta auth expects Unix seconds (see internal/delta/client.go), not milliseconds.
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	method := "GET"

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", a.apiKey)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("signature", a.sign(method, path, body, timestamp))

	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(b), 200))
	}
	return b, nil
}

// sign computes HMAC-SHA256 of method + timestamp + path + body per Delta spec.
func (a *DeltaReconciliationAdapter) sign(method, path, body, timestamp string) string {
	msg := method + timestamp + path + body
	mac := hmac.New(sha256.New, []byte(a.apiSecret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

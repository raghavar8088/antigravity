package execution

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"antigravity-engine/internal/exchange"
	"antigravity-engine/internal/strategy"
)

// BinanceLiveClient bridges the generic `Engine` interface specifically to Binance REST APIs.
// This is the absolute core of REAL execution.
type BinanceLiveClient struct {
	auth *exchange.Authenticator
	http *http.Client
	
	baseURL string
}

func NewBinanceLiveClient(key, secret string) *BinanceLiveClient {
	return &BinanceLiveClient{
		auth:    exchange.NewAuthenticator(key, secret),
		http:    &http.Client{Timeout: 5 * time.Second},
		baseURL: "https://api.binance.com", // Base URL for Spot accounts
	}
}

// PlaceMarketOrder physically routes a verified internal algorithmic request to the public internet.
func (b *BinanceLiveClient) PlaceMarketOrder(sig strategy.Signal) error {
	log.Printf("[LIVE EXEC] Attempting physical %s on Binance for %.4f %s", sig.Action, sig.TargetSize, sig.Symbol)

	endpoint := "/api/v3/order"
	
	// Build Binance standard URL query requirements
	params := url.Values{}
	params.Add("symbol", sig.Symbol)
	params.Add("side", string(sig.Action)) // BUY or SELL
	params.Add("type", "MARKET")
	params.Add("quantity", fmt.Sprintf("%.5f", sig.TargetSize))
	params.Add("timestamp", fmt.Sprintf("%d", time.Now().UnixMilli()))

	// Cryptographically secure the payload against our Secret Key
	signedQuery, err := b.auth.SignBinanceRequest(params)
	if err != nil {
		return fmt.Errorf("failed to sign order request: %w", err)
	}

	fullURL := fmt.Sprintf("%s%s?%s", b.baseURL, endpoint, signedQuery)

	req, err := http.NewRequest("POST", fullURL, nil)
	if err != nil {
		return err
	}
	
	// Inject standard Access Keys
	req.Header.Add("X-MBX-APIKEY", b.auth.APIKey)

	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("NETWORK ERROR: Execution layer failed to reach Binance: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("EXCHANGE ERROR: Binance rejected order with HTTP %d", resp.StatusCode)
	}

	log.Printf("[LIVE EXEC] SUCCESS! Legally executed %.4f %s to the global market.", sig.TargetSize, sig.Symbol)
	return nil
}

// GetPosition queries physical exchange holdings (e.g. Free wallet balance + Locked open sell orders)
func (b *BinanceLiveClient) GetPosition(symbol string) float64 {
	log.Println("[LIVE EXEC] Note: Invoked GetPosition REST Ping. Returning mocked live amount 0.0")
	// In production, this fires GET /api/v3/account to check "free" vs "locked" balances
	return 0.0
}

func (b *BinanceLiveClient) GetBalanceUSD() float64 {
	// Normally fetches USDT free balances exclusively
	return 0.0
}

func (b *BinanceLiveClient) ResetAccount() error {
	// Live clients do not support an account reset via the mock admin interface.
	// This is intentionally a no-op for real exchange integration.
	return nil
}

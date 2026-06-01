package marketdata

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultAngelOneBaseURL = "https://apiconnect.angelone.in"

type AngelOneQuote struct {
	Exchange      string
	TradingSymbol string
	SymbolToken   string
	Price         float64
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Change        float64
	PercentChange float64
	ExchangeTime  string
	FetchedAt     time.Time
}

type AngelOneClient struct {
	mu               sync.Mutex
	httpClient       *http.Client
	baseURL          string
	apiKey           string
	clientCode       string
	pin              string
	totpSecret       string
	totpCode         string
	jwtOverride      string
	niftyExchange    string
	niftySymbol      string
	niftySymbolToken string
	clientLocalIP    string
	clientPublicIP   string
	macAddress       string
	session          angelOneSession
}

type angelOneSession struct {
	jwtToken  string
	expiresAt time.Time
}

type angelOneLoginResponse struct {
	Status    bool `json:"status"`
	Message   string
	ErrorCode string `json:"errorcode"`
	Data      struct {
		JWTToken string `json:"jwtToken"`
	} `json:"data"`
}

type angelOneLTPResponse struct {
	Status    bool   `json:"status"`
	Message   string `json:"message"`
	ErrorCode string `json:"errorcode"`
	Data      struct {
		Exchange      string      `json:"exchange"`
		TradingSymbol string      `json:"tradingsymbol"`
		SymbolToken   string      `json:"symboltoken"`
		Open          interface{} `json:"open"`
		High          interface{} `json:"high"`
		Low           interface{} `json:"low"`
		Close         interface{} `json:"close"`
		LTP           interface{} `json:"ltp"`
	} `json:"data"`
}

func NewAngelOneClient() *AngelOneClient {
	exchange := strings.TrimSpace(os.Getenv("ANGELONE_NIFTY_EXCHANGE"))
	if exchange == "" {
		exchange = "NSE"
	}

	symbol := strings.TrimSpace(os.Getenv("ANGELONE_NIFTY_TRADING_SYMBOL"))
	if symbol == "" {
		symbol = "NIFTY 50"
	}

	symbolToken := strings.TrimSpace(os.Getenv("ANGELONE_NIFTY_SYMBOL_TOKEN"))
	if symbolToken == "" {
		symbolToken = "99926000"
	}

	baseURL := strings.TrimSpace(os.Getenv("ANGELONE_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultAngelOneBaseURL
	}

	clientLocalIP := strings.TrimSpace(os.Getenv("ANGELONE_CLIENT_LOCAL_IP"))
	if clientLocalIP == "" {
		clientLocalIP = "127.0.0.1"
	}
	clientPublicIP := strings.TrimSpace(os.Getenv("ANGELONE_CLIENT_PUBLIC_IP"))
	if clientPublicIP == "" {
		clientPublicIP = "127.0.0.1"
	}
	macAddress := strings.TrimSpace(os.Getenv("ANGELONE_MAC_ADDRESS"))
	if macAddress == "" {
		macAddress = "02:00:00:00:00:00"
	}

	return &AngelOneClient{
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
		baseURL:          strings.TrimRight(baseURL, "/"),
		apiKey:           strings.TrimSpace(os.Getenv("ANGELONE_API_KEY")),
		clientCode:       strings.TrimSpace(os.Getenv("ANGELONE_CLIENT_CODE")),
		pin:              strings.TrimSpace(os.Getenv("ANGELONE_PIN")),
		totpSecret:       strings.TrimSpace(os.Getenv("ANGELONE_TOTP_SECRET")),
		totpCode:         strings.TrimSpace(os.Getenv("ANGELONE_TOTP_CODE")),
		jwtOverride:      strings.TrimSpace(os.Getenv("ANGELONE_JWT_TOKEN")),
		niftyExchange:    exchange,
		niftySymbol:      symbol,
		niftySymbolToken: symbolToken,
		clientLocalIP:    clientLocalIP,
		clientPublicIP:   clientPublicIP,
		macAddress:       macAddress,
	}
}

func (c *AngelOneClient) MissingEnv() []string {
	if c.jwtOverride != "" {
		return nil
	}

	var missing []string
	if c.apiKey == "" {
		missing = append(missing, "ANGELONE_API_KEY")
	}
	if c.clientCode == "" {
		missing = append(missing, "ANGELONE_CLIENT_CODE")
	}
	if c.pin == "" {
		missing = append(missing, "ANGELONE_PIN")
	}
	if c.totpSecret == "" && c.totpCode == "" {
		missing = append(missing, "ANGELONE_TOTP_SECRET or ANGELONE_TOTP_CODE")
	}
	return missing
}

func (c *AngelOneClient) Enabled() bool {
	return len(c.MissingEnv()) == 0
}

func (c *AngelOneClient) FetchNiftyQuote(ctx context.Context) (AngelOneQuote, error) {
	token, err := c.ensureJWTToken(ctx)
	if err != nil {
		return AngelOneQuote{}, err
	}

	reqBody := map[string]string{
		"exchange":      c.niftyExchange,
		"tradingsymbol": c.niftySymbol,
		"symboltoken":   c.niftySymbolToken,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return AngelOneQuote{}, fmt.Errorf("marshal Angel One LTP request: %w", err)
	}

	request := func(jwtToken string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			c.baseURL+"/rest/secure/angelbroking/order/v1/getLtpData",
			strings.NewReader(string(body)),
		)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req, jwtToken)
		return c.httpClient.Do(req)
	}

	resp, err := request(token)
	if err != nil {
		return AngelOneQuote{}, fmt.Errorf("request Angel One LTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.clearSession()
		resp.Body.Close()

		token, err = c.ensureJWTToken(ctx)
		if err != nil {
			return AngelOneQuote{}, err
		}

		resp, err = request(token)
		if err != nil {
			return AngelOneQuote{}, fmt.Errorf("retry Angel One LTP: %w", err)
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return AngelOneQuote{}, fmt.Errorf("Angel One LTP returned status %d", resp.StatusCode)
	}

	var payload angelOneLTPResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return AngelOneQuote{}, fmt.Errorf("decode Angel One LTP payload: %w", err)
	}
	if !payload.Status {
		message := strings.TrimSpace(payload.Message)
		if payload.ErrorCode != "" {
			message = strings.TrimSpace(message + " (" + payload.ErrorCode + ")")
		}
		if message == "" {
			message = "Angel One LTP request was not successful"
		}
		return AngelOneQuote{}, fmt.Errorf("%s", message)
	}

	ltp, err := parseFlexibleFloat(payload.Data.LTP)
	if err != nil {
		return AngelOneQuote{}, fmt.Errorf("parse Angel One LTP: %w", err)
	}
	openPrice, _ := parseFlexibleFloat(payload.Data.Open)
	highPrice, _ := parseFlexibleFloat(payload.Data.High)
	lowPrice, _ := parseFlexibleFloat(payload.Data.Low)
	closePrice, _ := parseFlexibleFloat(payload.Data.Close)

	change := 0.0
	percentChange := 0.0
	if closePrice > 0 {
		change = ltp - closePrice
		percentChange = change / closePrice * 100
	}

	quote := AngelOneQuote{
		Exchange:      strings.TrimSpace(payload.Data.Exchange),
		TradingSymbol: strings.TrimSpace(payload.Data.TradingSymbol),
		SymbolToken:   strings.TrimSpace(payload.Data.SymbolToken),
		Price:         ltp,
		Open:          openPrice,
		High:          highPrice,
		Low:           lowPrice,
		Close:         closePrice,
		Change:        change,
		PercentChange: percentChange,
		FetchedAt:     time.Now().UTC(),
	}
	if quote.Exchange == "" {
		quote.Exchange = c.niftyExchange
	}
	if quote.TradingSymbol == "" {
		quote.TradingSymbol = c.niftySymbol
	}
	if quote.SymbolToken == "" {
		quote.SymbolToken = c.niftySymbolToken
	}
	quote.ExchangeTime = quote.FetchedAt.Format(time.RFC3339)

	return quote, nil
}

func (c *AngelOneClient) ensureJWTToken(ctx context.Context) (string, error) {
	if c.jwtOverride != "" {
		return c.jwtOverride, nil
	}
	if !c.Enabled() {
		return "", fmt.Errorf("Angel One probe is not configured: missing %s", strings.Join(c.MissingEnv(), ", "))
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session.jwtToken != "" && time.Now().UTC().Before(c.session.expiresAt) {
		return c.session.jwtToken, nil
	}

	totp, err := c.resolveTOTP()
	if err != nil {
		return "", err
	}

	reqBody := map[string]string{
		"clientcode": c.clientCode,
		"password":   c.pin,
		"totp":       totp,
		"state":      "LiveDataLab",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal Angel One login request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/rest/auth/angelbroking/user/v1/loginByPassword",
		strings.NewReader(string(body)),
	)
	if err != nil {
		return "", fmt.Errorf("build Angel One login request: %w", err)
	}
	c.setHeaders(req, "")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request Angel One login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Angel One login returned status %d", resp.StatusCode)
	}

	var payload angelOneLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode Angel One login payload: %w", err)
	}
	if !payload.Status || strings.TrimSpace(payload.Data.JWTToken) == "" {
		message := strings.TrimSpace(payload.Message)
		if payload.ErrorCode != "" {
			message = strings.TrimSpace(message + " (" + payload.ErrorCode + ")")
		}
		if message == "" {
			message = "Angel One login failed"
		}
		return "", fmt.Errorf("%s", message)
	}

	c.session = angelOneSession{
		jwtToken:  strings.TrimSpace(payload.Data.JWTToken),
		expiresAt: nextAngelOneSessionExpiry(time.Now().UTC()),
	}
	return c.session.jwtToken, nil
}

func (c *AngelOneClient) clearSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session = angelOneSession{}
}

// ForwardRequest proxies a POST request to an Angel One API path using the cached JWT.
// Returns the raw response body, HTTP status code, and any transport error.
func (c *AngelOneClient) ForwardRequest(ctx context.Context, path string, reqBody []byte) ([]byte, int, error) {
	token, err := c.ensureJWTToken(ctx)
	if err != nil {
		return nil, 0, err
	}

	do := func(jwt string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		c.setHeaders(req, jwt)
		return c.httpClient.Do(req)
	}

	resp, err := do(token)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.clearSession()
		resp.Body.Close()
		token, err = c.ensureJWTToken(ctx)
		if err != nil {
			return nil, resp.StatusCode, err
		}
		resp, err = do(token)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
	}

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (c *AngelOneClient) resolveTOTP() (string, error) {
	if c.totpCode != "" {
		return c.totpCode, nil
	}
	if c.totpSecret == "" {
		return "", fmt.Errorf("Angel One TOTP is missing")
	}
	return generateTOTP(c.totpSecret, time.Now().UTC())
}

func (c *AngelOneClient) setHeaders(req *http.Request, jwtToken string) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-PrivateKey", c.apiKey)
	req.Header.Set("X-UserType", "USER")
	req.Header.Set("X-SourceID", "WEB")
	req.Header.Set("X-ClientLocalIP", c.clientLocalIP)
	req.Header.Set("X-ClientPublicIP", c.clientPublicIP)
	req.Header.Set("X-MACAddress", c.macAddress)
	req.Header.Set("User-Agent", "RAIG-LiveDataLab/1.0")
	if jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+jwtToken)
	}
}

func nextAngelOneSessionExpiry(now time.Time) time.Time {
	ist := time.FixedZone("Asia/Kolkata", 5*3600+1800)
	localNow := now.In(ist)
	expiry := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 4, 55, 0, 0, ist)
	if !expiry.After(localNow) {
		expiry = expiry.Add(24 * time.Hour)
	}
	return expiry.UTC()
}

func generateTOTP(secret string, now time.Time) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	normalized = strings.TrimRight(normalized, "=")
	decoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	key, err := decoder.DecodeString(normalized)
	if err != nil {
		return "", fmt.Errorf("decode Angel One TOTP secret: %w", err)
	}

	counter := uint64(now.Unix() / 30)
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)

	hash := hmac.New(sha1.New, key)
	hash.Write(counterBytes[:])
	sum := hash.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	code := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)

	return fmt.Sprintf("%06d", code%1000000), nil
}

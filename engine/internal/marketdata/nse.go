package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultNSEBaseURL     = "https://www.nseindia.com"
	nifty50IndexName      = "NIFTY 50"
	nseIndicesRefererPath = "/market-data/live-market-indices"
)

type NSEIndexQuote struct {
	Index         string
	Price         float64
	Change        float64
	PercentChange float64
	ExchangeTime  string
	FetchedAt     time.Time
}

type NSEIndexClient struct {
	mu            sync.Mutex
	httpClient    *http.Client
	baseURL       string
	sessionPrimed bool
}

type nseAllIndicesResponse struct {
	Timestamp string                `json:"timestamp"`
	Data      []nseAllIndicesRecord `json:"data"`
}

type nseAllIndicesRecord struct {
	Index         string      `json:"index"`
	IndexSymbol   string      `json:"indexSymbol"`
	Last          interface{} `json:"last"`
	Variation     interface{} `json:"variation"`
	PercentChange interface{} `json:"percentChange"`
}

func NewNSEIndexClient() *NSEIndexClient {
	jar, _ := cookiejar.New(nil)

	return &NSEIndexClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
		baseURL: strings.TrimRight(getNSEBaseURL(), "/"),
	}
}

func getNSEBaseURL() string {
	if baseURL := strings.TrimSpace(os.Getenv("NSE_BASE_URL")); baseURL != "" {
		return baseURL
	}
	return defaultNSEBaseURL
}

func (c *NSEIndexClient) FetchNifty50Quote(ctx context.Context) (NSEIndexQuote, error) {
	if err := c.ensureSession(ctx); err != nil {
		return NSEIndexQuote{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/allIndices", nil)
	if err != nil {
		return NSEIndexQuote{}, fmt.Errorf("build NSE indices request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return NSEIndexQuote{}, fmt.Errorf("request NSE indices feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.resetSession()
		return NSEIndexQuote{}, fmt.Errorf("NSE indices feed returned status %d", resp.StatusCode)
	}

	var payload nseAllIndicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		c.resetSession()
		return NSEIndexQuote{}, fmt.Errorf("decode NSE indices payload: %w", err)
	}

	quote, err := parseNifty50Quote(payload)
	if err != nil {
		return NSEIndexQuote{}, err
	}
	quote.FetchedAt = time.Now().UTC()
	return quote, nil
}

func (c *NSEIndexClient) ensureSession(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionPrimed {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/", nil)
	if err != nil {
		return fmt.Errorf("build NSE session request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("prime NSE session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("NSE session bootstrap returned status %d", resp.StatusCode)
	}

	c.sessionPrimed = true
	return nil
}

func (c *NSEIndexClient) resetSession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionPrimed = false
}

func (c *NSEIndexClient) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", c.baseURL+nseIndicesRefererPath)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
}

func parseNifty50Quote(payload nseAllIndicesResponse) (NSEIndexQuote, error) {
	for _, record := range payload.Data {
		if !isNifty50Record(record) {
			continue
		}

		price, err := parseNSEFloat(record.Last)
		if err != nil {
			return NSEIndexQuote{}, fmt.Errorf("parse NIFTY 50 last price: %w", err)
		}
		change, err := parseNSEFloat(record.Variation)
		if err != nil {
			change = 0
		}
		percentChange, err := parseNSEFloat(record.PercentChange)
		if err != nil {
			percentChange = 0
		}

		return NSEIndexQuote{
			Index:         nifty50IndexName,
			Price:         price,
			Change:        change,
			PercentChange: percentChange,
			ExchangeTime:  strings.TrimSpace(payload.Timestamp),
		}, nil
	}

	return NSEIndexQuote{}, fmt.Errorf("%s not found in NSE indices payload", nifty50IndexName)
}

func isNifty50Record(record nseAllIndicesRecord) bool {
	index := strings.EqualFold(strings.TrimSpace(record.Index), nifty50IndexName)
	symbol := strings.EqualFold(strings.TrimSpace(record.IndexSymbol), nifty50IndexName)
	return index || symbol
}

func parseNSEFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case nil:
		return 0, fmt.Errorf("empty value")
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	case string:
		normalized := strings.ReplaceAll(strings.TrimSpace(typed), ",", "")
		if normalized == "" || normalized == "-" {
			return 0, fmt.Errorf("empty string value")
		}
		parsed, err := strconv.ParseFloat(normalized, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported value type %T", value)
	}
}

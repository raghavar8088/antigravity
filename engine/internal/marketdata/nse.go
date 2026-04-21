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

// ── Yahoo Finance fallback for ^NSEI ─────────────────────────────────────────

type yahooChartMeta struct {
	RegularMarketPrice float64 `json:"regularMarketPrice"`
	ChartPreviousClose float64 `json:"chartPreviousClose"`
	PreviousClose      float64 `json:"previousClose"`
}

type yahooQuoteIndicator struct {
	Close []float64 `json:"close"`
}

type yahooChartResult struct {
	Meta       *yahooChartMeta `json:"meta"`
	Timestamps []int64         `json:"timestamp"`
	Indicators *struct {
		Quote []yahooQuoteIndicator `json:"quote"`
	} `json:"indicators"`
}

type yahooChartResponse struct {
	Chart *struct {
		Result []yahooChartResult `json:"result"`
	} `json:"chart"`
}

// FetchNiftyWarmupBars fetches today's 1-minute NIFTY 50 close prices from
// Yahoo Finance so the NIFTY options engines can classify regime immediately
// on startup instead of waiting 55+ minutes to accumulate bars from live feed.
func FetchNiftyWarmupBars(ctx context.Context) ([]float64, error) {
	mirrors := []string{
		"https://query1.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false",
		"https://query2.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false",
	}
	for _, url := range mirrors {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := yahooHTTPClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var payload yahooChartResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			continue
		}
		if payload.Chart == nil || len(payload.Chart.Result) == 0 {
			continue
		}
		result := payload.Chart.Result[0]
		if result.Indicators == nil || len(result.Indicators.Quote) == 0 {
			continue
		}
		closes := result.Indicators.Quote[0].Close
		// Filter out zero/NaN values (market gaps, pre-open bars)
		valid := make([]float64, 0, len(closes))
		for _, c := range closes {
			if c > 0 {
				valid = append(valid, c)
			}
		}
		if len(valid) >= 5 {
			return valid, nil
		}
	}
	return nil, fmt.Errorf("could not fetch NIFTY warmup bars from Yahoo Finance")
}

var yahooHTTPClient = &http.Client{Timeout: 8 * time.Second}

// fetchNiftyFromYahoo fetches the live NIFTY 50 price from Yahoo Finance
// using the v8/chart endpoint which does not require authentication.
func fetchNiftyFromYahoo(ctx context.Context) (NSEIndexQuote, error) {
	mirrors := []string{
		"https://query1.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false",
		"https://query2.finance.yahoo.com/v8/finance/chart/%5ENSEI?interval=1m&range=1d&includePrePost=false",
	}
	for _, url := range mirrors {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := yahooHTTPClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var payload yahooChartResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			continue
		}
		if payload.Chart == nil || len(payload.Chart.Result) == 0 || payload.Chart.Result[0].Meta == nil {
			continue
		}
		meta := payload.Chart.Result[0].Meta
		if meta.RegularMarketPrice <= 0 {
			continue
		}
		prevClose := meta.ChartPreviousClose
		if prevClose <= 0 {
			prevClose = meta.PreviousClose
		}
		change := meta.RegularMarketPrice - prevClose
		pctChange := 0.0
		if prevClose > 0 {
			pctChange = change / prevClose * 100
		}
		return NSEIndexQuote{
			Index:         "NIFTY 50",
			Price:         meta.RegularMarketPrice,
			Change:        change,
			PercentChange: pctChange,
			ExchangeTime:  time.Now().Format("15:04:05"),
			FetchedAt:     time.Now().UTC(),
		}, nil
	}
	return NSEIndexQuote{}, fmt.Errorf("yahoo finance unavailable for ^NSEI")
}

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
	// Try NSE first; fall back to Yahoo Finance when NSE blocks or errors.
	quote, err := c.fetchFromNSE(ctx)
	if err == nil {
		return quote, nil
	}
	// NSE failed — use Yahoo Finance as fallback.
	yahooQuote, yahooErr := fetchNiftyFromYahoo(ctx)
	if yahooErr != nil {
		return NSEIndexQuote{}, fmt.Errorf("NSE: %w; Yahoo fallback: %v", err, yahooErr)
	}
	return yahooQuote, nil
}

func (c *NSEIndexClient) fetchFromNSE(ctx context.Context) (NSEIndexQuote, error) {
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

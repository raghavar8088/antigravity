package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultDeltaAPIBaseURL = "https://api.india.delta.exchange"

type DeltaTickerQuote struct {
	Symbol       string
	Price        float64
	Open         float64
	High         float64
	Low          float64
	Volume       float64
	Turnover     float64
	MarkPrice    float64
	SpotPrice    float64
	ContractType string
	ExchangeTime string
	FetchedAt    time.Time
}

type DeltaTickerClient struct {
	baseURL    string
	httpClient *http.Client
}

type deltaTickerResponse struct {
	Success bool              `json:"success"`
	Result  deltaTickerResult `json:"result"`
}

type deltaTickerResult struct {
	Symbol       string      `json:"symbol"`
	Close        interface{} `json:"close"`
	Open         interface{} `json:"open"`
	High         interface{} `json:"high"`
	Low          interface{} `json:"low"`
	Volume       interface{} `json:"volume"`
	Turnover     interface{} `json:"turnover"`
	MarkPrice    interface{} `json:"mark_price"`
	SpotPrice    interface{} `json:"spot_price"`
	ContractType string      `json:"contract_type"`
	Timestamp    interface{} `json:"timestamp"`
}

func NewDeltaTickerClient() *DeltaTickerClient {
	baseURL := strings.TrimSpace(os.Getenv("DELTA_API_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultDeltaAPIBaseURL
	}

	return &DeltaTickerClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *DeltaTickerClient) FetchTicker(ctx context.Context, symbol string) (DeltaTickerQuote, error) {
	if strings.TrimSpace(symbol) == "" {
		symbol = "BTCUSD"
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/v2/tickers/%s", c.baseURL, symbol),
		nil,
	)
	if err != nil {
		return DeltaTickerQuote{}, fmt.Errorf("build Delta ticker request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DeltaTickerQuote{}, fmt.Errorf("request Delta ticker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DeltaTickerQuote{}, fmt.Errorf("Delta ticker returned status %d", resp.StatusCode)
	}

	var payload deltaTickerResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return DeltaTickerQuote{}, fmt.Errorf("decode Delta ticker payload: %w", err)
	}
	if !payload.Success {
		return DeltaTickerQuote{}, fmt.Errorf("Delta ticker request was not successful")
	}

	price, err := parseFlexibleFloat(payload.Result.Close)
	if err != nil {
		return DeltaTickerQuote{}, fmt.Errorf("parse Delta last price: %w", err)
	}
	openPrice, _ := parseFlexibleFloat(payload.Result.Open)
	highPrice, _ := parseFlexibleFloat(payload.Result.High)
	lowPrice, _ := parseFlexibleFloat(payload.Result.Low)
	volume, _ := parseFlexibleFloat(payload.Result.Volume)
	turnover, _ := parseFlexibleFloat(payload.Result.Turnover)
	markPrice, _ := parseFlexibleFloat(payload.Result.MarkPrice)
	spotPrice, _ := parseFlexibleFloat(payload.Result.SpotPrice)

	quote := DeltaTickerQuote{
		Symbol:       strings.TrimSpace(payload.Result.Symbol),
		Price:        price,
		Open:         openPrice,
		High:         highPrice,
		Low:          lowPrice,
		Volume:       volume,
		Turnover:     turnover,
		MarkPrice:    markPrice,
		SpotPrice:    spotPrice,
		ContractType: strings.TrimSpace(payload.Result.ContractType),
		FetchedAt:    time.Now().UTC(),
	}

	if timestamp, err := parseFlexibleInt64(payload.Result.Timestamp); err == nil && timestamp > 0 {
		quote.ExchangeTime = time.Unix(timestamp, 0).UTC().Format(time.RFC3339)
	}
	if quote.ExchangeTime == "" {
		quote.ExchangeTime = quote.FetchedAt.Format(time.RFC3339)
	}

	return quote, nil
}

func parseFlexibleFloat(value interface{}) (float64, error) {
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

func parseFlexibleInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case nil:
		return 0, fmt.Errorf("empty value")
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return 0, fmt.Errorf("empty string value")
		}
		return strconv.ParseInt(normalized, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported timestamp type %T", value)
	}
}

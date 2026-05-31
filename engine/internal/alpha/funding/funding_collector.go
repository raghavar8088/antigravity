package funding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Collector struct {
	httpClient *http.Client
	baseURLs   map[string]string
}

func NewCollector() *Collector {
	return &Collector{
		httpClient: &http.Client{Timeout: 4 * time.Second},
		baseURLs: map[string]string{
			"binance":     "https://fapi.binance.com",
			"bybit":       "https://api.bybit.com",
			"hyperliquid": "https://api.hyperliquid.xyz",
		},
	}
}

func (c *Collector) Fetch(ctx context.Context, exchange, symbol string) (FundingSnapshot, error) {
	switch strings.ToLower(exchange) {
	case "binance":
		return c.fetchBinance(ctx, symbol)
	case "bybit":
		return c.fetchBybit(ctx, symbol)
	case "hyperliquid":
		return c.fetchHyperliquid(ctx, symbol)
	default:
		return FundingSnapshot{}, fmt.Errorf("unsupported funding exchange %q", exchange)
	}
}

func (c *Collector) fetchBinance(ctx context.Context, symbol string) (FundingSnapshot, error) {
	url := fmt.Sprintf("%s/fapi/v1/premiumIndex?symbol=%s", c.baseURLs["binance"], symbol)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return FundingSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return FundingSnapshot{}, fmt.Errorf("binance funding status %d", res.StatusCode)
	}
	var row struct {
		Symbol               string `json:"symbol"`
		LastFundingRate      string `json:"lastFundingRate"`
		NextFundingTime      int64  `json:"nextFundingTime"`
		InterestRate         string `json:"interestRate"`
		EstimatedSettlePrice string `json:"estimatedSettlePrice"`
	}
	if err := json.NewDecoder(res.Body).Decode(&row); err != nil {
		return FundingSnapshot{}, err
	}
	rate := parseFloat(row.LastFundingRate)
	return FundingSnapshot{
		Exchange:         "binance",
		Symbol:           row.Symbol,
		FundingRate:      rate,
		PredictedFunding: rate,
		NextFundingTime:  time.UnixMilli(row.NextFundingTime).UTC(),
		Timestamp:        time.Now().UTC(),
	}, nil
}

func (c *Collector) fetchBybit(ctx context.Context, symbol string) (FundingSnapshot, error) {
	url := fmt.Sprintf("%s/v5/market/tickers?category=linear&symbol=%s", c.baseURLs["bybit"], symbol)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := c.httpClient.Do(req)
	if err != nil {
		return FundingSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return FundingSnapshot{}, fmt.Errorf("bybit funding status %d", res.StatusCode)
	}
	var body struct {
		Result struct {
			List []struct {
				Symbol               string `json:"symbol"`
				FundingRate          string `json:"fundingRate"`
				NextFundingTime      string `json:"nextFundingTime"`
				PredictedFundingRate string `json:"predictedFundingRate"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return FundingSnapshot{}, err
	}
	if len(body.Result.List) == 0 {
		return FundingSnapshot{}, fmt.Errorf("bybit funding empty result")
	}
	row := body.Result.List[0]
	nextMs, _ := strconv.ParseInt(row.NextFundingTime, 10, 64)
	pred := parseFloat(row.PredictedFundingRate)
	rate := parseFloat(row.FundingRate)
	if pred == 0 {
		pred = rate
	}
	return FundingSnapshot{
		Exchange:         "bybit",
		Symbol:           row.Symbol,
		FundingRate:      rate,
		PredictedFunding: pred,
		NextFundingTime:  time.UnixMilli(nextMs).UTC(),
		Timestamp:        time.Now().UTC(),
	}, nil
}

func (c *Collector) fetchHyperliquid(ctx context.Context, symbol string) (FundingSnapshot, error) {
	body := strings.NewReader(fmt.Sprintf(`{"type":"metaAndAssetCtxs","coin":"%s"}`, strings.TrimSuffix(symbol, "USDT")))
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURLs["hyperliquid"]+"/info", body)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return FundingSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return FundingSnapshot{}, fmt.Errorf("hyperliquid funding status %d", res.StatusCode)
	}
	var raw []json.RawMessage
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return FundingSnapshot{}, err
	}
	if len(raw) < 2 {
		return FundingSnapshot{}, fmt.Errorf("hyperliquid funding empty result")
	}
	var contexts []map[string]any
	if err := json.Unmarshal(raw[1], &contexts); err != nil {
		return FundingSnapshot{}, err
	}
	coin := strings.TrimSuffix(symbol, "USDT")
	for _, ctxRow := range contexts {
		if fmt.Sprint(ctxRow["coin"]) != coin {
			continue
		}
		rate := parseFloat(fmt.Sprint(ctxRow["funding"]))
		return FundingSnapshot{
			Exchange:         "hyperliquid",
			Symbol:           symbol,
			FundingRate:      rate,
			PredictedFunding: rate,
			NextFundingTime:  time.Now().UTC().Add(time.Hour),
			Timestamp:        time.Now().UTC(),
		}, nil
	}
	return FundingSnapshot{}, fmt.Errorf("hyperliquid funding symbol %s not found", symbol)
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

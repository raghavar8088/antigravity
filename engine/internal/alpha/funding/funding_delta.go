package funding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Delta funding source for the alpha collector.
//
// This collector injects funding snapshots straight into the
// InstitutionalAlphaScalper family (FundingMeanReversion, Confluence). It read
// Binance and Bybit, so those strategies were reasoning about the leverage
// imbalance in books this engine does not trade. Funding is the payment that
// ties ONE perpetual to spot; another venue's is a different number about a
// different crowd.
//
// UNITS. Delta quotes funding as a PERCENT (0.01 = 0.01% per 8h). Binance and
// Bybit quote the same economic rate as a DECIMAL (0.0001), which is what
// FundingSnapshot.FundingRate means everywhere it is consumed here. The
// conversion is therefore mandatory, and getting it wrong is silent: an
// unremarkable funding rate arrives a hundred times too large and every
// funding-sensitive strategy reads a permanent extreme.

// deltaFundingEndpoint is Delta's public ticker (no auth).
const deltaFundingEndpoint = "https://api.india.delta.exchange/v2/tickers/%s"

// DeltaFundingPercentToDecimal converts Delta's percent-quoted funding into the
// decimal this package's consumers expect.
//
// Exported and named rather than written inline as "/100" so the conversion is
// visible at the call site and can be tested on its own.
func DeltaFundingPercentToDecimal(pct float64) float64 { return pct / 100 }

// deltaFundingResponse is Delta's ticker envelope; numerics are quoted strings.
type deltaFundingResponse struct {
	Success bool `json:"success"`
	Result  struct {
		Symbol           string `json:"symbol"`
		FundingRate      string `json:"funding_rate"`
		PredictedFunding string `json:"predicted_funding_rate"`
	} `json:"result"`
}

// fetchDelta reads the funding rate for a Delta perpetual.
func (c *Collector) fetchDelta(ctx context.Context, symbol string) (FundingSnapshot, error) {
	url := fmt.Sprintf(deltaFundingEndpoint, DeltaPerpSymbol(symbol))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return FundingSnapshot{}, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return FundingSnapshot{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return FundingSnapshot{}, fmt.Errorf("delta funding status %d", res.StatusCode)
	}

	var parsed deltaFundingResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return FundingSnapshot{}, err
	}
	if !parsed.Success || parsed.Result.FundingRate == "" {
		// An absent rate must be an error, never a zero snapshot. Zero funding is
		// a meaningful reading — perfectly balanced positioning — so returning it
		// on failure asserts something false about the market.
		return FundingSnapshot{}, fmt.Errorf("delta ticker carried no funding rate")
	}

	pct, err := strconv.ParseFloat(parsed.Result.FundingRate, 64)
	if err != nil {
		return FundingSnapshot{}, fmt.Errorf("parse funding_rate %q: %w", parsed.Result.FundingRate, err)
	}
	snap := FundingSnapshot{
		Exchange:    "delta",
		Symbol:      parsed.Result.Symbol,
		FundingRate: DeltaFundingPercentToDecimal(pct),
		Timestamp:   time.Now().UTC(),
	}
	if parsed.Result.PredictedFunding != "" {
		if p, perr := strconv.ParseFloat(parsed.Result.PredictedFunding, 64); perr == nil {
			snap.PredictedFunding = DeltaFundingPercentToDecimal(p)
		}
	}
	return snap, nil
}

// DeltaPerpSymbol maps another venue's notation onto Delta's perpetual.
//
// Callers are configured with "BTCUSDT" from the Binance era. Delta lists no
// such product, and asking for one returns an error rather than a rate — so
// without this the funding feed would simply stop, quietly, on every existing
// deployment.
func DeltaPerpSymbol(symbol string) string {
	s := symbol
	if s == "" {
		return "BTCUSD"
	}
	if len(s) > 4 && s[len(s)-4:] == "USDT" {
		return s[:len(s)-4] + "USD"
	}
	return s
}

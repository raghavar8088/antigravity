package marketdata

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// WarmupData holds historical candles fetched from Coinbase REST API
// for pre-filling strategy price buffers on engine startup.
type WarmupData struct {
	Candles1m []Candle
	Candles5m []Candle
}

func FetchWarmupCandles(symbol string) (*WarmupData, error) {
	log.Printf("[WARMUP] Fetching real-time 1m and 5m candles from Coinbase REST...")

	// Coinbase API limit is 300 per granular request.
	candles1m, err := fetchKlines(symbol, "60")
	if err != nil {
		return nil, fmt.Errorf("warmup 1m fetch failed: %w", err)
	}

	candles5m, err := fetchKlines(symbol, "300")
	if err != nil {
		return nil, fmt.Errorf("warmup 5m fetch failed: %w", err)
	}

	// Important: Coinbase returns descending time (newest first). Must reverse it!
	reverseCandles(candles1m)
	reverseCandles(candles5m)

	log.Printf("[WARMUP] ✅ Loaded %d x 1m candles and %d x 5m candles", len(candles1m), len(candles5m))
	return &WarmupData{
		Candles1m: candles1m,
		Candles5m: candles5m,
	}, nil
}

// fetchKlines calls the Coinbase REST candles endpoint.
// Response format: [[ time (unix secs), low, high, open, close, volume ], ...]
func fetchKlines(symbol, granularity string) ([]Candle, error) {
	url := fmt.Sprintf(
		"https://api.exchange.coinbase.com/products/%s/candles?granularity=%s",
		symbol, granularity,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "AntigravityEngine/4.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coinbase API returned status %d", resp.StatusCode)
	}

	var raw [][]float64
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	candles := make([]Candle, 0, len(raw))
	for _, kline := range raw {
		if len(kline) < 6 {
			continue
		}

		timeSecs := int64(kline[0])
		low := kline[1]
		high := kline[2]
		open := kline[3]
		closeP := kline[4]
		volume := kline[5]

		openTime := time.Unix(timeSecs, 0)

		// Determine length of candle in seconds based on granularity parameter
		var d time.Duration
		if granularity == "60" {
			d = 1 * time.Minute
		} else {
			d = 5 * time.Minute
		}
		
		closeTime := openTime.Add(d)

		candles = append(candles, Candle{
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeP,
			Volume:    volume,
			OpenTime:  openTime,
			CloseTime: closeTime,
		})
	}

	return candles, nil
}

func reverseCandles(c []Candle) {
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
	}
}

package marketdata

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// WarmupData holds historical candles fetched from Binance REST API
// for pre-filling strategy price buffers on engine startup.
type WarmupData struct {
	Candles1m []Candle
	Candles5m []Candle
}

// FetchWarmupCandles retrieves historical 1m and 5m klines from Binance
// to instantly warm up all strategy buffers instead of waiting for live data.
// This eliminates the warmup delay entirely on cold start / Render restart.
func FetchWarmupCandles(symbol string, count1m int, count5m int) (*WarmupData, error) {
	log.Printf("[WARMUP] Fetching %d x 1m and %d x 5m candles from Binance REST...", count1m, count5m)

	candles1m, err := fetchKlines(symbol, "1m", count1m)
	if err != nil {
		return nil, fmt.Errorf("warmup 1m fetch failed: %w", err)
	}

	candles5m, err := fetchKlines(symbol, "5m", count5m)
	if err != nil {
		return nil, fmt.Errorf("warmup 5m fetch failed: %w", err)
	}

	log.Printf("[WARMUP] ✅ Loaded %d x 1m candles and %d x 5m candles", len(candles1m), len(candles5m))
	return &WarmupData{
		Candles1m: candles1m,
		Candles5m: candles5m,
	}, nil
}

// fetchKlines calls the Binance REST klines endpoint.
// Response format: [[openTime, open, high, low, close, volume, closeTime, ...], ...]
func fetchKlines(symbol, interval string, limit int) ([]Candle, error) {
	url := fmt.Sprintf(
		"https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("binance API returned status %d", resp.StatusCode)
	}

	var raw [][]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	candles := make([]Candle, 0, len(raw))
	for _, kline := range raw {
		if len(kline) < 7 {
			continue
		}

		openTime := parseJSONInt64(kline[0])
		open := parseJSONFloat64(kline[1])
		high := parseJSONFloat64(kline[2])
		low := parseJSONFloat64(kline[3])
		closeP := parseJSONFloat64(kline[4])
		volume := parseJSONFloat64(kline[5])
		closeTime := parseJSONInt64(kline[6])

		candles = append(candles, Candle{
			Symbol:    "BTCUSDT",
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeP,
			Volume:    volume,
			OpenTime:  time.UnixMilli(openTime),
			CloseTime: time.UnixMilli(closeTime),
		})
	}

	return candles, nil
}

func parseJSONFloat64(raw json.RawMessage) float64 {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	var f float64
	json.Unmarshal(raw, &f)
	return f
}

func parseJSONInt64(raw json.RawMessage) int64 {
	var i int64
	if err := json.Unmarshal(raw, &i); err == nil {
		return i
	}
	var f float64
	json.Unmarshal(raw, &f)
	return int64(f)
}

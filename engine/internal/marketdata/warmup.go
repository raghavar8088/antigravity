package marketdata

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// WarmupData holds historical candles fetched from Binance REST API
// for pre-filling strategy price buffers on engine startup.
type WarmupData struct {
	Candles1m []Candle
	Candles5m []Candle
	Candles1h []Candle // directly fetched 1h klines; used by S1 (needs ≥55 bars)
}

func FetchWarmupCandles(symbol string) (*WarmupData, error) {
	log.Printf("[WARMUP] Fetching real-time 1m, 5m, and 1h candles from Binance REST...")

	candles1m, err := fetchBinanceKlines("BTCUSDT", "1m", 300)
	if err != nil {
		return nil, fmt.Errorf("warmup 1m fetch failed: %w", err)
	}

	candles5m, err := fetchBinanceKlines("BTCUSDT", "5m", 300)
	if err != nil {
		return nil, fmt.Errorf("warmup 5m fetch failed: %w", err)
	}

	// Fetch 100 × 1h candles directly so S1 EMA Ribbon (needs ≥55 1h bars) is
	// ready on the first evaluation cycle rather than waiting 55+ hours for live data.
	candles1h, err := fetchBinanceKlines("BTCUSDT", "1h", 100)
	if err != nil {
		// Non-fatal: ScalerBundle will synthesise 1h from 5m as a fallback.
		log.Printf("[WARMUP] ⚠️  1h kline fetch failed (will synthesise from 5m): %v", err)
		candles1h = nil
	}

	log.Printf("[WARMUP] ✅ Loaded %d x 1m, %d x 5m, %d x 1h candles",
		len(candles1m), len(candles5m), len(candles1h))
	return &WarmupData{
		Candles1m: candles1m,
		Candles5m: candles5m,
		Candles1h: candles1h,
	}, nil
}

// fetchBinanceKlines calls the Binance public klines endpoint (no auth needed).
// Response: [[openTime, open, high, low, close, volume, closeTime, ...], ...]
func fetchBinanceKlines(symbol, interval string, limit int) ([]Candle, error) {
	url := fmt.Sprintf(
		"https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "RAIGEngine/4.0")

	resp, err := client.Do(req)
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
		if len(kline) < 6 {
			continue
		}
		var openTimeMs int64
		var open, high, low, closeP, volume float64
		var closeTimeMs int64
		if err := json.Unmarshal(kline[0], &openTimeMs); err != nil {
			continue
		}
		// Fields 1-5 are strings in Binance format
		var openS, highS, lowS, closeS, volS string
		if err := json.Unmarshal(kline[1], &openS); err != nil {
			continue
		}
		if err := json.Unmarshal(kline[2], &highS); err != nil {
			continue
		}
		if err := json.Unmarshal(kline[3], &lowS); err != nil {
			continue
		}
		if err := json.Unmarshal(kline[4], &closeS); err != nil {
			continue
		}
		if err := json.Unmarshal(kline[5], &volS); err != nil {
			continue
		}
		if err := json.Unmarshal(kline[6], &closeTimeMs); err != nil {
			continue
		}
		fmt.Sscanf(openS, "%f", &open)
		fmt.Sscanf(highS, "%f", &high)
		fmt.Sscanf(lowS, "%f", &low)
		fmt.Sscanf(closeS, "%f", &closeP)
		fmt.Sscanf(volS, "%f", &volume)

		candles = append(candles, Candle{
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeP,
			Volume:    volume,
			OpenTime:  time.UnixMilli(openTimeMs),
			CloseTime: time.UnixMilli(closeTimeMs),
		})
	}

	return candles, nil
}

func reverseCandles(c []Candle) {
	for i, j := 0, len(c)-1; i < j; i, j = i+1, j-1 {
		c[i], c[j] = c[j], c[i]
	}
}

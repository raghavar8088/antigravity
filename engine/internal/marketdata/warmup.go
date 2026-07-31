package marketdata

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// WarmupData holds historical candles for pre-filling strategy price buffers on
// engine startup.
//
// Source records which venue they came from. It is not decoration: warming a
// strategy on one venue's history and then trading it on another's live feed
// puts a discontinuity right at the boundary — the indicators carry Binance's
// levels into Delta's book — and every early signal is computed across it.
type WarmupData struct {
	Candles1m []Candle
	Candles5m []Candle
	Candles1h []Candle // directly fetched 1h klines; used by S1 (needs ≥55 bars)
	Source    string
}

// warmupVenue chooses where warmup history comes from. Delta by default,
// because this engine executes on Delta and the live tick feed now reads the
// same book; set WARMUP_VENUE=binance to pin the old behaviour.
func warmupVenue() string {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("WARMUP_VENUE"))); v != "" {
		return v
	}
	return "delta"
}

// FetchWarmupCandles loads recent 1m/5m/1h history.
//
// Delta first, Binance as the declared fallback. The fallback is deliberate
// rather than incidental — Delta's history endpoint rate-limits, and a desk that
// cannot warm up at all is worse than one warmed on a correlated venue — but the
// venue actually used is recorded and logged, because a desk quietly running on
// fallback history is not measuring the book it trades.
func FetchWarmupCandles(symbol string) (*WarmupData, error) {
	if warmupVenue() == "delta" {
		if d, err := fetchDeltaWarmup(); err == nil {
			log.Printf("[WARMUP] ✅ Loaded %d x 1m, %d x 5m, %d x 1h candles from DELTA (the venue this engine trades)",
				len(d.Candles1m), len(d.Candles5m), len(d.Candles1h))
			return d, nil
		} else {
			log.Printf("[WARMUP] ⚠️  Delta history unavailable (%v) — falling back to Binance. Strategies will warm on a venue the engine does not execute on.", err)
		}
	}

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

	log.Printf("[WARMUP] ✅ Loaded %d x 1m, %d x 5m, %d x 1h candles from BINANCE",
		len(candles1m), len(candles5m), len(candles1h))
	return &WarmupData{
		Candles1m: candles1m,
		Candles5m: candles5m,
		Candles1h: candles1h,
		Source:    "binance",
	}, nil
}

// deltaWarmupSymbol is the perpetual this engine's strategies trade.
func deltaWarmupSymbol() string {
	if s := strings.TrimSpace(os.Getenv("DELTA_WARMUP_SYMBOL")); s != "" {
		return s
	}
	return "BTCUSD"
}

// fetchDeltaWarmup pulls the same three resolutions from Delta's candle history.
//
// 1m and 5m are required: a partial warmup is treated as a failure so the engine
// falls back as a whole rather than mixing two venues' history inside one
// strategy's buffers, which is harder to notice and harder to reason about than
// either venue alone.
func fetchDeltaWarmup() (*WarmupData, error) {
	sym := deltaWarmupSymbol()
	now := time.Now().UTC()

	c1m, err := fetchDeltaCandlesAsCandles(sym, "1m", now.Add(-300*time.Minute), now)
	if err != nil {
		return nil, fmt.Errorf("1m: %w", err)
	}
	if len(c1m) < 60 {
		return nil, fmt.Errorf("1m: only %d candles returned", len(c1m))
	}

	c5m, err := fetchDeltaCandlesAsCandles(sym, "5m", now.Add(-300*5*time.Minute), now)
	if err != nil {
		return nil, fmt.Errorf("5m: %w", err)
	}
	if len(c5m) < 60 {
		return nil, fmt.Errorf("5m: only %d candles returned", len(c5m))
	}

	// 1h is best-effort, exactly as on the Binance path: ScalerBundle can
	// synthesise it from 5m.
	c1h, err := fetchDeltaCandlesAsCandles(sym, "1h", now.Add(-100*time.Hour), now)
	if err != nil {
		log.Printf("[WARMUP] ⚠️  Delta 1h fetch failed (will synthesise from 5m): %v", err)
		c1h = nil
	}

	return &WarmupData{Candles1m: c1m, Candles5m: c5m, Candles1h: c1h, Source: "delta"}, nil
}

// fetchDeltaCandlesAsCandles adapts Delta history into the Candle shape the
// warmup path feeds to strategies.
func fetchDeltaCandlesAsCandles(symbol, resolution string, from, to time.Time) ([]Candle, error) {
	raw, err := FetchDeltaHistoricalCandles(symbol, resolution, from, to)
	if err != nil {
		return nil, err
	}
	// Delta reports candle volume in CONTRACTS, the same as its trade stream: a
	// recent BTCUSD 1h bar reads 222107, which is 222.1 BTC at 0.001 BTC per
	// contract, not 222,107 BTC. Binance klines report volume in BTC directly.
	//
	// Left unscaled, the venue switch would multiply every volume figure by a
	// thousand at the exact moment the feed changed — and volume-spike, OBV and
	// volume-profile strategies would read that as the largest surge in the
	// instrument's history rather than as a change of units.
	//
	// The tick client learns this value from the live socket; REST history
	// carries no such field, so the listed value is used and named rather than
	// buried as a magic number.
	cv := deltaDefaultContractValueBTC

	out := make([]Candle, 0, len(raw))
	for _, c := range raw {
		// Delta stamps a candle with its OPEN time and reports no close time, so
		// the close is derived from the resolution rather than copied from the
		// open — a zero-width candle would defeat any downstream bar-boundary or
		// staleness logic that reads CloseTime.
		out = append(out, Candle{
			Symbol: symbol,
			Open:   c.Open, High: c.High, Low: c.Low, Close: c.Close,
			Volume:    c.Volume * cv,
			OpenTime:  c.OpenTime,
			CloseTime: c.OpenTime.Add(resolutionDuration(resolution)),
		})
	}
	return out, nil
}

// resolutionDuration maps Delta's resolution strings to a bar width.
func resolutionDuration(res string) time.Duration {
	switch res {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Minute
	}
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

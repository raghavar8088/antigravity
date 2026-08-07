package sharedfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Binance fallback.
//
// Used only when Delta fails or rate-limits. It keeps the desks running instead
// of going blind, but it is genuinely second choice: the Live Engine executes on
// Delta, so a strategy scored on Binance prices is scored on a book it will
// never trade. Every snapshot carries its Source so a mixed-venue window shows
// up in the UI rather than passing as Delta data.

var binanceHTTP = &http.Client{Timeout: 30 * time.Second}

// ErrNotOnBinance means Binance does not list this contract at all.
//
// Distinct from a transient failure because the remedy is different: a timeout
// is worth retrying, a delisting is not. Delta lists 220 perpetuals and Binance
// does not carry ~18 of them, so without this the fallback re-requests a symbol
// that can never resolve, on every poll, forever — 18 guaranteed-400 HTTP calls
// per cycle, and 18 error lines that train the reader to ignore feed errors.
var ErrNotOnBinance = errors.New("sharedfeed: contract is not listed on Binance")

// binanceUnlisted remembers contracts Binance has told us it does not have.
//
// Permanent-negative cache. Binance answers -1121 "Invalid symbol", which is a
// statement about the exchange's listings rather than about this request, so it
// is worth remembering. Cleared only by a restart, which is the right cadence:
// a newly-listed pair is not urgent, and never retrying beats hammering.
var binanceUnlisted sync.Map

// binanceCode extracts Binance's numeric error code from an error body.
func binanceCode(body string) int {
	var e struct {
		Code int `json:"code"`
	}
	if json.Unmarshal([]byte(body), &e) != nil {
		return 0
	}
	return e.Code
}

// binanceSymbol maps a Delta symbol to its Binance equivalent.
//
// Delta India quotes perpetuals against USD (BTCUSD); Binance quotes the same
// pairs against USDT (BTCUSDT). Without this the fallback would 400 on every
// request and look like a network fault.
func binanceSymbol(deltaSymbol string) string {
	s := strings.ToUpper(strings.TrimSpace(deltaSymbol))
	if strings.HasSuffix(s, "USDT") {
		return s
	}
	if strings.HasSuffix(s, "USD") {
		return s + "T"
	}
	return s
}

// binanceInterval maps a Delta resolution to a Binance kline interval. Delta
// uses "1m"/"1h"/"1d"; Binance agrees on most but not all spellings.
func binanceInterval(resolution string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w":
		return strings.ToLower(resolution), nil
	case "60m":
		return "1h", nil
	case "240m":
		return "4h", nil
	case "1D":
		return "1d", nil
	default:
		return "", fmt.Errorf("binance: unsupported resolution %q", resolution)
	}
}

// BinanceFetcher retrieves closed klines from Binance's public REST API.
func BinanceFetcher(ctx context.Context, symbol, resolution string, from, to time.Time) ([]Bar, error) {
	interval, err := binanceInterval(resolution)
	if err != nil {
		return nil, err
	}
	sym := binanceSymbol(symbol)

	// Already known absent — fail immediately rather than spending a request to
	// be told the same thing again.
	if _, gone := binanceUnlisted.Load(sym); gone {
		return nil, fmt.Errorf("%w: %s", ErrNotOnBinance, sym)
	}

	url := fmt.Sprintf(
		"https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=1000",
		sym, interval, from.UnixMilli(), to.UnixMilli(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := binanceHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("binance klines: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("binance klines read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// -1121 is "Invalid symbol": a fact about Binance's listings, not about
		// this request. Recorded so the desk stops asking.
		if binanceCode(string(body)) == -1121 {
			if _, seen := binanceUnlisted.LoadOrStore(sym, true); !seen {
				log.Printf("[sharedfeed] %s is not listed on Binance — fallback disabled for it (Delta remains primary)", sym)
			}
			return nil, fmt.Errorf("%w: %s", ErrNotOnBinance, sym)
		}
		return nil, fmt.Errorf("binance klines %s: HTTP %d: %s", sym, resp.StatusCode, truncate(string(body), 200))
	}

	var raw [][]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("binance klines parse: %w", err)
	}

	out := make([]Bar, 0, len(raw))
	for _, r := range raw {
		if len(r) < 7 {
			continue
		}
		openMs, ok := r[0].(float64)
		if !ok {
			continue
		}
		open := time.UnixMilli(int64(openMs)).UTC()

		// Drop the in-progress bar: Binance returns the live candle, and serving
		// it would hand every strategy a price that can still move.
		closeMs, ok := r[6].(float64)
		if !ok || time.UnixMilli(int64(closeMs)).After(to) {
			continue
		}

		num := func(i int) float64 {
			s, ok := r[i].(string)
			if !ok {
				return 0
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return 0
			}
			return v
		}
		out = append(out, Bar{
			OpenTime: open,
			Open:     num(1),
			High:     num(2),
			Low:      num(3),
			Close:    num(4),
			Volume:   num(5),
		})
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

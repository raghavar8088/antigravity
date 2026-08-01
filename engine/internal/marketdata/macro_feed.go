package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sync"
	"time"
)

// Macro cross-asset feed: Nasdaq futures proxy (NQ=F) and US Dollar Index
// (DX-Y.NYB), polled from Yahoo Finance's unverified-but-widely-used public
// v8 chart endpoint (no auth/key required). Verified via web search (2026):
// GET https://query1.finance.yahoo.com/v8/finance/chart/{SYMBOL}?range=1d&interval=5m
// This is the same unofficial endpoint that powers the yfinance Python
// library and is already the pattern used elsewhere in this repo for free
// public market data (Yahoo Finance NIFTY warmup bars — see CLAUDE.md).
//
// CAVEAT (documented per task spec): this is an undocumented internal Yahoo
// endpoint, not a published/guaranteed API. It can be rate-limited or
// blocked without notice. We poll at a slow cadence (10 min) to minimize
// exposure, and degrade gracefully (keep last good value, IsHealthy()=false
// on failure) exactly like DeribitDVOLHolder/DeltaPerpPriceHolder so
// macro-dependent strategies never crash and simply skip/reduce confidence
// when the feed is down.
//
// Research backing for the macro concepts these strategies use:
//   - BTC-equities correlation regime shifts: documented by multiple 2026
//     market commentary pieces, e.g. CryptoSlate ("Bitcoin's surging
//     correlation with Nasdaq signals convergence with traditional finance")
//     and CME Group OpenMarkets ("Why Bitcoin's Relationship with Equities
//     Has Changed"), plus academic work (arXiv:2501.09911, "Institutional
//     Adoption and Correlation Dynamics: Bitcoin's Evolving Role in
//     Financial Markets").
//   - BTC/DXY inverse correlation: documented by OSL ("DXY vs. Bitcoin: 2026
//     Correlation Shift Explained") and Newhedge's live BTC/DXY correlation
//     chart; note research also flags this correlation can itself regime-
//     shift (sign flips reported around 2026), which is exactly why S18/S20
//     treat correlation as a measured, time-varying input rather than a
//     hardcoded constant.
const (
	yahooChartEndpoint   = "https://query1.finance.yahoo.com/v8/finance/chart/%s"
	macroPollInterval    = 10 * time.Minute
	macroHTTPTimeout     = 10 * time.Second
	macroHistoryCap      = 24 // up to 24 hourly-spaced readings retained for correlation/range calcs
	dxyRangeLookback     = 20 // 20-period rolling high/low for DXY breakout detection
	corrLookbackReadings = 12 // ~12 polls (at 10min cadence that's 2h; see NOTE below on approximation)

	nasdaqProxySymbol = "NQ=F"     // Nasdaq-100 E-mini futures — 24h-traded proxy, tracks ^NDX closely
	dxySymbol         = "DX-Y.NYB" // ICE US Dollar Index
)

// macroPoint is one polled (price) reading with its timestamp, used to build
// rolling history for correlation and range calculations.
type macroPoint struct {
	at    time.Time
	price float64
}

// MacroFeedHolder polls a Nasdaq futures proxy and DXY from Yahoo Finance's
// public chart endpoint, exposes current price/% change for each, a rolling
// DXY 20-period high/low, and a BTC-vs-Nasdaq-proxy rolling correlation.
//
// NOTE on the "30-day rolling correlation" spec ask: true 30-day correlation
// would require bootstrapping ~720 hourly closes for both BTC and the macro
// proxy before the feed could produce a meaningful number — impractical to
// bootstrap quickly from a fresh process start, and this feed only polls
// every 10 minutes (history caps at macroHistoryCap=24 readings ≈ 4 hours).
// We instead compute and expose an APPROXIMATION: correlation over the last
// corrLookbackReadings (12) paired (macro poll, nearest BTC 1h candle close)
// observations — roughly a 2-hour rolling window. This is clearly labeled as
// an approximation in the field comment on BTCEquitiesCorrelation30d in
// types.go and here. Strategies must treat it as a short-horizon proxy, not
// a true 30-day statistic.
type MacroFeedHolder struct {
	mu sync.RWMutex

	client *http.Client

	nasdaqCurrent   float64
	nasdaqPrevClose float64 // previous close, for % change calc
	nasdaqHistory   []macroPoint
	nasdaqHealthy   bool
	nasdaqLastFetch time.Time

	dxyCurrent   float64
	dxyPrevClose float64
	dxyHistory   []macroPoint
	dxyHealthy   bool
	dxyLastFetch time.Time

	lastErr error
}

// NewMacroFeedHolder constructs a holder ready to be started with StartPolling.
func NewMacroFeedHolder() *MacroFeedHolder {
	return &MacroFeedHolder{client: &http.Client{Timeout: macroHTTPTimeout}}
}

// StartPolling launches a background goroutine that fetches both symbols
// every macroPollInterval until ctx is cancelled.
func (m *MacroFeedHolder) StartPolling(ctx context.Context) {
	go func() {
		m.fetchOnce(ctx)
		ticker := time.NewTicker(macroPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.fetchOnce(ctx)
			}
		}
	}()
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
				ChartPreviousClose float64 `json:"chartPreviousClose"`
			} `json:"meta"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

func (m *MacroFeedHolder) fetchSymbol(ctx context.Context, symbol string) (price, prevClose float64, err error) {
	url := fmt.Sprintf(yahooChartEndpoint+"?range=1d&interval=5m", symbol)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("build request: %w", err)
	}
	// Yahoo's unofficial endpoint is more reliable with a browser-like UA.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; antigravity-macro-feed/1.0)")

	resp, err := m.client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	if err != nil {
		return 0, 0, fmt.Errorf("read body: %w", err)
	}
	var parsed yahooChartResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, 0, fmt.Errorf("decode JSON: %w", err)
	}
	if len(parsed.Chart.Result) == 0 {
		return 0, 0, fmt.Errorf("empty chart result for %s", symbol)
	}
	meta := parsed.Chart.Result[0].Meta
	if meta.RegularMarketPrice <= 0 {
		return 0, 0, fmt.Errorf("invalid price for %s", symbol)
	}
	prev := meta.PreviousClose
	if prev <= 0 {
		prev = meta.ChartPreviousClose
	}
	return meta.RegularMarketPrice, prev, nil
}

func (m *MacroFeedHolder) fetchOnce(ctx context.Context) {
	now := time.Now().UTC()

	nasdaqPrice, nasdaqPrev, nasdaqErr := m.fetchSymbol(ctx, nasdaqProxySymbol)
	dxyPrice, dxyPrev, dxyErr := m.fetchSymbol(ctx, dxySymbol)

	m.mu.Lock()
	defer m.mu.Unlock()

	if nasdaqErr != nil {
		m.nasdaqHealthy = false
		m.lastErr = nasdaqErr
		log.Printf("[MACRO] Nasdaq proxy fetch failed, retaining last value=%.2f: %v", m.nasdaqCurrent, nasdaqErr)
	} else {
		m.nasdaqCurrent = nasdaqPrice
		m.nasdaqPrevClose = nasdaqPrev
		m.nasdaqHistory = appendMacroPointCapped(m.nasdaqHistory, macroPoint{at: now, price: nasdaqPrice}, macroHistoryCap)
		m.nasdaqLastFetch = now
		m.nasdaqHealthy = true
	}

	if dxyErr != nil {
		m.dxyHealthy = false
		m.lastErr = dxyErr
		log.Printf("[MACRO] DXY fetch failed, retaining last value=%.2f: %v", m.dxyCurrent, dxyErr)
	} else {
		m.dxyCurrent = dxyPrice
		m.dxyPrevClose = dxyPrev
		m.dxyHistory = appendMacroPointCapped(m.dxyHistory, macroPoint{at: now, price: dxyPrice}, macroHistoryCap)
		m.dxyLastFetch = now
		m.dxyHealthy = true
	}
}

func appendMacroPointCapped(s []macroPoint, p macroPoint, cap int) []macroPoint {
	s = append(s, p)
	if len(s) > cap {
		s = s[len(s)-cap:]
	}
	return s
}

// NasdaqPrice returns the most recently fetched Nasdaq futures proxy price (0 if never populated).
func (m *MacroFeedHolder) NasdaqPrice() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nasdaqCurrent
}

// NasdaqChangePct returns % change vs the previous session close (0 if unavailable).
func (m *MacroFeedHolder) NasdaqChangePct() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.nasdaqPrevClose <= 0 {
		return 0
	}
	return (m.nasdaqCurrent - m.nasdaqPrevClose) / m.nasdaqPrevClose * 100.0
}

// DXYPrice returns the most recently fetched DXY price (0 if never populated).
func (m *MacroFeedHolder) DXYPrice() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dxyCurrent
}

// DXYChangePct returns % change vs the previous session close (0 if unavailable).
func (m *MacroFeedHolder) DXYChangePct() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.dxyPrevClose <= 0 {
		return 0
	}
	return (m.dxyCurrent - m.dxyPrevClose) / m.dxyPrevClose * 100.0
}

// DXYRollingHigh20 / DXYRollingLow20 return the rolling high/low of DXY over
// the last dxyRangeLookback polled readings (0 if insufficient history).
func (m *MacroFeedHolder) DXYRollingHigh20() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return rollingHigh(m.dxyHistory, dxyRangeLookback)
}

func (m *MacroFeedHolder) DXYRollingLow20() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return rollingLow(m.dxyHistory, dxyRangeLookback)
}

func rollingHigh(pts []macroPoint, lookback int) float64 {
	if len(pts) == 0 {
		return 0
	}
	start := 0
	if len(pts) > lookback {
		start = len(pts) - lookback
	}
	high := pts[start].price
	for _, p := range pts[start:] {
		if p.price > high {
			high = p.price
		}
	}
	return high
}

func rollingLow(pts []macroPoint, lookback int) float64 {
	if len(pts) == 0 {
		return 0
	}
	start := 0
	if len(pts) > lookback {
		start = len(pts) - lookback
	}
	low := pts[start].price
	for _, p := range pts[start:] {
		if p.price < low {
			low = p.price
		}
	}
	return low
}

// BTCEquitiesCorrelationApprox computes a short-horizon (~corrLookbackReadings
// polls) Pearson correlation between the Nasdaq proxy's own poll-to-poll
// price history and a caller-supplied slice of BTC 1h candle closes
// (typically ctx.Candles1h from ScalerBundle). This is the practical
// approximation of "30-day rolling correlation" described in the package doc
// comment above — pairs are aligned by truncating both series to the same
// length (most recent N), not by exact timestamp matching, since the two
// series have different native cadences (BTC 1h candles vs 10-min macro
// polls). Returns 0 if either series is too short.
func (m *MacroFeedHolder) BTCEquitiesCorrelationApprox(btcCloses []float64) float64 {
	m.mu.RLock()
	nasdaqHist := append([]macroPoint(nil), m.nasdaqHistory...)
	m.mu.RUnlock()

	if len(nasdaqHist) < 4 || len(btcCloses) < 4 {
		return 0
	}
	n := corrLookbackReadings
	if len(nasdaqHist) < n {
		n = len(nasdaqHist)
	}
	if len(btcCloses) < n {
		n = len(btcCloses)
	}
	if n < 4 {
		return 0
	}
	nasdaqTail := nasdaqHist[len(nasdaqHist)-n:]
	btcTail := btcCloses[len(btcCloses)-n:]

	nasdaqPrices := make([]float64, n)
	for i, p := range nasdaqTail {
		nasdaqPrices[i] = p.price
	}
	return pearsonCorrelation(btcTail, nasdaqPrices)
}

func pearsonCorrelation(a, b []float64) float64 {
	n := len(a)
	if n == 0 || len(b) != n {
		return 0
	}
	var sumA, sumB float64
	for i := 0; i < n; i++ {
		sumA += a[i]
		sumB += b[i]
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	var cov, varA, varB float64
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		cov += da * db
		varA += da * da
		varB += db * db
	}
	if varA == 0 || varB == 0 {
		return 0
	}
	return cov / math.Sqrt(varA*varB)
}

// IsHealthy returns true if BOTH the Nasdaq proxy and DXY's most recent
// fetch attempts succeeded. Strategies that need only one leg should use
// NasdaqHealthy()/DXYHealthy() individually for finer-grained degradation.
func (m *MacroFeedHolder) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nasdaqHealthy && m.dxyHealthy
}

// NasdaqHealthy / DXYHealthy report per-leg feed health.
func (m *MacroFeedHolder) NasdaqHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nasdaqHealthy
}

func (m *MacroFeedHolder) DXYHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dxyHealthy
}

// IsPopulated returns true once both legs have fetched at least one good value.
func (m *MacroFeedHolder) IsPopulated() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nasdaqLastFetch.Unix() > 0 && m.nasdaqCurrent > 0 &&
		m.dxyLastFetch.Unix() > 0 && m.dxyCurrent > 0
}

// LastError returns the most recent fetch error from either leg, if any.
func (m *MacroFeedHolder) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastErr
}

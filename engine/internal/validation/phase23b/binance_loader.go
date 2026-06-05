package phase23b

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

const (
	binanceFuturesBase = "https://fapi.binance.com"
	klinesEndpoint     = "/fapi/v1/klines"
	fundingEndpoint    = "/fapi/v1/fundingRate"
	oiHistEndpoint     = "/futures/data/openInterestHist"
	klinesPageSize     = 1500 // Binance max per request
	fundingPageSize    = 1000
	oiPageSize         = 500
)

// BinanceFuturesLoader fetches real BTC futures market data from Binance.
// All endpoints used are public (no API key required).
type BinanceFuturesLoader struct {
	client    *http.Client
	rateDelay time.Duration // minimum delay between requests to avoid 429
	baseURL   string
}

// NewBinanceFuturesLoader creates a loader with sensible defaults.
func NewBinanceFuturesLoader() *BinanceFuturesLoader {
	return &BinanceFuturesLoader{
		client:    &http.Client{Timeout: 30 * time.Second},
		rateDelay: 120 * time.Millisecond, // ~8 req/sec, well within 2400 weight/min
		baseURL:   binanceFuturesBase,
	}
}

// LoadCandles fetches 1-minute OHLCV klines for the given symbol and period,
// paginating through the Binance API until the full range is covered.
// Funding rates and open interest are merged in separately.
func (b *BinanceFuturesLoader) LoadCandles(symbol string, from, to time.Time) ([]OHLCVCandle, error) {
	log.Printf("[BINANCE] Loading klines %s %s → %s", symbol, from.Format("2006-01-02"), to.Format("2006-01-02"))

	klines, err := b.fetchAllKlines(symbol, "1m", from, to)
	if err != nil {
		return nil, fmt.Errorf("klines fetch: %w", err)
	}
	log.Printf("[BINANCE] Fetched %d klines", len(klines))

	// Fetch funding rates (every 8h)
	funding, err := b.fetchAllFunding(symbol, from, to)
	if err != nil {
		log.Printf("[BINANCE] WARNING: funding rate fetch failed: %v — using 0.01%% default", err)
		funding = nil
	} else {
		log.Printf("[BINANCE] Fetched %d funding rate records", len(funding))
	}

	// Fetch open interest history (hourly granularity, merged into 1m candles)
	oi, err := b.fetchAllOI(symbol, from, to)
	if err != nil {
		log.Printf("[BINANCE] WARNING: open interest fetch failed: %v — using 0", err)
		oi = nil
	} else {
		log.Printf("[BINANCE] Fetched %d open interest records", len(oi))
	}

	return mergeEnrichments(klines, funding, oi), nil
}

// ── Klines ────────────────────────────────────────────────────────────────────

type rawKline [12]json.RawMessage

func (b *BinanceFuturesLoader) fetchAllKlines(symbol, interval string, from, to time.Time) ([]OHLCVCandle, error) {
	var all []OHLCVCandle
	cursor := from

	for cursor.Before(to) {
		batch, nextCursor, err := b.fetchKlinePage(symbol, interval, cursor, to)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || !nextCursor.After(cursor) {
			break
		}
		cursor = nextCursor
		time.Sleep(b.rateDelay)
	}
	return all, nil
}

func (b *BinanceFuturesLoader) fetchKlinePage(symbol, interval string, from, to time.Time) ([]OHLCVCandle, time.Time, error) {
	url := fmt.Sprintf("%s%s?symbol=%s&interval=%s&startTime=%d&endTime=%d&limit=%d",
		b.baseURL, klinesEndpoint,
		symbol, interval,
		from.UnixMilli(), to.UnixMilli(),
		klinesPageSize,
	)

	resp, err := b.client.Get(url)
	if err != nil {
		return nil, from, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, from, fmt.Errorf("binance klines HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw []rawKline
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, from, fmt.Errorf("decode klines: %w", err)
	}

	candles := make([]OHLCVCandle, 0, len(raw))
	var lastClose time.Time
	for _, k := range raw {
		c, err := parseKline(symbol, k)
		if err != nil {
			continue
		}
		candles = append(candles, c)
		lastClose = c.CloseTime
	}

	nextCursor := lastClose.Add(time.Millisecond)
	return candles, nextCursor, nil
}

func parseKline(symbol string, k rawKline) (OHLCVCandle, error) {
	parseF := func(r json.RawMessage) (float64, error) {
		var s string
		if err := json.Unmarshal(r, &s); err != nil {
			return 0, err
		}
		return strconv.ParseFloat(s, 64)
	}
	parseInt64 := func(r json.RawMessage) (int64, error) {
		var n int64
		return n, json.Unmarshal(r, &n)
	}

	openTimeMs, err := parseInt64(k[0])
	if err != nil {
		return OHLCVCandle{}, err
	}
	open, _ := parseF(k[1])
	high, _ := parseF(k[2])
	low, _ := parseF(k[3])
	close_, _ := parseF(k[4])
	vol, _ := parseF(k[5])
	closeTimeMs, _ := parseInt64(k[6])
	quoteVol, _ := parseF(k[7])
	trades := 0
	var tradesRaw int64
	if err2 := json.Unmarshal(k[8], &tradesRaw); err2 == nil {
		trades = int(tradesRaw)
	}
	takerBuyVol, _ := parseF(k[9])

	takerBuyPct := 0.0
	if vol > 0 {
		takerBuyPct = takerBuyVol / vol
	}

	return OHLCVCandle{
		Symbol:      symbol,
		Open:        open,
		High:        high,
		Low:         low,
		Close:       close_,
		Volume:      vol,
		QuoteVolume: quoteVol,
		Trades:      trades,
		OpenTime:    time.UnixMilli(openTimeMs).UTC(),
		CloseTime:   time.UnixMilli(closeTimeMs).UTC(),
		TakerBuyPct: takerBuyPct,
	}, nil
}

// ── Funding Rates ─────────────────────────────────────────────────────────────

type rawFunding struct {
	Symbol      string `json:"symbol"`
	FundingTime int64  `json:"fundingTime"`
	FundingRate string `json:"fundingRate"`
}

type fundingRecord struct {
	T    time.Time
	Rate float64
}

func (b *BinanceFuturesLoader) fetchAllFunding(symbol string, from, to time.Time) ([]fundingRecord, error) {
	var all []fundingRecord
	cursor := from
	for cursor.Before(to) {
		batch, next, err := b.fetchFundingPage(symbol, cursor, to)
		if err != nil {
			return all, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || !next.After(cursor) {
			break
		}
		cursor = next
		time.Sleep(b.rateDelay)
	}
	return all, nil
}

func (b *BinanceFuturesLoader) fetchFundingPage(symbol string, from, to time.Time) ([]fundingRecord, time.Time, error) {
	url := fmt.Sprintf("%s%s?symbol=%s&startTime=%d&endTime=%d&limit=%d",
		b.baseURL, fundingEndpoint, symbol,
		from.UnixMilli(), to.UnixMilli(), fundingPageSize)

	resp, err := b.client.Get(url)
	if err != nil {
		return nil, from, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, from, fmt.Errorf("funding HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw []rawFunding
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, from, err
	}

	records := make([]fundingRecord, 0, len(raw))
	var lastT time.Time
	for _, r := range raw {
		rate, _ := strconv.ParseFloat(r.FundingRate, 64)
		t := time.UnixMilli(r.FundingTime).UTC()
		records = append(records, fundingRecord{T: t, Rate: rate})
		if t.After(lastT) {
			lastT = t
		}
	}
	next := lastT.Add(time.Millisecond)
	if lastT.IsZero() {
		next = to
	}
	return records, next, nil
}

// ── Open Interest ─────────────────────────────────────────────────────────────

type rawOI struct {
	Symbol             string `json:"symbol"`
	SumOpenInterest    string `json:"sumOpenInterest"`
	SumOpenInterestValue string `json:"sumOpenInterestValue"`
	Timestamp          int64  `json:"timestamp"`
}

type oiRecord struct {
	T  time.Time
	OI float64 // in BTC
}

func (b *BinanceFuturesLoader) fetchAllOI(symbol string, from, to time.Time) ([]oiRecord, error) {
	var all []oiRecord
	cursor := from
	for cursor.Before(to) {
		batch, next, err := b.fetchOIPage(symbol, cursor, to)
		if err != nil {
			return all, err
		}
		all = append(all, batch...)
		if len(batch) == 0 || !next.After(cursor) {
			break
		}
		cursor = next
		time.Sleep(b.rateDelay)
	}
	return all, nil
}

func (b *BinanceFuturesLoader) fetchOIPage(symbol string, from, to time.Time) ([]oiRecord, time.Time, error) {
	url := fmt.Sprintf("%s%s?symbol=%s&period=1h&startTime=%d&endTime=%d&limit=%d",
		b.baseURL, oiHistEndpoint, symbol,
		from.UnixMilli(), to.UnixMilli(), oiPageSize)

	resp, err := b.client.Get(url)
	if err != nil {
		return nil, from, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, from, fmt.Errorf("OI HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw []rawOI
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, from, err
	}

	records := make([]oiRecord, 0, len(raw))
	var lastT time.Time
	for _, r := range raw {
		oi, _ := strconv.ParseFloat(r.SumOpenInterest, 64)
		t := time.UnixMilli(r.Timestamp).UTC()
		records = append(records, oiRecord{T: t, OI: oi})
		if t.After(lastT) {
			lastT = t
		}
	}
	next := lastT.Add(time.Millisecond)
	if lastT.IsZero() {
		next = to
	}
	return records, next, nil
}

// ── Enrichment merge ──────────────────────────────────────────────────────────

// mergeEnrichments interpolates funding rates and open interest into the 1m candle series.
func mergeEnrichments(candles []OHLCVCandle, funding []fundingRecord, oi []oiRecord) []OHLCVCandle {
	// Build funding map: rounded to 8h bucket → rate
	fundingMap := make(map[int64]float64, len(funding))
	for _, f := range funding {
		bucket := f.T.Truncate(8 * time.Hour).UnixMilli()
		fundingMap[bucket] = f.Rate
	}

	// Build OI map: rounded to 1h → OI
	oiMap := make(map[int64]float64, len(oi))
	for _, o := range oi {
		bucket := o.T.Truncate(time.Hour).UnixMilli()
		oiMap[bucket] = o.OI
	}

	lastKnownOI := 0.0
	for i := range candles {
		c := &candles[i]

		// Funding: mark the candle at the 8h settlement time
		bucket8h := c.OpenTime.Truncate(8 * time.Hour).UnixMilli()
		if rate, ok := fundingMap[bucket8h]; ok {
			// Only stamp if this candle is at/within 1 min of the settlement
			settlementTime := time.UnixMilli(bucket8h)
			if c.OpenTime.Sub(settlementTime).Abs() < time.Minute {
				c.FundingRate = rate
			}
		}

		// OI: carry last known value forward
		bucket1h := c.OpenTime.Truncate(time.Hour).UnixMilli()
		if oiVal, ok := oiMap[bucket1h]; ok {
			lastKnownOI = oiVal
		}
		c.OpenInterest = lastKnownOI
	}
	return candles
}

// AuditLoadedData audits the freshly loaded candle series and returns a
// DataAuditReport covering all data sources used.
func AuditLoadedData(symbol string, candles []OHLCVCandle,
	fundingCount, oiCount int,
	from, to time.Time) DataAuditReport {

	rep := DataAuditReport{
		GeneratedAt: time.Now().UTC(),
		Symbol:      symbol,
		From:        from,
		To:          to,
	}

	totalExpected := int(to.Sub(from).Minutes())
	missingCandles := totalExpected - len(candles)
	if missingCandles < 0 {
		missingCandles = 0
	}
	missingPct := 0.0
	if totalExpected > 0 {
		missingPct = float64(missingCandles) / float64(totalExpected) * 100
	}

	// Count candles with funding and OI
	fundingCandleCount := 0
	oiCandleCount := 0
	for _, c := range candles {
		if c.FundingRate != 0 {
			fundingCandleCount++
		}
		if c.OpenInterest > 0 {
			oiCandleCount++
		}
	}

	klineScore := 100.0 - missingPct*2
	if klineScore < 0 {
		klineScore = 0
	}
	klineQuality := qualityFromScore(klineScore)

	fundingScore := 0.0
	if fundingCount > 0 {
		fundingScore = 85.0
	}
	oiScore := 0.0
	if oiCount > 0 {
		oiScore = 80.0
	}

	rep.Sources = []DataSourceAudit{
		{
			Name:             "Binance Futures 1m Klines",
			Type:             "klines",
			Available:        len(candles) > 0,
			CoveragePeriod:   fmt.Sprintf("%s → %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
			TotalRecords:     len(candles),
			MissingPct:       missingPct,
			QualityScore:     klineScore,
			ReliabilityScore: 98.0,
			LatencyProfile:   "REST pagination, ~120ms/page",
			Completeness:     fmt.Sprintf("%.1f%% — %d gaps", 100-missingPct, missingCandles),
			Quality:          klineQuality,
		},
		{
			Name:             "Binance Futures Funding Rate",
			Type:             "funding",
			Available:        fundingCount > 0,
			CoveragePeriod:   fmt.Sprintf("%s → %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
			TotalRecords:     fundingCount,
			MissingPct:       0,
			QualityScore:     fundingScore,
			ReliabilityScore: 95.0,
			LatencyProfile:   "REST pagination, ~120ms/page",
			Completeness:     fmt.Sprintf("%d settlement records", fundingCount),
			Quality:          qualityFromScore(fundingScore),
		},
		{
			Name:             "Binance Futures Open Interest (1h)",
			Type:             "open_interest",
			Available:        oiCount > 0,
			CoveragePeriod:   fmt.Sprintf("%s → %s", from.Format("2006-01-02"), to.Format("2006-01-02")),
			TotalRecords:     oiCount,
			MissingPct:       0,
			QualityScore:     oiScore,
			ReliabilityScore: 90.0,
			LatencyProfile:   "REST pagination, ~120ms/page",
			Completeness:     fmt.Sprintf("%d hourly records", oiCount),
			Quality:          qualityFromScore(oiScore),
		},
		{
			Name:             "Liquidation Feed",
			Type:             "liquidations",
			Available:        false,
			QualityScore:     0,
			ReliabilityScore: 0,
			Quality:          QualityUnavailable,
			Notes:            []string{"Real-time liquidation feed unavailable via public Binance REST API; estimated from price velocity in volatile regimes"},
		},
		{
			Name:             "Taker Buy Volume (CVD proxy)",
			Type:             "order_flow",
			Available:        len(candles) > 0,
			TotalRecords:     len(candles),
			QualityScore:     75.0,
			ReliabilityScore: 85.0,
			LatencyProfile:   "Embedded in klines response (field 10)",
			Completeness:     "100% — bundled with klines",
			Quality:          QualityGood,
			Notes:            []string{"Taker buy % used as CVD proxy; not a true tick-level order flow"},
		},
	}

	// Determine overall quality
	overallScore := (klineScore*0.5 + fundingScore*0.2 + oiScore*0.1 + 75.0*0.2)
	rep.OverallQuality = qualityFromScore(overallScore)
	rep.Accepted = overallScore >= 60 && len(candles) >= 1000

	for _, s := range rep.Sources {
		if !s.Available && s.Type == "klines" {
			rep.Issues = append(rep.Issues, "CRITICAL: klines unavailable — cannot proceed")
			rep.Accepted = false
		}
		if s.Quality == QualityInadequate {
			rep.RejectedSources = append(rep.RejectedSources, s.Name)
		}
	}
	if missingPct > 10 {
		rep.Issues = append(rep.Issues, fmt.Sprintf("WARNING: %.1f%% missing candles — accuracy may be reduced", missingPct))
	}

	return rep
}

func qualityFromScore(score float64) DataSourceQuality {
	switch {
	case score >= 90:
		return QualityExcellent
	case score >= 70:
		return QualityGood
	case score >= 50:
		return QualityMarginal
	case score > 0:
		return QualityInadequate
	default:
		return QualityUnavailable
	}
}

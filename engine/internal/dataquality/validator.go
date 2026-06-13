package dataquality

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// scorePenalty is the quality-score deduction per failed check severity.
const (
	penaltyMinor    = 5.0
	penaltyModerate = 25.0
	penaltySevere   = 50.0

	// maxPriceMovePct is the maximum percentage change in Close allowed
	// between consecutive candles (Rule 2).
	maxPriceMovePct = 0.30

	// washVolumeMultiple: volume > N× recent avg + price move < threshold
	// flags potential wash trading.
	washVolumeMultiple = 5.0
	washPriceMoveMin   = 0.0005 // 0.05% of range

	// volumeHistoryLen is the number of candles used for the wash-trade
	// volume average.
	volumeHistoryLen = 20
)

// Validator validates incoming candles and tracks per-source health.
// It is safe to use from multiple goroutines.
type Validator struct {
	lastValidPrice map[string]float64
	sourceHealth   map[string]SourceStatus
	volumeHistory  map[string][]float64 // rolling volume per symbol
	latestScore    float64              // most recent quality score, for /ready probe
	mu             sync.RWMutex
}

// NewValidator creates a ready-to-use Validator.
func NewValidator() *Validator {
	return &Validator{
		lastValidPrice: make(map[string]float64),
		sourceHealth:   make(map[string]SourceStatus),
		volumeHistory:  make(map[string][]float64),
	}
}

// ValidateCandle runs all validation rules against candle and returns a
// ValidationResult. intervalSeconds is the expected candle duration (e.g. 60
// for 1-minute candles) and is used for the timestamp-recency check.
func (v *Validator) ValidateCandle(candle OHLCV, source string, intervalSeconds int) ValidationResult {
	result := ValidationResult{
		Source:    source,
		Timestamp: time.Now().UTC(),
		Valid:     true,
		Severity:  SeverityOK,
	}

	score := 100.0
	var failures []string
	worstSeverity := SeverityOK

	upgrade := func(s Severity) {
		if severityRank(s) > severityRank(worstSeverity) {
			worstSeverity = s
		}
	}

	// Rule 1 — Timestamp recency.
	staleness := time.Since(candle.Time)
	maxAge := time.Duration(2*intervalSeconds) * time.Second
	if staleness > maxAge {
		failures = append(failures, fmt.Sprintf("stale candle: age=%s max=%s", staleness.Round(time.Second), maxAge))
		score -= penaltyModerate
		upgrade(SeverityModerate)
	}

	// Rule 2 — Price sanity vs last valid price.
	v.mu.RLock()
	last, hasLast := v.lastValidPrice[candle.Symbol]
	v.mu.RUnlock()
	if hasLast && last > 0 {
		move := math.Abs(candle.Close-last) / last
		if move > maxPriceMovePct {
			failures = append(failures, fmt.Sprintf("price spike: close=%.2f last=%.2f move=%.2f%%", candle.Close, last, move*100))
			score -= penaltyModerate
			upgrade(SeverityModerate)
		}
	}

	// Rule 3 — Volume must be positive.
	if candle.Volume <= 0 {
		failures = append(failures, "zero/negative volume")
		score -= penaltySevere
		upgrade(SeveritySevere)
	}

	// Rule 4 — OHLC integrity + NaN/Inf guards.
	ohlcFail := false
	for name, val := range map[string]float64{
		"Open": candle.Open, "High": candle.High,
		"Low": candle.Low, "Close": candle.Close, "Volume": candle.Volume,
	} {
		if math.IsNaN(val) || math.IsInf(val, 0) || val < 0 {
			failures = append(failures, fmt.Sprintf("invalid value %s=%.6g", name, val))
			score -= penaltySevere
			upgrade(SeveritySevere)
			ohlcFail = true
		}
	}
	if !ohlcFail {
		if candle.High < candle.Open || candle.High < candle.Close {
			failures = append(failures, fmt.Sprintf("high(%.2f) < open(%.2f) or close(%.2f)", candle.High, candle.Open, candle.Close))
			score -= penaltySevere
			upgrade(SeveritySevere)
		}
		if candle.Low > candle.Open || candle.Low > candle.Close {
			failures = append(failures, fmt.Sprintf("low(%.2f) > open(%.2f) or close(%.2f)", candle.Low, candle.Open, candle.Close))
			score -= penaltySevere
			upgrade(SeveritySevere)
		}
		if candle.High < candle.Low {
			failures = append(failures, fmt.Sprintf("high(%.2f) < low(%.2f)", candle.High, candle.Low))
			score -= penaltySevere
			upgrade(SeveritySevere)
		}
	}

	// Rule 5 — Wash trading detection (log only, do not reject).
	if candle.Volume > 0 {
		v.mu.Lock()
		hist := v.volumeHistory[candle.Symbol]
		if len(hist) >= volumeHistoryLen {
			avg := average(hist)
			candleRange := candle.High - candle.Low
			priceMove := 0.0
			if candleRange > 0 {
				priceMove = math.Abs(candle.Close-candle.Open) / candleRange
			}
			if avg > 0 && candle.Volume > washVolumeMultiple*avg && priceMove < washPriceMoveMin {
				slog.Warn("[dataquality] wash trade suspect",
					"symbol", candle.Symbol,
					"volume", candle.Volume,
					"avg_volume", avg,
					"price_move_pct", priceMove,
					"source", source,
				)
			}
		}
		hist = append(hist, candle.Volume)
		if len(hist) > volumeHistoryLen {
			hist = hist[len(hist)-volumeHistoryLen:]
		}
		v.volumeHistory[candle.Symbol] = hist
		v.mu.Unlock()
	}

	if score < 0 {
		score = 0
	}

	result.QualityScore = score
	result.FailedChecks = failures
	result.Severity = worstSeverity
	result.Valid = len(failures) == 0

	if len(failures) > 0 {
		slog.Warn("[dataquality] validation failure",
			"symbol", candle.Symbol,
			"source", source,
			"score", score,
			"failures", failures,
			"candle_time", candle.Time,
		)
	}

	// Update last valid price only on fully clean candles.
	v.mu.Lock()
	if result.Valid && candle.Close > 0 {
		v.lastValidPrice[candle.Symbol] = candle.Close
	}
	v.latestScore = score
	v.mu.Unlock()

	return result
}

// LatestScore returns the most recent quality score (0–100).
// Returns 100 if no candle has been validated yet.
func (v *Validator) LatestScore() float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.latestScore == 0 {
		return 100 // no data yet — assume healthy
	}
	return v.latestScore
}

// GetAction maps a quality score to the recommended trading action.
func (v *Validator) GetAction(score float64) Action {
	switch {
	case score >= 80:
		return ActionProceed
	case score >= 60:
		return ActionProceedReduced
	case score >= 30:
		return ActionSkipCandle
	default:
		return ActionHalt
	}
}

// ValidateCrossSource returns false if two price quotes from different sources
// diverge by more than 0.5%, indicating a feed anomaly.
func (v *Validator) ValidateCrossSource(price1 float64, source1 string, price2 float64, source2 string) bool {
	if price1 <= 0 {
		return false
	}
	divergence := math.Abs(price1-price2) / price1
	ok := divergence <= 0.005
	if !ok {
		slog.Warn("[dataquality] cross-source divergence",
			"source1", source1, "price1", price1,
			"source2", source2, "price2", price2,
			"divergence_pct", divergence*100,
		)
	}
	return ok
}

// MarkSourceDegraded records that source is producing low-quality data.
func (v *Validator) MarkSourceDegraded(source string) {
	v.mu.Lock()
	v.sourceHealth[source] = SourceDegraded
	v.mu.Unlock()
	slog.Warn("[dataquality] source marked DEGRADED", "source", source)
}

// MarkSourceUnavailable records that source is completely offline.
func (v *Validator) MarkSourceUnavailable(source string) {
	v.mu.Lock()
	v.sourceHealth[source] = SourceUnavailable
	v.mu.Unlock()
	slog.Warn("[dataquality] source marked UNAVAILABLE", "source", source)
}

// GetSourceHealth returns a snapshot of per-source health states.
func (v *Validator) GetSourceHealth() map[string]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make(map[string]string, len(v.sourceHealth))
	for k, s := range v.sourceHealth {
		out[k] = string(s)
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

func severityRank(s Severity) int {
	switch s {
	case SeverityOK:
		return 0
	case SeverityMinor:
		return 1
	case SeverityModerate:
		return 2
	case SeveritySevere:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}

func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

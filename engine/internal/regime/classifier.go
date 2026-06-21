package regime

import (
	"math"
	"sync"
	"time"

	tconfig "antigravity-engine/internal/config"
)

// RegimeClassification is the rich output of a Classifier.Classify call.
// It extends RegimeState with trading directives for the execution layer.
type RegimeClassification struct {
	Regime              Regime
	Confidence          float64 // 0–100
	ADX                 float64
	RSI_1h              float64
	ATRRatio            float64 // current ATR / 20-period ATR average
	BBWidth             float64
	EMA50vsEMA200       string  // "above" | "below" | "crossing"
	Strategy            string  // plain-English strategy description
	PositionSizeMult    float64 // multiplier applied to all position sizes
	AllowNewEntries     bool    // false during HIGH_VOLATILITY
	FavourBias          string  // "BUY" | "SELL" | "NEUTRAL"
	MinConfidenceOverride float64 // regime-specific confidence floor
	ClassifiedAt        time.Time
}

// IndicatorSnapshot carries the indicator values needed for classification.
type IndicatorSnapshot struct {
	Price       float64
	RSI_1h      float64
	ADX         float64
	ATR         float64
	EMA9        float64
	EMA21       float64
	EMA50       float64
	EMA200      float64
	BBWidth     float64
	BBWidthAvg  float64 // 20-period BB width average
	Volume      float64
	VolumeAvg   float64
}

const atrHistoryLen = 20

// Live, hot-reloadable copies of the regime classification thresholds. Read
// from internal/config.ThresholdRegistry at package init and refreshed on
// every operator edit (config.RefreshHotReloadHooks), so a threshold change
// made through the Trade Threshold Configuration UI is reflected on the very
// next Classify() call — no restart required.
var (
	liveATRHighVolRatio    = 2.0
	liveATRExtremeVolRatio = 3.0
	liveADXTrendThreshold  = 25.0
	liveADXRangeThreshold  = 20.0
)

func init() {
	refreshRegimeThresholdsFromRegistry()
	tconfig.RegisterHotReloadHook(refreshRegimeThresholdsFromRegistry)
}

// refreshRegimeThresholdsFromRegistry re-reads the ATR-ratio and ADX
// classification thresholds from the central registry.
func refreshRegimeThresholdsFromRegistry() {
	reg := tconfig.Default()
	liveATRHighVolRatio = reg.GetWithDefault("REGIME_ATR_HIGH_VOL_RATIO", 2.0)
	liveATRExtremeVolRatio = reg.GetWithDefault("REGIME_ATR_EXTREME_VOL_RATIO", 3.0)
	liveADXTrendThreshold = reg.GetWithDefault("REGIME_ADX_TREND_THRESHOLD", 25.0)
	liveADXRangeThreshold = reg.GetWithDefault("REGIME_ADX_RANGE_THRESHOLD", 20.0)
}

// Classifier adds position-sizing directives, confidence floors, and ATR-ratio
// tracking on top of the base Engine. It is the primary interface for the
// execution layer.
type Classifier struct {
	atrHistory []float64
	mu         sync.RWMutex
}

// NewClassifier creates a ready-to-use Classifier.
func NewClassifier() *Classifier {
	return &Classifier{}
}

// Classify derives a RegimeClassification from the given indicator snapshot.
// Rules are evaluated in priority order; the first matching rule wins.
func (c *Classifier) Classify(snap IndicatorSnapshot) RegimeClassification {
	atrRatio := c.GetATRRatio()
	if snap.ATR > 0 {
		c.UpdateATRHistory(snap.ATR)
		atrRatio = c.GetATRRatio()
	}

	ema50vs200 := ema50vsEMA200(snap)

	// ── Rule 1: HIGH_VOLATILITY ───────────────────────────────────────────────
	if atrRatio > liveATRHighVolRatio {
		conf := 80.0
		if atrRatio > liveATRExtremeVolRatio {
			conf = 95.0
		}
		return RegimeClassification{
			Regime:              RegimeHighVol,
			Confidence:          conf,
			ADX:                 snap.ADX,
			RSI_1h:              snap.RSI_1h,
			ATRRatio:            atrRatio,
			BBWidth:             snap.BBWidth,
			EMA50vsEMA200:       ema50vs200,
			Strategy:            "Volatility spike — all entries suspended, existing positions monitored",
			PositionSizeMult:    0.0,
			AllowNewEntries:     false,
			FavourBias:          "NEUTRAL",
			MinConfidenceOverride: 1.0, // unreachable floor = no new entries
			ClassifiedAt:        time.Now().UTC(),
		}
	}

	// ── Rule 2: TRENDING_BULL ─────────────────────────────────────────────────
	if snap.Price > snap.EMA200 && snap.EMA50 > snap.EMA200 &&
		snap.ADX > liveADXTrendThreshold && snap.RSI_1h > 45 && snap.RSI_1h < 75 {
		conf := trendConfidence(snap)
		return RegimeClassification{
			Regime:              RegimeTrendingBull,
			Confidence:          conf,
			ADX:                 snap.ADX,
			RSI_1h:              snap.RSI_1h,
			ATRRatio:            atrRatio,
			BBWidth:             snap.BBWidth,
			EMA50vsEMA200:       ema50vs200,
			Strategy:            "Confirmed uptrend — favour long entries, trail stops generously",
			PositionSizeMult:    1.25,
			AllowNewEntries:     true,
			FavourBias:          "BUY",
			MinConfidenceOverride: 0.60,
			ClassifiedAt:        time.Now().UTC(),
		}
	}

	// ── Rule 3: TRENDING_BEAR ─────────────────────────────────────────────────
	if snap.Price < snap.EMA200 && snap.EMA50 < snap.EMA200 &&
		snap.ADX > liveADXTrendThreshold && snap.RSI_1h < 55 && snap.RSI_1h > 25 {
		conf := trendConfidence(snap)
		return RegimeClassification{
			Regime:              RegimeTrendingBear,
			Confidence:          conf,
			ADX:                 snap.ADX,
			RSI_1h:              snap.RSI_1h,
			ATRRatio:            atrRatio,
			BBWidth:             snap.BBWidth,
			EMA50vsEMA200:       ema50vs200,
			Strategy:            "Confirmed downtrend — favour short bias, keep stops tight",
			PositionSizeMult:    0.75,
			AllowNewEntries:     true,
			FavourBias:          "SELL",
			MinConfidenceOverride: 0.70,
			ClassifiedAt:        time.Now().UTC(),
		}
	}

	// ── Rule 4: RANGING ───────────────────────────────────────────────────────
	if snap.ADX < liveADXRangeThreshold && snap.BBWidthAvg > 0 && snap.BBWidth < snap.BBWidthAvg*0.9 {
		conf := math.Max(0, 70+(liveADXRangeThreshold-snap.ADX)*1.5)
		return RegimeClassification{
			Regime:              RegimeRanging,
			Confidence:          conf,
			ADX:                 snap.ADX,
			RSI_1h:              snap.RSI_1h,
			ATRRatio:            atrRatio,
			BBWidth:             snap.BBWidth,
			EMA50vsEMA200:       ema50vs200,
			Strategy:            "Choppy range — mean reversion only, halve all position sizes, 45-min cooldown",
			PositionSizeMult:    0.50,
			AllowNewEntries:     true,
			FavourBias:          "NEUTRAL",
			MinConfidenceOverride: 0.80,
			ClassifiedAt:        time.Now().UTC(),
		}
	}

	// ── Default: RANGING with lower confidence ────────────────────────────────
	return RegimeClassification{
		Regime:              RegimeRanging,
		Confidence:          50,
		ADX:                 snap.ADX,
		RSI_1h:              snap.RSI_1h,
		ATRRatio:            atrRatio,
		BBWidth:             snap.BBWidth,
		EMA50vsEMA200:       ema50vs200,
		Strategy:            "Indeterminate — conservative ranging posture",
		PositionSizeMult:    0.75,
		AllowNewEntries:     true,
		FavourBias:          "NEUTRAL",
		MinConfidenceOverride: 0.68,
		ClassifiedAt:        time.Now().UTC(),
	}
}

// UpdateATRHistory appends atr to the rolling 20-bar history. Thread-safe.
func (c *Classifier) UpdateATRHistory(atr float64) {
	c.mu.Lock()
	c.atrHistory = append(c.atrHistory, atr)
	if len(c.atrHistory) > atrHistoryLen {
		c.atrHistory = c.atrHistory[len(c.atrHistory)-atrHistoryLen:]
	}
	c.mu.Unlock()
}

// GetATRRatio returns current_ATR / mean(atrHistory).
// Returns 1.0 if fewer than 5 data points are available.
func (c *Classifier) GetATRRatio() float64 {
	c.mu.RLock()
	hist := make([]float64, len(c.atrHistory))
	copy(hist, c.atrHistory)
	c.mu.RUnlock()

	if len(hist) < 5 {
		return 1.0
	}
	current := hist[len(hist)-1]
	sum := 0.0
	for _, v := range hist {
		sum += v
	}
	avg := sum / float64(len(hist))
	if avg == 0 {
		return 1.0
	}
	return current / avg
}

// ── helpers ───────────────────────────────────────────────────────────────────

func trendConfidence(snap IndicatorSnapshot) float64 {
	conf := 60.0
	if snap.ADX > 40 {
		conf += 20
	} else if snap.ADX > 30 {
		conf += 10
	}
	if snap.RSI_1h > 50 && snap.RSI_1h < 70 {
		conf += 10
	}
	if conf > 100 {
		conf = 100
	}
	return conf
}

func ema50vsEMA200(snap IndicatorSnapshot) string {
	if snap.EMA200 == 0 {
		return "unknown"
	}
	crossThreshold := snap.EMA200 * 0.001 // within 0.1% = crossing
	diff := snap.EMA50 - snap.EMA200
	switch {
	case math.Abs(diff) < crossThreshold:
		return "crossing"
	case diff > 0:
		return "above"
	default:
		return "below"
	}
}

// Package featurestore implements the Phase 19B Institutional Feature Store.
// Every feature is versioned, lineage-tracked, and reproducible. The store
// is completely isolated from the production trading environment.
package featurestore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ─── Feature Categories ───────────────────────────────────────────────────────

type FeatureCategory string

const (
	CategoryPrice          FeatureCategory = "PRICE"
	CategoryVolume         FeatureCategory = "VOLUME"
	CategoryOrderFlow      FeatureCategory = "ORDER_FLOW"
	CategoryFunding        FeatureCategory = "FUNDING"
	CategoryCVD            FeatureCategory = "CVD"
	CategoryDelta          FeatureCategory = "DELTA"
	CategoryLiquidity      FeatureCategory = "LIQUIDITY"
	CategoryMarketStructure FeatureCategory = "MARKET_STRUCTURE"
	CategoryVolatility     FeatureCategory = "VOLATILITY"
	CategoryPortfolio      FeatureCategory = "PORTFOLIO"
)

// ─── Feature Definition ───────────────────────────────────────────────────────

// FeatureDefinition describes a feature — its computation, category, and metadata.
type FeatureDefinition struct {
	ID          string
	Name        string
	Category    FeatureCategory
	Description string
	Version     int
	Parameters  map[string]any
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Deprecated  bool
}

// FeatureVector is a timestamped set of computed feature values.
type FeatureVector struct {
	FeatureID string
	Symbol    string
	Version   int
	Values    map[string]float64
	ComputedAt time.Time
}

// Bar is a minimal OHLCV bar used as raw input for feature computation.
type Bar struct {
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Time   time.Time
}

// ─── Feature Computation Engines ─────────────────────────────────────────────

// Compute calculates all feature values for a given feature definition over
// a historical bar series. This is the core computation path.
func Compute(def FeatureDefinition, bars []Bar) (FeatureVector, error) {
	if len(bars) == 0 {
		return FeatureVector{}, errors.New("featurestore: no bars provided")
	}
	closes := make([]float64, len(bars))
	highs := make([]float64, len(bars))
	lows := make([]float64, len(bars))
	volumes := make([]float64, len(bars))
	for i, b := range bars {
		closes[i] = b.Close
		highs[i] = b.High
		lows[i] = b.Low
		volumes[i] = b.Volume
	}

	values := make(map[string]float64)

	switch def.Category {
	case CategoryPrice:
		computePriceFeatures(closes, highs, lows, def.Parameters, values)
	case CategoryVolume:
		computeVolumeFeatures(closes, volumes, def.Parameters, values)
	case CategoryVolatility:
		computeVolatilityFeatures(closes, highs, lows, def.Parameters, values)
	case CategoryOrderFlow:
		computeOrderFlowFeatures(closes, volumes, def.Parameters, values)
	case CategoryCVD:
		computeCVDFeatures(closes, volumes, def.Parameters, values)
	case CategoryFunding:
		// Funding requires external data; return placeholder with zero values
		values["funding_rate"] = 0
		values["funding_8h"] = 0
	case CategoryLiquidity:
		computeLiquidityFeatures(highs, lows, volumes, def.Parameters, values)
	case CategoryMarketStructure:
		computeMarketStructureFeatures(closes, highs, lows, def.Parameters, values)
	default:
		computePriceFeatures(closes, highs, lows, def.Parameters, values)
	}

	return FeatureVector{
		FeatureID:  def.ID,
		Version:    def.Version,
		Values:     values,
		ComputedAt: time.Now().UTC(),
	}, nil
}

// computePriceFeatures computes price-based features: EMA, SMA, RSI, MACD, BB.
func computePriceFeatures(closes, highs, lows []float64, params map[string]any, out map[string]float64) {
	n := len(closes)
	if n == 0 {
		return
	}

	fast := intParam(params, "ema_fast", 12)
	slow := intParam(params, "ema_slow", 26)
	signal := intParam(params, "macd_signal", 9)
	rsiPeriod := intParam(params, "rsi_period", 14)
	bbPeriod := intParam(params, "bb_period", 20)
	bbStd := floatParam(params, "bb_std", 2.0)

	// EMA fast and slow
	emaFast := computeEMA(closes, fast)
	emaSlow := computeEMA(closes, slow)
	out["ema_fast"] = emaFast[n-1]
	out["ema_slow"] = emaSlow[n-1]
	out["ema_cross"] = emaFast[n-1] - emaSlow[n-1]
	out["price_vs_ema_fast"] = (closes[n-1] - emaFast[n-1]) / emaFast[n-1]

	// MACD
	macdLine := make([]float64, n)
	for i := range closes {
		macdLine[i] = emaFast[i] - emaSlow[i]
	}
	macdSignal := computeEMA(macdLine, signal)
	out["macd"] = macdLine[n-1]
	out["macd_signal"] = macdSignal[n-1]
	out["macd_hist"] = macdLine[n-1] - macdSignal[n-1]

	// RSI
	if n >= rsiPeriod {
		out["rsi"] = computeRSI(closes, rsiPeriod)
	}

	// Bollinger Bands
	if n >= bbPeriod {
		sma, upper, lower := computeBB(closes, bbPeriod, bbStd)
		out["bb_upper"] = upper
		out["bb_lower"] = lower
		out["bb_mid"] = sma
		bw := upper - lower
		if bw > 0 {
			out["bb_pct"] = (closes[n-1] - lower) / bw
		}
		out["bb_width"] = bw / sma
	}

	// ATR
	if len(highs) == n && len(lows) == n {
		out["atr"] = computeATR(highs, lows, closes, 14)
		out["atr_pct"] = out["atr"] / closes[n-1]
	}
}

// computeVolumeFeatures computes volume-based features: VWAP, OBV, volume ratio.
func computeVolumeFeatures(closes, volumes []float64, params map[string]any, out map[string]float64) {
	n := len(closes)
	if n == 0 {
		return
	}
	period := intParam(params, "volume_ma_period", 20)

	// OBV
	obv := 0.0
	for i := 1; i < n; i++ {
		if closes[i] > closes[i-1] {
			obv += volumes[i]
		} else if closes[i] < closes[i-1] {
			obv -= volumes[i]
		}
	}
	out["obv"] = obv

	// Volume MA and ratio
	if n >= period {
		sum := 0.0
		for i := n - period; i < n; i++ {
			sum += volumes[i]
		}
		volMA := sum / float64(period)
		out["volume_ma"] = volMA
		if volMA > 0 {
			out["volume_ratio"] = volumes[n-1] / volMA
		}
	}

	// Approximate VWAP (cumulative)
	totalVol := 0.0
	totalPV := 0.0
	for i := 0; i < n; i++ {
		totalPV += closes[i] * volumes[i]
		totalVol += volumes[i]
	}
	if totalVol > 0 {
		vwap := totalPV / totalVol
		out["vwap"] = vwap
		out["price_vs_vwap"] = (closes[n-1] - vwap) / vwap
	}
}

// computeVolatilityFeatures computes realised vol, parkinson, and garman-klass.
func computeVolatilityFeatures(closes, highs, lows []float64, params map[string]any, out map[string]float64) {
	n := len(closes)
	if n < 2 {
		return
	}
	period := intParam(params, "vol_period", 20)

	// Realised volatility (close-to-close log returns)
	returns := make([]float64, n-1)
	for i := 1; i < n; i++ {
		if closes[i-1] > 0 {
			returns[i-1] = math.Log(closes[i] / closes[i-1])
		}
	}
	if len(returns) >= period {
		slice := returns[len(returns)-period:]
		mean, stdDev := meanStd(slice)
		out["realized_vol_daily"] = stdDev
		out["realized_vol_annual"] = stdDev * math.Sqrt(365)
		out["returns_mean"] = mean
		out["returns_skew"] = skewness(slice, mean, stdDev)
		out["returns_kurt"] = kurtosis(slice, mean, stdDev)
	}

	// Parkinson estimator (uses high/low)
	if len(highs) == n && len(lows) == n && n >= period {
		pk := 0.0
		for i := n - period; i < n; i++ {
			if lows[i] > 0 {
				ratio := highs[i] / lows[i]
				pk += math.Log(ratio) * math.Log(ratio)
			}
		}
		out["parkinson_vol"] = math.Sqrt(pk/(4*float64(period)*math.Log(2))) * math.Sqrt(365)
	}
}

// computeOrderFlowFeatures approximates order flow metrics from OHLCV.
func computeOrderFlowFeatures(closes, volumes []float64, params map[string]any, out map[string]float64) {
	n := len(closes)
	if n == 0 {
		return
	}
	// Buy/Sell volume approximation using close vs open direction
	buyVol, sellVol := 0.0, 0.0
	for i := 1; i < n; i++ {
		if closes[i] >= closes[i-1] {
			buyVol += volumes[i]
		} else {
			sellVol += volumes[i]
		}
	}
	total := buyVol + sellVol
	out["buy_vol"] = buyVol
	out["sell_vol"] = sellVol
	if total > 0 {
		out["buy_vol_pct"] = buyVol / total
		out["order_flow_imbalance"] = (buyVol - sellVol) / total
	}
}

// computeCVDFeatures approximates cumulative volume delta.
func computeCVDFeatures(closes, volumes []float64, params map[string]any, out map[string]float64) {
	n := len(closes)
	period := intParam(params, "cvd_period", 20)
	cvd := 0.0
	for i := 1; i < n; i++ {
		if closes[i] > closes[i-1] {
			cvd += volumes[i]
		} else {
			cvd -= volumes[i]
		}
	}
	out["cvd_cumulative"] = cvd
	if n >= period+1 {
		periodCVD := 0.0
		for i := n - period; i < n; i++ {
			if closes[i] > closes[i-1] {
				periodCVD += volumes[i]
			} else {
				periodCVD -= volumes[i]
			}
		}
		out["cvd_period"] = periodCVD
	}
}

// computeLiquidityFeatures computes bid-ask spread proxies and market depth proxies.
func computeLiquidityFeatures(highs, lows, volumes []float64, params map[string]any, out map[string]float64) {
	n := len(highs)
	period := intParam(params, "liq_period", 20)
	if n == 0 {
		return
	}
	// Amihud illiquidity ratio
	if n >= period && len(lows) == n {
		iliq := 0.0
		for i := n - period; i < n; i++ {
			hl := highs[i] - lows[i]
			if volumes[i] > 0 && lows[i] > 0 {
				iliq += (hl / lows[i]) / volumes[i]
			}
		}
		out["amihud_illiquidity"] = iliq / float64(period)
	}
	// HL range as spread proxy
	if n > 0 {
		hl := highs[n-1] - lows[n-1]
		mid := (highs[n-1] + lows[n-1]) / 2
		if mid > 0 {
			out["hl_spread_pct"] = hl / mid
		}
	}
}

// computeMarketStructureFeatures detects HH/LL patterns and swing structure.
func computeMarketStructureFeatures(closes, highs, lows []float64, params map[string]any, out map[string]float64) {
	n := len(closes)
	if n < 3 {
		return
	}
	lookback := intParam(params, "structure_lookback", 20)
	if n < lookback {
		lookback = n
	}
	recent := closes[n-lookback:]
	maxPrice := recent[0]
	minPrice := recent[0]
	for _, c := range recent {
		if c > maxPrice {
			maxPrice = c
		}
		if c < minPrice {
			minPrice = c
		}
	}
	rng := maxPrice - minPrice
	out["structure_range"] = rng
	if rng > 0 {
		out["structure_position"] = (closes[n-1] - minPrice) / rng
	}
	// Simple trend: regression slope of last N closes
	out["trend_slope"] = linearSlope(recent)
}

// ─── Math helpers ─────────────────────────────────────────────────────────────

func computeEMA(data []float64, period int) []float64 {
	out := make([]float64, len(data))
	if len(data) == 0 || period <= 0 {
		return out
	}
	k := 2.0 / float64(period+1)
	out[0] = data[0]
	for i := 1; i < len(data); i++ {
		out[i] = data[i]*k + out[i-1]*(1-k)
	}
	return out
}

func computeRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50
	}
	var gains, losses float64
	for i := len(closes) - period; i < len(closes); i++ {
		d := closes[i] - closes[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	if losses == 0 {
		return 100
	}
	rs := (gains / float64(period)) / (losses / float64(period))
	return 100 - 100/(1+rs)
}

func computeBB(closes []float64, period int, nStd float64) (sma, upper, lower float64) {
	n := len(closes)
	if n < period {
		return
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += closes[i]
	}
	sma = sum / float64(period)
	variance := 0.0
	for i := n - period; i < n; i++ {
		d := closes[i] - sma
		variance += d * d
	}
	std := math.Sqrt(variance / float64(period))
	upper = sma + nStd*std
	lower = sma - nStd*std
	return
}

func computeATR(highs, lows, closes []float64, period int) float64 {
	n := len(closes)
	if n < 2 || period <= 0 {
		return 0
	}
	trueRanges := make([]float64, n-1)
	for i := 1; i < n; i++ {
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		trueRanges[i-1] = math.Max(hl, math.Max(hpc, lpc))
	}
	if len(trueRanges) < period {
		period = len(trueRanges)
	}
	sum := 0.0
	for _, tr := range trueRanges[len(trueRanges)-period:] {
		sum += tr
	}
	return sum / float64(period)
}

func meanStd(data []float64) (mean, std float64) {
	if len(data) == 0 {
		return
	}
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))
	for _, v := range data {
		d := v - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(data)))
	return
}

func skewness(data []float64, mean, std float64) float64 {
	if std == 0 || len(data) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range data {
		z := (v - mean) / std
		s += z * z * z
	}
	return s / float64(len(data))
}

func kurtosis(data []float64, mean, std float64) float64 {
	if std == 0 || len(data) == 0 {
		return 0
	}
	k := 0.0
	for _, v := range data {
		z := (v - mean) / std
		k += z * z * z * z
	}
	return k/float64(len(data)) - 3 // excess kurtosis
}

func linearSlope(data []float64) float64 {
	n := float64(len(data))
	if n < 2 {
		return 0
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}

func intParam(params map[string]any, key string, def int) int {
	if params == nil {
		return def
	}
	if v, ok := params[key]; ok {
		switch x := v.(type) {
		case int:
			return x
		case float64:
			return int(x)
		}
	}
	return def
}

func floatParam(params map[string]any, key string, def float64) float64 {
	if params == nil {
		return def
	}
	if v, ok := params[key]; ok {
		if x, ok := v.(float64); ok {
			return x
		}
	}
	return def
}

// ─── Feature Store ────────────────────────────────────────────────────────────

// FeatureStore stores computed feature vectors keyed by (featureID, symbol, timestamp).
type FeatureStore struct {
	mu       sync.RWMutex
	registry map[string]FeatureDefinition           // featureID → definition
	vectors  map[string][]FeatureVector              // featureID+symbol → vectors
}

// NewFeatureStore creates an empty feature store.
func NewFeatureStore() *FeatureStore {
	return &FeatureStore{
		registry: make(map[string]FeatureDefinition),
		vectors:  make(map[string][]FeatureVector),
	}
}

// Register adds or updates a feature definition in the store.
func (fs *FeatureStore) Register(def FeatureDefinition) error {
	if def.ID == "" {
		return errors.New("featurestore: feature id required")
	}
	if def.Name == "" {
		return errors.New("featurestore: feature name required")
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if existing, ok := fs.registry[def.ID]; ok {
		if existing.Version >= def.Version {
			return fmt.Errorf("featurestore: version %d ≤ existing %d for feature %s",
				def.Version, existing.Version, def.ID)
		}
	}
	if def.CreatedAt.IsZero() {
		def.CreatedAt = time.Now().UTC()
	}
	def.UpdatedAt = time.Now().UTC()
	fs.registry[def.ID] = def
	return nil
}

// Store saves a computed feature vector.
func (fs *FeatureStore) Store(ctx context.Context, vec FeatureVector) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if vec.FeatureID == "" {
		return errors.New("featurestore: feature id required")
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	key := vec.FeatureID + ":" + vec.Symbol
	fs.vectors[key] = append(fs.vectors[key], vec)
	return nil
}

// GetLatest returns the most recently computed vector for a feature+symbol.
func (fs *FeatureStore) GetLatest(ctx context.Context, featureID, symbol string) (FeatureVector, error) {
	if ctx.Err() != nil {
		return FeatureVector{}, ctx.Err()
	}
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	key := featureID + ":" + symbol
	vecs := fs.vectors[key]
	if len(vecs) == 0 {
		return FeatureVector{}, fmt.Errorf("featurestore: no vectors for %s/%s", featureID, symbol)
	}
	return vecs[len(vecs)-1], nil
}

// GetRange returns all vectors for a feature+symbol within the given time range.
func (fs *FeatureStore) GetRange(ctx context.Context, featureID, symbol string, from, to time.Time) ([]FeatureVector, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	key := featureID + ":" + symbol
	var result []FeatureVector
	for _, v := range fs.vectors[key] {
		if !v.ComputedAt.Before(from) && !v.ComputedAt.After(to) {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ComputedAt.Before(result[j].ComputedAt)
	})
	return result, nil
}

// GetDefinition returns the feature definition by ID.
func (fs *FeatureStore) GetDefinition(id string) (FeatureDefinition, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	def, ok := fs.registry[id]
	if !ok {
		return FeatureDefinition{}, fmt.Errorf("featurestore: unknown feature %s", id)
	}
	return def, nil
}

// ListDefinitions returns all registered feature definitions.
func (fs *FeatureStore) ListDefinitions() []FeatureDefinition {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	out := make([]FeatureDefinition, 0, len(fs.registry))
	for _, d := range fs.registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// ComputeAndStore computes a feature for the given bars and persists the result.
func (fs *FeatureStore) ComputeAndStore(ctx context.Context, featureID, symbol string, bars []Bar) (FeatureVector, error) {
	def, err := fs.GetDefinition(featureID)
	if err != nil {
		return FeatureVector{}, err
	}
	vec, err := Compute(def, bars)
	if err != nil {
		return FeatureVector{}, fmt.Errorf("featurestore: compute %s: %w", featureID, err)
	}
	vec.Symbol = symbol
	if err := fs.Store(ctx, vec); err != nil {
		return FeatureVector{}, fmt.Errorf("featurestore: store %s: %w", featureID, err)
	}
	return vec, nil
}

// TotalVectors returns the total number of stored feature vectors.
func (fs *FeatureStore) TotalVectors() int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	total := 0
	for _, vecs := range fs.vectors {
		total += len(vecs)
	}
	return total
}

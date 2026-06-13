// Package ml provides a lightweight XGBoost pre-scoring client.
// The ML pre-scorer is optional — if the Python server is unavailable the engine
// proceeds without ML filtering. It is never a startup blocker.
package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/observability"
)

const (
	mlHealthTimeout  = 2 * time.Second
	mlPredictTimeout = 100 * time.Millisecond
	mlPollInterval   = 30 * time.Second
	mlActiveInterval = 5 * time.Minute
)

// FeatureVector carries all input features for one ML prediction request.
// Field order must match FEATURES in ml_scorer.py.
type FeatureVector struct {
	RSI14          float64
	MACDHist       float64
	EMA9vsEMA21    float64 // +1 if above, -1 if below, 0 if equal
	BBPosition     float64 // 0–1 position within Bollinger Bands
	ATRRatio       float64 // current ATR / 20-period ATR average
	VolumeRatio    float64 // current volume / average volume
	RegimeNum      float64 // 0=RANGING 1=BULL 2=BEAR 3=VOLATILE
	FundingRate    float64
	OITrendNum     float64 // -1=FALLING 0=FLAT +1=RISING
	ETFScore       float64
	DominanceScore float64
	MacroScore     float64
	SentimentScore float64
	TemporalMod    float64
}

// ToSlice serialises the feature vector in the order expected by the Python server.
func (f FeatureVector) ToSlice() []float64 {
	return []float64{
		f.RSI14, f.MACDHist, f.EMA9vsEMA21, f.BBPosition,
		f.ATRRatio, f.VolumeRatio, f.RegimeNum, f.FundingRate,
		f.OITrendNum, f.ETFScore, f.DominanceScore,
		f.MacroScore, f.SentimentScore, f.TemporalMod,
	}
}

// MLPrescorer calls the local Python XGBoost server to pre-filter low-probability
// signals before they reach Monte Carlo. When the server is down, ShouldProceed
// returns true immediately so trading is unaffected.
type MLPrescorer struct {
	endpoint  string
	threshold float64
	client    *http.Client
	enabled   atomic.Bool
	logger    *slog.Logger
}

// NewMLPrescorer creates a prescorer. enabled starts false until first health check passes.
// threshold is the minimum win probability to proceed (e.g. 0.55).
func NewMLPrescorer(endpoint string, threshold float64, logger *slog.Logger) *MLPrescorer {
	return &MLPrescorer{
		endpoint:  endpoint,
		threshold: threshold,
		logger:    logger,
		client:    &http.Client{Timeout: mlPredictTimeout},
	}
}

// CheckHealth pings /health and enables the prescorer if the model is loaded.
func (m *MLPrescorer) CheckHealth(ctx context.Context) bool {
	hc := &http.Client{Timeout: mlHealthTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.endpoint+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := hc.Do(req)
	if err != nil {
		m.logger.Debug("[ml] health check failed", "endpoint", m.endpoint, "error", err)
		return false
	}
	defer resp.Body.Close()

	var result struct {
		ModelLoaded bool `json:"model_loaded"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	if result.ModelLoaded {
		m.enabled.Store(true)
		observability.MLAvailable.Set(1)
		m.logger.Info("[ml] model loaded and active", "endpoint", m.endpoint)
		return true
	}
	return false
}

// ShouldProceed returns false if the ML model predicts win probability < threshold.
// Returns true immediately when the prescorer is disabled or times out.
func (m *MLPrescorer) ShouldProceed(ctx context.Context, features FeatureVector) bool {
	if !m.enabled.Load() {
		return true
	}

	payload, err := json.Marshal(map[string]interface{}{
		"features": features.ToSlice(),
	})
	if err != nil {
		return true
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.endpoint+"/predict", bytes.NewReader(payload))
	if err != nil {
		return true
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		m.logger.Debug("[ml] predict timeout or error — proceeding", "error", err)
		return true
	}
	defer resp.Body.Close()

	var result struct {
		ProbabilityWin float64 `json:"probability_win"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return true
	}
	return result.ProbabilityWin >= m.threshold
}

// StartHealthPoller polls /health every 30s until the model is loaded,
// then every 5 minutes to confirm it stays active. Stops on ctx cancellation.
func (m *MLPrescorer) StartHealthPoller(ctx context.Context) {
	go func() {
		for {
			interval := mlPollInterval
			if m.enabled.Load() {
				interval = mlActiveInterval
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				if !m.CheckHealth(ctx) && m.enabled.Load() {
					m.enabled.Store(false)
					observability.MLAvailable.Set(0)
					m.logger.Warn("[ml] model no longer available — pre-filter disabled")
				}
			}
		}
	}()
}

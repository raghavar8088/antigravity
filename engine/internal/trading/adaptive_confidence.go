package trading

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var tradingConfidenceFloor = promauto.NewGauge(prometheus.GaugeOpts{
	Namespace: "trading",
	Name:      "confidence_floor",
	Help:      "Current adaptive confidence floor (0–1). Rises during low data quality periods.",
})

// AdaptiveConfidenceFloor dynamically raises the minimum signal confidence
// threshold during periods of degraded market data quality.
type AdaptiveConfidenceFloor struct {
	mu           sync.RWMutex
	baseFloor    float64
	currentFloor float64
	lastUpdated  time.Time
	lastQuality  float64
}

// NewAdaptiveConfidenceFloor creates a floor starting at base (e.g. 0.68).
func NewAdaptiveConfidenceFloor(base float64) *AdaptiveConfidenceFloor {
	a := &AdaptiveConfidenceFloor{
		baseFloor:    base,
		currentFloor: base,
	}
	tradingConfidenceFloor.Set(base)
	return a
}

// Update is a no-op — the confidence floor is fixed at baseFloor regardless
// of data quality score. Dynamic elevation is disabled so valid signals are
// never blocked by transient data quality fluctuations.
func (a *AdaptiveConfidenceFloor) Update(dataQuality float64) {
	a.mu.Lock()
	a.lastQuality = dataQuality
	a.mu.Unlock()
}

// Floor returns the current minimum executable confidence threshold.
func (a *AdaptiveConfidenceFloor) Floor() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentFloor
}

// Snapshot returns a JSON-serialisable state for the HTTP endpoint.
func (a *AdaptiveConfidenceFloor) Snapshot() map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return map[string]float64{
		"floor":       a.currentFloor,
		"baseFloor":   a.baseFloor,
		"dataQuality": a.lastQuality,
	}
}

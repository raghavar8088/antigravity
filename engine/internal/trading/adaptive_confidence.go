package trading

import (
	"log"
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

// Update recalculates the floor based on a 0.0–1.0 data quality score.
// Call this every time a candle is validated by the data quality engine.
func (a *AdaptiveConfidenceFloor) Update(dataQuality float64) {
	var newFloor float64
	switch {
	case dataQuality >= 0.90:
		newFloor = a.baseFloor
	case dataQuality >= 0.75:
		newFloor = 0.72
	case dataQuality >= 0.60:
		newFloor = 0.78
	case dataQuality >= 0.40:
		newFloor = 0.90
	default:
		newFloor = 1.01 // effective lockout — no trades pass
	}

	// Never go below the base floor.
	if newFloor < a.baseFloor {
		newFloor = a.baseFloor
	}

	a.mu.Lock()
	old := a.currentFloor
	a.currentFloor = newFloor
	a.lastUpdated = time.Now()
	a.lastQuality = dataQuality
	a.mu.Unlock()

	if old != newFloor {
		log.Printf("[CONFIDENCE] Floor adjusted %.2f → %.2f (data quality=%.2f)", old, newFloor, dataQuality)
	}
	tradingConfidenceFloor.Set(newFloor)
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

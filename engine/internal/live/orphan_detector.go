package live

import (
	"sync"
	"time"
)

// trackedOrder records an order that has been submitted but not yet confirmed closed.
type trackedOrder struct {
	ClientOrderID string
	StrategyID    string
	Symbol        string
	OpenedAt      time.Time
}

// OrphanReport describes a detected orphan order — one that has been open longer
// than the expected maximum window.
type OrphanReport struct {
	ClientOrderID string
	StrategyID    string
	Symbol        string
	OpenedAt      time.Time
	Age           time.Duration
	Severity      string // WARNING | CRITICAL
}

// OrphanDetector tracks in-flight orders and flags those never closed within OrphanAfter.
// It detects orphan orders, ghost positions, and runaway strategies.
type OrphanDetector struct {
	// OrphanAfter is the duration after which an open order is considered orphaned.
	// Exported to allow tests to shorten the window.
	OrphanAfter time.Duration

	mu   sync.RWMutex
	open map[string]trackedOrder
}

func NewOrphanDetector() *OrphanDetector {
	return &OrphanDetector{
		OrphanAfter: 5 * time.Minute,
		open:        make(map[string]trackedOrder),
	}
}

// TrackOpen registers an order as open. Call immediately after Submit returns without error.
func (od *OrphanDetector) TrackOpen(clientOrderID, strategyID, symbol string) {
	od.mu.Lock()
	defer od.mu.Unlock()
	od.open[clientOrderID] = trackedOrder{
		ClientOrderID: clientOrderID,
		StrategyID:    strategyID,
		Symbol:        symbol,
		OpenedAt:      time.Now().UTC(),
	}
}

// TrackClose marks an order as successfully closed. Call on full fill or confirmed cancel.
func (od *OrphanDetector) TrackClose(clientOrderID string) {
	od.mu.Lock()
	defer od.mu.Unlock()
	delete(od.open, clientOrderID)
}

// DetectOrphans returns all orders that have exceeded the OrphanAfter threshold.
// Severity is WARNING at 1× the threshold and CRITICAL at 3×.
func (od *OrphanDetector) DetectOrphans() []OrphanReport {
	od.mu.RLock()
	defer od.mu.RUnlock()
	now := time.Now().UTC()
	var orphans []OrphanReport
	for _, o := range od.open {
		age := now.Sub(o.OpenedAt)
		if age < od.OrphanAfter {
			continue
		}
		severity := "WARNING"
		if age >= 3*od.OrphanAfter {
			severity = "CRITICAL"
		}
		orphans = append(orphans, OrphanReport{
			ClientOrderID: o.ClientOrderID,
			StrategyID:    o.StrategyID,
			Symbol:        o.Symbol,
			OpenedAt:      o.OpenedAt,
			Age:           age,
			Severity:      severity,
		})
	}
	return orphans
}

// OpenCount returns the number of currently tracked open orders.
func (od *OrphanDetector) OpenCount() int {
	od.mu.RLock()
	defer od.mu.RUnlock()
	return len(od.open)
}

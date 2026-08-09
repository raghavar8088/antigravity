package delta

import (
	"log"
	"sort"
	"strings"
)

// SetStrategyEnabled switches one strategy on or off for the live desk.
//
// Reversible and immediate: an OFF strategy places no new orders from the next
// signal onward. Positions it already holds are deliberately left alone — the
// switch governs ENTRY, not custody. Closing a live position is a separate,
// louder action (close-all), and having a toggle silently market-close real
// positions would make it far more dangerous than it looks.
func (b *PerpBridge) SetStrategyEnabled(name string, enabled bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.strategyOff == nil {
		b.strategyOff = make(map[string]bool)
	}
	if enabled {
		delete(b.strategyOff, name)
	} else {
		b.strategyOff[name] = true
	}
	b.persistLocked()
	log.Printf("[PERP LIVE] strategy %s switched %s by owner", name, map[bool]string{true: "ON", false: "OFF"}[enabled])
}

// StrategyEnabled reports whether a strategy may open new live positions.
//
// Unknown names are enabled: see strategyOff on the Bridge.
func (b *PerpBridge) StrategyEnabled(name string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return !b.strategyOff[strings.TrimSpace(name)]
}

// DisabledStrategies lists the switched-off strategies, sorted.
func (b *PerpBridge) DisabledStrategies() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, 0, len(b.strategyOff))
	for n := range b.strategyOff {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// setDisabledStrategies restores the switch state from disk. Caller holds b.mu.
func (b *PerpBridge) setDisabledStrategiesLocked(names []string) {
	b.strategyOff = make(map[string]bool, len(names))
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			b.strategyOff[n] = true
		}
	}
}

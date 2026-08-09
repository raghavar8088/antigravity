package delta

import (
	"log"
	"sort"
	"strings"
)

// SetStrategyEnabled switches one STREAM on or off for the live desk.
//
// Keyed on (strategy, symbol) rather than the strategy name, because that is
// what the desk actually trades: ANTI_Recurrence_Quantification_Signal runs on
// COOKIEUSD, MUBARAKUSD and BLESSUSD as three independent positions. Switching
// it off because one instrument misbehaved should not silently stop the other
// two, and the usual reason to stop a stream is the instrument, not the logic.
//
// Reversible and immediate: an OFF strategy places no new orders from the next
// signal onward. Positions it already holds are deliberately left alone — the
// switch governs ENTRY, not custody. Closing a live position is a separate,
// louder action (close-all), and having a toggle silently market-close real
// positions would make it far more dangerous than it looks.
func (b *PerpBridge) SetStrategyEnabled(strategy, symbol string, enabled bool) {
	strategy = strings.TrimSpace(strategy)
	symbol = strings.TrimSpace(symbol)
	if strategy == "" || symbol == "" {
		return
	}
	name := perpStreamKey(strategy, symbol)
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
	log.Printf("[PERP LIVE] %s on %s switched %s by owner", strategy, strings.ToUpper(symbol),
		map[bool]string{true: "ON", false: "OFF"}[enabled])
}

// StrategyEnabled reports whether a stream may open new live positions.
//
// Unknown names are enabled: see strategyOff on the Bridge.
func (b *PerpBridge) StrategyEnabled(strategy, symbol string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return !b.strategyOff[perpStreamKey(strategy, symbol)]
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

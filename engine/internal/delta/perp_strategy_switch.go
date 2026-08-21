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
//
// Entries for streams that are no longer on the ALLOW-LIST are dropped.
//
// The switch is a standing owner decision and it survives a roster change,
// which is right while a stream is still routed and wrong once it is not: the
// file had accumulated 183 entries naming streams from rosters retired weeks
// earlier. They could never trade — the bridge consults the allow-list, not
// this map — so every one was a preference about nothing, and they were the
// reason the board could report more switched-off strategies than the desk has
// ever run.
//
// Dropping rather than keeping is the safe direction here BECAUSE the entries
// are all "off". Losing an off switch for a stream that is not routed changes
// nothing; keeping it makes the board describe a roster that no longer exists.
// A stream that is re-routed later starts enabled, which is the documented
// default for an unknown name.
func (b *PerpBridge) setDisabledStrategiesLocked(names []string) {
	b.strategyOff = make(map[string]bool, len(names))
	dropped := 0
	for _, n := range names {
		if n = strings.TrimSpace(n); n == "" {
			continue
		}
		// The stored key is "strategy|SYMBOL" — the same shape perpStreamKey
		// writes — so it is split and put through the real gate rather than
		// matched against a second, parallel notion of what is routed.
		if b.allow != nil {
			parts := strings.SplitN(n, "|", 2)
			if len(parts) != 2 || !b.allow.Allowed(parts[0], parts[1]) {
				dropped++
				continue
			}
		}
		b.strategyOff[n] = true
	}
	if dropped > 0 {
		log.Printf("[PERP LIVE] dropped %d switched-off entr(ies) for streams no longer on the roster", dropped)
	}
}

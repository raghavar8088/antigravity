package delta

import "log"

// EnableVolatilityStops makes the desk size its stop from measured per-symbol
// noise instead of the strategy's fixed fraction.
//
// Explicit rather than automatic, because it changes what the strategies ARE.
// A strategy whose stop moves from 0.6% to 4% is a different strategy with the
// same name: far fewer trades, far longer holds, and a record that cannot be
// pooled with what came before it.
func (b *PerpBridge) EnableVolatilityStops(v *VolatilityTracker) {
	if v == nil {
		return
	}
	b.mu.Lock()
	b.vol = v
	b.mu.Unlock()
	log.Printf("[PERP LIVE] volatility-scaled stops ON: stop = %.0fx the p90 one-minute range, per symbol, "+
		"reward:risk preserved, time stop scaled with it", volStopMultiple)
}

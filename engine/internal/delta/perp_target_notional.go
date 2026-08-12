package delta

import "log"

// SetTargetNotionalUSD sizes every order to roughly this position value.
//
// Explicit, because it changes what a "1 contract" record means. Results
// gathered before and after are not comparable in currency terms — only the
// ratios are, which is why the leaderboard reports return per trade and profit
// on deployed capital rather than rupees.
func (b *PerpBridge) SetTargetNotionalUSD(v float64) {
	if v <= 0 {
		return
	}
	b.mu.Lock()
	b.cfg.TargetNotionalUSD = v
	ceiling := b.cfg.EquityUSD * b.cfg.MaxAggregateLeverage
	slots := b.cfg.MaxConcurrentPositions
	b.mu.Unlock()

	log.Printf("[PERP LIVE] target position size $%.2f — %d concurrent needs $%.2f against a $%.2f book ceiling%s",
		v, slots, float64(slots)*v, ceiling,
		map[bool]string{true: "", false: " (the cap will refuse the last few)"}[float64(slots)*v <= ceiling])
}

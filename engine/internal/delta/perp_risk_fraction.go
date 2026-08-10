package delta

import "log"

// SetRiskPerTradeFraction changes the share of equity risked per trade.
//
// Exists because the default interacts badly with the aggregate cap: at 2% risk
// against a ~0.64% stop, one position is sized near 3.1x equity — the whole 3x
// book — so the first signal consumes everything and the remaining concurrency
// slots get nothing worth opening.
//
// Sizing nearer 1x equity lets all three slots fill. That matters when the
// purpose of trading is to collect evidence across the roster rather than to
// win on one stream: three streams reporting is three times the information for
// the same money at risk.
func (b *PerpBridge) SetRiskPerTradeFraction(f float64) {
	if f <= 0 || f >= 1 {
		return
	}
	b.mu.Lock()
	b.cfg.RiskPerTradeFraction = f
	equity := b.cfg.EquityUSD
	b.mu.Unlock()
	log.Printf("[PERP LIVE] risk per trade set to %.3f%% of equity ($%.4f on $%.2f)", f*100, equity*f, equity)
}

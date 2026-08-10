package delta

import "log"

// SetMaxConcurrentPositions changes how many live positions may be open at once.
//
// Raising it is safe in fixed-size mode and not otherwise: the cap is the only
// thing bounding total exposure once per-position notional is risk-derived, so
// this is deliberately explicit rather than something the desk tunes itself.
//
// The per-symbol cap still applies underneath, so the practical ceiling is the
// number of distinct symbols on the roster however high this is set.
func (b *PerpBridge) SetMaxConcurrentPositions(n int) {
	if n <= 0 {
		return
	}
	b.mu.Lock()
	b.cfg.MaxConcurrentPositions = n
	fixed := b.cfg.FixedContracts
	b.mu.Unlock()
	if fixed <= 0 {
		log.Printf("[PERP LIVE] WARNING: max concurrent positions raised to %d while sizing from RISK — "+
			"total exposure is now up to %d full-size positions", n, n)
	}
	log.Printf("[PERP LIVE] max concurrent positions set to %d", n)
}

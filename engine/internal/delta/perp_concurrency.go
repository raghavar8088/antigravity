package delta

import "log"

// SetMaxPositionsPerSymbol changes how many live positions may share one
// instrument.
//
// The default of 1 exists because unintended concentration is invisible until
// it costs money: three strategies opened the same COOKIEUSD short inside 13
// minutes and all three stopped out, which was one position in three pieces
// wearing three different names.
//
// But that cap counts POSITIONS, and the count is only a proxy for the risk.
// When the operator deliberately rosters several streams onto ONE symbol, the
// concentration IS the choice being made, and a cap of 1 stops being a risk
// control: it becomes a rule under which the first stream to signal trades and
// every other stream on that symbol is refused, permanently. That failure has
// already been measured on this desk — 21 of 31 streams took no fills at all
// because faster streams held the slots — and on a single-symbol roster it is
// not a bias but a total block on all but one stream.
//
// So the count cap is raised to fit the roster, and the AGGREGATE NOTIONAL cap
// is left to do the actual bounding, that being the control which corresponds
// to what can actually be lost.
func (b *PerpBridge) SetMaxPositionsPerSymbol(n int) {
	if n <= 0 {
		return
	}
	b.mu.Lock()
	b.cfg.MaxPositionsPerSymbol = n
	b.mu.Unlock()
	if n > 1 {
		log.Printf("[PERP LIVE] WARNING: up to %d positions may now share ONE symbol — "+
			"same-symbol same-direction streams are one bet in %d pieces, not %d independent bets; "+
			"aggregate notional is what bounds the loss", n, n, n)
	}
	log.Printf("[PERP LIVE] max positions per symbol set to %d", n)
}

// SetMaxAggregateLeverage changes the ceiling on notional across ALL open
// positions at once.
//
// This is the control that actually binds once the count caps are raised, so it
// is set explicitly rather than left at a default sized for a different roster.
func (b *PerpBridge) SetMaxAggregateLeverage(x float64) {
	if x <= 0 {
		return
	}
	b.mu.Lock()
	b.cfg.MaxAggregateLeverage = x
	equity := b.cfg.EquityUSD
	lev := b.cfg.LeverageForOrder
	b.mu.Unlock()
	margin := 0.0
	if lev > 0 {
		margin = equity * x / float64(lev)
	}
	log.Printf("[PERP LIVE] aggregate leverage cap %.2fx — $%.2f notional on $%.2f equity, "+
		"about $%.2f held as margin at %dx per order", x, equity*x, equity, margin, lev)
}

// SetMaxConcurrentPositions changes how many live positions may be open at once.
//
// Raising it is safe in fixed-size mode and not otherwise: the cap is the only
// thing bounding total exposure once per-position notional is risk-derived, so
// this is deliberately explicit rather than something the desk tunes itself.
//
// The per-symbol cap still applies underneath, so the practical ceiling is the
// number of distinct symbols on the roster however high this is set — unless
// that cap has itself been raised, which is what SetMaxPositionsPerSymbol is
// for on a roster that concentrates several streams on one instrument.
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

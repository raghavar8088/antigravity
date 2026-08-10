package delta

import "log"

// SetFixedContracts pins every order to a constant contract count.
//
// This is the smallest-size mode: it bypasses risk sizing and every notional
// cap so the desk trades the venue minimum and the question becomes "do the
// signals and the plumbing work" rather than "does this make money".
//
// The concurrency and per-symbol caps still apply. Those limit how many
// positions exist rather than how large they are, and a minimum-size position
// can still occupy a slot it should not.
func (b *PerpBridge) SetFixedContracts(n int) {
	if n <= 0 {
		return
	}
	b.mu.Lock()
	b.cfg.FixedContracts = n
	b.mu.Unlock()
	log.Printf("[PERP LIVE] FIXED SIZE: every order is %d contract(s) — risk-based sizing and notional caps bypassed", n)
}

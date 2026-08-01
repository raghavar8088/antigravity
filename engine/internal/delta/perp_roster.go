package delta

import (
	"log"
	"os"
	"strings"
	"sync"
)

// The scalp strategies permitted to place real perpetual orders.
//
// Owner-selected on 2026-08-01 from the scalp desk leaderboard, for a $100 live
// account. Overridable with SCALP_LIVE_STRATEGIES (comma-separated).
//
// PERMISSION, NOT ENDORSEMENT — and the distinction is unusually stark here.
// The desk's own promotion gate passes NONE of these: at the time they were
// chosen the leaderboard read "0 of 2416 streams pass", every one of the eight
// had between 2 and 12 closed trades, and their PF of 999 is the code's sentinel
// for "no losing trade yet" rather than a profit factor. The desk as a whole was
// -$14,808 over 7,192 trades at a 35.6% win rate, so eight 100%-win rows out of
// 2,416 streams is what variance produces, not evidence of edge.
//
// Six of the eight are ANTI_ mirrors, whose P&L is by construction their
// original's negated minus fees. Trading one live is a bet that its original has
// a persistent negative GROSS edge — a claim eleven trades cannot support.
//
// None of that makes this list wrong to have; it is the owner's capital and an
// explicit instruction. It does mean the account is sized so being wrong is
// survivable, and it means this comment exists so nobody later reads the list as
// though it were qualified.
var defaultScalpLiveStrategies = []string{
	"ANTI_M1_DoubleTop_10bp_Short",
	"ANTI_M1_Break_D60_T50_Long",
	"ANTI_M1_VWAP_Doji_Short",
	"ANTI_D20_EMA_Cross_9_21",
	"Historical_Vol_Percentile_Breakout",
	"ANTI_M1X_VWAP_TrendPull_Long",
	"M1X_Squeeze_Break_Short",
	"ANTI_M1_BB_Rev_CMF5_Long",
}

// ScalpLiveStrategies is the effective allow-list.
func ScalpLiveStrategies() []string {
	if raw := strings.TrimSpace(os.Getenv("SCALP_LIVE_STRATEGIES")); raw != "" {
		out := make([]string, 0, 8)
		for _, p := range strings.Split(raw, ",") {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultScalpLiveStrategies
}

// PerpAllowList gates which (strategy, symbol) streams may reach the venue.
//
// Symbol matters as much as strategy. The same strategy runs on eight symbols on
// the paper desk; only the pairing that was actually selected should trade, or
// promoting "ANTI_M1_VWAP_Doji_Short" would quietly enable seven streams nobody
// looked at.
type PerpAllowList struct {
	mu sync.RWMutex
	// byStrategy is nil when the list has never been set, which means allow
	// NOTHING. This is the opposite of the options bridge's legacy nil-means-all,
	// and deliberately so: a perpetual path that defaults to permitting every
	// strategy is not a safe default for a live account.
	byStrategy map[string]bool
	symbols    map[string]bool
}

// NewPerpAllowList starts closed: nothing is permitted until Set is called.
func NewPerpAllowList() *PerpAllowList {
	return &PerpAllowList{}
}

// Set replaces the allow-list.
func (a *PerpAllowList) Set(strategies, symbols []string) {
	sm := make(map[string]bool, len(strategies))
	for _, s := range strategies {
		if s = strings.TrimSpace(s); s != "" {
			sm[s] = true
		}
	}
	ym := make(map[string]bool, len(symbols))
	for _, s := range symbols {
		if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
			ym[s] = true
		}
	}
	a.mu.Lock()
	a.byStrategy = sm
	a.symbols = ym
	a.mu.Unlock()
	log.Printf("[PERP LIVE] allow-list: %d strateg(ies) on %d symbol(s): %v / %v",
		len(sm), len(ym), strategies, symbols)
}

// Allowed reports whether this stream may place a real order.
//
// Fails CLOSED on every uncertainty: an unset list, an unknown strategy, or an
// unknown symbol all deny.
func (a *PerpAllowList) Allowed(strategy, symbol string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.byStrategy) == 0 {
		return false
	}
	if !a.byStrategy[strategy] {
		return false
	}
	// An empty symbol set means "any symbol this strategy trades" — used when the
	// operator selected strategies without pinning symbols.
	if len(a.symbols) == 0 {
		return true
	}
	return a.symbols[strings.ToUpper(strings.TrimSpace(symbol))]
}

// Strategies lists the permitted strategy names.
func (a *PerpAllowList) Strategies() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]string, 0, len(a.byStrategy))
	for s := range a.byStrategy {
		out = append(out, s)
	}
	return out
}

// Count is how many strategies are permitted.
func (a *PerpAllowList) Count() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.byStrategy)
}

// ReportUnknown logs any allow-listed name that the desk does not actually run.
//
// The list is long hand-typed strings compared by equality, so a typo does not
// fail — it permits a strategy that does not exist while the roster reads as
// though it were enabled. That is the same silent shape as an allow-list entry
// for an instrument the engine cannot trade.
func (a *PerpAllowList) ReportUnknown(known map[string]bool) []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var unknown []string
	for s := range a.byStrategy {
		if !known[s] {
			unknown = append(unknown, s)
		}
	}
	if len(unknown) > 0 {
		log.Printf("[PERP LIVE] ⚠️  %d allow-listed name(s) match NO running strategy — they permit nothing: %v",
			len(unknown), unknown)
	} else {
		log.Printf("[PERP LIVE] allow-list: all %d strategies resolved", len(a.byStrategy))
	}
	return unknown
}

package delta

import (
	"sort"
	"strings"
)

// Symbols this desk will not trade, because their PRICE GRID cannot hold a stop.
//
// Measured on 2026-08-16 by TestPerpGridAudit_AgainstTheLiveVenue, which calls
// StopFractionFor and then stopGridTicks in the same order executePerpSignal
// does. Each symbol's stop came out under the 20-tick minimum, so every order on
// it is refused before sizing. They are removed rather than left to be refused
// one signal at a time: a contract that cannot hold a stop still produces paper
// fills, still ranks on the leaderboard, and is still selected for promotion —
// which is how eight top-ranked picks on MOVEUSD came to be chosen.
//
// This is a claim about the CONTRACT, not about the strategy or the market. A
// tick size is a property of the instrument and does not improve.
//
// What the numbers mean: "16.5 ticks" is the stop's width in minimum price
// increments. Under 20, a single rounding step moves price by more than 5% of
// the planned risk, so the stop that gets sent is materially not the stop the
// strategy chose.
//
//	RIVERUSD     4.0    SOPHUSD      6.0    BANKUSD      6.0    ORDERUSD  6.0
//	DOTUSD       6.8    XRPUSD       8.0    BMTUSD       8.0    REDUSD    8.0
//	SOLVUSD      9.9    TACUSD      10.0    IOUSD       10.0    COOKIEUSD 10.3
//	RAREUSD     10.3    ETHFIUSD    12.0    FARTCOINUSD 13.0    BUSD     13.2
//	MUBARAKUSD  13.7    ESPORTSUSD  14.0    PIPPINUSD   15.5    MMTUSD   15.8
//	LINKUSD     15.8    KAITOUSD    17.7    BILLUSD     18.2    LABUSD   19.0
//
// BNBUSD IS NOT HERE, and was on the first version of this list. It read 20.0
// ticks on the opening run and 53.9 and 59.9 on three later ones. One reading
// at the boundary is not a property of the contract, and banning the venue's
// second-largest perpetual on an outlier would have been a real cost paid for a
// measurement error. The re-measure is why it survived.
//
// MOVEUSD and 1000SATSUSD were already excluded and are folded in here so there
// is ONE list rather than a rule in the roster, a filter in the universe and a
// test asserting a third thing. MOVEUSD is 5.7 ticks; 1000SATSUSD cannot express
// a stop at all — a 0.35% move is 0.37 of one tick.
//
// The three symbols closest to the line — BILLUSD 18.2, LABUSD 19.0, KAITOUSD
// 17.7 — are inside the band where a volatility change could carry them across.
// They are still excluded: the gate is 20 for a reason, and a contract that
// needs a calm market to become tradeable is one that stops being tradeable
// exactly when a stop matters most.
var gridBlockedSymbols = map[string]string{
	"RIVERUSD":    "4.0 ticks",
	"SOPHUSD":     "6.0 ticks",
	"BANKUSD":     "6.0 ticks",
	"ORDERUSD":    "6.0 ticks",
	"MOVEUSD":     "5.7 ticks",
	"DOTUSD":      "6.8 ticks",
	"XRPUSD":      "10.0 ticks",
	"BMTUSD":      "8.0 ticks",
	"REDUSD":      "8.0 ticks",
	"SOLVUSD":     "9.9 ticks",
	"TACUSD":      "10.0 ticks",
	"IOUSD":       "10.0 ticks",
	"COOKIEUSD":   "10.3 ticks",
	"RAREUSD":     "10.3 ticks",
	"ETHFIUSD":    "12.0 ticks",
	"FARTCOINUSD": "13.0 ticks",
	"BUSD":        "13.2 ticks",
	"MUBARAKUSD":  "13.7 ticks",
	"ESPORTSUSD":  "14.0 ticks",
	"PIPPINUSD":   "15.5 ticks",
	"MMTUSD":      "15.8 ticks",
	"LINKUSD":     "15.8 ticks",
	"KAITOUSD":    "17.7 ticks",
	"BILLUSD":     "18.2 ticks",
	"LABUSD":      "19.0 ticks",
	"1000SATSUSD": "a 0.35% stop is 0.37 of one tick",
}

// GridBlockedReason returns why a symbol is excluded, or "" if it is not.
//
// A REASON rather than a bool, for the same argument the grid gate makes: an
// exclusion without a cause is how a desk goes quiet and nobody can say why.
func GridBlockedReason(symbol string) string {
	return gridBlockedSymbols[strings.ToUpper(strings.TrimSpace(symbol))]
}

// IsGridBlocked reports whether a symbol may not be traded on any desk.
func IsGridBlocked(symbol string) bool {
	return GridBlockedReason(symbol) != ""
}

// GridBlockedSymbols lists the excluded symbols, sorted, for logs and tests.
func GridBlockedSymbols() []string {
	out := make([]string, 0, len(gridBlockedSymbols))
	for s := range gridBlockedSymbols {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// FilterGridBlockedSymbols drops excluded symbols from a universe.
//
// Returns the kept symbols and the dropped ones, so the caller can log what it
// removed. Silently shrinking a universe is the failure this whole file exists
// to prevent — a desk that trades 24 fewer symbols without saying so looks
// identical to a desk whose strategies stopped firing.
func FilterGridBlockedSymbols(symbols []string) (kept, dropped []string) {
	kept = make([]string, 0, len(symbols))
	for _, s := range symbols {
		if IsGridBlocked(s) {
			dropped = append(dropped, strings.ToUpper(strings.TrimSpace(s)))
			continue
		}
		kept = append(kept, s)
	}
	return kept, dropped
}

// FilterGridBlockedStreams drops streams whose symbol is excluded.
func FilterGridBlockedStreams(streams []PerpStream) (kept, dropped []PerpStream) {
	kept = make([]PerpStream, 0, len(streams))
	for _, st := range streams {
		if IsGridBlocked(st.Symbol) {
			dropped = append(dropped, st)
			continue
		}
		kept = append(kept, st)
	}
	return kept, dropped
}

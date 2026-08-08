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
// The desk's own promotion gate passes NONE of these. When they were chosen the
// leaderboard read "0 of 2416 streams pass", each had between 2 and 31 closed
// trades, and a PF of 999 on several is the code's sentinel for "no losing trade
// yet" rather than a profit factor. The desk as a whole was -$14,808 over 7,192
// trades at a 35.6% win rate, so a screenful of 100%-win rows out of 2,416
// streams is what variance produces, not evidence of edge.
//
// Nearly all of them are ANTI_ mirrors, whose P&L is by construction their
// original's negated minus fees. Trading one live is a bet that its original has
// a persistent negative GROSS edge — a claim a dozen trades cannot support.
//
// None of that makes this list wrong to have; it is the owner's capital and an
// explicit instruction. It does mean the account is sized so being wrong is
// survivable, and it means this comment exists so nobody later reads the list as
// though it were qualified.
// defaultScalpLiveStreams is the owner's selection, as exact (strategy, symbol)
// STREAMS rather than two lists that get multiplied together.
//
// Replaced 2026-08-07. The previous roster of 10 names across ADAUSD/AVAXUSD
// formed 20 streams; the owner had chosen 14 rows. This selection is three
// rows, and three streams is what it enables.
//
// Selected off the Live Strategy Leaderboard on live terms — a $100 account at
// 3x with taker fees both legs. Their records at selection:
//
//	ANTI_M1_DoubleBottom_10bp_Long ADAUSD   61 trades  gross +34.87  fees 21.59  net +13.28
//	ANTI_M1_Break_D30_T20_Long     AVAXUSD  34 trades  gross +22.58  fees 12.04  net +10.54
//	ANTI_M1_Break_D60_T50_Long     AVAXUSD  19 trades  gross +15.45  fees  6.73  net  +8.72
//
// Fee drag 44-62%: roughly half of every dollar earned goes to the venue. That
// is the best ratio on the board and still means the edge must be twice the
// cost to be worth anything.
//
// NOT qualified. The gate asks for 200 trades per stream and these have 19-61,
// selected from the right tail of 2,416 streams on in-sample data. Being on
// this list is permission, not evidence.
var defaultScalpLiveStreams = []PerpStream{
	{Strategy: "ANTI_M1_DoubleBottom_10bp_Long", Symbol: "ADAUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAXUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "AVAXUSD"},
}

// defaultScalpPaperStreams are CANDIDATES: they paper-trade on the Live Engine
// Paper Desk but cannot reach the venue.
//
// A separate tier on purpose. Adding a stream to the live roster is a decision
// to spend money on it; adding it here is a decision to WATCH it on live terms
// first. Collapsing the two would mean every candidate a leaderboard suggested
// went straight to real capital — which is how the previous roster was built,
// and it lost $13.91 over 27 fills.
//
// Selected 2026-08-09 from the scalp leaderboard restated on live terms. Every
// one has 1-4 trades, so none is evidence of anything yet; that is precisely
// why they belong on paper rather than on the wallet.
var defaultScalpPaperStreams = []PerpStream{
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "MOVEUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "MOVEUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "LIGHTUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "LIGHTUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "XAIUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "1000SATSUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "1000SATSUSD"},
	{Strategy: "ANTI_M1_DoubleBottom_10bp_Long", Symbol: "1000SATSUSD"},
}

// ScalpPaperStreams is everything the Live Engine Paper Desk trades: the live
// roster plus the candidates.
//
// The live streams are included so the paper desk stays a faithful mirror of
// what real money is doing — that comparison is the module's whole purpose, and
// dropping them to make room for candidates would destroy it.
func ScalpPaperStreams() []PerpStream {
	live := ScalpLiveStreams()
	out := make([]PerpStream, 0, len(live)+len(defaultScalpPaperStreams))
	seen := map[string]bool{}
	for _, st := range append(append([]PerpStream{}, live...), defaultScalpPaperStreams...) {
		k := perpStreamKey(st.Strategy, st.Symbol)
		if !seen[k] {
			seen[k] = true
			out = append(out, st)
		}
	}
	return out
}

// PerpStreamPaperPermitted reports whether a stream may PAPER trade.
//
// Deliberately wider than PerpStreamPermitted, which gates the venue. A stream
// passing this and failing that is the normal state of a candidate.
func PerpStreamPaperPermitted(strategy, symbol string) bool {
	key := perpStreamKey(strategy, symbol)
	for _, st := range ScalpPaperStreams() {
		if perpStreamKey(st.Strategy, st.Symbol) == key {
			return true
		}
	}
	return false
}

// ScalpLiveStreams is the effective stream allow-list.
//
// SCALP_LIVE_STREAMS overrides it, as comma-separated strategy:SYMBOL pairs.
// A malformed entry is skipped with a log rather than silently widening the
// list — the failure mode to avoid is an unparsed entry becoming "allow all".
func ScalpLiveStreams() []PerpStream {
	raw := strings.TrimSpace(os.Getenv("SCALP_LIVE_STREAMS"))
	if raw == "" {
		return defaultScalpLiveStreams
	}
	out := make([]PerpStream, 0, 8)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bits := strings.SplitN(part, ":", 2)
		if len(bits) != 2 || strings.TrimSpace(bits[0]) == "" || strings.TrimSpace(bits[1]) == "" {
			log.Printf("[PERP LIVE] SCALP_LIVE_STREAMS: skipping malformed entry %q (want strategy:SYMBOL)", part)
			continue
		}
		out = append(out, PerpStream{
			Strategy: strings.TrimSpace(bits[0]),
			Symbol:   strings.ToUpper(strings.TrimSpace(bits[1])),
		})
	}
	if len(out) == 0 {
		log.Printf("[PERP LIVE] SCALP_LIVE_STREAMS parsed to nothing — falling back to the built-in selection")
		return defaultScalpLiveStreams
	}
	return out
}

// ScalpLiveStrategies is the distinct strategy names in the live selection,
// for display and for the SCALP_LIVE_STRATEGIES override.
//
// Derived from the streams so the two can never disagree. It is NOT the gate —
// the gate is the exact pair; a name here says only that some stream using it
// is permitted.
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
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	for _, st := range ScalpLiveStreams() {
		if !seen[st.Strategy] {
			seen[st.Strategy] = true
			out = append(out, st.Strategy)
		}
	}
	return out
}

// PerpStreamPermitted reports whether a stream is on the live selection.
//
// Reads the configured selection rather than a live bridge instance, so callers
// that exist whether or not trading is configured — the Live Engine Paper Desk —
// get the same answer the venue gate would give.
func PerpStreamPermitted(strategy, symbol string) bool {
	key := perpStreamKey(strategy, symbol)
	for _, st := range ScalpLiveStreams() {
		if perpStreamKey(st.Strategy, st.Symbol) == key {
			return true
		}
	}
	return false
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
	// pairs, when non-nil, pins the exact (strategy, symbol) STREAMS permitted.
	//
	// Set() takes two independent lists and forms their cross product, which is
	// not what an operator selecting rows off a leaderboard means. Choosing
	// three rows — DoubleBottom on ADAUSD, and two Breaks on AVAXUSD — enabled
	// six streams under the cross product, three of which nobody picked.
	//
	// The doc comment on this type already claimed "only the pairing that was
	// actually selected should trade". This makes that true.
	pairs map[string]bool
}

// PerpStream is one (strategy, symbol) pair — the unit an operator actually
// selects, and the unit that reaches the venue.
type PerpStream struct {
	Strategy string
	Symbol   string
}

func perpStreamKey(strategy, symbol string) string {
	return strings.TrimSpace(strategy) + "|" + strings.ToUpper(strings.TrimSpace(symbol))
}

// NewPerpAllowList starts closed: nothing is permitted until Set is called.
func NewPerpAllowList() *PerpAllowList {
	return &PerpAllowList{}
}

// SetPairs pins the exact streams permitted, replacing any previous list.
//
// Preferred over Set: it cannot enable a pairing that was not chosen. Set
// remains for callers that genuinely mean "these strategies on these symbols".
func (a *PerpAllowList) SetPairs(streams []PerpStream) {
	pm := make(map[string]bool, len(streams))
	sm := make(map[string]bool, len(streams))
	ym := make(map[string]bool, len(streams))
	for _, st := range streams {
		strat := strings.TrimSpace(st.Strategy)
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if strat == "" || sym == "" {
			continue
		}
		pm[perpStreamKey(strat, sym)] = true
		sm[strat] = true
		ym[sym] = true
	}
	a.mu.Lock()
	a.pairs = pm
	a.byStrategy = sm
	a.symbols = ym
	a.mu.Unlock()
	log.Printf("[PERP LIVE] allow-list: %d exact stream(s) permitted", len(pm))
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
	// Clear any pinned pairs: this call means "these strategies on these
	// symbols", and leaving a previous pair map in place would silently keep
	// enforcing a narrower list than the caller just asked for.
	a.pairs = nil
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
	// Exact streams win when pinned — no cross product, no inference.
	if a.pairs != nil {
		return a.pairs[perpStreamKey(strategy, symbol)]
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

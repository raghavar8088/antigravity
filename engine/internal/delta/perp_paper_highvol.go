package delta

import (
	"log"
	"sort"
	"strings"
	"sync"
)

// High Volume Crypto PAPER Trading — a NAMED book, on the owner's selection.
//
// The High Volume desk runs 1,674 strategies across twelve majors, which is
// 20,088 streams, and its leaderboard is a sort of those 20,088 records rather
// than a test of any of them. This book is the owner's answer to the obvious
// next question: take the handful at the top and let them trade a single
// balance on their own, where nothing else is competing for the slots and the
// result is not a slice of a board.
//
// WHAT THIS BOOK IS NOT. It is not a promotion, and the numbers that selected
// these nine cannot support one. Their records at selection run 18 to 39 closed
// trades against a gate that asks for 200; every one was chosen from the right
// tail of 20,088 in-sample streams, where profitable-looking rows exist whether
// or not any edge does. The desk as a whole was -$21,837 over 45,158 trades at
// a 25.1% win rate when they were picked, so nine winners inside it is a fact
// about dispersion before it is a fact about them.
//
// What it IS: a clean, separate record. Their leaderboard rows were earned
// while sharing three concurrency slots with 20,079 other streams, so most of
// their signals were refused and the trades they did take are the ones that
// happened to arrive when a slot was free — a FREQUENCY property, not a quality
// one. Here there are nine claimants for the same three slots, so what they do
// with a fair share of the book is a different measurement, and it starts from
// zero today rather than inheriting a number it did not earn under these rules.
//
// Paper only, structurally: membership of this book grants nothing at the
// venue. PerpStreamPermitted alone decides that, and it does not read this file.
const PaperAccountHighVol = "HVPAPER"

// HighVolPaperEquityUSD is the book's capital: $500, at the owner's direction.
//
// Five times the $100 the sibling books run, which matters for reading the
// result rather than for the strategies: the same trades are being sized five
// times larger, so the percentages transfer and the dollars do not. A -20% book
// is -$100 here and -$20 next door for identical behaviour.
const HighVolPaperEquityUSD = 500.0

// highVolPaperStreams is the owner's selection, made 2026-09-23 from the High
// Volume board's own "Live Terms" leaderboard.
//
// Their records at selection — $100 account, Delta taker fee both legs, which
// is the restatement the board shows rather than the notional-scaled figures:
//
//	rank  stream                                     n   WR    gross    fees    net      ROI
//	  2   MTF_1h_LevelRetest_Long        ADAUSD     19  77%  +31.29   -6.73  +24.56  +24.6%
//	  3   MTF_1d_OpeningRangeBreak_Long  DOGEUSD    35  79%  +35.68  -12.39  +23.29  +23.3%
//	  4   MTF_4h_LevelRetest_Long        HYPEUSD    18  77%  +29.16   -6.37  +22.79  +22.8%
//	 11   MTF_4h_LinRegBreak_Long        LTCUSD     37  80%  +28.27  -13.10  +15.17  +15.2%
//	 15   MTF_10m_DirectionalTriangle_Long UNIUSD   18  77%  +20.41   -6.37  +14.04  +14.0%
//	 34   MTF_10m_OpeningRangeBreak_Long UNIUSD     34  79%  +23.08  -12.04  +11.04  +11.0%
//	 35   MTF_4h_TripleTopBottom_Long    AVAXUSD    21  78%  +18.44   -7.43  +11.00  +11.0%
//	 37   MTF_10m_OptimalTradeEntry_Long UNIUSD     26  78%  +19.98   -9.20  +10.78  +10.8%
//	 40   MTF_10m_FairValueGap_Long      LTCUSD     39  80%  +24.45  -13.81  +10.64  +10.6%
//
// Every one is LONG, and that is worth seeing plainly: nine long streams across
// six symbols is one directional bet in nine pieces whenever the majors move
// together, which they usually do. The concurrency cap limits how much of that
// can be expressed at once — three positions, not nine — but it does not make
// the nine independent.
//
// Fee drag runs 21% to 56% of gross. Half of what the better half of these
// earned went to the venue, and that is the ratio to watch: an edge has to be
// roughly twice its cost before any of it reaches the balance.
//
// The 77-80% win rates are the board's LIVE-terms win rate, which counts a
// trade as a win when it closed net-positive on a $100 account. They sit against
// a desk-wide 25.1%, and a 55-point gap across nine rows selected from 20,088 is
// what the tail of a large sample looks like, not a discovery.
var highVolPaperStreams = []PerpStream{
	{Strategy: "MTF_1h_LevelRetest_Long", Symbol: "ADAUSD"},
	{Strategy: "MTF_1d_OpeningRangeBreak_Long", Symbol: "DOGEUSD"},
	{Strategy: "MTF_4h_LevelRetest_Long", Symbol: "HYPEUSD"},
	{Strategy: "MTF_4h_LinRegBreak_Long", Symbol: "LTCUSD"},
	{Strategy: "MTF_10m_DirectionalTriangle_Long", Symbol: "UNIUSD"},
	{Strategy: "MTF_10m_OpeningRangeBreak_Long", Symbol: "UNIUSD"},
	{Strategy: "MTF_4h_TripleTopBottom_Long", Symbol: "AVAXUSD"},
	{Strategy: "MTF_10m_OptimalTradeEntry_Long", Symbol: "UNIUSD"},
	{Strategy: "MTF_10m_FairValueGap_Long", Symbol: "LTCUSD"},
}

// HighVolPaperStreams returns the selection.
func HighVolPaperStreams() []PerpStream {
	out := make([]PerpStream, len(highVolPaperStreams))
	copy(out, highVolPaperStreams)
	return out
}

// namedPaperBook is a book that is neither numbered, gold, nor per-symbol: an
// explicit roster with its own capital.
//
// A registry rather than another case in ScalpPaperStreamsFor, because the two
// existing extension points both carry a rule this book breaks. Symbol books
// refuse any stream not on their one symbol ("a foreign symbol in a symbol book
// is a wiring bug") and this roster spans six; the gold book is filtered to the
// gold instruments for the same reason. A hand-picked cross-symbol selection is
// a third shape, so it gets its own one rather than loosening a check that is
// load-bearing for the other two.
type namedPaperBook struct {
	streams []PerpStream
	equity  float64
}

var (
	namedBookMu    sync.RWMutex
	namedBooks     = map[string]namedPaperBook{}
	namedBookOrder []string
)

// RegisterNamedPaperBook registers one roster under an id and returns how many
// streams it holds.
//
// Registration is what CREATES the book. Nothing here runs unless a process
// asks for it, so the scalp desk, the demo desk and the Top Crypto process are
// unaffected by this file existing — they would otherwise each grow a ninth
// book watching streams their operators never chose, and its trades would land
// in their payloads under an id their pages do not know.
//
// Re-registering replaces the roster and keeps the id's position, so a later
// call cannot silently produce two books that differ only in order.
func RegisterNamedPaperBook(id string, equityUSD float64, streams []PerpStream) int {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return 0
	}
	namedBookMu.Lock()
	defer namedBookMu.Unlock()

	rows := make([]PerpStream, 0, len(streams))
	seen := map[string]bool{}
	for _, st := range streams {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		name := strings.TrimSpace(st.Strategy)
		if sym == "" || name == "" {
			continue
		}
		k := perpStreamKey(name, sym)
		if seen[k] {
			continue
		}
		seen[k] = true
		rows = append(rows, PerpStream{Strategy: name, Symbol: sym})
	}
	if _, exists := namedBooks[id]; !exists {
		namedBookOrder = append(namedBookOrder, id)
	}
	namedBooks[id] = namedPaperBook{streams: rows, equity: equityUSD}
	log.Printf("[PAPER BOOK %s] registered: %d streams, $%.2f starting equity", id, len(rows), equityUSD)
	return len(rows)
}

// NamedPaperBookIDs returns the registered named books, in registration order.
func NamedPaperBookIDs() []string {
	namedBookMu.RLock()
	defer namedBookMu.RUnlock()
	out := make([]string, len(namedBookOrder))
	copy(out, namedBookOrder)
	return out
}

// IsNamedPaperBook reports whether an id belongs to a registered named book.
func IsNamedPaperBook(account string) bool {
	namedBookMu.RLock()
	defer namedBookMu.RUnlock()
	_, ok := namedBooks[strings.ToUpper(strings.TrimSpace(account))]
	return ok
}

// namedPaperBookStreams returns one named book's roster.
func namedPaperBookStreams(account string) ([]PerpStream, bool) {
	namedBookMu.RLock()
	defer namedBookMu.RUnlock()
	b, ok := namedBooks[strings.ToUpper(strings.TrimSpace(account))]
	if !ok {
		return nil, false
	}
	out := make([]PerpStream, len(b.streams))
	copy(out, b.streams)
	return out, true
}

// NamedPaperBookEquity returns one named book's starting capital.
//
// Reported as (value, registered) rather than a bare float, because a book that
// does not exist and a book that starts at zero have to be distinguishable: the
// caller falls back to the process-wide default for the first and must not for
// the second.
func NamedPaperBookEquity(account string) (float64, bool) {
	namedBookMu.RLock()
	defer namedBookMu.RUnlock()
	b, ok := namedBooks[strings.ToUpper(strings.TrimSpace(account))]
	if !ok {
		return 0, false
	}
	return b.equity, true
}

// RegisterHighVolPaperBook registers the High Volume Crypto Paper Trading book.
func RegisterHighVolPaperBook() int {
	return RegisterNamedPaperBook(PaperAccountHighVol, HighVolPaperEquityUSD, highVolPaperStreams)
}

// HighVolPaperUnservedStreams returns the rostered streams this process cannot
// actually trade, given the strategies it loaded and the symbols it resolved.
//
// It exists because both halves of a stream can be wrong in a way that never
// raises anything: a symbol below the turnover floor never enters the universe,
// and a strategy name that the pack no longer builds simply never signals. Both
// produce a row on the board that sits at zero forever, which reads as "quiet
// market" rather than "this will never fill". The desk logs what comes back
// here at boot so the two are told apart on day one instead of after a week of
// waiting for a trade that cannot arrive.
//
// Returns the streams as "STRATEGY SYMBOL" strings, sorted, for logging.
func HighVolPaperUnservedStreams(strategies, symbols []string) []string {
	haveStrategy := map[string]bool{}
	for _, s := range strategies {
		haveStrategy[strings.TrimSpace(s)] = true
	}
	haveSymbol := map[string]bool{}
	for _, s := range symbols {
		haveSymbol[strings.ToUpper(strings.TrimSpace(s))] = true
	}
	out := make([]string, 0, 2)
	for _, st := range highVolPaperStreams {
		missing := make([]string, 0, 2)
		if !haveStrategy[st.Strategy] {
			missing = append(missing, "strategy not in the pack")
		}
		if !haveSymbol[strings.ToUpper(st.Symbol)] {
			missing = append(missing, "symbol not in the universe")
		}
		if len(missing) > 0 {
			out = append(out, st.Strategy+" "+st.Symbol+" ("+strings.Join(missing, ", ")+")")
		}
	}
	sort.Strings(out)
	return out
}

package delta

import (
	"log"
	"os"
	"sort"
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
// Replaced 2026-08-16 at the owner's direction, from the Scalp Desk leaderboard.
//
// The owner selected fourteen rows. Six are here. The other eight are on
// MOVEUSD and are NOT on this list, for a reason that was measured against the
// live venue rather than inferred:
//
//	MOVEUSD marks at 0.00636728 against a 0.00001 tick. A 0.9% stop — the
//	narrowest this pack produces — is 5.7 TICKS wide. The grid gate needs 20.
//	Every order on it would be refused before sizing, so putting the eight on
//	this list would produce a roster that looks populated and can never fill.
//
// Its turnover is $27,833/day, rank 100 of 220. The eight leaderboard rows that
// ranked MOVEUSD at #1, #2, #5, #6, #9, #10, #31 and #60 were paper fills on a
// contract whose entire day's volume is $27.8k, priced at a mid the desk's fill
// model assumes it can always get. That is why they rank so highly: the
// contract that cannot be traded is also the one that cannot be filled against,
// so nothing in its paper record was ever tested by a real book.
//
// LABUSD is here and is expected to refuse as well — 7.2 ticks at the same 0.9%
// stop. It is included because the owner chose it and because the refusal will
// be VISIBLE: the grid gate logs a reason per signal, the UI shows the count,
// and five consecutive refusals disable the stream automatically. An assertion
// in a comment is worth less than a refusal the desk can show, and the two
// LABUSD rows are the cheapest way to prove the measurement above is right.
//
// Four streams can actually reach the venue: CROSSUSD (88 ticks), BLESSUSD
// (80), PIEVERSEUSD (76) and GIGGLEUSD (27). Three of those four were already
// on the previous roster.
//
// What this replaces: ten streams, of which the only one that traded with any
// frequency was MTF_1h_TrendPullback_Long on AVAAIUSD — 8 of the desk's 11
// live fills, net negative, and switched off by the owner at 00:11 UTC. AVAAI,
// AIOT, AIOUSD, LINKUSD and SKYAIUSD are dropped with it.
//
// NOT qualified, and the leaderboard says so itself. Twelve of the fourteen
// rows read "too few" in the Qualified column; the two that read YES have 33
// and 31 trades against a gate that asks for 200, and both are on MOVEUSD.
// Being on this list is permission, not evidence.
var defaultScalpLiveStreams = []PerpStream{
	// Executable — the stop clears the tick grid with room.
	{Strategy: "MTF_4h_SqueezeExpansion_Short", Symbol: "CROSSUSD"},
	{Strategy: "MTF_4h_RSITrendReset_Short", Symbol: "BLESSUSD"},
	{Strategy: "MTF_1h_RSITrendReset_Long", Symbol: "PIEVERSEUSD"},
	{Strategy: "MTF_1h_TrendPullback_Short", Symbol: "GIGGLEUSD"},

	// Grid-marginal at 7.2 ticks. Expect refusals, then auto-disable.
	{Strategy: "MTF_1h_RSITrendReset_Short", Symbol: "LABUSD"},
	{Strategy: "MTF_10m_TrendPullback_Short", Symbol: "LABUSD"},
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
// defaultScalpPaperStreams are CANDIDATES: they paper-trade on the Live Engine
// Paper Desk but cannot reach the venue.
//
// A separate tier on purpose. Adding a stream to the live roster is a decision
// to spend money on it; adding it here is a decision to WATCH it on live terms
// first. Collapsing the two is how the previous roster was built - straight
// from a leaderboard to real capital - and it lost $13.91 over 27 fills.
//
// History: the LIGHTUSD and XAIUSD candidates were promoted to the live roster;
// the MOVEUSD and 1000SATSUSD ones were removed outright because those
// contracts cannot support the trade at all - see the tradeable filter in
// cmd/scalp_prelive/symbols.go, which now excludes them from the universe so no
// strategy can pick them up again.
var defaultScalpPaperStreams = []PerpStream{
	// Added 2026-08-09 from the scalp leaderboard. Trade counts run 2-9, so the
	// Qualified column reads "too few" on every one — which is the reason they
	// are here rather than on the live roster.
	//
	// Symbol liquidity, checked before adding: SKYAIUSD $18.3M/day, TSTUSD
	// $7.5M, COOKIEUSD $3.0M, SAGAUSD $3.0M, AVAAIUSD $93k. All clear the tick
	// grid for a 0.35% stop. AVAAIUSD is the thin one and is worth watching for
	// that reason as much as any other — paper cannot show slippage, so a
	// result there will look better than a live one would.
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "Ornstein_Uhlenbeck_Reversion", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "M1X_VWAP_TrendPull_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Short", Symbol: "SAGAUSD"},
	{Strategy: "Ornstein_Uhlenbeck_Reversion", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAAIUSD"},
}

// The independent paper books. Each starts at its own $100.
//
// Separate accounts, not one list with a tag. Each starts at its own $100 and
// spends from its own balance, so a winner in one cannot fund a position in the
// other — which is the whole point of running two: they are competing
// hypotheses about which streams deserve capital, and a shared balance would
// let the better one subsidise the worse and hide it.
const (
	PaperAccount01 = "01"
	PaperAccount02 = "02"
	PaperAccount03 = "03"
	PaperAccount04 = "04"
	PaperAccount05 = "05"
	PaperAccount06 = "06"
	// PaperAccountGold is the Gold Desk's book. Named rather than numbered
	// because it is not another candidate roster competing with 01-06: it holds
	// a different asset, and the numbered books would silently absorb it into a
	// leaderboard that compares crypto scalps against each other.
	PaperAccountGold = "GOLD"
)

// GoldSymbols are the metal-backed perpetuals on Delta.
//
// XAUT (Tether Gold) and PAXG (PAX Gold) are TOKENS redeemable for a troy ounce,
// not interbank XAU/USD. They track spot gold closely and they are the only gold
// this venue can actually fill, but the basis is real and occasionally wide —
// the two quoted $15 apart (0.34%) at the time this was written, for an
// instrument that is nominally the same ounce of gold. Anything comparing this
// desk to a forex gold chart has to account for that.
//
// Silver (SLVONUSD) is listed here too and deliberately excluded: the desk was
// asked for gold, and adding a second metal would make every result a question
// about which metal rather than about the strategy.
func GoldSymbols() []string {
	return []string{"XAUTUSD", "PAXGUSD"}
}

// IsGoldSymbol reports whether a symbol belongs to the Gold Desk.
func IsGoldSymbol(symbol string) bool {
	u := strings.ToUpper(strings.TrimSpace(symbol))
	for _, g := range GoldSymbols() {
		if u == g {
			return true
		}
	}
	return false
}

// goldPaperStreams is the Gold book's watch list, registered at boot.
//
// Registered rather than hardcoded because the roster IS "every strategy the
// desk runs, on gold" — and that set is owned by the strategy pack, which
// changes. A hand-copied list would drift the moment a strategy is added or
// removed, and drift here does not fail loudly: it produces a board with rows
// for strategies that no longer exist and no rows for the ones that do.
var (
	goldPaperMu      sync.RWMutex
	goldPaperStreams []PerpStream
)

// SetGoldPaperStreams registers the Gold book's watch list. Called once at boot
// by the desk, which is the only component that knows both the strategy pack
// and the resolved symbol universe.
func SetGoldPaperStreams(streams []PerpStream) {
	goldPaperMu.Lock()
	defer goldPaperMu.Unlock()
	goldPaperStreams = make([]PerpStream, 0, len(streams))
	seen := map[string]bool{}
	for _, st := range streams {
		if !IsGoldSymbol(st.Symbol) {
			continue // a non-gold stream in the gold book is a wiring bug, not a preference
		}
		k := perpStreamKey(st.Strategy, st.Symbol)
		if seen[k] {
			continue
		}
		seen[k] = true
		goldPaperStreams = append(goldPaperStreams, PerpStream{Strategy: st.Strategy, Symbol: strings.ToUpper(st.Symbol)})
	}
	log.Printf("[GOLD DESK] watch list registered: %d streams across %v", len(goldPaperStreams), GoldSymbols())
}

// GoldPaperStreams returns the Gold book's watch list.
func GoldPaperStreams() []PerpStream {
	goldPaperMu.RLock()
	defer goldPaperMu.RUnlock()
	out := make([]PerpStream, len(goldPaperStreams))
	copy(out, goldPaperStreams)
	return out
}

// PaperAccountIDs is every book, in display order.
func PaperAccountIDs() []string {
	return []string{PaperAccount01, PaperAccount02, PaperAccount03, PaperAccount04, PaperAccount05, PaperAccount06, PaperAccountGold}
}

// defaultScalpPaperStreams02 is Account 02's watch list.
//
// Added 2026-08-09 from the scalp leaderboard. Trade counts run 1-14, so every
// row reads "too few" — which is why this is a paper book and not a promotion.
//
// It overlaps Account 01 on several streams by design: the same stream in two
// books with different neighbours shows how much of a result is the stream and
// how much is the company it keeps. Three concurrent positions is a real
// constraint, so which OTHER streams are competing for those slots changes what
// any one of them gets to trade.
var defaultScalpPaperStreams02 = []PerpStream{
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_D20_HeikinAshi_Flip", Symbol: "TSTUSD"},
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_D20_VWAP_Reversion", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_D20_HeikinAshi_Flip", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1X_VWAP_TrendPull_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "MUBARAKUSD"},
	{Strategy: "M1_MACD_Align_Long", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_D20_Momentum_ROC", Symbol: "AIOUSD"},
	{Strategy: "ANTI_D20_Stoch_Cross", Symbol: "AIOUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Long", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "BANKUSD"},
}

// defaultScalpPaperStreams03 is Account 03's watch list.
//
// Added 2026-08-09 from the scalp leaderboard. Trade counts run 3-19, so every
// row still reads "too few" — the same reason 01 and 02 exist rather than a
// promotion.
//
// It overlaps 02 heavily and that is the experiment: the two books hold many of
// the same streams in different combinations, and with only three concurrent
// slots per book, WHICH streams compete for them changes what each gets to
// trade. A stream that earns in one book and not the other is telling you the
// result was the company it kept, not the signal.
var defaultScalpPaperStreams03 = []PerpStream{
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "BANKUSD"},
	{Strategy: "ANTI_D20_HeikinAshi_Flip", Symbol: "TSTUSD"},
	{Strategy: "ANTI_D20_VWAP_Reversion", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1X_VWAP_TrendPull_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAAIUSD"},
}

// defaultScalpPaperStreams04 is Account 04's watch list.
//
// Added 2026-08-09 from the scalp leaderboard. 41 streams, the widest book so
// far, overlapping 02 and 03 substantially.
//
// Worth stating plainly: all four books are drawn from the top of the same
// leaderboard, and that leaderboard ranks 27,784 streams. Selecting the best
// rows four times over does not produce four independent tests — it produces
// four samples of the same right tail. What separates them is only the
// COMBINATION, because three concurrent slots per book means the streams
// compete with each other for the chance to trade.
//
// So a stream earning in one book and not another is evidence about crowding,
// not about the signal. Nothing here is evidence that any of these deserve
// money; the trade counts (3-26) are two orders of magnitude short of the gate.
var defaultScalpPaperStreams04 = []PerpStream{
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "BANKUSD"},
	{Strategy: "ANTI_D20_HeikinAshi_Flip", Symbol: "TSTUSD"},
	{Strategy: "ANTI_D20_VWAP_Reversion", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_MACD_Align_Long", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "LABUSD"},
	{Strategy: "M1_NR7_Expand_T20_Long", Symbol: "SOLVUSD"},
	{Strategy: "M1_NR7_Expand_T50_Long", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1X_VWAP_TrendPull_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "AVAAIUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAAIUSD"},
}

// defaultScalpPaperStreams05 is Account 05's watch list.
//
// Added 2026-08-09 from the scalp sweep leaderboard, taken in leaderboard order
// — the 30 highest-capital streams on that board.
//
// Read the trade counts before reading the returns. Only five of these thirty
// cleared the sweep's own qualification bar; the rest are single-digit and
// low-double-digit samples where a +35% return is one or two lucky fills. The
// board sorts on ending capital, which puts n=7 above n=62 whenever the small
// sample got lucky — so the ordering here is emphatically not a ranking of
// merit, and this book exists to find out which of them survives contact with
// three concurrent slots and a real fee.
//
// Fee drag is the number that decides most of these. The qualified rows carry
// 30-43% fee drag: ANTI_Recurrence_Quantification_Signal turned $80 gross into
// $54 net on COOKIEUSD, and its TSTUSD twin gave up 43% of gross. A stream that
// hands the venue two fifths of its edge needs that edge to be durable, and
// thirty streams competing for three slots is precisely the constraint that
// tells us whether it is.
//
// Four symbols here (XANUSD, BMTUSD, ARCUSD, BANKUSD) appear in no other book.
// The desk discovers its universe from Delta above a $50k turnover floor, so
// these streams go quiet if that turnover dries up rather than failing loudly.
var defaultScalpPaperStreams05 = []PerpStream{
	// Cleared the sweep's qualification bar (n >= 32).
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "TSTUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "BMTUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Short", Symbol: "TSTUSD"},
	// Did not qualify — sample too small to mean anything yet.
	{Strategy: "ANTI_D20_BB_Reversion", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_D20_RSI_Reversion", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "LABUSD"},
	{Strategy: "M1_VWAP_Rev_40bp_Short", Symbol: "LABUSD"},
	{Strategy: "ANTI_D20_MACD_Cross", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "BMTUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "BMTUSD"},
	{Strategy: "ANTI_D20_HeikinAshi_Flip", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Short", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "BANKUSD"},
	{Strategy: "ANTI_M1_MACD_Align_Short", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "COOKIEUSD"},
	{Strategy: "M1_VWAP_Rev_70bp_Short", Symbol: "LABUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "ARCUSD"},
	{Strategy: "ANTI_M1X_VWAP_TrendPull_Long", Symbol: "XANUSD"},
}

// defaultScalpPaperStreams06 is Account 06's watch list.
//
// Added 2026-08-10 from the same scalp sweep board as Account 05, taken in
// leaderboard order — but with one rule the other books do not have: no
// (strategy, symbol) pair that any of Accounts 01-05 already watches.
//
// The pair is the unit, not the strategy. ANTI_M1_RSI2_5_95_T50_Long on TSTUSD
// belongs to five other books, so it is absent here; the same strategy on
// BMTUSD, NEIROUSD and MMTUSD is present, because a strategy is not one thing
// across symbols. Twenty-nine of the top 124 rows were dropped by that rule.
//
// What that buys is a book whose result cannot be explained by the books beside
// it. Accounts 01-05 overlap heavily by design — the same stream with different
// neighbours competing for three slots is the experiment they run. This one
// asks the other question: whether the streams NO existing book has funded are
// worth funding. Every row here is new evidence or none at all.
//
// Read the trade counts first. Thirty-one of these ninety-five cleared the
// board's own n >= 30 bar; the remaining sixty-four are small samples where a
// +30% return is a handful of lucky fills, and the board sorts on ending capital
// rather than on merit, so the order below is not a ranking. Six symbols here
// (BEATUSD, SIRENUSD, BILLUSD, GRIFFAINUSD, KAITOUSD, VELVETUSD) appear in no
// other book, and the desk discovers its universe above a $50k turnover floor —
// so those streams go quiet if turnover dries up rather than failing loudly.
var defaultScalpPaperStreams06 = []PerpStream{
	// Cleared the board's own qualification bar (n >= 30).
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "BMTUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "LABUSD"},
	{Strategy: "ANTI_D20_Keltner_Breakout", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Long", Symbol: "BMTUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "BMTUSD"},
	{Strategy: "ANTI_M1_MACD_Align_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "BMTUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_Recurrence_Quantification_Signal", Symbol: "NEIROUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Long", Symbol: "MMTUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_Structural_Break_Detection", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Long", Symbol: "BEATUSD"},
	{Strategy: "ANTI_Ornstein_Uhlenbeck_Reversion", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Long", Symbol: "BEATUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "LISTAUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "MMTUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "LISTAUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_Structural_Break_Detection", Symbol: "MUBARAKUSD"},
	// Below 30 closed trades — carried for the sample, not for the return.
	{Strategy: "ANTI_D20_Stoch_Cross", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_40bp_Short", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_MACD_Align_Short", Symbol: "LISTAUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_ThreeCrows_Short", Symbol: "TSTUSD"},
	{Strategy: "ANTI_M1_MACD_Align_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_VWAP_Rev_70bp_Short", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_Burst_K4_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_RSI2_5_95_T50_Long", Symbol: "NEIROUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "LISTAUSD"},
	{Strategy: "ANTI_M1_DoubleBottom_20bp_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Short", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_ThreeSoldiers_Long", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_DoubleBottom_10bp_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Short", Symbol: "LISTAUSD"},
	{Strategy: "ANTI_D20_Stoch_Cross", Symbol: "BEATUSD"},
	{Strategy: "ANTI_M1XB_Trend_Pullback_E21_Long", Symbol: "MMTUSD"},
	{Strategy: "ANTI_M1_RSI2_10_90_T20_Long", Symbol: "NEIROUSD"},
	{Strategy: "ANTI_D20_Donchian_Breakout", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_EMA_Cross_T50_Short", Symbol: "LABUSD"},
	{Strategy: "ANTI_Structural_Break_Detection", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_D20_ZScore_Reversion", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Long", Symbol: "MMTUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "LISTAUSD"},
	{Strategy: "M1_Burst_K4_Short", Symbol: "BEATUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "VELVETUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Long", Symbol: "MMTUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_M1_HMA21_Flip_Short", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Short", Symbol: "COAIUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Long", Symbol: "KAITOUSD"},
	{Strategy: "ANTI_M1_EMA_Cross_T20_Long", Symbol: "LABUSD"},
	{Strategy: "Hidden_Liquidity_Detection", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T20_Short", Symbol: "LISTAUSD"},
	{Strategy: "M1_PinBar_W2_Short", Symbol: "BLESSUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "LABUSD"},
	{Strategy: "ANTI_D20_Opening_Range_Breakout", Symbol: "BMTUSD"},
	{Strategy: "ANTI_M1X_Asia_FalseBreak", Symbol: "MUBARAKUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Short", Symbol: "NEIROUSD"},
	{Strategy: "ANTI_M1_EMA_Cross_T20_Short", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Long", Symbol: "VELVETUSD"},
	{Strategy: "ANTI_M1_HMA34_Flip_Short", Symbol: "LABUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "SOLVUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Short", Symbol: "SOLVUSD"},
	{Strategy: "M1_RSI_Div_Short", Symbol: "SAGAUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "KAITOUSD"},
	{Strategy: "ANTI_M1_FailedBreak_60_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_ThreeCrows_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_InsideBar_V12_Long", Symbol: "NEIROUSD"},
	{Strategy: "ANTI_M1_BB_Rev_CMF0_Short", Symbol: "XANUSD"},
	{Strategy: "ANTI_M1_RSI_Div_Short", Symbol: "SIRENUSD"},
	{Strategy: "ANTI_M1_NR7_Expand_T50_Long", Symbol: "BILLUSD"},
	{Strategy: "ANTI_M1_MACD_Align_Short", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "LISTAUSD"},
	{Strategy: "ANTI_M1_FailedBreak_60_Long", Symbol: "SKYAIUSD"},
	{Strategy: "ANTI_M1_PinBar_W3_Long", Symbol: "BMTUSD"},
	{Strategy: "ANTI_M1_InsideBar_V20_Short", Symbol: "GRIFFAINUSD"},
	{Strategy: "ANTI_M1_FailedBreak_30_Long", Symbol: "COOKIEUSD"},
	{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "SAGAUSD"},
}

// ScalpPaperStreamsFor returns one account's watch list.
//
// Account 01 carries the LIVE roster plus its candidates, so it stays a faithful
// mirror of what real money is doing.
//
// Book membership never grants venue access, in either direction. Whether a
// stream can place a real order is decided solely by the live roster via
// PerpStreamPermitted, so adding a stream to any book here spends nothing. The
// converse matters more when reading the UI: a book may well contain streams
// that ARE live — Account 05 overlaps the live roster on 10 of its 30 — and
// those rows carry the LIVE chip because of the live roster, not because of the
// book they appear in.
func ScalpPaperStreamsFor(account string) []PerpStream {
	var src []PerpStream
	switch account {
	case PaperAccount02:
		src = defaultScalpPaperStreams02
	case PaperAccount03:
		src = defaultScalpPaperStreams03
	case PaperAccount04:
		src = defaultScalpPaperStreams04
	case PaperAccount05:
		src = defaultScalpPaperStreams05
	case PaperAccount06:
		src = defaultScalpPaperStreams06
	case PaperAccountGold:
		// Returned directly: it is already deduped and gold-filtered by
		// SetGoldPaperStreams, and an empty list here means the desk has not
		// registered one yet — which must stay empty rather than falling through
		// to the crypto default below and quietly filling a gold book with
		// altcoin streams.
		return GoldPaperStreams()
	}
	if src != nil {
		out := make([]PerpStream, 0, len(src))
		seen := map[string]bool{}
		for _, st := range src {
			k := perpStreamKey(st.Strategy, st.Symbol)
			if !seen[k] {
				seen[k] = true
				out = append(out, st)
			}
		}
		return out
	}
	return ScalpPaperStreams()
}

// PerpStreamPaperPermittedFor reports whether a stream may trade in an account.
func PerpStreamPaperPermittedFor(account, strategy, symbol string) bool {
	key := perpStreamKey(strategy, symbol)
	for _, st := range ScalpPaperStreamsFor(account) {
		if perpStreamKey(st.Strategy, st.Symbol) == key {
			return true
		}
	}
	return false
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
// Pairs returns the exact (strategy, symbol) streams this list permits, sorted.
//
// The desk trades STREAMS, not strategies: ANTI_Recurrence_Quantification_
// Signal runs on COOKIEUSD, MUBARAKUSD and BLESSUSD as three independent
// positions with three independent records. Reporting only the strategy names
// collapses those into one row and hides which instrument a result came from,
// which is the thing that decides whether a result means anything.
func (a *PerpAllowList) Pairs() []PerpStream {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]PerpStream, 0, len(a.pairs))
	for k := range a.pairs {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, PerpStream{Strategy: parts[0], Symbol: parts[1]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Strategy != out[j].Strategy {
			return out[i].Strategy < out[j].Strategy
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

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

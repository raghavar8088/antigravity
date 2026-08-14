package main

import (
	"math"
	"testing"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// withMirrors enables the mirror mechanism for one test.
//
// antiEnabled defaults to FALSE from 2026-08-14 — the inversion premise does
// not survive fees — but the mechanism still exists and must keep working for
// anyone who re-enables it. It is resolved once at boot into a package var, so
// t.Setenv cannot reach it; the var is set directly and restored.
func withMirrors(t *testing.T) {
	t.Helper()
	prev := antiEnabled
	antiEnabled = true
	t.Cleanup(func() { antiEnabled = prev })
}

// An anti-strategy is only meaningful if it is the exact inverse of a trade its
// original actually took. The previous attempt inverted the SIGNAL and let the
// mirror post its own post-only limit, which under this desk's fill rule
//
//	filled := (long && bar.Low <= limit) || (!long && bar.High >= limit)
//
// fills on the OPPOSITE price condition. The two halves therefore almost never
// traded together — 35 of 53 traded streams had no partner — and the mirror
// filled only when price moved its way, which put four fake 100%-win-rate rows
// at the top of the leaderboard.
//
// These tests pin the property that fixes it: if the original traded, the mirror
// traded, same bar, same entry, opposite side, exits swapped.

// stubStrategy emits a fixed direction on the first Evaluate and nothing after,
// so a test drives exactly one entry.
type stubStrategy struct {
	name  string
	dir   scalers.Direction
	fired bool
}

func (s *stubStrategy) Name() string                   { return s.name }
func (s *stubStrategy) ValidRegimes() []scalers.Regime { return nil }
func (s *stubStrategy) Evaluate(scalers.MarketContext) scalers.Signal {
	if s.fired {
		return scalers.Signal{Direction: scalers.DirectionNone}
	}
	s.fired = true
	return scalers.Signal{Strategy: s.name, Direction: s.dir, Confidence: 1}
}

func newMirrorDesk(t *testing.T, dir scalers.Direction) (*desk, *stubStrategy) {
	t.Helper()
	if !antiEnabled {
		t.Fatal("mirrors are disabled in this build; the pairing tests would pass vacuously")
	}
	st := &stubStrategy{name: "TEST_Strat", dir: dir}
	d := &desk{
		entries:     []scalers.RegistryEntry{{Strategy: st, Name: st.name}},
		combos:      map[string]*comboState{},
		symbols:     []*symbolState{{sym: "BTCUSD"}},
		notionalUSD: defaultNotionalUSD,
		started:     time.Now().UTC(),
	}
	return d, st
}

// flatBars builds enough 1m history for ATR(14) at a steady price.
func flatBars(n int, px float64) []scalers.Candle {
	out := make([]scalers.Candle, 0, n)
	base := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		out = append(out, scalers.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Minute),
			Open:     px, High: px * 1.0005, Low: px * 0.9995, Close: px, Volume: 1,
		})
	}
	return out
}

func ctxFor(bars []scalers.Candle) scalers.MarketContext {
	return scalers.MarketContext{Price: bars[len(bars)-1].Close, Candles1m: bars}
}

// step feeds one bar to the desk, advancing the symbol's bar index like poll().
func step(d *desk, bar scalers.Candle, hist []scalers.Candle) {
	ss := d.symbols[0]
	d.processBar(ss, ctxFor(append(hist, bar)), bar)
	ss.barIdx++
}

func mirrorState(d *desk) *comboState { return d.combos[comboKey("ANTI_TEST_Strat", "BTCUSD")] }
func origState(d *desk) *comboState   { return d.combos[comboKey("TEST_Strat", "BTCUSD")] }

// The core guarantee: a fill opens both halves, on the same bar, at one price.
func TestMirrorOpensOnTheOriginalsFill(t *testing.T) {
	withMirrors(t)
	d, _ := newMirrorDesk(t, scalers.DirectionLong)
	hist := flatBars(30, 100000)

	// Bar 0 places the pending; bar 1 trades through the limit and fills it.
	step(d, hist[len(hist)-1], hist)
	o := origState(d)
	if o.Pend == nil {
		t.Fatal("no pending order placed; the test never reached the fill path")
	}
	limit := o.Pend.Limit

	fill := scalers.Candle{
		OpenTime: hist[len(hist)-1].OpenTime.Add(time.Minute),
		Open:     limit, High: limit * 1.0001, Low: limit * 0.9999, Close: limit, Volume: 1,
	}
	step(d, fill, hist)

	if o.Pos == nil {
		t.Fatal("original did not fill")
	}
	m := mirrorState(d)
	if m == nil || m.Pos == nil {
		t.Fatal("original filled but its mirror opened nothing — the pair is not an inverse")
	}

	if m.Pos.Entry != o.Pos.Entry {
		t.Errorf("mirror entered at %.2f, original at %.2f — a mirror must inherit the fill, not find its own",
			m.Pos.Entry, o.Pos.Entry)
	}
	if m.Pos.EntryBar != o.Pos.EntryBar {
		t.Errorf("mirror entered on bar %d, original on bar %d", m.Pos.EntryBar, o.Pos.EntryBar)
	}
	if m.Pos.Dir == o.Pos.Dir {
		t.Errorf("both halves are %s — a same-side mirror re-risks the bet instead of inverting it", m.Pos.Dir)
	}
	if m.Pos.Profile != o.Pos.Profile {
		t.Errorf("mirror profile %q != original %q; a different TTL would let them close on different bars",
			m.Pos.Profile, o.Pos.Profile)
	}
	// A mirror never places an order of its own — that is the bug this replaces.
	if m.Pend != nil {
		t.Error("mirror holds a pending order; mirrors must ride the original's fill, never compete for one")
	}

	// The mirror INHERITS the risk distance and targets targetRewardRisk x it.
	//
	// This used to assert the swap — mirror target where the original stops, and
	// vice versa — which made the pair an exact inverse and handed every mirror
	// a 1:0.50 payoff needing a 66.7% win rate to break even. On paper they
	// cleared it at 79.7%; with real money they hit 33.3% and lost $13.91 over
	// 27 fills, gross negative before fees.
	const tol = 1e-6
	oSL := math.Abs(o.Pos.Entry - o.Pos.SL)
	mSL := math.Abs(m.Pos.Entry - m.Pos.SL)
	mTP := math.Abs(m.Pos.TP - m.Pos.Entry)

	if math.Abs(mSL-oSL) > tol {
		t.Errorf("mirror risk %.4f != original risk %.4f — the mirror must inherit the stop distance", mSL, oSL)
	}
	if got := mTP / mSL; math.Abs(got-targetRewardRisk) > 1e-6 {
		t.Errorf("mirror R:R = 1:%.4f, want 1:%.1f", got, targetRewardRisk)
	}
	// The direction must still invert, or it is not a mirror at all.
	if m.Pos.Dir == o.Pos.Dir {
		t.Errorf("mirror direction %s matches the original; a mirror must take the other side", m.Pos.Dir)
	}
	// And the target must sit on the profitable side for the mirror's OWN
	// direction — the failure mode that would put a short's target above entry.
	if m.Pos.Dir == "LONG" && m.Pos.TP <= m.Pos.Entry {
		t.Error("long mirror targets at or below entry")
	}
	if m.Pos.Dir == "SHORT" && m.Pos.TP >= m.Pos.Entry {
		t.Error("short mirror targets at or above entry")
	}
}

// Both sides of a pair must now carry a FAVOURABLE payoff. The old design
// guaranteed one of them a bad one, by construction.
func TestMirrorAndOriginalBothClearTheHouseRatio(t *testing.T) {
	withMirrors(t)
	d, _ := newMirrorDesk(t, scalers.DirectionLong)
	hist := flatBars(30, 100000)
	step(d, hist[len(hist)-1], hist)
	o := origState(d)
	if o == nil || o.Pend == nil {
		t.Fatal("no pending order placed")
	}
	limit := o.Pend.Limit
	step(d, scalers.Candle{
		OpenTime: hist[len(hist)-1].OpenTime.Add(time.Minute),
		Open:     limit, High: limit * 1.0001, Low: limit * 0.9999, Close: limit, Volume: 1,
	}, hist)

	opened := 0
	for _, cs := range d.combos {
		if cs.Pos == nil {
			continue
		}
		risk := math.Abs(cs.Pos.Entry - cs.Pos.SL)
		reward := math.Abs(cs.Pos.TP - cs.Pos.Entry)
		if risk <= 0 {
			t.Fatal("a position was opened with no risk distance")
		}
		if rr := reward / risk; rr < targetRewardRisk-1e-6 {
			t.Errorf("position R:R = 1:%.3f, below the house 1:%.1f — breakeven win rate would be %.1f%%",
				rr, targetRewardRisk, 100/(1+rr))
		}
		opened++
	}
	if opened < 2 {
		t.Fatalf("only %d position(s) open; the assertion above never ran on a pair", opened)
	}
}

// A short original must mirror to a long, with the same swap.
func TestMirrorInvertsShorts(t *testing.T) {
	withMirrors(t)
	d, _ := newMirrorDesk(t, scalers.DirectionShort)
	hist := flatBars(30, 100000)
	step(d, hist[len(hist)-1], hist)
	o := origState(d)
	if o.Pend == nil {
		t.Fatal("no pending order placed")
	}
	limit := o.Pend.Limit
	fill := scalers.Candle{
		OpenTime: hist[len(hist)-1].OpenTime.Add(time.Minute),
		Open:     limit, High: limit * 1.0001, Low: limit * 0.9999, Close: limit,
	}
	step(d, fill, hist)

	m := mirrorState(d)
	if o.Pos == nil || m == nil || m.Pos == nil {
		t.Fatal("short original and its mirror did not both open")
	}
	if o.Pos.Dir != "SHORT" || m.Pos.Dir != "LONG" {
		t.Fatalf("original %s / mirror %s; want SHORT / LONG", o.Pos.Dir, m.Pos.Dir)
	}
	// A short stops ABOVE and targets BELOW; its long mirror must do the reverse.
	if !(o.Pos.SL > o.Pos.Entry && o.Pos.TP < o.Pos.Entry) {
		t.Errorf("short original has stop %.2f / target %.2f around entry %.2f", o.Pos.SL, o.Pos.TP, o.Pos.Entry)
	}
	if !(m.Pos.SL < m.Pos.Entry && m.Pos.TP > m.Pos.Entry) {
		t.Errorf("long mirror has stop %.2f / target %.2f around entry %.2f", m.Pos.SL, m.Pos.TP, m.Pos.Entry)
	}
}

// The pair is NO LONGER an exact inverse, and that is the deliberate trade.
//
// This asserted that one half's gross exactly cancelled the other's, so a pair
// netted only the fees both paid. That property came from swapping the levels,
// and swapping the levels is what gave every mirror a 1:0.50 payoff — an exact
// inverse of a 1:3 strategy IS a 1:0.33 strategy, and nothing with a 1:0.33
// payoff belongs on a live account.
//
// What replaces it: both halves carry the house ratio, and the pair's outcomes
// are independent. The cost is that an ANTI_ strategy's P&L is no longer direct
// evidence about its original — they are two strategies now, not one bet read
// two ways.
func TestPairIsNoLongerAnExactInverse(t *testing.T) {
	withMirrors(t)
	d, _ := newMirrorDesk(t, scalers.DirectionLong)
	hist := flatBars(30, 100000)
	step(d, hist[len(hist)-1], hist)
	// The mirror opens on the original's FILL, so the pending order has to be
	// filled before either position exists.
	o := origState(d)
	if o == nil || o.Pend == nil {
		t.Fatal("no pending order placed; the test never reached the fill path")
	}
	limit := o.Pend.Limit
	step(d, scalers.Candle{
		OpenTime: hist[len(hist)-1].OpenTime.Add(time.Minute),
		Open:     limit, High: limit * 1.0001, Low: limit * 0.9999, Close: limit, Volume: 1,
	}, hist)

	os_, ms := origState(d), mirrorState(d)
	if os_ == nil || ms == nil || os_.Pos == nil || ms.Pos == nil {
		t.Fatal("expected both an original and its mirror to be open")
	}
	orig, mirror := os_.Pos, ms.Pos

	// Entry is still shared — the mirror rides the original's fill.
	if math.Abs(orig.Entry-mirror.Entry) > 1e-9 {
		t.Errorf("entries diverged: original %.6f, mirror %.6f", orig.Entry, mirror.Entry)
	}
	// Direction still inverts.
	if orig.Dir == mirror.Dir {
		t.Errorf("both legs are %s; the mirror must take the other side", orig.Dir)
	}
	// But the levels are NOT reflections of each other any more. If they were,
	// the mirror's reward would equal the original's risk.
	origRisk := math.Abs(orig.Entry - orig.SL)
	mirrorReward := math.Abs(mirror.TP - mirror.Entry)
	if math.Abs(mirrorReward-origRisk) < 1e-9 {
		t.Error("the mirror's target still sits exactly where the original stops — " +
			"the level swap is back, and with it the 1:0.50 payoff")
	}
	// Both must clear the house ratio. Under the old design exactly one of them
	// could, by construction.
	for name, p := range map[string]*position{"original": orig, "mirror": mirror} {
		risk := math.Abs(p.Entry - p.SL)
		reward := math.Abs(p.TP - p.Entry)
		if rr := reward / risk; rr < targetRewardRisk-1e-6 {
			t.Errorf("%s R:R = 1:%.3f, below the house 1:%.1f", name, rr, targetRewardRisk)
		}
	}
}

// A bar that spans BOTH levels resolves both halves on the conservative
// stop-first rule, so the pair loses twice. That is the honest outcome for an
// ambiguous bar; the test exists so the behaviour is a decision rather than a
// surprise, and so nobody "fixes" it into a fabricated inverse later.
func TestPairOnAnAmbiguousBarTakesBothStops(t *testing.T) {
	withMirrors(t)
	d, _ := newMirrorDesk(t, scalers.DirectionLong)
	hist := flatBars(30, 100000)
	step(d, hist[len(hist)-1], hist)
	o := origState(d)
	limit := o.Pend.Limit
	last := hist[len(hist)-1].OpenTime

	step(d, scalers.Candle{OpenTime: last.Add(time.Minute),
		Open: limit, High: limit * 1.0001, Low: limit * 0.9999, Close: limit}, hist)

	// Sweeps far past both the stop and the target.
	step(d, scalers.Candle{OpenTime: last.Add(2 * time.Minute),
		Open: limit, High: limit * 1.10, Low: limit * 0.90, Close: limit}, hist)

	m := mirrorState(d)
	if o.N != 1 || m.N != 1 {
		t.Fatalf("orig closed %d, mirror closed %d; both must resolve on the sweeping bar", o.N, m.N)
	}
	if o.NetSum > 0 || m.NetSum > 0 {
		t.Errorf("stop-first must apply to both halves on an ambiguous bar; got orig %.5f mirror %.5f",
			o.NetSum, m.NetSum)
	}
}

// Over a long run the mirror trades LESS OFTEN than its original, and that is
// expected now rather than a fault.
//
// This used to assert a strict partition: every original trade had exactly one
// mirror trade, and the two split the wins between them. That held only while
// the mirror's exits were reflections of the original's, so both legs always
// closed on the same bar.
//
// The legs now exit independently — the mirror stops at the inherited risk
// distance and targets 3x it — so a mirror can still be holding when its
// original re-signals. The open is skipped, because one position per stream is
// an invariant the original obeys too and stacking would give the mirror
// leverage its source never had.
//
// The cost is real and worth stating: an ANTI_ strategy now trades a subset of
// its original's signals, so their trade counts differ and neither is evidence
// about the other. That is the price of both legs carrying a payoff worth
// taking.
func TestMirrorTradesASubsetOfTheOriginalsSignals(t *testing.T) {
	withMirrors(t)
	d, st := newMirrorDesk(t, scalers.DirectionLong)
	hist := flatBars(30, 100000)
	last := hist[len(hist)-1].OpenTime

	// Re-arm the stub after each close so the stream trades repeatedly, and walk
	// price up and down so both stops and targets get hit.
	px := 100000.0
	for i := 0; i < 400; i++ {
		if origState(d) == nil || (origState(d).Pos == nil && origState(d).Pend == nil) {
			st.fired = false
		}
		// Wider than the old walk on purpose. Targets are now 3x the stop
		// (1.05%-1.80%), and a +-0.09% drift with 0.25% wicks almost never
		// reaches one — every exit would be the time stop and the test would
		// measure nothing about targets at all.
		switch {
		case i%37 < 18:
			px *= 1.004
		default:
			px *= 0.996
		}
		bar := scalers.Candle{
			OpenTime: last.Add(time.Duration(i+1) * time.Minute),
			Open:     px, High: px * 1.008, Low: px * 0.992, Close: px, Volume: 1,
		}
		hist = append(hist, bar)
		step(d, bar, hist)
	}

	o, m := origState(d), mirrorState(d)
	if o == nil || o.N < 20 {
		t.Fatalf("the original closed too few trades for this to mean anything")
	}
	if m == nil || m.N == 0 {
		t.Fatal("the mirror closed no trades at all — it is not trading, not merely trading less")
	}
	// A subset, never a superset: the mirror only opens on the original's fill.
	if m.N > o.N {
		t.Errorf("mirror closed %d trades against the original's %d; it cannot exceed its source", m.N, o.N)
	}
	// And not a vanishing subset. If skips dominated, the mirror would be a
	// live-listed strategy that almost never trades — the silent-zero shape.
	if ratio := float64(m.N) / float64(o.N); ratio < 0.4 {
		t.Errorf("mirror closed only %.0f%% of the original's trades (%d/%d); skips are dominating",
			ratio*100, m.N, o.N)
	}
	// Skips must be COUNTED, not swallowed, so the drift is visible on /health.
	if d.mirrorSkips == 0 && m.N < o.N {
		t.Error("the mirror traded less than its original but no skips were recorded")
	}
}

// The desk must report the mirrors it runs. Enumerating d.entries alone left
// half the desk off its own leaderboard.
func TestStreamNamesIncludesMirrors(t *testing.T) {
	withMirrors(t)
	d, _ := newMirrorDesk(t, scalers.DirectionLong)
	names := d.streamNames()
	if len(names) != 2*len(d.entries) {
		t.Fatalf("streamNames returned %d names for %d strategies; want one mirror each",
			len(names), len(d.entries))
	}
	if names[0] != "TEST_Strat" || names[1] != "ANTI_TEST_Strat" {
		t.Errorf("streamNames = %v; want the original followed by its mirror", names)
	}
}

// Mirroring a mirror would return the original under a confusing name and give
// the same hypothesis two accounts.
func TestMirrorsAreNotThemselvesMirrored(t *testing.T) {
	if got := mirrorOf("ANTI_Foo"); got != "" {
		t.Errorf("mirrorOf(ANTI_Foo) = %q, want \"\"", got)
	}
	if got := mirrorOf("Foo"); got != "ANTI_Foo" {
		t.Errorf("mirrorOf(Foo) = %q, want ANTI_Foo", got)
	}
	if got := scalers.OriginalStrategyName(mirrorOf("Foo")); got != "Foo" {
		t.Errorf("mirror name does not round-trip back to its original: %q", got)
	}
}

// The ANTI_ records already on disk were produced by the signal-inverting
// mirrors, whose fills were selection-biased. Loading them into the new
// mechanism would put two incompatible experiments under one name — the same
// mistake the venue cutover exists to prevent. The originals were never affected
// by that bug, so they must survive: they carry hundreds of trades of progress
// toward the 200-trade gate, and resetting them would be a real cost paid for
// nothing.
func TestLoad_DiscardsOldMirrorRecordsAndKeepsOriginals(t *testing.T) {
	d := newTestDesk(t)
	writeSnapshot(t, d.stateDir, snapshot{
		SavedAt:   time.Now().UTC(),
		DataVenue: currentDataVenue,
		// no MirrorModel — the pre-fix snapshot format
		Combos: map[string]*comboState{
			comboKey("S1", "BTCUSD"):      {NExp: 240, WinsExp: 130, EqExp: 1.2, PeakExp: 1.3},
			comboKey("ANTI_S1", "BTCUSD"): {NExp: 4, WinsExp: 4, EqExp: 1.1, PeakExp: 1.1},
		},
	})
	d.load()

	if _, ok := d.combos[comboKey("ANTI_S1", "BTCUSD")]; ok {
		t.Error("a mirror record from the old fill model was restored; its trades cannot be compared to the new ones")
	}
	o, ok := d.combos[comboKey("S1", "BTCUSD")]
	if !ok {
		t.Fatal("the original's record was discarded; the old mirror bug never touched it")
	}
	if o.N != 240 || o.Wins != 130 {
		t.Errorf("original restored as %d trades / %d wins, want 240/130", o.N, o.Wins)
	}
}

// A snapshot written by this build must round-trip intact — mirrors included.
func TestLoad_KeepsMirrorRecordsFromTheCurrentModel(t *testing.T) {
	d := newTestDesk(t)
	writeSnapshot(t, d.stateDir, snapshot{
		SavedAt: time.Now().UTC(), DataVenue: currentDataVenue, MirrorModel: currentMirrorModel,
		Combos: map[string]*comboState{
			comboKey("ANTI_S1", "BTCUSD"): {NExp: 44, WinsExp: 26, EqExp: 1.05, PeakExp: 1.06},
		},
	})
	d.load()

	m, ok := d.combos[comboKey("ANTI_S1", "BTCUSD")]
	if !ok {
		t.Fatal("a mirror record from the current model was discarded on restart")
	}
	if m.N != 44 || m.Wins != 26 {
		t.Errorf("mirror restored as %d/%d, want 44/26", m.N, m.Wins)
	}
}

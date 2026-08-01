package main

import (
	"math"
	"testing"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

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

	// The swap: the mirror's target sits where the original stops, and vice versa.
	const tol = 1e-6
	oSL := math.Abs(o.Pos.Entry - o.Pos.SL)
	oTP := math.Abs(o.Pos.TP - o.Pos.Entry)
	mSL := math.Abs(m.Pos.Entry - m.Pos.SL)
	mTP := math.Abs(m.Pos.TP - m.Pos.Entry)
	if math.Abs(mTP-oSL) > tol {
		t.Errorf("mirror target distance %.4f != original stop distance %.4f", mTP, oSL)
	}
	if math.Abs(mSL-oTP) > tol {
		t.Errorf("mirror stop distance %.4f != original target distance %.4f", mSL, oTP)
	}
}

// A short original must mirror to a long, with the same swap.
func TestMirrorInvertsShorts(t *testing.T) {
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

// The property the whole exercise exists for: when one half wins, the other
// loses the same gross amount, so the pair nets exactly the fees both paid.
// If this does not hold, an anti-strategy's P&L is not evidence about its
// original and the leaderboard comparison is meaningless.
func TestPairNetsMinusTwoFeeLoads(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  scalers.Direction
		// which way price runs after the fill
		up bool
	}{
		{"long original stopped out", scalers.DirectionLong, false},
		{"long original targeted", scalers.DirectionLong, true},
		{"short original stopped out", scalers.DirectionShort, true},
		{"short original targeted", scalers.DirectionShort, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := newMirrorDesk(t, tc.dir)
			hist := flatBars(30, 100000)
			step(d, hist[len(hist)-1], hist)
			o := origState(d)
			limit := o.Pend.Limit
			last := hist[len(hist)-1].OpenTime

			fill := scalers.Candle{
				OpenTime: last.Add(time.Minute),
				Open:     limit, High: limit * 1.0001, Low: limit * 0.9999, Close: limit,
			}
			step(d, fill, hist)
			if o.Pos == nil {
				t.Fatal("original did not fill")
			}

			// A bar wide enough in ONE direction to resolve both halves: whichever
			// level it reaches, the other half's opposite level sits at the same
			// price. Deliberately not spanning both — that case is covered below.
			var resolve scalers.Candle
			if tc.up {
				resolve = scalers.Candle{OpenTime: last.Add(2 * time.Minute),
					Open: limit, High: limit * 1.05, Low: limit, Close: limit * 1.05}
			} else {
				resolve = scalers.Candle{OpenTime: last.Add(2 * time.Minute),
					Open: limit, High: limit, Low: limit * 0.95, Close: limit * 0.95}
			}
			step(d, resolve, hist)

			m := mirrorState(d)
			if o.Pos != nil || m.Pos == nil && m.N == 0 {
				t.Fatalf("original open=%v mirror closed=%d — the pair did not resolve together", o.Pos != nil, m.N)
			}
			if o.N != 1 || m.N != 1 {
				t.Fatalf("original closed %d trades, mirror %d — a pair must close together", o.N, m.N)
			}
			if (o.NetSum > 0) == (m.NetSum > 0) {
				t.Fatalf("both halves went the same way (orig %.5f, mirror %.5f); one must lose what the other wins",
					o.NetSum, m.NetSum)
			}

			// Gross cancels exactly; what remains is the fees. Each side pays a
			// maker entry plus one exit, so the pair can never net above zero —
			// which is why an anti-strategy only earns when its original has a
			// genuinely negative GROSS edge, not merely a negative net.
			pair := o.NetSum + m.NetSum
			if pair > 0 {
				t.Errorf("pair netted +%.6f; a mirrored pair must always cost fees", pair)
			}
			minCost := 2 * (makerFee + makerFee) // both maker-exit (TP) legs
			maxCost := 2*(makerFee+takerFee) + 2*stopSlip
			if -pair < minCost*0.9 || -pair > maxCost*1.1 {
				t.Errorf("pair cost %.6f, outside the fee band [%.6f, %.6f] — gross did not cancel",
					-pair, minCost, maxCost)
			}
		})
	}
}

// A bar that spans BOTH levels resolves both halves on the conservative
// stop-first rule, so the pair loses twice. That is the honest outcome for an
// ambiguous bar; the test exists so the behaviour is a decision rather than a
// surprise, and so nobody "fixes" it into a fabricated inverse later.
func TestPairOnAnAmbiguousBarTakesBothStops(t *testing.T) {
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

// Every closed original trade must have a mirror trade, and vice versa. This is
// the count that was 35-of-53 wrong before, expressed as an invariant.
func TestNoUnpairedTradesOverALongRun(t *testing.T) {
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
		// A slow drift with periodic sharp reversals.
		switch {
		case i%37 < 18:
			px *= 1.0009
		default:
			px *= 0.9991
		}
		bar := scalers.Candle{
			OpenTime: last.Add(time.Duration(i+1) * time.Minute),
			Open:     px, High: px * 1.0025, Low: px * 0.9975, Close: px, Volume: 1,
		}
		step(d, bar, hist)
	}

	o, m := origState(d), mirrorState(d)
	// A floor, not just "> 0": this run closes ~44 trades, and a change that
	// quietly drops it to one or two would leave the invariants below technically
	// true and worthless.
	if o == nil || o.N < 20 {
		t.Fatalf("the original closed %d trades; too few for the pairing invariants to mean anything",
			func() int {
				if o == nil {
					return 0
				}
				return o.N
			}())
	}
	if m == nil {
		t.Fatal("no mirror account exists after the original traded")
	}
	if o.N != m.N {
		t.Errorf("original closed %d trades, mirror closed %d — %d unpaired",
			o.N, m.N, int(math.Abs(float64(o.N-m.N))))
	}
	if d.mirrorSkips != 0 {
		t.Errorf("%d mirror opens were skipped; the pair drifted out of step", d.mirrorSkips)
	}
	if d.mirrorOpens != int64(m.N) {
		t.Errorf("opened %d mirrors but closed %d", d.mirrorOpens, m.N)
	}
	// Wins must be complementary: a mirror wins exactly when its original loses.
	if o.Wins+m.Wins != o.N {
		t.Errorf("original won %d and mirror won %d out of %d paired trades; the two must partition the outcomes",
			o.Wins, m.Wins, o.N)
	}
}

// The desk must report the mirrors it runs. Enumerating d.entries alone left
// half the desk off its own leaderboard.
func TestStreamNamesIncludesMirrors(t *testing.T) {
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

package main

import (
	"math"
	"time"

	scalpers "antigravity-engine/internal/strategy/scalpers"
)

// roundTripCostPct is Delta's taker cost for entering and exiting, in percent.
//
// The same 0.118% the live pack prices against. Charged on every simulated
// trade without exception: an uncosted backtest of a strategy whose edge is
// smaller than its fees reports the edge and hides the fees, and this desk has
// already produced 7,000 trades that looked flat gross and lost net.
const roundTripCostPct = 0.118

// minRewardRisk is the eligibility bar the owner set: 1:6.
//
// An ELIGIBILITY filter, not a geometry setting, and the distinction decides
// whether it can work at all. Measured over 401 live trades, moving the target
// from 1:3 to 1:6 left expectancy unchanged (-0.354% -> -0.372%): a further
// target is reached proportionally less often, so the payoff rises and the hit
// rate falls by the same factor. Geometry redistributes outcomes; it cannot
// create them.
//
// What CAN change expectancy is trading fewer setups. Here 1:6 rejects any
// signal whose own structural target does not already sit six stops away — the
// strategy's levels are left exactly as it drew them. That selects a different
// population of trades rather than re-labelling the same one, which is the only
// version of this rule that could help. Whether it does is measured rather than
// assumed: every rejected signal is counted so the two populations can be
// compared afterwards, which is what the -min-rr flag is for. Setting it to 0
// runs the identical strategies at their own native targets, and the two runs
// side by side are the only way to tell a filter from a veto.
var minRewardRisk = 6.0

// maxBarsHeld caps a simulated hold at 200 bars of its own timeframe.
//
// Without a cap a losing trade whose stop is never touched stays open to the
// end of the series and quietly becomes an unrealised number no live desk would
// still be holding. 200 bars is long enough not to truncate a working trend
// trade and short enough that the trade resolves inside the window it opened in.
const maxBarsHeld = 200

// Trade is one completed simulated round trip.
type Trade struct {
	Strategy  string    `json:"strategy"`
	Symbol    string    `json:"symbol"`
	TF        string    `json:"tf"`
	Long      bool      `json:"long"`
	OpenedAt  time.Time `json:"opened_at"`
	ClosedAt  time.Time `json:"closed_at"`
	Entry     float64   `json:"entry"`
	Stop      float64   `json:"stop"`
	Target    float64   `json:"target"`
	Exit      float64   `json:"exit"`
	Reason    string    `json:"reason"`
	RR        float64   `json:"rr"`
	GrossPct  float64   `json:"gross_pct"`
	NetPct    float64   `json:"net_pct"`
	RMultiple float64   `json:"r_multiple"`
	BarsHeld  int       `json:"bars_held"`
}

// SimResult is one strategy run over one symbol.
type SimResult struct {
	Strategy string
	Symbol   string
	TF       string
	Trades   []Trade

	// Signals that fired but were refused, by cause.
	//
	// Kept because a strategy that produces 900 signals and takes 3 is a very
	// different object from one that produces 3, and a leaderboard built on
	// trades alone cannot tell them apart. The 1:6 rule is judged on this
	// number as much as on the trades it allowed through.
	Signals     int
	RejectedRR  int
	RejectedBad int
}

// ctxFor builds a MarketContext holding ONLY bars closed at or before i.
//
// The validity of this whole harness rests on this function. The slice is
// c[:i+1] — bar i is closed and visible, bar i+1 does not exist yet — and only
// the strategy's own timeframe is populated, since that is the only series
// mtfStrategy reads. Handing it the full series and trusting it to "use what it
// needs" is how a backtest comes to read tomorrow's candle.
func ctxFor(tf scalpers.HigherTF, c []scalpers.Candle, i int) scalpers.MarketContext {
	visible := c[:i+1]
	ctx := scalpers.MarketContext{Price: c[i].Close}
	switch tf {
	case scalpers.TF1m:
		ctx.Candles1m = visible
	case scalpers.TF5m:
		ctx.Candles5m = visible
	case scalpers.TF10m:
		ctx.Candles10m = visible
	case scalpers.TF15m:
		ctx.Candles15m = visible
	case scalpers.TF30m:
		ctx.Candles30m = visible
	case scalpers.TF45m:
		ctx.Candles45m = visible
	case scalpers.TF1h:
		ctx.Candles1h = visible
	case scalpers.TF4h:
		ctx.Candles4h = visible
	case scalpers.TF1d:
		ctx.Candles1d = visible
	}
	return ctx
}

// Simulate walks one strategy over one symbol's series on its own timeframe.
func Simulate(st scalpers.Strategy, name, symbol string, tf scalpers.HigherTF, c []scalpers.Candle) SimResult {
	res := SimResult{Strategy: name, Symbol: symbol, TF: string(tf)}
	minBars := tf.MinCandles()
	if len(c) < minBars+2 {
		return res
	}

	nextFree := 0 // index before which we are still holding
	for i := minBars; i < len(c)-1; i++ {
		if i < nextFree {
			// One position at a time per stream, exactly as the live desk
			// enforces. Without it a strategy that signals on every bar of a
			// trend books the same move twenty times and reports twenty
			// independent wins.
			continue
		}
		sig := st.Evaluate(ctxFor(tf, c, i))
		if sig.Direction == scalpers.DirectionNone {
			continue
		}
		res.Signals++
		long := sig.Direction == scalpers.DirectionLong

		// Entry at the NEXT bar's open. Filling at the close of the bar that
		// produced the signal assumes the decision and the fill happened at the
		// same instant on the same price — free money that does not exist.
		entry := c[i+1].Open
		stop, target := sig.StopLoss, sig.TakeProfit
		if stop <= 0 || target <= 0 || entry <= 0 {
			res.RejectedBad++
			continue
		}
		// The levels must still bracket the entry after slipping to the next
		// open. A gap through the stop turns a long into an instant loss with
		// an inverted stop, which is not a trade any desk would take.
		if (long && (stop >= entry || target <= entry)) || (!long && (stop <= entry || target >= entry)) {
			res.RejectedBad++
			continue
		}
		risk := math.Abs(entry - stop)
		if risk <= 0 {
			res.RejectedBad++
			continue
		}
		rr := math.Abs(target-entry) / risk
		if rr < minRewardRisk {
			res.RejectedRR++
			continue
		}

		tr, closedIdx := walk(c, i+1, entry, stop, target, long)
		tr.Strategy, tr.Symbol, tr.TF, tr.Long, tr.RR = name, symbol, string(tf), long, rr
		res.Trades = append(res.Trades, tr)
		nextFree = closedIdx + 1
	}
	return res
}

// walk resolves an open position bar by bar and returns the completed trade.
//
// When one bar contains BOTH the stop and the target, the STOP is taken. A bar
// is a summary, not a path: nothing in OHLC says which extreme came first, and
// choosing the target would turn every ambiguous bar into a 6R win. At a 1:6
// target the ambiguous bars are exactly the wide ones, so this assumption is
// load-bearing here in a way it would not be at 1:1 — the pessimistic reading
// is the only one that cannot flatter the result.
func walk(c []scalpers.Candle, from int, entry, stop, target float64, long bool) (Trade, int) {
	tr := Trade{OpenedAt: c[from].OpenTime, Entry: entry, Stop: stop, Target: target}
	last := from + maxBarsHeld
	if last > len(c)-1 {
		last = len(c) - 1
	}
	for j := from; j <= last; j++ {
		bar := c[j]
		hitStop := (long && bar.Low <= stop) || (!long && bar.High >= stop)
		hitTarget := (long && bar.High >= target) || (!long && bar.Low <= target)
		switch {
		case hitStop:
			tr.Exit, tr.Reason = stop, "SL"
		case hitTarget:
			tr.Exit, tr.Reason = target, "TP"
		default:
			continue
		}
		tr.ClosedAt, tr.BarsHeld = bar.OpenTime, j-from+1
		return finish(tr, long), j
	}
	tr.Exit, tr.Reason = c[last].Close, "TTL"
	tr.ClosedAt, tr.BarsHeld = c[last].OpenTime, last-from+1
	return finish(tr, long), last
}

func finish(tr Trade, long bool) Trade {
	gross := (tr.Exit - tr.Entry) / tr.Entry * 100
	if !long {
		gross = -gross
	}
	tr.GrossPct = gross
	tr.NetPct = gross - roundTripCostPct
	if riskPct := math.Abs(tr.Entry-tr.Stop) / tr.Entry * 100; riskPct > 0 {
		// R is measured NET. Computed on gross it reports a 6R winner that
		// actually netted 5.4R, and that difference is the entire margin on a
		// strategy whose edge is thin.
		tr.RMultiple = tr.NetPct / riskPct
	}
	return tr
}

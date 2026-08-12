package delta

import (
	"fmt"
	"math"
)

// minEntryStopTicks is how many ticks wide a stop must be to be worth trading.
//
// The bracket layer only asks for 2 ticks — enough to EXPRESS the stop. That is
// a different question from whether the stop can survive contact with the
// grid, and the gap between the two is where the losses came from.
//
// COOKIEUSD is the case. It ticks at 1e-05 against a ~0.0125 price, so one tick
// is 0.08% and a 0.64% stop is 8 ticks wide. Every rounding, trigger and fill
// is quantised into steps worth an eighth of the risk budget, which is why its
// stop-outs ran 1.5x to 1.75x and one could not be bracketed at all.
//
// 20 ticks puts any single quantisation step under 5% of the planned risk. It
// is a rule about the instrument rather than a list of banned symbols, so a new
// symbol with the same problem is excluded without anyone noticing it arrived.
const minEntryStopTicks = 20

// gridAutoDisableAfter is how many CONSECUTIVE grid refusals switch a stream
// off automatically.
//
// Not one. Stops are volatility-scaled, so the same stream clears the tick grid
// in a moving market and fails in a quiet one: measured live, both streams that
// had ever been refused had also traded, and one was net positive. Five in a
// row without a single order reaching sizing is a stream the grid genuinely
// cannot hold, not a quiet patch.
const gridAutoDisableAfter = 5

// stopSurvivesGrid reports why a stop cannot be traded on this symbol's price
// grid, or "" when it can.
//
// Returning a REASON rather than a bool: these refusals are logged per signal,
// and "refused" without a cause is how a desk goes quiet for a week before
// anyone works out why.
func stopSurvivesGrid(reg *PerpRegistry, symbol string, entry, stop float64) string {
	_, reason := stopGridTicks(reg, symbol, entry, stop)
	return reason
}

// stopGridTicks returns the stop's width in ticks and, if it is too narrow, the
// reason it cannot be traded.
//
// The width is returned even when the stop passes, so the desk can report the
// margin a stream is operating on rather than only a pass/fail.
func stopGridTicks(reg *PerpRegistry, symbol string, entry, stop float64) (float64, string) {
	if reg == nil || entry <= 0 || stop <= 0 {
		return 0, ""
	}
	prod, ok := reg.Lookup(symbol)
	if !ok || prod.TickSize <= 0 {
		// Unknown grid: permit. Refusing every order because a registry field is
		// missing is a worse failure than the one being prevented, and it would
		// present as the desk silently declining to trade.
		return 0, ""
	}
	ticks := math.Abs(entry-stop) / prod.TickSize
	if ticks < minEntryStopTicks {
		return ticks, fmt.Sprintf(
			"stop is %.1f ticks wide on %s (tick %g); under %d ticks the grid moves price in steps worth more than %.0f%% of the planned risk",
			ticks, symbol, prod.TickSize, minEntryStopTicks, 100/float64(minEntryStopTicks))
	}
	return ticks, ""
}

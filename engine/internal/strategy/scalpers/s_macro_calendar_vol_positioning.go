package scalpers

import (
	"fmt"
	"math"
	"time"
)

// S28 — Macro Calendar Vol Positioning
//
// Regime:     RANGING (pre-event behavior), VOLATILE (post-event behavior) —
//             both are returned from ValidRegimes() since this strategy
//             internally branches behavior based on time-to-event rather than
//             being gated to a single regime.
// Timeframes: 1h/15m + hardcoded macro calendar (see macroCalendarEvents below)
//
// Background: FOMC rate decisions and CPI prints are well-documented
// volatility catalysts for BTC (and risk assets broadly) — pre-event
// positioning tends to compress (de-risking into the print) while
// post-event moves are often choppy/whipsaw in the first few minutes before
// a cleaner directional break emerges. This strategy reduces entries in the
// pre-event window and waits out the initial post-event whipsaw.
//
// NEEDS PERIODIC MANUAL UPDATE — macroCalendarEvents below covers 2026 only.
// Add 2027 FOMC/CPI dates before year-end 2026, or this strategy silently
// stops finding any nearby events and will simply never fire (safe failure
// mode, not a crash, but it goes quiet).
//
// FOMC schedule assumption: ~8 meetings/year on the standard Fed cadence
// (roughly every 6-7 weeks), decision announced 14:00 ET / 18:00 or 19:00 UTC
// (UTC offset shifts with US DST — dates below use the prevailing UTC time
// for each meeting's calendar position). CPI prints are typically the
// 10th-15th of each month, 08:30 ET / 12:30 or 13:30 UTC. These are
// best-effort dates from general FOMC/BLS cadence knowledge, NOT pulled from
// a live calendar feed — treat as approximate and verify against the
// official FOMC/BLS calendars before relying on this in any live-capital
// context.
type macroEvent struct {
	Name string
	Time time.Time // UTC
}

var macroCalendarEvents = []macroEvent{
	{"FOMC", time.Date(2026, 1, 28, 19, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 2, 11, 13, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 3, 18, 18, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 3, 12, 12, 30, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 4, 14, 12, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 4, 29, 18, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 5, 12, 12, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 6, 17, 18, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 6, 11, 12, 30, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 7, 29, 18, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 8, 12, 12, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 9, 16, 18, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 9, 11, 12, 30, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 10, 13, 12, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 10, 28, 18, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 11, 12, 13, 30, 0, 0, time.UTC)},
	{"FOMC", time.Date(2026, 12, 16, 19, 0, 0, 0, time.UTC)},
	{"CPI", time.Date(2026, 12, 10, 13, 30, 0, 0, time.UTC)},
}

// nearestMacroEvent returns the event closest to now (by absolute distance),
// along with the signed duration until it (negative if it already occurred).
func nearestMacroEvent(now time.Time) (macroEvent, time.Duration, bool) {
	var best macroEvent
	var bestAbs time.Duration = -1
	for _, e := range macroCalendarEvents {
		d := e.Time.Sub(now)
		abs := d
		if abs < 0 {
			abs = -abs
		}
		if bestAbs == -1 || abs < bestAbs {
			bestAbs = abs
			best = e
		}
	}
	if bestAbs == -1 {
		return macroEvent{}, 0, false
	}
	return best, best.Time.Sub(now), true
}

type MacroCalendarVolPositioning struct{}

func (s *MacroCalendarVolPositioning) Name() string { return "Macro_Calendar_Vol_Positioning" }

func (s *MacroCalendarVolPositioning) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

func (s *MacroCalendarVolPositioning) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	event, until, ok := nearestMacroEvent(ctx.Now)
	if !ok {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	preEventWindow := until > 0 && until <= 4*time.Hour && until >= 2*time.Hour
	postEventWindow := until < 0 && -until <= 2*time.Hour

	if preEventWindow && ctx.Regime == RegimeRanging {
		// Pre-event: reduce entries / widen quality gates. Only fire on an
		// exceptionally clean mean-reversion setup at a statistical extreme,
		// with a heavily reduced confidence multiplier.
		c15 := ctx.Candles15m
		bb := BB(c15, 20)
		if bb.Upper == 0 {
			return NoSignal(name)
		}
		if price >= bb.Upper {
			sl := price + math.Max(1.0*atr15m, price*0.003)
			risk := sl - price
			tp := price - 2.0*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: 0.45, // heavily reduced — pre-event caution
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"pre-event (%s in %s) caution window: price at upper BB extreme=%.0f, "+
						"reduced-confidence fade only", event.Name, until.Round(time.Minute), bb.Upper,
				),
			}
		}
		if price <= bb.Lower {
			sl := price - math.Max(1.0*atr15m, price*0.003)
			risk := price - sl
			tp := price + 2.0*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.45,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"pre-event (%s in %s) caution window: price at lower BB extreme=%.0f, "+
						"reduced-confidence fade only", event.Name, until.Round(time.Minute), bb.Lower,
				),
			}
		}
		return NoSignal(name) // no exceptionally clean setup — stay flat pre-event
	}

	if postEventWindow && ctx.Regime == RegimeVolatile {
		// Post-event: skip the first 5-15min whipsaw, then require the FIRST
		// clean directional break with volume-surge confirmation.
		sinceEvent := -until
		if sinceEvent < 15*time.Minute {
			return NoSignal(name) // still inside the whipsaw exclusion window
		}

		c15 := ctx.Candles15m
		n := len(c15)
		if n < 5 {
			return NoSignal(name)
		}
		avgVol := AvgVolume(c15, 20)
		lastVol := c15[n-1].Volume
		volumeSurge := avgVol > 0 && lastVol > avgVol*1.5
		if !volumeSurge {
			return NoSignal(name)
		}

		swingHigh := SwingHigh(c15[:n-1], 8)
		swingLow := SwingLow(c15[:n-1], 8)
		last := c15[n-1]

		if swingHigh > 0 && last.Close > swingHigh {
			sl := price - math.Max(1.0*atr15m, price*0.003)
			risk := price - sl
			tp := price + 2.0*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: 0.7,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"post-event (%s, %s ago) first clean break ABOVE swing high=%.0f "+
						"with volume surge (%.0f vs avg %.0f)",
					event.Name, sinceEvent.Round(time.Minute), swingHigh, lastVol, avgVol,
				),
			}
		}
		if swingLow > 0 && last.Close < swingLow {
			sl := price + math.Max(1.0*atr15m, price*0.003)
			risk := sl - price
			tp := price - 2.0*risk
			if risk <= 0 {
				return NoSignal(name)
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: 0.7,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"post-event (%s, %s ago) first clean break BELOW swing low=%.0f "+
						"with volume surge (%.0f vs avg %.0f)",
					event.Name, sinceEvent.Round(time.Minute), swingLow, lastVol, avgVol,
				),
			}
		}
		return NoSignal(name)
	}

	return NoSignal(name)
}

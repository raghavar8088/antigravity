package scalpers

import "time"

// HigherTF is the set of candle series a strategy may reason over.
//
// Kept as one type so a strategy declares the timeframe it trades and the
// engine supplies exactly that, rather than every strategy reaching into
// MarketContext and silently getting nil for a series nobody populated. That is
// how Candles4h came to exist on the context, be read by nothing, and be filled
// by nobody — a 4h strategy would have evaluated against an empty slice and
// never fired, which looks identical to a strategy that simply found no setups.
type HigherTF string

const (
	TF1m  HigherTF = "1m"
	TF5m  HigherTF = "5m"
	TF10m HigherTF = "10m"
	TF15m HigherTF = "15m"
	TF30m HigherTF = "30m"
	TF1h  HigherTF = "1h"
	TF4h  HigherTF = "4h"
	TF1d  HigherTF = "1d"
)

// Step is the candle duration for a timeframe.
func (t HigherTF) Step() time.Duration {
	switch t {
	case TF1m:
		return time.Minute
	case TF5m:
		return 5 * time.Minute
	case TF10m:
		return 10 * time.Minute
	case TF15m:
		return 15 * time.Minute
	case TF30m:
		return 30 * time.Minute
	case TF1h:
		return time.Hour
	case TF4h:
		return 4 * time.Hour
	case TF1d:
		return 24 * time.Hour
	}
	return 0
}

// MinCandles is how many closed candles a strategy on this timeframe needs
// before its indicators mean anything.
//
// Indicators have warm-up: a 50-period moving average computed over 20 candles
// is not a slow average, it is a fast one wearing the wrong name. Every pack
// built on these timeframes must refuse to signal below this count rather than
// emit a confident number from a short window.
func (t HigherTF) MinCandles() int {
	// 120 on EVERY timeframe, set by the SLOWEST indicator in the packs rather
	// than by what feels reasonable per timeframe.
	//
	// The longest warm-up is EMA55, which mtfEMA seeds with an SMA and needs
	// 2n = 110 candles for. The earlier values here — 100 for 1h, 80 for 4h, 60
	// for 1d — sat BELOW that, so every EMA55 family on those timeframes would
	// have had ok=false forever and returned no signal. Silent, and identical in
	// appearance to a strategy that simply found no setups: exactly the failure
	// that left Candles4h nil and unread for weeks.
	//
	// The cost is honest: 4h needs 20 days of history and 1d needs 120 days
	// before they can trade. A strategy that cannot compute its own indicators
	// is not ready, and saying so late is better than signalling early on a
	// half-warmed average.
	return 120
}

// CandlesFor returns the series for this timeframe, and whether enough of it
// exists to evaluate.
//
// Returning the sufficiency flag rather than a bare slice forces the caller to
// handle the short-history case. A strategy that reads the slice and proceeds
// regardless produces its most confident signals when it knows least, because
// early candles carry the largest indicator error.
func (t HigherTF) CandlesFor(ctx MarketContext) ([]Candle, bool) {
	var c []Candle
	switch t {
	case TF1m:
		c = ctx.Candles1m
	case TF5m:
		c = ctx.Candles5m
	case TF10m:
		c = ctx.Candles10m
	case TF15m:
		c = ctx.Candles15m
	case TF30m:
		c = ctx.Candles30m
	case TF1h:
		c = ctx.Candles1h
	case TF4h:
		c = ctx.Candles4h
	case TF1d:
		c = ctx.Candles1d
	}
	return c, len(c) >= t.MinCandles()
}

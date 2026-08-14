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
	TF15m HigherTF = "15m"
	TF30m HigherTF = "30m"
	TF1h  HigherTF = "1h"
	TF4h  HigherTF = "4h"
	TF1d  HigherTF = "1d"
)

// Step is the candle duration for a timeframe.
func (t HigherTF) Step() time.Duration {
	switch t {
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
	switch t {
	case TF15m, TF30m:
		return 120
	case TF1h:
		return 100
	case TF4h:
		return 80
	case TF1d:
		return 60
	}
	return 0
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

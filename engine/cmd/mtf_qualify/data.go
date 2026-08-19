package main

import (
	"fmt"
	"sort"
	"time"

	"antigravity-engine/internal/marketdata"
	scalpers "antigravity-engine/internal/strategy/scalpers"
)

// deltaResolution maps a pack timeframe to the venue's own resolution string,
// or "" when the venue does not serve it and it must be resampled.
//
// 10m and 45m are NOT venue resolutions. Delta serves 1m/3m/5m/15m/30m/1h/2h/
// 4h/6h/1d, so those two are built from 5m and 15m respectively. This is worth
// stating rather than hiding: a resampled series is not identical to a native
// one — its opens and closes fall on boundaries the venue never quoted — and
// two of the nine timeframes the pack claims to trade are therefore a
// construction rather than a measurement.
func deltaResolution(tf scalpers.HigherTF) string {
	switch tf {
	case scalpers.TF1m:
		return "1m"
	case scalpers.TF5m:
		return "5m"
	case scalpers.TF15m:
		return "15m"
	case scalpers.TF30m:
		return "30m"
	case scalpers.TF1h:
		return "1h"
	case scalpers.TF4h:
		return "4h"
	case scalpers.TF1d:
		return "1d"
	}
	return "" // 10m, 45m
}

// resampleFrom names the timeframe a non-native one is built out of, and how
// many of those bars make one.
func resampleFrom(tf scalpers.HigherTF) (scalpers.HigherTF, int, bool) {
	switch tf {
	case scalpers.TF10m:
		return scalpers.TF5m, 2, true
	case scalpers.TF45m:
		return scalpers.TF15m, 3, true
	}
	return "", 0, false
}

// SymbolData holds one symbol's candles on every timeframe the pack uses.
type SymbolData struct {
	Symbol string
	Series map[scalpers.HigherTF][]scalpers.Candle
}

// Load fetches every timeframe for one symbol.
//
// Fetched per timeframe rather than resampled from 1m throughout. Resampling
// everything from one minute is tempting and wrong here: a 1d bar built from
// 1,440 minute bars inherits every gap and every missing minute in them, and
// the venue's own daily bar does not. Where the venue has the series, its
// version is the one the strategy would have seen.
func Load(symbol string, from, to time.Time, tfs []scalpers.HigherTF) (*SymbolData, error) {
	d := &SymbolData{Symbol: symbol, Series: map[scalpers.HigherTF][]scalpers.Candle{}}

	// Native series first, so the resampled ones have their source available.
	for _, tf := range tfs {
		res := deltaResolution(tf)
		if res == "" {
			continue
		}
		raw, err := marketdata.FetchDeltaHistoricalCandles(symbol, res, from, to)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", symbol, res, err)
		}
		d.Series[tf] = toCandles(raw)
	}

	for _, tf := range tfs {
		src, n, ok := resampleFrom(tf)
		if !ok {
			continue
		}
		base := d.Series[src]
		if len(base) == 0 {
			// Fetch the source even if the caller did not ask for it.
			raw, err := marketdata.FetchDeltaHistoricalCandles(symbol, deltaResolution(src), from, to)
			if err != nil {
				return nil, fmt.Errorf("%s %s (source for %s): %w", symbol, src, tf, err)
			}
			base = toCandles(raw)
		}
		d.Series[tf] = resample(base, n)
	}
	return d, nil
}

func toCandles(raw []marketdata.HistoricalCandle) []scalpers.Candle {
	out := make([]scalpers.Candle, 0, len(raw))
	for _, c := range raw {
		// A zero-priced bar is not a quiet market, it is a bad row. Letting one
		// through moves every average through the floor, and nothing errors.
		if c.Close <= 0 || c.High <= 0 || c.Low <= 0 || c.Open <= 0 {
			continue
		}
		out = append(out, scalpers.Candle{
			OpenTime: c.OpenTime.UTC(),
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   c.Volume,
		})
	}
	// Oldest first. The venue returns newest-first and every indicator in the
	// pack indexes the LAST element as the most recent, so venue order makes
	// each one read the past as the present — silently.
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime.Before(out[j].OpenTime) })
	return out
}

// resample aggregates n consecutive bars into one.
//
// Only WHOLE groups are emitted. A trailing partial group is dropped rather
// than published as a finished bar: a half-formed 45m candle looks exactly like
// a complete one to every strategy that reads it, and that is lookahead of the
// worst kind — the strategy sees a bar the market has not finished printing.
func resample(src []scalpers.Candle, n int) []scalpers.Candle {
	if n <= 1 || len(src) < n {
		return nil
	}
	out := make([]scalpers.Candle, 0, len(src)/n)
	for i := 0; i+n <= len(src); i += n {
		grp := src[i : i+n]
		bar := scalpers.Candle{
			OpenTime: grp[0].OpenTime,
			Open:     grp[0].Open,
			High:     grp[0].High,
			Low:      grp[0].Low,
			Close:    grp[len(grp)-1].Close,
		}
		for _, c := range grp {
			if c.High > bar.High {
				bar.High = c.High
			}
			if c.Low < bar.Low {
				bar.Low = c.Low
			}
			bar.Volume += c.Volume
		}
		out = append(out, bar)
	}
	return out
}

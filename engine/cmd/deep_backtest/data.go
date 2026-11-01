package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// series is one timeframe of UTC-aligned candles plus their open times (unix s).
type series struct {
	step int64 // bar length in seconds
	c    []scalers.Candle
	t    []int64
}

// closedBy returns how many bars have fully closed by unix time T.
func (s *series) closedBy(T int64) int {
	return sort.Search(len(s.t), func(i int) bool { return s.t[i]+s.step > T })
}

type fundingPrint struct {
	t    int64
	rate float64
}

// dataset is the deep history: 1m bars from the CSV and every higher timeframe
// resampled from them on UTC boundaries, so all timeframes agree by construction.
type dataset struct {
	m1, m5, m15, h1, h4 *series
	funding             []fundingPrint
	// cvdCum[i] = signed taker volume (taker buy - taker sell, BTC) summed over
	// 1m bars 0..i-1: the live desk's aggTrade CVD, reconstructed.
	cvdCum []float64
}

// loadDeep reads BTCUSDT_1m.csv (time,open,high,low,close,volume,taker_buy) and
// BTCUSDT_funding.csv (time,rate) written by research/btc_deep_2026_10/deep_fetch.py.
func loadDeep(dir string) (*dataset, error) {
	f, err := os.Open(dir + "/BTCUSDT_1m.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m1 := &series{step: 60, c: make([]scalers.Candle, 0, 4_900_000), t: make([]int64, 0, 4_900_000)}
	cum := make([]float64, 1, 4_900_001)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	sc.Scan() // header
	for sc.Scan() {
		p := strings.Split(sc.Text(), ",")
		if len(p) < 7 {
			return nil, fmt.Errorf("1m csv row %q: want 7 columns", sc.Text())
		}
		t, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("1m csv time %q: %w", p[0], err)
		}
		v := [6]float64{}
		for k := 0; k < 6; k++ {
			if v[k], err = strconv.ParseFloat(p[k+1], 64); err != nil {
				return nil, fmt.Errorf("1m csv %q: %w", sc.Text(), err)
			}
		}
		m1.t = append(m1.t, t)
		m1.c = append(m1.c, scalers.Candle{OpenTime: time.Unix(t, 0).UTC(),
			Open: v[0], High: v[1], Low: v[2], Close: v[3], Volume: v[4]})
		cum = append(cum, cum[len(cum)-1]+2*v[5]-v[4])
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	ds := &dataset{m1: m1, cvdCum: cum}
	ds.m5 = resample(m1, 300)
	ds.m15 = resample(m1, 900)
	ds.h1 = resample(m1, 3600)
	ds.h4 = resample(m1, 14400)

	ff, err := os.Open(dir + "/BTCUSDT_funding.csv")
	if err != nil {
		return nil, err
	}
	defer ff.Close()
	fs := bufio.NewScanner(ff)
	fs.Scan()
	for fs.Scan() {
		p := strings.Split(fs.Text(), ",")
		t, err1 := strconv.ParseInt(p[0], 10, 64)
		r, err2 := strconv.ParseFloat(p[1], 64)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("funding csv %q", fs.Text())
		}
		ds.funding = append(ds.funding, fundingPrint{t, r})
	}
	return ds, fs.Err()
}

// resample aggregates 1m bars into UTC-aligned buckets of step seconds. A bucket
// with missing minutes (an exchange outage) still becomes one bar of what traded.
func resample(m1 *series, step int64) *series {
	out := &series{step: step}
	for i := 0; i < len(m1.c); {
		b := m1.t[i] / step * step
		c := m1.c[i]
		bar := scalers.Candle{OpenTime: time.Unix(b, 0).UTC(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume}
		j := i + 1
		for ; j < len(m1.c) && m1.t[j]/step*step == b; j++ {
			x := m1.c[j]
			if x.High > bar.High {
				bar.High = x.High
			}
			if x.Low < bar.Low {
				bar.Low = x.Low
			}
			bar.Close = x.Close
			bar.Volume += x.Volume
		}
		out.c = append(out.c, bar)
		out.t = append(out.t, b)
		i = j
	}
	return out
}

// fundingAt returns the last print at or before T and up to 3 prints of history.
func (ds *dataset) fundingAt(T int64) (float64, []float64) {
	i := sort.Search(len(ds.funding), func(i int) bool { return ds.funding[i].t > T }) - 1
	if i < 0 || T-ds.funding[i].t > 9*3600 { // no print in the last 8h settlement: no data
		return 0, nil
	}
	h := make([]float64, 0, 3)
	for k := i - 2; k <= i; k++ {
		if k >= 0 {
			h = append(h, ds.funding[k].rate)
		}
	}
	return ds.funding[i].rate, h
}

// settlementRate is the funding rate charged at settlement time s (00/08/16 UTC):
// the recorded print within 4h, else the engine's synthetic 0.01% (v3
// FundingSimulator.rateAt), which covers 2017-2019 and Oct 2026.
func (ds *dataset) settlementRate(s int64) (float64, bool) {
	i := sort.Search(len(ds.funding), func(i int) bool { return ds.funding[i].t >= s-4*3600 })
	if i < len(ds.funding) && ds.funding[i].t < s+4*3600 {
		return ds.funding[i].rate, true
	}
	return 0.0001, false
}

// ── engine-mode adapters: the existing marketdata types ──────────────────────

func toHistorical(s *series, from, to int64) []marketdata.HistoricalCandle {
	lo := sort.Search(len(s.t), func(i int) bool { return s.t[i] >= from })
	hi := sort.Search(len(s.t), func(i int) bool { return s.t[i] > to })
	out := make([]marketdata.HistoricalCandle, 0, hi-lo)
	d := time.Duration(s.step)*time.Second - time.Millisecond
	for _, c := range s.c[lo:hi] {
		out = append(out, marketdata.HistoricalCandle{OpenTime: c.OpenTime, Open: c.Open, High: c.High,
			Low: c.Low, Close: c.Close, Volume: c.Volume, CloseTime: c.OpenTime.Add(d)})
	}
	return out
}

func (ds *dataset) toMTF(from, to time.Time) marketdata.MTFDataset {
	f, t := from.Unix(), to.Unix()
	out := marketdata.MTFDataset{Symbol: "BTCUSDT", From: from, To: to,
		Candles1m: toHistorical(ds.m1, f, t), Candles5m: toHistorical(ds.m5, f, t),
		Candles15m: toHistorical(ds.m15, f, t), Candles1h: toHistorical(ds.h1, f, t),
		Candles4h: toHistorical(ds.h4, f, t)}
	for _, p := range ds.funding {
		if p.t >= f && p.t <= t {
			out.FundingRates = append(out.FundingRates, marketdata.HistoricalFundingEntry{Time: time.Unix(p.t, 0).UTC(), Rate: p.rate})
		}
	}
	return out
}

package delta

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

// volStopMultiple is how many p90 one-minute ranges the stop must clear.
//
// A stop inside the noise is not a stop, it is a coin toss. Measured live: a
// 0.60% stop on TSTUSD, whose median MINUTE range is 1.13% — price crossed the
// entire stop distance twice over in an ordinary minute, so 9 of 9 trades
// closed SL at a median hold of 130 seconds and not one reached a third of its
// target.
//
// At 2x the 90th percentile, a single minute clears the stop about one time in
// a hundred rather than most minutes. The trade then resolves on direction,
// which is the only thing a signal can be right or wrong about.
const volStopMultiple = 2.0

// volBarsLookback is how many 1m bars the estimate is built from. Six hours is
// long enough that one violent minute cannot set the level for every trade, and
// short enough to follow a regime change within a session.
const volBarsLookback = 360

// volRefreshInterval is how often a symbol's estimate is recomputed. Volatility
// does not move fast enough to justify a fetch per signal, and a fetch per
// signal would put the venue in the entry path.
const volRefreshInterval = 20 * time.Minute

// VolatilityTracker measures per-symbol one-minute range and derives a stop
// distance from it.
//
// Per symbol because the spread is enormous and a single number is wrong for
// everything: XANUSD needs ~1.5%, BLESSUSD ~2.2%, TSTUSD ~4.0%. A shared
// default would leave the quiet symbols with stops far wider than they need and
// the noisy ones still inside the noise.
type VolatilityTracker struct {
	mu    sync.RWMutex
	cache map[string]volEstimate
	base  string
	http  *http.Client
}

type volEstimate struct {
	StopFraction float64
	P90Range     float64
	MedianRange  float64
	Bars         int
	At           time.Time
}

func NewVolatilityTracker(baseURL string) *VolatilityTracker {
	return &VolatilityTracker{
		cache: map[string]volEstimate{},
		base:  baseURL,
		http:  &http.Client{Timeout: 20 * time.Second},
	}
}

// StopFractionFor returns the stop distance as a fraction of price.
//
// The second return is false when no usable estimate exists. Callers must then
// keep the strategy's own stop rather than substitute a default: a hardcoded
// fallback here would silently apply one symbol's volatility to another, which
// is the failure this type exists to prevent.
func (v *VolatilityTracker) StopFractionFor(ctx context.Context, symbol string) (float64, bool) {
	v.mu.RLock()
	est, ok := v.cache[symbol]
	v.mu.RUnlock()
	if ok && time.Since(est.At) < volRefreshInterval {
		return est.StopFraction, est.StopFraction > 0
	}

	est, err := v.measure(ctx, symbol)
	if err != nil {
		// Keep serving a stale estimate rather than none. A measured number from
		// twenty minutes ago is far closer to the truth than the strategy's
		// fixed 0.6%, and refusing to answer would silently revert every symbol
		// to the geometry that produced 9 straight stop-outs.
		if ok && est.StopFraction > 0 {
			return est.StopFraction, true
		}
		log.Printf("[PERP VOL] %s: no estimate (%v) — keeping the strategy's own stop", symbol, err)
		return 0, false
	}

	v.mu.Lock()
	v.cache[symbol] = est
	v.mu.Unlock()
	// A zero estimate is NOT a zero stop, and the log must not say it is.
	//
	// On a quiet symbol every one-minute candle can print with high == low ==
	// open == close: the contract trades, and the price does not move a single
	// tick. p90 is then exactly 0, this function returns ok=false, and the caller
	// keeps the strategy's own stop — which is correct. But the line read "stop
	// set to 0.000%", which describes a stop at the entry price, and 47 of 79
	// symbols printed it at once during a quiet Sunday session. That reads as a
	// desk that has just disarmed every stop it owns.
	if est.StopFraction <= 0 {
		log.Printf("[PERP VOL] %s: 1m range median %.3f%% p90 %.3f%% over %d bars — no measurable movement, "+
			"keeping the strategy's own stop",
			symbol, est.MedianRange*100, est.P90Range*100, est.Bars)
		return 0, false
	}
	log.Printf("[PERP VOL] %s: 1m range median %.3f%% p90 %.3f%% over %d bars — stop set to %.3f%%",
		symbol, est.MedianRange*100, est.P90Range*100, est.Bars, est.StopFraction*100)
	return est.StopFraction, true
}

func (v *VolatilityTracker) measure(ctx context.Context, symbol string) (volEstimate, error) {
	end := time.Now().UTC().Unix()
	start := end - int64(volBarsLookback*60)
	url := fmt.Sprintf("%s/v2/history/candles?resolution=1m&symbol=%s&start=%d&end=%d", v.base, symbol, start, end)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return volEstimate{}, err
	}
	resp, err := v.http.Do(req)
	if err != nil {
		return volEstimate{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return volEstimate{}, fmt.Errorf("candles HTTP %d", resp.StatusCode)
	}

	var body struct {
		Result []struct {
			High  json.Number `json:"high"`
			Low   json.Number `json:"low"`
			Close json.Number `json:"close"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return volEstimate{}, err
	}

	ranges := make([]float64, 0, len(body.Result))
	for _, c := range body.Result {
		h, _ := strconv.ParseFloat(c.High.String(), 64)
		l, _ := strconv.ParseFloat(c.Low.String(), 64)
		cl, _ := strconv.ParseFloat(c.Close.String(), 64)
		if cl <= 0 || h < l {
			continue
		}
		ranges = append(ranges, (h-l)/cl)
	}
	// Too few bars is not an estimate. Deriving a stop from a handful of them
	// would be a guess wearing a measurement's clothes.
	if len(ranges) < 60 {
		return volEstimate{}, fmt.Errorf("only %d usable bars", len(ranges))
	}

	sort.Float64s(ranges)
	p90 := ranges[int(float64(len(ranges))*0.90)]
	med := ranges[len(ranges)/2]
	return volEstimate{
		StopFraction: p90 * volStopMultiple,
		P90Range:     p90,
		MedianRange:  med,
		Bars:         len(ranges),
		At:           time.Now(),
	}, nil
}

// volScaledLevels widens stop and target to clear the measured noise, keeping
// direction and the reward:risk the strategy asked for.
//
// The RATIO is kept rather than the absolute levels: the strategy's opinion
// about how much reward justifies its risk is respected, while its opinion
// about how wide the risk should be is replaced by measurement. Those are
// different claims and only the second has been shown wrong.
//
// WIDENS ONLY, from 2026-08-16. This is a floor, never a replacement.
//
// The purpose stated at volStopMultiple is that "the stop must CLEAR the noise"
// — a stop inside the noise resolves on a coin toss instead of on direction.
// That argument only ever justifies making a stop WIDER. Applying the measured
// distance unconditionally also made stops NARROWER on quiet symbols, which
// inverts the whole point: it puts the stop deeper into the noise, not further
// out of it.
//
// It reached the grid before anyone noticed. ARCUSD printed a p90 one-minute
// range of 0.014%, so 2x gave a 0.028% stop — 2.1 TICKS on a contract that has
// 65 ticks of room, and the grid gate refused the entry. The symbol looked
// broken; the scaler was.
//
// Note which way the failure pointed. The refusal was visible and safe, so this
// showed up as a stream that would not trade. Had the grid gate been one tick
// more permissive, the same signal would have opened a position whose stop sat
// two ticks from the entry — stopped out by a single quote, every time, and
// recorded as the strategy being wrong.
func volScaledLevels(entry, stop, target, stopFraction float64, long bool) (float64, float64) {
	if entry <= 0 || stopFraction <= 0 {
		return stop, target
	}
	origRisk := math.Abs(entry - stop)
	rr := 3.0
	if origRisk > 0 && target > 0 {
		if r := math.Abs(target-entry) / origRisk; r > 0 {
			rr = r
		}
	}
	risk := entry * stopFraction
	// The strategy's own stop already clears the noise — leave it alone.
	if origRisk > 0 && risk <= origRisk {
		return stop, target
	}
	if long {
		return entry - risk, entry + risk*rr
	}
	return entry + risk, entry - risk*rr
}

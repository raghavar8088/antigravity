package marketdata

import (
	"context"
	"log"
	"sync"
	"time"
)

// Delta candle feed for higher timeframes.
//
// The tick stream moved to Delta so signals and orders would read the same
// book. That left the 15m/1h candles still arriving from Binance, which is a
// worse state than either venue alone: the same strategy would evaluate a
// Delta-derived 1m series against Binance-derived 15m and 1h bars, and any
// comparison across timeframes — a pullback measured against the higher-timeframe
// trend, a breakout confirmed on 1h — would straddle two different instruments.
//
// DeltaKlineFeed polls Delta's candle history and emits each bar once, on close.
//
// Polling rather than streaming is deliberate. Delta's socket publishes candles,
// but the REST history endpoint is the same source the scalp desk and the warmup
// path already use, it needs no schema of its own, and one request per
// resolution per bar is negligible load. A 15m bar polled every 30s is at most
// 30s late, which is inside the tolerance of a strategy that trades on 15m bars.

// DeltaKlineFeed emits closed candles for a set of resolutions.
type DeltaKlineFeed struct {
	symbol      string
	resolutions []string

	// lastEmitted guards against emitting the same bar twice, which would
	// double-count volume and re-trigger any close-of-bar logic downstream.
	mu          sync.Mutex
	lastEmitted map[string]time.Time
}

// NewDeltaKlineFeed builds a feed for one symbol. Symbol is translated into
// Delta notation, so a caller configured with "BTCUSDT" or "BTC-USD" works.
func NewDeltaKlineFeed(symbol string, resolutions []string) *DeltaKlineFeed {
	return &DeltaKlineFeed{
		symbol:      DeltaSymbolFor(symbol),
		resolutions: resolutions,
		lastEmitted: map[string]time.Time{},
	}
}

// Start polls each resolution until ctx is cancelled, calling onCandle for every
// newly CLOSED bar. It blocks, so callers run it in a goroutine.
func (f *DeltaKlineFeed) Start(ctx context.Context, onCandle func(res string, c Candle)) {
	var wg sync.WaitGroup
	for _, res := range f.resolutions {
		wg.Add(1)
		go func(res string) {
			defer wg.Done()
			f.pollLoop(ctx, res, onCandle)
		}(res)
	}
	wg.Wait()
}

// pollInterval is how often to check for a new closed bar. Fast enough that a
// bar is picked up promptly, slow enough that the request rate stays trivial.
func pollInterval(res string) time.Duration {
	switch res {
	case "1m":
		return 20 * time.Second
	case "5m", "15m":
		return 30 * time.Second
	default:
		return 60 * time.Second
	}
}

func (f *DeltaKlineFeed) pollLoop(ctx context.Context, res string, onCandle func(string, Candle)) {
	t := time.NewTicker(pollInterval(res))
	defer t.Stop()

	// Prime immediately so a restart does not wait a full interval before the
	// feed reports as active.
	f.poll(ctx, res, onCandle)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			f.poll(ctx, res, onCandle)
		}
	}
}

func (f *DeltaKlineFeed) poll(ctx context.Context, res string, onCandle func(string, Candle)) {
	if ctx.Err() != nil {
		return
	}
	width := resolutionDuration(res)
	now := time.Now().UTC()
	// Ask for a few bars of slack so a missed poll is caught up rather than
	// leaving a permanent hole in the series.
	raw, err := FetchDeltaHistoricalCandles(f.symbol, res, now.Add(-5*width), now)
	if err != nil {
		log.Printf("[DELTA KLINES] %s %s fetch failed: %v", f.symbol, res, err)
		return
	}
	f.emitFrom(raw, res, now, onCandle)
}

// emitFrom converts fetched history into closed candles and hands each new one
// to onCandle exactly once. Split from poll so the two ways this can silently
// corrupt a strategy's view — emitting the forming bar, and emitting a bar twice
// — are testable without a network call.
func (f *DeltaKlineFeed) emitFrom(raw []HistoricalCandle, res string, now time.Time, onCandle func(string, Candle)) {
	width := resolutionDuration(res)
	for _, c := range raw {
		// Only CLOSED bars. The most recent bar is still forming, and emitting it
		// would hand strategies a partial candle whose high, low and volume all
		// change under them — then emit it again, differently, on the next poll.
		closeTime := c.OpenTime.Add(width)
		if !closeTime.Before(now) {
			continue
		}

		f.mu.Lock()
		key := res
		last, seen := f.lastEmitted[key]
		if seen && !c.OpenTime.After(last) {
			f.mu.Unlock()
			continue
		}
		f.lastEmitted[key] = c.OpenTime
		f.mu.Unlock()

		onCandle(res, Candle{
			Symbol: f.symbol,
			Open:   c.Open, High: c.High, Low: c.Low, Close: c.Close,
			// Contracts -> BTC, as everywhere else Delta data enters this engine.
			Volume:    c.Volume * DeltaContractValueBTC,
			OpenTime:  c.OpenTime,
			CloseTime: closeTime,
		})
	}
}

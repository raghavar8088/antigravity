package marketdata

import (
	"log"
	"sync"
	"time"
)

// Candle represents an OHLCV bar aggregated from raw ticks.
type Candle struct {
	Symbol    string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	Trades    int
	OpenTime  time.Time
	CloseTime time.Time
}

// ToTick converts a closed candle into a Tick (using Close price)
// so it can be fed to strategies that implement OnCandle(Tick).
func (c Candle) ToTick() Tick {
	return Tick{
		Symbol:   c.Symbol,
		Price:    c.Close,
		Quantity: c.Volume,
		Side:     "",
		TradeID:  0,
		TimeMs:   c.CloseTime.UnixMilli(),
	}
}

// CandleAggregator collects raw ticks and emits closed candles
// on 1-minute and 5-minute intervals.
type CandleAggregator struct {
	mu sync.Mutex

	// Current building candles
	current1m *Candle
	current5m *Candle

	// Output channels — orchestrator listens on these
	Candles1m chan Candle
	Candles5m chan Candle

	// Interval tracking
	current1mStart time.Time
	current5mStart time.Time

	// Stats
	totalTicks    int64
	totalCandles  int64
}

// NewCandleAggregator creates a new aggregator with buffered output channels.
func NewCandleAggregator() *CandleAggregator {
	return &CandleAggregator{
		Candles1m: make(chan Candle, 100),
		Candles5m: make(chan Candle, 100),
	}
}

// Feed processes a raw tick and emits candles when intervals close.
func (a *CandleAggregator) Feed(t Tick) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.totalTicks++
	now := time.Now()

	// ─── 1-Minute Candle ───
	minute1Start := now.Truncate(1 * time.Minute)
	if a.current1m == nil {
		// First tick — start building
		a.current1m = &Candle{
			Symbol:   t.Symbol,
			Open:     t.Price,
			High:     t.Price,
			Low:      t.Price,
			Close:    t.Price,
			Volume:   t.Quantity,
			Trades:   1,
			OpenTime: minute1Start,
		}
		a.current1mStart = minute1Start
	} else if minute1Start.After(a.current1mStart) {
		// New minute — close previous candle and emit
		a.current1m.CloseTime = a.current1mStart.Add(1 * time.Minute)
		a.totalCandles++

		// Non-blocking send
		select {
		case a.Candles1m <- *a.current1m:
		default:
			log.Println("[CANDLE AGG] Warning: 1m channel full, dropping candle")
		}

		// Start new candle
		a.current1m = &Candle{
			Symbol:   t.Symbol,
			Open:     t.Price,
			High:     t.Price,
			Low:      t.Price,
			Close:    t.Price,
			Volume:   t.Quantity,
			Trades:   1,
			OpenTime: minute1Start,
		}
		a.current1mStart = minute1Start
	} else {
		// Same minute — update current candle
		a.current1m.Close = t.Price
		a.current1m.Volume += t.Quantity
		a.current1m.Trades++
		if t.Price > a.current1m.High {
			a.current1m.High = t.Price
		}
		if t.Price < a.current1m.Low {
			a.current1m.Low = t.Price
		}
	}

	// ─── 5-Minute Candle ───
	minute5Start := now.Truncate(5 * time.Minute)
	if a.current5m == nil {
		a.current5m = &Candle{
			Symbol:   t.Symbol,
			Open:     t.Price,
			High:     t.Price,
			Low:      t.Price,
			Close:    t.Price,
			Volume:   t.Quantity,
			Trades:   1,
			OpenTime: minute5Start,
		}
		a.current5mStart = minute5Start
	} else if minute5Start.After(a.current5mStart) {
		a.current5m.CloseTime = a.current5mStart.Add(5 * time.Minute)

		select {
		case a.Candles5m <- *a.current5m:
		default:
			log.Println("[CANDLE AGG] Warning: 5m channel full, dropping candle")
		}

		a.current5m = &Candle{
			Symbol:   t.Symbol,
			Open:     t.Price,
			High:     t.Price,
			Low:      t.Price,
			Close:    t.Price,
			Volume:   t.Quantity,
			Trades:   1,
			OpenTime: minute5Start,
		}
		a.current5mStart = minute5Start
	} else {
		a.current5m.Close = t.Price
		a.current5m.Volume += t.Quantity
		a.current5m.Trades++
		if t.Price > a.current5m.High {
			a.current5m.High = t.Price
		}
		if t.Price < a.current5m.Low {
			a.current5m.Low = t.Price
		}
	}
}

// GetStats returns aggregator statistics.
func (a *CandleAggregator) GetStats() (totalTicks int64, totalCandles int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totalTicks, a.totalCandles
}

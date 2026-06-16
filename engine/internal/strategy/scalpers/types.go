package scalpers

import "time"

// Regime constants — must match engine/internal/regime definitions
type Regime string

const (
	RegimeTrending Regime = "TRENDING"
	RegimeRanging  Regime = "RANGING"
	RegimeVolatile Regime = "VOLATILE"
	RegimeUnknown  Regime = "UNKNOWN"
)

// Direction of a trade signal
type Direction string

const (
	DirectionLong  Direction = "LONG"
	DirectionShort Direction = "SHORT"
	DirectionNone  Direction = "NONE"
)

// Candle is a single OHLCV bar
type Candle struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// OrderBookSnapshot is a point-in-time view of the order book
type OrderBookSnapshot struct {
	BestBid     float64
	BestAsk     float64
	BidWallSize float64 // total bid liquidity within 0.3% of mid
	AskWallSize float64 // total ask liquidity within 0.3% of mid
	Imbalance   float64 // positive = more bids, negative = more asks, range -1..1
}

// IsPopulated returns true when the snapshot contains real order book data.
// Strategies should use this to decide whether to apply OB confirmation or
// fall back to CVD/MACD-only confirmation.
func (ob OrderBookSnapshot) IsPopulated() bool {
	return ob.BidWallSize > 0 || ob.AskWallSize > 0
}

// MarketContext is everything a strategy needs to evaluate a signal.
// Populated by the trading loop before calling Evaluate().
type MarketContext struct {
	// Regime
	Regime Regime

	// Current price
	Price float64

	// Candles — short and higher timeframe
	Candles1m  []Candle // last 100 1m candles
	Candles5m  []Candle // last 100 5m candles
	Candles15m []Candle // last 60 15m candles
	Candles1h  []Candle // last 48 1h candles
	Candles4h  []Candle // last 30 4h candles

	// Order flow
	CVD            float64   // cumulative volume delta (current)
	CVDPrev        float64   // CVD from 1 bar ago (divergence check)
	CVDHistory     []float64 // rolling CVD history (up to 5 readings, newest last)
	FundingRate    float64   // perpetual swap funding rate in percent (8h, e.g. 0.01 = 0.01% — raw Binance value × 100)
	FundingHistory []float64 // last 3 funding readings (newest last); nil = not available
	OpenInterest   float64   // current OI in BTC-equivalent (USD / price)
	OpenInterestPrev float64 // OI from previous reading
	OrderBook      OrderBookSnapshot

	// Session
	SessionName string    // "ASIA" | "LONDON" | "NEW_YORK"
	Now         time.Time // UTC
}

// Signal is the output of a strategy evaluation
type Signal struct {
	Strategy    string
	Direction   Direction
	Confidence  float64   // 0.0–1.0
	StopLoss    float64   // absolute price
	TakeProfit  float64   // absolute price (primary)
	TakeProfit2 float64   // absolute price (secondary, 0 = not set)
	Reason      string    // human-readable, logged to audit
	Timestamp   time.Time
}

// NoSignal returns a zero signal (no trade)
func NoSignal(strategyName string) Signal {
	return Signal{
		Strategy:  strategyName,
		Direction: DirectionNone,
		Timestamp: time.Now().UTC(),
	}
}

// Strategy is the interface every scalper strategy must implement
type Strategy interface {
	Name() string
	ValidRegimes() []Regime
	Evaluate(ctx MarketContext) Signal
}

// RegistryEntry wraps a strategy with metadata for the trading loop
type RegistryEntry struct {
	Strategy     Strategy
	Name         string
	Description  string
	Regimes      []Regime
	Timeframes   []string // e.g. ["15m","1h"]
	MaxPositions int      // how many simultaneous positions this strategy can hold
}

// Performance tracks live stats per strategy — used by FilterWinnersOnly
type Performance struct {
	StrategyName string
	TotalTrades  int
	WinRate      float64 // 0.0–1.0
	SharpeRatio  float64
	MaxDrawdown  float64 // 0.0–1.0
	Active       bool
	LastPnL      float64 // PnL of the most recently closed trade (USD)
}

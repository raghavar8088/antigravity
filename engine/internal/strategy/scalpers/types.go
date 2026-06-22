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

	// Per-level bid depth (volume within % of mid)
	BidDepth01pct float64 // total bid volume within 0.1% of mid price
	BidDepth05pct float64 // total bid volume within 0.5% of mid price
	BidDepth1pct  float64 // total bid volume within 1.0% of mid price

	// Per-level ask depth (volume within % of mid)
	AskDepth01pct float64
	AskDepth05pct float64
	AskDepth1pct  float64

	// TradeFlowImbalance: (buy_vol - sell_vol) / (buy_vol + sell_vol) over last 1m
	// Positive = buy-side aggressive, negative = sell-side aggressive. Range -1..1.
	TradeFlowImbalance float64
}

// OrderBookDepthLevels carries optional per-level LOB depth and trade-flow
// features so strategies can use continuous order-book inputs instead of
// the binary BidWallSize/AskWallSize gate. Populated via the variadic
// ScalerBundle.UpdateOrderBook parameter — callers that don't have this data
// yet can omit it entirely (backward compatible).
type OrderBookDepthLevels struct {
	BidDepth01pct, BidDepth05pct, BidDepth1pct float64
	AskDepth01pct, AskDepth05pct, AskDepth1pct float64
	TradeFlowImbalance                         float64
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
	CVD        float64   // cumulative volume delta (current)
	CVDPrev    float64   // CVD from 1 bar ago (divergence check)
	CVDHistory []float64 // rolling CVD history (up to 5 readings, newest last)
	// FundingRate is the raw Binance perpetual funding rate as a decimal.
	// Example: 0.0001 = 0.01% per 8h funding interval.
	// Do NOT multiply by 100. Thresholds in strategies use raw decimal values:
	//   S8 threshold: ±0.0003 = ±0.03% per 8h (crowded positioning signal).
	//   S3 threshold: ±0.0001 = ±0.01% per 8h (funding spike signal).
	FundingRate      float64
	FundingHistory   []float64 // last 3 funding readings (newest last); nil = not available
	OpenInterest     float64   // current OI in BTC-equivalent (USD / price)
	OpenInterestPrev float64   // OI from previous reading
	OrderBook        OrderBookSnapshot

	// Opening Range (NY session) — persisted by ScalerBundle across evaluation cycles
	ORHigh float64 // NY opening range high (0 if not yet formed)
	ORLow  float64 // NY opening range low (0 if not yet formed)

	// Session
	SessionName string    // "ASIA" | "LONDON" | "NEW_YORK"
	Now         time.Time // UTC

	// DVOL is the current Deribit BTC volatility index (30-day forward IV,
	// analogous to VIX). 0 if the feed has never populated. Strategies that
	// depend on DVOL must check DVOLPopulated/DVOLHealthy and degrade
	// gracefully (e.g. fall back to RealizedVol) rather than permanently
	// returning NoSignal when the feed is down.
	DVOL          float64
	DVOLHistory   []float64 // rolling history, up to last 12 readings (1hr @ 5min), oldest first
	DVOLPopulated bool      // true once the feed has fetched at least one good value
	DVOLHealthy   bool      // true if the most recent fetch attempt succeeded (not stale)

	// Liquidations — Binance BTCUSDT perpetual forced-liquidation USD notional,
	// split by the side of the liquidated position, over trailing windows.
	// Populated from BinanceLiquidationHolder. 0 values if the feed has never
	// populated; strategies must check LiquidationFeedHealthy and degrade
	// gracefully (S14) rather than misinterpreting 0 as "no liquidations".
	LongLiquidationsUSD5m   float64
	ShortLiquidationsUSD5m  float64
	LongLiquidationsUSD15m  float64
	ShortLiquidationsUSD15m float64
	// Rolling 1-minute liquidation rate (USD/min) by side, and the trailing
	// 1-hour average rate — used by S14's cascade-spike/deceleration detection.
	LongLiquidationsRate1m   float64
	ShortLiquidationsRate1m  float64
	LongLiquidationsAvg1h    float64
	ShortLiquidationsAvg1h   float64
	LiquidationFeedPopulated bool
	LiquidationFeedHealthy   bool

	// PerpSpotBasis — Binance BTCUSDT perpetual mark price vs Coinbase spot
	// (ctx.Price), for S16's basis-momentum signal. Populated from
	// BinancePerpPriceHolder. PerpPrice is 0 if the feed has never populated.
	PerpPrice          float64
	PerpPricePopulated bool
	PerpPriceHealthy   bool

	// Macro cross-asset feed — Nasdaq futures proxy (NQ=F) and US Dollar Index
	// (DXY), polled from Yahoo Finance's public chart endpoint every 10 min
	// (see marketdata/macro_feed.go MacroFeedHolder). Feeds S18-S21.
	NasdaqProxyPrice     float64
	NasdaqProxyChangePct float64
	DXYPrice             float64
	DXYChangePct         float64
	DXYRollingHigh20     float64 // 20-poll rolling high (breakout reference)
	DXYRollingLow20      float64 // 20-poll rolling low (breakdown reference)
	// BTCEquitiesCorrelation30d is an APPROXIMATION of a true 30-day rolling
	// correlation: true 30-day correlation needs ~720 hourly closes, which is
	// impractical to bootstrap quickly and exceeds what a 10-min-cadence feed
	// can usefully retain. This is instead a short-horizon (~2hr, last ~12
	// macro polls paired against the most recent BTC 1h candle closes)
	// Pearson correlation. Field name kept aligned to the spec's "30d" ask;
	// always treat as a short-horizon proxy, not a true monthly statistic.
	BTCEquitiesCorrelation30d float64
	MacroFeedPopulated        bool
	MacroFeedHealthy          bool
}

// Signal is the output of a strategy evaluation
type Signal struct {
	Strategy    string
	Direction   Direction
	Confidence  float64 // 0.0–1.0
	StopLoss    float64 // absolute price
	TakeProfit  float64 // absolute price (primary)
	TakeProfit2 float64 // absolute price (secondary, 0 = not set)
	Reason      string  // human-readable, logged to audit
	Timestamp   time.Time

	// IsShadow is true when this signal was produced by a strategy that is
	// not yet cleared for live trading (rollout-phase gated, or forced via
	// SHADOW_STRATEGIES). Shadow signals are fully evaluated — same
	// Direction/Confidence/SL/TP as a live signal would have — but are
	// routed to the shadow ledger instead of the paper OMS so the strategy
	// can accumulate a real performance track record without risking
	// account balance. See engine/internal/shadow.
	IsShadow bool
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

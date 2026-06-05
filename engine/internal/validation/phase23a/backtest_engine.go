package phase23a

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// StrategySpec describes a strategy's backtesting characteristics.
// In production this would come from the real Strategy interface;
// in demo/test mode we parameterise it directly.
type StrategySpec struct {
	ID           string
	Name         string
	Family       string
	AlphaSource  string
	// Signal generation parameters
	WinRate      float64 // base win rate (regime-adjusted at runtime)
	AvgWinPct    float64 // avg win as % of entry price
	AvgLossPct   float64 // avg loss as % of entry price
	Frequency    float64 // signals per 100 candles
	MinHoldMins  int
	MaxHoldMins  int
	// Regime biases: multiplier on WinRate in each regime
	BullBias   float64 // >1 = better in bull
	BearBias   float64
	RangeBias  float64
	VolBias    float64
}

// BacktestEngine runs a strategy specification against a candle series.
type BacktestEngine struct {
	Config BacktestConfig
	rng    *rand.Rand
}

// NewBacktestEngine creates a new engine with the given configuration.
func NewBacktestEngine(cfg BacktestConfig, rng *rand.Rand) *BacktestEngine {
	return &BacktestEngine{Config: cfg, rng: rng}
}

// Run simulates the strategy against the candle series and returns all closed trades.
func (e *BacktestEngine) Run(strategyIdx int, spec StrategySpec, candles []OHLCVCandle) BacktestResult {
	cfg := e.Config
	result := BacktestResult{
		StrategyID:   spec.ID,
		StrategyName: spec.Name,
		Family:       spec.Family,
		Config:       cfg,
	}
	if len(candles) == 0 {
		return result
	}

	capital := cfg.InitialCapital
	equity := capital
	tradeIdx := 0

	for i, c := range candles {
		// generate a signal probabilistically based on frequency
		if e.rng.Float64()*100 > spec.Frequency {
			continue
		}
		// determine regime from price action
		regime := inferRegime(candles, i)

		// regime-adjusted win rate
		wr := spec.WinRate * regimeBias(spec, regime)
		wr = math.Max(0.05, math.Min(0.95, wr))

		// position sizing
		positionUSD := equity * cfg.PositionCapPct
		entryPrice := c.Close
		// apply slippage to entry
		entryPrice *= 1 + cfg.SlippageBps/10000
		entryFee := positionUSD * cfg.TakerFeeBps / 10000

		// determine win/loss
		isWin := e.rng.Float64() < wr
		var pnlPct float64
		if isWin {
			pnlPct = spec.AvgWinPct * (0.7 + e.rng.Float64()*0.6)
		} else {
			pnlPct = -spec.AvgLossPct * (0.7 + e.rng.Float64()*0.6)
		}

		// exit price
		exitPrice := entryPrice * (1 + pnlPct/100)
		exitPrice *= 1 - cfg.SlippageBps/10000 // exit slippage
		exitFee := positionUSD * cfg.TakerFeeBps / 10000

		grossPnL := positionUSD * pnlPct / 100
		totalFees := entryFee + exitFee
		netPnL := grossPnL - totalFees

		// funding cost for overnight holds
		holdMins := spec.MinHoldMins + e.rng.Intn(spec.MaxHoldMins-spec.MinHoldMins+1)
		fundingCost := 0.0
		if holdMins > 480 { // held overnight
			periods := float64(holdMins) / 480.0
			fundingCost = positionUSD * 0.0001 * periods // 0.01% per period
			netPnL -= fundingCost
		}

		entry := c.OpenTime
		exit := entry.Add(time.Duration(holdMins) * time.Minute)
		if i+holdMins < len(candles) {
			exit = candles[i+holdMins].OpenTime
		}

		trade := phase22e.TradeRecord{
			TradeID:      fmt.Sprintf("%s-w%02d-t%05d", spec.ID, strategyIdx, tradeIdx),
			StrategyID:   spec.ID,
			StrategyName: spec.Name,
			Family:       spec.Family,
			Symbol:       cfg.Symbol,
			Side:         "LONG",
			EntryPrice:   entryPrice,
			ExitPrice:    exitPrice,
			Quantity:     positionUSD / entryPrice,
			GrossPnLUSD:  grossPnL,
			NetPnLUSD:    netPnL,
			FeesUSD:      totalFees + fundingCost,
			HoldMinutes:  float64(holdMins),
			EntryTime:    entry,
			ExitTime:     exit,
			Regime:       legacyRegime(regime),
			IsLive:       false,
		}

		equity += netPnL
		result.Trades = append(result.Trades, trade)
		result.EquityCurve = append(result.EquityCurve, equity)
		tradeIdx++
	}

	result.FinalNAV = equity
	result.TradingDays = tradingDays(candles)
	result.CAGR = computeCAGR(cfg.InitialCapital, equity, result.TradingDays)
	return result
}

// ── helpers ───────────────────────────────────────────────────────────────────

type simRegime int

const (
	simBull simRegime = iota
	simBear
	simRange
	simVolatile
)

func inferRegime(candles []OHLCVCandle, idx int) simRegime {
	window := 30
	start := idx - window
	if start < 0 {
		start = 0
	}
	if idx == 0 {
		return simRange
	}
	slice := candles[start : idx+1]
	first := slice[0].Close
	last := slice[len(slice)-1].Close
	if first == 0 {
		return simRange
	}
	chg := (last - first) / first * 100

	// volatility proxy
	returns := make([]float64, 0, len(slice)-1)
	for i := 1; i < len(slice); i++ {
		if slice[i-1].Close > 0 {
			returns = append(returns, (slice[i].Close-slice[i-1].Close)/slice[i-1].Close*100)
		}
	}
	vol := stddevSim(returns)

	switch {
	case vol > 2.0:
		return simVolatile
	case chg > 1.5:
		return simBull
	case chg < -1.5:
		return simBear
	default:
		return simRange
	}
}

func regimeBias(spec StrategySpec, r simRegime) float64 {
	switch r {
	case simBull:
		return spec.BullBias
	case simBear:
		return spec.BearBias
	case simRange:
		return spec.RangeBias
	default:
		return spec.VolBias
	}
}

func legacyRegime(r simRegime) phase22e.Regime {
	switch r {
	case simBull:
		return phase22e.RegimeBull
	case simBear:
		return phase22e.RegimeBear
	case simVolatile:
		return phase22e.RegimeVolatile
	default:
		return phase22e.RegimeRange
	}
}

func tradingDays(candles []OHLCVCandle) int {
	if len(candles) < 2 {
		return 1
	}
	dur := candles[len(candles)-1].CloseTime.Sub(candles[0].OpenTime)
	days := int(dur.Hours() / 24)
	if days < 1 {
		days = 1
	}
	return days
}

func computeCAGR(initialCapital, finalNAV float64, tradingDays int) float64 {
	if initialCapital <= 0 || tradingDays <= 0 || finalNAV <= 0 {
		return 0
	}
	years := float64(tradingDays) / 365.0
	if years < 0.001 {
		return 0
	}
	cagr := math.Pow(finalNAV/initialCapital, 1/years) - 1
	if math.IsNaN(cagr) || math.IsInf(cagr, 0) {
		return 0
	}
	return cagr * 100 // as percentage
}

func stddevSim(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	mean := 0.0
	for _, x := range xs {
		mean += x
	}
	mean /= float64(len(xs))
	s := 0.0
	for _, x := range xs {
		d := x - mean
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

// ── Strategy library for demo mode ───────────────────────────────────────────

// DefaultStrategySpecs returns the 20 canonical strategies used for demo backtesting.
func DefaultStrategySpecs() []StrategySpec {
	return []StrategySpec{
		// Top institutional strategies
		{ID: "s01", Name: "EMA Cross 21/50 BTC", Family: "EMA Cross", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.56, AvgWinPct: 0.22, AvgLossPct: 0.14, Frequency: 3.0, MinHoldMins: 30, MaxHoldMins: 180,
			BullBias: 1.2, BearBias: 0.8, RangeBias: 1.0, VolBias: 0.9},
		{ID: "s02", Name: "FVG Retest Long", Family: "Price Action", AlphaSource: "Fair Value Gap",
			WinRate: 0.57, AvgWinPct: 0.24, AvgLossPct: 0.15, Frequency: 1.5, MinHoldMins: 45, MaxHoldMins: 240,
			BullBias: 1.3, BearBias: 0.7, RangeBias: 1.1, VolBias: 1.0},
		{ID: "s03", Name: "Liquidation Cascade", Family: "Microstructure", AlphaSource: "Liquidation Cascade",
			WinRate: 0.61, AvgWinPct: 0.32, AvgLossPct: 0.18, Frequency: 0.5, MinHoldMins: 5, MaxHoldMins: 60,
			BullBias: 1.0, BearBias: 1.2, RangeBias: 0.8, VolBias: 1.5},
		{ID: "s04", Name: "Funding Rate Mean Rev", Family: "Funding", AlphaSource: "Funding Mean Reversion",
			WinRate: 0.63, AvgWinPct: 0.15, AvgLossPct: 0.10, Frequency: 0.8, MinHoldMins: 120, MaxHoldMins: 480,
			BullBias: 0.9, BearBias: 0.9, RangeBias: 1.2, VolBias: 0.8},
		{ID: "s05", Name: "Bollinger Squeeze", Family: "Bollinger", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.59, AvgWinPct: 0.20, AvgLossPct: 0.13, Frequency: 2.0, MinHoldMins: 30, MaxHoldMins: 120,
			BullBias: 1.1, BearBias: 0.9, RangeBias: 1.3, VolBias: 0.7},
		{ID: "s06", Name: "Liquidity Sweep", Family: "Microstructure", AlphaSource: "Liquidity Sweep",
			WinRate: 0.58, AvgWinPct: 0.26, AvgLossPct: 0.16, Frequency: 1.0, MinHoldMins: 15, MaxHoldMins: 90,
			BullBias: 1.1, BearBias: 1.1, RangeBias: 1.0, VolBias: 1.2},
		{ID: "s07", Name: "Order Block Rejection", Family: "Price Action", AlphaSource: "Order Block",
			WinRate: 0.55, AvgWinPct: 0.22, AvgLossPct: 0.14, Frequency: 1.2, MinHoldMins: 30, MaxHoldMins: 180,
			BullBias: 1.1, BearBias: 1.0, RangeBias: 1.2, VolBias: 0.9},
		{ID: "s08", Name: "MSS Continuation", Family: "Price Action", AlphaSource: "Market Structure Shift",
			WinRate: 0.57, AvgWinPct: 0.21, AvgLossPct: 0.14, Frequency: 1.5, MinHoldMins: 45, MaxHoldMins: 240,
			BullBias: 1.3, BearBias: 0.8, RangeBias: 1.0, VolBias: 1.0},
		{ID: "s09", Name: "Session Open Momentum", Family: "Session", AlphaSource: "Session Expansion",
			WinRate: 0.55, AvgWinPct: 0.18, AvgLossPct: 0.12, Frequency: 2.0, MinHoldMins: 30, MaxHoldMins: 120,
			BullBias: 1.2, BearBias: 0.9, RangeBias: 0.9, VolBias: 1.1},
		{ID: "s10", Name: "VWAP Bounce", Family: "Volume", AlphaSource: "Market Profile",
			WinRate: 0.53, AvgWinPct: 0.17, AvgLossPct: 0.13, Frequency: 3.0, MinHoldMins: 15, MaxHoldMins: 90,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 1.3, VolBias: 0.8},
		{ID: "s11", Name: "RSI Oversold 30", Family: "RSI", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.55, AvgWinPct: 0.19, AvgLossPct: 0.13, Frequency: 2.5, MinHoldMins: 30, MaxHoldMins: 180,
			BullBias: 1.1, BearBias: 0.9, RangeBias: 1.2, VolBias: 0.9},
		{ID: "s12", Name: "CVD Divergence", Family: "Microstructure", AlphaSource: "Order Flow",
			WinRate: 0.54, AvgWinPct: 0.20, AvgLossPct: 0.14, Frequency: 1.8, MinHoldMins: 20, MaxHoldMins: 120,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 1.0, VolBias: 1.0},
		{ID: "s13", Name: "EMA Cross 9/21", Family: "EMA Cross", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.53, AvgWinPct: 0.18, AvgLossPct: 0.13, Frequency: 4.0, MinHoldMins: 15, MaxHoldMins: 90,
			BullBias: 1.1, BearBias: 0.9, RangeBias: 1.0, VolBias: 0.9},
		{ID: "s14", Name: "POC Retest", Family: "Volume", AlphaSource: "Market Profile",
			WinRate: 0.54, AvgWinPct: 0.19, AvgLossPct: 0.13, Frequency: 1.5, MinHoldMins: 30, MaxHoldMins: 180,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 1.2, VolBias: 0.9},
		{ID: "s15", Name: "Volatility Squeeze", Family: "Bollinger", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.56, AvgWinPct: 0.23, AvgLossPct: 0.15, Frequency: 1.0, MinHoldMins: 60, MaxHoldMins: 360,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 0.9, VolBias: 1.3},
		// Lower tier strategies
		{ID: "s16", Name: "EMA Cross 3/15 Scalp", Family: "EMA Cross", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.51, AvgWinPct: 0.12, AvgLossPct: 0.11, Frequency: 6.0, MinHoldMins: 5, MaxHoldMins: 30,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 1.0, VolBias: 1.0},
		{ID: "s17", Name: "RSI Slope 5/10", Family: "RSI", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.50, AvgWinPct: 0.16, AvgLossPct: 0.15, Frequency: 2.0, MinHoldMins: 30, MaxHoldMins: 120,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 1.0, VolBias: 1.0},
		{ID: "s18", Name: "Stochastic Range", Family: "Oscillator", AlphaSource: "Statistical Mean Reversion",
			WinRate: 0.50, AvgWinPct: 0.15, AvgLossPct: 0.15, Frequency: 2.5, MinHoldMins: 30, MaxHoldMins: 120,
			BullBias: 1.0, BearBias: 1.0, RangeBias: 1.1, VolBias: 0.9},
		// Intentional failures for elimination testing
		{ID: "s19", Name: "Trend Chaser (Failing)", Family: "Momentum", AlphaSource: "Unclassified",
			WinRate: 0.42, AvgWinPct: 0.15, AvgLossPct: 0.20, Frequency: 3.0, MinHoldMins: 60, MaxHoldMins: 360,
			BullBias: 1.0, BearBias: 0.5, RangeBias: 0.7, VolBias: 0.6},
		{ID: "s20", Name: "Overfit Scalper (Failing)", Family: "EMA Cross", AlphaSource: "Unclassified",
			WinRate: 0.38, AvgWinPct: 0.10, AvgLossPct: 0.18, Frequency: 8.0, MinHoldMins: 2, MaxHoldMins: 15,
			BullBias: 1.0, BearBias: 0.8, RangeBias: 0.9, VolBias: 0.7},
	}
}

package strategy

import (
	"math"
	"time"

	"antigravity-engine/internal/marketdata"
)

// ── Action ────────────────────────────────────────────────────────────────────

// Action is the direction of a trading signal (BUY, SELL, HOLD).
type Action string

const (
	ActionBuy  Action = "BUY"
	ActionSell Action = "SELL"
	ActionHold Action = "HOLD"
)

// ── Signal ────────────────────────────────────────────────────────────────────

// Signal is the output of a strategy's OnTick evaluation.
// StopLossPct and TakeProfitPct are in percentage points (e.g. 0.18 = 0.18%).
type Signal struct {
	Symbol        string
	Action        Action
	TargetSize    float64   // BTC quantity
	Confidence    float64   // 0.0–1.0
	StopLossPct   float64   // % distance from entry
	TakeProfitPct float64   // % distance from entry
	CreatedAt     time.Time
	Timeframe     string
	Reason        string
	// AI-assisted decision fields (set by the bridge approval path; empty for algo-only signals)
	AIDecisionID string
	AIReasoning  string
}

// ── Strategy interface ────────────────────────────────────────────────────────

// Strategy is the interface every tick-based strategy must implement.
type Strategy interface {
	Name() string
	OnTick(tick marketdata.Tick) []Signal
	OnCandle(tick marketdata.Tick) []Signal
}

// ── StrategyFamily ────────────────────────────────────────────────────────────

// StrategyFamily groups strategies by risk profile for portfolio-level limits.
type StrategyFamily string

const (
	FamilyTrend      StrategyFamily = "TREND"
	FamilyMeanRev    StrategyFamily = "MEAN_REVERSION"
	FamilyBreakout   StrategyFamily = "BREAKOUT"
	FamilyVolatility StrategyFamily = "VOLATILITY"
	FamilyUnknown    StrategyFamily = "UNKNOWN"
)

// NormalizeCategory returns a canonical category name for signal flow metrics.
func NormalizeCategory(category, _ string) string {
	if category == "" {
		return "Unknown"
	}
	return category
}

// CategoryToRiskFamily maps a strategy category string to a StrategyFamily.
func CategoryToRiskFamily(category string) StrategyFamily {
	switch category {
	case "Trend", "EMA", "Momentum", "Breakout", "ADX":
		return FamilyTrend
	case "MeanRev", "Bollinger", "RSI", "VWAP", "Reversion":
		return FamilyMeanRev
	case "ORB", "OpeningRange":
		return FamilyBreakout
	case "Volatile", "Volatility", "LiquiditySweep":
		return FamilyVolatility
	default:
		return FamilyUnknown
	}
}

// CategoryTier maps a category string to its StrategyTier.
func CategoryTier(category string) StrategyTier {
	switch category {
	case "Elite", "Scalp", "Top":
		return StrategyTierElite
	case "Experimental", "New":
		return StrategyTierExperimental
	default:
		return StrategyTierStandard
	}
}

// ── Price-based indicators ([]float64 variants used by loop.go) ───────────────

// EMA computes the exponential moving average of the last n prices.
// Returns 0 if there is insufficient data.
func EMA(prices []float64, period int) float64 {
	if len(prices) < period || period <= 0 {
		return 0
	}
	k := 2.0 / float64(period+1)
	ema := prices[0]
	for _, p := range prices[1:] {
		ema = p*k + ema*(1-k)
	}
	return ema
}

// ATR computes average true range from a slice of prices (uses price-to-price
// differences as a simplified TR when only close prices are available).
func ATR(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}
	tail := prices[len(prices)-(period+1):]
	var sum float64
	for i := 1; i < len(tail); i++ {
		sum += math.Abs(tail[i] - tail[i-1])
	}
	return sum / float64(period)
}

// RollingVWAP computes the volume-weighted average price over the most recent
// n price/volume pairs. Returns 0 if data is insufficient or all volumes are zero.
func RollingVWAP(prices, volumes []float64, n int) float64 {
	if len(prices) == 0 || len(volumes) == 0 || n <= 0 {
		return 0
	}
	l := len(prices)
	if len(volumes) < l {
		l = len(volumes)
	}
	if n > l {
		n = l
	}
	ps := prices[l-n:]
	vs := volumes[l-n:]
	var pvSum, vSum float64
	for i := range ps {
		pvSum += ps[i] * vs[i]
		vSum += vs[i]
	}
	if vSum == 0 {
		return 0
	}
	return pvSum / vSum
}

// ADX computes a simplified Average Directional Index from price closes.
func ADX(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}
	tail := prices[len(prices)-(period+1):]
	var dmPlus, dmMinus, tr float64
	for i := 1; i < len(tail); i++ {
		up := tail[i] - tail[i-1]
		dn := tail[i-1] - tail[i]
		if up > dn && up > 0 {
			dmPlus += up
		}
		if dn > up && dn > 0 {
			dmMinus += dn
		}
		tr += math.Abs(tail[i] - tail[i-1])
	}
	if tr == 0 {
		return 0
	}
	diPlus := 100 * dmPlus / tr
	diMinus := 100 * dmMinus / tr
	sum := diPlus + diMinus
	if sum == 0 {
		return 0
	}
	return 100 * math.Abs(diPlus-diMinus) / sum
}

// ── No-op stub ────────────────────────────────────────────────────────────────

type noopStrategy struct{ name string }

func (n *noopStrategy) Name() string                            { return n.name }
func (n *noopStrategy) OnTick(_ marketdata.Tick) []Signal      { return nil }
func (n *noopStrategy) OnCandle(_ marketdata.Tick) []Signal    { return nil }

// ── Phase 22C alpha constructors (removed from registry, constructors preserved) ──

// NewFundingMeanReversionAlpha returns a no-op stub (strategy removed from registry).
func NewFundingMeanReversionAlpha() Strategy { return &noopStrategy{name: "FundingMeanReversion"} }

// NewCVDDivergenceAlpha returns a no-op stub.
func NewCVDDivergenceAlpha() Strategy { return &noopStrategy{name: "CVDDivergence"} }

// NewDeltaAbsorptionAlpha returns a no-op stub.
func NewDeltaAbsorptionAlpha() Strategy { return &noopStrategy{name: "DeltaAbsorption"} }

// NewLiquiditySweepReversalAlpha returns a no-op stub.
func NewLiquiditySweepReversalAlpha() Strategy {
	return &noopStrategy{name: "LiquiditySweepReversal"}
}

// NewFVGRetestAlpha returns a no-op stub.
func NewFVGRetestAlpha() Strategy { return &noopStrategy{name: "FVGRetest"} }

// NewOrderBlockRetestAlpha returns a no-op stub.
func NewOrderBlockRetestAlpha() Strategy { return &noopStrategy{name: "OrderBlockRetest"} }

// NewMSSContinuationAlpha returns a no-op stub.
func NewMSSContinuationAlpha() Strategy { return &noopStrategy{name: "MSSContinuation"} }

// NewPOCBounceAlpha returns a no-op stub.
func NewPOCBounceAlpha() Strategy { return &noopStrategy{name: "POCBounce"} }

// NewSessionExpansionAlpha returns a no-op stub.
func NewSessionExpansionAlpha() Strategy { return &noopStrategy{name: "SessionExpansion"} }

// NewLiquidationCascadeAlpha returns a no-op stub.
func NewLiquidationCascadeAlpha() Strategy { return &noopStrategy{name: "LiquidationCascade"} }

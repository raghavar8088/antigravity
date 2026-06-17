package execution

import "math"

// SlippageModel computes realistic, volatility- and size-scaled slippage for
// live paper execution. It replaces the previous fixed-bps assumption used
// for IOC fills and exits, mirroring (in spirit, not by import — that package
// is backtest-only) the volatility/impact scaling already used by the
// backtest engine's slippage model (internal/backtest/execution/slippage.go),
// so live paper P&L is held to the same realism standard as backtested
// results instead of looking artificially better.
type SlippageModel struct {
	BaseSlippageBps  float64 // floor, applied even in calm/liquid conditions
	ATRScalingFactor float64 // how strongly ATR-relative-to-price scales slippage
	MaxSlippageBps   float64 // hard cap
}

// NewSlippageModel returns a model with sane defaults: 1.2 bps base (matches
// the previous fixed IOC assumption it replaces), 1.0 ATR scaling, 10 bps cap.
func NewSlippageModel() *SlippageModel {
	return &SlippageModel{
		BaseSlippageBps:  1.2,
		ATRScalingFactor: 1.0,
		MaxSlippageBps:   10.0,
	}
}

// ComputeSlippageBps estimates adverse slippage in basis points for an order
// of orderSizeUSD given the current ATR, price, and average volume (BTC).
// atr <= 0 or avgVolume <= 0 disable the corresponding scaling term (the
// result degrades gracefully to BaseSlippageBps when no market data is fed
// yet), so this never produces less realistic results than the fixed-bps
// model it replaces.
func (m *SlippageModel) ComputeSlippageBps(orderSizeUSD, atr, price, avgVolume float64) float64 {
	base := m.BaseSlippageBps
	if base <= 0 {
		base = 1.2
	}
	bps := base

	if atr > 0 && price > 0 {
		atrPct := atr / price
		scaling := m.ATRScalingFactor
		if scaling <= 0 {
			scaling = 1.0
		}
		bps *= 1.0 + scaling*(atrPct/0.005) // doubles when ATR = 0.5% of price
	}

	if avgVolume > 0 && price > 0 && orderSizeUSD > 0 {
		volumeUSD := avgVolume * price
		if volumeUSD > 0 {
			impactFactor := orderSizeUSD / volumeUSD
			bps *= 1.0 + impactFactor*10
		}
	}

	cap := m.MaxSlippageBps
	if cap <= 0 {
		cap = 10.0
	}
	return math.Min(bps, cap)
}

// ApplyEntry applies adverse entry slippage to price for the given side.
// isBuy=true means the entry is a BUY (long entry or short cover); adverse
// means paying more.
func ApplyEntry(isBuy bool, price, slippageBps float64) float64 {
	factor := slippageBps / 10000.0
	if isBuy {
		return price * (1 + factor)
	}
	return price * (1 - factor)
}

// ApplyExit applies adverse exit slippage to price for the side being
// closed. isClosingLong=true means a long position is being sold to close
// (adverse = receiving less); false means a short is being bought back to
// close (adverse = paying more).
func ApplyExit(isClosingLong bool, price, slippageBps float64) float64 {
	factor := slippageBps / 10000.0
	if isClosingLong {
		return price * (1 - factor)
	}
	return price * (1 + factor)
}

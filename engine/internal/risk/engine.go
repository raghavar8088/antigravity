package risk

import (
	"errors"
	"fmt"
	"log"
	"math"
	"sync"

	"antigravity-engine/internal/strategy"
)

// RiskProfile represents system constraints applied globally or per-strategy.
type RiskProfile struct {
	MaxPositionBTC  float64
	MaxCapitalUSD   float64
	MaxDailyLossPct float64
}

type RiskEngine struct {
	mu      sync.RWMutex
	profile RiskProfile

	// Trackers
	currentExposureBTC float64
	currentLossUSD     float64
	dailyPnL           float64

	// Dynamic sizing
	lastATR float64 // Updated from market data
}

func NewRiskEngine(p RiskProfile) *RiskEngine {
	return &RiskEngine{
		profile:            p,
		currentExposureBTC: 0,
		currentLossUSD:     0,
	}
}

// Validate safely checks if an algorithmic signal is allowed to hit the exchange.
func (r *RiskEngine) Validate(sig strategy.Signal, currentPrice float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Symbol Check (Bitcoin pairs — supports both Binance and Coinbase formats)
	if sig.Symbol != "BTCUSDT" && sig.Symbol != "BTC-USD" && sig.Symbol != "BTC-USDT" {
		return errors.New("RISK_VIOLATION: Antigravity only supports BTC pairs")
	}

	// 2. Maximum Size Checks
	if sig.Action == strategy.ActionBuy {
		proposedExposure := r.currentExposureBTC + sig.TargetSize
		if proposedExposure > r.profile.MaxPositionBTC {
			return fmt.Errorf("RISK_VIOLATION: Max position exceeded (Has %.4f, Wants %.4f, Max %.4f)",
				r.currentExposureBTC, sig.TargetSize, r.profile.MaxPositionBTC)
		}

		tradeCost := sig.TargetSize * currentPrice
		if tradeCost > r.profile.MaxCapitalUSD {
			return fmt.Errorf("RISK_VIOLATION: Max capital exceeded ($%.2f wants, Max $%.2f)", tradeCost, r.profile.MaxCapitalUSD)
		}
	}

	// 3. Drawdown Check — circuit breaker
	maxLoss := r.profile.MaxCapitalUSD * r.profile.MaxDailyLossPct
	if r.dailyPnL < 0 && math.Abs(r.dailyPnL) >= maxLoss {
		return fmt.Errorf("RISK_VIOLATION: Circuit Breaker! Daily loss $%.2f exceeds limit $%.2f", r.dailyPnL, maxLoss)
	}

	// 4. Correlation guard — if exposure is already > 60% of max, require stronger conviction
	exposureRatio := r.currentExposureBTC / r.profile.MaxPositionBTC
	if exposureRatio > 0.6 && sig.Action == strategy.ActionBuy {
		if sig.Confidence < 0.8 {
			return fmt.Errorf("RISK_VIOLATION: High exposure (%.0f%%), requires confidence > 0.8 (got %.2f)",
				exposureRatio*100, sig.Confidence)
		}
	}

	return nil
}

// NotifyFill updates internal risk metrics after successful execution.
func (r *RiskEngine) NotifyFill(sig strategy.Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if sig.Action == strategy.ActionBuy {
		r.currentExposureBTC += sig.TargetSize
	} else if sig.Action == strategy.ActionSell {
		r.currentExposureBTC -= sig.TargetSize
		if r.currentExposureBTC < 0 {
			r.currentExposureBTC = 0
		}
	}
	log.Printf("[RISK MIDDLEWARE] Updated exposure: %.4f BTC", r.currentExposureBTC)
}

// RecordPnL tracks realized PnL for daily loss limit.
func (r *RiskEngine) RecordPnL(pnl float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dailyPnL += pnl
	if pnl < 0 {
		r.currentLossUSD += math.Abs(pnl)
	}
}

// GetExposure returns current BTC exposure.
func (r *RiskEngine) GetExposure() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentExposureBTC
}

// GetDailyPnL returns cumulative daily PnL.
func (r *RiskEngine) GetDailyPnL() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dailyPnL
}

// ResetDaily clears daily counters.
func (r *RiskEngine) ResetDaily() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dailyPnL = 0
	r.currentLossUSD = 0
	log.Println("[RISK ENGINE] Daily counters reset")
}

// Reset clears all runtime risk counters so the engine starts from a clean slate.
func (r *RiskEngine) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.currentExposureBTC = 0
	r.currentLossUSD = 0
	r.dailyPnL = 0
	r.lastATR = 0
	log.Println("[RISK ENGINE] Full state reset")
}

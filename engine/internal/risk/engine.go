package risk

import (
	"errors"
	"fmt"
	"log"

	"antigravity-engine/internal/strategy"
)

// RiskProfile represents system constraints applied globally or per-strategy.
type RiskProfile struct {
	MaxPositionBTC  float64
	MaxCapitalUSD   float64
	MaxDailyLossPct float64
}

type RiskEngine struct {
	profile RiskProfile
	
	// Trackers (In a real system, these pull from DB or Redis state)
	currentExposureBTC float64
	currentLossUSD     float64
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
	// 1. Symbol Check (Strictly Bitcoin)
	if sig.Symbol != "BTCUSDT" {
		return errors.New("RISK_VIOLATION: Antigravity only supports BTC pairs")
	}

	// 2. Maximum Size Checks
	if sig.Action == strategy.ActionBuy {
		proposedExposure := r.currentExposureBTC + sig.TargetSize
		if proposedExposure > r.profile.MaxPositionBTC {
			return fmt.Errorf("RISK_VIOLATION: Max position exceeded (Has %.2f, Wants %.2f, Max %.2f)", 
				r.currentExposureBTC, sig.TargetSize, r.profile.MaxPositionBTC)
		}

		tradeCost := sig.TargetSize * currentPrice
		if tradeCost > r.profile.MaxCapitalUSD {
			return fmt.Errorf("RISK_VIOLATION: Max capital exceeded ($%.2f wants, Max $%.2f)", tradeCost, r.profile.MaxCapitalUSD)
		}
	}

	// 3. Drawdown Check
	if r.currentLossUSD >= (r.profile.MaxCapitalUSD * r.profile.MaxDailyLossPct) {
		return errors.New("RISK_VIOLATION: Circuit Breaker! Maximum daily loss limit triggered")
	}

	return nil
}

// NotifyFill updates internal risk metrics after successful execution.
func (r *RiskEngine) NotifyFill(sig strategy.Signal) {
	if sig.Action == strategy.ActionBuy {
		r.currentExposureBTC += sig.TargetSize
	} else if sig.Action == strategy.ActionSell {
		r.currentExposureBTC -= sig.TargetSize
		if r.currentExposureBTC < 0 {
			r.currentExposureBTC = 0 // Prevent invalid negative state without explicit shorting
		}
	}
	log.Printf("[RISK MIDDLEWARE] Updated internal exposure: %.4f BTC", r.currentExposureBTC)
}

package risk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sync"

	"antigravity-engine/internal/ledger"
	riskv2 "antigravity-engine/internal/risk/v2"
	"antigravity-engine/internal/strategy"
)

// RiskProfile represents system constraints applied globally or per-strategy.
type RiskProfile struct {
	MaxPositionBTC  float64
	MaxCapitalUSD   float64
	MaxDailyLossPct float64
}

type RiskEngine struct {
	mu        sync.RWMutex
	profile   RiskProfile
	portfolio *PortfolioRiskEngine
	v2        *riskv2.Engine

	// Trackers
	currentExposureBTC float64 // Signed net BTC exposure; negative values represent shorts.
	currentLossUSD     float64
	dailyPnL           float64

	// Dynamic sizing
	lastATR float64 // Updated from market data

	// Ledger event sourcing (optional — nil disables event emission).
	ledger    ledger.Store
	accountID string
}

// WithLedger attaches an event store to the risk engine. After this call every
// Validate() call emits either RISK_APPROVED or RISK_BLOCKED to the ledger,
// creating a permanent, replayable audit trail of every risk decision.
func (r *RiskEngine) WithLedger(store ledger.Store, accountID string) *RiskEngine {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ledger = store
	r.accountID = accountID
	return r
}

// emitRiskEvent appends one risk event to the ledger.
// Must NOT be called while r.mu is held (uses its own context + ledger lock).
func (r *RiskEngine) emitRiskEvent(eventType ledger.EventType, sig strategy.Signal, currentPrice float64, violationReason string) {
	if r.ledger == nil {
		return
	}
	side := "BUY"
	if sig.Action == strategy.ActionSell {
		side = "SELL"
	}
	proposed := r.currentExposureBTC + signedDelta(sig.Action, sig.TargetSize)
	payload := ledger.RiskCheckPayload{
		Symbol:              sig.Symbol,
		Side:                side,
		RequestedSizeBTC:    sig.TargetSize,
		CurrentExposureBTC:  r.currentExposureBTC,
		ProposedExposureBTC: proposed,
		CurrentPriceUSD:     currentPrice,
		ProposedNotionalUSD: math.Abs(proposed) * currentPrice,
		DailyPnLUSD:         r.dailyPnL,
		Confidence:          sig.Confidence,
		ViolationReason:     violationReason,
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateRisk,
		AggregateID:   sig.Symbol,
		AccountID:     r.accountID,
		StrategyID:    "risk-engine",
		Symbol:        sig.Symbol,
		EventType:     eventType,
		Payload:       payload,
		Source:        "risk-engine",
	})
	if err != nil {
		return
	}
	_, _ = r.ledger.Append(context.Background(), ev)
}

func NewRiskEngine(p RiskProfile) *RiskEngine {
	limits := DefaultRiskLimits(p.MaxCapitalUSD)
	if p.MaxCapitalUSD > 0 {
		limits.EquityUSD = p.MaxCapitalUSD
	}
	if p.MaxPositionBTC > 0 && p.MaxCapitalUSD > 0 {
		// Preserve the legacy max capital envelope while the portfolio engine
		// takes over heat, exposure, concentration, and loss limits.
		limits.MaxNetExposurePct = 100
		limits.MaxGrossExposurePct = 200
	}
	return &RiskEngine{
		profile:            p,
		portfolio:          NewPortfolioRiskEngine(limits),
		v2:                 riskv2.NewEngine(limits.EquityUSD),
		currentExposureBTC: 0,
		currentLossUSD:     0,
	}
}

func signedDelta(action strategy.Action, size float64) float64 {
	switch action {
	case strategy.ActionBuy:
		return size
	case strategy.ActionSell:
		return -size
	default:
		return 0
	}
}

// Validate safely checks if an algorithmic signal is allowed to hit the exchange.
// Emits EventRiskApproved or EventRiskBlocked to the ledger after every check,
// creating a permanent audit trail of every risk decision.
func (r *RiskEngine) Validate(sig strategy.Signal, currentPrice float64) error {
	r.mu.RLock()
	validationErr := r.validateLocked(sig, currentPrice)
	r.mu.RUnlock()

	// Emit event outside the read lock to avoid holding both risk + ledger locks.
	if validationErr != nil {
		r.emitRiskEvent(ledger.EventRiskBlocked, sig, currentPrice, validationErr.Error())
	} else {
		r.emitRiskEvent(ledger.EventRiskApproved, sig, currentPrice, "")
	}
	return validationErr
}

// validateLocked performs the actual risk checks. Must be called with r.mu
// held for read. Extracted so Validate() can release the lock before emitting
// the ledger event, preventing any lock-ordering contention.
func (r *RiskEngine) validateLocked(sig strategy.Signal, currentPrice float64) error {

	// 1. Symbol Check (Bitcoin pairs - supports both Binance and Coinbase formats)
	if sig.Symbol != "BTCUSDT" && sig.Symbol != "BTC-USD" && sig.Symbol != "BTC-USDT" {
		return errors.New("RISK_VIOLATION: RAIG only supports BTC pairs")
	}

	proposedExposure := r.currentExposureBTC + signedDelta(sig.Action, sig.TargetSize)
	proposedAbsExposure := math.Abs(proposedExposure)

	// 2. Maximum Size Checks
	if proposedAbsExposure > r.profile.MaxPositionBTC {
		return fmt.Errorf("RISK_VIOLATION: Max position exceeded (Has %.4f, Wants %.4f, Max %.4f)",
			r.currentExposureBTC, proposedExposure, r.profile.MaxPositionBTC)
	}

	if currentPrice > 0 {
		proposedNotional := proposedAbsExposure * currentPrice
		if proposedNotional > r.profile.MaxCapitalUSD {
			return fmt.Errorf("RISK_VIOLATION: Max capital exceeded ($%.2f wants, Max $%.2f)", proposedNotional, r.profile.MaxCapitalUSD)
		}
	}

	// 3. Drawdown Check - circuit breaker
	maxLoss := r.profile.MaxCapitalUSD * r.profile.MaxDailyLossPct
	if r.dailyPnL < 0 && math.Abs(r.dailyPnL) >= maxLoss {
		return fmt.Errorf("RISK_VIOLATION: Circuit Breaker! Daily loss $%.2f exceeds limit $%.2f", r.dailyPnL, maxLoss)
	}

	// 4. Correlation guard - if exposure is already > 60% of max and we are increasing it,
	// require stronger conviction.
	exposureRatio := 0.0
	if r.profile.MaxPositionBTC > 0 {
		exposureRatio = proposedAbsExposure / r.profile.MaxPositionBTC
	}
	if exposureRatio > 0.6 && proposedAbsExposure > math.Abs(r.currentExposureBTC) {
		if sig.Confidence < 0.8 {
			return fmt.Errorf("RISK_VIOLATION: High exposure (%.0f%%), requires confidence > 0.8 (got %.2f)",
				exposureRatio*100, sig.Confidence)
		}
	}

	if r.portfolio != nil {
		order := RiskOrder{
			Symbol:         sig.Symbol,
			Strategy:       "UNSPECIFIED",
			StrategyFamily: "Unknown",
			Exchange:       "paper",
			Side:           sideFromAction(sig.Action),
			EntryPrice:     currentPrice,
			SizeBTC:        sig.TargetSize,
			NotionalUSD:    sig.TargetSize * currentPrice,
			StopLossPrice:  stopLossPriceFromSignal(sig, currentPrice),
			Leverage:       1,
			Confidence:     sig.Confidence,
			Regime:         RegimeUnknown,
		}
		decision := r.portfolio.ValidateTrade(order)
		if !decision.Approved {
			return fmt.Errorf("RISK_VIOLATION: %s", decision.Reason)
		}
	}

	if r.v2 != nil {
		req := riskv2.TradeRequest{
			Symbol:            sig.Symbol,
			Strategy:          "UNSPECIFIED",
			Family:            riskv2.FamilyReserve,
			Side:              sideFromActionV2(sig.Action),
			EntryPrice:        currentPrice,
			StopLossPrice:     stopLossPriceFromSignal(sig, currentPrice),
			RequestedSizeBTC:  sig.TargetSize,
			RequestedLeverage: 1,
			Confidence:        sig.Confidence,
			Exchange:          "paper",
		}
		decision := r.v2.ValidateTrade(req, riskv2.MarketState{Regime: riskv2.RegimeUnknown}, riskv2.StrategyMetrics{Strategy: "UNSPECIFIED", Family: riskv2.FamilyReserve, WinRate: 0.5, ProfitFactor: 1.2, Sharpe: 1.2, OOSProfitFactor: 1.1, OOSExpectancyUSD: 1, HealthScore: 60, TotalTrades: 30})
		if !decision.Approved {
			return decision.Error()
		}
	}

	return nil
}

// NotifyFill updates internal risk metrics after successful execution.
func (r *RiskEngine) NotifyFill(sig strategy.Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currentExposureBTC += signedDelta(sig.Action, sig.TargetSize)
	if math.Abs(r.currentExposureBTC) < 1e-9 {
		r.currentExposureBTC = 0
	}
	log.Printf("[RISK MIDDLEWARE] Updated net exposure: %.4f BTC", r.currentExposureBTC)
}

// RecordPnL tracks realized PnL for daily loss limit.
func (r *RiskEngine) RecordPnL(pnl float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dailyPnL += pnl
	if pnl < 0 {
		r.currentLossUSD += math.Abs(pnl)
	}
	if r.portfolio != nil {
		r.portfolio.RecordPnL(pnl)
	}
}

// GetExposure returns signed net BTC exposure.
func (r *RiskEngine) GetExposure() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentExposureBTC
}

// GetAbsoluteExposure returns the absolute BTC exposure for display purposes.
func (r *RiskEngine) GetAbsoluteExposure() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return math.Abs(r.currentExposureBTC)
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
	r.portfolio = NewPortfolioRiskEngine(DefaultRiskLimits(r.profile.MaxCapitalUSD))
	r.v2 = riskv2.NewEngine(r.profile.MaxCapitalUSD)
	log.Println("[RISK ENGINE] Full state reset")
}

func (r *RiskEngine) Portfolio() *PortfolioRiskEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.portfolio
}

func (r *RiskEngine) V2() *riskv2.Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.v2
}

func sideFromAction(action strategy.Action) Side {
	if action == strategy.ActionSell {
		return SideShort
	}
	return SideLong
}

func sideFromActionV2(action strategy.Action) riskv2.Side {
	if action == strategy.ActionSell {
		return riskv2.SideShort
	}
	return riskv2.SideLong
}

func stopLossPriceFromSignal(sig strategy.Signal, entry float64) float64 {
	if entry <= 0 {
		return 0
	}
	slPct := sig.StopLossPct
	if slPct <= 0 {
		slPct = 1
	}
	if sig.Action == strategy.ActionSell {
		return entry * (1 + slPct/100)
	}
	return entry * (1 - slPct/100)
}

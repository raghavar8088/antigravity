package positions

import (
	"fmt"
	"log"
	"math"

	"antigravity-engine/internal/strategy"
)

const (
	defaultStopLossPct   = 1.0  // 1% protective stop when missing from persistence
	defaultTakeProfitPct = 0.30 // matches ManagerConfig.MinTakeProfitPct
)

// EnsurePositionProtection fills missing SL/TP absolute prices from entry and side.
// Returns an error when protection cannot be derived (zero entry price).
func EnsurePositionProtection(pos *Position) error {
	if pos == nil {
		return fmt.Errorf("nil position")
	}
	if pos.EntryPrice <= 0 {
		return fmt.Errorf("position %s has invalid entry price %.4f", pos.ID, pos.EntryPrice)
	}

	slPct := pos.StopLossPct
	tpPct := pos.TakeProfitPct
	if slPct <= 0 {
		slPct = defaultStopLossPct
		pos.StopLossPct = slPct
	}
	if tpPct <= 0 {
		tpPct = defaultTakeProfitPct
		pos.TakeProfitPct = tpPct
	}

	needsSL := pos.StopLoss <= 0 || !isValidStop(pos.Side, pos.EntryPrice, pos.StopLoss)
	needsTP := pos.TakeProfit <= 0 || !isValidTakeProfit(pos.Side, pos.EntryPrice, pos.TakeProfit)

	if !needsSL && !needsTP {
		return nil
	}

	sl, tp := absoluteTargets(pos.Side, pos.EntryPrice, slPct, tpPct)
	if needsSL {
		pos.StopLoss = sl
		log.Printf("[POSITION PROTECTION] %s restored SL=%.2f (%.2f%%)", pos.ID, pos.StopLoss, slPct)
	}
	if needsTP {
		pos.TakeProfit = tp
		log.Printf("[POSITION PROTECTION] %s restored TP=%.2f (%.2f%%)", pos.ID, pos.TakeProfit, tpPct)
	}
	return nil
}

func absoluteTargets(side strategy.Action, entry, slPct, tpPct float64) (stopLoss, takeProfit float64) {
	if side == strategy.ActionBuy {
		stopLoss = entry * (1 - slPct/100)
		takeProfit = entry * (1 + tpPct/100)
	} else {
		stopLoss = entry * (1 + slPct/100)
		takeProfit = entry * (1 - tpPct/100)
	}
	return stopLoss, takeProfit
}

func isValidStop(side strategy.Action, entry, stop float64) bool {
	if side == strategy.ActionBuy {
		return stop > 0 && stop < entry
	}
	return stop > entry
}

func isValidTakeProfit(side strategy.Action, entry, tp float64) bool {
	if side == strategy.ActionBuy {
		return tp > entry
	}
	return tp > 0 && tp < entry
}

// ValidateOpenSignal ensures new positions always carry protective targets.
func ValidateOpenSignal(sig strategy.Signal) error {
	if sig.StopLossPct <= 0 {
		return fmt.Errorf("stop_loss_pct must be > 0 (got %.4f)", sig.StopLossPct)
	}
	if sig.TakeProfitPct <= 0 {
		return fmt.Errorf("take_profit_pct must be > 0 (got %.4f)", sig.TakeProfitPct)
	}
	if sig.TargetSize <= 0 || math.IsNaN(sig.TargetSize) {
		return fmt.Errorf("target_size must be > 0")
	}
	return nil
}

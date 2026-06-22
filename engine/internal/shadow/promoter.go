package shadow

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// WalkForwardLookup is the subset of scalpers.WalkForwardValidator the
// promoter needs. scalpers does not import trading or shadow, so this
// package can depend on it directly without an import cycle.
type WalkForwardLookup interface {
	GetStatus(strategyName string) scalers.StrategyStatus
}

// Promotion thresholds — intentionally identical to the live walk-forward
// bar (scalpers.walkForwardWinRateThreshold / walkForwardSharpeThreshold)
// so a promoted strategy has cleared the same statistical confidence bar a
// strategy would need to stay ACTIVE once live.
const (
	PromotionMinTrades      = 30
	PromotionMinWinRate     = 0.48
	PromotionMinSharpe      = 0.60
	promotionAuditMaxLength = 500
)

// PromotionAuditEntry records a single promote/demote action for the
// dashboard's audit trail.
type PromotionAuditEntry struct {
	StrategyName string    `json:"strategy_name"`
	Action       string    `json:"action"` // "PROMOTE" | "DEMOTE"
	Reason       string    `json:"reason"`
	At           time.Time `json:"at"`
}

// ShadowPromoter decides whether a shadow strategy has earned live trading
// status and flips the STRATEGY_LIVE_OVERRIDE knob that rollout_phase.go
// reads — no code deploy required.
type ShadowPromoter struct {
	mu      sync.Mutex
	ledger  *ShadowLedger
	wf      WalkForwardLookup
	overrides map[string]bool // in-process mirror of STRATEGY_LIVE_OVERRIDE for fast reads
	audit   []PromotionAuditEntry
}

// NewShadowPromoter wires a promoter against the shadow ledger and the live
// walk-forward validator (status check) used to confirm a strategy has
// passed its consecutive-window promotion bar in addition to the shadow
// performance thresholds.
func NewShadowPromoter(ledger *ShadowLedger, wf WalkForwardLookup) *ShadowPromoter {
	p := &ShadowPromoter{
		ledger:    ledger,
		wf:        wf,
		overrides: make(map[string]bool),
	}
	for name := range nameSetFromEnvLocal("STRATEGY_LIVE_OVERRIDE") {
		p.overrides[name] = true
	}
	return p
}

func nameSetFromEnvLocal(key string) map[string]bool {
	v := os.Getenv(key)
	out := make(map[string]bool)
	if v == "" {
		return out
	}
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out[p] = true
		}
	}
	return out
}

// CanPromote reports whether strategyName has cleared every promotion gate:
//  1. it currently has shadow trade history (i.e. it is actually in shadow,
//     not already live)
//  2. the live walk-forward validator shows it ACTIVE (passed its
//     consecutive-window confirmation)
//  3. shadow WinRate >= PromotionMinWinRate and Sharpe >= PromotionMinSharpe
//  4. shadow TotalTrades >= PromotionMinTrades
//
// Returns the human-readable reason either way so the UI can explain a
// disabled Promote button.
func (p *ShadowPromoter) CanPromote(strategyName string) (bool, string) {
	p.mu.Lock()
	alreadyLive := p.overrides[strategyName]
	p.mu.Unlock()
	if alreadyLive {
		return false, "strategy is already live (STRATEGY_LIVE_OVERRIDE)"
	}

	perf := p.ledger.GetPerformance(strategyName)
	if perf.TotalTrades == 0 {
		return false, "no shadow trade history yet"
	}
	if perf.TotalTrades < PromotionMinTrades {
		return false, fmt.Sprintf("needs %d shadow trades, has %d", PromotionMinTrades, perf.TotalTrades)
	}
	if perf.WinRate < PromotionMinWinRate {
		return false, fmt.Sprintf("win rate %.1f%% below %.0f%% threshold", perf.WinRate*100, PromotionMinWinRate*100)
	}
	if perf.SharpeRatio < PromotionMinSharpe {
		return false, fmt.Sprintf("Sharpe %.2f below %.2f threshold", perf.SharpeRatio, PromotionMinSharpe)
	}
	if p.wf != nil {
		status := p.wf.GetStatus(strategyName)
		if status != scalers.StatusActive {
			return false, fmt.Sprintf("walk-forward status is %s, not ACTIVE (needs consecutive passing windows)", status)
		}
	}
	return true, fmt.Sprintf("%d trades, %.1f%% win rate, Sharpe %.2f — exceeds all promotion thresholds",
		perf.TotalTrades, perf.WinRate*100, perf.SharpeRatio)
}

// Promote moves a strategy from shadow to live. It adds the strategy to the
// in-process STRATEGY_LIVE_OVERRIDE set (read by rollout_phase.go on every
// evaluation) AND persists the change to the process environment so it
// survives until the next deploy without requiring one now. From the very
// next eval cycle the strategy's signals carry IsShadow=false and route to
// the live paper OMS.
func (p *ShadowPromoter) Promote(ctx context.Context, strategyName string) error {
	ok, reason := p.CanPromote(strategyName)
	if !ok {
		return fmt.Errorf("cannot promote %s: %s", strategyName, reason)
	}

	p.mu.Lock()
	p.overrides[strategyName] = true
	p.persistOverridesLocked()
	p.recordAuditLocked(strategyName, "PROMOTE", reason)
	p.mu.Unlock()

	log.Printf("[SHADOW] Promoted %s to LIVE: %s", strategyName, reason)
	return nil
}

// Demote moves a strategy back from live to shadow — useful if a promoted
// strategy starts underperforming. Removing it from STRATEGY_LIVE_OVERRIDE
// causes rollout_phase.go to fall back to its assigned rollout phase (which,
// for a strategy that was promoted out of shadow, is by definition above
// the current phase, so it returns to shadow rather than going fully idle).
func (p *ShadowPromoter) Demote(ctx context.Context, strategyName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.overrides[strategyName] {
		return fmt.Errorf("%s is not currently live via override", strategyName)
	}
	delete(p.overrides, strategyName)
	p.persistOverridesLocked()
	p.recordAuditLocked(strategyName, "DEMOTE", "operator-initiated demotion back to shadow")
	log.Printf("[SHADOW] Demoted %s back to SHADOW", strategyName)
	return nil
}

// persistOverridesLocked writes the current override set back to
// STRATEGY_LIVE_OVERRIDE so rollout_phase.go's env-var read sees it
// immediately. Caller must hold p.mu.
func (p *ShadowPromoter) persistOverridesLocked() {
	names := make([]string, 0, len(p.overrides))
	for n := range p.overrides {
		names = append(names, n)
	}
	os.Setenv("STRATEGY_LIVE_OVERRIDE", strings.Join(names, ","))
}

func (p *ShadowPromoter) recordAuditLocked(name, action, reason string) {
	p.audit = append([]PromotionAuditEntry{{
		StrategyName: name,
		Action:       action,
		Reason:       reason,
		At:           time.Now().UTC(),
	}}, p.audit...)
	if len(p.audit) > promotionAuditMaxLength {
		p.audit = p.audit[:promotionAuditMaxLength]
	}
}

// AuditLog returns the promotion/demotion history, most recent first.
func (p *ShadowPromoter) AuditLog() []PromotionAuditEntry {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]PromotionAuditEntry, len(p.audit))
	copy(out, p.audit)
	return out
}

// IsLive reports whether strategyName is currently forced live via the
// promotion override (independent of its assigned rollout phase).
func (p *ShadowPromoter) IsLive(strategyName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.overrides[strategyName]
}

// EligibleCount returns how many tracked shadow strategies currently pass
// CanPromote — feeds the shadow_promotion_eligible_count gauge.
func (p *ShadowPromoter) EligibleCount() int {
	count := 0
	for _, perf := range p.ledger.AllPerformance() {
		if ok, _ := p.CanPromote(perf.StrategyName); ok {
			count++
		}
	}
	return count
}

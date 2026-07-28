// Package liveengine is the real-money control plane for the Live Engine module:
// the arm/disarm state machine, the server-enforced $100 capital ceiling, the
// one-way auto-disarm triggers, and the audit log. It owns policy, not broker
// I/O — effectors (arm the Delta bridge, close all, read the kill switch) are
// injected as hooks so the controller is decoupled and unit-testable, and so
// there is exactly one place that decides whether live trading is permitted.
//
// Ships DISARMED. Arming is a human action requiring an authenticated,
// typed-confirmation request; it is never automatic and never on boot. Every
// auto-disarm is one-way: re-arming is always a human action.
package liveengine

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// MaxTradableUSD is the hard, server-enforced capital ceiling for the Live
// Engine. The UI cannot raise it; equity above it is not tradeable. This is a
// limit, not a default.
const MaxTradableUSD = 100.0

// defaultMaxDailyLossUSD is the daily realized-loss limit. When the account is
// down this much versus the day's starting equity, the engine auto-disarms and
// closes all. Override with LIVE_ENGINE_MAX_DAILY_LOSS_USD.
const defaultMaxDailyLossUSD = 20.0

func maxDailyLossFromEnv() float64 {
	if v := os.Getenv("LIVE_ENGINE_MAX_DAILY_LOSS_USD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return defaultMaxDailyLossUSD
}

func utcDay(t time.Time) int { return int(t.UTC().Unix() / 86400) }

// ArmConfirmationPhrase is the exact text an operator must type to arm live
// trading. A bare toggle or a URL can never arm the engine — only this phrase,
// submitted through the authenticated arm request, can.
const ArmConfirmationPhrase = "ARM LIVE $100"

// defaultMaxConsecutiveRejects auto-disarms after this many broker rejects in a
// row — a run of rejects means the venue, margin, or product state is wrong and
// continuing would be blind.
const defaultMaxConsecutiveRejects = 3

// State is the arm/disarm state of the live engine.
type State string

const (
	StateDisarmed State = "DISARMED"
	StateArmed    State = "ARMED"
)

// AuditAction enumerates the events written to the immutable audit trail.
type AuditAction string

const (
	ActionArm          AuditAction = "ARM"
	ActionDisarm       AuditAction = "DISARM"
	ActionAutoDisarm   AuditAction = "AUTO_DISARM"
	ActionCloseAll     AuditAction = "CLOSE_ALL"
	ActionRosterChange AuditAction = "ROSTER_CHANGE"
	ActionRejectStreak AuditAction = "REJECT_STREAK"
	ActionArmRejected  AuditAction = "ARM_REJECTED"
	// Operator-driven kill switch from the Live Engine toggle.
	ActionKillSwitchOn  AuditAction = "KILL_SWITCH_ON"
	ActionKillSwitchOff AuditAction = "KILL_SWITCH_OFF"
	// ActionArmRestored records a restart resuming a deliberate human ON state.
	ActionArmRestored AuditAction = "ARM_RESTORED"
	// ActionReconMismatch records an engine-vs-Delta divergence that was observed
	// and surfaced but deliberately did NOT stop the engine.
	ActionReconMismatch AuditAction = "RECON_MISMATCH"
)

// AuditEntry is one immutable record in the audit trail.
type AuditEntry struct {
	At     time.Time   `json:"at"`
	Actor  string      `json:"actor"`
	Action AuditAction `json:"action"`
	Reason string      `json:"reason,omitempty"`
	Detail string      `json:"detail,omitempty"`
}

// Errors surfaced to the arm/disarm API.
var (
	ErrBadConfirmation = fmt.Errorf("liveengine: confirmation phrase does not match — type %q exactly to arm", ArmConfirmationPhrase)
	ErrKillSwitch      = fmt.Errorf("liveengine: cannot arm while the kill switch is active")
	ErrAlreadyArmed    = fmt.Errorf("liveengine: already armed")
	ErrNotConfigured   = fmt.Errorf("liveengine: broker not configured — cannot arm")
)

// Hooks are the injected effectors and probes. All are optional; a nil hook is
// treated conservatively (e.g. a nil killActive is treated as "cannot confirm
// safe" only where noted). Keeping these as function fields lets the controller
// be tested without a live Delta bridge.
type Hooks struct {
	// SetEffectorEnabled arms/disarms the underlying broker bridge. Required to arm.
	SetEffectorEnabled func(enabled bool)
	// IsConfigured reports whether the broker is configured (credentials present).
	IsConfigured func() bool
	// KillSwitchActive reports whether the institutional kill switch is active.
	KillSwitchActive func() bool
	// KillSwitchReason describes why the kill switch is active (empty when clear).
	KillSwitchReason func() string
	// SetKillSwitch activates (halt) or releases (resume) the institutional kill
	// switch on the operator's behalf. Optional; when nil the UI toggle is
	// unavailable and the switch must be driven from the admin surface.
	SetKillSwitch func(ctx context.Context, active bool, actor, reason string) error
	// CloseAll flattens every live position. Must work even if the strategy loop
	// is wedged, so it is called directly, not through the loop.
	CloseAll func(ctx context.Context) (map[string]any, error)
	// AccountEquityUSD reads the real broker equity. Used to snapshot the day's
	// starting equity at arm time for the daily-loss breaker.
	AccountEquityUSD func(ctx context.Context) (float64, error)
	// Now allows tests to control time; defaults to time.Now.
	Now func() time.Time
}

// Controller is the live-engine state machine. Safe for concurrent use.
type Controller struct {
	mu    sync.RWMutex
	state State

	armedBy          string
	armedAt          time.Time
	lastDisarmReason string
	lastDisarmAt     time.Time

	consecutiveRejects    int
	maxConsecutiveRejects int

	maxDailyLossUSD float64
	dayStartEquity  float64
	dayStartDate    int
	dayStartKnown   bool

	audit    []AuditEntry
	maxAudit int

	hooks Hooks
}

// New constructs a DISARMED controller. It never arms on construction, and never
// reads LIVE_ENGINE_AUTO_ENABLE — arming is a human action only.
func New(hooks Hooks) *Controller {
	if hooks.Now == nil {
		hooks.Now = time.Now
	}
	return &Controller{
		state:                 StateDisarmed,
		maxConsecutiveRejects: defaultMaxConsecutiveRejects,
		maxDailyLossUSD:       maxDailyLossFromEnv(),
		maxAudit:              500,
		hooks:                 hooks,
	}
}

func (c *Controller) now() time.Time { return c.hooks.Now().UTC() }

// appendAuditLocked records an audit entry. Caller holds c.mu.
func (c *Controller) appendAuditLocked(actor string, action AuditAction, reason, detail string) {
	c.audit = append(c.audit, AuditEntry{
		At:     c.now(),
		Actor:  actor,
		Action: action,
		Reason: reason,
		Detail: detail,
	})
	if len(c.audit) > c.maxAudit {
		c.audit = c.audit[len(c.audit)-c.maxAudit:]
	}
}

// Arm transitions to ARMED. It requires the exact typed confirmation phrase, a
// configured broker, and an inactive kill switch. Anything else is refused and
// audited as ARM_REJECTED. Arming is idempotent-safe: arming while armed errors
// rather than silently re-arming.
func (c *Controller) Arm(actor, confirmation string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if confirmation != ArmConfirmationPhrase {
		c.appendAuditLocked(actor, ActionArmRejected, "bad_confirmation", "")
		return ErrBadConfirmation
	}
	if c.hooks.IsConfigured != nil && !c.hooks.IsConfigured() {
		c.appendAuditLocked(actor, ActionArmRejected, "not_configured", "")
		return ErrNotConfigured
	}
	if c.hooks.KillSwitchActive != nil && c.hooks.KillSwitchActive() {
		c.appendAuditLocked(actor, ActionArmRejected, "kill_switch_active", "")
		return ErrKillSwitch
	}
	if c.state == StateArmed {
		return ErrAlreadyArmed
	}

	c.state = StateArmed
	c.armedBy = actor
	c.armedAt = c.now()
	c.consecutiveRejects = 0

	// Snapshot the day's starting equity for the daily-loss breaker.
	c.dayStartKnown = false
	if c.hooks.AccountEquityUSD != nil {
		if eq, err := c.hooks.AccountEquityUSD(context.Background()); err == nil && eq > 0 {
			c.dayStartEquity = eq
			c.dayStartDate = utcDay(c.now())
			c.dayStartKnown = true
		}
	}

	if c.hooks.SetEffectorEnabled != nil {
		c.hooks.SetEffectorEnabled(true)
	}
	// A human turned it on: clear any prior stop reason so a restart may resume.
	c.lastDisarmReason = ""
	c.appendAuditLocked(actor, ActionArm, "", fmt.Sprintf("ceiling=$%.0f dailyLossStop=$%.0f", MaxTradableUSD, c.maxDailyLossUSD))
	c.persistArmStateLocked()
	return nil
}

// CheckDailyLoss compares current equity against the day's starting equity and
// auto-disarms when the loss reaches the daily limit. It rolls the baseline over
// at UTC midnight. Returns true when it tripped, so the caller flattens. Call it
// on every monitor tick with the freshest real equity.
func (c *Controller) CheckDailyLoss(currentEquityUSD float64, now time.Time) (tripped bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	day := utcDay(now)
	if !c.dayStartKnown || day != c.dayStartDate {
		c.dayStartEquity = currentEquityUSD
		c.dayStartDate = day
		c.dayStartKnown = true
		return false
	}
	if c.state != StateArmed {
		return false
	}
	loss := c.dayStartEquity - currentEquityUSD
	if loss >= c.maxDailyLossUSD {
		c.disarmLocked("system", ActionAutoDisarm, "daily_loss_breaker",
			fmt.Sprintf("daily loss $%.2f ≥ $%.2f limit (day start $%.2f → now $%.2f)",
				loss, c.maxDailyLossUSD, c.dayStartEquity, currentEquityUSD))
		return true
	}
	return false
}

// DailyLossStatus reports the loss limit and today's loss so far (0 if unknown).
func (c *Controller) DailyLossStatus() (limitUSD, lossTodayUSD, dayStartEquityUSD float64, known bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.maxDailyLossUSD, 0, c.dayStartEquity, c.dayStartKnown
}

// Disarm is a manual, human disarm. Always allowed; always audited.
func (c *Controller) Disarm(actor, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disarmLocked(actor, ActionDisarm, reason, "")
}

// AutoDisarm is triggered by a safety condition (breaker, reject streak,
// reconciliation mismatch, stale data, feed loss). It is one-way: re-arming
// always requires a human Arm call. Safe to call when already disarmed (no-op
// beyond recording, so triggers can fire idempotently).
func (c *Controller) AutoDisarm(reason, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == StateDisarmed {
		return
	}
	c.disarmLocked("system", ActionAutoDisarm, reason, detail)
}

func (c *Controller) disarmLocked(actor string, action AuditAction, reason, detail string) {
	c.state = StateDisarmed
	c.lastDisarmReason = reason
	c.lastDisarmAt = c.now()
	if c.hooks.SetEffectorEnabled != nil {
		c.hooks.SetEffectorEnabled(false)
	}
	c.appendAuditLocked(actor, action, reason, detail)
	// Persist OFF (with its reason) so a restart cannot resurrect a stopped
	// engine — especially one stopped by a safety trigger.
	c.persistArmStateLocked()
}

// IsArmed reports whether live orders are currently permitted.
func (c *Controller) IsArmed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state == StateArmed
}

// TradableEquityUSD applies the hard ceiling: never more than MaxTradableUSD is
// tradeable, regardless of real account equity. Excess equity is not tradeable.
func (c *Controller) TradableEquityUSD(realEquityUSD float64) float64 {
	if realEquityUSD < 0 {
		return 0
	}
	if realEquityUSD > MaxTradableUSD {
		return MaxTradableUSD
	}
	return realEquityUSD
}

// RecordReject increments the consecutive-reject counter and auto-disarms when
// it reaches the threshold. Call on every broker reject.
func (c *Controller) RecordReject(detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutiveRejects++
	if c.state == StateArmed && c.consecutiveRejects >= c.maxConsecutiveRejects {
		c.disarmLocked("system", ActionAutoDisarm, "consecutive_broker_rejects",
			fmt.Sprintf("%d rejects in a row; last: %s", c.consecutiveRejects, detail))
	}
}

// RecordFillOK resets the consecutive-reject counter after a successful fill.
func (c *Controller) RecordFillOK() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.consecutiveRejects = 0
}

// NoteReconciliationMismatch records a divergence between engine state and Delta
// truth WITHOUT stopping the engine.
//
// A count mismatch is usually transient rather than dangerous: the adoption
// sweep pulls untracked positions into custody on the next tick, and an order
// filling between the two API reads produces a false positive. Halting on it
// stopped live trading repeatedly for benign reasons. The divergence is still
// surfaced loudly — logged, audited, and shown in red on the module — so it can
// never pass unnoticed; it simply no longer disarms by itself.
//
// The other safety stops are unchanged and still one-way: daily-loss breaker,
// consecutive broker rejects, stale market data, price-feed loss, kill switch.
func (c *Controller) NoteReconciliationMismatch(detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appendAuditLocked("system", ActionReconMismatch, "reconciliation_mismatch", detail)
}

// OnReconciliationMismatch auto-disarms: engine state and Delta truth diverged.
// Retained for callers that explicitly want a halt; the live monitor uses
// NoteReconciliationMismatch instead.
func (c *Controller) OnReconciliationMismatch(detail string) {
	c.AutoDisarm("reconciliation_mismatch", detail)
}

// OnStaleMarketData auto-disarms when market data is older than maxAge.
func (c *Controller) OnStaleMarketData(age, maxAge time.Duration) {
	if age > maxAge {
		c.AutoDisarm("stale_market_data", fmt.Sprintf("age=%s > max=%s", age, maxAge))
	}
}

// OnPriceFeedLost auto-disarms on loss of the price feed.
func (c *Controller) OnPriceFeedLost(detail string) {
	c.AutoDisarm("price_feed_lost", detail)
}

// OnDailyLossBreaker auto-disarms when the daily-loss circuit breaker trips.
func (c *Controller) OnDailyLossBreaker(detail string) {
	c.AutoDisarm("daily_loss_breaker", detail)
}

// CloseAll flattens all live positions and audits it. Works regardless of arm
// state and does not depend on the strategy loop.
func (c *Controller) CloseAll(ctx context.Context, actor string) (map[string]any, error) {
	c.mu.Lock()
	c.appendAuditLocked(actor, ActionCloseAll, "", "")
	closeAll := c.hooks.CloseAll
	c.mu.Unlock()

	if closeAll == nil {
		return nil, fmt.Errorf("liveengine: close-all not wired")
	}
	return closeAll(ctx)
}

// SetKillSwitch halts (active=true) or resumes (active=false) trading via the
// institutional kill switch, auditing the action. Halting also disarms the Live
// Engine immediately — an armed engine must never sit behind an active halt.
func (c *Controller) SetKillSwitch(ctx context.Context, active bool, actor, reason string) error {
	c.mu.RLock()
	fn := c.hooks.SetKillSwitch
	c.mu.RUnlock()
	if fn == nil {
		return fmt.Errorf("liveengine: kill switch control not wired")
	}
	if reason == "" {
		if active {
			reason = "manual halt from Live Engine"
		} else {
			reason = "manual resume from Live Engine"
		}
	}
	if err := fn(ctx, active, actor, reason); err != nil {
		return err
	}
	if active {
		// Halting must not leave the engine armed.
		c.mu.Lock()
		if c.state == StateArmed {
			c.disarmLocked(actor, ActionAutoDisarm, "kill_switch_active", reason)
		}
		c.appendAuditLocked(actor, ActionKillSwitchOn, reason, "")
		c.mu.Unlock()
		return nil
	}
	c.mu.Lock()
	c.appendAuditLocked(actor, ActionKillSwitchOff, reason, "")
	c.mu.Unlock()
	return nil
}

// RecordRosterChange audits a roster eligibility change with actor and timestamp.
func (c *Controller) RecordRosterChange(actor, detail string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.appendAuditLocked(actor, ActionRosterChange, "", detail)
}

// StateSnapshot is the read model for the UI.
type StateSnapshot struct {
	State                 State     `json:"state"`
	Armed                 bool      `json:"armed"`
	ArmedBy               string    `json:"armedBy,omitempty"`
	ArmedAt               time.Time `json:"armedAt,omitempty"`
	LastDisarmReason      string    `json:"lastDisarmReason,omitempty"`
	LastDisarmAt          time.Time `json:"lastDisarmAt,omitempty"`
	ConsecutiveRejects    int       `json:"consecutiveRejects"`
	MaxConsecutiveRejects int       `json:"maxConsecutiveRejects"`
	CeilingUSD            float64   `json:"ceilingUsd"`
	MaxDailyLossUSD       float64   `json:"maxDailyLossUsd"`
	DayStartEquityUSD     float64   `json:"dayStartEquityUsd"`
	Configured            bool      `json:"configured"`
	KillSwitchActive      bool      `json:"killSwitchActive"`
	KillSwitchReason      string    `json:"killSwitchReason,omitempty"`
	// KillSwitchControllable is true when the UI may toggle the kill switch.
	KillSwitchControllable bool `json:"killSwitchControllable"`
}

// Snapshot returns the current state for display.
func (c *Controller) Snapshot() StateSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	configured := true
	if c.hooks.IsConfigured != nil {
		configured = c.hooks.IsConfigured()
	}
	kill := false
	if c.hooks.KillSwitchActive != nil {
		kill = c.hooks.KillSwitchActive()
	}
	killReason := ""
	if kill && c.hooks.KillSwitchReason != nil {
		killReason = c.hooks.KillSwitchReason()
	}
	return StateSnapshot{
		State:                 c.state,
		Armed:                 c.state == StateArmed,
		ArmedBy:               c.armedBy,
		ArmedAt:               c.armedAt,
		LastDisarmReason:      c.lastDisarmReason,
		LastDisarmAt:          c.lastDisarmAt,
		ConsecutiveRejects:    c.consecutiveRejects,
		MaxConsecutiveRejects: c.maxConsecutiveRejects,
		CeilingUSD:            MaxTradableUSD,
		MaxDailyLossUSD:       c.maxDailyLossUSD,
		DayStartEquityUSD:     c.dayStartEquity,
		Configured:             configured,
		KillSwitchActive:       kill,
		KillSwitchReason:       killReason,
		KillSwitchControllable: c.hooks.SetKillSwitch != nil,
	}
}

// Audit returns a copy of the audit trail, newest last.
func (c *Controller) Audit() []AuditEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]AuditEntry, len(c.audit))
	copy(out, c.audit)
	return out
}

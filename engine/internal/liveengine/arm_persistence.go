package liveengine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Arm-state continuity across restarts.
//
// The engine used to always boot off, so every deploy silently stopped live
// trading until a human noticed. That is its own failure mode: the operator can
// believe the desk is running when it is not. This file makes the ON/OFF
// intention survive a restart — but only when it was a deliberate human ON.
//
// The guards below are what keep "auto-disarm is one-way" true. A restart may
// resume trading ONLY if all of these hold:
//
//	1. the engine was ARMED when the state was saved (a human turned it on);
//	2. it was not turned off by a safety trigger (daily-loss breaker, reject
//	   streak, reconciliation mismatch, stale data, feed loss, kill switch);
//	3. the kill switch is clear right now;
//	4. the broker is configured;
//	5. the saved state is fresh (armStateMaxAge) — a box that has been down for
//	   a long time must not wake up and start trading on stale intent.
//
// The day's loss baseline is carried across the restart when it is the same UTC
// day, so the $20 daily stop cannot be reset by restarting.

const armStateFileName = "live_engine_arm_state.json"

// armStateMaxAge bounds how stale a saved ARMED state may be and still resume.
// Beyond this a human must turn it back on.
const armStateMaxAge = 6 * time.Hour

// armStateHeartbeat re-saves the ON state on a timer so SavedAt tracks "last
// known alive", not "when a human armed it".
//
// Without the heartbeat the freshness guard measured the wrong thing: an engine
// armed 12h ago and running continuously ever since would refuse to resume after
// a 60-second redeploy, because SavedAt was only written on arm/disarm. That is
// the opposite of the guard's intent — it exists to stop a box that has been DOWN
// for hours from waking up on stale intent. With the heartbeat, now-SavedAt is
// the actual downtime, so the check finally means what it says.
const armStateHeartbeat = 5 * time.Minute

type persistedArmState struct {
	Armed          bool      `json:"armed"`
	ArmedBy        string    `json:"armedBy"`
	ArmedAt        time.Time `json:"armedAt"`
	SavedAt        time.Time `json:"savedAt"`
	DayStartEquity float64   `json:"dayStartEquity"`
	DayStartDate   int       `json:"dayStartDate"`
	// DisarmReason is empty for a human ON. Any non-empty value means the engine
	// was turned off (manually or by a safety trigger) and must NOT auto-resume.
	DisarmReason string `json:"disarmReason,omitempty"`
}

func armStatePath() string {
	dir := strings.TrimSpace(os.Getenv("ENGINE_DATA_DIR"))
	if dir == "" {
		dir = "./data"
	}
	return filepath.Join(dir, armStateFileName)
}

// persistArmStateLocked writes the current ON/OFF intention. Caller holds c.mu.
func (c *Controller) persistArmStateLocked() {
	path := armStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("[LIVE ENGINE] arm-state: mkdir failed: %v", err)
		return
	}
	st := persistedArmState{
		Armed:          c.state == StateArmed,
		ArmedBy:        c.armedBy,
		ArmedAt:        c.armedAt,
		SavedAt:        c.now(),
		DayStartEquity: c.dayStartEquity,
		DayStartDate:   c.dayStartDate,
		DisarmReason:   c.lastDisarmReason,
	}
	data, err := json.Marshal(st)
	if err != nil {
		log.Printf("[LIVE ENGINE] arm-state: marshal failed: %v", err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("[LIVE ENGINE] arm-state: write failed: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("[LIVE ENGINE] arm-state: rename failed: %v", err)
	}
}

// StartArmStateHeartbeat keeps the saved ON state fresh while the engine is
// armed, so a redeploy resumes trading but a long outage still does not. It only
// rewrites an ARMED state: an OFF state must stay off, and a state that a safety
// trigger wrote must keep its DisarmReason and its original timestamp.
func (c *Controller) StartArmStateHeartbeat(ctx context.Context) {
	go func() {
		t := time.NewTicker(armStateHeartbeat)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.mu.Lock()
				if c.state == StateArmed {
					c.persistArmStateLocked()
				}
				c.mu.Unlock()
			}
		}
	}()
}

// RestoreArmState re-enables live trading after a restart when — and only when —
// the engine was deliberately ON before and every guard still holds. Call once at
// startup, after the hooks are wired. Returns true when trading was resumed.
func (c *Controller) RestoreArmState(ctx context.Context) bool {
	data, err := os.ReadFile(armStatePath())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[LIVE ENGINE] arm-state: read failed: %v", err)
		}
		return false
	}
	var st persistedArmState
	if err := json.Unmarshal(data, &st); err != nil {
		log.Printf("[LIVE ENGINE] arm-state: unmarshal failed: %v", err)
		return false
	}

	if !st.Armed {
		log.Printf("[LIVE ENGINE] arm-state: engine was OFF before restart — staying off")
		return false
	}
	// A safety trigger turned it off: auto-disarm is one-way, never auto-resume.
	if st.DisarmReason != "" {
		log.Printf("[LIVE ENGINE] arm-state: previous session ended with %q — a human must turn it back on", st.DisarmReason)
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if age := c.now().Sub(st.SavedAt); age > armStateMaxAge {
		log.Printf("[LIVE ENGINE] arm-state: saved ON state is %s old (> %s) — not auto-resuming", age.Truncate(time.Minute), armStateMaxAge)
		c.appendAuditLocked("system", ActionArmRejected, "stale_arm_state", fmt.Sprintf("age=%s", age.Truncate(time.Minute)))
		return false
	}
	if c.hooks.IsConfigured != nil && !c.hooks.IsConfigured() {
		log.Printf("[LIVE ENGINE] arm-state: broker not configured — not auto-resuming")
		return false
	}
	if c.hooks.KillSwitchActive != nil && c.hooks.KillSwitchActive() {
		log.Printf("[LIVE ENGINE] arm-state: kill switch active — not auto-resuming")
		c.appendAuditLocked("system", ActionArmRejected, "kill_switch_active", "auto-resume blocked")
		return false
	}

	c.state = StateArmed
	c.armedBy = st.ArmedBy
	c.armedAt = st.ArmedAt
	c.consecutiveRejects = 0

	// Carry the day's loss baseline so a restart cannot reset the daily stop.
	today := utcDay(c.now())
	if st.DayStartDate == today && st.DayStartEquity > 0 {
		c.dayStartEquity = st.DayStartEquity
		c.dayStartDate = st.DayStartDate
		c.dayStartKnown = true
	} else if c.hooks.AccountEquityUSD != nil {
		if eq, err := c.hooks.AccountEquityUSD(ctx); err == nil && eq > 0 {
			c.dayStartEquity = eq
			c.dayStartDate = today
			c.dayStartKnown = true
		}
	}

	if c.hooks.SetEffectorEnabled != nil {
		c.hooks.SetEffectorEnabled(true)
	}
	c.appendAuditLocked("system", ActionArmRestored, "restart_resume",
		fmt.Sprintf("resumed the ON state set by %s; dayStartEquity=$%.2f", st.ArmedBy, c.dayStartEquity))
	c.persistArmStateLocked()
	log.Printf("[LIVE ENGINE] ✅ resumed ON after restart (set by %s) — daily stop baseline $%.2f", st.ArmedBy, c.dayStartEquity)
	return true
}

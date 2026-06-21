// Master Strictness Dial — a single 0-100 control that proportionally scales
// every signal-quality-relevant threshold at once, so an operator doesn't
// have to tune 12+ individual values to make the system "let more trades
// through" or "be more selective." Individual threshold cards remain the
// source of truth and stay fully usable for fine-tuning; this is a
// convenience layer on top of the same ThresholdRegistry.Set() path.
package config

import (
	"fmt"
	"math"
)

// StrictnessProfile describes how one threshold moves as the dial moves.
//
//   - "tightens": increasing the dial increases the threshold's value
//     (harder to pass), e.g. MIN_EXECUTABLE_CONFIDENCE.
//   - "loosens":  increasing the dial DECREASES the threshold's value,
//     because a smaller value is the stricter one for this particular
//     threshold, e.g. MAX_SIGNAL_STOP_LOSS_PCT (a narrower allowed stop
//     band is more conservative).
type StrictnessProfile struct {
	ThresholdKey string  `json:"thresholdKey"`
	Direction    string  `json:"direction"` // "tightens" | "loosens"
	LooseValue   float64 `json:"looseValue"`
	CurrentValue float64 `json:"currentValue"` // baseline, pulled live from the registry — never hardcoded
	StrictValue  float64 `json:"strictValue"`
}

// strictnessCatalog is the static LOOSE/STRICT table for every threshold the
// master dial controls. CurrentValue is intentionally absent here — it is
// always read live from the registry at profile-build time (BuildStrictness
// Profiles) so the dial's center point (50) stays meaningful even after an
// individual card has been tuned away from the shipped default.
//
// Only thresholds that represent genuine signal-quality permissiveness are
// included. Capacity limits (MAX_OPEN_LONG_TRADES), timing (cooldowns),
// capital protection (daily loss / intraday drawdown / PMS risk budget),
// position sizing (Kelly caps), and market-state detection (regime ADX/ATR)
// are deliberately excluded — see catalog.go's category comments and the
// threshold-config module's classification notes.
var strictnessCatalog = []struct {
	Key       string
	Direction string
	Loose     float64
	Strict    float64
}{
	// ── signal_quality ──────────────────────────────────────────────────
	// LOOSE sits clearly above the documented 0% win-rate incident boundary
	// (confidence 0.68 combined with an overly tight 0.25×ATR stop produced
	// noise-trade failures) rather than landing on it.
	{Key: "MIN_EXECUTABLE_CONFIDENCE", Direction: "tightens", Loose: 0.58, Strict: 0.80},
	{Key: "MIN_BRIDGE_CONFIDENCE", Direction: "tightens", Loose: 0.55, Strict: 0.78},
	{Key: "MIN_REWARD_TO_RISK_RATIO", Direction: "tightens", Loose: 1.60, Strict: 3.20},
	{Key: "MIN_SIGNAL_TAKE_PROFIT_PCT", Direction: "tightens", Loose: 0.10, Strict: 0.30},
	{Key: "MAX_SIGNAL_STOP_LOSS_PCT", Direction: "loosens", Loose: 2.50, Strict: 0.80},
	{Key: "SCALER_RR_MINIMUM", Direction: "tightens", Loose: 1.50, Strict: 2.80},

	// ── risk_management ─────────────────────────────────────────────────
	// Only the confidence floor that gates signal admission under
	// concentrated exposure is dial-controlled; the exposure threshold
	// itself and all capital-protection limits are not (see exclusions above).
	{Key: "RISK_CORRELATION_MIN_CONFIDENCE", Direction: "tightens", Loose: 0.65, Strict: 0.90},

	// ── walk_forward ────────────────────────────────────────────────────
	// Only the two promotion bars that are actually hot-reloaded from the
	// registry (scalpers.RefreshWalkForwardThresholdsFromRegistry) belong on
	// the master dial. WALKFORWARD_MAX_DRAWDOWN is a reserved no-op (the
	// validator does not track drawdown) and WALKFORWARD_CONSECUTIVE_WINDOWS is
	// read once from env at startup and never refreshed, so neither responded
	// to the dial — they were removed to keep the dial honest. Both remain
	// individually editable via the per-threshold config UI.
	{Key: "WALKFORWARD_WIN_RATE_THRESHOLD", Direction: "tightens", Loose: 0.40, Strict: 0.58},
	{Key: "WALKFORWARD_SHARPE_THRESHOLD", Direction: "tightens", Loose: 0.40, Strict: 0.95},

	// ── execution ───────────────────────────────────────────────────────
	{Key: "MIN_EXECUTION_WEIGHT_TO_TRADE", Direction: "tightens", Loose: 0.35, Strict: 0.70},
}

// driftToleranceAbs is the absolute tolerance used when comparing a live
// registry value against the dial's interpolation formula to decide whether
// the current state "is on the dial" at some clean position. Guards against
// floating-point false positives for drift, not against genuine edits.
const driftToleranceAbs = 0.001

// BuildStrictnessProfiles returns the full StrictnessProfile list with
// CurrentValue populated from each threshold's STABLE shipped default.
//
// CurrentValue is the dial's "CURRENT" anchor — the value that position 50
// interpolates to. It MUST be a fixed reference, not the live registry value.
// Reading it live (the original implementation) made the anchor move every
// time the dial was applied: after applying dial=18 the registry held the
// dial-18 values, those values then became the new "CurrentValue", so
// interpolate(50)==liveValue held again and DetectDialPosition could only ever
// report 50. It also made the "Custom configuration" drift banner impossible
// (position 50 always matched) and caused repeated applies to compound off the
// previous result instead of the baseline.
//
// The shipped default (defaultValueOf) is the stable baseline the LOOSE/STRICT
// endpoints in strictnessCatalog were chosen around, so it is the correct
// anchor. A live individual edit now correctly shows as drift (no clean dial
// position), which is exactly what the drift detection feature is for.
func BuildStrictnessProfiles(reg *ThresholdRegistry) []StrictnessProfile {
	out := make([]StrictnessProfile, 0, len(strictnessCatalog))
	for _, e := range strictnessCatalog {
		baseline := reg.Get(e.Key)
		if m, ok := reg.GetMeta(e.Key); ok {
			baseline = defaultValueOf(m)
		}
		out = append(out, StrictnessProfile{
			ThresholdKey: e.Key,
			Direction:    e.Direction,
			LooseValue:   e.Loose,
			CurrentValue: baseline,
			StrictValue:  e.Strict,
		})
	}
	return out
}

// interpolate maps dialPosition (0-100) to a threshold's value for one
// profile. 0-50 interpolates LOOSE→CURRENT; 50-100 interpolates
// CURRENT→STRICT. Linear in both halves; the two halves need not have the
// same slope (CURRENT is rarely the exact midpoint of LOOSE and STRICT).
func interpolate(p StrictnessProfile, dialPosition float64) float64 {
	d := clampF(dialPosition, 0, 100)
	if d <= 50 {
		t := d / 50
		return p.LooseValue + (p.CurrentValue-p.LooseValue)*t
	}
	t := (d - 50) / 50
	return p.CurrentValue + (p.StrictValue-p.CurrentValue)*t
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ComputeStrictnessValues returns the target value for every dial-controlled
// threshold at the given dial position, keyed by threshold key. Pure
// function — does not touch the registry.
func ComputeStrictnessValues(dialPosition float64, profiles []StrictnessProfile) map[string]float64 {
	out := make(map[string]float64, len(profiles))
	for _, p := range profiles {
		out[p.ThresholdKey] = interpolate(p, dialPosition)
	}
	return out
}

// ThresholdDelta is one threshold's before/after value as part of a batch
// dial application, nested inside a single audit log entry.
type ThresholdDelta struct {
	Key      string  `json:"key" bson:"key"`
	OldValue float64 `json:"oldValue" bson:"old_value"`
	NewValue float64 `json:"newValue" bson:"new_value"`
}

// ApplyStrictnessDial computes every dial-controlled threshold's new value
// at dialPosition and applies them all via registry.Set(), clamping each
// target to that threshold's own [min,max] catalog bounds (the dial's
// LOOSE/STRICT endpoints are already chosen to sit inside those bounds, but
// values are clamped defensively rather than allowed to error mid-batch).
// Returns the deltas for audit logging and the updated ThresholdValue list
// for the HTTP response — it does NOT write the audit entry itself, that is
// the caller's responsibility (HandleApplyStrictness writes ONE entry for
// the whole batch).
func ApplyStrictnessDial(reg *ThresholdRegistry, dialPosition float64) ([]ThresholdDelta, []ThresholdValue, error) {
	if dialPosition < 0 || dialPosition > 100 {
		return nil, nil, fmt.Errorf("config: dial position %v outside [0,100]", dialPosition)
	}
	profiles := BuildStrictnessProfiles(reg)
	targets := ComputeStrictnessValues(dialPosition, profiles)

	deltas := make([]ThresholdDelta, 0, len(profiles))
	updated := make([]ThresholdValue, 0, len(profiles))
	for _, p := range profiles {
		target := targets[p.ThresholdKey]
		m, ok := reg.GetMeta(p.ThresholdKey)
		if ok {
			target = clampF(target, m.Min, m.Max)
		}
		old := reg.Get(p.ThresholdKey)
		uv, err := reg.Set(p.ThresholdKey, target, "strictness_dial")
		if err != nil {
			return nil, nil, fmt.Errorf("config: strictness dial failed applying %s=%v: %w", p.ThresholdKey, target, err)
		}
		deltas = append(deltas, ThresholdDelta{Key: p.ThresholdKey, OldValue: old, NewValue: uv.CurrentValue})
		updated = append(updated, uv)
	}

	RefreshHotReloadHooks()
	return deltas, updated, nil
}

// DetectDialPosition compares the live registry values of every
// dial-controlled threshold against the interpolation formula and returns
// the clean dial position (0-100, in steps of 1) they correspond to, or nil
// if the live values do not match any single position within tolerance
// (i.e. at least one threshold has drifted from the dial via an individual
// edit). Position 50 always matches trivially when nothing has been
// changed, since CurrentValue is read live and interpolate(50)==CurrentValue
// by construction.
func DetectDialPosition(reg *ThresholdRegistry) *float64 {
	profiles := BuildStrictnessProfiles(reg)
	if len(profiles) == 0 {
		return nil
	}
	for pos := 0; pos <= 100; pos++ {
		matchesAll := true
		for _, p := range profiles {
			want := interpolate(p, float64(pos))
			got := reg.Get(p.ThresholdKey)
			if math.Abs(want-got) > driftToleranceAbs {
				matchesAll = false
				break
			}
		}
		if matchesAll {
			d := float64(pos)
			return &d
		}
	}
	return nil
}

// EstimatedImpactPct is a rough, client-displayable estimate of how much
// stricter/looser the dial position is versus today's baseline (dial=50),
// expressed as a simple average of each tightening profile's fractional
// movement toward its STRICT bound. Intentionally coarse — it is a directional
// indicator for the UI's "Estimated impact" line, not a backtested forecast.
func EstimatedImpactPct(dialPosition float64, profiles []StrictnessProfile) float64 {
	if len(profiles) == 0 {
		return 0
	}
	var sum float64
	for _, p := range profiles {
		span := p.StrictValue - p.LooseValue
		if span == 0 {
			continue
		}
		now := interpolate(p, dialPosition)
		baseline := p.CurrentValue
		// Normalize movement toward STRICT as positive "more selective".
		frac := (now - baseline) / span
		if p.Direction == "loosens" {
			frac = -frac
		}
		sum += frac
	}
	avg := sum / float64(len(profiles))
	return avg * 100
}

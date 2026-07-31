package options

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Anti-strategies: an exact P&L mirror of every existing strategy.
//
// # What makes a mirror
//
// Swapping the stop and target ALONE does not invert a result — a long with a
// wider stop and tighter target is still a long, and still loses when price
// falls. Inversion needs both halves:
//
//  1. the side flips (a CALL buyer's mirror is a PUT buyer, which profits from
//     the move the CALL was hurt by), and
//  2. the stop and target distances swap.
//
// Together they mirror exactly. Take a long CALL entered at premium P with
// TP +50% and SL -35%:
//
//	original hits its STOP  -> loses 35%
//	mirror   hits its TARGET-> gains 35%   (mirror TP = original SL)
//	original hits its TARGET-> gains 50%
//	mirror   hits its STOP  -> loses 50%   (mirror SL = original TP)
//
// Every outcome of one is the negation of the other, which is precisely the
// "if the existing one loses, this one gains" behaviour requested.
//
// # The part that decides whether this actually earns
//
// Fees are paid by BOTH sides. Writing G for gross P&L and F for fees:
//
//	original net = G - F
//	mirror   net = -G - F
//	the pair     = -2F
//
// A mirrored pair always loses two lots of fees. So an anti-strategy is
// profitable only when its original has a genuinely NEGATIVE GROSS edge — not
// merely a negative net. A strategy that earns before costs and loses after
// them has a mirror that loses before costs and loses even harder after.
//
// This desk already records gross and net separately, so which originals
// qualify is measurable rather than assumed. Anti-strategies are run to find
// that out, and the promotion gate reads net, so a mirror that only looks good
// gross cannot be promoted.

// AntiPrefix marks a mirrored strategy. Kept as a constant so the UI, the hunt
// leaderboard and the promotion gate all agree on what an anti-strategy is.
const AntiPrefix = "ANTI_"

// IsAnti reports whether a strategy name is a mirror.
func IsAnti(name string) bool {
	return len(name) > len(AntiPrefix) && name[:len(AntiPrefix)] == AntiPrefix
}

// OriginalName strips the prefix, so a mirror can be compared against its source.
func OriginalName(name string) string {
	if !IsAnti(name) {
		return name
	}
	return name[len(AntiPrefix):]
}

// invertType flips CALL to PUT and back. This is what makes the mirror gain
// where the original loses; without it, swapping the exits only changes the
// risk/reward on the same directional bet.
func invertType(t OptionType) OptionType {
	if t == Call {
		return Put
	}
	return Call
}

// BuildAntiStrategies returns one mirror per input strategy.
//
// The entry signal, strike distance, expiry and cooldown are copied unchanged:
// the mirror must trade at the SAME moments as its original, or the two are not
// comparable and the mirror is just another strategy.
func BuildAntiStrategies(defs []StrategyDef) []StrategyDef {
	out := make([]StrategyDef, 0, len(defs))
	for _, d := range defs {
		if IsAnti(d.Name) {
			// Mirroring a mirror returns the original, which would double-count
			// the same hypothesis under a confusing name.
			continue
		}
		anti := d
		anti.Name = AntiPrefix + d.Name
		// IDs must not collide with the originals. Offsetting by a fixed block
		// keeps the mapping obvious in logs and in the leaderboard.
		anti.ID = d.ID + AntiIDOffset
		anti.Type = invertType(d.Type)
		// The swap that completes the mirror.
		anti.TakeProfitPct = d.StopLossPct
		anti.StopLossPct = d.TakeProfitPct
		anti.Category = d.Category
		out = append(out, anti)
	}
	return out
}

// AntiIDOffset separates mirror IDs from original IDs. Large enough that no
// realistic strategy count can reach it.
const AntiIDOffset = 10000

// WithAntiStrategies returns the originals followed by their mirrors.
//
// Idempotent: applying it to a set that already contains mirrors adds nothing.
// Without that guard a second application produced a duplicate mirror for every
// original — same name, same ID — which would split one strategy's trade record
// across two entries and keep BOTH halves under the promotion gate's minimum.
func WithAntiStrategies(defs []StrategyDef) []StrategyDef {
	existing := make(map[string]bool, len(defs))
	for _, d := range defs {
		existing[d.Name] = true
	}

	out := make([]StrategyDef, 0, len(defs)*2)
	out = append(out, defs...)
	for _, a := range BuildAntiStrategies(defs) {
		if existing[a.Name] {
			continue
		}
		existing[a.Name] = true
		out = append(out, a)
	}
	return out
}

// ValidateAntiPairing checks that every mirror is a true inverse of its source.
// Returned as an error rather than a panic so a bad pairing surfaces in tests
// and at boot instead of corrupting a live leaderboard silently.
func ValidateAntiPairing(all []StrategyDef) error {
	byName := make(map[string]StrategyDef, len(all))
	for _, d := range all {
		byName[d.Name] = d
	}
	for _, d := range all {
		if !IsAnti(d.Name) {
			continue
		}
		orig, ok := byName[OriginalName(d.Name)]
		if !ok {
			return fmt.Errorf("anti strategy %q has no original", d.Name)
		}
		if d.Type == orig.Type {
			return fmt.Errorf("%q has the same option type as its original — it would not mirror, only re-risk", d.Name)
		}
		if d.TakeProfitPct != orig.StopLossPct {
			return fmt.Errorf("%q take-profit %.4f should equal the original's stop %.4f",
				d.Name, d.TakeProfitPct, orig.StopLossPct)
		}
		if d.StopLossPct != orig.TakeProfitPct {
			return fmt.Errorf("%q stop %.4f should equal the original's take-profit %.4f",
				d.Name, d.StopLossPct, orig.TakeProfitPct)
		}
		if d.Signal != orig.Signal {
			return fmt.Errorf("%q uses signal %q, not the original's %q — a mirror must trade at the same moments",
				d.Name, d.Signal, orig.Signal)
		}
		if d.ExpiryMinutes != orig.ExpiryMinutes || d.StrikePctOTM != orig.StrikePctOTM {
			return fmt.Errorf("%q does not match its original's strike/expiry selection", d.Name)
		}
	}
	return nil
}

// antiStrategiesEnabled reports whether mirrors should run. Defaults ON: the
// pair is the measurement, and running only the originals answers a narrower
// question than the desk is being asked.
func antiStrategiesEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("ANTI_STRATEGIES"))
	if raw == "" {
		return true
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return true
	}
	return v
}

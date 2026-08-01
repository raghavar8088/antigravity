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
// The exact inverse of "buy this contract at premium P" is "SELL THIS SAME
// CONTRACT at premium P". Same strike, same expiry, same option type, opposite
// side. Every tick then moves the two by equal and opposite amounts.
//
// This originally flipped the option TYPE instead — a long CALL's mirror was a
// long PUT — and swapped the exits. Both halves stayed LONG PREMIUM, which is
// not an inverse at all: a long call and a long put share the sign of theta and
// vega, so both decay as time passes and both gain when volatility rises. In a
// flat market BOTH lose. The pair's combined P&L was not "minus fees" but
// "minus fees minus two lots of decay", and it failed worst in exactly the quiet
// conditions where most of these strategies trade.
//
// # Where the mirror now lives
//
// Not here. A mirror makes no decisions, so it is no longer a strategy that
// evaluates signals and picks contracts: the desk opens it in
// openMirrorLocked() the moment an original fills, on the original's own
// contract, and closes it when the original closes at the same premium. See
// anti_mirror.go.
//
// What remains in this file is the accounting shell — a StrategyDef per mirror
// so it has a name, an ID, a stake and a row on the leaderboard — plus the
// naming contract the desk, the UI and the promotion gate all have to agree on.
//
// The def is a COPY of its original, deliberately including the option type and
// the exits. The type must match because the mirror trades that same contract.
// The exits are inherited from the original at close time and are never read
// from here; leaving them swapped would suggest a policy the mirror does not
// have.
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
// merely a negative net. A strategy that earns before costs and loses after them
// has a mirror that loses before costs and loses even harder after.

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
		// Type is NOT flipped: the mirror sells the SAME contract the original
		// bought. Flipping it produced a long put against a long call — two
		// positions with the same theta and vega sign, which lose together in a
		// flat market instead of cancelling.
		//
		// The exits are NOT swapped either. The mirror inherits its original's
		// exit at close time (see closeMirrorLocked); carrying swapped
		// percentages here would advertise a policy it does not run.
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

// ValidateAntiPairing checks that every mirror still matches its original.
//
// The rules changed when the mirror stopped being a differently-typed strategy
// and became a short of the SAME contract. What has to hold now is that the two
// defs describe one instrument: same option type, same strike and expiry
// selection, same entry signal. The inversion itself is a property of the
// POSITION (OptionPosition.ShortPremium), not of the def, so there is nothing
// about direction to assert here.
//
// This previously required the OPPOSITE type and SWAPPED exits, and would now
// reject every correctly-built mirror. That is worth stating plainly: the
// validator enforced the bug.
//
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
		if d.Type != orig.Type {
			return fmt.Errorf("%q is a %s but its original is a %s — a mirror SELLS the same contract, "+
				"so a different type makes two positions with the same theta and vega sign rather than an inverse",
				d.Name, d.Type, orig.Type)
		}
		if d.TakeProfitPct != orig.TakeProfitPct || d.StopLossPct != orig.StopLossPct {
			return fmt.Errorf("%q carries different exits from its original; a mirror inherits its original's exit at close time and runs none of its own",
				d.Name)
		}
		if d.Signal != orig.Signal {
			return fmt.Errorf("%q uses signal %q, not the original's %q — a mirror must ride the same entries",
				d.Name, d.Signal, orig.Signal)
		}
		if d.ExpiryMinutes != orig.ExpiryMinutes || d.StrikePctOTM != orig.StrikePctOTM {
			return fmt.Errorf("%q does not match its original's strike/expiry selection, so the two are not the same contract", d.Name)
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

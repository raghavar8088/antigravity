package scalpers

import "fmt"

// Anti-strategies for the scalp desk.
//
// Same idea as the option desks' mirrors, but a scalp strategy is behaviour
// rather than data: it emits a Signal on each bar. The mirror therefore wraps
// the original and inverts its output, so the two fire at exactly the same
// moments on exactly the same bars and differ only in direction.
//
// Inversion is the half that matters. Swapping the stop and target alone leaves
// a long as a long — it changes the risk/reward of the same bet, not its sign.
// Flipping the side is what makes the mirror gain where the original loses; the
// swapped exits then make the mirroring exact:
//
//	original LONG, stop -0.35%, target +0.70%
//	  price falls 0.35% -> original stops out (-0.35%)
//	                    -> mirror SHORT hits its target (+0.35%)
//	  price rises 0.70% -> original targets   (+0.70%)
//	                    -> mirror stops out   (-0.70%)
//
// The exit swap lives in the desk's execution profile (see the scalp desk's
// profileCfg), not here: this wrapper owns direction, the profile owns distance.
//
// FEES ARE THE CATCH. Both sides pay them, so a mirrored pair nets -2x fees.
// An anti-strategy earns only when its original has a negative GROSS edge, not
// merely a negative net. A strategy that makes money before costs and loses
// after has a mirror that loses on both counts.

// AntiPrefix marks a mirrored strategy.
const AntiPrefix = "ANTI_"

// IsAntiStrategy reports whether a name is a mirror.
func IsAntiStrategy(name string) bool {
	return len(name) > len(AntiPrefix) && name[:len(AntiPrefix)] == AntiPrefix
}

// OriginalStrategyName strips the prefix so a mirror can be compared to its source.
func OriginalStrategyName(name string) string {
	if !IsAntiStrategy(name) {
		return name
	}
	return name[len(AntiPrefix):]
}

// antiStrategy wraps a strategy and inverts every directional signal it emits.
type antiStrategy struct {
	inner Strategy
	name  string
}

func (a *antiStrategy) Name() string { return a.name }

// ValidRegimes is copied from the original. A mirror must be eligible in the
// same conditions, or it would trade a different subset of bars and stop being
// a comparison.
func (a *antiStrategy) ValidRegimes() []Regime { return a.inner.ValidRegimes() }

// Evaluate runs the original and flips the direction of whatever it returns.
// A no-trade stays a no-trade: a mirror must not invent entries the original
// never took, or the two are no longer the same experiment.
func (a *antiStrategy) Evaluate(ctx MarketContext) Signal {
	s := a.inner.Evaluate(ctx)
	s.Direction = invertDirection(s.Direction)
	return s
}

// invertDirection flips long and short and leaves everything else alone.
func invertDirection(d Direction) Direction {
	switch d {
	case DirectionLong:
		return DirectionShort
	case DirectionShort:
		return DirectionLong
	default:
		// No position, or a value this build does not recognise. Returning it
		// unchanged is the safe reading: inventing a direction here would make
		// the mirror trade where the original stood aside.
		return d
	}
}

// BuildAntiPack returns one mirror per entry.
//
// Metadata is copied so the mirror is scheduled identically — same timeframes,
// same position limit, same data compatibility. Only the name and the direction
// of its signals differ.
func BuildAntiPack(entries []RegistryEntry) []RegistryEntry {
	out := make([]RegistryEntry, 0, len(entries))
	for _, e := range entries {
		if e.Strategy == nil || IsAntiStrategy(e.Name) {
			// Mirroring a mirror returns the original under a confusing name.
			continue
		}
		name := AntiPrefix + e.Name
		anti := e
		anti.Name = name
		anti.Strategy = &antiStrategy{inner: e.Strategy, name: name}
		anti.Description = "Inverse of " + e.Name + " — same entries, opposite side, exits swapped"
		out = append(out, anti)
	}
	return out
}

// WithAntiPack returns the originals followed by their mirrors.
//
// Idempotent: applying it to a set that already contains mirrors adds nothing.
// A duplicate name would split one strategy's record across two entries and keep
// both halves under the promotion gate's minimum.
func WithAntiPack(entries []RegistryEntry) []RegistryEntry {
	existing := make(map[string]bool, len(entries))
	for _, e := range entries {
		existing[e.Name] = true
	}

	out := make([]RegistryEntry, 0, len(entries)*2)
	out = append(out, entries...)
	for _, a := range BuildAntiPack(entries) {
		if existing[a.Name] {
			continue
		}
		existing[a.Name] = true
		out = append(out, a)
	}
	return out
}

// ValidateAntiPack checks every mirror has a source and a unique name, so a
// broken pairing fails loudly instead of quietly splitting a strategy's record.
func ValidateAntiPack(all []RegistryEntry) error {
	seen := map[string]bool{}
	for _, e := range all {
		if seen[e.Name] {
			return fmt.Errorf("duplicate strategy name %q — its trade record would be split", e.Name)
		}
		seen[e.Name] = true
	}
	for _, e := range all {
		if !IsAntiStrategy(e.Name) {
			continue
		}
		if !seen[OriginalStrategyName(e.Name)] {
			return fmt.Errorf("anti strategy %q has no original", e.Name)
		}
		if e.Strategy == nil {
			return fmt.Errorf("anti strategy %q has no implementation", e.Name)
		}
	}
	return nil
}

package scalpers

// Naming for the scalp desk's anti-strategies.
//
// # Why there is no anti-STRATEGY here
//
// There was, briefly: a wrapper that ran the original's Evaluate() and inverted
// the direction, registered alongside it so the desk ran both. It does not work
// on this desk, and the reason is worth keeping.
//
// The scalp desk is post-only. A signal places a limit at the current close and
// the desk fills it on a later bar when price trades through:
//
//	filled := (long && bar.Low <= limit) || (!long && bar.High >= limit)
//
// A long fills when price falls to the limit; a short fills when price rises to
// it. Invert the signal and the mirror posts on the other side of the same
// price, so the two fill on OPPOSITE conditions. Price cannot satisfy both on
// one bar, and over a session the pair almost never both traded — 35 of 53
// traded streams had no partner.
//
// Worse than incomplete, the survivors were biased: a mirror could only fill
// when price moved toward its limit, so it was selected for having started in
// its favour. Four of them reached the top of the leaderboard at a 100% win
// rate on identical P&L, which is what prompted the question that found this.
//
// # What replaced it
//
// The mirror inherits the original's FILL instead of competing for its own. The
// scalp desk opens it in processBar the moment an original fills — same bar,
// same entry price, opposite side, stop and target distances swapped — so a pair
// is an exact inverse by construction rather than by coincidence. See
// openMirror() in cmd/scalp_prelive.
//
// That leaves this file with only the naming contract, which the desk, the
// leaderboard and the promotion gate all have to agree on.
//
// # What an anti-strategy can and cannot earn
//
// Both halves pay fees. Writing G for gross and F for fees:
//
//	original net = G - F
//	mirror   net = -G - F
//	the pair     = -2F
//
// So a mirror is profitable only when its original has a genuinely negative
// GROSS edge. A strategy that earns before costs and loses after has a mirror
// that loses on both counts — the mirror is not a way to profit from a losing
// strategy, only from a wrong one.

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

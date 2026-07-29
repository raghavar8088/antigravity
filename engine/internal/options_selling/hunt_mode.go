package options_selling

import (
	"log"
	"os"
	"strconv"
)

// Hunt mode.
//
// The desk was built as a ROTATING roster: rank 50 strategies, let the best 13
// trade real paper capital, and leave the other 37 shadow-trading. That is the
// right shape for running a desk — concentrate capital on what is working.
//
// It is the wrong shape for a strategy HUNT. The hunt funds every strategy with
// its own $1,000 and asks which of them grows it, and the promotion gate needs
// >=200 real trades per strategy. Under a 13-strategy cap the other 37 never
// place a real trade at all, so they can never reach 200 and are permanently
// ineligible no matter how good they are. The search would only ever be able to
// find something among the 13 that were already winning.
//
// Hunt mode therefore lifts both caps so all strategies trade concurrently, and
// funds the desk to match. Set OPTIONS_HUNT_MODE=false to restore the rotating
// roster.

// huntModeEnabled reports whether every strategy should trade concurrently.
func huntModeEnabled() bool {
	raw := os.Getenv("OPTIONS_HUNT_MODE")
	if raw == "" {
		return true // the hunt is the current purpose of these desks
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("[OPTIONS SELLING] OPTIONS_HUNT_MODE=%q is not a boolean — defaulting to hunt mode ON", raw)
		return true
	}
	return v
}

// maxActiveStrategies is the concurrent-active cap. In hunt mode there is no
// cap: every strategy must be able to build its own record.
func (e *Engine) maxActiveStrategies() int {
	if huntModeEnabled() {
		return len(e.states)
	}
	return optionMaxActiveStrategies
}

// maxStrategiesPerCategory caps how many of one family may be active at once.
// Lifted in hunt mode for the same reason: a category cap silently decides which
// hypotheses are allowed to be tested.
func (e *Engine) maxStrategiesPerCategory() int {
	if huntModeEnabled() {
		return len(e.states)
	}
	return optionMaxStrategiesPerCategory
}

// HuntStakePerStrategy is the per-strategy account the hunt funds.
const HuntStakePerStrategy = 1000.0

// huntDeskBalance is what the desk needs so every strategy can actually hold a
// position at once.
//
// The engine keeps ONE balance and refuses an open when costBasis exceeds it. If
// the desk were funded with a single $1,000 while 50 strategies traded, they
// would starve each other and the "which grew fastest" question would be decided
// by who happened to open first — an artefact of arrival order, not edge.
func huntDeskBalance(strategies int) float64 {
	if strategies <= 0 {
		strategies = 1
	}
	return float64(strategies) * HuntStakePerStrategy
}

package options

import "testing"

// The desk was built as a rotating roster: rank 50 strategies, let the best 13
// trade real paper capital, leave 37 shadow-trading. That is right for running a
// desk and wrong for a hunt — a strategy that never places a real trade can
// never reach the gate's 200-trade minimum, so 37 of 50 were permanently
// ineligible no matter how good they were. The search could only ever find
// something among the 13 already winning.

func TestHuntMode_AllStrategiesCanBeActive(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	e := NewEngine()

	if got, want := e.maxActiveStrategies(), len(e.states); got != want {
		t.Fatalf("maxActiveStrategies = %d, want %d (all of them)", got, want)
	}
	if got, want := e.maxStrategiesPerCategory(), len(e.states); got != want {
		t.Fatalf("maxStrategiesPerCategory = %d, want %d — a category cap silently "+
			"decides which hypotheses may be tested", got, want)
	}
}

// The rotating roster must still be available: it is the right shape for
// actually running a desk once the hunt has picked winners.
func TestHuntMode_OffRestoresRotatingRoster(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "false")
	e := NewEngine()

	if got := e.maxActiveStrategies(); got != optionMaxActiveStrategies {
		t.Errorf("maxActiveStrategies = %d, want the rotating cap %d", got, optionMaxActiveStrategies)
	}
	if got := e.maxStrategiesPerCategory(); got != optionMaxStrategiesPerCategory {
		t.Errorf("maxStrategiesPerCategory = %d, want %d", got, optionMaxStrategiesPerCategory)
	}
}

// A malformed flag must not silently disable the hunt.
func TestHuntMode_BadFlagDefaultsOn(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "yes-please")
	if !huntModeEnabled() {
		t.Fatal("an unparseable OPTIONS_HUNT_MODE must default to hunt mode ON")
	}
}

// One shared $1,000 across 50 strategies means they starve each other, and the
// leaderboard would measure who opened first rather than who has an edge.
func TestHuntMode_DeskIsFundedForEveryStrategy(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	e := NewEngine()

	want := huntDeskBalance(len(e.states))
	if e.balance < want {
		t.Fatalf("desk balance %.0f < %.0f needed to fund %d strategies at $%.0f each",
			e.balance, want, len(e.states), HuntStakePerStrategy)
	}
}

// Every strategy must report its stake, or the UI shows a column of zeros for
// the strategies the old roster left on the bench.
func TestHuntMode_ActiveStrategiesReportTheirStake(t *testing.T) {
	t.Setenv("OPTIONS_HUNT_MODE", "true")
	e := NewEngine()

	statuses := e.StrategyStatuses()
	if len(statuses) == 0 {
		t.Fatal("no strategies built")
	}

	active, staked := 0, 0
	for _, s := range statuses {
		if s.RosterState == StrategyRosterActive {
			active++
			if s.AllocationUSD == HuntStakePerStrategy {
				staked++
			}
		}
	}
	if active == 0 {
		t.Fatal("no strategy became active in hunt mode")
	}
	if staked != active {
		t.Fatalf("%d of %d active strategies report the $%.0f hunt stake",
			staked, active, HuntStakePerStrategy)
	}
}

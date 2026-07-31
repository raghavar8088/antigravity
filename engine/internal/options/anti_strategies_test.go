package options

import "testing"

// A mirror must be an exact inverse: same entries, opposite side, exits swapped.
// Getting only half of that produces a strategy that looks like a hedge and is
// not one — swapping the exits alone leaves a long as a long, and flipping the
// side alone leaves the risk/reward pointing the wrong way.

func TestBuildAntiStrategies_MirrorsEveryStrategy(t *testing.T) {
	defs := buildAllStrategies()
	anti := BuildAntiStrategies(defs)

	if len(anti) != len(defs) {
		t.Fatalf("built %d mirrors for %d strategies", len(anti), len(defs))
	}
	for i, a := range anti {
		o := defs[i]
		if a.Name != AntiPrefix+o.Name {
			t.Errorf("mirror %d named %q, want %q", i, a.Name, AntiPrefix+o.Name)
		}
		if a.Type == o.Type {
			t.Errorf("%s has the same option type as its original — it would not invert P&L", a.Name)
		}
		if a.TakeProfitPct != o.StopLossPct {
			t.Errorf("%s TP %.4f != original SL %.4f", a.Name, a.TakeProfitPct, o.StopLossPct)
		}
		if a.StopLossPct != o.TakeProfitPct {
			t.Errorf("%s SL %.4f != original TP %.4f", a.Name, a.StopLossPct, o.TakeProfitPct)
		}
		// Same entries, or the two are not comparable and the mirror is just
		// another strategy wearing a related name.
		if a.Signal != o.Signal {
			t.Errorf("%s trades signal %q, original trades %q", a.Name, a.Signal, o.Signal)
		}
		if a.ExpiryMinutes != o.ExpiryMinutes || a.StrikePctOTM != o.StrikePctOTM {
			t.Errorf("%s does not match its original's strike/expiry selection", a.Name)
		}
	}
}

// Distinct IDs matter: a collision would merge two strategies' records and make
// both unpromotable through an accounting artefact.
func TestBuildAntiStrategies_IDsDoNotCollide(t *testing.T) {
	all := WithAntiStrategies(buildAllStrategies())
	seenID := map[int]string{}
	seenName := map[string]bool{}
	for _, d := range all {
		if prev, dup := seenID[d.ID]; dup {
			t.Errorf("ID %d used by both %q and %q", d.ID, prev, d.Name)
		}
		seenID[d.ID] = d.Name
		if seenName[d.Name] {
			t.Errorf("duplicate name %q — its trade record would be split", d.Name)
		}
		seenName[d.Name] = true
	}
}

// Mirroring a mirror would return the original under a confusing name.
func TestBuildAntiStrategies_DoesNotMirrorMirrors(t *testing.T) {
	once := WithAntiStrategies(buildAllStrategies())
	twice := WithAntiStrategies(once)
	if len(twice) != len(once) {
		t.Fatalf("re-mirroring added %d strategies; mirrors must not be mirrored", len(twice)-len(once))
	}
}

func TestValidateAntiPairing_AcceptsGeneratedSet(t *testing.T) {
	if err := ValidateAntiPairing(WithAntiStrategies(buildAllStrategies())); err != nil {
		t.Fatalf("generated set failed its own validation: %v", err)
	}
}

// The validator must actually catch a half-built mirror, or it is decoration.
func TestValidateAntiPairing_CatchesSameSideMirror(t *testing.T) {
	base := StrategyDef{
		ID: 1, Name: "S1", Type: Call, TakeProfitPct: 0.5, StopLossPct: 0.35,
		Signal: "sig", ExpiryMinutes: 150,
	}
	broken := base
	broken.ID = 10001
	broken.Name = AntiPrefix + "S1"
	broken.TakeProfitPct, broken.StopLossPct = base.StopLossPct, base.TakeProfitPct
	// Side NOT flipped — the exact half-fix that produces a fake hedge.

	if err := ValidateAntiPairing([]StrategyDef{base, broken}); err == nil {
		t.Fatal("a same-side mirror passed validation; it would not invert P&L")
	}
}

func TestValidateAntiPairing_CatchesUnswappedExits(t *testing.T) {
	base := StrategyDef{ID: 1, Name: "S1", Type: Call, TakeProfitPct: 0.5, StopLossPct: 0.35, Signal: "sig"}
	broken := base
	broken.ID = 10001
	broken.Name = AntiPrefix + "S1"
	broken.Type = Put // side flipped, exits left alone

	if err := ValidateAntiPairing([]StrategyDef{base, broken}); err == nil {
		t.Fatal("a mirror with unswapped exits passed validation")
	}
}

// BuildStrategies must actually SHIP the mirrors, or the generator is dead code.
func TestBuildStrategies_IncludesAntiByDefault(t *testing.T) {
	t.Setenv("ANTI_STRATEGIES", "")
	all := BuildStrategies()

	anti := 0
	for _, d := range all {
		if IsAnti(d.Name) {
			anti++
		}
	}
	if anti == 0 {
		t.Fatal("BuildStrategies shipped no mirrors; the desk would run originals only")
	}
	if anti*2 != len(all) {
		t.Errorf("%d mirrors in a set of %d — expected an even split", anti, len(all))
	}
}

func TestBuildStrategies_AntiCanBeDisabled(t *testing.T) {
	t.Setenv("ANTI_STRATEGIES", "false")
	for _, d := range BuildStrategies() {
		if IsAnti(d.Name) {
			t.Fatalf("mirror %q ran with ANTI_STRATEGIES=false", d.Name)
		}
	}
}

// A malformed flag must not silently halve the desk.
func TestAntiStrategies_BadFlagDefaultsOn(t *testing.T) {
	t.Setenv("ANTI_STRATEGIES", "maybe")
	if !antiStrategiesEnabled() {
		t.Fatal("an unparseable ANTI_STRATEGIES must default ON")
	}
}

func TestOriginalName_RoundTrips(t *testing.T) {
	if got := OriginalName(AntiPrefix + "Foo"); got != "Foo" {
		t.Errorf("OriginalName = %q, want Foo", got)
	}
	if got := OriginalName("Foo"); got != "Foo" {
		t.Errorf("a non-mirror name must pass through unchanged, got %q", got)
	}
}

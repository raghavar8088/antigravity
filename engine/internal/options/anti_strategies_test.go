package options

import "testing"

// An anti-strategy must be an exact P&L inverse of its original.
//
// It was not. Mirrors were built by flipping the option TYPE — a long CALL's
// mirror was a long PUT — with the exits swapped. Both halves stayed LONG
// PREMIUM, and a long call and a long put share the sign of theta and vega: both
// decay as time passes, both gain when volatility rises. In a flat market BOTH
// lose. The pair's combined P&L was not "minus fees" but "minus fees minus two
// lots of decay", and it broke worst in exactly the quiet conditions where most
// of these strategies trade.
//
// The inverse of buying a contract is SELLING THAT SAME CONTRACT. So the def is
// now a copy of its original — same type, same exits, same entries — and the
// inversion lives on the position (OptionPosition.ShortPremium), created at
// fill time by openMirrorLocked.

func TestBuildAntiStrategies_MirrorsTheSameContract(t *testing.T) {
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
		// The type must MATCH. Flipping it is the bug this replaced: it produced
		// two long-premium positions instead of two sides of one contract.
		if a.Type != o.Type {
			t.Errorf("%s is a %s but mirrors a %s — it must sell the SAME contract, "+
				"or the pair shares theta/vega sign instead of cancelling", a.Name, a.Type, o.Type)
		}
		// Exits are inherited at close time, so the def must not advertise
		// different ones.
		if a.TakeProfitPct != o.TakeProfitPct || a.StopLossPct != o.StopLossPct {
			t.Errorf("%s carries exits %.2f/%.2f against the original's %.2f/%.2f; a mirror runs no exit policy of its own",
				a.Name, a.TakeProfitPct, a.StopLossPct, o.TakeProfitPct, o.StopLossPct)
		}
		// Same entries and the same contract selection, or the two are not
		// comparable and the mirror is just another strategy wearing a related
		// name.
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

// The validator must reject the OLD construction, which is what it used to
// require. A differently-typed mirror is two long-premium positions, not an
// inverse.
func TestValidateAntiPairing_RejectsAFlippedType(t *testing.T) {
	base := StrategyDef{
		ID: 1, Name: "S1", Type: Call, TakeProfitPct: 0.5, StopLossPct: 0.35,
		Signal: "sig", ExpiryMinutes: 150,
	}
	broken := base
	broken.ID = 10001
	broken.Name = AntiPrefix + "S1"
	broken.Type = Put // the old, wrong construction

	if err := ValidateAntiPairing([]StrategyDef{base, broken}); err == nil {
		t.Fatal("a put mirroring a call passed validation; both are long premium and lose together in a flat market")
	}
}

// Swapped exits are equally wrong now: the mirror inherits its original's exit
// and runs none of its own, so advertising different percentages describes a
// policy that does not exist.
func TestValidateAntiPairing_RejectsSwappedExits(t *testing.T) {
	base := StrategyDef{ID: 1, Name: "S1", Type: Call, TakeProfitPct: 0.5, StopLossPct: 0.35, Signal: "sig"}
	broken := base
	broken.ID = 10001
	broken.Name = AntiPrefix + "S1"
	broken.TakeProfitPct, broken.StopLossPct = base.StopLossPct, base.TakeProfitPct

	if err := ValidateAntiPairing([]StrategyDef{base, broken}); err == nil {
		t.Fatal("a mirror advertising swapped exits passed validation")
	}
}

func TestValidateAntiPairing_RejectsADifferentSignal(t *testing.T) {
	base := StrategyDef{ID: 1, Name: "S1", Type: Call, TakeProfitPct: 0.5, StopLossPct: 0.35, Signal: "sig"}
	broken := base
	broken.ID = 10001
	broken.Name = AntiPrefix + "S1"
	broken.Signal = "other"

	if err := ValidateAntiPairing([]StrategyDef{base, broken}); err == nil {
		t.Fatal("a mirror riding a different entry signal passed validation")
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

func TestMirrorNameFor_DoesNotMirrorMirrors(t *testing.T) {
	if got := mirrorNameFor("Foo"); got != AntiPrefix+"Foo" {
		t.Errorf("mirrorNameFor(Foo) = %q", got)
	}
	if got := mirrorNameFor(AntiPrefix + "Foo"); got != "" {
		t.Errorf("mirrorNameFor(ANTI_Foo) = %q, want empty", got)
	}
}

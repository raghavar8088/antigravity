package config

import "testing"

// newDialTestRegistry builds a registry seeded with the shipped default for
// every dial-controlled threshold, independent of ambient env vars, so the
// dial round-trip is deterministic.
func newDialTestRegistry(t *testing.T) *ThresholdRegistry {
	t.Helper()
	reg := NewThresholdRegistry()
	reg.RegisterMeta(Catalog())
	for _, e := range strictnessCatalog {
		m, ok := reg.GetMeta(e.Key)
		if !ok {
			t.Fatalf("missing meta for dial key %s", e.Key)
		}
		if _, err := reg.Set(e.Key, defaultValueOf(m), "default"); err != nil {
			t.Fatalf("seed default %s: %v", e.Key, err)
		}
	}
	return reg
}

func dialPos(p *float64) interface{} {
	if p == nil {
		return "nil(drift)"
	}
	return *p
}

// TestStrictnessDialRoundTrip is the regression guard for the bug where the
// dial always read back 50 regardless of what was applied: BuildStrictness
// Profiles used to anchor CurrentValue to the LIVE registry value, so applying
// the dial moved the anchor with it and interpolate(50)==liveValue held again.
// With a stable (default) anchor, an applied position must read back exactly.
func TestStrictnessDialRoundTrip(t *testing.T) {
	reg := newDialTestRegistry(t)

	if pos := DetectDialPosition(reg); pos == nil || *pos != 50 {
		t.Fatalf("baseline dial = %v, want 50", dialPos(pos))
	}

	for _, want := range []float64{18, 30, 75, 50} {
		if _, _, err := ApplyStrictnessDial(reg, want); err != nil {
			t.Fatalf("apply %v: %v", want, err)
		}
		got := DetectDialPosition(reg)
		if got == nil || *got != want {
			t.Fatalf("after apply %v, dial reads %v, want %v (regression: dial not detected)", want, dialPos(got), want)
		}
	}
}

// TestStrictnessDialDoesNotCompound verifies a second apply interpolates from
// the fixed baseline, not from the previous apply's result.
func TestStrictnessDialDoesNotCompound(t *testing.T) {
	reg := newDialTestRegistry(t)

	if _, _, err := ApplyStrictnessDial(reg, 20); err != nil {
		t.Fatalf("apply 20: %v", err)
	}
	deltas, _, err := ApplyStrictnessDial(reg, 50)
	if err != nil {
		t.Fatalf("apply 50: %v", err)
	}
	// Returning to 50 must restore every threshold to its shipped default.
	for _, d := range deltas {
		m, _ := reg.GetMeta(d.Key)
		if got, def := reg.Get(d.Key), defaultValueOf(m); absF(got-def) > driftToleranceAbs {
			t.Fatalf("%s after apply 50 = %v, want default %v", d.Key, got, def)
		}
	}
}

// TestStrictnessDialIndividualEditDrifts confirms a single off-curve edit makes
// the dial report drift (nil) rather than a false clean position.
func TestStrictnessDialIndividualEditDrifts(t *testing.T) {
	reg := newDialTestRegistry(t)
	if _, _, err := ApplyStrictnessDial(reg, 40); err != nil {
		t.Fatalf("apply 40: %v", err)
	}

	const key = "MIN_EXECUTABLE_CONFIDENCE"
	m, _ := reg.GetMeta(key)
	off := clampF(reg.Get(key)+0.013, m.Min, m.Max)
	if _, err := reg.Set(key, off, "manual"); err != nil {
		t.Fatalf("individual edit: %v", err)
	}

	if pos := DetectDialPosition(reg); pos != nil {
		t.Fatalf("after individual edit, dial = %v, want drift (nil)", *pos)
	}
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

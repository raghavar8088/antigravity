package options

import "testing"

func TestBuildStrategiesCount(t *testing.T) {
	// Every strategy now also runs as its mirror, so the shipped set is double
	// the base library. The mirrors are the point, not an accident: the pair is
	// what distinguishes a genuinely negative edge from one that fees ate.
	base := len(buildAllStrategies())
	got := len(BuildStrategies())
	if got != base*2 {
		t.Fatalf("expected %d strategies (%d base + %d mirrors), got %d", base*2, base, base, got)
	}
}

func TestBuildAllStrategiesCount(t *testing.T) {
	if got := len(buildAllStrategies()); got != 50 {
		t.Fatalf("expected 50 base option strategies, got %d", got)
	}
}

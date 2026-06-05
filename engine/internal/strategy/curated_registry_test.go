package strategy

import "testing"

func TestBuildCuratedScalpersHasUniqueNamesAndExpectedCount(t *testing.T) {
	entries := BuildCuratedScalpers()
	if got, want := len(entries), 606; got != want {
		t.Fatalf("expected %d strategies, got %d", want, got)
	}
	if len(entries) <= 30 {
		t.Fatalf("expected full curated roster, got capped count %d", len(entries))
	}

	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		name := entry.Strategy.Name()
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate strategy name detected: %s", name)
		}
		seen[name] = struct{}{}
	}
}

func TestBuildExpansionPackExpectedCount(t *testing.T) {
	if got, want := len(buildExpansionPack()), 301; got != want {
		t.Fatalf("expected %d expansion strategies, got %d", want, got)
	}
}

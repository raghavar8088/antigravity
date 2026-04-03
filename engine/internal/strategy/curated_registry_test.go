package strategy

import "testing"

func TestBuildCuratedScalpersHasUniqueNamesAndExpectedCount(t *testing.T) {
	entries := BuildCuratedScalpers()
	if got, want := len(entries), 600; got != want {
		t.Fatalf("expected %d strategies, got %d", want, got)
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

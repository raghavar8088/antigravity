package ai

import "testing"

func TestAIStrategyLibraryHasExpectedCoverage(t *testing.T) {
	library := GetAIStrategyLibrary()
	if got, want := len(library), 60; got != want {
		t.Fatalf("expected %d AI strategies, got %d", want, got)
	}

	seenIDs := make(map[int]struct{}, len(library))
	seenSlugs := make(map[string]struct{}, len(library))
	for _, strategy := range library {
		if strategy.ID == 0 {
			t.Fatalf("strategy %q missing id", strategy.Name)
		}
		if strategy.Slug == "" {
			t.Fatalf("strategy %q missing slug", strategy.Name)
		}
		if strategy.SupportLevel == "" {
			t.Fatalf("strategy %q missing support level", strategy.Name)
		}
		if _, ok := seenIDs[strategy.ID]; ok {
			t.Fatalf("duplicate strategy id %d", strategy.ID)
		}
		if _, ok := seenSlugs[strategy.Slug]; ok {
			t.Fatalf("duplicate strategy slug %q", strategy.Slug)
		}
		seenIDs[strategy.ID] = struct{}{}
		seenSlugs[strategy.Slug] = struct{}{}
	}
}

func TestAIStrategyLibrarySummaryMatchesLibrary(t *testing.T) {
	summary := SummarizeAIStrategyLibrary()
	if got, want := summary.Total, len(GetAIStrategyLibrary()); got != want {
		t.Fatalf("summary total = %d, want %d", got, want)
	}
	if len(summary.BySupportLevel) == 0 {
		t.Fatal("expected support levels in summary")
	}
	if len(GetAIStrategyCategories()) == 0 {
		t.Fatal("expected at least one AI strategy category")
	}
}

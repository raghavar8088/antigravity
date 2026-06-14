package ai

import "testing"

func TestAIStrategyLibraryIsEmpty(t *testing.T) {
	if got := len(GetAIStrategyLibrary()); got != 0 {
		t.Fatalf("expected no AI strategy blueprints, got %d", got)
	}
}

func TestAIStrategyLibrarySummaryIsEmpty(t *testing.T) {
	summary := SummarizeAIStrategyLibrary()
	if summary.Total != 0 {
		t.Fatalf("summary total = %d, want 0", summary.Total)
	}
	if len(summary.ByCategory) != 0 || len(summary.ByStyle) != 0 || len(summary.BySupportLevel) != 0 {
		t.Fatal("expected empty summary buckets")
	}
	if len(GetAIStrategyCategories()) != 0 {
		t.Fatal("expected no AI strategy categories")
	}
}

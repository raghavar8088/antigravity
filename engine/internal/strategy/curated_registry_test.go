package strategy

import "testing"

func TestBuildCuratedScalpersIsEmpty(t *testing.T) {
	if got := len(BuildCuratedScalpers()); got != 0 {
		t.Fatalf("expected no curated strategies, got %d", got)
	}
}

func TestBuildExpansionPackIsEmpty(t *testing.T) {
	if got := len(buildExpansionPack()); got != 0 {
		t.Fatalf("expected no expansion strategies, got %d", got)
	}
}

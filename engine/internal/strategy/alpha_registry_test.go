package strategy

import "testing"

func TestBuildAllScalpersHasNoAlphaStrategies(t *testing.T) {
	if got := len(BuildAllScalpers()); got != 0 {
		t.Fatalf("expected no registered alpha strategies, got %d", got)
	}
}

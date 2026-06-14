package options

import "testing"

func TestBuildStrategiesIsEmpty(t *testing.T) {
	if got := len(BuildStrategies()); got != 0 {
		t.Fatalf("expected no live option strategies, got %d", got)
	}
}

func TestBuildAllStrategiesIsEmpty(t *testing.T) {
	if got := len(buildAllStrategies()); got != 0 {
		t.Fatalf("expected no base option strategies, got %d", got)
	}
}

package options

import "testing"

func TestBuildStrategiesCount(t *testing.T) {
	if got := len(BuildStrategies()); got != 50 {
		t.Fatalf("expected 50 live option strategies, got %d", got)
	}
}

func TestBuildAllStrategiesCount(t *testing.T) {
	if got := len(buildAllStrategies()); got != 50 {
		t.Fatalf("expected 50 base option strategies, got %d", got)
	}
}

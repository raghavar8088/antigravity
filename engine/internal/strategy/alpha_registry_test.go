package strategy

import "testing"

func TestBuildAllScalpersIncludesInstitutionalAlphaStrategies(t *testing.T) {
	entries := BuildAllScalpers()
	want := map[string]bool{
		"FundingMeanReversion_Alpha":   false,
		"CVDDivergence_Alpha":          false,
		"DeltaAbsorption_Alpha":        false,
		"LiquiditySweepReversal_Alpha": false,
		"FVGRetest_Alpha":              false,
		"OrderBlockRetest_Alpha":       false,
		"MSSContinuation_Alpha":        false,
		"POCBounce_Alpha":              false,
		"SessionExpansion_Alpha":       false,
	}
	for _, entry := range entries {
		if _, ok := want[entry.Strategy.Name()]; ok {
			want[entry.Strategy.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("expected alpha strategy %s in registry", name)
		}
	}
}

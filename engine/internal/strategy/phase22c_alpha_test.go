package strategy

import "testing"

func TestPhase22CAlphaRegistryIsEmpty(t *testing.T) {
	if got := len(BuildCuratedScalpers()); got != 0 {
		t.Fatalf("expected Phase 22C alphas to be absent from registry, got %d entries", got)
	}
}

func TestPhase22CAlphaConstructorsRemainAvailable(t *testing.T) {
	constructors := []Strategy{
		NewFundingMeanReversionAlpha(),
		NewCVDDivergenceAlpha(),
		NewDeltaAbsorptionAlpha(),
		NewLiquiditySweepReversalAlpha(),
		NewFVGRetestAlpha(),
		NewOrderBlockRetestAlpha(),
		NewMSSContinuationAlpha(),
		NewPOCBounceAlpha(),
		NewSessionExpansionAlpha(),
		NewLiquidationCascadeAlpha(),
	}

	for _, strategy := range constructors {
		if strategy.Name() == "" {
			t.Fatal("expected alpha constructor to preserve a strategy name")
		}
	}
}

package strategy

import "testing"

// Phase 22C alphas were retired from live trading; their constructors remain
// (see below) but none of them may appear in the curated registry. The old
// version of this test asserted the ENTIRE registry was empty — obsolete since
// the registry was rebuilt with the whitelist/shadow tiers.
func TestPhase22CAlphasAbsentFromRegistry(t *testing.T) {
	alphaNames := map[string]bool{}
	for _, s := range []Strategy{
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
	} {
		alphaNames[s.Name()] = true
	}
	for _, e := range BuildCuratedScalpers() {
		if alphaNames[e.Strategy.Name()] {
			t.Fatalf("Phase 22C alpha %q must not be in the curated registry", e.Strategy.Name())
		}
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

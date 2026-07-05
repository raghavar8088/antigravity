package strategy

import "testing"

// The curated registry is two-tier gated (tradeEngineEnabled live whitelist +
// tradeEngineShadow ledger tier in scalpers/curated_registry.go). It must be
// non-empty — an empty registry means the trade engine boots with zero
// strategies and silently never trades (the May 2026 "Winners Only Gate"
// incident). The exact membership is asserted in
// scalpers.TestCuratedRegistryMatchesTradeEngineWhitelist.
func TestBuildCuratedScalpersNotEmpty(t *testing.T) {
	if got := len(BuildCuratedScalpers()); got == 0 {
		t.Fatal("BuildCuratedScalpers returned 0 strategies — trade engine would boot with nothing to trade")
	}
}

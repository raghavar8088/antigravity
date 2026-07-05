package scalpers

import (
	"strings"
	"testing"
)

// The Trade Engine registry is whitelist-gated in two tiers:
//   - tradeEngineEnabled: OOS-validated strategies that trade live
//   - tradeEngineShadow:  candidates pinned to the shadow ledger
//
// This test asserts the registry contains exactly the union of both tiers —
// nothing missing, nothing extra — and that the tiers do not overlap.
func TestCuratedRegistryMatchesTradeEngineWhitelist(t *testing.T) {
	for name := range tradeEngineShadow {
		if tradeEngineEnabled[name] {
			t.Fatalf("strategy %q is in BOTH tradeEngineEnabled and tradeEngineShadow", name)
		}
	}

	all := BuildCuratedScalpers()
	t.Logf("total registered strategies: %d (live whitelist %d, shadow tier %d)",
		len(all), len(tradeEngineEnabled), len(tradeEngineShadow))

	byName := map[string]bool{}
	for _, e := range all {
		byName[e.Name] = true
	}

	missing := []string{}
	for name := range tradeEngineEnabled {
		if !byName[name] {
			missing = append(missing, name)
		}
	}
	for name := range tradeEngineShadow {
		if !byName[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing %d whitelisted/shadow strategies from registry: %s", len(missing), strings.Join(missing, ", "))
	}

	extra := []string{}
	for name := range byName {
		if !tradeEngineEnabled[name] && !tradeEngineShadow[name] {
			extra = append(extra, name)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("found %d strategies in registry outside both tiers: %s", len(extra), strings.Join(extra, ", "))
	}

	// Shadow-tier entries must be pinned to shadow at the code level so they
	// can never reach the live OMS regardless of env configuration.
	for _, e := range all {
		if tradeEngineShadow[e.Name] {
			if _, ok := e.Strategy.(*forcedShadowStrategy); !ok {
				t.Fatalf("shadow-tier strategy %q is not wrapped in forcedShadowStrategy", e.Name)
			}
		}
	}
}

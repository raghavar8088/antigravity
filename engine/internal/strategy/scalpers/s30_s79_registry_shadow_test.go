package scalpers

import (
	"strings"
	"testing"
)

// The Trade Engine registry is whitelist-gated (tradeEngineEnabled): only the
// operator-selected strategies from the Strategy Leadership Board are
// registered. This test asserts the registry contains exactly that set —
// nothing missing, nothing extra.
func TestCuratedRegistryMatchesTradeEngineWhitelist(t *testing.T) {
	all := BuildCuratedScalpers()
	t.Logf("total registered strategies: %d", len(all))

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
	if len(missing) > 0 {
		t.Fatalf("missing %d whitelisted strategies from registry: %s", len(missing), strings.Join(missing, ", "))
	}

	extra := []string{}
	for name := range byName {
		if !tradeEngineEnabled[name] {
			extra = append(extra, name)
		}
	}
	if len(extra) > 0 {
		t.Fatalf("found %d non-whitelisted strategies in registry: %s", len(extra), strings.Join(extra, ", "))
	}
}

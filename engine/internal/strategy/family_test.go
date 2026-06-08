package strategy

import (
	"testing"
)

func TestMetadataFromCategory_FamilyWiring(t *testing.T) {
	cases := map[string]StrategyFamily{
		"Trend":        FamilyTrend,
		"Momentum":     FamilyMomentum,
		"Mean Reversion": FamilyMeanReversion,
		"Statistical":  FamilyStatArb,
		"Breakout":     FamilyBreakout,
		"Unknown Cat":  FamilyReserve,
	}
	for category, want := range cases {
		meta := MetadataFromCategory("test_"+category, category)
		if meta.Family != want {
			t.Errorf("category %q: Family=%s want %s", category, meta.Family, want)
		}
		if meta.Tier == "" {
			t.Errorf("category %q: Tier should be set", category)
		}
	}
}

func TestMetadataFromRegistry_FamilyWiring(t *testing.T) {
	entry := RegistryEntry{
		Strategy:  NewEMACrossScalper(8, 21),
		Category:  "Trend",
		Timeframe: "1m",
	}
	meta := MetadataFromRegistry(entry)
	if meta.Family != FamilyTrend {
		t.Fatalf("expected FamilyTrend, got %s", meta.Family)
	}
	if meta.Name != entry.Strategy.Name() {
		t.Fatalf("expected name %q, got %q", entry.Strategy.Name(), meta.Name)
	}
}

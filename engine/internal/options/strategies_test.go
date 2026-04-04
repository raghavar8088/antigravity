package options

import "testing"

func TestBuildStrategiesFiltersWeakShortExpiryAndOTMStrategies(t *testing.T) {
	all := buildAllStrategies()
	live := BuildStrategies()

	if len(all) == 0 {
		t.Fatal("expected full strategy library to be non-empty")
	}
	if len(live) == 0 {
		t.Fatal("expected live-approved strategy set to be non-empty")
	}
	if len(live) >= len(all) {
		t.Fatalf("expected live-approved set (%d) to be smaller than full library (%d)", len(live), len(all))
	}

	for _, def := range live {
		if def.ExpiryMinutes < minLiveExpiryMinutes {
			t.Fatalf("strategy %s should have been filtered for expiry %d", def.Name, def.ExpiryMinutes)
		}
		if def.StrikePctOTM > maxLiveStrikePctOTM {
			t.Fatalf("strategy %s should have been filtered for strike pct %.4f", def.Name, def.StrikePctOTM)
		}
	}
}

func TestBuildStrategiesKeepsBothCallsAndPuts(t *testing.T) {
	live := BuildStrategies()

	var calls, puts int
	for _, def := range live {
		switch def.Type {
		case Call:
			calls++
		case Put:
			puts++
		}
	}

	if calls == 0 || puts == 0 {
		t.Fatalf("expected both calls and puts in live set, got calls=%d puts=%d", calls, puts)
	}
}

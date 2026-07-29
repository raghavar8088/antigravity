package scalpers

import "log"

// BuildHuntPack returns EVERY scalp strategy registered in this application,
// de-duplicated by name.
//
// The scalp desk shipped running BuildScalp100 — 100 of the 151 strategies that
// exist here. The other 51 (17 Delta20 + 34 Curated) were built, registered and
// never run, so the strategy hunt could not have found them however good they
// were. A search that cannot see two thirds of its own candidates is not a
// search.
//
// De-duplication is by strategy name and first-wins, so the packs stay
// composable: adding a pack that overlaps an existing one cannot silently
// produce two accounts for the same strategy, which would split its trade
// record in half and keep both halves below the promotion gate's minimum.
func BuildHuntPack() []RegistryEntry {
	packs := []struct {
		name    string
		entries []RegistryEntry
	}{
		{"Scalp100", BuildScalp100()},
		{"Delta20", BuildDelta20Pack()},
		{"Curated", BuildCuratedScalpers()},
	}

	seen := make(map[string]string, 160)
	out := make([]RegistryEntry, 0, 160)
	dupes := 0

	for _, p := range packs {
		added := 0
		for _, e := range p.entries {
			if from, exists := seen[e.Name]; exists {
				dupes++
				log.Printf("[HUNT PACK] duplicate %q in %s (already from %s) — keeping the first",
					e.Name, p.name, from)
				continue
			}
			seen[e.Name] = p.name
			out = append(out, e)
			added++
		}
		log.Printf("[HUNT PACK] %-9s +%d strategies", p.name, added)
	}

	log.Printf("[HUNT PACK] %d unique strategies across %d packs (%d duplicate name(s) dropped)",
		len(out), len(packs), dupes)
	return out
}

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

	// Mirrors are deliberately NOT added here.
	//
	// They were, briefly, as signal wrappers that inverted each Evaluate() and
	// posted their own order. Under this desk's post-only fill model that does
	// not mirror anything: the original posts a limit on one side and the mirror
	// posts one on the other, and
	//
	//	filled := (long && bar.Low <= limit) || (!long && bar.High >= limit)
	//
	// means they fill on OPPOSITE conditions. Price cannot do both on a bar, so
	// the pair almost never both traded — 35 of 53 traded streams had no partner
	// — and the mirror only ever filled when price moved toward its limit. That
	// is a selection bias dressed as an inverse, and it put four 100%-win-rate
	// rows at the top of the board.
	//
	// A mirror must inherit the original's FILL, not compete for its own. The
	// desk creates it in processBar the moment an original fills, at the same
	// price on the same bar. See mirrorOf().
	return out
}

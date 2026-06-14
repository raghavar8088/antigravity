package strategy

// ── curated_expansion_pack.go ─────────────────────────────────────────────────
// Replaces the empty stub. Currently a pass-through — future strategies
// added to expand the pack go here without touching curated_registry.go.

// buildExpansionPack returns additional strategies for the expansion tier.
// These are added on top of the core 7 strategies in BuildAllScalpers().
// Currently returns the same core set — extend here as new strategies are built.
func buildExpansionPack() []RegistryEntry {
	// Expansion pack is empty at v1.1 — the core 7 strategies are sufficient.
	// Add future strategies here:
	//   return []RegistryEntry{
	//       { Strategy: &FundingRateFade{}, Name: "Funding_Rate_Fade", ... },
	//   }
	return nil
}

// BuildFullRegistry returns the core strategies plus any expansion pack entries.
// Use this for backtesting the full universe — curated_registry filters it down
// for live trading.
func BuildFullRegistry() []RegistryEntry {
	core := BuildAllScalpers()
	expansion := buildExpansionPack()
	return append(core, expansion...)
}

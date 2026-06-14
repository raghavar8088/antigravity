package strategy

// WINNERS_ONLY gate metadata is preserved for callers that report registry
// policy, but no strategies remain registered in this application.
const (
	WinnersOnlyGateActive        = true
	WinnersOnlyGateActivatedDate = "2026-05-01"
)

// FilterWinnersOnly returns an empty strategy list because all strategy
// inventories have been removed.
func FilterWinnersOnly(_ []RegistryEntry) []RegistryEntry {
	return nil
}

// BuildCuratedScalpers returns no strategies.
func BuildCuratedScalpers() []RegistryEntry {
	return nil
}

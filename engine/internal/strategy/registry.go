package strategy

// StrategyTier classifies strategy quality for drawdown-regime enforcement.
type StrategyTier string

const (
	StrategyTierElite        StrategyTier = "ELITE"
	StrategyTierStandard     StrategyTier = "STANDARD"
	StrategyTierExperimental StrategyTier = "EXPERIMENTAL"
)

// StrategyMetadata is the institutional metadata envelope for a registry entry.
type StrategyMetadata struct {
	Name      string
	Category  string
	Timeframe string
	Tier      StrategyTier
	Family    StrategyFamily
}

// MetadataFromRegistry derives StrategyMetadata from a registry entry.
func MetadataFromRegistry(entry RegistryEntry) StrategyMetadata {
	return StrategyMetadata{
		Name:      entry.Strategy.Name(),
		Category:  entry.Category,
		Timeframe: entry.Timeframe,
		Tier:      TierFromCategory(entry.Category),
		Family:    CategoryToRiskFamily(entry.Category),
	}
}

// MetadataFromCategory builds metadata for a live strategy when only the tracker
// category is available (e.g. during executeThroughInstitutionalPath).
func MetadataFromCategory(name, category string) StrategyMetadata {
	return StrategyMetadata{
		Name:     name,
		Category: category,
		Tier:     TierFromCategory(category),
		Family:   CategoryToRiskFamily(category),
	}
}

// TierFromCategory maps a registry category string to its StrategyTier.
func TierFromCategory(category string) StrategyTier {
	return CategoryTier(category)
}

type RegistryEntry struct {
	Strategy  Strategy
	Category  string
	Timeframe string
}

// BuildAllScalpers returns no strategies.
func BuildAllScalpers() []RegistryEntry {
	return nil
}

// StrategyGroups organizes strategies by their intended processing timeframe.
type StrategyGroups struct {
	Tick []RegistryEntry
	M1   []RegistryEntry
	M5   []RegistryEntry
	M15  []RegistryEntry
	H1   []RegistryEntry
}

// GroupByTimeframe separates strategies into processing groups.
func GroupByTimeframe(entries []RegistryEntry) StrategyGroups {
	var groups StrategyGroups
	for _, e := range entries {
		switch e.Timeframe {
		case "tick":
			groups.Tick = append(groups.Tick, e)
		case "5m":
			groups.M5 = append(groups.M5, e)
		case "15m":
			groups.M15 = append(groups.M15, e)
		case "1h":
			groups.H1 = append(groups.H1, e)
		default:
			groups.M1 = append(groups.M1, e)
		}
	}
	return groups
}

func GetStrategyNames() []string {
	entries := BuildAllScalpers()
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Strategy.Name()
	}
	return names
}

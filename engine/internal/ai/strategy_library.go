package ai

type StrategySupportLevel string

const (
	StrategySupportRuleReady    StrategySupportLevel = "RULE_READY"
	StrategySupportExternalData StrategySupportLevel = "EXTERNAL_DATA_REQUIRED"
	StrategySupportBlueprint    StrategySupportLevel = "ENGINE_BLUEPRINT"
)

type AIStrategyBlueprint struct {
	ID               int                  `json:"id"`
	Slug             string               `json:"slug"`
	Name             string               `json:"name"`
	Category         string               `json:"category"`
	Style            string               `json:"style"`
	Timeframe        string               `json:"timeframe"`
	CoreEdge         string               `json:"coreEdge"`
	EntryModel       string               `json:"entryModel"`
	ExitModel        string               `json:"exitModel"`
	RiskModel        string               `json:"riskModel"`
	PrimarySignals   []string             `json:"primarySignals"`
	DataRequirements []string             `json:"dataRequirements"`
	SupportLevel     StrategySupportLevel `json:"supportLevel"`
}

type StrategyLibrarySummary struct {
	Total          int            `json:"total"`
	ByCategory     map[string]int `json:"byCategory"`
	ByStyle        map[string]int `json:"byStyle"`
	BySupportLevel map[string]int `json:"bySupportLevel"`
}

var aiStrategyLibrary = []AIStrategyBlueprint{}

func GetAIStrategyLibrary() []AIStrategyBlueprint {
	out := make([]AIStrategyBlueprint, len(aiStrategyLibrary))
	copy(out, aiStrategyLibrary)
	return out
}

func SummarizeAIStrategyLibrary() StrategyLibrarySummary {
	return StrategyLibrarySummary{
		Total:          len(aiStrategyLibrary),
		ByCategory:     make(map[string]int),
		ByStyle:        make(map[string]int),
		BySupportLevel: make(map[string]int),
	}
}

func BuildAIStrategyCatalogPrompt(_ int) string {
	return "RAIG AI strategy library:\n"
}

func GetAIStrategySlugs() []string {
	return nil
}

func GetAIStrategyCategories() []string {
	return nil
}

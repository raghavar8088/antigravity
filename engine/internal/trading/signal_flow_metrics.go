package trading

import "sync"

const (
	SignalStageGenerated             = "Generated"
	SignalStageAggregator            = "Aggregator"
	SignalStageCooldownFilter        = "Signal Cooldown"
	SignalStageDominanceFilter       = "Dominance Filter"
	SignalStageScoreFilter           = "Score Filter"
	SignalStageCategoryDeduplication = "Category Deduplication"
	SignalStageThroughputCap         = "Throughput Cap"
	SignalStageRegimeFilter          = "Regime Filter"
	SignalStageExecutionWeightFilter = "Execution Weight Filter"
	SignalStageConfidenceFilter      = "Confidence Filter"
	SignalStageRiskFilter            = "Risk Filter"
	SignalStageExecution             = "Execution"
)

type SignalFlowStageMetrics struct {
	Stage        string  `json:"stage"`
	Input        int64   `json:"input"`
	Output       int64   `json:"output"`
	Rejected     int64   `json:"rejected"`
	RejectionPct float64 `json:"rejectionPct"`
}

type SignalFlowSnapshot struct {
	Stages             []SignalFlowStageMetrics `json:"stages"`
	RejectedByReason   map[string]int64         `json:"rejectedByReason"`
	RejectedByCategory map[string]int64         `json:"rejectedByCategory"`
}

type signalFlowCounters struct {
	input    int64
	output   int64
	rejected int64
}

type SignalFlowMetrics struct {
	mu sync.Mutex

	stages             map[string]*signalFlowCounters
	rejectedByReason   map[string]int64
	rejectedByCategory map[string]int64
}

func NewSignalFlowMetrics() *SignalFlowMetrics {
	return &SignalFlowMetrics{
		stages:             make(map[string]*signalFlowCounters),
		rejectedByReason:   make(map[string]int64),
		rejectedByCategory: make(map[string]int64),
	}
}

func (m *SignalFlowMetrics) RecordStage(stage string, input, output int) {
	if m == nil {
		return
	}
	if input < 0 {
		input = 0
	}
	if output < 0 {
		output = 0
	}
	rejected := input - output
	if rejected < 0 {
		rejected = 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	counters := m.stages[stage]
	if counters == nil {
		counters = &signalFlowCounters{}
		m.stages[stage] = counters
	}
	counters.input += int64(input)
	counters.output += int64(output)
	counters.rejected += int64(rejected)
}

func (m *SignalFlowMetrics) RecordRejection(stage, reason, category string) {
	if m == nil {
		return
	}
	key := stage + ": " + reason
	if category == "" {
		category = "unknown"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.rejectedByReason[key]++
	m.rejectedByCategory[category]++
}

func (m *SignalFlowMetrics) Snapshot() SignalFlowSnapshot {
	if m == nil {
		return SignalFlowSnapshot{}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stageOrder := []string{
		SignalStageGenerated,
		SignalStageAggregator,
		SignalStageCooldownFilter,
		SignalStageDominanceFilter,
		SignalStageScoreFilter,
		SignalStageCategoryDeduplication,
		SignalStageThroughputCap,
		SignalStageRegimeFilter,
		SignalStageExecutionWeightFilter,
		SignalStageConfidenceFilter,
		SignalStageRiskFilter,
		SignalStageExecution,
	}

	stages := make([]SignalFlowStageMetrics, 0, len(stageOrder))
	for _, stage := range stageOrder {
		counters := m.stages[stage]
		if counters == nil {
			stages = append(stages, SignalFlowStageMetrics{Stage: stage})
			continue
		}
		rejectionPct := 0.0
		if counters.input > 0 {
			rejectionPct = float64(counters.rejected) / float64(counters.input) * 100
		}
		stages = append(stages, SignalFlowStageMetrics{
			Stage:        stage,
			Input:        counters.input,
			Output:       counters.output,
			Rejected:     counters.rejected,
			RejectionPct: rejectionPct,
		})
	}

	rejectedByReason := make(map[string]int64, len(m.rejectedByReason))
	for key, value := range m.rejectedByReason {
		rejectedByReason[key] = value
	}
	rejectedByCategory := make(map[string]int64, len(m.rejectedByCategory))
	for key, value := range m.rejectedByCategory {
		rejectedByCategory[key] = value
	}

	return SignalFlowSnapshot{
		Stages:             stages,
		RejectedByReason:   rejectedByReason,
		RejectedByCategory: rejectedByCategory,
	}
}

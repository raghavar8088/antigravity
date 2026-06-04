package trading

import (
	"sort"
	"sync"
)

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

// SignalFlowDiagnostics extends SignalFlowSnapshot with per-strategy and
// per-category approval/rejection breakdowns for production observability.
type SignalFlowDiagnostics struct {
	SignalFlowSnapshot

	// Per-strategy breakdown
	ApprovedByStrategy  map[string]int64 `json:"approvedByStrategy"`
	RejectedByStrategy  map[string]int64 `json:"rejectedByStrategy"`
	ExecutedByStrategy  map[string]int64 `json:"executedByStrategy"`

	// Per-category approval counts (complements existing RejectedByCategory)
	ApprovedByCategory map[string]int64 `json:"approvedByCategory"`

	// Top bottleneck stages: stages ordered by rejection count descending
	TopBottlenecks []SignalFlowStageMetrics `json:"topBottlenecks"`

	// Overall funnel summary
	TotalGenerated int64   `json:"totalGenerated"`
	TotalExecuted  int64   `json:"totalExecuted"`
	OverallPassPct float64 `json:"overallPassPct"`
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

	// Per-strategy tracking (Phase 22A diagnostics)
	approvedByStrategy map[string]int64
	rejectedByStrategy map[string]int64
	executedByStrategy map[string]int64
	approvedByCategory map[string]int64
}

func NewSignalFlowMetrics() *SignalFlowMetrics {
	return &SignalFlowMetrics{
		stages:             make(map[string]*signalFlowCounters),
		rejectedByReason:   make(map[string]int64),
		rejectedByCategory: make(map[string]int64),
		approvedByStrategy: make(map[string]int64),
		rejectedByStrategy: make(map[string]int64),
		executedByStrategy: make(map[string]int64),
		approvedByCategory: make(map[string]int64),
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

// RecordStrategyApproval records that a signal from strategyName was approved
// and passed to the execution path.
func (m *SignalFlowMetrics) RecordStrategyApproval(strategyName, category string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.approvedByStrategy[strategyName]++
	m.approvedByCategory[category]++
}

// RecordStrategyRejection records that a signal from strategyName was rejected
// at the named stage.
func (m *SignalFlowMetrics) RecordStrategyRejection(strategyName string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rejectedByStrategy[strategyName]++
}

// RecordStrategyExecution records that an order was actually submitted for a
// strategy signal (post risk-gate, post OMS).
func (m *SignalFlowMetrics) RecordStrategyExecution(strategyName string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executedByStrategy[strategyName]++
}

// Diagnostics returns a full SignalFlowDiagnostics snapshot including the
// per-strategy breakdown and top-bottleneck ranking.
func (m *SignalFlowMetrics) Diagnostics() SignalFlowDiagnostics {
	if m == nil {
		return SignalFlowDiagnostics{}
	}

	snap := m.Snapshot()

	m.mu.Lock()
	defer m.mu.Unlock()

	approved := make(map[string]int64, len(m.approvedByStrategy))
	for k, v := range m.approvedByStrategy {
		approved[k] = v
	}
	rejected := make(map[string]int64, len(m.rejectedByStrategy))
	for k, v := range m.rejectedByStrategy {
		rejected[k] = v
	}
	executed := make(map[string]int64, len(m.executedByStrategy))
	for k, v := range m.executedByStrategy {
		executed[k] = v
	}
	approvedCat := make(map[string]int64, len(m.approvedByCategory))
	for k, v := range m.approvedByCategory {
		approvedCat[k] = v
	}

	// Build top-bottleneck list sorted by rejection count descending.
	bottlenecks := make([]SignalFlowStageMetrics, 0, len(snap.Stages))
	for _, s := range snap.Stages {
		if s.Rejected > 0 {
			bottlenecks = append(bottlenecks, s)
		}
	}
	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].Rejected > bottlenecks[j].Rejected
	})

	var totalGenerated, totalExecuted int64
	for _, s := range snap.Stages {
		if s.Stage == SignalStageGenerated {
			totalGenerated = s.Input
		}
		if s.Stage == SignalStageExecution {
			totalExecuted = s.Output
		}
	}
	overallPassPct := 0.0
	if totalGenerated > 0 {
		overallPassPct = float64(totalExecuted) / float64(totalGenerated) * 100
	}

	return SignalFlowDiagnostics{
		SignalFlowSnapshot:  snap,
		ApprovedByStrategy:  approved,
		RejectedByStrategy:  rejected,
		ExecutedByStrategy:  executed,
		ApprovedByCategory:  approvedCat,
		TopBottlenecks:      bottlenecks,
		TotalGenerated:      totalGenerated,
		TotalExecuted:       totalExecuted,
		OverallPassPct:      overallPassPct,
	}
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

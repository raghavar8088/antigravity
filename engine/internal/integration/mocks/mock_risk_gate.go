package mocks

import "sync"

// RiskDecision is the verdict returned by the MockRiskGate.
type RiskDecision struct {
	Approved bool
	Reason   string
}

// MockRiskGate approves or rejects all signals based on ApproveAll.
type MockRiskGate struct {
	ApproveAll bool // if true, approve all; if false, reject all

	mu             sync.Mutex
	LastSubmission interface{}
}

// Submit records the last submission and returns an approval decision.
func (g *MockRiskGate) Submit(signal interface{}, positionUSD float64, cycleID string) RiskDecision {
	g.mu.Lock()
	g.LastSubmission = signal
	g.mu.Unlock()

	if g.ApproveAll {
		return RiskDecision{Approved: true}
	}
	return RiskDecision{Approved: false, Reason: "mock_risk_rejected"}
}

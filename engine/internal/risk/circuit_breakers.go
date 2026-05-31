package risk

type CircuitState string

const (
	CircuitStateClosed CircuitState = "CLOSED"
	CircuitStateOpen   CircuitState = "OPEN"
)

type CircuitBreakerState struct {
	State  CircuitState
	Reason string
}

func EvaluateCircuitBreakers(metrics PortfolioMetrics, exchangeHealthy bool, liquidityScore float64, strategyFailureRate float64) CircuitBreakerState {
	switch {
	case metrics.DrawdownPct >= 10:
		return CircuitBreakerState{State: CircuitStateOpen, Reason: "rapid drawdown"}
	case !exchangeHealthy:
		return CircuitBreakerState{State: CircuitStateOpen, Reason: "exchange failure"}
	case liquidityScore > 0 && liquidityScore < 0.25:
		return CircuitBreakerState{State: CircuitStateOpen, Reason: "liquidity collapse"}
	case strategyFailureRate >= 0.60:
		return CircuitBreakerState{State: CircuitStateOpen, Reason: "mass strategy failure"}
	case metrics.PortfolioHeatPct >= HeatHardBlockPct:
		return CircuitBreakerState{State: CircuitStateOpen, Reason: "portfolio heat hard block"}
	default:
		return CircuitBreakerState{State: CircuitStateClosed}
	}
}

func (e *PortfolioRiskEngine) SetCircuitBreaker(state CircuitBreakerState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.circuit = state
	if state.State == CircuitStateOpen {
		e.alertLocked(AlertSeverityCritical, "Circuit breaker open", state.Reason)
		e.auditLocked("CIRCUIT_BREAKER", state.Reason, "", "")
	}
}

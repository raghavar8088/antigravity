package risk

import "time"

type RiskAuditEvent struct {
	Type      string
	Message   string
	Strategy  string
	Symbol    string
	CreatedAt time.Time
}

func (e *PortfolioRiskEngine) AuditTrail() []RiskAuditEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]RiskAuditEvent(nil), e.audit...)
}

func (e *PortfolioRiskEngine) auditLocked(eventType, message, strategy, symbol string) {
	e.audit = append(e.audit, RiskAuditEvent{
		Type:      eventType,
		Message:   message,
		Strategy:  strategy,
		Symbol:    symbol,
		CreatedAt: time.Now().UTC(),
	})
	if len(e.audit) > 10000 {
		e.audit = e.audit[len(e.audit)-10000:]
	}
}

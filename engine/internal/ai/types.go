package ai

import "time"

// AgentRole identifies which agent produced a result.
type AgentRole string

const (
	RoleBull  AgentRole = "BULL"
	RoleBear  AgentRole = "BEAR"
	RoleRisk  AgentRole = "RISK"
	RoleMacro AgentRole = "MACRO" // Gemini Flash top-down analyst
)

// AgentSignal is the raw output from Bull or Bear agent.
type AgentSignal struct {
	Role        AgentRole `json:"role"`
	ShouldTrade bool      `json:"shouldTrade"`
	Confidence  float64   `json:"confidence"`
	Thesis      string    `json:"thesis"`
	SizeBTC     float64   `json:"sizeBtc"`
	StopLossPct float64   `json:"stopLossPct"`
	TakeProfitPct float64 `json:"takeProfitPct"`
	Error       string    `json:"error,omitempty"`
}

// RiskVerdict is the Risk Agent's output after reviewing Bull/Bear signals.
type RiskVerdict struct {
	Approved      bool   `json:"approved"`
	ApprovedAction string `json:"approvedAction"` // BUY, SELL, HOLD
	VetoReason    string `json:"vetoReason,omitempty"`
	Reasoning     string `json:"reasoning"`
	AdjustedSize  float64 `json:"adjustedSize"`
	Error         string `json:"error,omitempty"`
}

// AIDecision is the final combined output of all four agents (Bull, Bear, Macro, Risk).
type AIDecision struct {
	ID             string      `json:"id"`
	Timestamp      time.Time   `json:"timestamp"`
	Price          float64     `json:"price"`
	BullSignal     AgentSignal `json:"bullSignal"`
	BearSignal     AgentSignal `json:"bearSignal"`
	MacroSignal    AgentSignal `json:"macroSignal"` // Gemini top-down analyst
	RiskVerdict    RiskVerdict `json:"riskVerdict"`
	FinalAction    string      `json:"finalAction"` // BUY, SELL, HOLD
	FinalReasoning string      `json:"finalReasoning"`
	Executed       bool        `json:"executed"`
	Regime         string      `json:"regime"`
	AuditLogs      []AuditLog  `json:"auditLogs,omitempty"` // Manual strategy signals audited by AI
}

// AuditLog tracks a specific strategy signal that was reviewed by the AI.
type AuditLog struct {
	ID           string    `json:"id"`
	StrategyName string    `json:"strategyName"`
	Action       string    `json:"action"`
	Approved     bool      `json:"approved"`
	Reason       string    `json:"reason"`
	Confidence   float64   `json:"confidence"`
	Timestamp    time.Time `json:"timestamp"`
}

// MarketContext is the data snapshot sent to Claude agents.
type MarketContext struct {
	Symbol        string
	Price         float64
	Regime        string
	RSI           float64
	ATR           float64
	VWAP          float64
	ADX           float64
	EMAFast       float64
	EMASlow       float64
	RecentCandles []CandleSummary
	OpenPositions int
	LongPositions int
	ShortPositions int
	Balance        float64
	DailyPnL       float64
	TotalPnL       float64
	ConsecutiveLosses int
}

// CandleSummary is a compact OHLCV representation for Claude prompts.
type CandleSummary struct {
	Open   float64 `json:"o"`
	High   float64 `json:"h"`
	Low    float64 `json:"l"`
	Close  float64 `json:"c"`
	Volume float64 `json:"v"`
}

// InsightStore holds the last N AI decisions in memory for the API.
type InsightStore struct {
	decisions []AIDecision
	auditLogs []AuditLog
	maxSize   int
}

func NewInsightStore(maxSize int) *InsightStore {
	return &InsightStore{
		decisions: make([]AIDecision, 0, maxSize),
		auditLogs: make([]AuditLog, 0, 100),
		maxSize:   maxSize,
	}
}

func (s *InsightStore) AddAudit(l AuditLog) {
	s.auditLogs = append([]AuditLog{l}, s.auditLogs...)
	if len(s.auditLogs) > 100 {
		s.auditLogs = s.auditLogs[:100]
	}
}

func (s *InsightStore) GetAuditLogs(n int) []AuditLog {
	if n > len(s.auditLogs) {
		n = len(s.auditLogs)
	}
	result := make([]AuditLog, n)
	copy(result, s.auditLogs[:n])
	return result
}

func (s *InsightStore) Add(d AIDecision) {
	s.decisions = append([]AIDecision{d}, s.decisions...)
	if len(s.decisions) > s.maxSize {
		s.decisions = s.decisions[:s.maxSize]
	}
}

func (s *InsightStore) GetRecent(n int) []AIDecision {
	if n > len(s.decisions) {
		n = len(s.decisions)
	}
	result := make([]AIDecision, n)
	copy(result, s.decisions[:n])
	return result
}

func (s *InsightStore) Latest() *AIDecision {
	if len(s.decisions) == 0 {
		return nil
	}
	d := s.decisions[0]
	return &d
}

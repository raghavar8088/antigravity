package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────
// Incident Types
// ─────────────────────────────────────────────

// IncidentSeverity classifies incident urgency.
type IncidentSeverity string

const (
	SeverityCritical IncidentSeverity = "CRITICAL"
	SeverityHigh     IncidentSeverity = "HIGH"
	SeverityMedium   IncidentSeverity = "MEDIUM"
	SeverityLow      IncidentSeverity = "LOW"
)

// IncidentStatus tracks the lifecycle state of an incident.
type IncidentStatus string

const (
	StatusTriggered     IncidentStatus = "TRIGGERED"
	StatusAcknowledged  IncidentStatus = "ACKNOWLEDGED"
	StatusInvestigating IncidentStatus = "INVESTIGATING"
	StatusMitigating    IncidentStatus = "MITIGATING"
	StatusResolved      IncidentStatus = "RESOLVED"
)

// IncidentCategory classifies the domain of the incident.
type IncidentCategory string

const (
	CategoryExecution      IncidentCategory = "EXECUTION"
	CategoryRisk           IncidentCategory = "RISK"
	CategoryMarketData     IncidentCategory = "MARKET_DATA"
	CategoryReconciliation IncidentCategory = "RECONCILIATION"
	CategoryInfrastructure IncidentCategory = "INFRASTRUCTURE"
	CategorySecurity       IncidentCategory = "SECURITY"
	CategoryLedger         IncidentCategory = "LEDGER"
)

// ─────────────────────────────────────────────
// Incident Model
// ─────────────────────────────────────────────

// Incident represents a production incident from trigger to resolution.
type Incident struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    IncidentSeverity  `json:"severity"`
	Category    IncidentCategory  `json:"category"`
	Status      IncidentStatus    `json:"status"`
	TriggeredAt time.Time         `json:"triggered_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	Timeline    []IncidentEvent   `json:"timeline"`
	RootCause   string            `json:"root_cause,omitempty"`
	Postmortem  *Postmortem       `json:"postmortem,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
	Component   string            `json:"component"`
	Tags        map[string]string `json:"tags,omitempty"`
}

// IncidentEvent is a timestamped entry in the incident timeline.
type IncidentEvent struct {
	At     time.Time      `json:"at"`
	Actor  string         `json:"actor"`
	Status IncidentStatus `json:"status"`
	Note   string         `json:"note"`
}

// Postmortem holds the post-incident analysis.
type Postmortem struct {
	WrittenAt      time.Time     `json:"written_at"`
	Author         string        `json:"author"`
	Summary        string        `json:"summary"`
	RootCause      string        `json:"root_cause"`
	ContribFactors []string      `json:"contributing_factors"`
	ActionItems    []ActionItem  `json:"action_items"`
	MTTR           time.Duration `json:"mttr"`
	CapitalImpact  float64       `json:"capital_impact_usd"`
}

// ActionItem is a remediation task arising from a postmortem.
type ActionItem struct {
	Title    string    `json:"title"`
	Owner    string    `json:"owner"`
	DueDate  time.Time `json:"due_date"`
	Priority string    `json:"priority"`
	Done     bool      `json:"done"`
}

// Duration returns the incident duration (triggered→resolved).
func (i *Incident) Duration() time.Duration {
	if i.ResolvedAt == nil {
		return time.Since(i.TriggeredAt)
	}
	return i.ResolvedAt.Sub(i.TriggeredAt)
}

// ─────────────────────────────────────────────
// Incident Manager
// ─────────────────────────────────────────────

// IncidentManager tracks active and historical incidents.
type IncidentManager struct {
	mu         sync.RWMutex
	active     map[string]*Incident
	history    []*Incident
	maxHistory int
	logger     *slog.Logger
	auditPath  string
}

// NewIncidentManager creates a new IncidentManager.
// auditPath is an optional path to append JSON incident records for audit persistence.
func NewIncidentManager(auditPath string) *IncidentManager {
	return &IncidentManager{
		active:     make(map[string]*Incident),
		maxHistory: 1000,
		logger:     Logger,
		auditPath:  auditPath,
	}
}

// Trigger creates and records a new incident.
func (m *IncidentManager) Trigger(ctx context.Context, title, description string, severity IncidentSeverity, category IncidentCategory, component string) *Incident {
	id := fmt.Sprintf("INC-%s-%s", time.Now().UTC().Format("20060102-150405"), newSpanID()[:6])
	now := time.Now().UTC()

	incident := &Incident{
		ID:          id,
		Title:       title,
		Description: description,
		Severity:    severity,
		Category:    category,
		Status:      StatusTriggered,
		TriggeredAt: now,
		UpdatedAt:   now,
		Component:   component,
		TraceID:     TraceIDFromContext(ctx),
		Timeline: []IncidentEvent{{
			At:     now,
			Actor:  "system",
			Status: StatusTriggered,
			Note:   description,
		}},
	}

	m.mu.Lock()
	m.active[id] = incident
	m.mu.Unlock()

	// Metrics
	IncidentTriggered.WithLabelValues(string(severity), string(category)).Inc()
	ActiveIncidents.WithLabelValues(string(severity)).Inc()

	// Structured log
	m.logger.Log(ctx, LevelFatal, "INCIDENT TRIGGERED",
		slog.String("event_type", "INCIDENT_TRIGGERED"),
		slog.String("incident_id", id),
		slog.String("title", title),
		slog.String("severity", string(severity)),
		slog.String("category", string(category)),
		slog.String("component", component),
	)

	m.persist(incident)
	return incident
}

// Acknowledge marks an incident as acknowledged by the named on-call engineer.
func (m *IncidentManager) Acknowledge(ctx context.Context, incidentID, actor, note string) error {
	return m.transition(ctx, incidentID, StatusAcknowledged, actor, note)
}

// Investigate marks an incident as under investigation.
func (m *IncidentManager) Investigate(ctx context.Context, incidentID, actor, note string) error {
	return m.transition(ctx, incidentID, StatusInvestigating, actor, note)
}

// Mitigate marks an incident as being actively mitigated.
func (m *IncidentManager) Mitigate(ctx context.Context, incidentID, actor, note string) error {
	return m.transition(ctx, incidentID, StatusMitigating, actor, note)
}

// Resolve marks an incident as resolved and moves it to history.
func (m *IncidentManager) Resolve(ctx context.Context, incidentID, actor, rootCause string) (*Incident, error) {
	m.mu.Lock()
	incident, ok := m.active[incidentID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("incident %s not found", incidentID)
	}

	now := time.Now().UTC()
	incident.Status = StatusResolved
	incident.UpdatedAt = now
	incident.ResolvedAt = &now
	incident.RootCause = rootCause
	incident.Timeline = append(incident.Timeline, IncidentEvent{
		At:     now,
		Actor:  actor,
		Status: StatusResolved,
		Note:   "Root cause: " + rootCause,
	})

	delete(m.active, incidentID)
	m.history = append(m.history, incident)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.mu.Unlock()

	IncidentResolved.WithLabelValues(string(incident.Severity), string(incident.Category)).Inc()
	ActiveIncidents.WithLabelValues(string(incident.Severity)).Dec()
	IncidentMTTR.WithLabelValues(string(incident.Severity)).Observe(incident.Duration().Seconds())

	m.logger.Log(ctx, LevelInfo, "incident resolved",
		slog.String("event_type", "INCIDENT_RESOLVED"),
		slog.String("incident_id", incidentID),
		slog.String("root_cause", rootCause),
		slog.Float64("duration_minutes", incident.Duration().Minutes()),
	)

	m.persist(incident)
	return incident, nil
}

// AddPostmortem attaches a postmortem to a resolved incident.
func (m *IncidentManager) AddPostmortem(incidentID string, pm Postmortem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, i := range m.history {
		if i.ID == incidentID {
			pm.WrittenAt = time.Now().UTC()
			i.Postmortem = &pm
			return nil
		}
	}
	return fmt.Errorf("resolved incident %s not found in history", incidentID)
}

// ActiveList returns a copy of all currently active incidents.
func (m *IncidentManager) ActiveList() []*Incident {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Incident, 0, len(m.active))
	for _, i := range m.active {
		out = append(out, i)
	}
	return out
}

func (m *IncidentManager) transition(ctx context.Context, incidentID string, status IncidentStatus, actor, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	incident, ok := m.active[incidentID]
	if !ok {
		return fmt.Errorf("incident %s not found", incidentID)
	}
	now := time.Now().UTC()
	incident.Status = status
	incident.UpdatedAt = now
	incident.Timeline = append(incident.Timeline, IncidentEvent{
		At:     now,
		Actor:  actor,
		Status: status,
		Note:   note,
	})
	m.logger.Log(ctx, LevelWarn, "incident status update",
		slog.String("event_type", "INCIDENT_UPDATE"),
		slog.String("incident_id", incidentID),
		slog.String("status", string(status)),
		slog.String("actor", actor),
	)
	return nil
}

func (m *IncidentManager) persist(incident *Incident) {
	if m.auditPath == "" {
		return
	}
	f, err := os.OpenFile(m.auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	_ = enc.Encode(incident)
}

// ─────────────────────────────────────────────
// Incident Prometheus Metrics
// ─────────────────────────────────────────────

var (
	IncidentTriggered = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "incident",
		Name:      "triggered_total",
		Help:      "Total incidents triggered per severity and category.",
	}, []string{"severity", "category"})

	IncidentResolved = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "incident",
		Name:      "resolved_total",
		Help:      "Total incidents resolved per severity and category.",
	}, []string{"severity", "category"})

	ActiveIncidents = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "incident",
		Name:      "active_count",
		Help:      "Current number of active (unresolved) incidents per severity.",
	}, []string{"severity"})

	IncidentMTTR = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "incident",
		Name:      "mttr_seconds",
		Help:      "Mean time to resolution (MTTR) in seconds per severity.",
		Buckets:   []float64{60, 300, 600, 900, 1800, 3600, 7200, 14400, 28800},
	}, []string{"severity"})
)

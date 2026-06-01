package security

import (
	"sync"
	"time"
)

// SecurityProjection is a CQRS read model over security events.
// It aggregates counts and statuses for dashboard consumption.
type SecurityProjection struct {
	mu sync.RWMutex

	// Counters (reset on projection rebuild).
	FailedLogins       int
	PermissionDenials  int
	RateLimitViolations int
	JWTValidationFails int
	ServiceAuthFails   int
	ActiveIncidents    int

	// Active sessions count (updated by session manager).
	ActiveSessions int

	// Most recent events of each type (for dashboards).
	LastUnauthorizedAttempt *SecurityEvent
	LastKillSwitchTrigger   *SecurityEvent
	LastRiskOverride        *SecurityEvent
	LastSecretRotation      *SecurityEvent

	// Hourly buckets for Prometheus / Grafana (last 24 hours).
	HourlyFailedLogins [24]int
	HourlyDenials      [24]int
	currentHour        int
	hourReset          time.Time
}

// NewSecurityProjection creates a fresh projection.
func NewSecurityProjection() *SecurityProjection {
	return &SecurityProjection{
		currentHour: time.Now().Hour(),
		hourReset:   time.Now(),
	}
}

// Apply processes a SecurityEvent and updates the projection.
func (sp *SecurityProjection) Apply(ev SecurityEvent) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.maybeRotateHour()

	switch ev.Type {
	case EventPermissionDenied, EventUnauthorizedAccess:
		sp.FailedLogins++
		sp.HourlyFailedLogins[sp.currentHour]++
		sp.LastUnauthorizedAttempt = &ev

	case EventPermissionGranted:
		// no-op for counters

	case EventRateLimitExceeded:
		sp.RateLimitViolations++
		sp.HourlyDenials[sp.currentHour]++

	case EventKillSwitchTriggered:
		sp.LastKillSwitchTrigger = &ev

	case EventRiskOverride:
		sp.LastRiskOverride = &ev

	case EventSecretRotated:
		sp.LastSecretRotation = &ev

	case EventSecurityIncidentOpened:
		sp.ActiveIncidents++

	case EventSecurityIncidentClosed:
		if sp.ActiveIncidents > 0 {
			sp.ActiveIncidents--
		}
	}
}

// Snapshot returns a point-in-time copy of all projection fields for API serving.
func (sp *SecurityProjection) Snapshot() SecurityProjectionSnapshot {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return SecurityProjectionSnapshot{
		FailedLogins:         sp.FailedLogins,
		PermissionDenials:    sp.PermissionDenials,
		RateLimitViolations:  sp.RateLimitViolations,
		JWTValidationFails:   sp.JWTValidationFails,
		ServiceAuthFails:     sp.ServiceAuthFails,
		ActiveIncidents:      sp.ActiveIncidents,
		ActiveSessions:       sp.ActiveSessions,
		HourlyFailedLogins:   sp.HourlyFailedLogins,
		HourlyDenials:        sp.HourlyDenials,
	}
}

// SecurityProjectionSnapshot is a value-type copy for JSON serialization.
type SecurityProjectionSnapshot struct {
	FailedLogins        int     `json:"failed_logins"`
	PermissionDenials   int     `json:"permission_denials"`
	RateLimitViolations int     `json:"rate_limit_violations"`
	JWTValidationFails  int     `json:"jwt_validation_fails"`
	ServiceAuthFails    int     `json:"service_auth_fails"`
	ActiveIncidents     int     `json:"active_incidents"`
	ActiveSessions      int     `json:"active_sessions"`
	HourlyFailedLogins  [24]int `json:"hourly_failed_logins"`
	HourlyDenials       [24]int `json:"hourly_denials"`
}

func (sp *SecurityProjection) maybeRotateHour() {
	hour := time.Now().Hour()
	if hour != sp.currentHour {
		sp.HourlyFailedLogins[hour] = 0
		sp.HourlyDenials[hour] = 0
		sp.currentHour = hour
	}
}

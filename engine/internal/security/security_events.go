package security

import "time"

// SecurityEventType classifies a security incident or noteworthy event.
type SecurityEventType string

const (
	EventUserLoggedIn            SecurityEventType = "USER_LOGGED_IN"
	EventUserLoggedOut           SecurityEventType = "USER_LOGGED_OUT"
	EventPermissionGranted       SecurityEventType = "PERMISSION_GRANTED"
	EventPermissionDenied        SecurityEventType = "PERMISSION_DENIED"
	EventTradeExecuted           SecurityEventType = "TRADE_EXECUTED"
	EventTradeCancelled          SecurityEventType = "TRADE_CANCELLED"
	EventKillSwitchTriggered     SecurityEventType = "KILL_SWITCH_TRIGGERED"
	EventRiskOverride            SecurityEventType = "RISK_OVERRIDE"
	EventSecretRotated           SecurityEventType = "SECRET_ROTATED"
	EventVaultAccessed           SecurityEventType = "VAULT_ACCESSED"
	EventSystemShutdown          SecurityEventType = "SYSTEM_SHUTDOWN"
	EventUnauthorizedAccess      SecurityEventType = "UNAUTHORIZED_ACCESS_ATTEMPT"
	EventRateLimitExceeded       SecurityEventType = "RATE_LIMIT_EXCEEDED"
	EventSessionCreated          SecurityEventType = "SESSION_CREATED"
	EventSessionRevoked          SecurityEventType = "SESSION_REVOKED"
	EventBruteForceDetected      SecurityEventType = "BRUTE_FORCE_DETECTED"
	EventTokenAbuseDetected      SecurityEventType = "TOKEN_ABUSE_DETECTED"
	EventPrivilegeEscalation     SecurityEventType = "PRIVILEGE_ESCALATION_ATTEMPT"
	EventServiceAuthFailure      SecurityEventType = "SERVICE_AUTH_FAILURE"
	EventSecurityIncidentOpened  SecurityEventType = "SECURITY_INCIDENT_OPENED"
	EventSecurityIncidentClosed  SecurityEventType = "SECURITY_INCIDENT_CLOSED"
)

// SecurityEvent is a rich audit record for security-relevant actions.
type SecurityEvent struct {
	Type        SecurityEventType `json:"type"`
	Timestamp   time.Time         `json:"timestamp"`
	UserID      string            `json:"user_id,omitempty"`
	Email       string            `json:"email,omitempty"`
	Role        string            `json:"role,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	ServiceName string            `json:"service_name,omitempty"`
	IP          string            `json:"ip"`
	Endpoint    string            `json:"endpoint,omitempty"`
	Resource    string            `json:"resource,omitempty"`
	Result      string            `json:"result"`
	Severity    string            `json:"severity"` // INFO | WARN | CRITICAL
	Details     string            `json:"details,omitempty"`
}

const (
	SeverityInfo     = "INFO"
	SeverityWarning  = "WARN"
	SeverityCritical = "CRITICAL"
)

// IncidentType classifies detected security anomalies.
type IncidentType string

const (
	IncidentBruteForce        IncidentType = "BRUTE_FORCE"
	IncidentTokenAbuse        IncidentType = "TOKEN_ABUSE"
	IncidentPrivEscalation    IncidentType = "PRIVILEGE_ESCALATION"
	IncidentExcessiveAPI      IncidentType = "EXCESSIVE_API_USAGE"
	IncidentAuthFailureSpike  IncidentType = "AUTH_FAILURE_SPIKE"
	IncidentUnauthorizedTrade IncidentType = "UNAUTHORIZED_TRADING_ATTEMPT"
	IncidentBadServiceAuth    IncidentType = "BAD_SERVICE_AUTH"
	IncidentBadToken          IncidentType = "BAD_TOKEN"
)

// SecurityIncident is a detected anomaly requiring investigation.
type SecurityIncident struct {
	ID          string
	Type        IncidentType
	IP          string
	Endpoint    string
	FirstSeen   time.Time
	LastSeen    time.Time
	EventCount  int
	Resolved    bool
	Description string
}

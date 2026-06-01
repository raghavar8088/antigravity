package security

import (
	"log"
	"sync"
	"time"
)

// SecurityMonitor tracks anomaly signals and raises incidents.
// Detects: brute force, token abuse, auth failure spikes, privilege escalation.
type SecurityMonitor struct {
	incidents   sync.Map // incidentID → *SecurityIncident
	failCounts  sync.Map // "type:ip" → *failureCounter
	events      []SecurityEvent
	eventsMu    sync.Mutex
	maxEvents   int
	handlers    []func(SecurityIncident)
}

type failureCounter struct {
	count    int
	window   time.Time
	mu       sync.Mutex
}

func (c *failureCounter) increment() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.Sub(c.window) > time.Minute {
		c.count = 0
		c.window = now
	}
	c.count++
	return c.count
}

// NewSecurityMonitor creates a monitor with optional incident handlers.
func NewSecurityMonitor(handlers ...func(SecurityIncident)) *SecurityMonitor {
	sm := &SecurityMonitor{maxEvents: 50_000}
	sm.handlers = handlers
	go sm.gcLoop()
	return sm
}

// OnSecurityEvent registers an event and triggers incident detection logic.
func (sm *SecurityMonitor) OnSecurityEvent(ev SecurityEvent) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	sm.eventsMu.Lock()
	sm.events = append(sm.events, ev)
	if len(sm.events) > sm.maxEvents {
		sm.events = sm.events[1:]
	}
	sm.eventsMu.Unlock()

	if ev.Severity == SeverityCritical {
		log.Printf("[SECURITY CRITICAL] %s user=%s ip=%s endpoint=%s detail=%s",
			ev.Type, ev.UserID, ev.IP, ev.Endpoint, ev.Details)
	}
}

// recordFailure increments the failure counter for a given incident type and IP.
// Raises an incident when thresholds are exceeded.
func (sm *SecurityMonitor) recordFailure(incidentType IncidentType, ip, endpoint string) {
	key := string(incidentType) + ":" + ip
	v, _ := sm.failCounts.LoadOrStore(key, &failureCounter{window: time.Now()})
	count := v.(*failureCounter).increment()

	threshold := sm.thresholdFor(incidentType)
	if count >= threshold {
		sm.raiseIncident(incidentType, ip, endpoint, count)
	}
}

func (sm *SecurityMonitor) thresholdFor(t IncidentType) int {
	switch t {
	case IncidentBruteForce, IncidentAuthFailureSpike:
		return 5
	case IncidentTokenAbuse:
		return 10
	case IncidentExcessiveAPI:
		return 500
	default:
		return 3
	}
}

// raiseIncident creates or updates a SecurityIncident.
func (sm *SecurityMonitor) raiseIncident(itype IncidentType, ip, endpoint string, count int) {
	key := string(itype) + ":" + ip
	now := time.Now()
	v, loaded := sm.incidents.LoadOrStore(key, &SecurityIncident{
		ID:        generateID(),
		Type:      itype,
		IP:        ip,
		Endpoint:  endpoint,
		FirstSeen: now,
		LastSeen:  now,
		EventCount: count,
	})
	incident := v.(*SecurityIncident)
	if loaded {
		incident.LastSeen = now
		incident.EventCount = count
	}

	sm.OnSecurityEvent(SecurityEvent{
		Type:      EventSecurityIncidentOpened,
		IP:        ip,
		Endpoint:  endpoint,
		Severity:  SeverityCritical,
		Details:   string(itype) + " detected",
		Result:    "INCIDENT",
		Timestamp: now,
	})

	log.Printf("[SECURITY] INCIDENT %s ip=%s endpoint=%s count=%d", itype, ip, endpoint, count)

	for _, h := range sm.handlers {
		go h(*incident)
	}
}

// RecentEvents returns the last n security events.
func (sm *SecurityMonitor) RecentEvents(n int) []SecurityEvent {
	sm.eventsMu.Lock()
	defer sm.eventsMu.Unlock()
	if n <= 0 || n > len(sm.events) {
		n = len(sm.events)
	}
	start := len(sm.events) - n
	cp := make([]SecurityEvent, n)
	copy(cp, sm.events[start:])
	return cp
}

// OpenIncidents returns all unresolved incidents.
func (sm *SecurityMonitor) OpenIncidents() []SecurityIncident {
	var result []SecurityIncident
	sm.incidents.Range(func(_, v interface{}) bool {
		inc := v.(*SecurityIncident)
		if !inc.Resolved {
			result = append(result, *inc)
		}
		return true
	})
	return result
}

// ResolveIncident marks an incident resolved.
func (sm *SecurityMonitor) ResolveIncident(id string) bool {
	var found bool
	sm.incidents.Range(func(k, v interface{}) bool {
		inc := v.(*SecurityIncident)
		if inc.ID == id {
			inc.Resolved = true
			found = true
			sm.OnSecurityEvent(SecurityEvent{
				Type:      EventSecurityIncidentClosed,
				Severity:  SeverityInfo,
				Details:   "incident resolved: " + id,
				Result:    "RESOLVED",
				Timestamp: time.Now().UTC(),
			})
			return false
		}
		return true
	})
	return found
}

// gcLoop clears old resolved incidents every hour.
func (sm *SecurityMonitor) gcLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		sm.incidents.Range(func(k, v interface{}) bool {
			inc := v.(*SecurityIncident)
			if inc.Resolved && time.Since(inc.LastSeen) > 24*time.Hour {
				sm.incidents.Delete(k)
			}
			return true
		})
	}
}

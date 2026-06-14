package kelly

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var kellySessionMult = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "trading",
	Name:      "kelly_session_mult",
	Help:      "Kelly position-size multiplier applied for the current trading session.",
}, []string{"session"})

// Session represents a named trading session.
type Session string

const (
	SessionAsia    Session = "ASIA"
	SessionLondon  Session = "LONDON"
	SessionNewYork Session = "NEW_YORK"
	SessionOff     Session = "OFF"
)

// DetectSession classifies a UTC time into a named trading session.
func DetectSession(t time.Time) Session {
	hour := t.UTC().Hour()
	switch {
	case hour >= 0 && hour < 7:
		return SessionAsia
	case hour >= 7 && hour < 12:
		return SessionLondon
	case hour >= 13 && hour < 21:
		return SessionNewYork
	default:
		return SessionOff
	}
}

// SessionKellyMultiplier returns the Kelly position-size multiplier for a session.
func SessionKellyMultiplier(s Session) float64 {
	var mult float64
	switch s {
	case SessionLondon:
		mult = 1.00
	case SessionNewYork:
		mult = 1.00
	case SessionAsia:
		mult = 0.50
	case SessionOff:
		mult = 0.25
	default:
		mult = 0.50
	}
	kellySessionMult.WithLabelValues(string(s)).Set(mult)
	return mult
}

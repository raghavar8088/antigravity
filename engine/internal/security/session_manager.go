package security

import (
	"context"
	"sync"
	"time"
)

// Session tracks an active authenticated session.
type Session struct {
	SessionID    string
	UserID       string
	Email        string
	Role         Role
	IP           string
	UserAgent    string
	CreatedAt    time.Time
	LastSeenAt   time.Time
	ExpiresAt    time.Time
	Revoked      bool
	RevokeReason string
}

// IsValid returns true if the session is not expired and not revoked.
func (s *Session) IsValid() bool {
	return !s.Revoked && time.Now().Before(s.ExpiresAt)
}

// SessionManager maintains the active session registry.
// Used for real-time revocation and concurrent session enforcement.
type SessionManager struct {
	sessions sync.Map // sessionID → *Session
	maxPerUser int    // max concurrent sessions per user
	mu         sync.Mutex
}

// NewSessionManager creates a session manager.
// maxConcurrent = 0 means unlimited.
func NewSessionManager(maxConcurrent int) *SessionManager {
	sm := &SessionManager{maxPerUser: maxConcurrent}
	go sm.gcLoop()
	return sm
}

// Register adds a new session. If maxPerUser is exceeded, the oldest session is revoked.
func (sm *SessionManager) Register(sess Session) {
	if sm.maxPerUser > 0 {
		sm.enforceLimit(sess.UserID, sess.SessionID)
	}
	sm.sessions.Store(sess.SessionID, &sess)
}

// Touch updates the LastSeenAt timestamp for a session.
func (sm *SessionManager) Touch(sessionID string) {
	if v, ok := sm.sessions.Load(sessionID); ok {
		v.(*Session).LastSeenAt = time.Now()
	}
}

// Revoke marks a session as revoked. All future requests bearing this session ID
// will be rejected at the authentication middleware.
func (sm *SessionManager) Revoke(ctx context.Context, sessionID, reason string) bool {
	v, ok := sm.sessions.Load(sessionID)
	if !ok {
		return false
	}
	s := v.(*Session)
	s.Revoked = true
	s.RevokeReason = reason
	return true
}

// RevokeAll revokes all active sessions for a user (forced logout).
func (sm *SessionManager) RevokeAll(ctx context.Context, userID, reason string) int {
	count := 0
	sm.sessions.Range(func(k, v interface{}) bool {
		s := v.(*Session)
		if s.UserID == userID && !s.Revoked {
			s.Revoked = true
			s.RevokeReason = reason
			count++
		}
		return true
	})
	return count
}

// IsRevoked returns true if the session has been revoked.
func (sm *SessionManager) IsRevoked(sessionID string) bool {
	v, ok := sm.sessions.Load(sessionID)
	if !ok {
		return false
	}
	return v.(*Session).Revoked
}

// ActiveCount returns the number of non-revoked, non-expired sessions.
func (sm *SessionManager) ActiveCount() int {
	count := 0
	sm.sessions.Range(func(_, v interface{}) bool {
		if v.(*Session).IsValid() {
			count++
		}
		return true
	})
	return count
}

// ActiveForUser returns active sessions for a specific user.
func (sm *SessionManager) ActiveForUser(userID string) []*Session {
	var result []*Session
	sm.sessions.Range(func(_, v interface{}) bool {
		s := v.(*Session)
		if s.UserID == userID && s.IsValid() {
			result = append(result, s)
		}
		return true
	})
	return result
}

// enforceLimit removes the oldest session for a user if the limit is exceeded.
func (sm *SessionManager) enforceLimit(userID, newSessionID string) {
	active := sm.ActiveForUser(userID)
	if len(active) < sm.maxPerUser {
		return
	}
	// Revoke the oldest session.
	oldest := active[0]
	for _, s := range active[1:] {
		if s.CreatedAt.Before(oldest.CreatedAt) {
			oldest = s
		}
	}
	if oldest.SessionID != newSessionID {
		oldest.Revoked = true
		oldest.RevokeReason = "concurrent session limit exceeded"
	}
}

// gcLoop periodically removes expired sessions from memory.
func (sm *SessionManager) gcLoop() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		sm.sessions.Range(func(k, v interface{}) bool {
			s := v.(*Session)
			if !s.IsValid() && time.Since(s.ExpiresAt) > time.Hour {
				sm.sessions.Delete(k)
			}
			return true
		})
	}
}

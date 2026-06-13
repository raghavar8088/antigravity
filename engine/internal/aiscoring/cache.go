package aiscoring

import (
	"sync"
	"time"
)

// ScoreCache is a thread-safe TTL cache for SignalScore values.
type ScoreCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	score     SignalScore
	expiresAt time.Time
}

// NewScoreCache creates a ScoreCache with the given TTL per entry.
func NewScoreCache(ttl time.Duration) *ScoreCache {
	return &ScoreCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

// Set stores a score under key.
func (c *ScoreCache) Set(key string, score SignalScore) {
	c.mu.Lock()
	c.entries[key] = cacheEntry{
		score:     score,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()
}

// Get retrieves a non-expired score. Returns (nil, false) if missing or stale.
func (c *ScoreCache) Get(key string) (*SignalScore, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	s := e.score
	return &s, true
}

// Cleanup removes all expired entries. Call from a periodic ticker.
func (c *ScoreCache) Cleanup() {
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.entries {
		if now.After(e.expiresAt) {
			delete(c.entries, k)
		}
	}
	c.mu.Unlock()
}

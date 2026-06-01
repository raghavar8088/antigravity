package vault

import (
	"sync"
	"time"
)

// cachedSecret stores a secret value with an expiry timestamp.
type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// SecretCache is a thread-safe in-memory cache for secret values.
// Secrets are never written to disk and are evicted after TTL.
type SecretCache struct {
	entries sync.Map
	ttl     time.Duration
}

// NewSecretCache creates a cache with the given TTL.
func NewSecretCache(ttl time.Duration) *SecretCache {
	c := &SecretCache{ttl: ttl}
	go c.gcLoop()
	return c
}

// Get returns the cached value if present and not expired.
func (c *SecretCache) Get(key string) (string, bool) {
	v, ok := c.entries.Load(key)
	if !ok {
		return "", false
	}
	entry := v.(cachedSecret)
	if time.Now().After(entry.expiresAt) {
		c.entries.Delete(key)
		return "", false
	}
	return entry.value, true
}

// Set stores a value in the cache.
func (c *SecretCache) Set(key, value string) {
	c.entries.Store(key, cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	})
}

// Invalidate removes a specific key from the cache, forcing re-fetch on next Get.
// Called during secret rotation to ensure the new value is loaded immediately.
func (c *SecretCache) Invalidate(key string) {
	c.entries.Delete(key)
}

// InvalidateAll clears the entire cache. Used during emergency rotation.
func (c *SecretCache) InvalidateAll() {
	c.entries.Range(func(k, _ interface{}) bool {
		c.entries.Delete(k)
		return true
	})
}

// Size returns the number of non-expired entries in the cache.
func (c *SecretCache) Size() int {
	count := 0
	now := time.Now()
	c.entries.Range(func(_, v interface{}) bool {
		if now.Before(v.(cachedSecret).expiresAt) {
			count++
		}
		return true
	})
	return count
}

// gcLoop periodically removes expired entries.
func (c *SecretCache) gcLoop() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.entries.Range(func(k, v interface{}) bool {
			if now.After(v.(cachedSecret).expiresAt) {
				c.entries.Delete(k)
			}
			return true
		})
	}
}

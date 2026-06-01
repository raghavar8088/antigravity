package security

import (
	"net/http"
	"sync"
	"time"
)

// RateLimitPolicy defines limits for a class of requests.
type RateLimitPolicy struct {
	RequestsPerSecond float64
	BurstSize         int
}

// DefaultPolicies provides sensible defaults for each route class.
var DefaultPolicies = map[string]RateLimitPolicy{
	"global":     {RequestsPerSecond: 200, BurstSize: 50},
	"api":        {RequestsPerSecond: 100, BurstSize: 30},
	"admin":      {RequestsPerSecond: 5, BurstSize: 3},
	"trade":      {RequestsPerSecond: 20, BurstSize: 10},
	"auth":       {RequestsPerSecond: 5, BurstSize: 3},
	"health":     {RequestsPerSecond: 60, BurstSize: 20},
}

// tokenBucket is a per-key token bucket state.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per nanosecond
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rps float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rps / float64(time.Second),
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	b.tokens += float64(elapsed) * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimiter manages per-IP token buckets across all route classes.
type RateLimiter struct {
	buckets  sync.Map // key: "policy:ip" → *tokenBucket
	policies map[string]RateLimitPolicy
}

// NewRateLimiter creates a RateLimiter with the given policies.
func NewRateLimiter(policies map[string]RateLimitPolicy) *RateLimiter {
	if policies == nil {
		policies = DefaultPolicies
	}
	return &RateLimiter{policies: policies}
}

// Allow returns true if the request from ip is within policy limits.
func (rl *RateLimiter) Allow(policyName, ip string) bool {
	policy, ok := rl.policies[policyName]
	if !ok {
		policy = rl.policies["global"]
	}
	key := policyName + ":" + ip
	v, _ := rl.buckets.LoadOrStore(key, newTokenBucket(policy.RequestsPerSecond, policy.BurstSize))
	return v.(*tokenBucket).Allow()
}

// PolicyFor maps a request path to a rate limit policy name.
func PolicyFor(path string) string {
	if IsAdminPath(path) {
		return "admin"
	}
	if hasPrefix(path, "/api/delta-live/order") {
		return "trade"
	}
	if hasPrefix(path, "/api/auth") {
		return "auth"
	}
	if path == "/health" || path == "/api/health" {
		return "health"
	}
	if hasPrefix(path, "/api/") {
		return "api"
	}
	return "global"
}

// RateLimitExceeded returns a 429 response with a Retry-After header.
func RateLimitExceeded(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":"rate limit exceeded","code":429}`)) //nolint:errcheck
}

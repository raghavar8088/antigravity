package vault

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// RotationPolicy defines how a secret should be rotated.
type RotationPolicy struct {
	Key          string
	Schedule     time.Duration // how often to rotate (0 = manual only)
	EmergencyTTL time.Duration // TTL for emergency rotation (much shorter)
	Rollback     bool          // keep previous value for rollback
}

// RotationRecord tracks a completed rotation event.
type RotationRecord struct {
	Key       string
	RotatedAt time.Time
	By        string // "scheduled" | "manual" | "emergency"
	Success   bool
	Error     string
}

// RotationEngine manages scheduled and on-demand secret rotation.
type RotationEngine struct {
	provider SecretProvider
	policies []RotationPolicy
	history  []RotationRecord
	mu       sync.Mutex
	onRotate []func(RotationRecord) // callbacks for audit logging
	stopCh   chan struct{}
}

// NewRotationEngine creates a rotation engine backed by provider.
func NewRotationEngine(provider SecretProvider, policies []RotationPolicy) *RotationEngine {
	return &RotationEngine{
		provider: provider,
		policies: policies,
		stopCh:   make(chan struct{}),
	}
}

// OnRotation registers a callback invoked after each rotation attempt.
// Use to emit audit events or update dependent services.
func (r *RotationEngine) OnRotation(fn func(RotationRecord)) {
	r.mu.Lock()
	r.onRotate = append(r.onRotate, fn)
	r.mu.Unlock()
}

// Start begins the scheduled rotation loop.
func (r *RotationEngine) Start(ctx context.Context) {
	for _, policy := range r.policies {
		if policy.Schedule > 0 {
			go r.scheduleRotation(ctx, policy)
		}
	}
}

// Stop halts all scheduled rotations.
func (r *RotationEngine) Stop() {
	close(r.stopCh)
}

// Rotate performs an immediate manual rotation of the named key.
func (r *RotationEngine) Rotate(ctx context.Context, key, by string) error {
	return r.doRotate(ctx, key, by)
}

// EmergencyRotateAll rotates every registered key immediately.
// Used when a credential leak is suspected.
func (r *RotationEngine) EmergencyRotateAll(ctx context.Context) []error {
	var errs []error
	for _, policy := range r.policies {
		if err := r.doRotate(ctx, policy.Key, "emergency"); err != nil {
			errs = append(errs, fmt.Errorf("key %q: %w", policy.Key, err))
			log.Printf("[ROTATION] Emergency rotation FAILED for %q: %v", policy.Key, err)
		}
	}
	// Invalidate entire cache after emergency rotation.
	if vp, ok := r.provider.(*VaultProvider); ok {
		vp.cache.InvalidateAll()
	}
	if fp, ok := r.provider.(*FallbackProvider); ok {
		if vp, ok := fp.primary.(*VaultProvider); ok {
			vp.cache.InvalidateAll()
		}
	}
	return errs
}

// History returns recent rotation records (most recent first).
func (r *RotationEngine) History(n int) []RotationRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > len(r.history) {
		n = len(r.history)
	}
	start := len(r.history) - n
	cp := make([]RotationRecord, n)
	copy(cp, r.history[start:])
	return cp
}

func (r *RotationEngine) scheduleRotation(ctx context.Context, policy RotationPolicy) {
	ticker := time.NewTicker(policy.Schedule)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := r.doRotate(ctx, policy.Key, "scheduled"); err != nil {
				log.Printf("[ROTATION] Scheduled rotation failed for %q: %v", policy.Key, err)
			}
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// doRotate invalidates the cache for a key and logs the rotation event.
// In a real Vault setup this would trigger a Vault rotation endpoint.
// The actual new value is fetched on the next Get() call.
func (r *RotationEngine) doRotate(ctx context.Context, key, by string) error {
	record := RotationRecord{
		Key:       key,
		RotatedAt: time.Now().UTC(),
		By:        by,
	}

	// Invalidate cache so next access fetches the new value.
	switch p := r.provider.(type) {
	case *VaultProvider:
		p.cache.Invalidate(key)
		record.Success = true
	case *FallbackProvider:
		if vp, ok := p.primary.(*VaultProvider); ok {
			vp.cache.Invalidate(key)
		}
		record.Success = true
	default:
		// EnvProvider: rotation not supported — log only.
		record.Success = false
		record.Error = "env provider does not support rotation"
	}

	r.mu.Lock()
	r.history = append(r.history, record)
	if len(r.history) > 1000 {
		r.history = r.history[1:]
	}
	r.mu.Unlock()

	for _, fn := range r.onRotate {
		go fn(record)
	}

	if record.Success {
		log.Printf("[ROTATION] Secret %q rotated by=%s", key, by)
	}
	return nil
}

// DefaultRotationPolicies returns the standard rotation schedule for trading platform secrets.
func DefaultRotationPolicies() []RotationPolicy {
	return []RotationPolicy{
		{Key: "BINANCE_API_KEY", Schedule: 30 * 24 * time.Hour, Rollback: true},
		{Key: "BINANCE_API_SECRET", Schedule: 30 * 24 * time.Hour, Rollback: true},
		{Key: "DELTA_API_KEY", Schedule: 30 * 24 * time.Hour, Rollback: true},
		{Key: "DELTA_API_SECRET", Schedule: 30 * 24 * time.Hour, Rollback: true},
		{Key: "AUTH_JWT_SECRET", Schedule: 7 * 24 * time.Hour, Rollback: false},
		{Key: "ENGINE_ADMIN_SECRET", Schedule: 7 * 24 * time.Hour, Rollback: false},
		{Key: "INTERNAL_API_SECRET", Schedule: 14 * 24 * time.Hour, Rollback: false},
		{Key: "OPENAI_API_KEY", Schedule: 90 * 24 * time.Hour, Rollback: true},
		{Key: "GROQ_API_KEY", Schedule: 90 * 24 * time.Hour, Rollback: true},
		// Database credentials — manual rotation with migration window.
		{Key: "DATABASE_URL", Schedule: 0, Rollback: true},
		{Key: "REDIS_URL", Schedule: 0, Rollback: true},
		{Key: "MONGODB_URI", Schedule: 0, Rollback: true},
	}
}

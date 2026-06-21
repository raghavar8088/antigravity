// Package config provides a central runtime-editable registry for every
// tunable trading threshold in the engine. It replaces scattered
// os.Getenv() reads and hardcoded constants with a single source of truth
// that can be inspected and mutated without a redeploy.
//
// Design notes:
//   - On startup, Init() populates the registry from environment variables
//     using exactly the same parse-with-default pattern already used across
//     the codebase (see getInitialPaperBalanceUSD in cmd/antigravity/main.go
//     and parseFloatEnv in internal/trading/loop.go).
//   - Existing package-level vars (internal/trading, internal/risk, etc.)
//     continue to exist — they are seeded from the registry at startup and,
//     for hot-reloadable keys, re-read from the registry on Get() calls
//     added at their use sites. This keeps the change low-risk: the kill
//     switch and order execution path are untouched, and every existing
//     call site keeps working even if a particular constant has not yet
//     been wired through Get().
//   - Set() never silently clamps. Out-of-range values are rejected.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

// ThresholdValue is the live state of one registry entry.
type ThresholdValue struct {
	Key          string    `json:"key"`
	CurrentValue float64   `json:"currentValue"`
	DefaultValue float64   `json:"defaultValue"`
	Source       string    `json:"source"` // "env" | "default" | "runtime_override"
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Meta carries the static, non-numeric metadata for a threshold that the
// registry does not itself know how to derive (bounds, descriptions, risk
// level). Populated by RegisterMeta at startup from the catalog in
// engine/internal/config/catalog.go.
type Meta struct {
	Key             string
	Category        string
	Label           string
	Description     string
	DataType        string // percentage | decimal | integer | currency | duration
	Unit            string
	Min             float64
	Max             float64
	RecommendedMin  float64
	RecommendedMax  float64
	RiskLevel       string // low | medium | high | critical
	WhatIfIncreased string
	WhatIfDecreased string
	RequiresRestart bool
	EnvVar          string
}

// ThresholdRegistry is the central, thread-safe store for every tunable
// trading threshold. Use the package-level Default() instance unless a
// test needs an isolated registry.
type ThresholdRegistry struct {
	mu     sync.RWMutex
	values map[string]ThresholdValue
	meta   map[string]Meta
}

var (
	defaultRegistry     *ThresholdRegistry
	defaultRegistryOnce sync.Once
)

// Default returns the process-wide singleton registry, creating and
// populating it from environment variables on first call.
func Default() *ThresholdRegistry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = NewThresholdRegistry()
		defaultRegistry.InitFromEnv()
		defaultRegistry.RegisterMeta(Catalog())
	})
	return defaultRegistry
}

// NewThresholdRegistry creates an empty registry. Most callers should use
// Default() instead; this constructor exists for tests.
func NewThresholdRegistry() *ThresholdRegistry {
	return &ThresholdRegistry{
		values: make(map[string]ThresholdValue),
		meta:   make(map[string]Meta),
	}
}

// parseFloatEnvCfg mirrors getInitialPaperBalanceUSD / parseFloatEnv used
// throughout cmd/antigravity/main.go and internal/trading/loop.go: read the
// env var, fall back to def on missing/invalid input, and never panic.
func parseFloatEnvCfg(key string, def float64) (float64, string) {
	v := os.Getenv(key)
	if v == "" {
		return def, "default"
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("[CONFIG] WARNING: invalid %s=%q, using default %v", key, v, def)
		return def, "default"
	}
	return f, "env"
}

// InitFromEnv populates the registry from the Catalog's declared env vars
// and defaults. Safe to call multiple times (idempotent re-read of env).
func (r *ThresholdRegistry) InitFromEnv() {
	entries := Catalog()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range entries {
		val, source := parseFloatEnvCfg(m.EnvVar, defaultValueOf(m))
		r.values[m.Key] = ThresholdValue{
			Key:          m.Key,
			CurrentValue: val,
			DefaultValue: defaultValueOf(m),
			Source:       source,
			UpdatedAt:    time.Now().UTC(),
		}
	}
}

// defaultValueOf picks the recommended-range midpoint-free "shipped" default.
// The catalog stores the default directly via RecommendedMin==RecommendedMax
// convention is NOT used; instead Catalog() entries set Min/Max for
// validation and the literal default lives in the entry's DefaultValue field
// carried via metaDefaults. See catalog.go for the source of truth.
func defaultValueOf(m Meta) float64 {
	if d, ok := metaDefaults[m.Key]; ok {
		return d
	}
	return 0
}

// RegisterMeta loads static metadata (labels, descriptions, bounds, risk
// levels) into the registry. Called once at startup after InitFromEnv.
func (r *ThresholdRegistry) RegisterMeta(entries []Meta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range entries {
		r.meta[m.Key] = m
	}
}

// Get returns the live float64 value for key, or 0 if unknown.
func (r *ThresholdRegistry) Get(key string) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.values[key]; ok {
		return v.CurrentValue
	}
	return 0
}

// GetWithDefault returns the live value for key, or def if the key is not
// registered at all (defensive fallback for call sites added before the
// catalog entry existed).
func (r *ThresholdRegistry) GetWithDefault(key string, def float64) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if v, ok := r.values[key]; ok {
		return v.CurrentValue
	}
	return def
}

// GetMeta returns the static metadata for key.
func (r *ThresholdRegistry) GetMeta(key string) (Meta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.meta[key]
	return m, ok
}

// ErrUnknownKey is returned by Set when the key has no catalog entry.
type ErrUnknownKey struct{ Key string }

func (e ErrUnknownKey) Error() string { return fmt.Sprintf("config: unknown threshold key %q", e.Key) }

// ErrOutOfRange is returned by Set when value falls outside [min,max].
// Set NEVER silently clamps — the caller must supply a value Within bounds.
type ErrOutOfRange struct {
	Key        string
	Value      float64
	Min, Max   float64
}

func (e ErrOutOfRange) Error() string {
	return fmt.Sprintf("config: value %v for %q is outside allowed range [%v, %v]", e.Value, e.Key, e.Min, e.Max)
}

// Set validates value against the catalog's [min,max] bounds and, if valid,
// applies it immediately and records the change. It never clamps — an
// out-of-range value is rejected with ErrOutOfRange so the caller (the HTTP
// handler) can surface a clear error to the operator.
//
// For keys marked RequiresRestart in the catalog, the new value is still
// written (so it is visible and will apply after a restart) but callers
// should surface requiresRestart=true to the operator; nothing in the
// running process re-reads a restart-tier value implicitly.
func (r *ThresholdRegistry) Set(key string, value float64, source string) (ThresholdValue, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.meta[key]
	if !ok {
		return ThresholdValue{}, ErrUnknownKey{Key: key}
	}
	if value < m.Min || value > m.Max {
		return ThresholdValue{}, ErrOutOfRange{Key: key, Value: value, Min: m.Min, Max: m.Max}
	}

	updated := ThresholdValue{
		Key:          key,
		CurrentValue: value,
		DefaultValue: defaultValueOf(m),
		Source:       source,
		UpdatedAt:    time.Now().UTC(),
	}
	r.values[key] = updated

	if m.RequiresRestart {
		log.Printf("[CONFIG] %s set to %v (source=%s) — REQUIRES RESTART to take effect", key, value, source)
	} else {
		log.Printf("[CONFIG] %s set to %v (source=%s) — effective immediately", key, value, source)
	}
	return updated, nil
}

// All returns a snapshot copy of every registered threshold value.
func (r *ThresholdRegistry) All() map[string]ThresholdValue {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]ThresholdValue, len(r.values))
	for k, v := range r.values {
		out[k] = v
	}
	return out
}

// AllMeta returns a snapshot copy of every registered threshold's metadata.
func (r *ThresholdRegistry) AllMeta() map[string]Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]Meta, len(r.meta))
	for k, v := range r.meta {
		out[k] = v
	}
	return out
}

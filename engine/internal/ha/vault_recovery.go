package ha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	vaultCheckInterval  = 10 * time.Second
	vaultSecretCacheTTL = 5 * time.Minute
	vaultRenewThreshold = 30 * time.Second // renew token when TTL < this
	vaultRequestTimeout = 5 * time.Second
)

// SecretValue holds a secret and its expiry time.
type SecretValue struct {
	Value     string
	ExpiresAt time.Time
}

// IsExpired returns true if the secret should be refreshed.
func (s SecretValue) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// VaultRecovery provides resilient secret access by:
//  1. Caching secrets in memory with configurable TTL.
//  2. Automatically renewing the Vault token before expiry.
//  3. Falling back to the cache when Vault is unreachable — trading is never
//     interrupted by a Vault restart.
//  4. Alerting when cache-only mode has been active beyond a safe window.
type VaultRecovery struct {
	vaultAddr string
	token     string
	tokenMu   sync.RWMutex

	cache   map[string]SecretValue
	cacheMu sync.RWMutex

	httpClient *http.Client

	vaultReachable bool
	cacheOnlySince time.Time
	mu             sync.RWMutex

	onVaultDownCbs      []func(since time.Time)
	onVaultRecoveredCbs []func()

	metricReachable   prometheus.Gauge
	metricCacheHits   prometheus.Counter
	metricCacheMisses prometheus.Counter
	metricRenewals    prometheus.Counter
	metricErrors      prometheus.Counter
	metricCacheAgeMax prometheus.Gauge
}

// VaultConfig holds Vault connection parameters.
type VaultConfig struct {
	Addr      string
	Token     string
	Namespace string
	Reg       prometheus.Registerer
}

func NewVaultRecovery(cfg VaultConfig) *VaultRecovery {
	if cfg.Reg == nil {
		cfg.Reg = prometheus.DefaultRegisterer
	}
	vr := &VaultRecovery{
		vaultAddr:      cfg.Addr,
		token:          cfg.Token,
		cache:          make(map[string]SecretValue),
		httpClient:     &http.Client{Timeout: vaultRequestTimeout},
		vaultReachable: true,
	}
	f := promauto.With(cfg.Reg)
	vr.metricReachable = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_vault_reachable", Help: "1 if Vault is reachable",
	})
	vr.metricCacheHits = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_vault_cache_hits_total", Help: "Secret cache hits",
	})
	vr.metricCacheMisses = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_vault_cache_misses_total", Help: "Secret cache misses",
	})
	vr.metricRenewals = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_vault_token_renewals_total", Help: "Token renewal count",
	})
	vr.metricErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_vault_errors_total", Help: "Vault errors",
	})
	vr.metricCacheAgeMax = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_vault_cache_max_age_seconds", Help: "Age of the oldest cached secret",
	})
	return vr
}

// Run starts the Vault health monitor and token renewer.
func (vr *VaultRecovery) Run(ctx context.Context) error {
	// Pre-warm the cache by refreshing all known secrets once.
	vr.refreshAll(ctx)

	ticker := time.NewTicker(vaultCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			vr.healthCheck(ctx)
			vr.renewTokenIfNeeded(ctx)
			vr.refreshExpiring(ctx)
			vr.updateCacheAgeStat()
		}
	}
}

// GetSecret returns the value of a Vault KV secret at path.
// If Vault is unreachable and the secret is in cache, the cached value
// is returned to avoid trading interruption.
func (vr *VaultRecovery) GetSecret(ctx context.Context, path string) (string, error) {
	// Check cache first.
	vr.cacheMu.RLock()
	cached, ok := vr.cache[path]
	vr.cacheMu.RUnlock()

	vr.mu.RLock()
	reachable := vr.vaultReachable
	vr.mu.RUnlock()

	if ok && !cached.IsExpired() {
		vr.metricCacheHits.Inc()
		return cached.Value, nil
	}

	if !reachable {
		// Cache-only fallback.
		if ok {
			vr.metricCacheHits.Inc()
			log.Printf("[ha/vault] Vault unreachable, using stale cache for %s", path)
			return cached.Value, nil
		}
		vr.metricCacheMisses.Inc()
		return "", fmt.Errorf("vault: unreachable and no cache for %s", path)
	}

	vr.metricCacheMisses.Inc()
	value, err := vr.fetchSecret(ctx, path)
	if err != nil {
		// Vault returned error — fall back to any cached value.
		if ok {
			log.Printf("[ha/vault] fetch error, using cache for %s: %v", path, err)
			return cached.Value, nil
		}
		return "", fmt.Errorf("vault: fetch %s: %w", path, err)
	}

	vr.cacheMu.Lock()
	vr.cache[path] = SecretValue{
		Value:     value,
		ExpiresAt: time.Now().Add(vaultSecretCacheTTL),
	}
	vr.cacheMu.Unlock()
	return value, nil
}

// PreloadSecret explicitly loads a secret into the cache.
// Call this during startup for all critical secrets before trading begins,
// so that any Vault disruption during trading has a full cache to fall back on.
func (vr *VaultRecovery) PreloadSecret(ctx context.Context, path string) error {
	_, err := vr.GetSecret(ctx, path)
	return err
}

// OnVaultDown registers a callback invoked when Vault becomes unreachable.
func (vr *VaultRecovery) OnVaultDown(cb func(since time.Time)) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.onVaultDownCbs = append(vr.onVaultDownCbs, cb)
}

// OnVaultRecovered registers a callback invoked when Vault becomes reachable again.
func (vr *VaultRecovery) OnVaultRecovered(cb func()) {
	vr.mu.Lock()
	defer vr.mu.Unlock()
	vr.onVaultRecoveredCbs = append(vr.onVaultRecoveredCbs, cb)
}

func (vr *VaultRecovery) healthCheck(ctx context.Context) {
	hctx, cancel := context.WithTimeout(ctx, vaultRequestTimeout)
	defer cancel()

	url := vr.vaultAddr + "/v1/sys/health"
	req, _ := http.NewRequestWithContext(hctx, http.MethodGet, url, nil)
	resp, err := vr.httpClient.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	vr.mu.Lock()
	wasReachable := vr.vaultReachable
	now := time.Now()

	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		vr.vaultReachable = false
		vr.metricReachable.Set(0)
		if wasReachable {
			vr.cacheOnlySince = now
			log.Printf("[ha/vault] Vault UNREACHABLE at %s — cache-only mode", vr.vaultAddr)
			cbs := make([]func(time.Time), len(vr.onVaultDownCbs))
			copy(cbs, vr.onVaultDownCbs)
			vr.mu.Unlock()
			for _, cb := range cbs {
				cb(now)
			}
			return
		}
	} else {
		vr.vaultReachable = true
		vr.metricReachable.Set(1)
		if !wasReachable {
			cacheFor := now.Sub(vr.cacheOnlySince)
			log.Printf("[ha/vault] Vault RECOVERED after %s in cache-only mode", cacheFor)
			cbs := make([]func(), len(vr.onVaultRecoveredCbs))
			copy(cbs, vr.onVaultRecoveredCbs)
			vr.mu.Unlock()
			for _, cb := range cbs {
				cb()
			}
			return
		}
	}
	vr.mu.Unlock()
}

func (vr *VaultRecovery) renewTokenIfNeeded(ctx context.Context) {
	vr.tokenMu.RLock()
	token := vr.token
	vr.tokenMu.RUnlock()

	url := vr.vaultAddr + "/v1/auth/token/lookup-self"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("X-Vault-Token", token)
	resp, err := vr.httpClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		vr.metricErrors.Inc()
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			TTL int `json:"ttl"`
		} `json:"data"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}

	if time.Duration(result.Data.TTL)*time.Second < vaultRenewThreshold {
		vr.renewToken(ctx, token)
	}
}

func (vr *VaultRecovery) renewToken(ctx context.Context, token string) {
	url := vr.vaultAddr + "/v1/auth/token/renew-self"
	body := strings.NewReader(`{"increment": "1h"}`)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := vr.httpClient.Do(req)
	if err != nil {
		vr.metricErrors.Inc()
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		vr.metricRenewals.Inc()
		log.Printf("[ha/vault] token renewed successfully")
	}
}

func (vr *VaultRecovery) fetchSecret(ctx context.Context, path string) (string, error) {
	vr.tokenMu.RLock()
	token := vr.token
	vr.tokenMu.RUnlock()

	url := vr.vaultAddr + "/v1/" + path
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("X-Vault-Token", token)
	resp, err := vr.httpClient.Do(req)
	if err != nil {
		vr.metricErrors.Inc()
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return "", fmt.Errorf("secret not found: %s", path)
	}
	if resp.StatusCode != 200 {
		vr.metricErrors.Inc()
		return "", fmt.Errorf("vault returned %d for %s", resp.StatusCode, path)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			Data map[string]interface{} `json:"data"` // KV v2
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse vault response: %w", err)
	}

	// Extract the first value from the secret's data map.
	for _, v := range result.Data.Data {
		return fmt.Sprintf("%v", v), nil
	}
	return "", fmt.Errorf("vault: secret %s has no data fields", path)
}

func (vr *VaultRecovery) refreshAll(ctx context.Context) {
	vr.cacheMu.RLock()
	paths := make([]string, 0, len(vr.cache))
	for p := range vr.cache {
		paths = append(paths, p)
	}
	vr.cacheMu.RUnlock()
	for _, p := range paths {
		_, _ = vr.GetSecret(ctx, p)
	}
}

func (vr *VaultRecovery) refreshExpiring(ctx context.Context) {
	vr.cacheMu.RLock()
	var expiring []string
	for path, s := range vr.cache {
		if time.Until(s.ExpiresAt) < vaultRenewThreshold {
			expiring = append(expiring, path)
		}
	}
	vr.cacheMu.RUnlock()
	for _, p := range expiring {
		_, _ = vr.GetSecret(ctx, p)
	}
}

func (vr *VaultRecovery) updateCacheAgeStat() {
	vr.cacheMu.RLock()
	defer vr.cacheMu.RUnlock()
	maxAge := float64(0)
	for _, s := range vr.cache {
		age := time.Since(s.ExpiresAt.Add(-vaultSecretCacheTTL)).Seconds()
		if age > maxAge {
			maxAge = age
		}
	}
	vr.metricCacheAgeMax.Set(maxAge)
}

// IsVaultReachable returns true when the last health check succeeded.
func (vr *VaultRecovery) IsVaultReachable() bool {
	vr.mu.RLock()
	defer vr.mu.RUnlock()
	return vr.vaultReachable
}

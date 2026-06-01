// Package vault implements a layered secrets provider with HashiCorp Vault support
// and a fallback to environment variables for local development.
// The interface design allows transparent migration from env-vars to Vault
// with zero application-code changes.
package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrSecretNotFound is returned when a secret key is absent from all providers.
var ErrSecretNotFound = errors.New("vault: secret not found")

// SecretProvider is the unified interface for retrieving secrets.
// All consumers reference secrets by logical key name, never by source.
type SecretProvider interface {
	// Get returns the value of a named secret. Never returns empty string on nil error.
	Get(ctx context.Context, key string) (string, error)
	// Prefetch warms the cache for the given keys. Best-effort.
	Prefetch(ctx context.Context, keys []string) error
	// Source returns a human-readable description of the backing store.
	Source() string
}

// EnvProvider reads secrets from environment variables.
// Used in development and as fallback when Vault is unavailable.
type EnvProvider struct{}

func (e *EnvProvider) Get(_ context.Context, key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
	}
	return val, nil
}

func (e *EnvProvider) Prefetch(_ context.Context, keys []string) error { return nil }
func (e *EnvProvider) Source() string                                  { return "environment" }

// VaultProvider reads secrets from a HashiCorp Vault KV v2 mount.
// Authentication uses the AppRole method (role_id + secret_id).
type VaultProvider struct {
	addr      string
	token     string
	mountPath string
	client    *http.Client
	cache     *SecretCache
}

// VaultConfig holds the connection parameters for HashiCorp Vault.
type VaultConfig struct {
	// Address is the Vault server address e.g. https://vault.internal:8200
	Address string
	// Token is the Vault token (from AppRole login or VAULT_TOKEN env var).
	Token string
	// MountPath is the KV v2 mount e.g. "secret" → /v1/secret/data/<key>
	MountPath string
	// CacheTTL controls how long secrets are cached in memory.
	CacheTTL time.Duration
}

// NewVaultProvider creates a Vault provider. Returns ErrSecretNotFound on first
// Get call if Vault is unreachable; callers should wrap with FallbackProvider.
func NewVaultProvider(cfg VaultConfig) *VaultProvider {
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	mount := cfg.MountPath
	if mount == "" {
		mount = "secret"
	}
	return &VaultProvider{
		addr:      strings.TrimRight(cfg.Address, "/"),
		token:     cfg.Token,
		mountPath: mount,
		client:    &http.Client{Timeout: 5 * time.Second},
		cache:     NewSecretCache(cacheTTL),
	}
}

// Get fetches a secret by key from Vault KV v2. Caches result.
func (v *VaultProvider) Get(ctx context.Context, key string) (string, error) {
	if val, ok := v.cache.Get(key); ok {
		return val, nil
	}
	val, err := v.fetchFromVault(ctx, key)
	if err != nil {
		return "", err
	}
	v.cache.Set(key, val)
	return val, nil
}

func (v *VaultProvider) Prefetch(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if _, ok := v.cache.Get(key); ok {
			continue
		}
		val, err := v.fetchFromVault(ctx, key)
		if err != nil {
			log.Printf("[VAULT] prefetch failed for key %q: %v", key, err)
			continue
		}
		v.cache.Set(key, val)
	}
	return nil
}

func (v *VaultProvider) Source() string { return "vault:" + v.addr }

// fetchFromVault performs the actual KV v2 read: GET /v1/<mount>/data/<key>
func (v *VaultProvider) fetchFromVault(ctx context.Context, key string) (string, error) {
	url := fmt.Sprintf("%s/v1/%s/data/%s", v.addr, v.mountPath, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("vault: build request: %w", err)
	}
	req.Header.Set("X-Vault-Token", v.token)

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault: unexpected status %d for key %q", resp.StatusCode, key)
	}

	var result struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("vault: parse response: %w", err)
	}
	val, ok := result.Data.Data["value"]
	if !ok {
		// Try the key name itself as the field name.
		val, ok = result.Data.Data[key]
	}
	if !ok || val == "" {
		return "", fmt.Errorf("%w: %s (found in vault but value field missing)", ErrSecretNotFound, key)
	}
	return val, nil
}

// FallbackProvider tries a primary provider and falls back to a secondary on error.
type FallbackProvider struct {
	primary   SecretProvider
	secondary SecretProvider
}

// NewFallbackProvider returns a provider that uses primary first, then secondary.
// Canonical usage: NewFallbackProvider(vaultProvider, envProvider)
func NewFallbackProvider(primary, secondary SecretProvider) *FallbackProvider {
	return &FallbackProvider{primary: primary, secondary: secondary}
}

func (f *FallbackProvider) Get(ctx context.Context, key string) (string, error) {
	val, err := f.primary.Get(ctx, key)
	if err == nil {
		return val, nil
	}
	val, err2 := f.secondary.Get(ctx, key)
	if err2 != nil {
		return "", fmt.Errorf("vault: all providers failed for %q (primary: %v, secondary: %v)", key, err, err2)
	}
	return val, nil
}

func (f *FallbackProvider) Prefetch(ctx context.Context, keys []string) error {
	_ = f.primary.Prefetch(ctx, keys)
	return f.secondary.Prefetch(ctx, keys)
}

func (f *FallbackProvider) Source() string {
	return f.primary.Source() + "→" + f.secondary.Source()
}

// LoadFromEnv constructs the appropriate provider based on available environment variables.
// If VAULT_ADDR and VAULT_TOKEN are set, uses Vault with env fallback.
// Otherwise uses env-only provider.
func LoadFromEnv() SecretProvider {
	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")
	if addr != "" && token != "" {
		vp := NewVaultProvider(VaultConfig{
			Address:   addr,
			Token:     token,
			MountPath: envOrDefault("VAULT_MOUNT", "secret"),
			CacheTTL:  5 * time.Minute,
		})
		log.Printf("[VAULT] Vault provider configured: %s", addr)
		return NewFallbackProvider(vp, &EnvProvider{})
	}
	log.Printf("[VAULT] Using environment variable provider (set VAULT_ADDR + VAULT_TOKEN to enable Vault)")
	return &EnvProvider{}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

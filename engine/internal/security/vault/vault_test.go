package vault

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEnvProvider_Get(t *testing.T) {
	t.Setenv("TEST_SECRET", "myvalue")
	p := &EnvProvider{}
	val, err := p.Get(context.Background(), "TEST_SECRET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "myvalue" {
		t.Fatalf("want myvalue got %q", val)
	}
}

func TestEnvProvider_Missing(t *testing.T) {
	p := &EnvProvider{}
	_, err := p.Get(context.Background(), "NONEXISTENT_KEY_XYZ_9999")
	if err == nil {
		t.Fatal("expected error for missing env var")
	}
}

func TestSecretCache_GetSet(t *testing.T) {
	cache := NewSecretCache(time.Minute)
	cache.Set("apiKey", "secret123")
	val, ok := cache.Get("apiKey")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val != "secret123" {
		t.Fatalf("want secret123 got %q", val)
	}
}

func TestSecretCache_Expire(t *testing.T) {
	cache := NewSecretCache(10 * time.Millisecond)
	cache.Set("k1", "v1")
	time.Sleep(20 * time.Millisecond)
	_, ok := cache.Get("k1")
	if ok {
		t.Fatal("expired entry should not be returned")
	}
}

func TestSecretCache_Invalidate(t *testing.T) {
	cache := NewSecretCache(time.Minute)
	cache.Set("k2", "v2")
	cache.Invalidate("k2")
	_, ok := cache.Get("k2")
	if ok {
		t.Fatal("invalidated entry should not be returned")
	}
}

func TestSecretCache_InvalidateAll(t *testing.T) {
	cache := NewSecretCache(time.Minute)
	cache.Set("k1", "v1")
	cache.Set("k2", "v2")
	cache.Set("k3", "v3")
	cache.InvalidateAll()
	if cache.Size() != 0 {
		t.Fatalf("cache should be empty after InvalidateAll, size=%d", cache.Size())
	}
}

func TestVaultProvider_Get_Success(t *testing.T) {
	// Mock Vault server returning a KV v2 response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"data":{"value":"my-api-key"}}}`))
	}))
	defer server.Close()

	vp := NewVaultProvider(VaultConfig{
		Address:   server.URL,
		Token:     "test-token",
		MountPath: "secret",
		CacheTTL:  time.Minute,
	})
	val, err := vp.Get(context.Background(), "BINANCE_API_KEY")
	if err != nil {
		t.Fatalf("vault get error: %v", err)
	}
	if val != "my-api-key" {
		t.Fatalf("want my-api-key got %q", val)
	}
}

func TestVaultProvider_Get_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	vp := NewVaultProvider(VaultConfig{
		Address:   server.URL,
		Token:     "test-token",
		MountPath: "secret",
		CacheTTL:  time.Minute,
	})
	_, err := vp.Get(context.Background(), "MISSING_KEY")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestVaultProvider_Caches_SecondCall(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"data":{"value":"cached-value"}}}`))
	}))
	defer server.Close()

	vp := NewVaultProvider(VaultConfig{
		Address:   server.URL,
		Token:     "tok",
		MountPath: "secret",
		CacheTTL:  time.Minute,
	})
	ctx := context.Background()
	_, _ = vp.Get(ctx, "KEY1")
	_, _ = vp.Get(ctx, "KEY1") // second call should use cache
	if callCount != 1 {
		t.Fatalf("expected 1 Vault call (cache hit on second), got %d", callCount)
	}
}

func TestFallbackProvider_UsesSecondary(t *testing.T) {
	t.Setenv("FALLBACK_SECRET", "fallback-value")
	// Primary always fails.
	primaryFail := &EnvProvider{} // won't find FALLBACK_SECRET in primary context

	// FallbackProvider with env as both (env will find it).
	primary := &mockFailProvider{}
	secondary := &EnvProvider{}
	fp := NewFallbackProvider(primary, secondary)
	val, err := fp.Get(context.Background(), "FALLBACK_SECRET")
	if err != nil {
		t.Fatalf("fallback provider error: %v", err)
	}
	if val != "fallback-value" {
		t.Fatalf("want fallback-value got %q", val)
	}
	_ = primaryFail
}

type mockFailProvider struct{}

func (m *mockFailProvider) Get(_ context.Context, key string) (string, error) {
	return "", ErrSecretNotFound
}
func (m *mockFailProvider) Prefetch(_ context.Context, _ []string) error { return nil }
func (m *mockFailProvider) Source() string                               { return "mock-fail" }

func TestRotationEngine_ManualRotate(t *testing.T) {
	provider := &EnvProvider{}
	policies := []RotationPolicy{{Key: "SOME_KEY"}}
	engine := NewRotationEngine(provider, policies)

	var recorded *RotationRecord
	engine.OnRotation(func(r RotationRecord) {
		recorded = &r
	})

	err := engine.Rotate(context.Background(), "SOME_KEY", "test")
	if err != nil {
		t.Fatalf("rotate error: %v", err)
	}
	time.Sleep(10 * time.Millisecond) // wait for goroutine callback
	if recorded == nil {
		t.Fatal("rotation callback should have been called")
	}
	if recorded.Key != "SOME_KEY" {
		t.Errorf("key: want SOME_KEY got %s", recorded.Key)
	}
	if recorded.By != "test" {
		t.Errorf("by: want test got %s", recorded.By)
	}
}

func TestRotationEngine_History(t *testing.T) {
	provider := &EnvProvider{}
	engine := NewRotationEngine(provider, nil)
	for i := 0; i < 5; i++ {
		_ = engine.Rotate(context.Background(), "KEY_A", "manual")
	}
	history := engine.History(3)
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
}

func TestRotationEngine_EmergencyRotateAll(t *testing.T) {
	provider := &EnvProvider{}
	policies := DefaultRotationPolicies()
	engine := NewRotationEngine(provider, policies)
	errs := engine.EmergencyRotateAll(context.Background())
	// EnvProvider rotation is a no-op (returns success=false but no error).
	if len(errs) > 0 {
		t.Fatalf("emergency rotate returned errors: %v", errs)
	}
	history := engine.History(100)
	if len(history) != len(policies) {
		t.Fatalf("expected %d history entries, got %d", len(policies), len(history))
	}
}

func TestLoadFromEnv_DefaultsToEnv(t *testing.T) {
	// Without VAULT_ADDR set, should return EnvProvider.
	p := LoadFromEnv()
	if p.Source() != "environment" {
		t.Fatalf("without VAULT_ADDR should use env provider, got %q", p.Source())
	}
}

package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const defaultCacheTTL = 5 * time.Minute

// SecretClient retrieves secrets from AWS Secrets Manager with a local cache
// and optional environment variable fallback for development.
type SecretClient struct {
	awsClient   *secretsmanager.Client
	cache       map[string]cachedSecret
	mu          sync.RWMutex
	cacheTTL    time.Duration
	region      string
	useLocal    bool // if true, fall back to env vars when AWS call fails
}

type cachedSecret struct {
	value     string
	expiresAt time.Time
}

// NewSecretClient creates a SecretClient.
//
//   - region: AWS region where secrets are stored (e.g. "ap-south-1")
//   - useLocalFallback: if true, env vars are used when AWS is unreachable
//     (safe for local development; must be false in production)
func NewSecretClient(region string, useLocalFallback bool) (*SecretClient, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil && !useLocalFallback {
		return nil, fmt.Errorf("secrets: load AWS config: %w", err)
	}

	var awsClient *secretsmanager.Client
	if err == nil {
		awsClient = secretsmanager.NewFromConfig(cfg)
	}

	return &SecretClient{
		awsClient: awsClient,
		cache:     make(map[string]cachedSecret),
		cacheTTL:  defaultCacheTTL,
		region:    region,
		useLocal:  useLocalFallback,
	}, nil
}

// Get returns the value for secretPath. It checks the local cache first,
// then AWS Secrets Manager, and optionally falls back to environment variables.
// The raw secret value is never logged.
func (c *SecretClient) Get(secretPath string) (string, error) {
	// ── Cache check ───────────────────────────────────────────────────────────
	c.mu.RLock()
	if entry, ok := c.cache[secretPath]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.RUnlock()
		return entry.value, nil
	}
	c.mu.RUnlock()

	// ── AWS Secrets Manager ───────────────────────────────────────────────────
	if c.awsClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		input := &secretsmanager.GetSecretValueInput{
			SecretId: aws.String(secretPath),
		}
		result, err := c.awsClient.GetSecretValue(ctx, input)
		if err == nil && result.SecretString != nil {
			value := *result.SecretString
			c.storeCache(secretPath, value)
			slog.Debug("[secrets] loaded from AWS Secrets Manager",
				"path", secretPath,
			)
			return value, nil
		}
		if err != nil {
			slog.Warn("[secrets] AWS Secrets Manager call failed",
				"path", secretPath,
				"err", err,
			)
		}
	}

	// ── Env var fallback ──────────────────────────────────────────────────────
	if c.useLocal {
		envKey, ok := envFallbackMap[secretPath]
		if !ok {
			// Best-effort: convert path to env var name.
			envKey = pathToEnvVar(secretPath)
		}
		value := os.Getenv(envKey)
		if value != "" {
			c.storeCache(secretPath, value)
			slog.Debug("[secrets] loaded from env var fallback",
				"path", secretPath,
				"env_key", envKey,
			)
			return value, nil
		}
	}

	return "", fmt.Errorf("secrets: %q not found in AWS Secrets Manager or environment", secretPath)
}

// MustGet returns the secret value or panics with a descriptive error.
// Use only at engine startup for secrets that are required to boot.
func (c *SecretClient) MustGet(secretPath string) string {
	val, err := c.Get(secretPath)
	if err != nil {
		panic(fmt.Sprintf("secrets: required secret %q unavailable: %v", secretPath, err))
	}
	return val
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (c *SecretClient) storeCache(path, value string) {
	c.mu.Lock()
	c.cache[path] = cachedSecret{value: value, expiresAt: time.Now().Add(c.cacheTTL)}
	c.mu.Unlock()
}

// pathToEnvVar converts /btcpilot/angelone/totp → BTCPILOT_ANGELONE_TOTP.
func pathToEnvVar(path string) string {
	s := strings.TrimPrefix(path, "/")
	s = strings.ReplaceAll(s, "/", "_")
	return strings.ToUpper(s)
}

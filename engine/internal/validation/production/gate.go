// Package production implements fail-fast boot gates for institutional deployments.
package production

import (
	"fmt"
	"os"
	"strings"
)

// BootGateConfig holds environment checks for production startup.
type BootGateConfig struct {
	EnforceAuth       bool
	RequireDatabase   bool
	RequireAdminSecret bool
	MinAdminSecretLen int
	ECSMode           bool // true when running in ECS (SQLITE_PATH empty expected)
}

// BootGateResult is the outcome of RunBootGate.
type BootGateResult struct {
	Passed   bool
	Blockers []string
	Warnings []string
}

// DefaultBootGateConfig returns production-safe defaults.
func DefaultBootGateConfig() BootGateConfig {
	enforce := strings.EqualFold(os.Getenv("SECURITY_ENFORCE_AUTH"), "true")
	ecs := os.Getenv("SQLITE_PATH") == "" && os.Getenv("AWS_REGION") != ""
	return BootGateConfig{
		EnforceAuth:        enforce,
		RequireDatabase:    enforce,
		RequireAdminSecret: enforce,
		MinAdminSecretLen:  32,
		ECSMode:            ecs,
	}
}

// RunBootGate validates critical production prerequisites before trading starts.
func RunBootGate(cfg BootGateConfig) BootGateResult {
	r := BootGateResult{Passed: true}

	if !cfg.EnforceAuth {
		r.Warnings = append(r.Warnings, "SECURITY_ENFORCE_AUTH is not true — dev mode")
		return r
	}

	admin := strings.TrimSpace(os.Getenv("ENGINE_ADMIN_SECRET"))
	if cfg.RequireAdminSecret {
		if admin == "" {
			r.Blockers = append(r.Blockers, "ENGINE_ADMIN_SECRET missing — admin endpoints unprotected")
		} else if len(admin) < cfg.MinAdminSecretLen {
			r.Blockers = append(r.Blockers, fmt.Sprintf(
				"ENGINE_ADMIN_SECRET too short (%d chars, min %d)",
				len(admin), cfg.MinAdminSecretLen,
			))
		}
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.RequireDatabase && dbURL == "" {
		r.Blockers = append(r.Blockers, "DATABASE_URL missing — kill switch and ledger are non-durable")
	}

	if cfg.ECSMode && os.Getenv("SQLITE_PATH") != "" {
		r.Warnings = append(r.Warnings, "SQLITE_PATH set in ECS mode — use Aurora only")
	}

	// Detect obvious placeholder secrets
	placeholders := []string{"changeme", "your-secret", "TODO", "example"}
	for _, p := range placeholders {
		if strings.Contains(strings.ToLower(admin), p) {
			r.Blockers = append(r.Blockers, "ENGINE_ADMIN_SECRET appears to be a placeholder value")
			break
		}
	}

	r.Passed = len(r.Blockers) == 0
	return r
}

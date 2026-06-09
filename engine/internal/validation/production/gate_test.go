package production

import (
	"os"
	"testing"
)

func TestRunBootGate_DevModePasses(t *testing.T) {
	os.Unsetenv("SECURITY_ENFORCE_AUTH")
	os.Unsetenv("ENGINE_ADMIN_SECRET")
	os.Unsetenv("DATABASE_URL")

	r := RunBootGate(BootGateConfig{EnforceAuth: false})
	if !r.Passed {
		t.Fatalf("expected pass in dev mode, got blockers: %v", r.Blockers)
	}
}

func TestRunBootGate_ProductionBlocksMissingSecrets(t *testing.T) {
	os.Setenv("SECURITY_ENFORCE_AUTH", "true")
	os.Unsetenv("ENGINE_ADMIN_SECRET")
	os.Unsetenv("DATABASE_URL")
	t.Cleanup(func() {
		os.Unsetenv("SECURITY_ENFORCE_AUTH")
	})

	r := RunBootGate(DefaultBootGateConfig())
	if r.Passed {
		t.Fatal("expected failure when secrets missing in production mode")
	}
	if len(r.Blockers) < 2 {
		t.Fatalf("expected at least 2 blockers, got %v", r.Blockers)
	}
}

func TestRunBootGate_ProductionPassesWithSecrets(t *testing.T) {
	os.Setenv("SECURITY_ENFORCE_AUTH", "true")
	os.Setenv("ENGINE_ADMIN_SECRET", "a-very-long-production-secret-value-32chars-minimum")
	os.Setenv("DATABASE_URL", "postgres://user:pass@host/db?sslmode=require")
	t.Cleanup(func() {
		os.Unsetenv("SECURITY_ENFORCE_AUTH")
		os.Unsetenv("ENGINE_ADMIN_SECRET")
		os.Unsetenv("DATABASE_URL")
	})

	r := RunBootGate(BootGateConfig{
		EnforceAuth:        true,
		RequireDatabase:    true,
		RequireAdminSecret: true,
		MinAdminSecretLen:  32,
	})
	if !r.Passed {
		t.Fatalf("expected pass, got blockers: %v", r.Blockers)
	}
}

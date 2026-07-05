package reconciliationv2

import (
	"os"
	"testing"
)

// The kill-switch hook tests exercise the ARMED kill-switch path (production
// runs with KILL_SWITCH_ENABLED=true in .env, which `go test` does not load).
func TestMain(m *testing.M) {
	if os.Getenv("KILL_SWITCH_ENABLED") == "" {
		os.Setenv("KILL_SWITCH_ENABLED", "true")
	}
	os.Exit(m.Run())
}

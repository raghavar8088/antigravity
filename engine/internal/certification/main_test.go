package certification

import (
	"os"
	"testing"
)

// The certification suites certify the ARMED kill-switch behaviour (production
// runs with KILL_SWITCH_ENABLED=true in .env, which `go test` does not load).
// Without this, killswitch.Service ignores every trigger and the entire
// chaos/DR/security family fails at setup.
func TestMain(m *testing.M) {
	if os.Getenv("KILL_SWITCH_ENABLED") == "" {
		os.Setenv("KILL_SWITCH_ENABLED", "true")
	}
	os.Exit(m.Run())
}

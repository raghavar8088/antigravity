package certification

import (
	"os"
	"testing"
)

// Fund-ops certification certifies the ARMED kill-switch behaviour (production
// runs with KILL_SWITCH_ENABLED=true in .env, which `go test` does not load).
func TestMain(m *testing.M) {
	if os.Getenv("KILL_SWITCH_ENABLED") == "" {
		os.Setenv("KILL_SWITCH_ENABLED", "true")
	}
	os.Exit(m.Run())
}

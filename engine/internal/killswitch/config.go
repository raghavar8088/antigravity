package killswitch

import (
	"os"
	"strings"
)

// EnabledFromEnv returns whether the institutional kill switch is armed.
// Set KILL_SWITCH_ENABLED=true to enable; any other value or unset disables it.
func EnabledFromEnv() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("KILL_SWITCH_ENABLED")))
	switch v {
	case "1", "true", "yes", "on", "enabled":
		return true
	default:
		return false
	}
}

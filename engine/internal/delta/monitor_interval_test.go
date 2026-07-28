package delta

import (
	"testing"
	"time"
)

// SL/TP is enforced by polling, so this interval bounds how far price can run
// past a stop before anything reacts. It was 5 minutes, which let a -50% stop go
// unenforced for a long time on a volatile option.
func TestMonitorInterval_DefaultIsTight(t *testing.T) {
	t.Setenv("LIVE_MONITOR_INTERVAL_SECONDS", "")
	if got := monitorInterval(); got != 30*time.Second {
		t.Fatalf("default monitor interval got %s, want 30s", got)
	}
}

func TestMonitorInterval_EnvOverrideWithFloor(t *testing.T) {
	t.Setenv("LIVE_MONITOR_INTERVAL_SECONDS", "10")
	if got := monitorInterval(); got != 10*time.Second {
		t.Fatalf("override got %s, want 10s", got)
	}
	// Absurdly small values must not be honoured (would hammer the venue).
	t.Setenv("LIVE_MONITOR_INTERVAL_SECONDS", "1")
	if got := monitorInterval(); got != 30*time.Second {
		t.Fatalf("sub-floor override must fall back to the default, got %s", got)
	}
	t.Setenv("LIVE_MONITOR_INTERVAL_SECONDS", "garbage")
	if got := monitorInterval(); got != 30*time.Second {
		t.Fatalf("garbage override must fall back to the default, got %s", got)
	}
}

package main

import "testing"

// The universe must be read from the venue the process trades on.
//
// This was hardcoded to production, so -symbols auto resolved the LIVE universe
// regardless of configuration. The demo desk booted against demo credentials
// and loaded 89 live symbols, none of which exist there — every stream was dead
// on arrival, and the failure looked like a market-data problem rather than a
// misconfiguration.
func TestPerpUniverseBaseURL_FollowsTheConfiguredVenue(t *testing.T) {
	t.Setenv("DELTA_BASE_URL", "")
	t.Setenv("DELTA_API_BASE_URL", "")
	t.Setenv("DELTA_TESTNET", "")
	if got := perpUniverseBaseURL(); got != "https://api.india.delta.exchange" {
		t.Errorf("unconfigured = %q; production is the correct default", got)
	}

	t.Setenv("DELTA_TESTNET", "true")
	if got := perpUniverseBaseURL(); got != "https://cdn-ind.testnet.deltaex.org" {
		t.Errorf("DELTA_TESTNET=true = %q; must not resolve production symbols", got)
	}

	// An explicit host wins over the flag, and a trailing slash must not produce
	// a double slash in the request path.
	t.Setenv("DELTA_BASE_URL", "https://example.test/")
	if got := perpUniverseBaseURL(); got != "https://example.test" {
		t.Errorf("explicit override = %q", got)
	}
}

// Credentials and symbol resolution must never point at different venues:
// product IDs are venue-specific, so that combination can send a well-formed
// order for the wrong instrument with nothing in the request looking wrong.
func TestPerpUniverseBaseURL_TestnetNeverResolvesProduction(t *testing.T) {
	t.Setenv("DELTA_TESTNET", "true")
	t.Setenv("DELTA_BASE_URL", "")
	t.Setenv("DELTA_API_BASE_URL", "")
	for _, bad := range []string{"api.india.delta.exchange", "cdn.india.deltaex.org"} {
		if got := perpUniverseBaseURL(); got == "https://"+bad {
			t.Fatalf("testnet resolved the production host %q", bad)
		}
	}
}

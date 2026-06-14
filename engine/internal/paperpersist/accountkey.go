package paperpersist

// accountkey.go — Authoritative account key validation at engine startup.
//
// The frontend hardcodes FrontendAccountKey in client/src/lib/broker/ownerAccountKey.ts.
// The engine MUST use the same value or Paper Desk reads a different MongoDB
// document set than the engine writes.

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// FrontendAccountKey is the canonical owner account key shared with the Next.js
// frontend (client/src/lib/broker/ownerAccountKey.ts OWNER_ACCOUNT_KEY).
const FrontendAccountKey = "mock_trading_main"

// ValidateAccountKeyAlignment returns an error when the engine account key is
// anonymous, empty, or diverges from the frontend constant.
func ValidateAccountKeyAlignment() error {
	key := AccountKey()

	if key == "" {
		return fmt.Errorf("account key is empty — set OWNER_ACCOUNT_KEY=%q", FrontendAccountKey)
	}
	if strings.HasPrefix(key, "anon_") {
		return fmt.Errorf(
			"anonymous account keys are retired (got %q) — set OWNER_ACCOUNT_KEY=%q on Lightsail and remove DESK_WORKER_ACCOUNT_KEY",
			key, FrontendAccountKey,
		)
	}
	if key != FrontendAccountKey {
		return fmt.Errorf(
			"engine OWNER_ACCOUNT_KEY=%q does not match frontend constant %q — Paper Desk will show empty data",
			key, FrontendAccountKey,
		)
	}
	return nil
}

// LogAccountKeyAlignment prints PASS/FAIL and returns whether startup may continue.
// When fatalOnMismatch is true, mismatches are logged at fatal level.
func LogAccountKeyAlignment(fatalOnMismatch bool) bool {
	err := ValidateAccountKeyAlignment()
	if err == nil {
		log.Printf("[paperpersist/account] ✅ account_key=%q aligned with frontend", AccountKey())
		return true
	}

	msg := fmt.Sprintf("[paperpersist/account] ❌ FATAL account key alignment: %v", err)
	if fatalOnMismatch {
		log.Fatal(msg)
	}
	log.Print(msg)
	return false
}

// EffectiveOwnerAccountKey returns OWNER_ACCOUNT_KEY env or the frontend default.
// Used by envcheck and diagnostics only — runtime writes use package ownerAccountKey.
func EffectiveOwnerAccountKey() string {
	if v := os.Getenv("OWNER_ACCOUNT_KEY"); v != "" {
		return v
	}
	return FrontendAccountKey
}

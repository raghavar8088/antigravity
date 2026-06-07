package paperpersist

// envcheck.go — Engine-side environment variable validation.
//
// Called once at startup (before NewMongoManager) to produce a clear
// human-readable report.  Missing MONGODB_URI causes a loud WARNING not a
// fatal — the engine degrades gracefully to SQLite-only.  A future gate can
// promote this to log.Fatal if full MongoDB persistence is mandatory.

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// EnvStatus is the severity of a single check.
type EnvStatus string

const (
	EnvPass    EnvStatus = "PASS"
	EnvWarning EnvStatus = "WARNING"
	EnvFail    EnvStatus = "FAIL"
)

// EnvCheckResult is one variable's validation outcome.
type EnvCheckResult struct {
	Key       string
	Status    EnvStatus
	ValueHint string // safe display — never logs secrets in plaintext
	Message   string
	Fix       string
}

// EnvReport is the full startup check.
type EnvReport struct {
	OK              bool // true when no FAIL entries
	MongoReady      bool
	AccountKey      string
	DatabaseName    string
	Checks          []EnvCheckResult
}

// CheckEnv validates all variables required by the paperpersist package and
// returns a structured report.  Never reads secrets beyond checking presence.
func CheckEnv() EnvReport {
	var checks []EnvCheckResult

	// MONGODB_URI
	uri := os.Getenv("MONGODB_URI")
	switch {
	case uri == "":
		checks = append(checks, EnvCheckResult{
			Key:       "MONGODB_URI",
			Status:    EnvFail,
			ValueHint: "missing",
			Message:   "Not set — engine will run without MongoDB persistence (SQLite only).",
			Fix:       "Set MONGODB_URI=mongodb+srv://... in the engine environment or root .env file.",
		})
	case !strings.HasPrefix(uri, "mongodb+srv://") && !strings.HasPrefix(uri, "mongodb://"):
		checks = append(checks, EnvCheckResult{
			Key:       "MONGODB_URI",
			Status:    EnvFail,
			ValueHint: "set (wrong scheme)",
			Message:   "URI must start with mongodb+srv:// or mongodb://.",
			Fix:       "Correct the URI in the engine environment.",
		})
	default:
		// Mask credentials for the hint
		masked := uri
		if at := strings.Index(uri, "@"); at > 0 {
			if scheme := strings.Index(uri, "://"); scheme >= 0 {
				masked = uri[:scheme+3] + "<redacted>@" + uri[at+1:]
			}
		}
		checks = append(checks, EnvCheckResult{
			Key:       "MONGODB_URI",
			Status:    EnvPass,
			ValueHint: masked,
			Message:   "MongoDB URI configured.",
		})
	}

	// MONGODB_DB
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		checks = append(checks, EnvCheckResult{
			Key:       "MONGODB_DB",
			Status:    EnvWarning,
			ValueHint: "not set (defaulting to \"loop_trades\")",
			Message:   "No MONGODB_DB set — using default \"loop_trades\".",
			Fix:       "Add MONGODB_DB=loop_trades to make this explicit.",
		})
		dbName = "loop_trades"
	} else {
		checks = append(checks, EnvCheckResult{
			Key:       "MONGODB_DB",
			Status:    EnvPass,
			ValueHint: fmt.Sprintf("%q", dbName),
			Message:   "Database name configured.",
		})
	}

	// OWNER_ACCOUNT_KEY — must match client/src/lib/ownerAuth.ts exactly.
	effectiveKey := EffectiveOwnerAccountKey()
	switch err := ValidateAccountKeyAlignment(); {
	case err != nil:
		checks = append(checks, EnvCheckResult{
			Key:       "OWNER_ACCOUNT_KEY",
			Status:    EnvFail,
			ValueHint: fmt.Sprintf("%q", effectiveKey),
			Message:   err.Error(),
			Fix:       fmt.Sprintf("Set OWNER_ACCOUNT_KEY=%q on Lightsail/Vercel and remove any DESK_WORKER_ACCOUNT_KEY=anon_* values.", FrontendAccountKey),
		})
	case os.Getenv("OWNER_ACCOUNT_KEY") == "":
		checks = append(checks, EnvCheckResult{
			Key:       "OWNER_ACCOUNT_KEY",
			Status:    EnvPass,
			ValueHint: fmt.Sprintf("not set (effective: %q)", effectiveKey),
			Message:   fmt.Sprintf("Using default %q — matches frontend constant.", FrontendAccountKey),
		})
	default:
		checks = append(checks, EnvCheckResult{
			Key:       "OWNER_ACCOUNT_KEY",
			Status:    EnvPass,
			ValueHint: fmt.Sprintf("%q", effectiveKey),
			Message:   "Explicitly set and matches frontend constant.",
		})
	}

	// Legacy worker key — warn when still set to a different value.
	if legacy := os.Getenv("DESK_WORKER_ACCOUNT_KEY"); legacy != "" && legacy != effectiveKey {
		status := EnvWarning
		msg := fmt.Sprintf("DESK_WORKER_ACCOUNT_KEY=%q is deprecated — all paths use %q", legacy, effectiveKey)
		if strings.HasPrefix(legacy, "anon_") {
			status = EnvFail
			msg = fmt.Sprintf("DESK_WORKER_ACCOUNT_KEY=%q is a retired anonymous key", legacy)
		}
		checks = append(checks, EnvCheckResult{
			Key:       "DESK_WORKER_ACCOUNT_KEY",
			Status:    status,
			ValueHint: fmt.Sprintf("%q", legacy),
			Message:   msg,
			Fix:       fmt.Sprintf("Unset DESK_WORKER_ACCOUNT_KEY or set it to %q. Remove anon_* keys from VPS/Vercel env.", FrontendAccountKey),
		})
	}

	fails := 0
	for _, c := range checks {
		if c.Status == EnvFail {
			fails++
		}
	}
	mongoURI := os.Getenv("MONGODB_URI")
	mongoReady := mongoURI != "" &&
		(strings.HasPrefix(mongoURI, "mongodb+srv://") || strings.HasPrefix(mongoURI, "mongodb://"))

	return EnvReport{
		OK:           fails == 0,
		MongoReady:   mongoReady,
		AccountKey:   effectiveKey,
		DatabaseName: dbName,
		Checks:       checks,
	}
}

// LogEnvReport prints the startup env report.  Call before NewMongoManager.
func LogEnvReport() EnvReport {
	r := CheckEnv()
	sep := strings.Repeat("═", 62)
	log.Printf("[paperpersist/env] %s", sep)
	log.Printf("[paperpersist/env]   Engine Persistence — Environment Report")
	log.Printf("[paperpersist/env] %s", sep)
	for _, c := range r.Checks {
		icon := "✅"
		if c.Status == EnvWarning {
			icon = "⚠️ "
		} else if c.Status == EnvFail {
			icon = "❌"
		}
		log.Printf("[paperpersist/env]   %s %-28s %s  %s", icon, c.Key, c.Status, c.ValueHint)
		if c.Status != EnvPass {
			log.Printf("[paperpersist/env]       Problem : %s", c.Message)
			if c.Fix != "" {
				log.Printf("[paperpersist/env]       Fix     : %s", c.Fix)
			}
		}
	}
	log.Printf("[paperpersist/env] %s", sep)
	log.Printf("[paperpersist/env]   mongo_ready  : %v", r.MongoReady)
	log.Printf("[paperpersist/env]   account_key  : %s", r.AccountKey)
	log.Printf("[paperpersist/env]   database     : %s", r.DatabaseName)
	if !r.MongoReady {
		log.Printf("[paperpersist/env]   ⛔  MongoDB is NOT configured — engine running in SQLite-only mode.")
		log.Printf("[paperpersist/env]      Paper Desk UI will show empty state until MONGODB_URI is set.")
	} else {
		log.Printf("[paperpersist/env]   ✅  MongoDB environment ready — connecting...")
	}
	log.Printf("[paperpersist/env] %s", sep)
	return r
}

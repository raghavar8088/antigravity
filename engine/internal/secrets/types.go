// Package secrets provides a cached AWS Secrets Manager client with local
// environment variable fallback for development environments.
//
// All sensitive credentials (broker keys, TOTP secrets, JWT signing keys) are
// read from AWS Secrets Manager in production. The engine binary never logs
// or surfaces raw secret values.
package secrets

// Secret path constants — mirror the paths stored in AWS Secrets Manager.
// These are also used as the keys for the env-var fallback mapping.
const (
	SecretAngelOneTOTP   = "/btcpilot/angelone/totp"
	SecretAngelOneAPIKey = "/btcpilot/angelone/api-key"
	SecretDeltaAPIKey    = "/btcpilot/delta/api-key"
	SecretDeltaAPISecret = "/btcpilot/delta/api-secret"
	SecretMongoURI       = "/btcpilot/mongodb/uri"
	SecretJWTSecret      = "/btcpilot/auth/jwt-secret"
)

// envFallbackMap converts a Secrets Manager path to its equivalent env var
// name when UseLocalFallback is enabled.
var envFallbackMap = map[string]string{
	SecretAngelOneTOTP:   "ANGELONE_TOTP_SECRET",
	SecretAngelOneAPIKey: "ANGELONE_API_KEY",
	SecretDeltaAPIKey:    "DELTA_API_KEY",
	SecretDeltaAPISecret: "DELTA_API_SECRET",
	SecretMongoURI:       "MONGODB_URI",
	SecretJWTSecret:      "AUTH_JWT_SECRET",
}

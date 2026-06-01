package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ErrJWT categories for caller differentiation.
var (
	ErrJWTMalformed  = errors.New("jwt: malformed token")
	ErrJWTExpired    = errors.New("jwt: token expired")
	ErrJWTInvalid    = errors.New("jwt: invalid signature")
	ErrJWTClaims     = errors.New("jwt: claims invalid")
)

// JWTClaims represents the standard + custom claims in the engine JWT.
// Interoperable with the Next.js jwtSession.ts implementation (HS256 / JSON payload).
type JWTClaims struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Role      string `json:"role,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// Expired returns true if the token is past its expiry.
func (c JWTClaims) Expired() bool {
	return time.Now().Unix() > c.ExpiresAt
}

// jwtHeader is the fixed base64url-encoded header for HS256 JWTs.
const jwtHeader = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

// ParseJWT validates an HS256 JWT token signed with secret and returns its claims.
// Compatible with tokens produced by the Next.js jwtSession.ts module.
func ParseJWT(token, secret string) (JWTClaims, error) {
	if token == "" {
		return JWTClaims{}, ErrJWTMalformed
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return JWTClaims{}, ErrJWTMalformed
	}

	// Verify signature: HMAC-SHA256(header + "." + payload, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return JWTClaims{}, ErrJWTInvalid
	}

	// Decode payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return JWTClaims{}, ErrJWTMalformed
	}
	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return JWTClaims{}, ErrJWTClaims
	}
	if claims.ExpiresAt > 0 && claims.Expired() {
		return JWTClaims{}, ErrJWTExpired
	}
	return claims, nil
}

// IssueJWT creates a signed HS256 JWT for internal service-to-service use.
func IssueJWT(claims JWTClaims, secret string) (string, error) {
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	sigInput := jwtHeader + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return sigInput + "." + sig, nil
}

// ExtractBearerToken parses the Authorization header and returns the token.
// Accepts: "Bearer <token>" format.
func ExtractBearerToken(authHeader string) (string, bool) {
	if len(authHeader) <= 7 {
		return "", false
	}
	prefix := authHeader[:7]
	if !strings.EqualFold(prefix, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(authHeader[7:])
	if token == "" {
		return "", false
	}
	return token, true
}

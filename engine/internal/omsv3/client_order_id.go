package omsv3

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// GenerateClientOrderID creates an institutional-grade client order ID.
//
// Format: {SYMBOL}-{YYYYMMDD}-{HHMMSS}-{8HEX}
// Example: BTCUSDT-20260601-143022-a3f7bc92
//
// Properties:
//   - Lexicographically sortable by trade time
//   - Unique across restarts (random 4-byte suffix)
//   - Exchange-identifiable (symbol prefix)
//   - Idempotency-key-safe (used as the ledger IdempotencyKey prefix)
func GenerateClientOrderID(symbol string) (string, error) {
	normalized := strings.ToUpper(strings.ReplaceAll(symbol, "-", ""))
	if normalized == "" {
		normalized = "UNKNOWN"
	}
	if len(normalized) > 8 {
		normalized = normalized[:8]
	}

	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("omsv3: GenerateClientOrderID random bytes: %w", err)
	}

	now := time.Now().UTC()
	return fmt.Sprintf("%s-%s-%s-%s",
		normalized,
		now.Format("20060102"),
		now.Format("150405"),
		hex.EncodeToString(b[:]),
	), nil
}

// ParsedOrderID is the decoded form of a client order ID.
type ParsedOrderID struct {
	Symbol string
	Date   string // YYYYMMDD
	Time   string // HHMMSS
	Suffix string // 8-char hex
}

// ParseClientOrderID validates and decodes a GenerateClientOrderID result.
// Returns (ParsedOrderID, true) on success; ("", false) when the format is invalid.
func ParseClientOrderID(id string) (ParsedOrderID, bool) {
	parts := strings.Split(id, "-")
	if len(parts) != 4 {
		return ParsedOrderID{}, false
	}
	if len(parts[1]) != 8 || len(parts[2]) != 6 || len(parts[3]) != 8 {
		return ParsedOrderID{}, false
	}
	if _, err := hex.DecodeString(parts[3]); err != nil {
		return ParsedOrderID{}, false
	}
	return ParsedOrderID{
		Symbol: parts[0],
		Date:   parts[1],
		Time:   parts[2],
		Suffix: parts[3],
	}, true
}

// IdempotencyKey returns the idempotency key for a specific event on this order.
// Stored in the ledger to prevent duplicate event submissions.
func IdempotencyKey(clientOrderID string, eventType string) string {
	return clientOrderID + ":" + eventType
}

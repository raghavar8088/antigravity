package eventstore

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
)

// generateID produces a 16-byte hex event ID.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", monotonic())
	}
	return hex.EncodeToString(b)
}

// typeName returns the reflect type name of v for use as event_type.
func typeName(v interface{}) string {
	if v == nil {
		return "unknown"
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

var monoCounter int64

func monotonic() int64 {
	monoCounter++
	return monoCounter
}

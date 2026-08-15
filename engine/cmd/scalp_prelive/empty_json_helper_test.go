package main

import (
	"os"
	"testing"
)

// readSourceFile reads a file from this package's directory for source-level
// assertions.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

package paperpersist

import (
	"os"
	"testing"
)

func TestValidateAccountKeyAlignmentDefault(t *testing.T) {
	os.Unsetenv("OWNER_ACCOUNT_KEY")
	if err := ValidateAccountKeyAlignment(); err != nil {
		t.Fatalf("expected pass for default key, got %v", err)
	}
}

func TestValidateAccountKeyAlignmentRejectsAnon(t *testing.T) {
	t.Setenv("OWNER_ACCOUNT_KEY", "anon_e7da5e39")
	if err := ValidateAccountKeyAlignment(); err == nil {
		t.Fatal("expected error for anon key")
	}
}

func TestValidateAccountKeyAlignmentRejectsMismatch(t *testing.T) {
	t.Setenv("OWNER_ACCOUNT_KEY", "mock_trading_default")
	if err := ValidateAccountKeyAlignment(); err == nil {
		t.Fatal("expected mismatch error")
	}
}

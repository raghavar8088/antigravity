package v2

import "testing"

func TestEnforceExecutionFloorRejectsSubFloor(t *testing.T) {
	_, err := EnforceExecutionFloor("TestStrat", 0.005, 0.01, 0.5)
	if err == nil {
		t.Fatal("expected rejection below MinExecutionSizeBTC")
	}
	if err.Error() != "RISK_REJECT: recommended size below execution floor" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnforceExecutionFloorAllowsAtFloor(t *testing.T) {
	size, err := EnforceExecutionFloor("TestStrat", MinExecutionSizeBTC, 0.02, 1.0)
	if err != nil {
		t.Fatalf("expected pass at floor: %v", err)
	}
	if size != MinExecutionSizeBTC {
		t.Fatalf("expected size unchanged, got %.6f", size)
	}
}

package v2

import "testing"

// The execution floor is a BTC-denominated FUTURES sizing rule: do not take a
// sub-0.01 BTC position. It must keep rejecting sub-floor futures sizes — that
// control is not weakened anywhere.
func TestEnforceExecutionFloor_StillRejectsSubFloorFutures(t *testing.T) {
	t.Setenv("MIN_EXECUTION_SIZE_BTC", "")
	if _, err := EnforceExecutionFloor("SomeFuturesStrategy", 0.004, 0.2, 1.0); err == nil {
		t.Fatal("a sub-floor futures size must still be rejected")
	}
	if _, err := EnforceExecutionFloor("SomeFuturesStrategy", 0.01, 0.2, 1.0); err != nil {
		t.Fatalf("a size at the floor must pass, got %v", err)
	}
	if _, err := EnforceExecutionFloor("SomeFuturesStrategy", 0.5, 0.2, 1.0); err != nil {
		t.Fatalf("a size above the floor must pass, got %v", err)
	}
}

// Documents why the Live Engine option path is exempt: one Delta option contract
// is 0.001 BTC, an order of magnitude under the futures floor, so the floor could
// never be satisfied by a legitimate 1-contract option buy.
func TestExecutionFloor_OptionContractIsBelowFuturesFloorByDesign(t *testing.T) {
	t.Setenv("MIN_EXECUTION_SIZE_BTC", "")
	const oneOptionContractBTC = 0.001
	if oneOptionContractBTC >= executionFloor() {
		t.Fatal("premise changed: an option contract now meets the futures floor — re-check the exemption")
	}
}

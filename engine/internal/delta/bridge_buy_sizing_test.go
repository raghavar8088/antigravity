package delta

import "testing"

// Defect 1: a $100 wallet — the configured account size — must land in the
// intended 10% / 3-contract tier, not fall through to the 12% / 5 default.
func TestTieredBuySizing_HundredDollarWalletIsTenPercentTier(t *testing.T) {
	t.Setenv("DELTA_BUY_RISK_PCT", "")
	t.Setenv("DELTA_BUY_MAX_CONTRACTS", "")
	risk, maxC, _ := TieredBuySizing(100)
	if risk != 0.10 {
		t.Fatalf("risk at $100: got %.2f, want 0.10", risk)
	}
	if maxC != 3 {
		t.Fatalf("maxContracts at $100: got %d, want 3", maxC)
	}
}

func TestTieredBuySizing_AboveHundredIsDefaultTier(t *testing.T) {
	t.Setenv("DELTA_BUY_RISK_PCT", "")
	t.Setenv("DELTA_BUY_MAX_CONTRACTS", "")
	risk, maxC, _ := TieredBuySizing(100.01)
	if risk != 0.12 || maxC != 5 {
		t.Fatalf("above $100: got risk=%.2f maxC=%d, want 0.12/5", risk, maxC)
	}
}

// Defect 3: when the risk budget cannot afford one contract, the result is zero
// with a skip reason — never a forced position larger than the budget.
func TestBuyingContractsFromWallet_ZeroWhenBudgetCannotAffordOne(t *testing.T) {
	t.Setenv("DELTA_BUY_RISK_PCT", "")
	t.Setenv("DELTA_BUY_MAX_CONTRACTS", "")
	// $100 wallet, 10% budget = $10; premium $12/contract → cannot afford one.
	n, reason := BuyingContractsFromWallet(100, 12)
	if n != 0 {
		t.Fatalf("expected 0 contracts when budget < premium, got %d", n)
	}
	if reason == "" {
		t.Fatal("expected a skip reason when returning zero contracts")
	}
}

// Sizing uses floor: contracts × premium never exceeds the budget.
func TestBuyingContractsFromWallet_FloorsNeverExceedsBudget(t *testing.T) {
	t.Setenv("DELTA_BUY_RISK_PCT", "")
	t.Setenv("DELTA_BUY_MAX_CONTRACTS", "")
	// $100 wallet, 10% = $10 budget; premium $1.78 → floor(10/1.78)=5, capped at maxC=3.
	n, reason := BuyingContractsFromWallet(100, 1.78)
	if reason != "" {
		t.Fatalf("unexpected skip: %s", reason)
	}
	if n != 3 {
		t.Fatalf("got %d contracts, want 3 (maxC cap)", n)
	}
	if cost := float64(n) * 1.78; cost > 100*0.10+1e-9 {
		t.Fatalf("cost $%.2f exceeds budget $10.00", cost)
	}
}

func TestBuyingContractsFromWallet_BelowMinWalletIsZero(t *testing.T) {
	t.Setenv("DELTA_MIN_WALLET_USD", "5")
	n, reason := BuyingContractsFromWallet(3, 1.0)
	if n != 0 || reason == "" {
		t.Fatalf("wallet below minimum must return 0 with reason, got n=%d reason=%q", n, reason)
	}
}

// The independent server-side backstop rejects an over-budget order regardless of
// how the contract count was produced.
func TestAssertBuyWithinBudget(t *testing.T) {
	t.Setenv("DELTA_BUY_RISK_PCT", "")
	t.Setenv("DELTA_BUY_MAX_CONTRACTS", "")
	// $100 wallet, 10% = $10 budget. 3 contracts × $1.78 = $5.34 → within budget.
	if err := AssertBuyWithinBudget(100, 1.78, 3); err != nil {
		t.Fatalf("within-budget order rejected: %v", err)
	}
	// 3 contracts × $4.00 = $12.00 > $10 budget → must reject.
	if err := AssertBuyWithinBudget(100, 4.00, 3); err == nil {
		t.Fatal("expected rejection of over-budget order")
	}
	// Exceeding the max contract count for the tier → reject.
	if err := AssertBuyWithinBudget(100, 0.10, 4); err == nil {
		t.Fatal("expected rejection when contracts exceed tier max")
	}
	// No real premium → cannot verify → reject.
	if err := AssertBuyWithinBudget(100, 0, 1); err == nil {
		t.Fatal("expected rejection when premium quote is missing")
	}
}

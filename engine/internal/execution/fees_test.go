package execution

import "testing"

func TestCanonicalTradeFees(t *testing.T) {
	fees := CanonicalTradeFees(100_000, 101_000, 0.01)
	if fees.EntryFee <= 0 || fees.ExitFee <= 0 {
		t.Fatalf("expected positive fees: %+v", fees)
	}
	if fees.TotalFee != fees.EntryFee+fees.ExitFee {
		t.Fatalf("total fee mismatch: %+v", fees)
	}
}

func TestCanonicalNetPnL(t *testing.T) {
	gross := 100.0
	fees := CanonicalTradeFees(50_000, 50_100, 0.02)
	net := CanonicalNetPnL(gross, fees)
	if net >= gross {
		t.Fatalf("net should be less than gross after fees: gross=%.4f net=%.4f", gross, net)
	}
}

func TestCalculateNetPnLUsesFees(t *testing.T) {
	gross := 50.0
	net := CalculateNetPnL(gross, 100_000, 100_500, 0.01)
	fees := CanonicalTradeFees(100_000, 100_500, 0.01)
	want := CanonicalNetPnL(gross, fees)
	if net != want {
		t.Fatalf("CalculateNetPnL=%.6f want=%.6f", net, want)
	}
}

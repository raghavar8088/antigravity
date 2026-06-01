package v3

import (
	"math"
	"testing"
)

func TestCommissionEngine_BinanceTakerFee(t *testing.T) {
	eng := NewCommissionEngine(ExchangeBinance, TierStandard)
	result := eng.Calculate(100_000, ClassTaker)
	wantBps := 4.0
	if math.Abs(result.FeeBps-wantBps) > 0.001 {
		t.Fatalf("Binance standard taker fee: want %f bps got %f", wantBps, result.FeeBps)
	}
	wantUSD := 100_000 * 4.0 / 10_000
	if math.Abs(result.FeeUSD-wantUSD) > 0.01 {
		t.Fatalf("Binance taker fee USD: want %f got %f", wantUSD, result.FeeUSD)
	}
}

func TestCommissionEngine_BybitMakerRebate(t *testing.T) {
	eng := NewCommissionEngine(ExchangeBybit, TierStandard)
	result := eng.Calculate(100_000, ClassMaker)
	if result.FeeBps >= 0 {
		t.Fatal("Bybit maker should be a rebate (negative bps)")
	}
	if !result.IsRebate {
		t.Fatal("IsRebate should be true for Bybit maker")
	}
}

func TestCommissionEngine_AllExchanges_ValidFees(t *testing.T) {
	exchanges := []Exchange{ExchangeBinance, ExchangeBybit, ExchangeOKX, ExchangeDelta}
	tiers := []VolumeTier{TierStandard, TierTier1, TierTier2, TierTier3, TierVIP}
	for _, ex := range exchanges {
		for _, tier := range tiers {
			eng := NewCommissionEngine(ex, tier)
			r := eng.Calculate(50_000, ClassTaker)
			if r.FeeUSD < -100 {
				t.Errorf("%s/%s: fee USD suspiciously negative: %f", ex, tier, r.FeeUSD)
			}
			rt := eng.RoundTrip(50_000, ClassMaker, ClassTaker)
			if rt.RoundTripBps == 0 {
				t.Errorf("%s/%s: round-trip bps is zero (unexpected)", ex, tier)
			}
		}
	}
}

func TestCommissionEngine_VIPCheaperThanStandard(t *testing.T) {
	std := NewCommissionEngine(ExchangeBinance, TierStandard)
	vip := NewCommissionEngine(ExchangeBinance, TierVIP)
	rStd := std.Calculate(100_000, ClassTaker)
	rVIP := vip.Calculate(100_000, ClassTaker)
	if rVIP.FeeUSD >= rStd.FeeUSD {
		t.Fatalf("VIP fee (%f) should be less than standard fee (%f)", rVIP.FeeUSD, rStd.FeeUSD)
	}
}

func TestCommissionEngine_RoundTrip_GreaterThanSingleLeg(t *testing.T) {
	eng := NewCommissionEngine(ExchangeBinance, TierStandard)
	single := eng.Calculate(100_000, ClassTaker)
	rt := eng.RoundTrip(100_000, ClassTaker, ClassTaker)
	if rt.RoundTripUSD <= single.FeeUSD {
		t.Fatal("round-trip cost must exceed single leg cost")
	}
}

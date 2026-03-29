package risk

import (
	"math"
	"testing"

	"antigravity-engine/internal/strategy"
)

func TestValidateRejectsOversizedShort(t *testing.T) {
	engine := NewRiskEngine(RiskProfile{
		MaxPositionBTC:  1,
		MaxCapitalUSD:   100000,
		MaxDailyLossPct: 0.05,
	})

	err := engine.Validate(strategy.Signal{
		Symbol:     "BTC-USD",
		Action:     strategy.ActionSell,
		TargetSize: 1.1,
	}, 50000)
	if err == nil {
		t.Fatal("expected oversized short to be rejected")
	}
}

func TestNotifyFillTracksSignedExposure(t *testing.T) {
	engine := NewRiskEngine(RiskProfile{MaxPositionBTC: 2})

	engine.NotifyFill(strategy.Signal{Action: strategy.ActionSell, TargetSize: 0.4})
	if got := engine.GetExposure(); got != -0.4 {
		t.Fatalf("expected -0.4 BTC net exposure, got %.4f", got)
	}
	if got := engine.GetAbsoluteExposure(); got != 0.4 {
		t.Fatalf("expected 0.4 BTC absolute exposure, got %.4f", got)
	}

	engine.NotifyFill(strategy.Signal{Action: strategy.ActionBuy, TargetSize: 0.1})
	if got := engine.GetExposure(); math.Abs(got-(-0.3)) > 1e-9 {
		t.Fatalf("expected -0.3 BTC net exposure after cover, got %.4f", got)
	}
}

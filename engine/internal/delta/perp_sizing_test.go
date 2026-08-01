package delta

import (
	"errors"
	"math"
	"testing"
)

// A $100 live account is small enough that sizing mistakes are terminal rather
// than merely expensive. The paper desk's flat $3,000 notional is 30x this
// balance, and one 0.35% stop on it costs $10.50 — over a tenth of the account
// per trade — so the size cannot simply be copied across.

func riskTestRegistry(t *testing.T) *PerpRegistry {
	return registryFrom(t, realPerpTickers)
}

// Risk-based sizing: a 2% risk on $100 with a 0.35% stop should put $2 at stake,
// not a tenth of the account.
func TestPlanPerpOrder_SizesFromRiskNotFromAFixedNotional(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	entry := 0.17258828
	stop := entry * (1 - 0.0035) // the scalp desk's 0.35% stop
	target := entry * (1 + 0.0070)

	plan, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, stop, target, 0, 0)
	if err != nil {
		t.Fatalf("PlanPerpOrder: %v", err)
	}
	// $100 x 2% / 0.0035 = $571 notional, capped by 5x leverage to $500.
	if plan.NotionalUSD > 500.01 {
		t.Errorf("notional $%.2f exceeds the 5x leverage cap on $100", plan.NotionalUSD)
	}
	if plan.RiskUSD > 2.5 {
		t.Errorf("risk $%.2f on a $100 account; the paper desk's $3,000 ticket would risk $10.50",
			plan.RiskUSD)
	}
	if plan.Contracts < 1 {
		t.Errorf("sized %d contracts", plan.Contracts)
	}
	if plan.Side != "buy" {
		t.Errorf("side %q, want buy", plan.Side)
	}
}

// Leverage must be bounded independently of the risk maths. A very tight stop
// makes "2% risk" imply enormous notional, and a resting stop can gap through —
// it is not a guarantee.
func TestPlanPerpOrder_CapsLeverageWhenTheStopIsTight(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	entry := 63083.98754328
	stop := entry * (1 - 0.0005) // 0.05% stop: 2% risk would imply 40x
	plan, err := PlanPerpOrder(reg, cfg, "BTCUSD", true, entry, stop, entry*1.001, 0, 0)
	if err != nil {
		t.Fatalf("PlanPerpOrder: %v", err)
	}
	if plan.Leverage > cfg.MaxLeverage+1e-6 {
		t.Errorf("leverage %.2fx exceeds the %.0fx cap — a tight stop must not buy unlimited size",
			plan.Leverage, cfg.MaxLeverage)
	}
}

// The concurrency cap is what stops a desk that runs hundreds of paper streams
// from trying to hold hundreds of funded positions.
func TestPlanPerpOrder_RefusesBeyondTheConcurrencyCap(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	_, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, 0.17, 0.169, 0.172, cfg.MaxConcurrentPositions, 0)
	if !errors.Is(err, ErrTooManyPositions) {
		t.Fatalf("at the cap the planner gave %v, want ErrTooManyPositions", err)
	}
}

// A stop on the wrong side of entry inverts the sizing maths silently: the
// "risk" becomes a gain and the notional goes the wrong way.
func TestPlanPerpOrder_RejectsAStopOnTheWrongSide(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, 0.17, 0.18, 0.19, 0, 0); err == nil {
		t.Error("a LONG with its stop above entry was accepted")
	}
	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", false, 0.17, 0.16, 0.15, 0, 0); err == nil {
		t.Error("a SHORT with its stop below entry was accepted")
	}
}

// When the risk-sized order is below one contract the signal must be SKIPPED,
// never rounded up. Rounding up is how a "$2 risk" silently becomes a $12 risk
// on an account that cannot absorb it.
func TestPlanPerpOrder_SkipsRatherThanRoundingUp(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(1) // $1 account: one BTCUSD contract is ~$63

	entry := 63083.98754328
	_, err := PlanPerpOrder(reg, cfg, "BTCUSD", true, entry, entry*0.9965, entry*1.007, 0, 0)
	if !errors.Is(err, ErrRiskTooSmall) {
		t.Fatalf("a sub-contract order gave %v, want ErrRiskTooSmall", err)
	}
}

// A short must be sized identically and routed sell-side.
func TestPlanPerpOrder_HandlesShorts(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	entry := 579.25787832
	plan, err := PlanPerpOrder(reg, cfg, "BNBUSD", false, entry, entry*1.0035, entry*0.993, 0, 0)
	if err != nil {
		t.Fatalf("PlanPerpOrder: %v", err)
	}
	if plan.Side != "sell" {
		t.Errorf("side %q, want sell", plan.Side)
	}
	if plan.StopPrice <= plan.LimitPrice {
		t.Errorf("short stop %.4f must sit above entry %.4f", plan.StopPrice, plan.LimitPrice)
	}
	if plan.RiskUSD > 2.5 {
		t.Errorf("risk $%.2f on $100", plan.RiskUSD)
	}
}

// Prices must land on the venue's tick. Delta rejects an off-tick limit, and a
// rejected entry reads as a strategy that declined to trade rather than as an
// order that was never valid.
func TestPlanPerpOrder_SnapsPricesToTheVenueTick(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	// BTCUSD tick is 0.5.
	plan, err := PlanPerpOrder(reg, cfg, "BTCUSD", true, 63083.98754328, 62863.19, 63525.57, 0, 0)
	if err != nil {
		t.Fatalf("PlanPerpOrder: %v", err)
	}
	for name, p := range map[string]float64{"limit": plan.LimitPrice, "stop": plan.StopPrice, "target": plan.TargetPrice} {
		if math.Abs(p/0.5-math.Round(p/0.5)) > 1e-9 {
			t.Errorf("%s price %.6f is not on the 0.5 tick", name, p)
		}
	}
}

// An unfunded account must size nothing at all.
func TestPlanPerpOrder_RefusesWithNoEquity(t *testing.T) {
	reg := riskTestRegistry(t)
	if _, err := PlanPerpOrder(reg, DefaultPerpRiskConfig(0), "ADAUSD", true, 0.17, 0.169, 0.172, 0, 0); err == nil {
		t.Fatal("sized an order against zero equity")
	}
}

// The $100 posture itself: conservative enough that a bad day is survivable.
func TestDefaultPerpRiskConfig_IsConservativeForASmallAccount(t *testing.T) {
	cfg := DefaultPerpRiskConfig(100)
	if cfg.RiskPerTradeFraction > 0.02 {
		t.Errorf("risk per trade %.1f%% is above 2%% on a $100 account", cfg.RiskPerTradeFraction*100)
	}
	if cfg.MaxLeverage > 5 {
		t.Errorf("leverage cap %.0fx is too high for this account", cfg.MaxLeverage)
	}
	if cfg.MaxConcurrentPositions > 3 {
		t.Errorf("%d concurrent positions on $100 leaves each one unfundable", cfg.MaxConcurrentPositions)
	}
	// Worst case: every concurrent position stops out at once.
	worst := cfg.RiskPerTradeFraction * float64(cfg.MaxConcurrentPositions) * 100
	if worst > 10 {
		t.Errorf("all positions stopping together costs %.0f%% of the account", worst)
	}
}

// The per-order leverage cap alone is misleading: three positions at the
// per-order ceiling is three times that ceiling on one account. On a $116 wallet
// shared with the options engine, a 15x book was never fundable — and the
// resulting margin rejections would have read as an infrastructure fault rather
// than as a risk config that could not work.
func TestPlanPerpOrder_CapsAggregateExposureAcrossOpenPositions(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)
	ceiling := cfg.EquityUSD * cfg.MaxAggregateLeverage

	entry := 0.17258828
	stop := entry * (1 - 0.0035)

	// A book already at half the ceiling may only add the remaining half.
	plan, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, stop, entry*1.007, 1, ceiling/2)
	if err != nil {
		t.Fatalf("PlanPerpOrder: %v", err)
	}
	if plan.NotionalUSD > ceiling/2+0.01 {
		t.Errorf("added $%.2f on top of a $%.2f book against a $%.2f ceiling",
			plan.NotionalUSD, ceiling/2, ceiling)
	}

	// A book AT the ceiling adds nothing at all.
	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, stop, entry*1.007, 1, ceiling); !errors.Is(err, ErrAggregateExposureReached) {
		t.Fatalf("at the ceiling the planner gave %v, want ErrAggregateExposureReached", err)
	}
}

// The whole book must fit inside the account with room for the options engine,
// which shares this wallet.
func TestDefaultPerpRiskConfig_BookFitsTheAccount(t *testing.T) {
	cfg := DefaultPerpRiskConfig(100)
	if cfg.MaxAggregateLeverage > cfg.MaxLeverage {
		t.Errorf("aggregate cap %.0fx exceeds the per-order cap %.0fx, so it never binds",
			cfg.MaxAggregateLeverage, cfg.MaxLeverage)
	}
	// Margin the full book would consume at the venue leverage actually sent.
	if cfg.LeverageForOrder <= 0 {
		t.Fatal("no explicit order leverage: margin per position would be whatever the account is set to")
	}
	marginPct := cfg.MaxAggregateLeverage / float64(cfg.LeverageForOrder) * 100
	if marginPct > 50 {
		t.Errorf("a full book consumes %.0f%% of equity as margin, leaving too little for the options engine that shares this wallet", marginPct)
	}
}

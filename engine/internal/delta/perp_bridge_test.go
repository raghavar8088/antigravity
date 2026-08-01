package delta

import (
	"context"
	"testing"
	"time"
)

// This bridge owns real money on a $100 account. Every test here is about a way
// it could lose that account to a mistake rather than to a strategy.

func bridgeFixture(t *testing.T) *PerpBridge {
	t.Helper()
	reg := registryFrom(t, realPerpTickers)
	b := NewPerpBridge(&Client{}, reg, 100)
	b.AllowList().Set(ScalpLiveStrategies(), []string{"ADAUSD", "BNBUSD"})
	return b
}

// Disarmed is the only safe default. A bridge that trades before a human arms it
// is a bridge that trades during a deploy, a restart, or a test.
func TestPerpBridge_StartsDisarmed(t *testing.T) {
	b := bridgeFixture(t)
	if b.IsArmed() {
		t.Fatal("bridge armed itself")
	}
	got := b.OnPaperOpen(context.Background(), "ANTI_M1_DoubleTop_10bp_Short", "ADAUSD", true, 0.17, 0.1694, 0.1712, time.Hour)
	if got != nil {
		t.Fatal("a disarmed bridge placed an order")
	}
}

// Arming must refuse when it would be meaningless or unsafe.
func TestPerpBridge_ArmRefusesWhenItCannotTradeSafely(t *testing.T) {
	reg := registryFrom(t, realPerpTickers)

	if err := NewPerpBridge(nil, reg, 100).Arm("me", "test"); err == nil {
		t.Error("armed with no Delta client")
	}
	if err := NewPerpBridge(&Client{}, reg, 100).Arm("me", "test"); err == nil {
		t.Error("armed with an empty allow-list — it would permit nothing")
	}
	empty := NewPerpBridge(&Client{}, NewPerpRegistry(), 100)
	empty.AllowList().Set(ScalpLiveStrategies(), nil)
	if err := empty.Arm("me", "test"); err == nil {
		t.Error("armed with an empty product registry — it could not size an order")
	}
}

// Only the selected streams may trade, even once armed.
func TestPerpBridge_OnlyAllowListedStreamsTrade(t *testing.T) {
	b := bridgeFixture(t)
	if err := b.Arm("test", "unit"); err != nil {
		t.Fatalf("Arm: %v", err)
	}
	ctx := context.Background()

	// Not on the list.
	if b.OnPaperOpen(ctx, "Some_Unlisted_Strategy", "ADAUSD", true, 0.17, 0.1694, 0.1712, time.Hour) != nil {
		t.Error("an unlisted strategy placed an order")
	}
	// On the list but a symbol nobody selected.
	if b.OnPaperOpen(ctx, "ANTI_M1_DoubleTop_10bp_Short", "BTCUSD", true, 63000, 62779, 63441, time.Hour) != nil {
		t.Error("a listed strategy traded a symbol that was never selected")
	}
	// The ORIGINAL of a selected mirror is a different, opposite bet.
	if b.OnPaperOpen(ctx, "M1_DoubleTop_10bp_Short", "ADAUSD", true, 0.17, 0.1694, 0.1712, time.Hour) != nil {
		t.Error("the original traded when only its ANTI_ mirror was selected")
	}
}

// The kill switch must block new exposure while it is active.
func TestPerpBridge_KillSwitchBlocksNewOrders(t *testing.T) {
	b := bridgeFixture(t)
	_ = b.Arm("test", "unit")
	active := true
	b.SetKillSwitch(func() bool { return active })

	if b.OnPaperOpen(context.Background(), "ANTI_M1_DoubleTop_10bp_Short", "ADAUSD", true, 0.17, 0.1694, 0.1712, time.Hour) != nil {
		t.Fatal("an order was placed with the kill switch active")
	}
}

// CUSTODY. A long stops out when the mark falls to its stop and targets when it
// rises to its target; a short is the mirror image. Getting this backwards
// closes winners and rides losers.
func TestPerpExitReason_LongAndShort(t *testing.T) {
	long := &PerpLiveTrade{Side: "buy", EntryPrice: 100, StopPrice: 99, TargetPrice: 102}
	for mark, want := range map[float64]string{98: "SL", 99: "SL", 100: "", 101: "", 102: "TP", 103: "TP"} {
		if got := perpExitReason(long, mark, time.Now()); got != want {
			t.Errorf("long at mark %.0f gave %q, want %q", mark, got, want)
		}
	}
	short := &PerpLiveTrade{Side: "sell", EntryPrice: 100, StopPrice: 101, TargetPrice: 98}
	for mark, want := range map[float64]string{102: "SL", 101: "SL", 100: "", 99: "", 98: "TP", 97: "TP"} {
		if got := perpExitReason(short, mark, time.Now()); got != want {
			t.Errorf("short at mark %.0f gave %q, want %q", mark, got, want)
		}
	}
}

// The bridge checks a single MARK price, not a bar's high/low range. For a
// well-formed position the stop and target therefore cannot both fire: a long
// needs mark <= stop < entry < target <= mark, which is unsatisfiable.
//
// That is a stronger guarantee than the paper desk has — it scans a bar range
// and must apply stop-first precedence to resolve genuine ambiguity — and it is
// worth pinning, because it is the reason no precedence rule is needed here.
func TestPerpExitReason_LevelsAreMutuallyExclusiveOnASingleMark(t *testing.T) {
	long := &PerpLiveTrade{Side: "buy", EntryPrice: 100, StopPrice: 99, TargetPrice: 102}
	short := &PerpLiveTrade{Side: "sell", EntryPrice: 100, StopPrice: 101, TargetPrice: 98}

	for mark := 90.0; mark <= 110.0; mark += 0.25 {
		for _, tr := range []*PerpLiveTrade{long, short} {
			hitStop := (tr.long() && mark <= tr.StopPrice) || (!tr.long() && mark >= tr.StopPrice)
			hitTarget := (tr.long() && mark >= tr.TargetPrice) || (!tr.long() && mark <= tr.TargetPrice)
			if hitStop && hitTarget {
				t.Fatalf("%s at mark %.2f satisfies BOTH stop and target — the levels are misordered", tr.Side, mark)
			}
			reason := perpExitReason(tr, mark, time.Now())
			switch {
			case hitStop && reason != "SL":
				t.Errorf("%s at %.2f should stop out, got %q", tr.Side, mark, reason)
			case hitTarget && reason != "TP":
				t.Errorf("%s at %.2f should target, got %q", tr.Side, mark, reason)
			case !hitStop && !hitTarget && reason != "":
				t.Errorf("%s at %.2f exited with %q but neither level was reached", tr.Side, mark, reason)
			}
		}
	}
}

// And a misordered position cannot be created in the first place: the planner
// refuses a stop on the wrong side of entry, which is what would make the two
// levels overlap.
func TestPerpExitReason_MisorderedPositionsCannotBePlanned(t *testing.T) {
	reg := registryFrom(t, realPerpTickers)
	cfg := DefaultPerpRiskConfig(100)
	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, 0.17, 0.18, 0.175, 0, 0); err == nil {
		t.Fatal("a long with its stop above entry was planned; its stop and target would overlap")
	}
}

// A position the venue no longer reports was closed by something other than this
// bridge — liquidation, a manual close, an exchange action. It must be
// reconciled, not managed as a ghost forever.
func TestPerpBridge_ReconcilesAPositionThatVanishedFromTheVenue(t *testing.T) {
	b := bridgeFixture(t)
	tr := &PerpLiveTrade{
		Strategy: "ANTI_M1_DoubleTop_10bp_Short", Symbol: "ADAUSD", ProductID: 16,
		Side: "buy", Contracts: 100, EntryPrice: 0.17, StopPrice: 0.169, TargetPrice: 0.172,
		Status: "OPEN",
	}
	b.open[perpKey(tr.Strategy, tr.Symbol)] = tr

	b.finish(tr, tr.EntryPrice, "CLOSED_EXTERNALLY")

	if len(b.open) != 0 {
		t.Error("the vanished position is still tracked as open")
	}
	h := b.History()
	if len(h) != 1 || h[0].ExitReason != "CLOSED_EXTERNALLY" {
		t.Fatalf("history = %+v", h)
	}
}

// Realised P&L must use the SYMBOL's contract value, not BTC's. On ADAUSD
// (contract value 1.0) a BTC assumption would understate P&L a thousandfold.
func TestPerpBridge_RealisedPnLUsesTheSymbolsContractValue(t *testing.T) {
	b := bridgeFixture(t)
	tr := &PerpLiveTrade{
		Strategy: "ANTI_M1_DoubleTop_10bp_Short", Symbol: "ADAUSD", ProductID: 16,
		Side: "buy", Contracts: 1000, EntryPrice: 0.17, Status: "OPEN",
	}
	b.open[perpKey(tr.Strategy, tr.Symbol)] = tr

	b.finish(tr, 0.18, "TP") // +0.01 x 1000 contracts x 1.0 ADA = $10

	h := b.History()
	if len(h) != 1 {
		t.Fatalf("history has %d entries", len(h))
	}
	// GROSS is the contract-value check: +0.01 x 1000 x 1.0 ADA = $10.
	// A BTC contract value would give $0.01 — the thousandfold sizing trap.
	if got := h[0].GrossPnL; got < 9.99 || got > 10.01 {
		t.Errorf("gross $%.4f, want ~$10 — a 0.001 contract value would give $0.01", got)
	}
	// REALISED is net of both fee legs and must therefore be strictly smaller.
	// It reporting exactly gross is what hid $1.9086 of fees in one day.
	if h[0].FeesUSD <= 0 {
		t.Error("no fees booked on a closed trade")
	}
	if h[0].RealisedPnL >= h[0].GrossPnL {
		t.Errorf("realised $%.4f is not below gross $%.4f; fees were not subtracted",
			h[0].RealisedPnL, h[0].GrossPnL)
	}
}

// A short's P&L must have the opposite sign.
func TestPerpBridge_ShortPnLIsSignedCorrectly(t *testing.T) {
	b := bridgeFixture(t)
	tr := &PerpLiveTrade{
		Strategy: "M1X_Squeeze_Break_Short", Symbol: "BNBUSD", ProductID: 21,
		Side: "sell", Contracts: 10, EntryPrice: 580, Status: "OPEN",
	}
	b.open[perpKey(tr.Strategy, tr.Symbol)] = tr

	b.finish(tr, 570, "TP") // short gains when price falls

	if got := b.History()[0].RealisedPnL; got <= 0 {
		t.Errorf("a short that fell 10 booked $%.4f; it should be a gain", got)
	}
}

// One live position per stream, exactly as the paper desk holds one. Stacking
// would give the live account leverage the paper record never had.
func TestPerpBridge_OnePositionPerStream(t *testing.T) {
	b := bridgeFixture(t)
	_ = b.Arm("test", "unit")
	b.open[perpKey("ANTI_M1_DoubleTop_10bp_Short", "ADAUSD")] = &PerpLiveTrade{Status: "OPEN"}

	if b.OnPaperOpen(context.Background(), "ANTI_M1_DoubleTop_10bp_Short", "ADAUSD", true, 0.17, 0.1694, 0.1712, time.Hour) != nil {
		t.Fatal("a second position was opened on a stream that already holds one")
	}
}

// Disarming must NOT orphan funded positions — the monitor keeps owning them.
func TestPerpBridge_DisarmKeepsOpenPositionsUnderCustody(t *testing.T) {
	b := bridgeFixture(t)
	_ = b.Arm("test", "unit")
	b.open[perpKey("ANTI_M1_DoubleTop_10bp_Short", "ADAUSD")] = &PerpLiveTrade{
		Strategy: "ANTI_M1_DoubleTop_10bp_Short", Symbol: "ADAUSD", Status: "OPEN",
	}

	b.Disarm("test", "unit")
	if b.IsArmed() {
		t.Fatal("still armed")
	}
	if len(b.Stats().OpenPositions) != 1 {
		t.Error("disarming dropped an open position; a funded position must stay under custody")
	}
}

// Stats must report the posture an operator needs to sanity-check the account.
func TestPerpBridge_StatsReportTheRiskPosture(t *testing.T) {
	b := bridgeFixture(t)
	s := b.Stats()
	if s.EquityUSD != 100 {
		t.Errorf("equity $%.2f", s.EquityUSD)
	}
	if s.RiskPerTrade > 2.01 {
		t.Errorf("risk per trade $%.2f on a $100 account", s.RiskPerTrade)
	}
	// The roster grows as the owner selects more; what matters is that Stats
	// reports exactly what ScalpLiveStrategies ships, with nothing lost or
	// invented between the two.
	if len(s.Strategies) != len(ScalpLiveStrategies()) {
		t.Errorf("Stats reports %d strategies, allow-list ships %d", len(s.Strategies), len(ScalpLiveStrategies()))
	}
	if s.Armed {
		t.Error("stats report armed on a fresh bridge")
	}
	if s.ProductsKnown == 0 {
		t.Error("no products known")
	}
}

func TestPerpBridge_SetEquityRebasesRisk(t *testing.T) {
	b := bridgeFixture(t)
	b.SetEquity(250)
	if got := b.Config().EquityUSD; got != 250 {
		t.Errorf("equity %v", got)
	}
	if got := b.Stats().RiskPerTrade; got < 4.9 || got > 5.1 {
		t.Errorf("risk per trade $%.2f on $250, want ~$5", got)
	}
}

func TestPerpBridge_MonitorStopsWithContext(t *testing.T) {
	b := bridgeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Monitor(ctx, 10*time.Millisecond); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Monitor did not stop when its context was cancelled")
	}
}

// THE TIME STOP. Over 500 measured paper trades the scalp desk exited 456 of
// them (91.2%) on time, 30 on the stop and 14 on the target. A live bridge with
// only a stop and a target reproduces under 9% of the desk's behaviour and holds
// the other 91% indefinitely — positions the paper record shows as closed hours
// earlier, still open and still funded.
func TestPerpExitReason_ClosesOnTheTimeStop(t *testing.T) {
	opened := time.Now().UTC().Add(-90 * time.Minute)
	tr := &PerpLiveTrade{
		Side: "buy", EntryPrice: 100, StopPrice: 99, TargetPrice: 102,
		OpenedAt: opened, ExpiresAt: opened.Add(60 * time.Minute),
	}
	// Price is between the levels: without a time stop this holds forever.
	if got := perpExitReason(tr, 100.5, opened.Add(59*time.Minute)); got != "" {
		t.Errorf("exited %q before the TTL elapsed", got)
	}
	if got := perpExitReason(tr, 100.5, opened.Add(61*time.Minute)); got != "TTL" {
		t.Fatalf("past its TTL the position gave %q, want TTL — it would otherwise hold indefinitely", got)
	}
}

// A price exit must win over the time stop when both are true on one tick: the
// position exited because it reached a real level, not because the clock ran out.
func TestPerpExitReason_PriceExitBeatsTheTimeStop(t *testing.T) {
	opened := time.Now().UTC().Add(-2 * time.Hour)
	tr := &PerpLiveTrade{
		Side: "buy", EntryPrice: 100, StopPrice: 99, TargetPrice: 102,
		OpenedAt: opened, ExpiresAt: opened.Add(time.Minute),
	}
	if got := perpExitReason(tr, 98, time.Now()); got != "SL" {
		t.Errorf("stopped-out AND expired gave %q, want SL", got)
	}
	if got := perpExitReason(tr, 103, time.Now()); got != "TP" {
		t.Errorf("targeted AND expired gave %q, want TP", got)
	}
}

// A trade with no TTL set must never expire spuriously.
func TestPerpExitReason_NoTTLMeansNoTimeExit(t *testing.T) {
	tr := &PerpLiveTrade{Side: "buy", EntryPrice: 100, StopPrice: 99, TargetPrice: 102}
	if got := perpExitReason(tr, 100.5, time.Now().Add(1000*time.Hour)); got != "" {
		t.Errorf("a trade with no ExpiresAt exited %q", got)
	}
}

// The custody loop already reads the venue's marks to decide exits. Publishing
// them is what lets the page show a real number instead of a placeholder — and
// it must be the SAME figure custody acts on, or the screen and the risk engine
// disagree about the position.
func TestPerpBridge_PublishesMarkAndUnrealisedPnL(t *testing.T) {
	b := bridgeFixture(t)
	tr := &PerpLiveTrade{
		Strategy: "ANTI_M1_VWAP_Doji_Short", Symbol: "ADAUSD", ProductID: 16614,
		Side: "buy", Contracts: 1000, EntryPrice: 0.17, Status: "OPEN",
	}
	b.open[perpKey(tr.Strategy, tr.Symbol)] = tr

	b.markLive(tr, 0.18)

	if tr.MarkPrice != 0.18 {
		t.Errorf("mark %v, want 0.18", tr.MarkPrice)
	}
	// +0.01 x 1000 contracts x 1.0 ADA = $10. A BTC contract value would give
	// $0.01 — the same thousandfold trap as sizing. Unrealised is quoted GROSS,
	// since the exit fee is not owed until the position is actually closed.
	if tr.UnrealizedPnL < 9.99 || tr.UnrealizedPnL > 10.01 {
		t.Errorf("unrealised $%.4f, want ~$10", tr.UnrealizedPnL)
	}
}

// A short's unrealised P&L must have the opposite sign, or the page reports a
// winning position as losing.
func TestPerpBridge_UnrealisedPnLIsSignedBySide(t *testing.T) {
	b := bridgeFixture(t)
	tr := &PerpLiveTrade{
		Strategy: "M1X_Squeeze_Break_Short", Symbol: "BNBUSD", ProductID: 15042,
		Side: "sell", Contracts: 10, EntryPrice: 580, Status: "OPEN",
	}
	b.open[perpKey(tr.Strategy, tr.Symbol)] = tr

	b.markLive(tr, 570) // a short gains when price falls
	if tr.UnrealizedPnL <= 0 {
		t.Errorf("a short that fell 10 shows $%.4f; it should be a gain", tr.UnrealizedPnL)
	}
	b.markLive(tr, 590)
	if tr.UnrealizedPnL >= 0 {
		t.Errorf("a short that rose 10 shows $%.4f; it should be a loss", tr.UnrealizedPnL)
	}
}

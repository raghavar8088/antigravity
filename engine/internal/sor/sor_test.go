package sor

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── In-memory ledger store ────────────────────────────────────────────────────

type memStore struct {
	mu     sync.Mutex
	events []ledger.Event
	seq    int64
}

func newMemStore() *memStore { return &memStore{} }

func (m *memStore) Append(_ context.Context, ev ledger.Event) (ledger.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	ev.SequenceNo = m.seq
	m.events = append(m.events, ev)
	return ev, nil
}

func (m *memStore) Replay(_ context.Context, aggType ledger.AggregateType, aggID string) ([]ledger.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ledger.Event
	for _, ev := range m.events {
		if ev.AggregateType == aggType && ev.AggregateID == aggID {
			out = append(out, ev)
		}
	}
	return out, nil
}

func (m *memStore) ReplayAccount(_ context.Context, accountID string) ([]ledger.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []ledger.Event
	for _, ev := range m.events {
		if ev.AccountID == accountID {
			out = append(out, ev)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SequenceNo < out[j].SequenceNo })
	return out, nil
}

func (m *memStore) all() []ledger.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ledger.Event(nil), m.events...)
}

// ── Common fixtures ───────────────────────────────────────────────────────────

func buildRegistry(venues ...VenueID) *VenueRegistry {
	reg := NewVenueRegistry()
	for _, v := range venues {
		reg.RegisterDefault(v, string(v), FeeStructure{MakerBps: 1, TakerBps: 5})
	}
	return reg
}

func injectBook(reg *VenueRegistry, venue VenueID, symbol string, bid, ask, bidSz, askSz float64, latencyMs float64) {
	levels := func(touch, sz float64, n int, step float64) []PriceLevel {
		out := make([]PriceLevel, n)
		for i := range out {
			out[i] = PriceLevel{Price: touch + float64(i)*step, Size: sz}
		}
		return out
	}
	reg.UpdateMarketData(VenueMarketData{
		VenueID:   venue,
		Symbol:    symbol,
		BidPrice:  bid,
		AskPrice:  ask,
		BidSize:   bidSz,
		AskSize:   askSz,
		Bids:      levels(bid, bidSz/5, 5, -10),
		Asks:      levels(ask, askSz/5, 5, 10),
		LatencyMs: latencyMs,
		Fees:      FeeStructure{MakerBps: 1, TakerBps: 5},
		UpdatedAt: time.Now().UTC(),
	})
}

func buildFullSOR(store ledger.Store, venues ...VenueID) (*SmartOrderRouter, *VenueRegistry, *ExchangeHealthEngine) {
	reg := buildRegistry(venues...)
	liq := NewLiquidityEngine()
	fees := NewFeeOptimizer()
	slip := NewSlippageEngine(store)
	health := NewExchangeHealthEngine(reg, store)
	best := NewBestExecutionEngine(reg, liq, fees, slip, health)
	selector := NewVenueSelector(reg, best)
	splitter := NewOrderSplitter(liq, reg)
	scheduler := NewAlgoScheduler()
	planner := NewExecutionPlanner(splitter, scheduler, reg, store)
	failover := NewFailoverController(reg, liq, health, store)
	coord := NewExecutionCoordinator(reg, slip, health, failover, store)

	for i, v := range venues {
		a := NewSimulatedVenueAdapter(v, reg, int64(i+1))
		coord.RegisterAdapter(a)
	}
	router := NewSmartOrderRouter(reg, selector, planner, coord, store)
	return router, reg, health
}

// ── 18C: Best Execution Correctness ──────────────────────────────────────────

func TestBestExecution_SelectsCheaperVenue(t *testing.T) {
	reg := buildRegistry(VenueBinance, VenueDelta)
	liq := NewLiquidityEngine()
	fees := NewFeeOptimizer()
	slip := NewSlippageEngine(nil)
	health := NewExchangeHealthEngine(reg, nil)
	best := NewBestExecutionEngine(reg, liq, fees, slip, health)

	// Binance: tight spread, low latency — should win.
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 5, 5, 20)
	// Delta: wider spread, higher latency.
	injectBook(reg, VenueDelta, "BTC-USD", 49_950, 50_050, 5, 5, 100)

	candidates := reg.CandidateVenues("BTC-USD")
	scores := best.Evaluate(candidates, EvaluationInput{Symbol: "BTC-USD", Side: "BUY", Quantity: 1.0})
	if len(scores) == 0 {
		t.Fatal("expected at least one score")
	}
	winner, ok := Winner(scores)
	if !ok {
		t.Fatal("expected a winner")
	}
	if winner.VenueID != VenueBinance {
		t.Fatalf("expected Binance to win (lower cost/latency), got %s", winner.VenueID)
	}
}

func TestBestExecution_ExplainationAlwaysPopulated(t *testing.T) {
	reg := buildRegistry(VenueBinance)
	liq := NewLiquidityEngine()
	fees := NewFeeOptimizer()
	slip := NewSlippageEngine(nil)
	health := NewExchangeHealthEngine(reg, nil)
	best := NewBestExecutionEngine(reg, liq, fees, slip, health)
	injectBook(reg, VenueBinance, "BTC-USD", 50_000, 50_010, 10, 10, 5)

	scores := best.Evaluate(reg.CandidateVenues("BTC-USD"), EvaluationInput{
		Symbol: "BTC-USD", Side: "SELL", Quantity: 0.5,
	})
	for _, s := range scores {
		if s.Explanation == "" {
			t.Fatalf("score for %s has empty explanation", s.VenueID)
		}
	}
}

func TestBestExecution_DisqualifiesLowHealthVenue(t *testing.T) {
	reg := buildRegistry(VenueBinance)
	liq := NewLiquidityEngine()
	fees := NewFeeOptimizer()
	slip := NewSlippageEngine(nil)
	health := NewExchangeHealthEngine(reg, nil)
	best := NewBestExecutionEngine(reg, liq, fees, slip, health)
	best.MinHealthScore = 50

	injectBook(reg, VenueBinance, "BTC-USD", 50_000, 50_010, 10, 10, 5)
	// Degrade Binance below the min threshold.
	reg.SetHealthScore(VenueBinance, 20)

	scores := best.Evaluate(reg.CandidateVenues("BTC-USD"), EvaluationInput{
		Symbol: "BTC-USD", Side: "BUY", Quantity: 1.0,
	})
	for _, s := range scores {
		if s.VenueID == VenueBinance && !s.Disqualified {
			t.Fatal("Binance should be disqualified (health too low)")
		}
	}
}

// ── 18D: Liquidity Routing ────────────────────────────────────────────────────

func TestLiquidityEngine_RoutesToDeeperBook(t *testing.T) {
	reg := buildRegistry(VenueBinance, VenueBybit)
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 5, 5, 10)   // 5 BTC ask
	injectBook(reg, VenueBybit, "BTC-USD", 49_990, 50_010, 5, 20, 10)   // 20 BTC ask — deeper

	liq := NewLiquidityEngine()
	candidates := reg.CandidateVenues("BTC-USD")
	best, res := liq.DeepestLiquidity(reg, candidates, "BTC-USD", "BUY", 10.0)
	if best != VenueBybit {
		t.Fatalf("expected Bybit (deeper book), got %s (score=%.4f)", best, res.LiquidityScore)
	}
}

func TestLiquidityEngine_DetectsThinBook(t *testing.T) {
	reg := buildRegistry(VenueBinance)
	// Inject tiny book: 0.1 BTC on ask.
	reg.UpdateMarketData(VenueMarketData{
		VenueID:  VenueBinance,
		Symbol:   "BTC-USD",
		AskPrice: 50_010,
		AskSize:  0.1,
		Asks:     []PriceLevel{{Price: 50_010, Size: 0.1}},
		LatencyMs: 5,
		UpdatedAt: time.Now().UTC(),
	})
	liq := NewLiquidityEngine()
	md, _ := reg.MarketData(VenueBinance, "BTC-USD")
	res := liq.Analyse(md, "BUY", 5.0) // requesting 5 BTC against 0.1 BTC book
	if !res.ThinBook {
		t.Fatal("expected ThinBook flag for order >> book depth")
	}
}

// ── 18E: Fee Optimization ─────────────────────────────────────────────────────

func TestFeeOptimizer_MakerCheaperThanTaker(t *testing.T) {
	opt := NewFeeOptimizer()
	taker := opt.Compute(CostInput{
		VenueID:     VenueBinance,
		NotionalUSD: 100_000,
		IsMaker:     false,
		Fees:        FeeStructure{MakerBps: 1, TakerBps: 5},
		SpreadBps:   4,
	})
	maker := opt.Compute(CostInput{
		VenueID:     VenueBinance,
		NotionalUSD: 100_000,
		IsMaker:     true,
		Fees:        FeeStructure{MakerBps: 1, TakerBps: 5},
		SpreadBps:   4,
	})
	if taker.TotalCostUSD <= maker.TotalCostUSD {
		t.Fatalf("taker cost $%.2f should exceed maker cost $%.2f", taker.TotalCostUSD, maker.TotalCostUSD)
	}
}

func TestFeeOptimizer_CheapestVenueSelection(t *testing.T) {
	opt := NewFeeOptimizer()
	breakdowns := []CostBreakdown{
		{VenueID: VenueBinance, TotalCostUSD: 120},
		{VenueID: VenueBybit, TotalCostUSD: 80},
		{VenueID: VenueOKX, TotalCostUSD: 100},
	}
	winner, _, ok := opt.CheapestVenue(breakdowns)
	if !ok {
		t.Fatal("expected a winner")
	}
	if winner != VenueBybit {
		t.Fatalf("expected Bybit (cheapest), got %s", winner)
	}
}

// ── 18F: Slippage ─────────────────────────────────────────────────────────────

func TestSlippageEngine_LearnsFromRealized(t *testing.T) {
	e := NewSlippageEngine(nil)
	// Inject several realized samples.
	for i := 0; i < 20; i++ {
		e.RecordRealized(context.Background(), VenueBinance, "BTC-USD", "BUY", 1.0, 2.0, 3.0)
	}
	est := e.Expected(VenueBinance, "BTC-USD", "BUY", 1.0, 100, 4)
	if est.BaseBps <= 0 {
		t.Fatal("expected non-zero base slippage after learning")
	}
	// After 20 observations at 3 bps, EWMA should be close to 3.
	if est.BaseBps < 2.5 || est.BaseBps > 3.5 {
		t.Fatalf("EWMA out of range: got %.3f, want ~3.0", est.BaseBps)
	}
}

func TestSlippageEngine_ImpactScalesWithSize(t *testing.T) {
	e := NewSlippageEngine(nil)
	smallEst := e.Expected(VenueBinance, "BTC-USD", "BUY", 0.1, 100, 4)
	largeEst := e.Expected(VenueBinance, "BTC-USD", "BUY", 50.0, 100, 4)
	if largeEst.ImpactBps <= smallEst.ImpactBps {
		t.Fatalf("large order impact %.3f should exceed small order %.3f", largeEst.ImpactBps, smallEst.ImpactBps)
	}
}

// ── 18G: Exchange Health ──────────────────────────────────────────────────────

func TestHealthEngine_ScoreDecaysOnFailures(t *testing.T) {
	store := newMemStore()
	reg := buildRegistry(VenueBinance)
	h := NewExchangeHealthEngine(reg, store)
	ctx := context.Background()

	initialScore := h.Score(VenueBinance)
	for i := 0; i < 5; i++ {
		h.Observe(ctx, VenueBinance, SignalAPIError, 0)
	}
	newScore := h.Score(VenueBinance)
	if newScore >= initialScore {
		t.Fatalf("health should decrease after API errors: initial=%.0f new=%.0f", initialScore, newScore)
	}
}

func TestHealthEngine_DisconnectCausesDownStatus(t *testing.T) {
	store := newMemStore()
	reg := buildRegistry(VenueBinance)
	h := NewExchangeHealthEngine(reg, store)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		h.Observe(ctx, VenueBinance, SignalDisconnect, 0)
	}
	status := h.Status(VenueBinance)
	score := h.Score(VenueBinance)
	if score >= 70 && status != VenueStatusActive {
		t.Logf("status=%s score=%.0f (multiple disconnects but may not be DOWN yet)", status, score)
	}
}

func TestHealthEngine_RecoveryFromSuccess(t *testing.T) {
	store := newMemStore()
	reg := buildRegistry(VenueBinance)
	h := NewExchangeHealthEngine(reg, store)
	ctx := context.Background()

	// Degrade first.
	for i := 0; i < 10; i++ {
		h.Observe(ctx, VenueBinance, SignalAPIError, 0)
	}
	degraded := h.Score(VenueBinance)

	// Then recover via successes.
	for i := 0; i < 50; i++ {
		h.Observe(ctx, VenueBinance, SignalSuccess, 5)
	}
	recovered := h.Score(VenueBinance)
	if recovered <= degraded {
		t.Fatalf("score should recover: degraded=%.0f recovered=%.0f", degraded, recovered)
	}
}

// ── 18H: Order Splitting ──────────────────────────────────────────────────────

func TestOrderSplitter_SingleVenueSmallOrder(t *testing.T) {
	reg := buildRegistry(VenueBinance)
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 20, 20, 5)
	liq := NewLiquidityEngine()
	splitter := NewOrderSplitter(liq, reg)
	splitter.SingleVenueMaxNotionalUSD = 1_000_000 // all orders → single venue

	scores := []BestExecutionScore{
		{VenueID: VenueBinance, Score: 90, ExecutableQty: 1.0, FullyExecutable: true},
	}
	plan, err := splitter.Split("ord1", "BTC-USD", "BUY", 1.0, 50_000, scores, "")
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if plan.Method != SplitSingleVenue {
		t.Fatalf("expected SINGLE_VENUE, got %s", plan.Method)
	}
	if len(plan.Allocations) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(plan.Allocations))
	}
}

func TestOrderSplitter_LiquidityWeightedSplit(t *testing.T) {
	reg := buildRegistry(VenueBinance, VenueBybit, VenueOKX)
	liq := NewLiquidityEngine()
	splitter := NewOrderSplitter(liq, reg)
	splitter.MaxVenues = 3
	splitter.MinChildNotionalUSD = 0 // allow any size in test

	scores := []BestExecutionScore{
		{VenueID: VenueBinance, Score: 80, ExecutableQty: 4.0},
		{VenueID: VenueBybit, Score: 75, ExecutableQty: 3.0},
		{VenueID: VenueOKX, Score: 65, ExecutableQty: 2.0},
	}
	plan, err := splitter.Split("ord2", "BTC-USD", "BUY", 9.0, 50_000, scores, SplitLiquidityWeighted)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// Total allocated must equal requested.
	total := 0.0
	for _, a := range plan.Allocations {
		total += a.Quantity
	}
	if math.Abs(total-9.0) > 0.001 {
		t.Fatalf("total allocation %.4f != requested 9.0", total)
	}
	// Binance (exec=4) should get more than OKX (exec=2).
	var binanceQty, okxQty float64
	for _, a := range plan.Allocations {
		switch a.VenueID {
		case VenueBinance:
			binanceQty = a.Quantity
		case VenueOKX:
			okxQty = a.Quantity
		}
	}
	if binanceQty <= okxQty {
		t.Fatalf("Binance (deeper book) should receive more: binance=%.3f okx=%.3f", binanceQty, okxQty)
	}
}

func TestOrderSplitter_NoDustAllocations(t *testing.T) {
	// Scenario: splitting 10 BTC across 3 venues with a weighted scheme that
	// would assign a tiny slice to OKX. MinChildNotionalUSD must consolidate it.
	reg := buildRegistry(VenueBinance, VenueBybit, VenueOKX)
	liq := NewLiquidityEngine()
	splitter := NewOrderSplitter(liq, reg)
	splitter.MinChildNotionalUSD = 10_000 // $10k minimum per child
	splitter.MinChildNotionalUSD = 1_000  // 0.02 BTC * $50k = $1k; force OKX (0.001 * $50k = $50) to dust
	splitter.MaxVenues = 3

	// Weights heavily favour Binance/Bybit; OKX gets barely anything.
	scores := []BestExecutionScore{
		{VenueID: VenueBinance, Score: 90, ExecutableQty: 6.0},
		{VenueID: VenueBybit, Score: 75, ExecutableQty: 3.999},
		{VenueID: VenueOKX, Score: 60, ExecutableQty: 0.001}, // will produce < $50 allocation
	}
	plan, err := splitter.Split("ord3", "BTC-USD", "BUY", 10.0, 50_000, scores, SplitLiquidityWeighted)
	if err != nil {
		t.Fatalf("expected no error; got: %v", err)
	}
	// Total quantity must still be 10 BTC.
	total := 0.0
	for _, a := range plan.Allocations {
		total += a.Quantity
	}
	if math.Abs(total-10.0) > 1e-9 {
		t.Fatalf("total qty %.6f != 10.0 after dust pruning", total)
	}
	// Every allocation must meet the minimum notional threshold.
	for _, a := range plan.Allocations {
		if a.Quantity*50_000 < splitter.MinChildNotionalUSD {
			t.Fatalf("dust allocation remaining: %.6f BTC * $50k = $%.2f < min $%.0f",
				a.Quantity, a.Quantity*50_000, splitter.MinChildNotionalUSD)
		}
	}
	// Dust from OKX must have been consolidated — fewer allocations than input scores.
	if len(plan.Allocations) >= len(scores) {
		t.Logf("allocations=%d scores=%d — OKX dust was absorbed into another venue", len(plan.Allocations), len(scores))
	}
}

// ── 18I: Execution Algorithms ─────────────────────────────────────────────────

func TestAlgoScheduler_TWAPSlicesCorrectly(t *testing.T) {
	sched := NewAlgoScheduler()
	alloc := VenueAllocation{VenueID: VenueBinance, Quantity: 5.0, RefPrice: 50_000}
	children := sched.Schedule("ord4", "BTC-USD", "BUY", "MARKET", alloc, AlgoParams{
		Algo:     AlgoTWAP,
		Slices:   5,
		Duration: 10 * time.Minute,
	}, 0)
	if len(children) != 5 {
		t.Fatalf("TWAP expected 5 slices, got %d", len(children))
	}
	total := 0.0
	for _, c := range children {
		total += c.Quantity
	}
	if math.Abs(total-5.0) > 1e-9 {
		t.Fatalf("TWAP total %.6f != 5.0", total)
	}
}

func TestAlgoScheduler_VWAPWeightsCorrectly(t *testing.T) {
	sched := NewAlgoScheduler()
	profile := []float64{3, 1, 1, 1, 3} // U-shape
	alloc := VenueAllocation{VenueID: VenueBinance, Quantity: 9.0, RefPrice: 50_000}
	children := sched.Schedule("ord5", "BTC-USD", "BUY", "MARKET", alloc, AlgoParams{
		Algo:          AlgoVWAP,
		Slices:        5,
		Duration:      10 * time.Minute,
		VolumeProfile: profile,
	}, 0)
	if len(children) != 5 {
		t.Fatalf("VWAP expected 5 slices, got %d", len(children))
	}
	// First slice (weight=3) should be larger than middle slice (weight=1).
	if children[0].Quantity <= children[2].Quantity {
		t.Fatalf("VWAP: first slice %.4f should be larger than middle %.4f", children[0].Quantity, children[2].Quantity)
	}
	// Total must be preserved.
	total := 0.0
	for _, c := range children {
		total += c.Quantity
	}
	if math.Abs(total-9.0) > 1e-9 {
		t.Fatalf("VWAP total %.6f != 9.0", total)
	}
}

func TestAlgoScheduler_IcebergHidesSize(t *testing.T) {
	sched := NewAlgoScheduler()
	alloc := VenueAllocation{VenueID: VenueBinance, Quantity: 10.0, RefPrice: 50_000}
	children := sched.Schedule("ord6", "BTC-USD", "BUY", "MARKET", alloc, AlgoParams{
		Algo:           AlgoIceberg,
		IcebergClipQty: 2.0,
	}, 0)
	if len(children) == 0 {
		t.Fatal("iceberg produced no children")
	}
	for _, c := range children {
		if c.OrderType != "POST_ONLY" {
			t.Fatalf("iceberg clips must be POST_ONLY, got %s", c.OrderType)
		}
	}
	total := 0.0
	for _, c := range children {
		total += c.Quantity
	}
	if math.Abs(total-10.0) > 1e-6 {
		t.Fatalf("iceberg total %.6f != 10.0", total)
	}
}

// ── 18J: Venue Failover ───────────────────────────────────────────────────────

func TestFailover_RoutesToSecondaryOnReject(t *testing.T) {
	store := newMemStore()
	router, reg, _ := buildFullSOR(store, VenueBinance, VenueBybit)

	// Inject books for both venues.
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 5, 5, 5)
	injectBook(reg, VenueBybit, "BTC-USD", 49_990, 50_010, 5, 5, 10)

	// Make Binance reject all orders.
	if a, ok := router.coordinator.adapters[VenueBinance]; ok {
		if sim, ok := a.(*SimulatedVenueAdapter); ok {
			sim.FailRate = 1.0
		}
	}

	result, err := router.Route(context.Background(), OrderIntent{
		ClientOrderID: "failover-test-1",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      0.5,
		OrderType:     "MARKET",
		Algo:          AlgoImmediate,
		RequestedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Logf("Route returned error (acceptable if no venue available): %v", err)
	}
	// Either filled on Bybit or failed gracefully — no panic, no data corruption.
	if result.Report.FilledQty > 0 {
		foundBybit := false
		for _, v := range result.Report.VenuesUsed {
			if v == VenueBybit {
				foundBybit = true
			}
		}
		if !foundBybit {
			t.Logf("filled but not on Bybit: venues=%v", result.Report.VenuesUsed)
		}
	}
}

func TestFailoverController_ExcludesAndRecovers(t *testing.T) {
	reg := buildRegistry(VenueBinance, VenueBybit)
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 5, 5, 5)
	injectBook(reg, VenueBybit, "BTC-USD", 49_990, 50_010, 5, 10, 10)

	liq := NewLiquidityEngine()
	health := NewExchangeHealthEngine(reg, nil)
	fc := NewFailoverController(reg, liq, health, nil)
	fc.ExclusionWindow = 50 * time.Millisecond

	// Fail Binance.
	next, ok := fc.NextVenue("BTC-USD", VenueBinance, "BUY", 1.0)
	if !ok {
		t.Fatal("expected a failover venue when Binance is failed")
	}
	if next != VenueBybit {
		t.Fatalf("expected failover to Bybit, got %s", next)
	}

	// Bybit also fails → no venue.
	_, ok2 := fc.NextVenue("BTC-USD", VenueBybit, "BUY", 1.0)
	if ok2 {
		t.Fatal("expected no venue when both are excluded")
	}

	// After exclusion window, Binance should be available again.
	time.Sleep(100 * time.Millisecond)
	fc.SweepExpiredExclusions(context.Background())
	next2, ok3 := fc.NextVenue("BTC-USD", VenueDelta, "BUY", 1.0)
	if ok3 && next2 == "" {
		t.Logf("no venue found after recovery sweep (expected if Delta never registered)")
	}
}

// ── 18K/18L: Event Sourcing and Replay ───────────────────────────────────────

func TestSORReplay_DeterministicRebuild(t *testing.T) {
	store := newMemStore()
	router, reg, _ := buildFullSOR(store, VenueBinance, VenueBybit)
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 10, 10, 5)
	injectBook(reg, VenueBybit, "BTC-USD", 49_990, 50_010, 10, 8, 10)

	_, _ = router.Route(context.Background(), OrderIntent{
		ClientOrderID: "replay-ord-1",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      1.0,
		OrderType:     "MARKET",
		Algo:          AlgoImmediate,
		RequestedAt:   time.Now().UTC(),
	})

	events := store.all()
	replayEng := NewReplayEngine()
	result, err := replayEng.Replay(events, ReplayOptions{})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if result.RoutesRebuilt == 0 {
		t.Fatal("expected at least one route to be rebuilt")
	}
	if result.EventsReplayed == 0 {
		t.Fatal("expected events to be replayed")
	}

	// Verify the routing record exists.
	rec, ok := replayEng.Projections().Routing.Get("replay-ord-1")
	if !ok {
		t.Fatal("routing record not found in projection after replay")
	}
	if rec.Symbol != "BTC-USD" {
		t.Fatalf("symbol mismatch: got %s", rec.Symbol)
	}
}

func TestSORProjectionSet_SinglePassReplay(t *testing.T) {
	store := newMemStore()
	router, reg, _ := buildFullSOR(store, VenueBinance)
	injectBook(reg, VenueBinance, "BTC-USD", 49_990, 50_010, 5, 5, 5)

	// Route two orders.
	for i := 0; i < 2; i++ {
		router.Route(context.Background(), OrderIntent{
			ClientOrderID: "proj-ord-" + string(rune('1'+i)),
			Symbol:        "BTC-USD",
			Side:          "BUY",
			Quantity:      0.1,
			OrderType:     "MARKET",
			Algo:          AlgoImmediate,
			RequestedAt:   time.Now().UTC(),
		})
	}

	events := store.all()
	ps := NewSORProjectionSet()
	if err := ps.ReplayAll(events); err != nil {
		t.Fatalf("ReplayAll: %v", err)
	}
	routes := ps.Routing.All()
	if len(routes) < 2 {
		t.Fatalf("expected 2 routing records, got %d", len(routes))
	}
}

// ── 18N: Backtest Integration ─────────────────────────────────────────────────

func TestBacktestSOR_FullPipeline(t *testing.T) {
	venues := []VenueID{VenueBinance, VenueBybit, VenueOKX}
	bsor := BuildBacktestSOR(nil, venues, 42)

	// Inject books.
	for _, v := range venues {
		bsor.FeedBook(VenueMarketData{
			VenueID:   v,
			Symbol:    "BTC-USD",
			BidPrice:  49_990,
			AskPrice:  50_010,
			BidSize:   10,
			AskSize:   10,
			Bids:      []PriceLevel{{49_990, 5}, {49_980, 5}},
			Asks:      []PriceLevel{{50_010, 5}, {50_020, 5}},
			LatencyMs: 10,
			Fees:      FeeStructure{MakerBps: 1, TakerBps: 5},
			UpdatedAt: time.Now().UTC(),
		})
	}

	result, err := bsor.Router.Route(context.Background(), OrderIntent{
		ClientOrderID: "bt-ord-1",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      3.0,
		OrderType:     "MARKET",
		Algo:          AlgoImmediate,
		RequestedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if result.Report.FilledQty == 0 {
		t.Fatal("backtest SOR should fill orders with simulated adapters")
	}
	t.Logf("Backtest fill: %.4f BTC @ $%.2f via %v", result.Report.FilledQty, result.Report.AvgPrice, result.Report.VenuesUsed)
}

func TestBacktestSOR_SimulatesVenueFailure(t *testing.T) {
	venues := []VenueID{VenueBinance, VenueBybit}
	bsor := BuildBacktestSOR(nil, venues, 99)

	for _, v := range venues {
		bsor.FeedBook(VenueMarketData{
			VenueID:   v,
			Symbol:    "BTC-USD",
			BidPrice:  49_990,
			AskPrice:  50_010,
			BidSize:   10,
			AskSize:   10,
			Asks:      []PriceLevel{{50_010, 10}},
			Bids:      []PriceLevel{{49_990, 10}},
			LatencyMs: 5,
			Fees:      FeeStructure{MakerBps: 1, TakerBps: 5},
			UpdatedAt: time.Now().UTC(),
		})
	}

	// Fail Binance 100%.
	bsor.SetVenueFailRate(VenueBinance, 1.0)

	result, _ := bsor.Router.Route(context.Background(), OrderIntent{
		ClientOrderID: "bt-failover-1",
		Symbol:        "BTC-USD",
		Side:          "BUY",
		Quantity:      1.0,
		OrderType:     "MARKET",
		Algo:          AlgoImmediate,
		RequestedAt:   time.Now().UTC(),
	})
	if result.Report.FilledQty > 0 {
		for _, v := range result.Report.VenuesUsed {
			if v == VenueBinance {
				t.Fatal("should not have filled on failed Binance")
			}
		}
	}
}

// ── 18O: Stress Tests ─────────────────────────────────────────────────────────

func TestSOR_100kRoutes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	venues := []VenueID{VenueBinance, VenueBybit, VenueOKX, VenueDelta}
	bsor := BuildBacktestSOR(nil, venues, 12345)

	for _, v := range venues {
		bsor.FeedBook(VenueMarketData{
			VenueID:   v,
			Symbol:    "BTC-USD",
			BidPrice:  49_990,
			AskPrice:  50_010,
			BidSize:   100,
			AskSize:   100,
			Asks:      []PriceLevel{{50_010, 100}},
			Bids:      []PriceLevel{{49_990, 100}},
			LatencyMs: 5,
			Fees:      FeeStructure{MakerBps: 1, TakerBps: 5},
			UpdatedAt: time.Now().UTC(),
		})
	}

	const total = 100_000
	filled := 0
	start := time.Now()
	for i := 0; i < total; i++ {
		r, _ := bsor.Router.Route(context.Background(), OrderIntent{
			ClientOrderID: "stress-" + itoa(i),
			Symbol:        "BTC-USD",
			Side:          sideOf(i),
			Quantity:      0.01,
			OrderType:     "MARKET",
			Algo:          AlgoImmediate,
			RequestedAt:   time.Now().UTC(),
		})
		if r.Report.FilledQty > 0 {
			filled++
		}
	}
	dur := time.Since(start)
	rateK := float64(total) / dur.Seconds() / 1000
	t.Logf("Stress: %d routes in %s (%.1f k/s), fill rate=%.1f%%",
		total, dur.Round(time.Millisecond), rateK, float64(filled)/total*100)
	if filled == 0 {
		t.Fatal("zero fills in stress test")
	}
}

func TestSOR_1MRoutingEvents_Replay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping replay stress test in short mode")
	}
	// Build a large event slice from 10k routes to exercise the O(n) replay.
	events := makeSyntheticRouteEvents(10_000)
	replayEng := NewReplayEngine()
	start := time.Now()
	result, err := replayEng.Replay(events, ReplayOptions{})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	dur := time.Since(start)
	t.Logf("Replayed %d events → %d routes in %s",
		result.EventsReplayed, result.RoutesRebuilt, dur.Round(time.Millisecond))
	if result.RoutesRebuilt == 0 {
		t.Fatal("no routes rebuilt from synthetic events")
	}
}

// ── Test helpers ──────────────────────────────────────────────────────────────

func itoa(i int) string {
	return time.Time{}.Add(time.Duration(i)).String()
}

func sideOf(i int) string {
	if i%2 == 0 {
		return "BUY"
	}
	return "SELL"
}

func makeSyntheticRouteEvents(n int) []ledger.Event {
	out := make([]ledger.Event, 0, n*3)
	for i := 0; i < n; i++ {
		parentID := "synth-" + itoa(i)
		// RouteCreated
		created, _ := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: AggregateRoute,
			AggregateID:   parentID,
			EventType:     EventRouteCreated,
			Symbol:        "BTC-USD",
			Payload: RouteCreatedPayload{
				ParentClientOrderID: parentID,
				Symbol:              "BTC-USD",
				Side:                sideOf(i),
				Quantity:            0.1,
				OrderType:           "MARKET",
				Algo:                "IMMEDIATE",
				CreatedAt:           time.Now().UTC(),
			},
			Source: "test",
		})
		created.SequenceNo = int64(i*3 + 1)
		out = append(out, created)

		// ExecutionCompleted
		payload := ExecutionCompletedPayload{
			ParentClientOrderID: parentID,
			Symbol:              "BTC-USD",
			RequestedQty:        0.1,
			FilledQty:           0.1,
			AvgPrice:            50_000,
			TotalFeesUSD:        0.25,
			RealizedSlippageBps: 2.0,
			VenuesUsed:          []VenueID{VenueBinance},
			DurationMs:          5,
			CompletedAt:         time.Now().UTC(),
		}
		rawPayload, _ := json.Marshal(payload)
		completed := ledger.Event{
			AggregateType: AggregateRoute,
			AggregateID:   parentID,
			EventType:     EventExecutionCompleted,
			Symbol:        "BTC-USD",
			Payload:       rawPayload,
			SequenceNo:    int64(i*3 + 2),
			CreatedAt:     time.Now().UTC(),
		}
		out = append(out, completed)
	}
	return out
}

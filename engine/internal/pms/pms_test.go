package pms

import (
	"context"
	"math"
	"sort"
	"sync"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── In-memory ledger store for tests ─────────────────────────────────────────

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

// allEvents returns all stored events — used only within tests.
func (m *memStore) allEvents() []ledger.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]ledger.Event(nil), m.events...)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newTestPortfolioManager(t *testing.T) (*PortfolioManager, *memStore) {
	t.Helper()
	store := newMemStore()
	mgr := NewPortfolioManager(store)
	return mgr, store
}

func createTestPortfolio(t *testing.T, mgr *PortfolioManager, id, name string, nav float64) *Portfolio {
	t.Helper()
	p, err := mgr.CreatePortfolio(context.Background(), PortfolioCreatedPayload{
		PortfolioID:  id,
		Name:         name,
		Type:         "MASTER",
		BaseCurrency: "USD",
		InitialNAV:   nav,
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreatePortfolio: %v", err)
	}
	return p
}

// ── 17A: Portfolio Replay ─────────────────────────────────────────────────────

func TestPortfolioReplay_Deterministic(t *testing.T) {
	mgr, store := newTestPortfolioManager(t)
	ctx := context.Background()

	p := createTestPortfolio(t, mgr, "p1", "Test Master", 1_000_000)
	if p.CurrentNAV != 1_000_000 {
		t.Fatalf("initial NAV: got %.0f want 1000000", p.CurrentNAV)
	}

	// Update NAV
	if err := mgr.UpdateNAV(ctx, "p1", 1_050_000); err != nil {
		t.Fatalf("UpdateNAV: %v", err)
	}

	// Replay from events in store
	evts, _ := store.Replay(ctx, AggregatePortfolio, "p1")
	p2, err := ReplayPortfolio(evts)
	if err != nil {
		t.Fatalf("ReplayPortfolio: %v", err)
	}
	if p2.CurrentNAV != 1_050_000 {
		t.Fatalf("replayed NAV: got %.0f want 1050000", p2.CurrentNAV)
	}
	if p2.Version != p.Version {
		t.Fatalf("version mismatch: replayed=%d live=%d", p2.Version, p.Version)
	}
}

func TestPortfolioReplay_AllocationRoundtrip(t *testing.T) {
	mgr, store := newTestPortfolioManager(t)
	ctx := context.Background()

	createTestPortfolio(t, mgr, "p2", "Alloc Test", 500_000)

	allocEngine := NewAllocationEngine(mgr, store)
	_, err := allocEngine.Allocate(ctx, AllocationRequest{
		PortfolioID:       "p2",
		TotalCapitalUSD:   500_000,
		Method:            AllocationFixed,
		FixedWeights:      map[string]float64{"strat_ema": 20.0, "strat_cvd": 15.0},
		Strategies: []StrategyMetrics{
			{StrategyID: "strat_ema", StrategyName: "EMA Cross"},
			{StrategyID: "strat_cvd", StrategyName: "CVD Divergence"},
		},
		CashReservePct:    10.0,
		MaxSingleAllocPct: 30.0,
	})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// Replay and verify allocations are preserved
	evts, _ := store.Replay(ctx, AggregatePortfolio, "p2")
	replayed, err := ReplayPortfolio(evts)
	if err != nil {
		t.Fatalf("ReplayPortfolio: %v", err)
	}
	if len(replayed.Allocations) != 2 {
		t.Fatalf("allocations: got %d want 2", len(replayed.Allocations))
	}
}

// ── 17B: Allocation Engine ────────────────────────────────────────────────────

func TestAllocationEngine_FixedAllocSumsToTarget(t *testing.T) {
	mgr, store := newTestPortfolioManager(t)
	createTestPortfolio(t, mgr, "p3", "Fixed Alloc", 1_000_000)

	engine := NewAllocationEngine(mgr, store)
	result, err := engine.Allocate(context.Background(), AllocationRequest{
		PortfolioID:     "p3",
		TotalCapitalUSD: 1_000_000,
		Method:          AllocationFixed,
		FixedWeights:    map[string]float64{"s1": 20.0, "s2": 15.0, "s3": 10.0},
		Strategies: []StrategyMetrics{
			{StrategyID: "s1", StrategyName: "S1"},
			{StrategyID: "s2", StrategyName: "S2"},
			{StrategyID: "s3", StrategyName: "S3"},
		},
		CashReservePct: 5.0,
	})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	total := 0.0
	for _, w := range result.Weights {
		total += w.AllocPct
	}
	// Total must not exceed (100 - cash reserve) = 95%
	if total > 96.0 {
		t.Fatalf("total alloc %.2f%% exceeds deployable budget", total)
	}
	if result.CashReserveUSD != 50_000 {
		t.Fatalf("cash reserve: got %.0f want 50000", result.CashReserveUSD)
	}
}

func TestAllocationEngine_SharpeWeighted(t *testing.T) {
	mgr, store := newTestPortfolioManager(t)
	createTestPortfolio(t, mgr, "p4", "Sharpe Alloc", 1_000_000)

	engine := NewAllocationEngine(mgr, store)
	strategies := []StrategyMetrics{
		{StrategyID: "s1", StrategyName: "High Sharpe", SharpeRatio: 2.0},
		{StrategyID: "s2", StrategyName: "Mid Sharpe", SharpeRatio: 1.0},
		{StrategyID: "s3", StrategyName: "Low Sharpe", SharpeRatio: 0.5},
	}
	result, err := engine.Allocate(context.Background(), AllocationRequest{
		PortfolioID:     "p4",
		TotalCapitalUSD: 1_000_000,
		Method:          AllocationSharpeWeighted,
		Strategies:      strategies,
		CashReservePct:  5.0,
	})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	var highAlloc, lowAlloc float64
	for _, w := range result.Weights {
		switch w.StrategyID {
		case "s1":
			highAlloc = w.AllocPct
		case "s3":
			lowAlloc = w.AllocPct
		}
	}
	if highAlloc <= lowAlloc {
		t.Fatalf("high-Sharpe alloc %.2f%% should exceed low-Sharpe %.2f%%", highAlloc, lowAlloc)
	}
}

// ── 17C: Strategy Budget Engine ───────────────────────────────────────────────

func TestStrategyBudgetEngine_BlocksOverBudget(t *testing.T) {
	mgr, store := newTestPortfolioManager(t)
	createTestPortfolio(t, mgr, "p5", "Budget Test", 1_000_000)

	budgetEngine := NewStrategyBudgetEngine(mgr, store)
	err := budgetEngine.SetBudget(context.Background(), StrategyBudget{
		StrategyID:         "s_ema",
		StrategyName:       "EMA Cross",
		PortfolioID:        "p5",
		TotalBudgetUSD:     100_000,
		DailyLossLimitUSD:  5_000,
		WeeklyLossLimitUSD: 15_000,
		MonthlyDDLimitUSD:  30_000,
	})
	if err != nil {
		t.Fatalf("SetBudget: %v", err)
	}

	// 80k is within budget
	if v := budgetEngine.CheckBudget("s_ema", 80_000); v != nil {
		t.Fatalf("expected nil violation for 80k, got %+v", v)
	}

	// 120k exceeds 100k budget
	v := budgetEngine.CheckBudget("s_ema", 120_000)
	if v == nil {
		t.Fatal("expected TOTAL_BUDGET violation for 120k, got nil")
	}
	if v.Type != "TOTAL_BUDGET" {
		t.Fatalf("violation type: got %s want TOTAL_BUDGET", v.Type)
	}
}

func TestStrategyBudgetEngine_DailyLossAutoDisables(t *testing.T) {
	mgr, store := newTestPortfolioManager(t)
	createTestPortfolio(t, mgr, "p6", "Loss Test", 500_000)

	engine := NewStrategyBudgetEngine(mgr, store)
	_ = engine.SetBudget(context.Background(), StrategyBudget{
		StrategyID:        "s_rsi",
		StrategyName:      "RSI",
		PortfolioID:       "p6",
		TotalBudgetUSD:    50_000,
		DailyLossLimitUSD: 2_500,
	})

	// Record loss exceeding daily limit
	engine.RecordLoss(context.Background(), "s_rsi", 3_000)

	b := engine.budgets["s_rsi"]
	if b == nil {
		t.Fatal("budget not found")
	}
	if b.Enabled {
		t.Fatal("strategy should be auto-disabled after daily loss breach")
	}
}

// ── 17D: Portfolio Risk Budget ────────────────────────────────────────────────

func TestPortfolioRiskBudget_BlocksWhenHeatExceeded(t *testing.T) {
	store := newMemStore()
	rb := NewPortfolioRiskBudget(store)

	budget := RiskBudget{
		MaxHeatPct:      10.0,
		MaxDrawdownPct:  20.0,
		MaxDailyLossPct: 3.0,
		MaxGrossExpPct:  200.0,
	}
	rb.InitPortfolio("p_risk", budget)

	// Simulate 8% heat already consumed
	rb.UpdateState("p_risk", 8.0, 2.0, 3.0, 5.0, 0, 0, 0, 50.0, 10.0, nil, nil, nil)

	equity := 1_000_000.0
	// 30k additional dollar risk → adds 3% heat → 8+3=11% > 10% limit
	violations := rb.CheckPortfolioRisk(context.Background(), "p_risk", budget, 30_000, equity)
	if len(violations) == 0 {
		t.Fatal("expected HEAT violation but got none")
	}
	found := false
	for _, v := range violations {
		if v.Type == "HEAT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected HEAT violation, got %+v", violations)
	}
}

// ── 17E: Exposure Aggregation ─────────────────────────────────────────────────

func TestExposureAggregation_GrossAndNet(t *testing.T) {
	store := newMemStore()
	engine := NewExposureAggregationEngine(store)
	engine.SetNAV("port_exp", 1_000_000)

	engine.AddPosition(context.Background(), "port_exp", PositionExposure{
		PositionID:  "pos1",
		StrategyID:  "s1",
		Symbol:      "BTC-USD",
		Exchange:    "BINANCE",
		Sector:      "CRYPTO",
		Side:        "BUY",
		Quantity:    1.0,
		EntryPrice:  50_000,
		MarkPrice:   50_000,
		NotionalUSD: 50_000,
	})
	engine.AddPosition(context.Background(), "port_exp", PositionExposure{
		PositionID:  "pos2",
		StrategyID:  "s2",
		Symbol:      "BTC-USD",
		Exchange:    "DELTA",
		Sector:      "CRYPTO",
		Side:        "SELL",
		Quantity:    0.5,
		EntryPrice:  50_000,
		MarkPrice:   50_000,
		NotionalUSD: 25_000,
	})

	snap := engine.Snapshot("port_exp")
	if snap.GrossNotionalUSD != 75_000 {
		t.Fatalf("gross: got %.0f want 75000", snap.GrossNotionalUSD)
	}
	if snap.NetNotionalUSD != 25_000 {
		t.Fatalf("net: got %.0f want 25000", snap.NetNotionalUSD)
	}
	if math.Abs(snap.GrossExpPct-7.5) > 0.01 {
		t.Fatalf("gross exp pct: got %.2f want 7.50", snap.GrossExpPct)
	}
}

// ── 17F: Portfolio Optimizer ──────────────────────────────────────────────────

func TestPortfolioOptimizer_RiskParityLowerVolGetsHigherWeight(t *testing.T) {
	opt := NewPortfolioOptimizer()
	result, err := opt.Optimize(context.Background(), OptimizationInput{
		PortfolioID: "p_opt",
		Method:      OptMethodRiskParity,
		Strategies: []StrategyProfile{
			{StrategyID: "low_vol", StrategyName: "Low Vol", VolatilityAnnual: 0.05},
			{StrategyID: "high_vol", StrategyName: "High Vol", VolatilityAnnual: 0.20},
		},
		CashReservePct: 0,
	})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	var lowVolPct, highVolPct float64
	for _, w := range result.OptimalWeights {
		switch w.StrategyID {
		case "low_vol":
			lowVolPct = w.AllocPct
		case "high_vol":
			highVolPct = w.AllocPct
		}
	}
	if lowVolPct <= highVolPct {
		t.Fatalf("risk parity: low-vol %.2f%% should exceed high-vol %.2f%%", lowVolPct, highVolPct)
	}
}

func TestPortfolioOptimizer_MVOPreservesTotalWeight(t *testing.T) {
	opt := NewPortfolioOptimizer()
	result, err := opt.Optimize(context.Background(), OptimizationInput{
		PortfolioID: "p_mvo",
		Method:      OptMethodMVO,
		Strategies: []StrategyProfile{
			{StrategyID: "s1", SharpeRatio: 1.5, VolatilityAnnual: 0.10},
			{StrategyID: "s2", SharpeRatio: 0.8, VolatilityAnnual: 0.15},
			{StrategyID: "s3", SharpeRatio: 1.2, VolatilityAnnual: 0.12},
		},
		CashReservePct: 5.0,
	})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	total := 0.0
	for _, w := range result.OptimalWeights {
		total += w.AllocPct
	}
	// Must be ≤ deployable = 95%
	if total > 96.0 {
		t.Fatalf("MVO total weight %.2f%% exceeds deployable budget", total)
	}
}

// ── 17G: Multi-Account Isolation ─────────────────────────────────────────────

func TestAccountManager_CapitalSegregation(t *testing.T) {
	store := newMemStore()
	mgr := NewAccountManager(store)
	ctx := context.Background()

	_, err := mgr.CreateAccount(ctx, ManagedAccount{
		AccountID:  "acc1",
		Name:       "Prop Account",
		Type:       AccountTypeProp,
		Currency:   "USD",
		InitialNAV: 500_000,
	})
	if err != nil {
		t.Fatalf("CreateAccount acc1: %v", err)
	}
	_, err = mgr.CreateAccount(ctx, ManagedAccount{
		AccountID:  "acc2",
		Name:       "Paper Account",
		Type:       AccountTypePaper,
		Currency:   "USD",
		InitialNAV: 1_000_000,
	})
	if err != nil {
		t.Fatalf("CreateAccount acc2: %v", err)
	}

	// Reserve from acc1
	if err := mgr.ReserveCapital("acc1", 100_000); err != nil {
		t.Fatalf("ReserveCapital acc1: %v", err)
	}

	// acc2 must be unaffected
	snap2, _ := mgr.Snapshot("acc2")
	if snap2.AvailableCash != 1_000_000 {
		t.Fatalf("acc2 cash affected: got %.0f want 1000000", snap2.AvailableCash)
	}

	// acc1 must reflect reservation
	snap1, _ := mgr.Snapshot("acc1")
	if snap1.AvailableCash != 400_000 {
		t.Fatalf("acc1 cash: got %.0f want 400000", snap1.AvailableCash)
	}

	// Must reject overdraft
	err = mgr.ReserveCapital("acc1", 500_000)
	if err == nil {
		t.Fatal("expected ErrInsufficientFunds but got nil")
	}
}

// ── 17H: Master/Sub Account ───────────────────────────────────────────────────

func TestMasterAccountController_DistributesCapital(t *testing.T) {
	store := newMemStore()
	accMgr := NewAccountManager(store)
	portfolioMgr := NewPortfolioManager(store)
	rb := NewPortfolioRiskBudget(store)
	ctx := context.Background()

	_, _ = accMgr.CreateAccount(ctx, ManagedAccount{
		AccountID:  "master1",
		Name:       "Master",
		Type:       AccountTypeMaster,
		Currency:   "USD",
		InitialNAV: 2_000_000,
	})

	ctrl := NewMasterAccountController(accMgr, portfolioMgr, rb, store)
	subs := []SubAllocationSpec{
		{SubAccountID: "sub1", SubPortfolioID: "port_sub1", AllocPct: 40.0, Enabled: true},
		{SubAccountID: "sub2", SubPortfolioID: "port_sub2", AllocPct: 30.0, Enabled: true},
	}
	err := ctrl.RegisterMaster(ctx, "master1", "port_master1", 10.0, subs)
	if err != nil {
		t.Fatalf("RegisterMaster: %v", err)
	}

	snap, ok := ctrl.MasterSnapshot("master1")
	if !ok {
		t.Fatal("master snapshot not found")
	}
	if len(snap.SubAllocations) != 2 {
		t.Fatalf("sub allocations: got %d want 2", len(snap.SubAllocations))
	}
	if snap.AllocatedPct != 70.0 {
		t.Fatalf("allocated pct: got %.1f want 70.0", snap.AllocatedPct)
	}
}

// ── 17J/17K: PMS Replay and Projections ──────────────────────────────────────

func TestPMSReplay_DeterministicRebuild(t *testing.T) {
	store := newMemStore()
	mgr := NewPortfolioManager(store)
	accMgr := NewAccountManager(store)
	budgetEngine := NewStrategyBudgetEngine(mgr, store)
	ctx := context.Background()

	_, _ = mgr.CreatePortfolio(ctx, PortfolioCreatedPayload{
		PortfolioID:  "replay_p1",
		Name:         "Replay Test",
		Type:         "MASTER",
		BaseCurrency: "USD",
		InitialNAV:   1_000_000,
		CreatedAt:    time.Now().UTC(),
	})
	_ = mgr.UpdateNAV(ctx, "replay_p1", 1_100_000)

	// Rebuild a fresh manager via replay
	mgr2 := NewPortfolioManager(store)
	accMgr2 := NewAccountManager(store)
	budgetEngine2 := NewStrategyBudgetEngine(mgr2, store)
	replayEngine := NewPMSReplayEngine(mgr2, accMgr2, budgetEngine2)

	allEvents := store.allEvents()
	result, err := replayEngine.Replay(ctx, allEvents, ReplayOptions{})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if result.PortfoliosLoaded != 1 {
		t.Fatalf("portfolios loaded: got %d want 1", result.PortfoliosLoaded)
	}

	p, err := mgr2.Get("replay_p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p.CurrentNAV != 1_100_000 {
		t.Fatalf("replayed NAV: got %.0f want 1100000", p.CurrentNAV)
	}

	_ = accMgr
	_ = budgetEngine
}

func TestPMSProjectionSet_SinglePassReplay(t *testing.T) {
	store := newMemStore()
	mgr := NewPortfolioManager(store)
	ctx := context.Background()

	_, _ = mgr.CreatePortfolio(ctx, PortfolioCreatedPayload{
		PortfolioID:  "proj_p1",
		Name:         "Projection Test",
		Type:         "MASTER",
		BaseCurrency: "USD",
		InitialNAV:   500_000,
		CreatedAt:    time.Now().UTC(),
	})
	_ = mgr.UpdateNAV(ctx, "proj_p1", 520_000)

	events := store.allEvents()
	projSet := NewPMSProjectionSet()
	if err := projSet.ReplayAll(events); err != nil {
		t.Fatalf("ReplayAll: %v", err)
	}

	state, ok := projSet.Portfolio.State("proj_p1")
	if !ok {
		t.Fatal("portfolio not found in projection")
	}
	if state.CurrentNAV != 520_000 {
		t.Fatalf("projected NAV: got %.0f want 520000", state.CurrentNAV)
	}
}

// ── 17M: No Capital Leakage ───────────────────────────────────────────────────

func TestNoCapitalLeakage(t *testing.T) {
	store := newMemStore()
	accMgr := NewAccountManager(store)
	ctx := context.Background()

	_, _ = accMgr.CreateAccount(ctx, ManagedAccount{
		AccountID:  "leak_acc",
		Name:       "Leak Test",
		Type:       AccountTypeProp,
		Currency:   "USD",
		InitialNAV: 1_000_000,
	})

	const trades = 100
	for i := 0; i < trades; i++ {
		_ = accMgr.ReserveCapital("leak_acc", 10_000)
		pnl := 500.0
		if i%3 == 0 {
			pnl = -300.0
		}
		accMgr.ReleaseCapital("leak_acc", 10_000, pnl)
	}

	snap, _ := accMgr.Snapshot("leak_acc")
	// Count exactly: i%3==0 for i in [0,99] → 0,3,6,...,99 → 34 values
	losses := 0
	for i := 0; i < trades; i++ {
		if i%3 == 0 {
			losses++
		}
	}
	wins := trades - losses
	expectedPnL := float64(wins)*500.0 + float64(losses)*(-300.0)
	expectedNAV := 1_000_000.0 + expectedPnL
	if math.Abs(snap.CurrentNAV-expectedNAV) > 0.01 {
		t.Fatalf("capital leakage: NAV=%.2f expected=%.2f diff=%.6f",
			snap.CurrentNAV, expectedNAV, snap.CurrentNAV-expectedNAV)
	}
	if snap.AllocatedUSD != 0 {
		t.Fatalf("no open positions expected, allocated=%.2f", snap.AllocatedUSD)
	}
}

// ── Analytics Tests ───────────────────────────────────────────────────────────

func TestPortfolioAnalytics_SharpeRatio(t *testing.T) {
	engine := NewAnalyticsEngine()

	// Synthetic 252-day return series: 10% annual return, 15% annualised vol
	daily := 0.10 / 252
	vol := 0.15 / math.Sqrt(252)
	returns := make([]float64, 252)
	for i := range returns {
		returns[i] = daily + vol*(float64(i%7)-3.0)/10.0
	}

	perf := engine.Compute("p_analytics", 1_000_000, nil, returns, 0.05, nil)
	if perf.SharpeRatio <= 0 {
		t.Fatalf("Sharpe ratio should be positive, got %.4f", perf.SharpeRatio)
	}
	if perf.AnnualisedReturnPct <= 0 {
		t.Fatalf("annualised return should be positive, got %.4f", perf.AnnualisedReturnPct)
	}
	if perf.MaxDrawdownPct < 0 {
		t.Fatalf("max drawdown should be non-negative, got %.4f", perf.MaxDrawdownPct)
	}
}

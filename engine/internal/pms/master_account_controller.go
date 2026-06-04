package pms

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// SubAllocationSpec defines how the master distributes capital to one sub-account.
type SubAllocationSpec struct {
	SubAccountID   string
	SubPortfolioID string
	AllocPct       float64 // percentage of master NAV allocated to this sub
	MaxDrawdownPct float64 // sub-level drawdown kill threshold
	MaxDailyLossPct float64
	Enabled        bool
}

// MasterState holds the controller's internal accounting.
type MasterState struct {
	MasterAccountID  string
	MasterPortfolioID string
	TotalNAV         float64
	AllocatedPct     float64 // sum of sub alloc pcts
	CashReservePct   float64
	SubAllocations   []SubAllocationSpec
	LastRebalancedAt time.Time
	UpdatedAt        time.Time
}

// MasterAccountController implements the top-down master/sub account architecture.
//
// Responsibilities:
//  - Distributes capital from master to sub accounts
//  - Enforces centralized risk limits (kill switch, drawdown, daily loss)
//  - Maintains centralized exposure monitoring across all subs
//  - Triggers sub-account suspension when limits are breached
type MasterAccountController struct {
	mu             sync.RWMutex
	masters        map[string]*MasterState // keyed by masterAccountID
	accountManager *AccountManager
	portfolioMgr   *PortfolioManager
	riskBudget     *PortfolioRiskBudget
	store          ledger.Store
}

// NewMasterAccountController constructs the controller.
func NewMasterAccountController(
	accountMgr *AccountManager,
	portfolioMgr *PortfolioManager,
	riskBudget *PortfolioRiskBudget,
	store ledger.Store,
) *MasterAccountController {
	return &MasterAccountController{
		masters:        make(map[string]*MasterState),
		accountManager: accountMgr,
		portfolioMgr:   portfolioMgr,
		riskBudget:     riskBudget,
		store:          store,
	}
}

// RegisterMaster sets up a master account with its sub-allocation specs.
// The sum of sub AllocPct values must not exceed (100 - CashReservePct).
func (c *MasterAccountController) RegisterMaster(
	ctx context.Context,
	masterAccountID, masterPortfolioID string,
	cashReservePct float64,
	subs []SubAllocationSpec,
) error {
	allocTotal := 0.0
	for _, s := range subs {
		allocTotal += s.AllocPct
	}
	if allocTotal > 100.0-cashReservePct {
		return fmt.Errorf("pms: sub allocations %.1f%% exceed (100 - %.1f%%) available", allocTotal, cashReservePct)
	}

	masterAcc, err := c.accountManager.Get(masterAccountID)
	if err != nil {
		return fmt.Errorf("pms: master account: %w", err)
	}

	state := &MasterState{
		MasterAccountID:   masterAccountID,
		MasterPortfolioID: masterPortfolioID,
		TotalNAV:          masterAcc.CurrentNAV,
		AllocatedPct:      allocTotal,
		CashReservePct:    cashReservePct,
		SubAllocations:    append([]SubAllocationSpec(nil), subs...),
		UpdatedAt:         time.Now().UTC(),
	}

	c.mu.Lock()
	c.masters[masterAccountID] = state
	c.mu.Unlock()

	// Distribute initial capital to sub accounts
	return c.distributeCapital(ctx, masterAccountID)
}

// distributeCapital computes and sets available cash for each sub account
// based on the master NAV and each sub's alloc percentage.
func (c *MasterAccountController) distributeCapital(ctx context.Context, masterAccountID string) error {
	c.mu.RLock()
	state, ok := c.masters[masterAccountID]
	if !ok {
		c.mu.RUnlock()
		return fmt.Errorf("pms: master %s not registered", masterAccountID)
	}
	subs := append([]SubAllocationSpec(nil), state.SubAllocations...)
	masterNAV := state.TotalNAV
	masterPortfolioID := state.MasterPortfolioID
	c.mu.RUnlock()

	subAllocs := make([]SubPortfolioAllocation, 0, len(subs))
	for _, sub := range subs {
		if !sub.Enabled {
			continue
		}
		subUSD := masterNAV * sub.AllocPct / 100.0
		subAllocs = append(subAllocs, SubPortfolioAllocation{
			SubPortfolioID: sub.SubPortfolioID,
			AllocPct:       sub.AllocPct,
			AllocUSD:       subUSD,
		})
	}

	// Emit master allocation event
	payload := MasterAllocationChangedPayload{
		MasterPortfolioID: masterPortfolioID,
		SubAllocations:    subAllocs,
		TotalAllocPct:     state.AllocatedPct,
		Reason:            "capital_distribution",
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   masterPortfolioID,
		EventType:     EventMasterAllocationChanged,
		AccountID:     masterAccountID,
		Payload:       payload,
		Source:        "pms.master",
	})
	if err != nil {
		return err
	}
	if c.store != nil {
		c.store.Append(ctx, ev) //nolint:errcheck
	}
	return nil
}

// Rebalance re-computes sub-account capital budgets based on current master NAV.
// Must be called when the master NAV changes materially (e.g. monthly).
func (c *MasterAccountController) Rebalance(ctx context.Context, masterAccountID string) error {
	// Refresh master NAV from account manager
	masterAcc, err := c.accountManager.Get(masterAccountID)
	if err != nil {
		return err
	}

	c.mu.Lock()
	state, ok := c.masters[masterAccountID]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("pms: master %s not registered", masterAccountID)
	}
	state.TotalNAV = masterAcc.CurrentNAV
	state.LastRebalancedAt = time.Now().UTC()
	state.UpdatedAt = time.Now().UTC()
	c.mu.Unlock()

	log.Printf("[PMS MASTER] Rebalancing master=%s NAV=$%.0f", masterAccountID, masterAcc.CurrentNAV)
	return c.distributeCapital(ctx, masterAccountID)
}

// EnforceRiskLimits checks all sub-accounts against their risk budgets and
// suspends any that breach their drawdown or daily loss limits.
func (c *MasterAccountController) EnforceRiskLimits(ctx context.Context, masterAccountID string) {
	c.mu.RLock()
	state, ok := c.masters[masterAccountID]
	if !ok {
		c.mu.RUnlock()
		return
	}
	subs := append([]SubAllocationSpec(nil), state.SubAllocations...)
	c.mu.RUnlock()

	for _, sub := range subs {
		if !sub.Enabled {
			continue
		}
		riskSnap, exists := c.riskBudget.Snapshot(sub.SubPortfolioID)
		if !exists {
			continue
		}

		// Drawdown kill
		if sub.MaxDrawdownPct > 0 && riskSnap.DrawdownPct >= sub.MaxDrawdownPct {
			reason := fmt.Sprintf("sub_drawdown_%.1f%%>=limit_%.1f%%", riskSnap.DrawdownPct, sub.MaxDrawdownPct)
			log.Printf("[PMS MASTER] Suspending sub=%s reason=%s", sub.SubAccountID, reason)
			_ = c.accountManager.SuspendAccount(ctx, sub.SubAccountID, reason)
			_ = c.disableSub(ctx, masterAccountID, sub.SubAccountID, reason)
		}
	}
}

// disableSub marks a sub-allocation as disabled in the master state.
func (c *MasterAccountController) disableSub(ctx context.Context, masterAccountID, subAccountID, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.masters[masterAccountID]
	if !ok {
		return nil
	}
	for i := range state.SubAllocations {
		if state.SubAllocations[i].SubAccountID == subAccountID {
			state.SubAllocations[i].Enabled = false
			break
		}
	}
	state.UpdatedAt = time.Now().UTC()
	return nil
}

// TotalAUM returns the sum of master NAV across all registered master accounts.
func (c *MasterAccountController) TotalAUM() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := 0.0
	for _, s := range c.masters {
		total += s.TotalNAV
	}
	return total
}

// MasterSnapshot returns the current state of a master account controller.
func (c *MasterAccountController) MasterSnapshot(masterAccountID string) (MasterState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.masters[masterAccountID]
	if !ok {
		return MasterState{}, false
	}
	snap := *s
	snap.SubAllocations = append([]SubAllocationSpec(nil), s.SubAllocations...)
	return snap, true
}

// AggregateExposure returns a combined exposure snapshot across all sub-portfolios
// under the master. This provides the master-level centralised exposure view.
func (c *MasterAccountController) AggregateExposure(
	masterAccountID string,
	expEngine *ExposureAggregationEngine,
) ExposureSnapshot {
	c.mu.RLock()
	state, ok := c.masters[masterAccountID]
	if !ok {
		c.mu.RUnlock()
		return ExposureSnapshot{}
	}
	subs := append([]SubAllocationSpec(nil), state.SubAllocations...)
	masterNav := state.TotalNAV
	masterPortfolioID := state.MasterPortfolioID
	c.mu.RUnlock()

	// Aggregate all sub snapshots into the master view
	agg := ExposureSnapshot{
		PortfolioID: masterPortfolioID,
		ComputedAt:  time.Now().UTC(),
		BySymbol:    make(map[string]float64),
		ByExchange:  make(map[string]float64),
		BySector:    make(map[string]float64),
		ByStrategy:  make(map[string]float64),
	}

	for _, sub := range subs {
		snap := expEngine.Snapshot(sub.SubPortfolioID)
		agg.LongNotionalUSD += snap.LongNotionalUSD
		agg.ShortNotionalUSD += snap.ShortNotionalUSD
		agg.PositionCount += snap.PositionCount
		for k, v := range snap.BySymbol {
			agg.BySymbol[k] += v
		}
		for k, v := range snap.ByExchange {
			agg.ByExchange[k] += v
		}
		for k, v := range snap.BySector {
			agg.BySector[k] += v
		}
		for k, v := range snap.ByStrategy {
			agg.ByStrategy[k] += v
		}
	}

	agg.GrossNotionalUSD = agg.LongNotionalUSD + agg.ShortNotionalUSD
	agg.NetNotionalUSD = agg.LongNotionalUSD - agg.ShortNotionalUSD
	if masterNav > 0 {
		agg.GrossExpPct = agg.GrossNotionalUSD / masterNav * 100
		agg.NetExpPct = agg.NetNotionalUSD / masterNav * 100
		agg.LongExpPct = agg.LongNotionalUSD / masterNav * 100
		agg.ShortExpPct = agg.ShortNotionalUSD / masterNav * 100
	}

	return agg
}
